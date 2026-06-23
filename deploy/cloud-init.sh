#!/bin/bash
# go_iot 本番インスタンス 初回自動構成 (cloud-init launch script / Lightsail user-data)
# 実行: 初回ブート時に root で1回のみ (再実行不可)。ログ: /var/log/cloud-init-output.log
# 方針: 秘密値(DBパスワード/SESSION_SECRET)は平文で焼かない。
#   - DBパスワードはインスタンス内で生成し /root/go_iot_db_pass(600) に保管 (外に出さない)
#   - SESSION_SECRET と systemd EnvironmentFile は A-3 で人が安全注入する
# 注意: Lightsail は user-data を Lightsail 初期化スクリプトに連結し /bin/sh(dash) で実行する。
# そのため #!/bin/bash は無視されうる → bash 専用構文(set -o pipefail 等)を使わず POSIX sh 互換にする。
set -eu
export DEBIAN_FRONTEND=noninteractive
log() { echo "[go_iot cloud-init] $(date -Is) $*"; }
log "=== 開始 ==="

# 設定値 (秘密ではない。必要なら作成時に置換)
PG_VER=16
APP_DIR=/opt/go_iot
APP_USER=go_iot
SWAP_SIZE=2G              # Nano-0.5GB は 1G に下げる
SWAPPINESS=10

# ---------------------------------------------------------------------------
# 0. タイムゾーン (JST。ログと recorded_at(JST) の突き合わせのため)
# ---------------------------------------------------------------------------
if [ "$(cat /etc/timezone 2>/dev/null || true)" != "Asia/Tokyo" ]; then
  log "タイムゾーンを Asia/Tokyo に設定"
  timedatectl set-timezone Asia/Tokyo || ln -sf /usr/share/zoneinfo/Asia/Tokyo /etc/localtime
fi

# ---------------------------------------------------------------------------
# 1. swap (低メモリ機の OOM 回避・最優先・冪等)
# ---------------------------------------------------------------------------
if ! swapon --show | grep -q '/swapfile'; then
  log "swap ($SWAP_SIZE) を作成"
  fallocate -l "$SWAP_SIZE" /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=2048
  chmod 600 /swapfile
  mkswap /swapfile
  swapon /swapfile
fi
grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
# 普段は物理RAM優先 (ピーク時のみ swap)
printf 'vm.swappiness=%s\nvm.vfs_cache_pressure=50\n' "$SWAPPINESS" > /etc/sysctl.d/99-swap.conf
sysctl --system >/dev/null

# ---------------------------------------------------------------------------
# 2. パッケージ更新 + 共通ツール
# ---------------------------------------------------------------------------
log "apt update / 共通パッケージ"
apt-get update -y
apt-get install -y curl ca-certificates gnupg lsb-release debian-keyring debian-archive-keyring apt-transport-https

# ---------------------------------------------------------------------------
# 3. SSH ハードニング (パスワード認証無効 / root SSH 無効 / 公開鍵のみ)
#    Lightsail 既定鍵(authorized_keys)は cloud-init が配置済み。鍵ログインを切らない。
#    drop-in を 99- で後勝ちにし、cloud-init の 50-cloud-init.conf を上書き。
#    注意: Ubuntu 22.04.2+/24.04 は ssh が socket activation (ssh.socket) の場合があり、
#          ssh.service の restart だけでは設定が再読込されないことがある。両系統を扱う。
# ---------------------------------------------------------------------------
log "SSH ハードニング"
cat > /etc/ssh/sshd_config.d/99-hardening.conf <<'SSHD'
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin no
PubkeyAuthentication yes
SSHD
if sshd -t; then
  systemctl daemon-reload
  # socket activation 環境では ssh.socket を、従来型では ssh(.service)/sshd を再起動
  if systemctl is-active --quiet ssh.socket; then
    systemctl restart ssh.socket
  fi
  systemctl restart ssh 2>/dev/null || systemctl restart sshd 2>/dev/null || true
else
  log "WARN: sshd -t 失敗。設定を反映せず継続 (締め出し回避)"
fi

# ---------------------------------------------------------------------------
# 4. PostgreSQL 16 (native apt / PGDG) + 低メモリチューニング + localhost bind
# ---------------------------------------------------------------------------
if ! command -v psql >/dev/null 2>&1; then
  log "PGDG リポジトリ + postgresql-${PG_VER} 導入"
  install -d /usr/share/postgresql-common/pgdg
  curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc \
    -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc
  echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] \
https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" \
    > /etc/apt/sources.list.d/pgdg.list
  apt-get update -y
  apt-get install -y "postgresql-${PG_VER}"
fi

PG_CONF="/etc/postgresql/${PG_VER}/main/postgresql.conf"   # Debian/Ubuntu 配置 (要確認)
PG_HBA="/etc/postgresql/${PG_VER}/main/pg_hba.conf"

# 4-1. localhost のみ待受 (5432 を外部 bind しない。Debian 既定も localhost だが明示固定)
log "PostgreSQL を localhost 待受に固定 + 低メモリチューニング"
sed -i "s/^#\?listen_addresses\s*=.*/listen_addresses = 'localhost'/" "$PG_CONF"
sed -i "s/^#\?password_encryption\s*=.*/password_encryption = scram-sha-256/" "$PG_CONF"

# 4-2. 低メモリチューニング (RAM 1GB級・DBとapp同居前提。値は要調整)
#      既存同名行と衝突しないよう、専用ブロックを冪等に管理
if ! grep -q '# --- go_iot tuning ---' "$PG_CONF"; then
  cat >> "$PG_CONF" <<'PGCONF'

# --- go_iot tuning --- (RAM 1GB級・DBとapp同居前提 / 値は要調整)
shared_buffers = 128MB
effective_cache_size = 512MB
work_mem = 8MB
maintenance_work_mem = 64MB
max_connections = 20
PGCONF
fi

# 4-3. pg_hba を local / ループバックのみ (外部 host 行を作らない)
#      Debian 既定はほぼこの形。scram-sha-256 に統一。
sed -i 's/^\(host\s\+all\s\+all\s\+127\.0\.0\.1\/32\s\+\).*/\1scram-sha-256/' "$PG_HBA" || true
sed -i 's/^\(host\s\+all\s\+all\s\+::1\/128\s\+\).*/\1scram-sha-256/' "$PG_HBA" || true

systemctl enable postgresql
systemctl restart postgresql

# 4-4. 本番ロール go_iot / DB go_iot を作成 (冪等)
#      DBパスワードはインスタンス内で生成し /root/go_iot_db_pass(600) に保管 (平文を焼かない)。
#      A-3 で systemd EnvironmentFile の DATABASE_URL を組み立てる際にこのファイルを参照する。
PASS_FILE=/root/go_iot_db_pass
if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='go_iot'" | grep -q 1; then
  log "本番ロール go_iot を生成パスワードで作成 (パスワードは $PASS_FILE 600 に保管)"
  # URLエンコード事故を避けるため記号を除いた32文字
  DB_PASS="$(openssl rand -base64 24 | tr -d '/+=' | cut -c1-32)"
  ( umask 077; printf '%s' "$DB_PASS" > "$PASS_FILE" )  # umask 077 をサブシェルに閉じ込め後続(Caddy鍵644等)へ漏らさない
  chmod 600 "$PASS_FILE"
  # psql の :'pass' 変数展開は -c では効かない(リテラル送出される)ため、
  # stdin スクリプトモードで \set + :'pass' を使い安全にクオートする。
  sudo -u postgres psql -v ON_ERROR_STOP=1 <<SQL
\set pass '$DB_PASS'
CREATE ROLE go_iot WITH LOGIN PASSWORD :'pass';
SQL
  unset DB_PASS
fi
sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='go_iot'" | grep -q 1 \
  || sudo -u postgres createdb -O go_iot go_iot

# ---------------------------------------------------------------------------
# 5. アプリ配置先 + 専用システムユーザ + 環境ファイルの枠 (秘密は焼かない)
# ---------------------------------------------------------------------------
log "配置先 $APP_DIR と専用ユーザ $APP_USER を用意"
id "$APP_USER" >/dev/null 2>&1 || useradd --system --no-create-home --shell /usr/sbin/nologin "$APP_USER"
install -d -m 750 -o "$APP_USER" -g "$APP_USER" "$APP_DIR"
# 環境ファイルは「枠」だけ作る。SESSION_SECRET と DATABASE_URL は A-3 で人が安全注入。
if [ ! -f "$APP_DIR/.env" ]; then
  cat > "$APP_DIR/.env" <<'ENVTPL'
APP_ENV=production
APP_PORT=8080
# DATABASE_URL=postgres://go_iot:<URLエンコード済みパスワード>@localhost:5432/go_iot?sslmode=disable
# SESSION_SECRET=<32文字以上のランダム値>   # A-3 で安全注入 (平文を焼かない)
ENVTPL
  chown "$APP_USER":"$APP_USER" "$APP_DIR/.env"
  chmod 600 "$APP_DIR/.env"
fi

# ---------------------------------------------------------------------------
# 6. Caddy (公式 apt) + Caddyfile 雛形
#    DNS 伝播前に Let's Encrypt 取得が暴発しないよう、雛形は :80 (HTTP・自動TLS要求なし) にする。
#    ドメイン確定後 (A-4) に本番ドメインへ差し替えて自動TLSを有効化する。
# ---------------------------------------------------------------------------
if ! command -v caddy >/dev/null 2>&1; then
  log "Caddy 導入 (公式 apt)"
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
    | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
  chmod 644 /usr/share/keyrings/caddy-stable-archive-keyring.gpg   # _apt が鍵を読めるよう644 (NO_PUBKEY回避・umask漏れ対策)
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
    > /etc/apt/sources.list.d/caddy-stable.list
  chmod 644 /etc/apt/sources.list.d/caddy-stable.list
  apt-get update -y
  apt-get install -y caddy
fi
# 雛形: 到達確認用に :80 (HTTP) → 127.0.0.1:8080。本番ドメイン+自動TLSは A-4 で差し替え。
if ! grep -q 'go_iot placeholder' /etc/caddy/Caddyfile 2>/dev/null; then
  cp -n /etc/caddy/Caddyfile /etc/caddy/Caddyfile.bak 2>/dev/null || true
  cat > /etc/caddy/Caddyfile <<'CADDY'
# go_iot placeholder (A-4 で本番ドメイン + email に差し替えて自動TLSを有効化する)
# 例:
#   {
#       email <ADMIN_EMAIL>
#   }
#   <DOMAIN> {
#       reverse_proxy localhost:8080
#   }
:80 {
    reverse_proxy localhost:8080
}
CADDY
  caddy fmt --overwrite /etc/caddy/Caddyfile || true
fi
systemctl enable caddy
systemctl restart caddy || true   # アプリ未起動でも Caddy 自体は上がる

log "=== 完了 (アプリ本体/SESSION_SECRET/DATABASE_URL/本番ドメインTLSは A-3/A-4 で) ==="
