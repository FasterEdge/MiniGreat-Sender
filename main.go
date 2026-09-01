// FasterEdge 开源项目 · https://github.com/FasterEdge · https://gitee.com/FasterEdge
// MiniGreat-Sender: 全方面多协议请求调试发送工具。
// 支持 TCP/UDP/HTTP/WebSocket/MQTT/Modbus/串口/RF-AT/CAN/SPI/I2C/BLE,
// 有线无线全覆盖, 提供 CLI 与本地 Web 调试面板两种使用方式。
package main

import (
	"os"

	"minigreat-sender/internal/cli"
)

var version = "1.0.20260902" // 可通过 -ldflags "-X main.version=..." 覆盖

func main() {
	os.Exit(cli.Run(os.Args, os.Stdout, os.Stderr))
}