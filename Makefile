# MyEtcd Makefile

# 变量定义
BINARY_NAME=myetcd
SERVER_BINARY=bin/server
CLIENT_BINARY=bin/client
WATCH_BINARY=bin/watch
LEASE_BINARY=bin/lease
RAFT_BINARY=bin/raft
METRICS_BINARY=bin/metrics
CONFIG_BINARY=bin/config
LOGGER_BINARY=bin/logger
BENCHMARK_BINARY=bin/benchmark
GO_FILES=$(shell find . -name "*.go" -type f)
GO_MOD_FILE=go.mod

# 默认目标
.PHONY: all
all: clean deps test build

# 清理构建文件
.PHONY: clean
clean:
	@echo "Cleaning build files..."
	@rm -rf bin/
	@rm -rf data/
	@go clean -cache

# 安装依赖
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# 格式化代码
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# 代码检查
.PHONY: lint
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, installing..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		golangci-lint run; \
	fi

# 运行测试
.PHONY: test
test:
	@echo "Running tests..."
	@go test -v ./...

# 运行测试并生成覆盖率报告
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	@mkdir -p coverage
	@go test -v -coverprofile=coverage/coverage.out ./...
	@go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "Coverage report generated: coverage/coverage.html"

# 构建服务器
.PHONY: build-server
build-server:
	@echo "Building server..."
	@mkdir -p bin
	@go build -o $(SERVER_BINARY) ./cmd/server

# 构建客户端
.PHONY: build-client
build-client:
	@echo "Building client..."
	@mkdir -p bin
	@go build -o $(CLIENT_BINARY) ./examples/client

# 构建Watch示例
.PHONY: build-watch
build-watch:
	@echo "Building watch example..."
	@mkdir -p bin
	@go build -o $(WATCH_BINARY) ./examples/watch

# 构建租约示例
.PHONY: build-lease
build-lease:
	@echo "Building lease example..."
	@mkdir -p bin
	@go build -o $(LEASE_BINARY) ./examples/lease

# 构建Raft示例
.PHONY: build-raft
build-raft:
	@echo "Building raft example..."
	@mkdir -p bin
	@go build -o $(RAFT_BINARY) ./examples/raft

# 构建指标示例
.PHONY: build-metrics
build-metrics:
	@echo "Building metrics example..."
	@mkdir -p bin
	@go build -o $(METRICS_BINARY) ./examples/metrics

# 构建配置示例
.PHONY: build-config
build-config:
	@echo "Building config example..."
	@mkdir -p bin
	@go build -o $(CONFIG_BINARY) ./examples/config

# 构建日志示例
.PHONY: build-logger
build-logger:
	@echo "Building logger example..."
	@mkdir -p bin
	@go build -o $(LOGGER_BINARY) ./examples/logger

# 构建基准测试示例
.PHONY: build-benchmark
build-benchmark:
	@echo "Building benchmark example..."
	@mkdir -p bin
	@go build -o $(BENCHMARK_BINARY) ./examples/benchmark

# 构建所有二进制文件
.PHONY: build
build: build-server build-client build-watch build-lease build-raft build-metrics build-config build-logger build-benchmark

# 运行服务器
.PHONY: run-server
run-server: build-server
	@echo "Starting MyEtcd server..."
	@mkdir -p data
	@./$(SERVER_BINARY) -data-dir=./data -port=:2379

# 运行客户端示例
.PHONY: run-client
run-client: build-client
	@echo "Running client example..."
	@./$(CLIENT_BINARY)

# 运行Watch示例
.PHONY: run-watch
run-watch: build-watch
	@echo "Running watch example..."
	@./$(WATCH_BINARY)

# 运行租约示例
.PHONY: run-lease
run-lease: build-lease
	@echo "Running lease example..."
	@./$(LEASE_BINARY)

# 运行Raft示例
.PHONY: run-raft
run-raft: build-raft
	@echo "Running raft example..."
	@./$(RAFT_BINARY) node1 8001 3

# 运行指标示例
.PHONY: run-metrics
run-metrics: build-metrics
	@echo "Running metrics example..."
	@./$(METRICS_BINARY)

# 运行配置示例
.PHONY: run-config
run-config: build-config
	@echo "Running config example..."
	@./$(CONFIG_BINARY)

# 运行日志示例
.PHONY: run-logger
run-logger: build-logger
	@echo "Running logger example..."
	@./$(LOGGER_BINARY)

# 运行基准测试示例
.PHONY: run-benchmark
run-benchmark: build-benchmark
	@echo "Running benchmark example..."
	@./$(BENCHMARK_BINARY)

# 开发模式（启动服务器）
.PHONY: dev
dev:
	@echo "Starting MyEtcd in development mode..."
	@mkdir -p data
	@go run ./cmd/server -data-dir=./data -port=:2379

# 开发模式（运行客户端）
.PHONY: dev-client
dev-client:
	@echo "Running client in development mode..."
	@go run ./examples/client

# 开发模式（运行Watch示例）
.PHONY: dev-watch
dev-watch:
	@echo "Running watch example in development mode..."
	@go run ./examples/watch

# 开发模式（运行租约示例）
.PHONY: dev-lease
dev-lease:
	@echo "Running lease example in development mode..."
	@go run ./examples/lease

# 开发模式（运行Raft示例）
.PHONY: dev-raft
dev-raft:
	@echo "Running raft example in development mode..."
	@go run ./examples/raft node1 8001 3

# 开发模式（运行指标示例）
.PHONY: dev-metrics
dev-metrics:
	@echo "Running metrics example in development mode..."
	@go run ./examples/metrics

# 开发模式（运行配置示例）
.PHONY: dev-config
dev-config:
	@echo "Running config example in development mode..."
	@go run ./examples/config

# 开发模式（运行日志示例）
.PHONY: dev-logger
dev-logger:
	@echo "Running logger example in development mode..."
	@go run ./examples/logger

# 开发模式（运行基准测试示例）
.PHONY: dev-benchmark
dev-benchmark:
	@echo "Running benchmark example in development mode..."
	@go run ./examples/benchmark

# 基准测试命令
.PHONY: benchmark
benchmark:
	@echo "运行默认基准测试..."
	go run examples/benchmark/main.go

.PHONY: benchmark-custom
benchmark-custom:
	@echo "运行自定义基准测试..."
	go run examples/benchmark/main.go -clients 20 -operations 50000 -concurrent 200

.PHONY: benchmark-large
benchmark-large:
	@echo "运行大规模基准测试..."
	go run examples/benchmark/main.go -clients 50 -operations 100000 -key-size 32 -value-size 1024

.PHONY: benchmark-stress
benchmark-stress:
	@echo "运行压力测试..."
	go run examples/benchmark/main.go -clients 100 -operations 200000 -concurrent 500 -duration 60

.PHONY: benchmark-report
benchmark-report:
	@echo "生成基准测试报告..."
	go run examples/benchmark/main.go -clients 10 -operations 10000 > benchmark-report.txt
	@echo "报告已保存到 benchmark-report.txt"

.PHONY: benchmark-test
benchmark-test:
	@echo "运行基准测试单元测试..."
	go test -v ./tests/benchmark_test.go

.PHONY: benchmark-bench
benchmark-bench:
	@echo "运行Go基准测试..."
	go test -bench=. -benchmem ./tests/

# 交叉编译
.PHONY: build-all
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p bin
	
	# Linux AMD64
	@echo "Building for linux/amd64..."
	@GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/server
	
	# Linux ARM64
	@echo "Building for linux/arm64..."
	@GOOS=linux GOARCH=arm64 go build -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/server
	
	# macOS AMD64
	@echo "Building for darwin/amd64..."
	@GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/server
	
	# macOS ARM64
	@echo "Building for darwin/arm64..."
	@GOOS=darwin GOARCH=arm64 go build -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/server
	
	# Windows AMD64
	@echo "Building for windows/amd64..."
	@GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/server

# Docker构建
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	@docker build -t myetcd:latest .

# 构建多平台Docker镜像
.PHONY: docker-build-all
docker-build-all:
	@echo "Building multi-platform Docker images..."
	@docker buildx build --platform linux/amd64,linux/arm64 -t myetcd:latest --push .

# 运行单节点Docker容器
.PHONY: docker-run
docker-run:
	@echo "Running MyEtcd in Docker..."
	@docker run -d --name myetcd-single -p 2379:2379 -p 2380:2380 -v $(PWD)/data:/app/data myetcd:latest

# 停止Docker容器
.PHONY: docker-stop
docker-stop:
	@echo "Stopping MyEtcd Docker container..."
	@docker stop myetcd-single || true
	@docker rm myetcd-single || true

# 启动Docker Compose（单节点）
.PHONY: docker-up-single
docker-up-single:
	@echo "Starting MyEtcd single node with Docker Compose..."
	@docker-compose up -d myetcd-single

# 停止Docker Compose（单节点）
.PHONY: docker-down-single
docker-down-single:
	@echo "Stopping MyEtcd single node..."
	@docker-compose stop myetcd-single
	@docker-compose rm -f myetcd-single

# 启动Docker Compose（集群）
.PHONY: docker-up-cluster
docker-up-cluster:
	@echo "Starting MyEtcd cluster with Docker Compose..."
	@docker-compose --profile cluster up -d

# 停止Docker Compose（集群）
.PHONY: docker-down-cluster
docker-down-cluster:
	@echo "Stopping MyEtcd cluster..."
	@docker-compose --profile cluster down

# 启动Docker Compose（监控）
.PHONY: docker-up-monitoring
docker-up-monitoring:
	@echo "Starting MyEtcd with monitoring..."
	@docker-compose --profile monitoring up -d

# 停止Docker Compose（监控）
.PHONY: docker-down-monitoring
docker-down-monitoring:
	@echo "Stopping MyEtcd monitoring..."
	@docker-compose --profile monitoring down

# 启动完整环境（集群+监控）
.PHONY: docker-up-all
docker-up-all:
	@echo "Starting MyEtcd full environment..."
	@docker-compose --profile cluster --profile monitoring up -d

# 停止完整环境
.PHONY: docker-down-all
docker-down-all:
	@echo "Stopping MyEtcd full environment..."
	@docker-compose --profile cluster --profile monitoring down -v

# 查看Docker Compose状态
.PHONY: docker-ps
docker-ps:
	@echo "MyEtcd Docker containers status:"
	@docker-compose ps

# 查看Docker日志
.PHONY: docker-logs
docker-logs:
	@docker-compose logs -f

# 清理Docker资源
.PHONY: docker-clean
docker-clean:
	@echo "Cleaning Docker resources..."
	@docker-compose down -v --remove-orphans
	@docker system prune -f
	@docker volume prune -f

# 生成文档
.PHONY: docs
docs:
	@echo "Generating documentation..."
	@if command -v godoc >/dev/null 2>&1; then \
		echo "Starting godoc server on :6060..."; \
		godoc -http=:6060; \
	else \
		echo "godoc not found, installing..."; \
		go install golang.org/x/tools/cmd/godoc@latest; \
		echo "Starting godoc server on :6060..."; \
		godoc -http=:6060; \
	fi

# 基准测试
.PHONY: bench
bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

# 竞争检测
.PHONY: race
race:
	@echo "Running tests with race detection..."
	@go test -race -v ./...

# 内存泄漏检测
.PHONY: memory
memory:
	@echo "Running tests with memory profiling..."
	@mkdir -p profile
	@go test -memprofile=profile/mem.prof -v ./...
	@go tool pprof -text profile/mem.prof

# CPU性能分析
.PHONY: cpu
cpu:
	@echo "Running with CPU profiling..."
	@mkdir -p profile
	@go test -cpuprofile=profile/cpu.prof -v ./...
	@go tool pprof -text profile/cpu.prof

# 安装开发工具
.PHONY: install-tools
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/godoc@latest
	@go install github.com/air-verse/air@latest

# 热重载开发
.PHONY: watch
watch:
	@echo "Starting hot reload development..."
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air not found, please run 'make install-tools' first"; \
	fi

# 检查更新
.PHONY: update
update:
	@echo "Updating dependencies..."
	@go list -u -m all
	@go get -u ./...
	@go mod tidy

# 验证构建
.PHONY: verify
verify: fmt lint test
	@echo "All checks passed!"

# 快速启动（启动服务器和运行客户端示例）
.PHONY: demo
demo:
	@echo "Starting MyEtcd demo..."
	@mkdir -p data
	@echo "Starting server in background..."
	@./$(SERVER_BINARY) -data-dir=./data -port=:2379 &
	@sleep 2
	@echo "Running client demo..."
	@./$(CLIENT_BINARY)
	@echo "Stopping server..."
	@pkill -f $(SERVER_BINARY) || true

# 帮助信息
.PHONY: help
help:
	@echo "MyEtcd Makefile Commands:"
	@echo ""
	@echo "Build Commands:"
	@echo "  build          - Build server and client binaries"
	@echo "  build-server   - Build server binary only"
	@echo "  build-client   - Build client binary only"
	@echo "  build-watch    - Build watch example only"
	@echo "  build-lease    - Build lease example only"
	@echo "  build-raft     - Build raft example only"
	@echo "  build-metrics  - Build metrics example only"
	@echo "  build-config   - Build config example only"
	@echo "  build-logger   - Build logger example only"
	@echo "  build-benchmark- Build benchmark example only"
	@echo "  build-all      - Build for multiple platforms"
	@echo ""
	@echo "Run Commands:"
	@echo "  run-server     - Build and run server"
	@echo "  run-client     - Build and run client example"
	@echo "  run-watch      - Build and run watch example"
	@echo "  run-lease      - Build and run lease example"
	@echo "  run-raft       - Build and run raft example"
	@echo "  run-metrics    - Build and run metrics example"
	@echo "  run-config     - Build and run config example"
	@echo "  run-logger     - Build and run logger example"
	@echo "  run-benchmark  - Build and run benchmark example"
	@echo "  dev            - Run server in development mode"
	@echo "  dev-client     - Run client in development mode"
	@echo "  dev-watch      - Run watch example in development mode"
	@echo "  dev-lease      - Run lease example in development mode"
	@echo "  dev-raft       - Run raft example in development mode"
	@echo "  dev-metrics    - Run metrics example in development mode"
	@echo "  dev-config     - Run config example in development mode"
	@echo "  dev-logger     - Run logger example in development mode"
	@echo "  dev-benchmark  - Run benchmark example in development mode"
	@echo "  demo           - Quick demo (starts server and runs client)"
	@echo ""
	@echo "Test Commands:"
	@echo "  test           - Run all tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  bench          - Run benchmarks"
	@echo "  race           - Run tests with race detection"
	@echo "  benchmark      - Run default benchmark tests"
	@echo "  benchmark-custom- Run custom benchmark tests"
	@echo "  benchmark-large- Run large-scale benchmark tests"
	@echo "  benchmark-stress- Run stress tests"
	@echo "  benchmark-report- Generate benchmark report"
	@echo "  benchmark-test - Run benchmark unit tests"
	@echo "  benchmark-bench- Run Go benchmarks"
	@echo ""
	@echo "Development Commands:"
	@echo "  fmt            - Format code"
	@echo "  lint           - Run linter"
	@echo "  verify         - Run fmt, lint and test"
	@echo "  watch          - Hot reload development"
	@echo ""
	@echo "Docker Commands:"
	@echo "  docker-build      - Build Docker image"
	@echo "  docker-build-all  - Build multi-platform Docker images"
	@echo "  docker-run        - Run single node in Docker container"
	@echo "  docker-stop       - Stop Docker container"
	@echo "  docker-up-single  - Start single node with Docker Compose"
	@echo "  docker-down-single- Stop single node"
	@echo "  docker-up-cluster - Start cluster with Docker Compose"
	@echo "  docker-down-cluster- Stop cluster"
	@echo "  docker-up-monitoring - Start with monitoring"
	@echo "  docker-down-monitoring - Stop monitoring"
	@echo "  docker-up-all     - Start full environment (cluster + monitoring)"
	@echo "  docker-down-all   - Stop full environment"
	@echo "  docker-ps         - Show container status"
	@echo "  docker-logs       - Show container logs"
	@echo "  docker-clean      - Clean Docker resources"
	@echo ""
	@echo "Utility Commands:"
	@echo "  clean             - Clean build files"
	@echo "  deps              - Install dependencies"
	@echo "  update            - Update dependencies"
	@echo "  docs              - Generate and serve documentation"
	@echo "  help              - Show this help message"
