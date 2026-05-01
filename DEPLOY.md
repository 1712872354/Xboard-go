# Xboard-Go 部署指南

## 一键部署

下载一键部署脚本并运行，脚本会自动下载对应平台的发布包并完成全部部署。

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/1712872354/Xboard-go/master/install.sh -o install.sh && chmod +x install.sh && ./install.sh
```

指定版本：

```bash
./install.sh v2.0.2
```

### Windows（PowerShell）

```powershell
Invoke-WebRequest -Uri https://raw.githubusercontent.com/1712872354/Xboard-go/master/install.ps1 -OutFile install.ps1; .\install.ps1
```

### 脚本执行流程

```
① 检测系统架构
② 下载对应平台的发布包（或使用本地文件）
③ 生成配置文件（config.yaml）
④ 初始化数据库 — 自动检测首次安装，选择"全新安装"时：
   安全确认 → 删除所有表 → 自动建表 → 生成初始化数据
⑤ 创建管理员账号（保留数据模式）
⑥ 启动服务（支持 Systemd 安装）
```

---

## 手动步骤

如果脚本交互不满足需求，可手动执行：

### 1. 下载发布包

从 [Releases 页面](https://github.com/1712872354/Xboard-go/releases) 下载对应平台的压缩包解压。

### 2. 配置

```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml，关键配置：
#   app.key         32位随机密钥
#   app.url         服务对外地址
#   database.*      数据库连接信息
```

### 3. 初始化

> 首次启动时会自动创建表结构，无需手动迁移。

```bash
# 创建管理员
./xboard --seed --config config.yaml
```

### 4. 启动服务

```bash
# API 服务
./xboard --config config.yaml

# 调度器（新开终端）
./xboard-scheduler --config config.yaml
```

### 5. 验证

```
http://服务器IP:8080          → 用户前端
http://服务器IP:8080/admin    → 管理后台
```

---

## Systemd 服务（Linux 生产推荐）

一键部署脚本会提示安装。如选择手动安装：

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

## 升级

重新下载一键部署脚本运行即可：

```bash
curl -fsSL https://raw.githubusercontent.com/1712872354/Xboard-go/master/install.sh -o install.sh && chmod +x install.sh && ./install.sh v新版本号
```

或手动替换二进制文件：

```bash
systemctl stop xboard xboard-scheduler
tar -xzf xboard-新版本-linux-amd64.tar.gz -C /tmp/
cp /tmp/xboard-新版本-linux-amd64/xboard /opt/xboard/
cp /tmp/xboard-新版本-linux-amd64/xboard-scheduler /opt/xboard/
cp -r /tmp/xboard-新版本-linux-amd64/web/* /opt/xboard/web/
systemctl start xboard xboard-scheduler
```

---

> **注意**：调度器（`xboard-scheduler`）负责定时任务（订单超时、流量重置、统计等），**必须与 API 服务一起运行**。
