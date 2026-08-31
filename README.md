<div align="center">
  <img src="Logo.png" alt="MiniGreat-Sender" width="120"/>
  <h2>MiniGreat-Sender</h2>
  <h3>全方面多协议请求调试发送工具</h3>
</div>

### 一、项目简介

- 一个**全方面多协议请求调试发送工具**，支持各种模态请求协议与请求方式，有线无线全覆盖，纯 Go 实现。
- 配套接收端为 **[MiniGreat-Receiver](../MiniGreat-Receiver/)**，本工具负责「发」，另一端负责「收」，成对使用可完成完整的链路调试。
- 提供 **CLI 命令行** 与 **本地 Web 调试面板** 两种使用形态：CLI 适合脚本化、批量与自动化场景；Web 面板适合交互式可视化调试。
- 所有协议驱动通过统一抽象（`Request/Response/Driver`）注册，协议可插拔、易扩展。
- Web 静态资源通过 `go:embed` 嵌入二进制，单文件即可部署，无需额外静态目录。

### 二、主要特性

| 类别 | 协议 | 说明 |
|------|------|------|
| 有线网络 | TCP / UDP | 连接发送并读取响应，支持超时 |
| 有线网络 | HTTP / HTTPS | 全部请求方法 + 自定义头 + 任意载荷，TLS 可跳过校验 |
| 有线网络 | WebSocket | `ws://` `wss://` 文本 / 二进制消息 |
| 工控 | MQTT | 连接 broker 发布消息，QoS 0/1/2、Retain、鉴权 |
| 工控 | Modbus | RTU + TCP，功能码 01/02/03/04/05/06/0F/10 |
| 串口 | UART/RS232/RS485 | 波特率 / 数据位 / 校验 / 停止位全可配 |
| 无线射频 | RF-AT 透传 | LoRa / 433MHz / Zigbee / BLE-SPP 等经串口 AT 指令 |
| 无线 BLE | 蓝牙 | BlueZ 扫描 / 连接 / GATT 特征读写与读回 |
| 总线 | CAN | SocketCAN 标准 / 扩展 / 远程帧 |
| 总线 | SPI | `/dev/spidevX.Y` 模式 / 频率 / 位宽可配，全双工 |
| 总线 | I2C | `/dev/i2c-N` 指定从机，支持寄存器寻址读写 |

- 载荷格式三选一：`--hex`（十六进制，可含空格）、`--txt`（文本）、`--b64`（base64），也可 `--json file.json` 传入完整请求参数。
- 跨平台：网络 / 工控 / 串口 / 射频类在 Linux/macOS/Windows 均可运行；CAN / SPI / I2C / BLE 需 Linux（树莓派 / Jetson 等），非 Linux 平台给出明确提示。

### 三、快速开始

> **环境要求**：Go 1.21+。

```bash
make build                      # 或 go build -o minigreat-sender .
./minigreat-sender list         # 查看全部支持的协议
./minigreat-sender help         # 查看全部选项
```

**网络类：**

```bash
./minigreat-sender send --proto tcp --remote 192.168.1.100:9000 --txt "hello"
./minigreat-sender send --proto udp --remote 192.168.1.100:5005 --hex "AABB CC"
./minigreat-sender send --proto http --url http://192.168.1.100:8080/api --method POST \
    --header 'Content-Type: application/json' --txt '{"cmd":"ping"}'
./minigreat-sender send --proto ws --url ws://192.168.1.100:9002 --txt "hello"
```

**工控类：**

```bash
./minigreat-sender send --proto mqtt --broker tcp://127.0.0.1:1883 --topic test --txt "hi" --qos 1
./minigreat-sender send --proto modbus --remote 192.168.1.100:502 --func 03 --addr 0 --qty 10
./minigreat-sender send --proto modbus --remote 192.168.1.100:502 --func 06 --addr 100 --values 1234
```

**串口 / 射频：**

```bash
./minigreat-sender send --proto serial --device /dev/ttyUSB0 --baud 115200 --txt "AT+GMR"
./minigreat-sender send --proto rf --device /dev/ttyUSB0 --baud 9600 --txt "AT+LORA=1"
```

**总线类（Linux）：**

```bash
./minigreat-sender send --proto can --iface vcan0 --id 0x123 --hex "01020304"
./minigreat-sender send --proto spi --device /dev/spidev0.0 --mode 0 --hex "AA"
./minigreat-sender send --proto i2c --bus 1 --addr 0x48 --reg 0x00 --qty 4
```

**蓝牙（Linux, BlueZ）：**

```bash
./minigreat-sender send --proto ble --addr AA:BB:CC:DD:EE:FF \
    --service 0000ffe0-0000-1000-8000-00805f9b34fb \
    --char 0000ffe1-0000-1000-8000-00805f9b34fb --txt "hi" --readresp
```

### 四、Web 调试面板

```bash
./minigreat-sender web --addr 127.0.0.1:8080 --open
```

打开浏览器即得可视化面板：

- 左侧选择协议 → 填写连接参数与载荷 → 点「🚀 发送」。
- 右侧「实时日志」经 WebSocket 推送每次收发记录；「最近响应」展示完整 JSON 响应（状态码 / 耗时 / HEX / 元信息）。
- 载荷格式（Hex / Text / Base64）与超时均可在面板内切换。

### 五、Docker 部署

```bash
# 构建（amd64 默认；树莓派 / Jetson 用 arm64）
make docker                          # 或 make docker TARGETARCH=arm64

# 运行 CLI（硬件协议需 --privileged --network host）
docker run --rm -it --privileged --network host minigreat-sender:latest \
    send --proto serial --device /dev/ttyUSB0 ...

# 运行 Web 面板（已配置硬件透传示例）
docker compose up -d sender-web
```

> 串口 / SPI / I2C / CAN / BLE 需要容器以 `--privileged --network host` 运行，并通过 `devices` / `volumes` 透传宿主设备与 D-Bus（见 `docker-compose.yml`）。镜像为 Alpine 静态二进制，任意 Linux 主机一键部署（含跨架构）。

### 六、目录结构

```
MiniGreat-Sender/
├─ main.go                     # 入口
├─ internal/
│  ├─ cli/                     # 命令行子命令（send / web / list）
│  ├─ core/                    # 核心抽象：Request / Response / Driver / 载荷解析
│  ├─ driver/                  # 各协议发送驱动
│  │  ├─ netdrv/               # TCP / UDP
│  │  ├─ httpdrv/              # HTTP / HTTPS
│  │  ├─ wsdrv/                # WebSocket
│  │  ├─ mqttdrv/              # MQTT
│  │  ├─ modbusdrv/            # Modbus RTU / TCP
│  │  ├─ serialdrv/            # 串口 UART/RS232/RS485
│  │  ├─ rfdrv/                # 射频 AT 透传
│  │  ├─ candrv/  spidrv/  i2cdrv/  bledrv/   # Linux 硬件协议（+非 Linux 桩）
│  └─ web/                     # Web 调试面板（go:embed 内嵌静态资源）
├─ Dockerfile
├─ docker-compose.yml
├─ Makefile
├─ go.mod / go.sum
├─ Logo.png
└─ README.md / README_en.md
```

### 七、平台支持

| 类别 | Linux | macOS / Windows |
|------|:-----:|:---------------:|
| TCP / UDP / HTTP / WS / MQTT / Modbus | ✅ | ✅ |
| 串口 / RF-AT | ✅ | ✅ |
| CAN / SPI / I2C / BLE | ✅ | ❌（提示改用 Linux） |
| Docker | ✅（amd64 / arm64） | — |

### 八、License

Apache License 2.0
