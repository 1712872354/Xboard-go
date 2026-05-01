#!/bin/bash
# =============================================================
#  Xboard-Go 一键部署脚本
#  支持两种运行方式：
#    1. 独立运行（自动下载）:
#       bash <(curl -fsSL https://github.com/1712872354/Xboard-go/releases/latest/download/install.sh)
#       bash <(curl -fsSL ...) v2.0.2  # 指定版本
#    2. 解压后运行（本地文件）:
#       tar -xzf xboard-*.tar.gz && cd xboard-*/ && chmod +x install.sh && ./install.sh
# =============================================================

set -e

# ---- 配色 ----
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; CYAN='\033[0;36m'; NC='\033[0m'
ok()  { echo -e "  ${GREEN}✓${NC} $1"; }
warn() { echo -e "  ${YELLOW}⚠${NC} $1"; }
err() { echo -e "  ${RED}✗${NC} $1"; }
step() { echo -e "\n${CYAN}━━━ [$1/6] $2 ━━━${NC}"; }
info() { echo -e "  ${BLUE}→${NC} $1"; }

# ---- 配置 ----
REPO="1712872354/Xboard-go"
RELEASES_URL="https://github.com/$REPO/releases"
API_URL="https://api.github.com/repos/$REPO/releases"

# ---- 参数: 版本号（可选），默认最新 ----
VERSION="${1:-latest}"

# =============================================================
# 检测工具是否可用
# =============================================================
check_tool() {
  if ! command -v "$1" &>/dev/null; then
    err "缺少 $1，请先安装"
    exit 1
  fi
}

# =============================================================
# Step 1: 检测环境
# =============================================================
step 1 "检测运行环境"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) err "不支持的架构: $ARCH"; exit 1 ;;
esac

if [ "$OS" != "linux" ] && [ "$OS" != "darwin" ]; then
  err "本脚本仅支持 Linux / macOS，当前系统: $OS"
  info "Windows 请使用 install.ps1"
  exit 1
fi
ok "系统: $OS ($ARCH)"

check_tool curl
check_tool tar

if ! command -v mysql &>/dev/null; then
  warn "未检测到 MySQL 客户端，如使用 SQLite 可忽略"
fi
ok "环境检测通过"

# =============================================================
# Step 2: 获取发布包
# =============================================================
step 2 "获取发布包"

LOCAL_MODE=false
INSTALL_DIR=$(cd "$(dirname "$0")" 2>/dev/null && pwd || echo "")

if [ -n "$INSTALL_DIR" ] && [ -f "$INSTALL_DIR/xboard" ] && [ -f "$INSTALL_DIR/xboard-scheduler" ]; then
  LOCAL_MODE=true
  WORKDIR="$INSTALL_DIR"
  ok "检测到本地文件，跳过下载"
else
  WORKDIR="/opt/xboard"

  if [ "$VERSION" = "latest" ]; then
    info "正在获取最新版本信息..."
    HTTP_CODE=$(curl -fsSL -o /dev/null -w "%{http_code}" "$API_URL/latest" 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
      DOWNLOAD_URL=$(curl -fsSL "$API_URL/latest" | grep -oP '"browser_download_url":\s*"\K[^"]+' | grep "${OS}-${ARCH}\.tar\.gz$" | head -1)
    fi
    # latest 不可用（如 pre-release），回退到取第一个 Release
    if [ -z "$DOWNLOAD_URL" ]; then
      info "尝试获取所有 Release 列表..."
      DOWNLOAD_URL=$(curl -fsSL "$API_URL?per_page=1" | grep -oP '"browser_download_url":\s*"\K[^"]+' | grep "${OS}-${ARCH}\.tar\.gz$" | head -1)
    fi
  else
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/xboard-${VERSION}-${OS}-${ARCH}.tar.gz"
    # 预检版本是否存在
    HTTP_CODE=$(curl -fsSL -o /dev/null -w "%{http_code}" "$DOWNLOAD_URL" 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "404" ] || [ "$HTTP_CODE" = "000" ]; then
      err "版本 $VERSION 不存在或尚无该平台的发布包"
      info "请前往 $RELEASES_URL 查看可用的版本和平台"
      exit 1
    fi
  fi

  if [ -z "$DOWNLOAD_URL" ]; then
    err "未找到 $OS-$ARCH 架构的发布包"
    info "请前往 $RELEASES_URL 手动下载"
    exit 1
  fi

  FILENAME=$(basename "$DOWNLOAD_URL")
  info "下载: $DOWNLOAD_URL"

  sudo mkdir -p "$WORKDIR"
  sudo curl -fsSL -o "/tmp/$FILENAME" "$DOWNLOAD_URL" --progress-bar || {
    err "下载失败，请检查网络或版本号是否正确"
    exit 1
  }
  ok "下载完成 ($(du -h "/tmp/$FILENAME" | cut -f1))"

  sudo tar -xzf "/tmp/$FILENAME" -C "$WORKDIR" --strip-components=1
  rm -f "/tmp/$FILENAME"
  ok "解压到 $WORKDIR"
fi

cd "$WORKDIR"
chmod +x xboard xboard-scheduler 2>/dev/null || true

# =============================================================
# Step 3: 配置文件
# =============================================================
step 3 "配置文件"

if [ ! -f config.yaml ]; then
  if [ -f config.example.yaml ]; then
    cp config.example.yaml config.yaml
  else
    err "缺少 config.example.yaml，无法生成配置文件"
    exit 1
  fi
fi

echo "  请配置以下关键项（直接回车使用默认值）:"
echo ""

# app.key
DEFAULT_KEY=$(tr -dc 'a-zA-Z0-9!@#$%^&*()_+-=' < /dev/urandom | head -c 32)
read -rp "  app.key [自动生成]: " APP_KEY
APP_KEY=${APP_KEY:-$DEFAULT_KEY}
sed -i "s/key: \"your-app-key-here\"/key: \"$APP_KEY\"/" config.yaml
info "app.key 已设置"

# app.url
DEFAULT_URL="http://$(hostname -I 2>/dev/null | awk '{print $1}'):8080"
[ -z "$DEFAULT_URL" ] && DEFAULT_URL="http://localhost:8080"
read -rp "  app.url [$DEFAULT_URL]: " APP_URL
APP_URL=${APP_URL:-$DEFAULT_URL}
sed -i "s|url: \"http://localhost:8080\"|url: \"$APP_URL\"|" config.yaml
info "app.url 已设置"

# database.driver
read -rp "  数据库类型 (mysql/sqlite) [mysql]: " DB_DRIVER
DB_DRIVER=${DB_DRIVER:-mysql}
sed -i "s/driver: mysql/driver: $DB_DRIVER/" config.yaml

if [ "$DB_DRIVER" = "mysql" ]; then
  read -rp "  数据库主机 [127.0.0.1]: " DB_HOST
  DB_HOST=${DB_HOST:-127.0.0.1}
  sed -i "s/host: 127.0.0.1/host: $DB_HOST/" config.yaml

  read -rp "  数据库端口 [3306]: " DB_PORT
  DB_PORT=${DB_PORT:-3306}
  sed -i "s/port: 3306/port: $DB_PORT/" config.yaml

  read -rp "  数据库名 [xboard]: " DB_NAME
  DB_NAME=${DB_NAME:-xboard}
  sed -i "s/dbname: xboard/dbname: $DB_NAME/" config.yaml

  read -rp "  数据库用户 [root]: " DB_USER
  DB_USER=${DB_USER:-root}
  sed -i "s/username: root/username: $DB_USER/" config.yaml

  read -rsp "  数据库密码: " DB_PASS
  echo ""
  sed -i "s/password: \"\"/password: \"$DB_PASS\"/" config.yaml
fi
info "database 已配置"

# redis
read -rp "  Redis 主机 [127.0.0.1]: " REDIS_HOST
REDIS_HOST=${REDIS_HOST:-127.0.0.1}
sed -i "s/host: 127.0.0.1/host: $REDIS_HOST/" config.yaml

read -rp "  Redis 端口 [6379]: " REDIS_PORT
REDIS_PORT=${REDIS_PORT:-6379}
sed -i "s/port: 6379/port: $REDIS_PORT/" config.yaml

read -rsp "  Redis 密码（无密码直接回车）: " REDIS_PASS
echo ""
if [ -n "$REDIS_PASS" ]; then
  sed -i "s/password: \"\"/password: \"$REDIS_PASS\"/" config.yaml
fi
info "redis 已配置"

ok "配置文件已完成"

# =============================================================
# Step 4: 数据库迁移
# =============================================================
step 4 "数据库迁移"

if ./xboard --migrate --config config.yaml; then
  ok "数据库迁移完成"
else
  err "迁移失败，请检查数据库配置"
  info "查看详细错误: ./xboard --migrate --config config.yaml"
  exit 1
fi

# =============================================================
# Step 5: 创建管理员
# =============================================================
step 5 "管理员账号"

read -rp "  是否创建默认管理员？[Y/n]: " SEED
SEED=${SEED:-Y}
if [ "$SEED" = "Y" ] || [ "$SEED" = "y" ]; then
  if ./xboard --seed --config config.yaml; then
    echo ""
    ok "管理员创建成功（请记录上方输出的邮箱和密码）"
  else
    warn "管理员创建失败（可能已存在，可忽略）"
  fi
else
  ok "跳过"
fi

# =============================================================
# Step 6: 启动服务
# =============================================================
step 6 "启动服务"

SYSCTL_AVAILABLE=false
command -v systemctl &>/dev/null && SYSCTL_AVAILABLE=true

if $SYSCTL_AVAILABLE; then
  echo "  请选择启动方式:"
  echo "    1) Systemd 服务（生产推荐，开机自启）"
  echo "    2) 前台启动（测试用）"
  echo "    3) 跳过，稍后手动启动"
  read -rp "  请输入 [1/2/3]: " START_MODE
else
  echo "  请选择启动方式:"
  echo "    1) 后台 nohup 启动"
  echo "    2) 前台启动（测试用）"
  echo "    3) 跳过，稍后手动启动"
  read -rp "  请输入 [1/2/3]: " START_MODE
fi

case "${START_MODE:-1}" in
  1)
    if $SYSCTL_AVAILABLE; then
      cat > /tmp/xboard.service << EOF
[Unit]
Description=Xboard API Service
After=network.target

[Service]
Type=simple
WorkingDirectory=$WORKDIR
ExecStart=$WORKDIR/xboard --config $WORKDIR/config.yaml
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

      cat > /tmp/xboard-scheduler.service << EOF
[Unit]
Description=Xboard Scheduler
After=network.target

[Service]
Type=simple
WorkingDirectory=$WORKDIR
ExecStart=$WORKDIR/xboard-scheduler --config $WORKDIR/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

      sudo cp /tmp/xboard.service /etc/systemd/system/xboard.service
      sudo cp /tmp/xboard-scheduler.service /etc/systemd/system/xboard-scheduler.service
      sudo systemctl daemon-reload
      sudo systemctl enable xboard xboard-scheduler
      sudo systemctl restart xboard xboard-scheduler

      sleep 2
      if sudo systemctl is-active --quiet xboard; then
        ok "API 服务 (Systemd) 已启动"
      else
        err "API 服务启动失败，查看日志: journalctl -u xboard -f"
      fi
      if sudo systemctl is-active --quiet xboard-scheduler; then
        ok "调度器 (Systemd) 已启动"
      else
        err "调度器启动失败，查看日志: journalctl -u xboard-scheduler -f"
      fi
    else
      nohup ./xboard --config config.yaml > xboard.log 2>&1 &
      XB_PID=$!
      nohup ./xboard-scheduler --config config.yaml > scheduler.log 2>&1 &
      SC_PID=$!
      sleep 2
      if kill -0 "$XB_PID" 2>/dev/null; then
        ok "API 服务已启动 (PID: $XB_PID)"
      else
        err "API 服务启动失败，查看 xboard.log"
      fi
      if kill -0 "$SC_PID" 2>/dev/null; then
        ok "调度器已启动 (PID: $SC_PID)"
      else
        err "调度器启动失败，查看 scheduler.log"
      fi
    fi
    ;;
  2)
    info "前台启动（按 Ctrl+C 停止）"
    echo ""
    echo "  API 服务:  ./xboard --config config.yaml"
    echo "  调度器:    ./xboard-scheduler --config config.yaml（新开终端）"
    echo ""
    ./xboard --config config.yaml
    ;;
  *)
    ok "跳过启动"
    ;;
esac

# =============================================================
# 完成
# =============================================================
echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}   Xboard-Go 部署完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

HOST_IP=$(ip route get 1 2>/dev/null | awk '{print $7;exit}' || hostname -I 2>/dev/null | awk '{print $1}' || echo "localhost")
PORT=$(grep '^\s*port:' config.yaml 2>/dev/null | head -1 | awk '{print $2}')
PORT=${PORT:-8080}
ADMIN_PATH=$(grep 'secure_path:' config.yaml 2>/dev/null | awk '{print $2}' | tr -d '"')
ADMIN_PATH=${ADMIN_PATH:-/admin}

echo "   访问地址:  ${CYAN}http://$HOST_IP:$PORT${NC}"
echo "   管理后台:  ${CYAN}http://$HOST_IP:$PORT$ADMIN_PATH${NC}"
echo ""
echo "   部署目录:  $WORKDIR"
echo "   配置文件:  $WORKDIR/config.yaml"
echo ""

if $SYSCTL_AVAILABLE && [ "${START_MODE:-1}" = "1" ]; then
  echo "   进程管理:"
  echo "     ${BLUE}systemctl status xboard${NC}"
  echo "     ${BLUE}systemctl status xboard-scheduler${NC}"
  echo "     ${BLUE}journalctl -u xboard -f${NC}"
elif [ "${START_MODE:-1}" != "2" ] && [ "${START_MODE:-1}" != "3" ]; then
  echo "   进程管理:"
  echo "     ${BLUE}ps aux | grep xboard${NC}"
  echo "     ${BLUE}kill <PID>${NC}  停止服务"
fi

echo ""
info "升级新版本时，重新运行本脚本即可:"
echo "  bash <(curl -fsSL https://github.com/$REPO/releases/latest/download/install.sh) 新版本号"
echo ""
info "如使用 MySQL，建议定期备份:"
echo "  mysqldump -u root -p xboard > xboard_backup_\$(date +%Y%m%d).sql"
echo "========================================"
