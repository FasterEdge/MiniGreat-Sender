# MiniGreat-Sender 多阶段构建
# 用法:
#   docker build -t minigreat-sender .
#   docker build --build-arg TARGETARCH=arm64 -t minigreat-sender:arm64 .   # 交叉构建
# 运行(需要访问串口/SPI/I2C/CAN/BLE 时建议加 --privileged --network host):
#   docker run --rm -it --privileged --network host minigreat-sender send --proto serial --device /dev/ttyUSB0 ...
#   docker run --rm -it --privileged --network host -p 8080:8080 minigreat-sender web --addr 0.0.0.0:8080

# ---------- 构建阶段 ----------
# go.mod 要求 go >= 1.25.0, 使用 1.24 会触发 GOTOOLCHAIN 隐式下载(走 go.dev),
# 在依赖走 goproxy.cn 的受限网络环境中会构建失败; 对齐 1.25 保证构建确定性。
FROM golang:1.25-alpine AS builder

ARG TARGETARCH=amd64
ARG VERSION=1.0.20260902

WORKDIR /src

# 国内网络加速
ENV GOPROXY=https://goproxy.cn,direct
# 先拷贝依赖清单以利用缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 静态编译, 便于在任何 Linux 运行(含精简容器)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/minigreat-sender .

# ---------- 运行阶段 ----------
FROM alpine:3.21

# 蓝牙扫描/串口/GPIO 等需要系统工具; bluetoothctl 供 BLE 调试辅助
RUN apk add --no-cache \
    bluez \
    coreutils \
    tzdata \
    ca-certificates \
    && mkdir -p /etc/bluetooth

WORKDIR /app
COPY --from=builder /out/minigreat-sender /usr/local/bin/minigreat-sender

# 时区默认可改
ENV TZ=Asia/Shanghai

ENTRYPOINT ["minigreat-sender"]
CMD ["help"]
