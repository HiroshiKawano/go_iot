#!/usr/bin/env bash
# redeploy.sh — go_iot 本番(AWS Lightsail: go-iot-prod)へのオンデマンド・リデプロイ自動化
#
# 背景: SSH 接続元(egress)が VPN で接続ごとに変動し、管理元 IP 自体も動的なため、
#       SSH(22) を単一 /32 に固定する運用は破綻する。本スクリプトは毎回その場で
#       現在の egress を検出して 22 を一時開放し、作業後に必ず管理元 /32 へ復元する。
#
# 流れ:
#   0. デプロイ元ガード (main・clean・origin/main 一致を強制。本番は常に main を反映)
#   1. amd64 クロスビルド (sync-css -> templ generate -> go build)   ※FW開放前に済ませ開放時間を最小化
#   2. 現在の egress IP を検出 (3回サンプル: 全一致=/32 / 変動=/24)
#   3. Lightsail FW の 22 を [検出CIDR, 管理元CIDR] へ一時開放 (80/443 据置・8080/5432 非公開維持)
#   4. 一時 SSH 鍵を取得し 22 到達を待機
#   4.5 DB マイグレーション: 未適用があれば★入替の前に★ goose up (トンネル経由・後方互換 migration 前提)
#   5. scp -> 旧バイナリ backup -> swap -> systemd restart -> 検証 (失敗時は旧バイナリへ自動ロールバック)
#   6. 外部 HTTPS /health と配信 Version を検証
#   7. ★trap EXIT で必ず★ goose トンネル終了 + FW を管理元 /32 のみへ復元 (途中失敗・中断でも実行)
#
# 前提: aws CLI v2 + プロファイル go-iot-mcp / git / go (go tool goose) / python3 / nc。
#       FW 変更は call_aws(MCP) ではなくローカル CLI で行う(MCP の elicit 同意が
#       クライアント未対応で却下されるため。既知の回避策)。
#       migration は「追加専用＝後方互換(expand-contract)」であること(STEP 4.5 の不変条件・本文参照)。
#
# 使い方:
#   bash deploy/redeploy.sh                 # 通常デプロイ(main 限定・未適用 migration は自動 goose up)
#   bash deploy/redeploy.sh --keep-fw-open  # 復元せず FW を開けたまま(デバッグ用・手動で戻すこと)
#   bash deploy/redeploy.sh --allow-non-main# main 限定ガードを迂回(緊急ホットフィックス専用・後で main へ取込)
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
DBTUNNEL_PORT=15432                     # goose 用の一時 SSH トンネル(ローカル)ポート

# ---- 引数 ----
KEEP_FW_OPEN=0          # --keep-fw-open  : 終了後も FW を開けたまま(デバッグ用・手動で戻すこと)
ALLOW_NON_MAIN=0        # --allow-non-main: main 以外/未同期からのデプロイを許可(緊急ホットフィックス専用)
for arg in "$@"; do
  case "$arg" in
    --keep-fw-open)   KEEP_FW_OPEN=1 ;;
    --allow-non-main) ALLOW_NON_MAIN=1 ;;
    *) printf 'ERROR: 不明な引数: %s (使用可: --keep-fw-open / --allow-non-main)\n' "$arg" >&2; exit 2 ;;
  esac
done

AWS="aws --profile $PROFILE --region $REGION"
log() { printf '\n\033[1;36m== %s ==\033[0m\n' "$*"; }
die() { printf '\033[1;31mERROR: %s\033[0m\n' "$*" >&2; exit 1; }

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

# ===== 0. デプロイ元ガード(本番は常に main のレビュー済みコードを反映する) =====
# 作業ブランチ直デプロイ・未コミット・origin/main 未同期を禁止し、配信 Version と本番を一致させる。
# (過去事例: feature ブランチを直接デプロイ→main 未マージで「本番が未マージ commit を指す」状態になった)
# 緊急ホットフィックス時のみ --allow-non-main で迂回(その場合も後で必ず main へ取り込む)。
if [ "$ALLOW_NON_MAIN" -eq 0 ]; then
  BR="$(git rev-parse --abbrev-ref HEAD)"
  [ "$BR" = main ] || die "デプロイ元が main でない(現在: $BR)。main へマージしてから実行(緊急時は --allow-non-main)"
  [ -z "$(git status --porcelain)" ] || die "作業ツリーに未コミット変更あり。commit してから実行(本番=コミット済み main のみ)"
  git fetch -q origin main || die "git fetch origin main 失敗"
  [ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || die "ローカル main が origin/main と不一致。pull/push で同期してから実行"
  echo "デプロイ元ガード OK: main・clean・origin/main と一致"
else
  echo "⚠️ --allow-non-main: main 限定ガードを迂回(緊急モード)。後で必ず main へ取り込むこと"
fi

# ---- FW 操作ヘルパ (22 の cidrs だけ差し替え・80/443 anywhere・8080/5432 は常に非公開) ----
set_ssh_cidrs() { # $1 = JSON 配列要素(例: '"a/32","b/24"')
  $AWS lightsail put-instance-public-ports --instance-name "$INSTANCE" --port-infos \
"[{\"fromPort\":22,\"toPort\":22,\"protocol\":\"tcp\",\"cidrs\":[$1]},{\"fromPort\":80,\"toPort\":80,\"protocol\":\"tcp\",\"cidrs\":[\"0.0.0.0/0\"]},{\"fromPort\":443,\"toPort\":443,\"protocol\":\"tcp\",\"cidrs\":[\"0.0.0.0/0\"]}]" >/dev/null
}
show_ports() {
  $AWS lightsail get-instance-port-states --instance-name "$INSTANCE" \
    --query "portStates[].[fromPort,join(',',cidrs)]" --output json || true
}

# ---- 終了時に必ず後始末(goose 用 SSH トンネル終了 + FW 復元) ----
FW_OPENED=0
TUNNEL_PID=""
cleanup() {
  if [ -n "$TUNNEL_PID" ]; then kill "$TUNNEL_PID" 2>/dev/null || true; echo "(goose トンネル終了)"; TUNNEL_PID=""; fi
  [ "$FW_OPENED" -eq 1 ] || return 0
  if [ "$KEEP_FW_OPEN" -eq 1 ]; then
    echo "(--keep-fw-open: FW 復元をスキップ。手動で 22 を $ADMIN_CIDR へ戻すこと)"; return 0
  fi
  log "FW 復元: 22 -> $ADMIN_CIDR のみ"
  if set_ssh_cidrs "\"$ADMIN_CIDR\""; then echo "FW 復元 OK"; else
    echo "⚠️ FW 復元失敗: 手動で 22 を $ADMIN_CIDR へ戻すこと"; fi
  show_ports
}
trap cleanup EXIT

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

# ===== 4.5 DB マイグレーション(★バイナリ入替の「前」に適用★) =====
# 重要(不変条件): 本ステップは「追加専用＝後方互換(expand-contract)」の migration を前提とする。
#   入替前に goose up するため、適用後〜入替までの数秒は【旧バイナリ × 新スキーマ】で動く。
#   ADD COLUMN(nullable) / ADD INDEX / ADD TABLE 等は旧バイナリに無害。一方 DROP/RENAME/型変更/
#   NOT NULL 追加 等の破壊的変更は旧バイナリを壊すため、その種は expand-contract で複数回に分けること。
#   未適用が無ければ no-op(冪等)。goose 失敗時はバイナリを入替えずに中止(prod は旧コード×直前スキーマで継続)。
#   DB は localhost 5432(外部非公開)のため SSH トンネル(ローカル DBTUNNEL_PORT)越しに適用する。
log "DB マイグレーション確認(goose・SSH トンネル経由)"
DBPASS="$($SSH 'sudo cat /root/go_iot_db_pass')"
[ -n "$DBPASS" ] || die "DB パスワード取得失敗(/root/go_iot_db_pass)"
ssh -fN -i "$PEM" -o StrictHostKeyChecking=accept-new -o ExitOnForwardFailure=yes \
  -L "$DBTUNNEL_PORT:localhost:5432" "$SSH_USER@$STATIC_IP"
TUNNEL_PID="$(pgrep -f "ssh -fN.*$DBTUNNEL_PORT:localhost:5432" | head -1)"
sleep 2
export GOOSE_DRIVER=postgres
export GOOSE_DBSTRING="postgres://$APP_USER:${DBPASS}@localhost:${DBTUNNEL_PORT}/go_iot?sslmode=disable"
STATUS="$(go tool goose -dir db/migrations status 2>&1)" || die "goose status 失敗(DB接続/トンネル確認): $STATUS"
echo "$STATUS"
if printf '%s' "$STATUS" | grep -q "Pending"; then
  echo "未適用 migration あり -> goose up(入替の前に適用)"
  go tool goose -dir db/migrations up || die "goose up 失敗 -> デプロイ中止(バイナリ未入替・prod 継続)"
else
  echo "未適用 migration なし(goose 最新・no-op)"
fi
kill "$TUNNEL_PID" 2>/dev/null || true; echo "(goose トンネル終了)"; TUNNEL_PID=""

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
