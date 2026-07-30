.PHONY: all build build-arm64 build-amd64 build-linux run test lint clean deps

BINARY_NAME=dts-panel
VERSION=$(shell git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS=-ldflags="-s -w -X main.Version=$(VERSION)"

all: build-arm64 build-amd64

# 构建 ARM64 (Oracle ARM 部署目标)
build-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc go build $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 ./cmd/panel/
	@echo "✓ 构建完成: $(BINARY_NAME)-linux-arm64"

# 构建 AMD64
build-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 ./cmd/panel/
	@echo "✓ 构建完成: $(BINARY_NAME)-linux-amd64"

# 本地构建
build:
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/panel/
	@echo "✓ 构建完成: $(BINARY_NAME)"

# 运行 Web 面板
run:
	go run ./cmd/panel/

# 运行 CLI
cli:
	go run ./cmd/cli/ $(ARGS)

# 测试
test:
	go test -v ./...

# 代码检查
lint:
	golangci-lint run ./...

# 清理
clean:
	rm -f $(BINARY_NAME)*
	rm -rf ./bin/

# 安装依赖
deps:
	go mod download

# 格式化
fmt:
	go fmt ./...
	goimports -w ./

# 查看架构
info:
	@echo "当前环境: $(shell go env GOOS)/$(shell go env GOARCH)"
	@echo "可用架构: $(shell go tool dist list | grep linux | cut -d/ -f2 | tr '\n' ' ')"
