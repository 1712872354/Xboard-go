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
DEFAULT_KEY=$(tr -dc 'a-zA-Z0-9' < /dev/urandom | head -c 32)
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
# Step 4: 初始化数据库（首次安装检测 + 重装）
# =============================================================
step 4 "初始化数据库"

# 自动检测是否首次安装
DB_HOST=$(grep 'host:' config.yaml | head -1 | awk '{print $2}')
DB_PORT=$(grep 'port:' config.yaml | head -1 | awk '{print $2}')
DB_USER=$(grep 'username:' config.yaml | head -1 | awk '{print $2}')
DB_PASS=$(grep 'password:' config.yaml | head -1 | awk '{print $2}')
DB_NAME=$(grep 'dbname:' config.yaml | head -1 | awk '{print $2}')
DB_DRIVER=$(grep 'driver:' config.yaml | head -2 | tail -1 | awk '{print $2}')

HAS_TABLES=false
HAS_ADMIN=false

if [ "$DB_DRIVER" = "mysql" ] && [ -n "$DB_NAME" ]; then
  MYSQL_CMD="mysql -h $DB_HOST -P $DB_PORT -u $DB_USER"
  [ -n "$DB_PASS" ] && MYSQL_CMD="$MYSQL_CMD -p$DB_PASS"

  TABLE_COUNT=$($MYSQL_CMD -N $DB_NAME -e "SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA='$DB_NAME'" 2>/dev/null || echo "0")
  ADMIN_COUNT=$($MYSQL_CMD -N $DB_NAME -e "SELECT COUNT(*) FROM v2_user WHERE is_admin=1" 2>/dev/null || echo "0")

  [ "$TABLE_COUNT" -gt 0 ] 2>/dev/null && HAS_TABLES=true
  [ "$ADMIN_COUNT" -gt 0 ] 2>/dev/null && HAS_ADMIN=true
fi

echo ""
if [ "$HAS_TABLES" = false ]; then
  echo "  ${GREEN}检测到首次安装，数据库中无现有表${NC}"
elif [ "$HAS_ADMIN" = false ]; then
  echo "  ${YELLOW}检测到数据库有表但尚未创建管理员${NC}"
else
  echo "  ${BLUE}检测到已有管理员账号，如需重置请选择全新安装${NC}"
fi
echo ""

# 安全确认机制
echo "  请选择数据库初始化方式:"
echo "    1) ${RED}全新安装${NC} — 删除所有表 → 重建表结构 → 生成初始化数据"
echo "    2) 保留现有数据 — 仅创建管理员（若无则跳过）"
echo ""
read -rp "  请选择 [1/2]: " INIT_MODE

if [ "$INIT_MODE" = "1" ]; then
  echo ""
  echo "  ${RED}╔══════════════════════════════════════════════════════╗${NC}"
  echo "  ${RED}║  危险操作警告！                                      ║${NC}"
  echo "  ${RED}║  将删除数据库 ${DB_NAME} 中的 ${RED}所有表${NC}               ${RED}║${NC}"
  echo "  ${RED}║  此操作${NC}${RED}不可恢复${NC}${RED}！                                   ║${NC}"
  echo "  ${RED}╚══════════════════════════════════════════════════════╝${NC}"
  echo ""

  # 列出要删除的表
  ALL_TABLES=$($MYSQL_CMD -N $DB_NAME -e "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA='$DB_NAME' ORDER BY TABLE_NAME" 2>/dev/null)
  if [ -n "$ALL_TABLES" ]; then
    TABLE_LIST=$(echo "$ALL_TABLES" | tr '\n' ' ')
    echo "  将要删除以下 ${RED}$(echo "$ALL_TABLES" | wc -l)${NC} 个表:"
    echo "$ALL_TABLES" | sed 's/^/    - /'
    echo ""
  fi

  # 安全检查 1：输入数据库名确认
  echo -n "  输入 ${RED}数据库名称 ($DB_NAME)${NC} 确认: "
  read DB_CONFIRM
  echo ""

  if [ "$DB_CONFIRM" != "$DB_NAME" ]; then
    err "数据库名称输入错误，已取消操作"
    exit 1
  fi

  # 安全检查 2：输入 YES 二次确认
  echo -n "  输入 ${RED}YES${NC}（大写）再次确认不可恢复操作: "
  read YES_CONFIRM
  echo ""

  if [ "$YES_CONFIRM" != "YES" ]; then
    err "确认输入错误，已取消操作"
    exit 1
  fi

  if [ "$DB_DRIVER" = "mysql" ]; then
    # 禁用外键检查，避免删除时因外键约束失败
    $MYSQL_CMD -N $DB_NAME -e "SET FOREIGN_KEY_CHECKS = 0"

    # 获取所有表并删除
    ALL_TABLES=$($MYSQL_CMD -N $DB_NAME -e "SELECT GROUP_CONCAT(TABLE_NAME) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA='$DB_NAME'")
    if [ -n "$ALL_TABLES" ]; then
      $MYSQL_CMD -N $DB_NAME -e "DROP TABLE IF EXISTS $ALL_TABLES"
      ok "已删除所有表"
    fi

    $MYSQL_CMD -N $DB_NAME -e "SET FOREIGN_KEY_CHECKS = 1"
  fi

  ok "数据库已清空，正在重建表结构和初始化数据..."
  echo ""

  # 运行 xboard --seed：内部自动完成 AutoMigrate 建表 + 创建管理员
  if ./xboard --seed --config config.yaml; then
    ok "表结构创建完成"
    echo ""
    ok "默认管理员创建成功（请记录上方输出的邮箱和密码）"
    SEED_DONE=true
  else
    err "数据库初始化失败，请检查配置"
    exit 1
  fi
else
  ok "保留现有数据"

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
