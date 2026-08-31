<div align="center">
  <img src="Logo.png" alt="MiniGreat-Sender" width="120"/>
  <h2>MiniGreat-Sender</h2>
  <h3>All-in-One Multi-Protocol Request Debugging & Sending Tool</h3>
</div>

### 1. Introduction

- An **all-in-one multi-protocol request debugging & sending tool**, supporting various request protocols and methods over both wired and wireless links, implemented purely in Go.
- Its companion receiver is **[MiniGreat-Receiver](../MiniGreat-Receiver/)**: this tool "sends", the other "receives", and the pair covers full end-to-end link debugging.
- Two usage modes: a **CLI** for scripting, batch and automation, and a **local Web debug panel** for interactive visualization.
- All protocol drivers are registered through a unified abstraction (`Request/Response/Driver`), making protocols pluggable and easy to extend.
- Web static assets are embedded into the binary via `go:embed`, so a single file deploys with no extra static directory.

### 2. Key Features

| Category | Protocol | Description |
|----------|----------|-------------|
| Wired network | TCP / UDP | Connect, send and read responses with timeout support |
| Wired network | HTTP / HTTPS | All request methods + custom headers + arbitrary payloads, TLS verification skippable |
| Wired network | WebSocket | Text / binary messages over `ws://` `wss://` |
| Industrial | MQTT | Publish to a broker with QoS 0/1/2, Retain and auth |
| Industrial | Modbus | RTU + TCP, function codes 01/02/03/04/05/06/0F/10 |
| Serial | UART/RS232/RS485 | Baud rate / data bits / parity / stop bits fully configurable |
| Wireless RF | RF-AT passthrough | LoRa / 433MHz / Zigbee / BLE-SPP modules via serial AT commands |
| Wireless BLE | Bluetooth | BlueZ scan / connect / GATT characteristic read-write & readback |
| Bus | CAN | SocketCAN standard / extended / remote frames |
| Bus | SPI | `/dev/spidevX.Y` with mode / speed / bits, full duplex |
| Bus | I2C | `/dev/i2c-N` with slave addressing and register-based read/write |

- Payload in three formats: `--hex` (hex with optional spaces), `--txt` (text), `--b64` (base64); or `--json file.json` for the full request spec.
- Cross-platform: network / industrial / serial / RF protocols run on Linux, macOS and Windows; CAN / SPI / I2C / BLE require Linux (Raspberry Pi / Jetson etc.) and give a clear notice on other platforms.

### 3. Quick Start

> **Requirements**: Go 1.21+.

```bash
make build                    # or: go build -o minigreat-sender .
./minigreat-sender list       # list all supported protocols
./minigreat-sender help       # show all options
```

**Network:**

```bash
./minigreat-sender send --proto tcp --remote 192.168.1.100:9000 --txt "hello"
./minigreat-sender send --proto udp --remote 192.168.1.100:5005 --hex "AABB CC"
./minigreat-sender send --proto http --url http://192.168.1.100:8080/api --method POST \
    --header 'Content-Type: application/json' --txt '{"cmd":"ping"}'
./minigreat-sender send --proto ws --url ws://192.168.1.100:9002 --txt "hello"
```

**Industrial:**

```bash
./minigreat-sender send --proto mqtt --broker tcp://127.0.0.1:1883 --topic test --txt "hi" --qos 1
./minigreat-sender send --proto modbus --remote 192.168.1.100:502 --func 03 --addr 0 --qty 10
./minigreat-sender send --proto modbus --remote 192.168.1.100:502 --func 06 --addr 100 --values 1234
```

**Serial / RF:**

```bash
./minigreat-sender send --proto serial --device /dev/ttyUSB0 --baud 115200 --txt "AT+GMR"
./minigreat-sender send --proto rf --device /dev/ttyUSB0 --baud 9600 --txt "AT+LORA=1"
```

**Bus (Linux):**

```bash
./minigreat-sender send --proto can --iface vcan0 --id 0x123 --hex "01020304"
./minigreat-sender send --proto spi --device /dev/spidev0.0 --mode 0 --hex "AA"
./minigreat-sender send --proto i2c --bus 1 --addr 0x48 --reg 0x00 --qty 4
```

**Bluetooth (Linux, BlueZ):**

```bash
./minigreat-sender send --proto ble --addr AA:BB:CC:DD:EE:FF \
    --service 0000ffe0-0000-1000-8000-00805f9b34fb \
    --char 0000ffe1-0000-1000-8000-00805f9b34fb --txt "hi" --readresp
```

### 4. Web Debug Panel

```bash
./minigreat-sender web --addr 127.0.0.1:8080 --open
```

Open the browser for a visual panel:

- Pick a protocol on the left → fill in connection parameters and payload → click "🚀 Send".
- The "Live Log" on the right pushes every send/receive via WebSocket; "Latest Response" shows the full JSON response (status code / latency / HEX / metadata).
- Payload format (Hex / Text / Base64) and timeout can be toggled right in the panel.

### 5. Docker Deployment

```bash
# Build (amd64 by default; use arm64 for Raspberry Pi / Jetson)
make docker                        # or: make docker TARGETARCH=arm64

# Run CLI (hardware protocols need --privileged --network host)
docker run --rm -it --privileged --network host minigreat-sender:latest \
    send --proto serial --device /dev/ttyUSB0 ...

# Run the Web panel (hardware passthrough already configured)
docker compose up -d sender-web
```

> Serial / SPI / I2C / CAN / BLE require the container to run with `--privileged --network host` and to pass through host devices and D-Bus via `devices` / `volumes` (see `docker-compose.yml`). The image is an Alpine static binary, deployable on any Linux host with one command (including cross-arch).

### 6. Directory Structure

```
MiniGreat-Sender/
├─ main.go                     # Entry point
├─ internal/
│  ├─ cli/                     # CLI subcommands (send / web / list)
│  ├─ core/                    # Core abstractions: Request / Response / Driver / payload parsing
│  ├─ driver/                  # Protocol send drivers
│  │  ├─ netdrv/               # TCP / UDP
│  │  ├─ httpdrv/              # HTTP / HTTPS
│  │  ├─ wsdrv/                # WebSocket
│  │  ├─ mqttdrv/              # MQTT
│  │  ├─ modbusdrv/            # Modbus RTU / TCP
│  │  ├─ serialdrv/            # Serial UART/RS232/RS485
│  │  ├─ rfdrv/                # RF AT passthrough
│  │  ├─ candrv/  spidrv/  i2cdrv/  bledrv/   # Linux hardware protocols (+ non-Linux stubs)
│  └─ web/                     # Web debug panel (embedded via go:embed)
├─ Dockerfile
├─ docker-compose.yml
├─ Makefile
├─ go.mod / go.sum
├─ Logo.png
└─ README.md / README_en.md
```

### 7. Platform Support

| Category | Linux | macOS / Windows |
|----------|:-----:|:---------------:|
| TCP / UDP / HTTP / WS / MQTT / Modbus | ✅ | ✅ |
| Serial / RF-AT | ✅ | ✅ |
| CAN / SPI / I2C / BLE | ✅ | ❌ (use Linux) |
| Docker | ✅ (amd64 / arm64) | — |

### 8. License

MIT
