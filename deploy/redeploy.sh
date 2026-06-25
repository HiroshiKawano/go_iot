#!/usr/bin/env bash
# redeploy.sh — go_iot 本番(AWS Lightsail: go-iot-prod)へのオンデマンド・リデプロイ自動化
#
# 背景: SSH 接続元(egress)が VPN で接続ごとに変動し、管理元 IP 自体も動的なため、
#       SSH(22) を単一 /32 に固定する運用は破綻する。本スクリプトは毎回その場で
#       現在の egress を検出して 22 を一時開放し、作業後に必ず管理元 /32 へ復元する。
#
# 流れ:
#   1. amd64 クロスビルド (sync-css -> templ generate -> go build)   ※FW開放前に済ませ開放時間を最小化
#   2. 現在の egress IP を検出 (3回サンプル: 全一致=/32 / 変動=/24)
#   3. Lightsail FW の 22 を [検出CIDR, 管理元CIDR] へ一時開放 (80/443 据置・8080/5432 非公開維持)
#   4. 一時 SSH 鍵を取得し 22 到達を待機
#   5. scp -> 旧バイナリ backup -> swap -> systemd restart -> 検証 (失敗時は旧バイナリへ自動ロールバック)
#   6. 外部 HTTPS /health と配信 Version を検証
#   7. ★trap EXIT で必ず★ FW を管理元 /32 のみへ復元 (途中失敗・中断でも実行)
#
# 前提: aws CLI v2 + プロファイル go-iot-mcp / git / go / python3 / nc。
#       FW 変更は call_aws(MCP) ではなくローカル CLI で行う(MCP の elicit 同意が
#       クライアント未対応で却下されるため。既知の回避策)。
#
# 使い方:
#   bash deploy/redeploy.sh               # 通常デプロイ
#   bash deploy/redeploy.sh --keep-fw-open# 復元せず開けたまま(デバッグ用・手動で戻すこと)
set -euo pipefail

# ===== 設定(環境に合わせて変更可) =====
INSTANCE="go-iot-prod"
REGION="ap-northeast-1"
PROFILE="go-iot-mcp"
STATIC_IP="57.182.65.19"
FQDN="57.182.65.19.sslip.io"
SSH_USER="ubuntu"
REMOTE_DIR="/opt/go_iot"
APP_USER="go_iot"
SERVICE="go_iot"
ADMIN_CIDR="123.226.213.236/32"        # 復元先(管理元。動的なら適宜更新)
PEM="$HOME/.ssh/lightsail-goiot.pem"
KEEP_BACKUPS=3                          # サーバ上に残す旧バイナリ backup の世代数

KEEP_FW_OPEN=0
[ "${1:-}" = "--keep-fw-open" ] && KEEP_FW_OPEN=1

AWS="aws --profile $PROFILE --region $REGION"
log() { printf '\n\033[1;36m== %s ==\033[0m\n' "$*"; }
die() { printf '\033[1;31mERROR: %s\033[0m\n' "$*" >&2; exit 1; }

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

# ---- FW 操作ヘルパ (22 の cidrs だけ差し替え・80/443 anywhere・8080/5432 は常に非公開) ----
set_ssh_cidrs() { # $1 = JSON 配列要素(例: '"a/32","b/24"')
  $AWS lightsail put-instance-public-ports --instance-name "$INSTANCE" --port-infos \
"[{\"fromPort\":22,\"toPort\":22,\"protocol\":\"tcp\",\"cidrs\":[$1]},{\"fromPort\":80,\"toPort\":80,\"protocol\":\"tcp\",\"cidrs\":[\"0.0.0.0/0\"]},{\"fromPort\":443,\"toPort\":443,\"protocol\":\"tcp\",\"cidrs\":[\"0.0.0.0/0\"]}]" >/dev/null
}
show_ports() {
  $AWS lightsail get-instance-port-states --instance-name "$INSTANCE" \
    --query "portStates[].[fromPort,join(',',cidrs)]" --output json || true
}

# ---- 終了時に必ず FW を復元(開放した場合のみ) ----
FW_OPENED=0
restore_fw() {
  [ "$FW_OPENED" -eq 1 ] || return 0
  if [ "$KEEP_FW_OPEN" -eq 1 ]; then
    echo "(--keep-fw-open: FW 復元をスキップ。手動で 22 を $ADMIN_CIDR へ戻すこと)"; return 0
  fi
  log "FW 復元: 22 -> $ADMIN_CIDR のみ"
  if set_ssh_cidrs "\"$ADMIN_CIDR\""; then echo "FW 復元 OK"; else
    echo "⚠️ FW 復元失敗: 手動で 22 を $ADMIN_CIDR へ戻すこと"; fi
  show_ports
}
trap restore_fw EXIT

# ===== 1. ビルド(FW 開放前) =====
log "amd64 クロスビルド"
make sync-css
go tool templ generate >/dev/null
VER="$(git rev-parse --short HEAD)"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags="-s -w -X github.com/HiroshiKawano/go_iot/internal/view.Version=$VER" \
  -o go_iot_server ./cmd/server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" \
  -o go_iot_gen-token ./cmd/gen-token
file go_iot_server | grep -q "x86-64" || die "ビルド成果物が x86-64 でない(GOARCH 確認)"
LSHA="$(shasum -a 256 go_iot_server | awk '{print $1}')"
echo "Version=$VER  server sha256=$LSHA"

# ===== 2. egress 検出 =====
log "egress IP 検出(3 回サンプル)"
IPS="$(for _ in 1 2 3; do curl -s -m 10 https://checkip.amazonaws.com; done | sort -u)"
[ -n "$IPS" ] || die "egress IP を取得できません"
COUNT="$(printf '%s\n' "$IPS" | grep -c .)"
FIRST="$(printf '%s\n' "$IPS" | head -1)"
if [ "$COUNT" -eq 1 ]; then
  CLIENT_CIDR="$FIRST/32"; echo "固定 egress: $CLIENT_CIDR"
else
  CLIENT_CIDR="$(echo "$FIRST" | awk -F. '{print $1"."$2"."$3".0/24"}')"
  echo "変動 egress(VPN 等) -> /24 採用: $CLIENT_CIDR  (観測: $(echo "$IPS" | tr '\n' ' '))"
fi

# ===== 3. FW 一時開放 =====
log "FW 一時開放: 22 -> [$CLIENT_CIDR, $ADMIN_CIDR]"
set_ssh_cidrs "\"$CLIENT_CIDR\",\"$ADMIN_CIDR\""
FW_OPENED=1
show_ports

# ===== 4. SSH 鍵取得 + 到達待ち =====
log "一時 SSH 鍵 取得"
$AWS lightsail get-instance-access-details --instance-name "$INSTANCE" --output json \
  | python3 -c "import sys,json;d=json.load(sys.stdin)['accessDetails'];open('$PEM','w').write(d['privateKey']);open('$PEM-cert.pub','w').write(d['certKey'])"
chmod 600 "$PEM" "$PEM-cert.pub"

log "SSH(22) 到達待ち(最大 60s)"
REACH=0
for _ in $(seq 1 12); do
  if nc -z -G 5 "$STATIC_IP" 22 2>/dev/null; then REACH=1; break; fi
  sleep 5
done
[ "$REACH" -eq 1 ] || die "22 へ到達できません(FW 反映待ち / egress が想定外 / sshd 不調)"
SSH="ssh -i $PEM -o StrictHostKeyChecking=accept-new -o ConnectTimeout=20 $SSH_USER@$STATIC_IP"

# ===== 5. 配布 -> backup -> swap -> restart -> 検証(失敗時ロールバック) =====
log "scp 配布"
scp -i "$PEM" -o StrictHostKeyChecking=accept-new \
  go_iot_server go_iot_gen-token "$SSH_USER@$STATIC_IP:/home/$SSH_USER/"

log "サーバ側: backup -> swap -> restart -> 検証"
$SSH bash -s <<EOF
set -e
REMOTE_DIR='$REMOTE_DIR'; APP_USER='$APP_USER'; SERVICE='$SERVICE'
LSHA='$LSHA'; SSH_USER='$SSH_USER'; KEEP=$KEEP_BACKUPS
TS=\$(date +%Y%m%d-%H%M%S)
sudo cp -p "\$REMOTE_DIR/go_iot_server"    "\$REMOTE_DIR/go_iot_server.bak-\$TS"
sudo cp -p "\$REMOTE_DIR/go_iot_gen-token" "\$REMOTE_DIR/go_iot_gen-token.bak-\$TS"
sudo mv "/home/\$SSH_USER/go_iot_server" "/home/\$SSH_USER/go_iot_gen-token" "\$REMOTE_DIR/"
sudo chown "\$APP_USER:\$APP_USER" "\$REMOTE_DIR/go_iot_server" "\$REMOTE_DIR/go_iot_gen-token"
sudo chmod 755 "\$REMOTE_DIR/go_iot_server" "\$REMOTE_DIR/go_iot_gen-token"
RSHA=\$(sudo sha256sum "\$REMOTE_DIR/go_iot_server" | awk '{print \$1}')
echo "remote server sha256=\$RSHA"
[ "\$RSHA" = "\$LSHA" ] || { echo 'sha256 不一致(転送破損?)'; exit 90; }
sudo systemctl restart "\$SERVICE"; sleep 3
ACT=\$(systemctl is-active "\$SERVICE")
H=\$(curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/health)
echo "is-active=\$ACT  internal-health=\$H"
if [ "\$ACT" != active ] || [ "\$H" != 200 ]; then
  echo '⚠️ 起動/health 異常 -> 旧バイナリへロールバック'
  sudo cp -p "\$REMOTE_DIR/go_iot_server.bak-\$TS"    "\$REMOTE_DIR/go_iot_server"
  sudo cp -p "\$REMOTE_DIR/go_iot_gen-token.bak-\$TS" "\$REMOTE_DIR/go_iot_gen-token"
  sudo chown "\$APP_USER:\$APP_USER" "\$REMOTE_DIR/go_iot_server" "\$REMOTE_DIR/go_iot_gen-token"
  sudo systemctl restart "\$SERVICE"; sleep 3
  echo "rollback後 is-active=\$(systemctl is-active \$SERVICE) health=\$(curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/health)"
  exit 91
fi
# 古い backup を掃除(最新 \$KEEP 世代を保持)
ls -1t "\$REMOTE_DIR"/go_iot_server.bak-*    2>/dev/null | tail -n +\$((KEEP+1)) | xargs -r sudo rm -f
ls -1t "\$REMOTE_DIR"/go_iot_gen-token.bak-* 2>/dev/null | tail -n +\$((KEEP+1)) | xargs -r sudo rm -f
sudo journalctl -u "\$SERVICE" -n 5 --no-pager
EOF

# ===== 6. 外部 HTTPS 検証 =====
log "外部 HTTPS 検証(Caddy 経由)"
HC="$(curl -sS -m 15 -o /dev/null -w '%{http_code}' "https://$FQDN/health")"
echo "HTTPS /health=$HC"
VSEEN="$(curl -sS -m 15 "https://$FQDN/login" | grep -oE '\?v=[a-z0-9]+' | head -1 || true)"
echo "配信 Version=$VSEEN  (期待 ?v=$VER)"
[ "$HC" = 200 ] || die "外部 health 異常"

# ===== 7. ローカル成果物の掃除(FW 復元は trap で実行) =====
log "ローカル成果物 削除"
rm -f go_iot_server go_iot_gen-token

echo "✅ デプロイ完了 (commit $VER)"
