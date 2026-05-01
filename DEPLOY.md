# Xboard-Go 部署指南

从 GitHub Releases 下载预编译发布包，使用一键部署脚本即可快速完成部署。

---

## 1. 下载

在 [Releases 页面](https://github.com/1712872354/Xboard-go/releases) 下载对应平台的压缩包：

| 平台 | 架构 | 文件 |
|------|------|------|
| Linux | amd64 | `xboard-v版本号-linux-amd64.tar.gz` |
| Linux | arm64 | `xboard-v版本号-linux-arm64.tar.gz` |
| macOS | Intel | `xboard-v版本号-darwin-amd64.tar.gz` |
| macOS | Apple Silicon | `xboard-v版本号-darwin-arm64.tar.gz` |
| Windows | amd64 | `xboard-v版本号-windows-amd64.zip` |

## 2. 解压 & 一键部署

### Linux / macOS

```bash
tar -xzf xboard-*.tar.gz
cd xboard-*/
chmod +x install.sh
./install.sh
```

脚本会自动完成：环境检查 → 配置文件生成 → 数据库迁移 → 管理员创建 → 服务启动 → Systemd 安装（可选）。

### Windows

```powershell
Expand-Archive .\xboard-*.zip -DestinationPath .
cd xboard-*/
.\install.ps1
```

## 3. 目录结构

解压后包含：

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

## 4. 手动配置

如需自定义配置，编辑 `config.yaml`：

```yaml
app:
  key: "生成一个32位随机字符串"    # 必须修改！
  url: "http://你的IP:8080"
  secure_path: "/admin"

database:
  driver: mysql               # mysql 或 sqlite
  host: 127.0.0.1
  port: 3306
  dbname: xboard
  username: root
  password: "数据库密码"
```

### 命令行操作

```bash
# 手动执行迁移
./xboard --migrate --config config.yaml

# 创建管理员
./xboard --seed --config config.yaml

# 启动服务
./xboard --config config.yaml
./xboard-scheduler --config config.yaml
```

## 5. Systemd 服务（Linux 生产环境）

一键部署脚本会自动询问并安装。如需手动安装：

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

## 6. 验证

```
http://你的IP:8080          → 用户前端
http://你的IP:8080/admin    → 管理后台
```

## 7. 升级

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
