// Package cli 提供命令行子命令式发送工具。
// 用法示例:
//
//	minigreat-sender send --proto tcp --remote 127.0.0.1:9000 --txt "hello"
//	minigreat-sender send --proto http --url http://x/ --method POST --txt '{"a":1}'
//	minigreat-sender send --proto mqtt --broker tcp://127.0.0.1:1883 --topic test --txt hi --qos 1
//	minigreat-sender send --proto modbus --remote 127.0.0.1:502 --func 03 --addr 0 --qty 10
//	minigreat-sender send --proto serial --device /dev/ttyUSB0 --baud 115200 --hex "AABB"
//	minigreat-sender send --proto can --iface vcan0 --id 0x123 --hex "01020304"
//	minigreat-sender send --proto spi --device /dev/spidev0.0 --hex "AA"
//	minigreat-sender send --proto i2c --bus 1 --addr 0x48 --hex "00"
//	minigreat-sender send --proto ble --addr XX:XX --char <uuid> --txt "hi"
//	minigreat-sender web --addr :8080      # 启动 Web 调试面板
//	minigreat-sender list                    # 列出全部协议
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"minigreat-sender/internal/core"
	"minigreat-sender/internal/registry"
)

// Run 解析 os.Args 并执行。返回进程退出码。
func Run(args []string, stdout, stderr *os.File) int {
	if len(args) < 2 {
		printUsage(stderr)
		return 2
	}
	cmd := args[1]
	switch cmd {
	case "list":
		return cmdList(stdout)
	case "send", "s":
		return cmdSend(args[2:], stdout, stderr)
	case "web", "w":
		return cmdWeb(args[2:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "未知子命令: %s\n\n", cmd)
		printUsage(stderr)
		return 2
	}
}

func printUsage(w *os.File) {
	fmt.Fprint(w, `MiniGreat-Sender - 全方面多协议请求调试发送工具

用法:
  minigreat-sender list                    列出全部支持的协议
  minigreat-sender send <选项>             执行一次发送
  minigreat-sender web   <选项>            启动本地 Web 调试面板
  minigreat-sender help                    显示本帮助

send 子命令通用选项:
  --proto <名>      协议名: tcp|udp|http|ws|mqtt|modbus|serial|rf|can|spi|i2c|ble
  --timeout <dur>   超时 (如 3s, 默认按协议)
  --hex <str>       十六进制载荷, 如 "AABB CC"
  --txt <str>       文本载荷
  --b64 <str>       base64 载荷
  --json <file>     从 JSON 文件读取完整请求参数 (字段见 internal/core)

网络类 (tcp/udp):       --remote host:port
HTTP (http):            --url <url> --method GET|POST|... --header K:V(可多次) --insecure
WebSocket (ws):         --url ws://...
MQTT (mqtt):            --broker tcp://host:1883 --topic t --qos 0-2 --retain --user --pass --client
Modbus (modbus):        --remote host:502 或 --device /dev/ttyUSB0 --func 01/02/03/04/05/06/0F/10
                        --addr --qty --values 1,2,3 --unit 1 --baud 9600
串口 (serial/rf):       --device /dev/ttyUSB0 --baud 115200 --databits 8 --parity N --stopbits 1
CAN (can):              --iface can0 --id 0x123 --ext --rtr
SPI (spi):              --device /dev/spidev0.0 --mode 0 --bits 8 --speed 1000000
I2C (i2c):              --bus 1 --addr 0x48 --reg -1 --qty 0
BLE (ble):              --addr XX:XX:XX:XX:XX:XX --service <uuid> --char <uuid> --readresp

web 子命令选项:
  --addr host:port    监听地址 (默认 127.0.0.1:8080)
`)
}

func cmdList(w *os.File) int {
	reg := registry.New()
	fmt.Fprintln(w, "MiniGreat-Sender 支持的协议:")
	for _, n := range reg.Names() {
		d, _ := reg.Get(n)
		fmt.Fprintf(w, "  %-8s %s\n", n, d.Description())
	}
	return 0
}

// sendFlags 承载命令行解析出的请求。
type sendFlags struct {
	proto     string
	timeout   string
	hex       string
	txt       string
	b64       string
	jsonFile  string

	remote    string
	url       string
	method    string
	headers   multiFlag
	insecure  bool

	broker    string
	client    string
	user      string
	pass      string
	topic     string
	qos       int
	retain    bool

	funcCode  string
	addr      int
	qty       int
	values    string
	unit      int

	device    string
	baud      int
	databits  int
	parity    string
	stopbits  int

	iface     string
	canID     string
	ext       bool
	rtr       bool

	spiDevice string
	mode      int
	bits      int
	speed     int64

	bus       int
	i2cAddr   int
	reg       int

	bleAddr   string
	svc       string
	chr       string
	readresp  bool
}

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

func cmdSend(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var f sendFlags
	fs.StringVar(&f.proto, "proto", "", "协议名")
	fs.StringVar(&f.timeout, "timeout", "", "超时 (如 3s)")
	fs.StringVar(&f.hex, "hex", "", "十六进制载荷")
	fs.StringVar(&f.txt, "txt", "", "文本载荷")
	fs.StringVar(&f.b64, "b64", "", "base64 载荷")
	fs.StringVar(&f.jsonFile, "json", "", "从 JSON 文件读取请求")

	fs.StringVar(&f.remote, "remote", "", "远程地址 host:port")
	fs.StringVar(&f.url, "url", "", "HTTP/WS URL")
	fs.StringVar(&f.method, "method", "", "HTTP 方法")
	fs.Var(&f.headers, "header", "HTTP 头 K:V (可多次)")
	fs.BoolVar(&f.insecure, "insecure", false, "跳过 TLS 校验")

	fs.StringVar(&f.broker, "broker", "", "MQTT broker")
	fs.StringVar(&f.client, "client", "", "MQTT client id")
	fs.StringVar(&f.user, "user", "", "MQTT/其它 用户名")
	fs.StringVar(&f.pass, "pass", "", "密码")
	fs.StringVar(&f.topic, "topic", "", "MQTT topic")
	fs.IntVar(&f.qos, "qos", 0, "MQTT QoS 0-2")
	fs.BoolVar(&f.retain, "retain", false, "MQTT retain")

	fs.StringVar(&f.funcCode, "func", "", "Modbus 功能码 (03/06/10 等)")
	fs.IntVar(&f.addr, "addr", 0, "Modbus 寄存器/线圈地址 或 I2C 寄存器")
	fs.IntVar(&f.qty, "qty", 1, "Modbus 读取数量 或 I2C 读取字节数")
	fs.StringVar(&f.values, "values", "", "Modbus 写入值, 逗号分隔")
	fs.IntVar(&f.unit, "unit", 1, "Modbus 从机地址")

	fs.StringVar(&f.device, "device", "", "串口设备")
	fs.IntVar(&f.baud, "baud", 115200, "波特率")
	fs.IntVar(&f.databits, "databits", 8, "数据位")
	fs.StringVar(&f.parity, "parity", "N", "校验 N/E/O")
	fs.IntVar(&f.stopbits, "stopbits", 1, "停止位")

	fs.StringVar(&f.iface, "iface", "", "CAN 接口")
	fs.StringVar(&f.canID, "id", "0x000", "CAN ID")
	fs.BoolVar(&f.ext, "ext", false, "CAN 扩展帧")
	fs.BoolVar(&f.rtr, "rtr", false, "CAN 远程帧")

	fs.StringVar(&f.spiDevice, "spi", "", "SPI 设备 /dev/spidevX.Y")
	fs.IntVar(&f.mode, "mode", 0, "SPI 模式")
	fs.IntVar(&f.bits, "bits", 8, "SPI 位宽")
	fs.Int64Var(&f.speed, "speed", 1000000, "SPI 频率 Hz")

	fs.IntVar(&f.bus, "bus", -1, "I2C 总线号")
	fs.IntVar(&f.i2cAddr, "addr2", 0, "I2C 从机地址(替代 --addr)")
	fs.IntVar(&f.reg, "reg", -1, "I2C 寄存器, -1 不带")

	fs.StringVar(&f.bleAddr, "ble", "", "BLE 目标 MAC")
	fs.StringVar(&f.svc, "service", "", "BLE 服务 UUID")
	fs.StringVar(&f.chr, "char", "", "BLE 特征 UUID")
	fs.BoolVar(&f.readresp, "readresp", false, "BLE 写后读回")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	// 组装请求
	req, err := buildRequest(&f)
	if err != nil {
		fmt.Fprintln(stderr, "参数错误:", err)
		return 2
	}
	if f.jsonFile != "" {
		b, rerr := os.ReadFile(f.jsonFile)
		if rerr != nil {
			fmt.Fprintln(stderr, "读取 JSON 失败:", rerr)
			return 2
		}
		if err := json.Unmarshal(b, req); err != nil {
			fmt.Fprintln(stderr, "解析 JSON 失败:", err)
			return 2
		}
		if req.Payload == nil {
			// JSON 中可能用了 payloadHex/payloadTxt
		}
	}

	reg := registry.New()
	d, ok := reg.Get(req.Protocol)
	if !ok {
		fmt.Fprintf(stderr, "未知协议: %s (可用: %s)\n", req.Protocol, strings.Join(reg.Names(), ", "))
		return 2
	}
	if err := d.Validate(req); err != nil {
		fmt.Fprintln(stderr, "参数校验失败:", err)
		return 2
	}
	ctx := context.Background()
	resp, err := d.Send(ctx, req)
	if err != nil {
		fmt.Fprintln(stderr, "发送失败:", err)
		return 1
	}
	printResponse(stdout, resp)
	return 0
}

func buildRequest(f *sendFlags) (*core.Request, error) {
	req := &core.Request{Protocol: f.proto}
	if f.timeout != "" {
		d, err := time.ParseDuration(f.timeout)
		if err != nil {
			return nil, fmt.Errorf("timeout 解析失败: %w", err)
		}
		req.Timeout = d
	}
	req.PayloadHex = f.hex
	req.PayloadTxt = f.txt
	req.PayloadB64 = f.b64

	req.RemoteAddr = f.remote
	req.URL = f.url
	req.Method = f.method
	req.Headers = map[string]string{}
	for _, h := range f.headers {
		k, v, ok := strings.Cut(h, ":")
		if ok {
			req.Headers[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	req.Insecure = f.insecure

	req.Broker = f.broker
	req.ClientID = f.client
	req.Username = f.user
	req.Password = f.pass
	req.Topic = f.topic
	req.QoS = byte(f.qos)
	req.Retain = f.retain

	if f.funcCode != "" {
		fc, err := parseUint(f.funcCode, 16)
		if err != nil {
			return nil, fmt.Errorf("func 解析失败: %w", err)
		}
		req.ModbusFunc = byte(fc)
	}
	req.ModbusAddr = uint16(f.addr)
	req.ModbusQuantity = uint16(f.qty)
	if f.values != "" {
		for _, v := range strings.Split(f.values, ",") {
			n, err := parseUint(strings.TrimSpace(v), 10)
			if err != nil {
				return nil, fmt.Errorf("values 解析失败: %w", err)
			}
			req.ModbusValues = append(req.ModbusValues, uint16(n))
		}
	}
	req.ModbusUnitID = byte(f.unit)
	req.ModbusBaud = f.baud
	req.ModbusDataBits = f.databits
	req.ModbusParity = f.parity
	req.ModbusStopBits = f.stopbits

	req.SerialDevice = f.device
	req.SerialBaud = f.baud
	req.SerialDataBits = f.databits
	req.SerialParity = f.parity
	req.SerialStopBits = f.stopbits

	req.CANInterface = f.iface
	if f.canID != "" {
		id, err := parseUint(f.canID, 0)
		if err != nil {
			return nil, fmt.Errorf("id 解析失败: %w", err)
		}
		req.CANID = uint32(id)
	}
	req.CANExt = f.ext
	req.CANRTR = f.rtr

	req.SPIDevice = f.spiDevice
	req.SPIMode = uint8(f.mode)
	req.SPIBits = uint8(f.bits)
	req.SPISpeed = f.speed

	req.I2CBus = f.bus
	addr := f.i2cAddr
	if addr == 0 {
		addr = f.addr
	}
	req.I2CAddr = addr
	req.I2CRegister = f.reg
	if f.qty > 0 {
		req.ModbusQuantity = uint16(f.qty)
	}

	req.BLEAddress = f.bleAddr
	req.BLEService = f.svc
	req.BLEChar = f.chr
	req.BLEReadResp = f.readresp

	return req, nil
}

func parseUint(s string, base int) (uint64, error) {
	var v uint64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

func printResponse(w *os.File, resp *core.Response) {
	fmt.Fprintf(w, "协议: %s  状态: %s  耗时: %dms\n", resp.Protocol, resp.Status, resp.LatencyMS)
	if resp.Error != "" {
		fmt.Fprintln(w, "错误:", resp.Error)
	}
	if resp.DataTxt != "" {
		fmt.Fprintln(w, "响应文本:", resp.DataTxt)
	}
	if resp.DataHex != "" {
		fmt.Fprintln(w, "响应HEX:", resp.DataHex)
	}
	if len(resp.Meta) > 0 {
		b, _ := json.MarshalIndent(resp.Meta, "", "  ")
		fmt.Fprintln(w, "元信息:", string(b))
	}
}