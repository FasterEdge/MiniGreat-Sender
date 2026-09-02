package modbusdrv

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/goburrow/modbus"

	"github.com/FasterEdge/MiniGreat-Sender/internal/core"
)

// ModbusDriver 实现 Modbus RTU 与 TCP 客户端。
type ModbusDriver struct{}

// Name 返回协议名。
func (ModbusDriver) Name() string { return "modbus" }

// Description 返回描述。
func (ModbusDriver) Description() string {
	return "Modbus RTU/TCP 客户端: 01/02/03/04/05/06/0F/10 功能码读写"
}

// Validate 校验参数。
func (ModbusDriver) Validate(req *core.Request) error {
	if req.ModbusFunc == 0 {
		return fmt.Errorf("modbus: modbusFunc 不能为空")
	}
	if req.RemoteAddr == "" && req.SerialDevice == "" {
		return fmt.Errorf("modbus: 需要 remoteAddr(TCP) 或 serialDevice(RTU)")
	}
	return nil
}

// Send 执行一次 Modbus 操作。
func (ModbusDriver) Send(ctx context.Context, req *core.Request) (*core.Response, error) {
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	var client modbus.Client
	var transport string
	var closer interface{ Close() error }

	if req.SerialDevice != "" {
		transport = "rtu"
		baud := req.ModbusBaud
		if baud == 0 {
			baud = 9600
		}
		databits := req.ModbusDataBits
		if databits == 0 {
			databits = 8
		}
		stopbits := req.ModbusStopBits
		if stopbits == 0 {
			stopbits = 1
		}
		parity := req.ModbusParity
		if parity == "" {
			parity = "N"
		}
		h := modbus.NewRTUClientHandler(req.SerialDevice)
		h.BaudRate = baud
		h.DataBits = databits
		h.Parity = parity
		h.StopBits = stopbits
		h.SlaveId = req.ModbusUnitID
		h.Timeout = timeout
		h.Logger = nil
		if err := h.Connect(); err != nil {
			return nil, fmt.Errorf("modbus(rtu): 连接 %s 失败: %w", req.SerialDevice, err)
		}
		client = modbus.NewClient(h)
		closer = h
	} else {
		transport = "tcp"
		h := modbus.NewTCPClientHandler(req.RemoteAddr)
		h.SlaveId = req.ModbusUnitID
		h.Timeout = timeout
		h.Logger = nil
		if err := h.Connect(); err != nil {
			return nil, fmt.Errorf("modbus(tcp): 连接 %s 失败: %w", req.RemoteAddr, err)
		}
		client = modbus.NewClient(h)
		closer = h
	}
	if closer != nil {
		defer closer.Close() // #nosec G104
	}

	start := time.Now()
	var data []byte
	var err error
	switch req.ModbusFunc {
	case 0x01:
		data, err = client.ReadCoils(req.ModbusAddr, req.ModbusQuantity)
	case 0x02:
		data, err = client.ReadDiscreteInputs(req.ModbusAddr, req.ModbusQuantity)
	case 0x03:
		data, err = client.ReadHoldingRegisters(req.ModbusAddr, req.ModbusQuantity)
	case 0x04:
		data, err = client.ReadInputRegisters(req.ModbusAddr, req.ModbusQuantity)
	case 0x05:
		if len(req.ModbusValues) == 0 {
			return nil, fmt.Errorf("modbus: 功能码05需要 values[0]")
		}
		data, err = client.WriteSingleCoil(req.ModbusAddr, req.ModbusValues[0])
	case 0x06:
		if len(req.ModbusValues) == 0 {
			return nil, fmt.Errorf("modbus: 功能码06需要 values[0]")
		}
		data, err = client.WriteSingleRegister(req.ModbusAddr, req.ModbusValues[0])
	case 0x0F:
		// 写多个线圈: values 中的每个 uint16 低 8 位 -> 一个线圈字节
		buf := coilsToBytes(req.ModbusValues)
		data, err = client.WriteMultipleCoils(req.ModbusAddr, uint16(len(buf)*8), buf)
	case 0x10:
		buf := regsToBytes(req.ModbusValues)
		data, err = client.WriteMultipleRegisters(req.ModbusAddr, uint16(len(req.ModbusValues)), buf)
	default:
		return nil, fmt.Errorf("modbus: 不支持的功能码 0x%02X", req.ModbusFunc)
	}
	latency := time.Since(start).Milliseconds()
	_ = ctx

	resp := &core.Response{Protocol: "modbus", Status: "ok", LatencyMS: latency}
	if err != nil {
		resp.Status = "error"
		resp.Error = err.Error()
		resp.Meta = map[string]any{"transport": transport, "func": fmt.Sprintf("0x%02X", req.ModbusFunc)}
		return resp, nil
	}
	resp.Data = data
	resp.DataHex = core.FormatDataHex(data)
	resp.DataTxt = core.FormatDataTxt(data)
	resp.Meta = map[string]any{
		"transport": transport,
		"func":      fmt.Sprintf("0x%02X", req.ModbusFunc),
		"bytes":     len(data),
	}
	return resp, nil
}

// coilsToBytes 将 []uint16 中的每个值的低 8 位按位打包成线圈字节数组。
func coilsToBytes(values []uint16) []byte {
	if len(values) == 0 {
		return nil
	}
	// 每个 uint16 表示 1 个字节的线圈状态(8个线圈)
	out := make([]byte, len(values))
	for i, v := range values {
		out[i] = byte(v)
	}
	return out
}

// regsToBytes 将 []uint16 转为大端 []byte (Modbus 寄存器 2 字节/个)。
func regsToBytes(values []uint16) []byte {
	out := make([]byte, 0, len(values)*2)
	for _, v := range values {
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, v)
		out = append(out, buf...)
	}
	return out
}
