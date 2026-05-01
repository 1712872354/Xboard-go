# Xboard-Go 部署指南（从发布包部署）

本文档适用于从 GitHub Releases 页面下载预编译的发布包进行部署。

---

## 1. 下载发布包

在 [Releases 页面](https://github.com/your-org/xboard-go/releases) 找到对应版本，根据操作系统下载：

| 平台 | 架构 | 下载文件 |
|------|------|----------|
| Linux | amd64 | `xboard-v版本号-linux-amd64.tar.gz` |
| Linux | arm64 | `xboard-v版本号-linux-arm64.tar.gz` |
| macOS | Intel | `xboard-v版本号-darwin-amd64.tar.gz` |
| macOS | Apple Silicon | `xboard-v版本号-darwin-arm64.tar.gz` |
| Windows | amd64 | `xboard-v版本号-windows-amd64.zip` |

## 2. 解压

```bash
# Linux / macOS
tar -xzf xboard-*.tar.gz
cd xboard-*/

# Windows: 右键解压或使用 PowerShell
# Expand-Archive .\xboard-*.zip -DestinationPath .
# cd xboard-*/
```

## 3. 目录结构

```
xboard-版本号-系统-架构/
├── xboard                   # API 服务主程序
├── xboard-scheduler         # 定时任务调度器
├── config.example.yaml      # 配置示例文件
├── Dockerfile               # Docker 构建文件
└── web/
    ├── index.html           # 用户前端
    ├── admin.html           # 管理后台
    ├── assets/              # 静态资源
    └── theme/               # 主题文件
```

## 4. 配置

```bash
cp config.example.yaml config.yaml
```

根据实际环境编辑 `config.yaml`，关键配置：

```yaml
app:
  key: "生成一个32位随机字符串"    # 首次部署必须修改！
  url: "http://你的服务器IP:8080"  # 对外访问地址
  secure_path: "/admin"           # 管理后台路径，建议修改

database:
  driver: mysql                   # mysql 或 sqlite
  host: 127.0.0.1
  port: 3306
  dbname: xboard
  username: root
  password: "数据库密码"
  table_prefix: "v2_"

redis:
  host: 127.0.0.1
  port: 6379
```

## 5. 初始化

```bash
# 运行数据库迁移（创建表结构）
./xboard --migrate --config config.yaml

# 创建默认管理员（首次部署）
./xboard --seed --config config.yaml
# 执行后终端会输出管理员邮箱和密码，请保存
```

## 6. 启动

### 前台运行（测试用）

```bash
# 启动 API 服务
./xboard --config config.yaml

# 新开终端，启动调度器
./xboard-scheduler --config config.yaml
```

### 后台运行

```bash
nohup ./xboard --config config.yaml > xboard.log 2>&1 &
nohup ./xboard-scheduler --config config.yaml > scheduler.log 2>&1 &
```

### Systemd 服务（生产推荐）

创建 `/etc/systemd/system/xboard.service`：

```ini
[Unit]
Description=Xboard API Service
After=network.target mysql.service redis.service

[Service]
Type=simple
WorkingDirectory=/opt/xboard
ExecStart=/opt/xboard/xboard --config /opt/xboard/config.yaml
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

创建 `/etc/systemd/system/xboard-scheduler.service`：

```ini
[Unit]
Description=Xboard Scheduler
After=network.target mysql.service redis.service

[Service]
Type=simple
WorkingDirectory=/opt/xboard
ExecStart=/opt/xboard/xboard-scheduler --config /opt/xboard/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload
systemctl enable xboard xboard-scheduler
systemctl start xboard
systemctl start xboard-scheduler
```

## 7. 验证

打开浏览器访问 `http://你的服务器IP:8080`，应看到用户前端页面。
访问 `http://你的服务器IP:8080/admin`（或你修改的 secure_path），使用管理员账号登录。

## 8. 升级

```bash
# 停服
systemctl stop xboard xboard-scheduler

# 下载新版解压，只替换二进制和前端文件
tar -xzf xboard-新版本-linux-amd64.tar.gz -C /tmp/
cp /tmp/xboard-新版本-linux-amd64/xboard /opt/xboard/
cp /tmp/xboard-新版本-linux-amd64/xboard-scheduler /opt/xboard/
cp -r /tmp/xboard-新版本-linux-amd64/web/* /opt/xboard/web/

# 运行迁移（如有数据库变更，自动执行）
/opt/xboard/xboard --migrate --config /opt/xboard/config.yaml

# 启服
systemctl start xboard xboard-scheduler
```

---

> **注意**：调度器（`xboard-scheduler`）负责定时任务（订单超时、流量重置、统计、佣金结算等），**必须与 API 服务一起运行**。
