# Xboard-Go 部署文档

```text
Xboard-Go v2.x | Go 重构版代理管理面板
```

---

## 目录

1. [环境要求](#1-环境要求)
2. [快速部署（手动构建）](#2-快速部署手动构建)
3. [通过 GitHub Release 部署](#3-通过-github-release-部署)
4. [Docker 部署](#4-docker-部署)
5. [配置文件说明](#5-配置文件说明)
6. [启动与停止](#6-启动与停止)
7. [Systemd 服务管理](#7-systemd-服务管理)
8. [升级指南](#8-升级指南)
9. [发布流程（维护者）](#9-发布流程维护者)
10. [常见问题](#10-常见问题)

---

## 1. 环境要求

| 组件 | 最低版本 | 备注 |
|------|---------|------|
| Go (仅编译需要) | 1.22+ | 如需自行编译 |
| MySQL | 8.0+ | 生产环境推荐 |
| SQLite | 3.x | 低负载测试环境可用 |
| Redis | 7+ | 可选，推荐用于缓存和队列 |
| OS | Linux / macOS / Windows | 跨平台支持 |

### 端口要求

- `8080`：API 服务 HTTP 端口（可在配置文件中自定义）

---

## 2. 快速部署（手动构建）

### 2.1 下载源码

```bash
git clone https://github.com/your-org/xboard-go.git
cd xboard-go
```

### 2.2 编译构建

```bash
# 安装依赖
go mod tidy

# 编译所有组件
make build

# 编译产物位于 build/ 目录：
#   build/xboard              - API 服务
#   build/xboard-scheduler    - 定时任务调度器
```

### 2.3 初始化配置

```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml，配置数据库、Redis、SMTP 等
```

### 2.4 运行数据库迁移

```bash
./build/xboard --migrate --config config.yaml
```

### 2.5 创建默认管理员

```bash
./build/xboard --seed --config config.yaml
```

> 执行后终端会输出默认管理员的邮箱和密码，请妥善保存。

### 2.6 启动服务

**前台运行（开发测试）**：

```bash
# 启动 API 服务
./build/xboard --config config.yaml

# 新开终端，启动调度器
./build/xboard-scheduler --config config.yaml
```

**后台运行（生产环境）**：

```bash
# 使用 nohup
nohup ./build/xboard --config config.yaml > xboard.log 2>&1 &
nohup ./build/xboard-scheduler --config config.yaml > scheduler.log 2>&1 &

# 或使用 screen / tmux
```

生产环境推荐使用 [Systemd 服务管理](#7-systemd-服务管理)。

---

## 3. 通过 GitHub Release 部署

### 3.1 获取发布包

从项目的 [Releases 页面](https://github.com/your-org/xboard-go/releases) 下载对应平台的压缩包。

| 平台 | 架构 | 文件名示例 |
|------|------|-----------|
| Linux | amd64 | `xboard-v2.0.2-linux-amd64.tar.gz` |
| Linux | arm64 | `xboard-v2.0.2-linux-arm64.tar.gz` |
| macOS | amd64 (Intel) | `xboard-v2.0.2-darwin-amd64.tar.gz` |
| macOS | arm64 (Apple Silicon) | `xboard-v2.0.2-darwin-arm64.tar.gz` |
| Windows | amd64 | `xboard-v2.0.2-windows-amd64.zip` |

### 3.2 解压并配置

```bash
# Linux / macOS
tar -xzf xboard-{version}-{os}-{arch}.tar.gz
cd xboard-{version}-{os}-{arch}/

# Windows（使用 PowerShell 或 7-Zip）
# 解压 ZIP 文件后进入目录
```

### 3.3 目录结构

解压后的目录结构如下：

```
xboard-{version}-{os}-{arch}/
├── xboard                  # API 服务主程序
├── xboard-scheduler        # 定时任务调度器
├── xboard-cli              # CLI 管理工具（如包含）
├── config.example.yaml     # 配置示例文件
├── Dockerfile              # Docker 构建文件
└── web/
    ├── index.html          # 前端入口
    ├── admin.html          # 管理后台入口
    ├── assets/             # 静态资源
    └── theme/              # 主题文件
```

### 3.4 初始化运行

```bash
# 1. 创建配置
cp config.example.yaml config.yaml
# 编辑配置...

# 2. 数据库迁移
./xboard --migrate --config config.yaml

# 3. 创建管理员（首次部署）
./xboard --seed --config config.yaml

# 4. 启动服务
./xboard --config config.yaml
```

---

## 4. Docker 部署

### 4.1 使用 GitHub Packages 镜像

```bash
# 拉取镜像
docker pull ghcr.io/your-org/xboard-go:{version}

# 运行容器
docker run -d \
  --name xboard \
  -p 8080:8080 \
  -v /path/to/config.yaml:/app/config.yaml \
  -v /path/to/storage:/app/storage \
  -v /path/to/plugins:/app/plugins \
  --restart unless-stopped \
  ghcr.io/your-org/xboard-go:{version}
```

### 4.2 自行构建 Docker 镜像

```bash
# 从源码构建
make docker-build

# 或手动构建
docker build -t xboard:latest .

# 运行
docker run -d \
  --name xboard \
  -p 8080:8080 \
  -v ./config.yaml:/app/config.yaml \
  xboard:latest
```

### 4.3 Docker Compose

创建 `docker-compose.yaml`：

```yaml
version: '3.8'

services:
  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: root123
      MYSQL_DATABASE: xboard
      MYSQL_CHARACTER_SET_SERVER: utf8mb4
      MYSQL_COLLATION_SERVER: utf8mb4_unicode_ci
    volumes:
      - mysql_data:/var/lib/mysql
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data
    restart: unless-stopped

  xboard:
    image: ghcr.io/your-org/xboard-go:latest
    ports:
      - "8080:8080"
    volumes:
      - ./config.yaml:/app/config.yaml
      - ./storage:/app/storage
      - ./plugins:/app/plugins
    depends_on:
      - mysql
      - redis
    restart: unless-stopped

  xboard-scheduler:
    image: ghcr.io/your-org/xboard-go:latest
    entrypoint: ["/app/xboard-scheduler", "--config", "/app/config.yaml"]
    volumes:
      - ./config.yaml:/app/config.yaml
      - ./storage:/app/storage
    depends_on:
      - mysql
      - redis
    restart: unless-stopped

volumes:
  mysql_data:
  redis_data:
```

```bash
# 启动全部服务
docker compose up -d

# 首次部署需执行迁移和初始化
docker compose exec xboard /app/xboard --migrate --config /app/config.yaml
docker compose exec xboard /app/xboard --seed --config /app/config.yaml
```

---

## 5. 配置文件说明

`config.yaml` 完整配置项如下（参见 `config.example.yaml`）：

```yaml
server:
  mode: debug              # 运行模式: debug / release / test
  port: 8080               # HTTP 监听端口
  read_timeout: 30s
  write_timeout: 30s

app:
  name: "Xboard"
  version: "2.0.1"
  key: "your-app-key-here"      # 32 位随机密钥，用于加密和 JWT 签名
  debug: true
  url: "http://localhost:8080"  # 服务对外访问地址
  secure_path: "/admin"         # 管理后台路径
  subscribe_path: "/s"          # 订阅路径
  invite_force: false           # 是否强制邀请注册
  invite_commission: 0          # 邀请返利比例
  try_out_plan: ""              # 试用套餐标识（留空关闭试用）
  try_out_hours: 2              # 试用时长（小时）
  email_whitelist_enabled: false
  email_whitelist: []
  stop_register: false          # 关闭注册
  enable_auto_backup: false
  backup_interval: 86400        # 自动备份间隔（秒）
  telegram_bot_token: ""        # Telegram Bot Token

database:
  driver: mysql              # mysql 或 sqlite
  host: 127.0.0.1
  port: 3306
  dbname: xboard
  username: root
  password: ""
  charset: utf8mb4
  table_prefix: "v2_"
  # SQLite 模式
  sqlite_path: "./storage/data/xboard.db"
  # 连接池
  max_idle_conns: 25
  max_open_conns: 100
  conn_max_lifetime: 300s

redis:
  host: 127.0.0.1
  port: 6379
  password: ""
  db: 0
  prefix: "xboard_"

log:
  level: debug             # debug / info / warn / error
  format: console          # console / json
  output: stdout           # stdout / file
  file: "./storage/logs/xboard.log"
  max_size: 100            # MB
  max_backups: 30
  max_age: 7               # days

cache:
  driver: redis            # redis / memory
  ttl: 3600                # 缓存 TTL（秒）

queue:
  driver: redis            # redis / sync
  max_attempts: 3
  retry_delay: 5s

cors:
  allowed_origins: ["*"]
  allowed_methods: ["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"]
  allowed_headers: ["Content-Type", "Authorization"]

smtp:
  host: ""
  port: 465
  username: ""
  password: ""
  encryption: "tls"        # tls / ssl
  from_address: ""
  from_name: "Xboard"

telegram:
  bot_token: ""
  proxy: ""                # socks5://127.0.0.1:1080

plugins:
  path: "./plugins"
  core_path: "./plugins-core"
  auto_install: true

rate_limit:
  enabled: true
  requests: 60             # 每分钟允许的请求数
  duration: 60s
```

### 关键配置项说明

| 配置项 | 说明 |
|--------|------|
| `app.key` | **32 位随机字符串**，用于 JWT 签名和加密。首次部署必须修改 |
| `app.url` | 服务对外访问 URL，用于支付回调、订阅链接等场景 |
| `app.secure_path` | 管理后台入口路径，建议修改为自定义路径增加安全性 |
| `database.table_prefix` | 数据库表前缀，默认为 `v2_` |
| `app.stop_register` | 开启后关闭新用户注册 |

---

## 6. 启动与停止

### API 服务

```bash
# 启动
./xboard --config config.yaml

# 启动（指定端口）
./xboard --config config.yaml
# 端口已在 config.yaml 的 server.port 中配置

# 仅运行数据库迁移
./xboard --migrate --config config.yaml

# 创建管理员种子数据
./xboard --seed --config config.yaml

# 查看版本
./xboard --version
```

### 调度器（必须单独启动）

```bash
# 启动
./xboard-scheduler --config config.yaml

# 查看版本
./xboard-scheduler --version
```

> **注意**：调度器负责定时任务（订单超时、流量重置、每日统计、佣金结算、日志清理、提醒邮件等），**必须与 API 服务一起运行**，否则相应的定时功能无法工作。

### 优雅停止

API 服务和调度器都支持 `SIGINT`（Ctrl+C）和 `SIGTERM` 信号进行优雅关闭。

---

## 7. Systemd 服务管理

### API 服务

创建 `/etc/systemd/system/xboard.service`：

```ini
[Unit]
Description=Xboard API Service
After=network.target mysql.service redis.service
Wants=mysql.service redis.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/xboard
ExecStart=/opt/xboard/xboard --config /opt/xboard/config.yaml
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

### 调度器服务

创建 `/etc/systemd/system/xboard-scheduler.service`：

```ini
[Unit]
Description=Xboard Scheduler Service
After=network.target mysql.service redis.service xboard.service
Wants=mysql.service redis.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/xboard
ExecStart=/opt/xboard/xboard-scheduler --config /opt/xboard/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### 启用并启动

```bash
systemctl daemon-reload
systemctl enable xboard xboard-scheduler
systemctl start xboard
systemctl start xboard-scheduler

# 查看状态
systemctl status xboard
systemctl status xboard-scheduler

# 查看日志
journalctl -u xboard -f
journalctl -u xboard-scheduler -f
```

---

## 8. 升级指南

### 8.1 通过 Release 包升级

```bash
# 1. 备份数据和配置
cp /opt/xboard/config.yaml /opt/xboard/config.yaml.bak
cp -r /opt/xboard/storage /opt/xboard/storage.bak

# 2. 停服
systemctl stop xboard xboard-scheduler

# 3. 下载新版本并解压
tar -xzf xboard-{new-version}-linux-amd64.tar.gz -C /tmp/
cp /tmp/xboard-{new-version}-linux-amd64/xboard /opt/xboard/
cp /tmp/xboard-{new-version}-linux-amd64/xboard-scheduler /opt/xboard/
# 如有前端更新
cp -r /tmp/xboard-{new-version}-linux-amd64/web/* /opt/xboard/web/

# 4. 运行数据库迁移（自动执行，也可手动）
/opt/xboard/xboard --migrate --config /opt/xboard/config.yaml

# 5. 启服
systemctl start xboard xboard-scheduler
```

### 8.2 Docker 升级

```bash
# 拉取新镜像
docker pull ghcr.io/your-org/xboard-go:{new-version}

# 重建容器
docker compose up -d
```

---

## 9. 发布流程（维护者）

### 9.1 手动触发构建并发布

1. 进入 GitHub 仓库的 **Actions** 页面
2. 选择 **Release** 工作流
3. 点击 **Run workflow**
4. 填写参数：

| 参数 | 说明 | 示例 |
|------|------|------|
| `version` | 发布版本号 | `v2.0.2` |
| `pre_release` | 标记为预发布 | 勾选表示预发布 |
| `build_cli` | 是否构建 CLI 工具 | 按需勾选 |

5. 点击 **Run workflow** 触发构建

### 9.2 工作流执行流程

```
手动触发 (workflow_dispatch)
  ├── build (策略矩阵: 5 个平台)
  │   ├── 编译 xboard (API 服务)
  │   ├── 编译 xboard-scheduler (调度器)
  │   ├── 编译 xboard-cli (可选 CLI 工具)
  │   └── 打包分发给对应平台的压缩包
  ├── docker (构建并推送 Docker 镜像)
  │   └── 推送到 ghcr.io 并标记 version + latest
  └── release (创建 GitHub Release)
      ├── 下载所有编译产物
      ├── 生成发布说明
      └── 上传到 Releases 页面
```

### 9.3 构建产物说明

构建完成后，Release 页面将包含以下产物：

| 产物 | 说明 |
|------|------|
| `xboard-{version}-linux-amd64.tar.gz` | Linux amd64 |
| `xboard-{version}-linux-arm64.tar.gz` | Linux arm64 |
| `xboard-{version}-darwin-amd64.tar.gz` | macOS Intel |
| `xboard-{version}-darwin-arm64.tar.gz` | macOS Apple Silicon |
| `xboard-{version}-windows-amd64.zip` | Windows amd64 |
| `ghcr.io/your-org/xboard-go:{version}` | Docker 镜像 |
| `ghcr.io/your-org/xboard-go:latest` | Docker 镜像 (latest) |

每个压缩包包含：
- `xboard` / `xboard.exe` — API 服务主程序
- `xboard-scheduler` / `xboard-scheduler.exe` — 定时任务调度器
- `xboard-cli` — CLI 管理工具（如选中）
- `config.example.yaml` — 配置示例
- `web/` — 前端资源
- `Dockerfile` — Docker 构建文件

---

## 10. 常见问题

### Q: 启动后访问页面返回 404？

检查 `config.yaml` 中的 `app.url` 是否正确配置，以及 `web/` 目录是否存在。

### Q: 数据库迁移失败？

```bash
# 检查数据库连接配置
./xboard --migrate --config config.yaml

# 常见原因：
# - 数据库服务未启动
# - 用户名/密码错误
# - 数据库不存在（需先创建数据库）
# - 端口错误
```

### Q: 调度器不执行定时任务？

```bash
# 确认调度器进程在运行
ps aux | grep xboard-scheduler

# 检查日志输出
tail -f storage/logs/xboard.log

# 确保调度器与 API 服务使用相同的配置文件
```

### Q: 如何修改管理后台路径？

修改 `config.yaml` 中的 `app.secure_path` 字段，默认为 `/admin`。修改后重启服务。

### Q: 如何关闭注册？

设置 `config.yaml` 中 `app.stop_register: true`，重启后生效。

### Q: JWT 令牌提示无效？

```bash
# 重启服务后，所有已签发的令牌将失效
# 用户需重新登录

# 如果修改了 app.key，所有令牌同样会失效
```

### Q: 数据备份建议？

```bash
# MySQL 备份
mysqldump -u root -p xboard > xboard_backup_$(date +%Y%m%d).sql

# 同时备份 config.yaml 和 storage/ 目录
cp config.yaml config.yaml.$(date +%Y%m%d).bak
tar -czf storage.$(date +%Y%m%d).tar.gz storage/
```
