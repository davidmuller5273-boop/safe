#!/usr/bin/env bash
# Ubuntu 22.04/24.04 installer for Safe (SafeW bot + admin).
set -euo pipefail

APP_USER=safe
APP_DIR=/opt/safe
ENV_DIR=/etc/safe

if [[ "$(id -u)" -ne 0 ]]; then
  echo "请使用 root 运行: sudo bash scripts/install-ubuntu.sh"
  exit 1
fi

apt-get update
apt-get install -y ca-certificates curl nginx mysql-server redis-server
timedatectl set-timezone Asia/Shanghai || true

id -u "$APP_USER" >/dev/null 2>&1 || useradd --system --home "$APP_DIR" --shell /usr/sbin/nologin "$APP_USER"
mkdir -p "$APP_DIR/bin" "$APP_DIR/config" "$APP_DIR/web" "$ENV_DIR"
chown -R "$APP_USER:$APP_USER" "$APP_DIR"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

if [[ -x "$ROOT_DIR/bin/safe" ]]; then
  install -m 0755 "$ROOT_DIR/bin/safe" "$APP_DIR/bin/safe"
elif [[ -f "$ROOT_DIR/bin/safe" ]]; then
  install -m 0755 "$ROOT_DIR/bin/safe" "$APP_DIR/bin/safe"
else
  echo "未找到 bin/safe，请先在项目根执行: make build"
  exit 1
fi

if [[ ! -f "$APP_DIR/config/app.yml" ]]; then
  install -m 0640 "$ROOT_DIR/config/app.example.yml" "$APP_DIR/config/app.yml"
  chown "$APP_USER:$APP_USER" "$APP_DIR/config/app.yml"
fi

if [[ ! -f "$ENV_DIR/safe.env" ]]; then
  install -m 0640 "$ROOT_DIR/.env.example" "$ENV_DIR/safe.env"
  # Point CONFIG_FILE at installed yaml
  if ! grep -q '^CONFIG_FILE=' "$ENV_DIR/safe.env"; then
    echo "CONFIG_FILE=$APP_DIR/config/app.yml" >> "$ENV_DIR/safe.env"
  else
    sed -i "s|^CONFIG_FILE=.*|CONFIG_FILE=$APP_DIR/config/app.yml|" "$ENV_DIR/safe.env"
  fi
fi

install -m 0644 "$ROOT_DIR/deploy/systemd/"*.service /etc/systemd/system/
systemctl daemon-reload

echo
echo "安装完成。下一步："
echo "  1) 编辑 $APP_DIR/config/app.yml 与 $ENV_DIR/safe.env（设置 MYSQL_* JWT_SECRET DEVELOPER_USER_IDS）"
echo "  2) 创建 MySQL 库表用户后执行: sudo -u $APP_USER CONFIG_FILE=$APP_DIR/config/app.yml $APP_DIR/bin/safe setup"
echo "  3) 可选: 构建前端 chatadmin (npm install && npm run build) 并复制 dist 到 $APP_DIR/web"
echo "  4) systemctl enable --now safe-admin safe-lottery-collector safe-safew-bot"
echo "  5) 参考 deploy/nginx-safe.conf.example 配置 Nginx"
