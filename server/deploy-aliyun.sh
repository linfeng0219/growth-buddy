#!/usr/bin/env bash
set -euo pipefail

APP_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$APP_DIR"

echo "=== 成长小伙伴 · Go + SQLite 阿里云一键部署脚本 ==="
mkdir -p "$APP_DIR/data/releases"

# 1. 自动生成安全的 .env（若尚不存在；已有 .env 与 data/growth_buddy.db 完全保留无损迁移）
if [ ! -f "$APP_DIR/.env" ]; then
  SECRET=""
  if command -v openssl >/dev/null 2>&1; then
    SECRET="$(openssl rand -hex 32)"
  elif command -v python3 >/dev/null 2>&1; then
    SECRET="$(python3 -c 'import secrets; print(secrets.token_hex(32))')"
  else
    SECRET="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
  fi

  cat > "$APP_DIR/.env" <<EOF
PORT=3000
HOST=0.0.0.0
DB_PATH=./data/growth_buddy.db
RELEASES_DIR=./data/releases
SESSION_SECRET=${SECRET}
ADMIN_USERNAME=admin
ADMIN_PASSWORD=admin1234
EOF
  echo "[1/3] 已自动生成 .env（包含随机 64 位 SESSION_SECRET 与默认管理员 admin / admin1234）"
else
  echo "[1/3] 检测到已有 .env，保留现有配置与数据库"
fi

# 2. 优先使用 Docker Compose（镜像仅 ~85MB，1秒级构建，不依赖宿主机 Go/Node/glibc 版本）
if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  echo "[2/3] 检测到 Docker Compose，正在启动 Go + SQLite 服务..."
  docker compose up -d --build
  echo "[3/3] 等待服务就绪..."
  sleep 2
  if curl -sf http://127.0.0.1:3000/api/health; then
    echo ""
    echo "✅ Go + SQLite 服务部署成功！"
  else
    echo "⚠️ 容器已启动，请运行 docker compose logs -f 查看日志。"
  fi
  echo ""
  echo "👉 首页/家长端: http://你的域名/parent.html"
  echo "👉 系统管理端:  http://你的域名/admin.html（含「App 远程升级发布」，默认 admin / admin1234）"
  echo "💡 1Panel 网站反向代理请确认 client_max_body_size 至少设为 160M，以便直接上传 APK。"
  exit 0
fi

# 3. 备选：直接原生运行预编译单二进制 + systemd（零 Docker、零 Node 依赖）
chmod +x "$APP_DIR/growth-buddy-server"
if [ "$(id -u)" -eq 0 ] && command -v systemctl >/dev/null 2>&1; then
  SERVICE_FILE="/etc/systemd/system/growth-buddy.service"
  cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=Growth Buddy Server (Go + SQLite)
After=network.target

[Service]
Type=simple
WorkingDirectory=$APP_DIR
EnvironmentFile=$APP_DIR/.env
ExecStart=$APP_DIR/growth-buddy-server
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable --now growth-buddy
  systemctl restart growth-buddy
  sleep 1
  curl -sf http://127.0.0.1:3000/api/health && echo ""
  echo "✅ 已以原生 systemd 服务启动（growth-buddy）！"
else
  nohup "$APP_DIR/growth-buddy-server" > "$APP_DIR/server.log" 2>&1 &
  sleep 1
  curl -sf http://127.0.0.1:3000/api/health && echo ""
  echo "✅ 已在后台启动（日志：$APP_DIR/server.log）"
fi
