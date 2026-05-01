# Xboard-Go 部署指南

提供一键部署脚本，支持独立运行（自动下载发布包）和本地运行两种模式。

---

## 方式一：一键部署（自动下载运行）

无需手动下载，直接执行以下命令：

### Linux / macOS

```bash
# 部署最新版
bash <(curl -fsSL https://github.com/1712872354/Xboard-go/releases/latest/download/install.sh)

# 部署指定版本
bash <(curl -fsSL https://github.com/1712872354/Xboard-go/releases/latest/download/install.sh) v2.0.2
```

### Windows（PowerShell）

```powershell
# 部署最新版
irm https://github.com/1712872354/Xboard-go/releases/latest/download/install.ps1 | iex

# 部署指定版本
$v="v2.0.2"; irm https://github.com/1712872354/Xboard-go/releases/latest/download/install.ps1 | iex
```

脚本会自动完成全部步骤：

```
① 检测系统架构 → ② 下载对应发布包 → ③ 生成配置
④ 数据库迁移 → ⑤ 创建管理员 → ⑥ 启动服务（可选 Systemd）
```

---

## 方式二：下载发布包后部署

### 1. 下载

在 [Releases 页面](https://github.com/1712872354/Xboard-go/releases) 下载对应平台的压缩包：

| 平台 | 架构 | 文件 |
|------|------|------|
| Linux | amd64 | `xboard-v版本号-linux-amd64.tar.gz` |
| Linux | arm64 | `xboard-v版本号-linux-arm64.tar.gz` |
| macOS | Intel | `xboard-v版本号-darwin-amd64.tar.gz` |
| macOS | Apple Silicon | `xboard-v版本号-darwin-arm64.tar.gz` |
| Windows | amd64 | `xboard-v版本号-windows-amd64.zip` |

### 2. 解压并运行一键脚本

```bash
# Linux / macOS
tar -xzf xboard-*.tar.gz
cd xboard-*/
chmod +x install.sh
./install.sh
```

```powershell
# Windows
Expand-Archive .\xboard-*.zip -DestinationPath .
cd xboard-*/
.\install.ps1
```

### 3. 解压目录结构

```
xboard-版本号-系统-架构/
├── xboard                   # API 服务主程序
├── xboard-scheduler         # 定时任务调度器
├── install.sh / install.ps1 # 一键部署脚本
├── config.example.yaml      # 配置示例
├── Dockerfile               # Docker 构建文件
└── web/
    ├── index.html           # 用户前端
    ├── admin.html           # 管理后台
    └── assets/              # 静态资源
```

---

## 手动操作

如需跳过脚本交互，可直接手动执行：

```bash
# 1. 配置
cp config.example.yaml config.yaml
# 编辑 config.yaml，关键项：
#   app.key       → 32位随机密钥
#   app.url       → 服务对外地址
#   database.*    → 数据库连接信息

# 2. 数据库迁移
./xboard --migrate --config config.yaml

# 3. 创建管理员
./xboard --seed --config config.yaml

# 4. 启动服务
./xboard --config config.yaml           # API 服务
./xboard-scheduler --config config.yaml  # 调度器（新开终端）
```

---

## Systemd 服务（Linux 生产推荐）

一键部署脚本会提示安装。手动安装：

```bash
cat > /etc/systemd/system/xboard.service << 'EOF'
[Unit]
Description=Xboard API Service
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/xboard
ExecStart=/opt/xboard/xboard --config /opt/xboard/config.yaml
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/xboard-scheduler.service << 'EOF'
[Unit]
Description=Xboard Scheduler
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/xboard
ExecStart=/opt/xboard/xboard-scheduler --config /opt/xboard/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable xboard xboard-scheduler
systemctl start xboard xboard-scheduler
```

---

## 验证

| 地址 | 说明 |
|------|------|
| `http://服务器IP:8080` | 用户前端 |
| `http://服务器IP:8080/admin` | 管理后台 |

---

## 升级

重新运行一键部署脚本指定新版本即可：

```bash
bash <(curl -fsSL https://github.com/1712872354/Xboard-go/releases/latest/download/install.sh) v新版本号
```

或手动操作：

```bash
systemctl stop xboard xboard-scheduler
tar -xzf xboard-新版本-linux-amd64.tar.gz -C /tmp/
cp /tmp/xboard-新版本-linux-amd64/xboard /opt/xboard/
cp /tmp/xboard-新版本-linux-amd64/xboard-scheduler /opt/xboard/
cp -r /tmp/xboard-新版本-linux-amd64/web/* /opt/xboard/web/
/opt/xboard/xboard --migrate --config /opt/xboard/config.yaml
systemctl start xboard xboard-scheduler
```

---

> **注意**：调度器（`xboard-scheduler`）负责定时任务（订单超时、流量重置、统计等），**必须与 API 服务一起运行**。
