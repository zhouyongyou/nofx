#!/usr/bin/env bash
set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  echo "需要使用 sudo 运行：sudo bash scripts/deploy-ubuntu.sh"
  exit 1
fi

APP=nofx
INSTALL_DIR=/opt/nofx
FRONT_ROOT=/var/www/nofx
ENV_FILE=$INSTALL_DIR/.env
SERVICE_FILE=/etc/systemd/system/nofx.service
SITE_FILE=/etc/nginx/sites-available/nofx.conf

export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get install -y curl git rsync nginx openssl build-essential ca-certificates

GO_VER=1.25.0
if ! command -v go >/dev/null 2>&1 || ! go version | grep -q "go$GO_VER"; then
  curl -fsSL https://go.dev/dl/go${GO_VER}.linux-amd64.tar.gz -o /tmp/go.tar.gz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tar.gz
  echo 'export PATH=/usr/local/go/bin:$PATH' > /etc/profile.d/go.sh
  chmod 644 /etc/profile.d/go.sh
  export PATH=/usr/local/go/bin:$PATH
fi

if ! command -v node >/dev/null 2>&1; then
  curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
  apt-get install -y nodejs
fi

mkdir -p "$INSTALL_DIR/bin"
rsync -a --delete --exclude ".git" --exclude "node_modules" --exclude "web/node_modules" . "$INSTALL_DIR/"

cd "$INSTALL_DIR"
mkdir -p secrets
if [ ! -f secrets/rsa_key ]; then
  openssl genrsa -out secrets/rsa_key 2048
  chmod 600 secrets/rsa_key
fi
if [ ! -f secrets/rsa_key.pub ]; then
  openssl rsa -in secrets/rsa_key -pubout -out secrets/rsa_key.pub
  chmod 644 secrets/rsa_key.pub
fi

if [ -f config.json.example ] && [ ! -f config.json ]; then
  cp config.json.example config.json
fi
if [ -f .env.example ] && [ ! -f "$ENV_FILE" ]; then
  cp .env.example "$ENV_FILE"
fi

DATA_KEY=$(openssl rand -base64 32)
JWT_KEY=$(openssl rand -base64 64)
sed -i "s|DATA_ENCRYPTION_KEY=.*|DATA_ENCRYPTION_KEY=$DATA_KEY|" "$ENV_FILE"
sed -i "s|# JWT_SECRET=|JWT_SECRET=$JWT_KEY|" "$ENV_FILE"
chmod 600 "$ENV_FILE"

if [ ! -f config.db ]; then
  install -m 600 /dev/null config.db
fi

/usr/local/go/bin/go build -o "$INSTALL_DIR/bin/$APP" ./main.go

if ! id -u "$APP" >/dev/null 2>&1; then
  useradd -r -s /usr/sbin/nologin -d "$INSTALL_DIR" "$APP"
fi
chown -R "$APP":"$APP" "$INSTALL_DIR"

cat > "$SERVICE_FILE" <<'EOF'
[Unit]
Description=NOFX Backend Service
After=network.target

[Service]
Type=simple
User=nofx
WorkingDirectory=/opt/nofx
EnvironmentFile=/opt/nofx/.env
ExecStart=/opt/nofx/bin/nofx
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now nofx

cd "$INSTALL_DIR/web"
npm ci
npm run build
mkdir -p "$FRONT_ROOT"
rsync -a --delete dist/ "$FRONT_ROOT/"

cat > "$SITE_FILE" <<'EOF'
server {
    listen 80;
    server_name _;
    root /var/www/nofx;
    index index.html;
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/x-javascript application/javascript application/json application/xml+rss;
    location / { try_files $uri $uri/ /index.html; }
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
    location /api/ {
        proxy_pass http://127.0.0.1:8080/api/;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_connect_timeout 300s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;
    }
    location /health { return 200 "OK\n"; add_header Content-Type text/plain; access_log off; }
}
EOF

ln -sf "$SITE_FILE" /etc/nginx/sites-enabled/nofx.conf
if [ -f /etc/nginx/sites-enabled/default ]; then
  rm -f /etc/nginx/sites-enabled/default
fi
nginx -t
systemctl restart nginx

if command -v ufw >/dev/null 2>&1; then
  ufw allow 80/tcp || true
fi

sleep 1
curl -sf http://127.0.0.1/api/health >/dev/null || echo "后端健康检查失败：请查看 nofx 服务日志"
curl -sf http://127.0.0.1/health >/dev/null || echo "前端健康检查失败：请查看 Nginx 日志"

echo "部署完成：前端 http://服务器IP/  后端 http://服务器IP/api/"