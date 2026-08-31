# MiniGreat-Sender Makefile
VERSION ?= 1.0.20260901
BIN     := minigreat-sender
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build darwin linux linux-arm64 linux-amd64 test vet clean docker docker-arm64 run-web run-list

all: build

# 当前平台构建
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) .

darwin:
	GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)-darwin-arm64 .

# Linux 交叉构建 (树莓派/Jetson/服务器)
linux:
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)-linux-arm64 .

linux-amd64:
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)-linux-amd64 .

linux-arm64:
	GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)-linux-arm64 .

test:
	go vet ./...
	go test ./...

# Docker 构建 (默认 amd64; 指定 arm64: make docker TARGETARCH=arm64)
docker:
	@test -n "$(TARGETARCH)" || TARGETARCH=$$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/'); \
	echo "build docker for $$TARGETARCH"; \
	docker build --build-arg TARGETARCH=$$TARGETARCH --build-arg VERSION=$(VERSION) -t $(BIN):latest .

docker-arm64:
	docker build --build-arg TARGETARCH=arm64 --build-arg VERSION=$(VERSION) -t $(BIN):arm64 .

# Docker 运行 Web 面板 (需硬件透传时再组合 compose)
run-web:
	docker run --rm -it --privileged --network host $(BIN):latest web --addr 0.0.0.0:8080

clean:
	rm -f $(BIN) $(BIN)-*