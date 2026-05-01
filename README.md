# Xboard - Go 重构版

基于 PHP Laravel 版 Xboard 项目的 **Go 语言重构**，保持了全部原有功能，并显著提升了性能与可维护性。

## 项目结构

```
xboard-go/
├── cmd/
│   ├── server/               # API 服务入口
│   └── scheduler/            # 定时任务入口
├── internal/
│   ├── auth/                 # JWT 认证
│   ├── bootstrap/            # 应用初始化
│   ├── config/               # 配置管理
│   ├── cron/                 # 定时任务调度
│   ├── database/             # 数据库连接
│   ├── handler/
│   │   ├── v1/              # V1 API 处理器
│   │   └── v2/              # V2 API 处理器（管理后台）
│   ├── middleware/           # HTTP 中间件
│   ├── model/               # GORM 数据模型（32 个）
│   ├── plugin/              # 插件系统
│   ├── protocol/            # 代理协议实现（12 种）
│   ├── queue/               # 异步任务队列
│   ├── router/              # 路由定义
│   ├── service/             # 业务逻辑层
│   └── websocket/           # WebSocket 节点通信
├── pkg/
│   ├── response/            # API 响应格式化
│   └── ...                  # 通用工具包
├── web/                     # 前端静态资源
├── storage/                 # 运行时数据
├── migrations/              # 数据库迁移
├── config.example.yaml      # 配置示例
├── Dockerfile               # 多阶段构建
├── Makefile                 # 构建命令
└── go.mod                   # Go 模块定义
```

## 核心技术栈

| 组件 | 技术选型 |
|------|----------|
| Web 框架 | Gin (高性能 HTTP 框架) |
| ORM | GORM (MySQL / SQLite) |
| 缓存 | Redis / 内存缓存 |
| 认证 | JWT (golang-jwt) |
| WebSocket | Gorilla WebSocket |
| 配置 | Viper |
| 日志 | Zap (高性能结构化日志) |
| 定时任务 | robfig/cron v3 |
| 插件系统 | 接口契约 + 注册模式 |

## 核心特性

### 插件系统（创新重设计）

与 PHP 版本的动态 `require` 不同，Go 版采用**编译时注册 + 接口契约**模式：

- **PaymentPlugin 接口**：统一 `Pay()`, `Notify()`, `Form()` 方法
- **FeaturePlugin 接口**：支持路由注册、命令注册
- **全局注册表**：`plugin.Register()` 在 `init()` 时注册
- **钩子系统**：`HookManager` 支持 `Listen`/`Dispatch`/`Register`/`ApplyFilter`
- **预定义钩子**：8 个核心业务钩子（订单、支付、用户注册等）

### 协议管理

12 种代理协议统一通过 `Protocol` 接口管理：
- `Flags()` - 协议标识
- `GenerateConfig()` - 用户配置生成

采用注册表模式替代 PHP 的 glob+反射方式。

### 并发模型

- **WebSocket**：goroutine-per-connection 原生支持
- **订单处理**：悲观锁 + 事务保护
- **流量处理**：Redis Stream + 批量 Job
- **定时任务**：`withoutOverlapping` 防重叠执行

## 快速开始

### 前置要求

- Go 1.22+
- MySQL 8.0+ 或 SQLite 3
- Redis 7+ (可选，用于缓存和队列)

### 配置

```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml 配置数据库连接
```

### 构建 & 运行

```bash
# 初始化依赖
go mod tidy

# 构建
make build

# 运行数据库迁移
./build/xboard --migrate --config config.yaml

# 创建默认管理员
./build/xboard --seed --config config.yaml

# 启动 API 服务
make run

# 启动调度器（单独进程）
make run-scheduler
```

### Docker 部署

```bash
make docker-build
docker run -d -p 8080:8080 -v ./config.yaml:/app/config.yaml xboard:latest
```

## 性能提升

| 场景 | PHP 版 | Go 版 (预估) | 提升 |
|------|--------|-------------|------|
| API 吞吐量 | ~800 QPS | ~3,000 QPS | 3-4x |
| WebSocket 节点 | ~2,000 | ~10,000+ | 5x+ |
| 内存占用 | ~30MB/req | ~1MB/req | 30x |
| 启动时间 | ~1s | < 50ms | 20x |
| Docker 镜像 | ~500MB | ~30MB | 16x |

## 许可证

MIT License - 同原始项目
