// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
// Package core 定义 MiniGreat-Sender 的核心抽象：
// 协议驱动接口(Driver)、请求(Request)、响应(Response)与协议注册表。
package core

import (
	"context"
	"time"
)

// Request 是一次发送请求的统一描述。
// 不同协议只取用自己关心的字段, 未使用字段忽略。
type Request struct {
	// 协议名, 例如 tcp/udp/http/ws/mqtt/modbus/serial/ble/can/spi/i2c/rf
	Protocol string `json:"protocol" yaml:"protocol"`

	// ---- 通用 ----
	Timeout    time.Duration `json:"timeout" yaml:"timeout"`         // 超时(如 5s)
	Payload    []byte        `json:"payload,omitempty" yaml:"-"`     // 原始载荷(已解码)
	PayloadHex string        `json:"payloadHex,omitempty" yaml:"payloadHex"` // 十六进制文本
	PayloadTxt string        `json:"payloadTxt,omitempty" yaml:"payloadTxt"` // 普通文本
	PayloadB64 string        `json:"payloadB64,omitempty" yaml:"payloadB64"` // base64 文本

	// ---- 网络: tcp/udp ----
	RemoteAddr string `json:"remoteAddr,omitempty" yaml:"remoteAddr"` // host:port

	// ---- http/ws ----
	URL     string            `json:"url,omitempty" yaml:"url"`
	Method  string            `json:"method,omitempty" yaml:"method"` // GET/POST/...
	Headers map[string]string `json:"headers,omitempty" yaml:"headers"`
	Insecure bool             `json:"insecure,omitempty" yaml:"insecure"` // 跳过TLS校验

	// ---- mqtt ----
	Broker   string `json:"broker,omitempty" yaml:"broker"` // tcp://host:port 或 ssl://...
	ClientID string `json:"clientId,omitempty" yaml:"clientId"`
	Username string `json:"username,omitempty" yaml:"username"`
	Password string `json:"password,omitempty" yaml:"password"`
	Topic    string `json:"topic,omitempty" yaml:"topic"`
	QoS      byte   `json:"qos,omitempty" yaml:"qos"`
	Retain   bool   `json:"retain,omitempty" yaml:"retain"`

	// ---- modbus ----
	ModbusUnitID    byte   `json:"modbusUnitId,omitempty" yaml:"modbusUnitId"`
	ModbusFunc      byte   `json:"modbusFunc,omitempty" yaml:"modbusFunc"` // 01/02/03/04/05/06/0F/10
	ModbusAddr      uint16 `json:"modbusAddr,omitempty" yaml:"modbusAddr"`
	ModbusQuantity  uint16 `json:"modbusQuantity,omitempty" yaml:"modbusQuantity"`
	ModbusValues    []uint16 `json:"modbusValues,omitempty" yaml:"modbusValues"`
	ModbusBaud      int    `json:"modbusBaud,omitempty" yaml:"modbusBaud"`           // RTU模式波特率
	ModbusDataBits  int    `json:"modbusDataBits,omitempty" yaml:"modbusDataBits"`
	ModbusParity    string `json:"modbusParity,omitempty" yaml:"modbusParity"`       // N/E/O
	ModbusStopBits  int    `json:"modbusStopBits,omitempty" yaml:"modbusStopBits"`

	// ---- serial ----
	SerialDevice string `json:"serialDevice,omitempty" yaml:"serialDevice"` // /dev/ttyUSB0
	SerialBaud   int    `json:"serialBaud,omitempty" yaml:"serialBaud"`
	SerialDataBits int  `json:"serialDataBits,omitempty" yaml:"serialDataBits"`
	SerialParity string `json:"serialParity,omitempty" yaml:"serialParity"`
	SerialStopBits int  `json:"serialStopBits,omitempty" yaml:"serialStopBits"`

	// ---- ble ----
	BLEAddress  string `json:"bleAddress,omitempty" yaml:"bleAddress"` // 目标MAC, 为空则扫描选取
	BLEService  string `json:"bleService,omitempty" yaml:"bleService"` // 服务UUID
	BLEChar     string `json:"bleChar,omitempty" yaml:"bleChar"`       // 特征UUID
	BLEReadResp bool   `json:"bleReadResp,omitempty" yaml:"bleReadResp"` // 写后是否读回

	// ---- can ----
	CANInterface string `json:"canInterface,omitempty" yaml:"canInterface"` // can0/vcan0
	CANID        uint32 `json:"canId,omitempty" yaml:"canId"`
	CANExt       bool   `json:"canExt,omitempty" yaml:"canExt"` // 扩展帧
	CANRTR       bool   `json:"canRtr,omitempty" yaml:"canRtr"` // 远程帧

	// ---- spi ----
	SPIDevice string `json:"spiDevice,omitempty" yaml:"spiDevice"` // /dev/spidev0.0
	SPIMode   uint8  `json:"spiMode,omitempty" yaml:"spiMode"`
	SPIBits   uint8  `json:"spiBits,omitempty" yaml:"spiBits"`
	SPISpeed  int64  `json:"spiSpeed,omitempty" yaml:"spiSpeed"`

	// ---- i2c ----
	I2CBus    int    `json:"i2cBus,omitempty" yaml:"i2cBus"`       // /dev/i2c-N 的 N
	I2CAddr   int    `json:"i2cAddr,omitempty" yaml:"i2cAddr"`     // 7位从机地址
	I2CRegister int  `json:"i2cRegister,omitempty" yaml:"i2cRegister"` // -1 表示不写寄存器
}

// Response 是一次发送的完整结果。
type Response struct {
	Protocol string `json:"protocol"`
	Status   string `json:"status"` // ok / error / timeout
	LatencyMS int64 `json:"latencyMs"`
	Data     []byte `json:"data,omitempty"`
	DataHex  string `json:"dataHex,omitempty"`
	DataTxt  string `json:"dataTxt,omitempty"`
	Error    string `json:"error,omitempty"`
	// Meta 存放协议附加信息, 如 mqtt packet id / modbus 响应寄存器值 / ble 设备名
	Meta map[string]any `json:"meta,omitempty"`
}

// Driver 是所有协议发送驱动的统一接口。
type Driver interface {
	// Name 返回协议名(与 Request.Protocol 对应, 小写)。
	Name() string
	// Description 返回一句话描述。
	Description() string
	// Validate 在发送前校验参数, 返回错误信息。
	Validate(req *Request) error
	// Send 执行一次发送并返回响应。ctx 用于超时与取消。
	Send(ctx context.Context, req *Request) (*Response, error)
}

// Registry 协议注册表。
type Registry struct {
	drivers map[string]Driver
}

// NewRegistry 创建空注册表。
func NewRegistry() *Registry {
	return &Registry{drivers: make(map[string]Driver)}
}

// Register 注册一个驱动。
func (r *Registry) Register(d Driver) {
	r.drivers[d.Name()] = d
}

// Get 按名获取驱动。
func (r *Registry) Get(name string) (Driver, bool) {
	d, ok := r.drivers[name]
	return d, ok
}

// Names 返回全部已注册协议名。
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.drivers))
	for n := range r.drivers {
		out = append(out, n)
	}
	return out
}