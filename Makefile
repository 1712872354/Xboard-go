.PHONY: build build-server build-scheduler run dev clean test lint help

APP_NAME     := xboard
BUILD_DIR    := build
COMMIT_HASH  := $(shell git log --format="%h" -1 2>/dev/null || echo "dev")
BUILD_TIME   := $(shell date "+%Y-%m-%d/%H:%M:%S")
LD_FLAGS     := -ldflags "-X main.Version=$(COMMIT_HASH) -X main.BuildTime=$(BUILD_TIME) -s -w"

help: ## 显示帮助信息
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: clean ## 编译所有组件
	@echo "Building $(APP_NAME)..."
	CGO_ENABLED=0 go build $(LD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./cmd/server/
	CGO_ENABLED=0 go build $(LD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-scheduler ./cmd/scheduler/
	@echo "Build complete: $(BUILD_DIR)/"

build-server: ## 仅编译 API 服务
	CGO_ENABLED=0 go build $(LD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./cmd/server/

build-scheduler: ## 仅编译调度器
	CGO_ENABLED=0 go build $(LD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-scheduler ./cmd/scheduler/

build-all: build ## 别名

run: ## 启动 API server (开发)
	go run ./cmd/server/ --config config.yaml

run-scheduler: ## 启动调度器 (开发)
	go run ./cmd/scheduler/ --config config.yaml

dev: ## 热重载开发 (需要 air)
	air --server

clean: ## 清理构建产物
	rm -rf $(BUILD_DIR)/

test: ## 运行测试
	go test ./... -v -count=1 -race -coverprofile=coverage.out

test-short: ## 运行短测试
	go test ./... -short -count=1

lint: ## 代码检查
	golangci-lint run ./...

vet: ## Go vet
	go vet ./...

tidy: ## 整理依赖
	go mod tidy

vendor: ## 创建 vendor 目录
	go mod vendor

migrate: ## 运行数据库迁移
	go run ./cmd/server/ --config config.yaml --migrate

seed: ## 填充测试数据
	go run ./cmd/server/ --config config.yaml --seed

docker-build: ## 构建 Docker 镜像
	docker build -t $(APP_NAME):latest -f Dockerfile .

.PHONY: install-tools
install-tools: ## 安装开发工具
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
