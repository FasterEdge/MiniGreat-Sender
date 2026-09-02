// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
package rfdrv

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.bug.st/serial"

	"github.com/FasterEdge/MiniGreat-Sender/internal/core"
)

// RFDriver 实现射频模块(LoRa/433MHz/Zigbee/蓝牙串口透传等)经串口 AT 指令控制。
// 典型用法: 打开模块串口, 发送 AT 指令或透传数据, 读取模块返回/接收数据。
type RFDriver struct{}

// Name 返回协议名。
func (RFDriver) Name() string { return "rf" }

// Description 返回描述。
func (RFDriver) Description() string {
	return "射频模块透传(LoRa/433MHz/Zigbee/BLE-SPP): 经串口发 AT 指令或透传数据"
}

// Validate 校验参数。
func (RFDriver) Validate(req *core.Request) error {
	if req.SerialDevice == "" {
		return fmt.Errorf("rf: serialDevice 不能为空 (如 /dev/ttyUSB0 接射频模块)")
	}
	return nil
}

// Send 执行一次射频模块操作。
// 载荷优先按 AT 指令发送; 也可透传原始字节。
func (RFDriver) Send(ctx context.Context, req *core.Request) (*core.Response, error) {
	payload, err := core.ResolvePayload(req)
	if err != nil {
		return nil, err
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	baud := req.SerialBaud
	if baud == 0 {
		baud = 9600
	}
	databits := req.SerialDataBits
	if databits == 0 {
		databits = 8
	}
	stopbits := req.SerialStopBits
	if stopbits == 0 {
		stopbits = 1
	}
	parity := serial.NoParity
	switch req.SerialParity {
	case "E", "e":
		parity = serial.EvenParity
	case "O", "o":
		parity = serial.OddParity
	}

	mode := &serial.Mode{
		BaudRate: baud,
		DataBits: databits,
		Parity:   parity,
		StopBits: serial.StopBits(stopbits),
	}
	port, err := serial.Open(req.SerialDevice, mode)
	if err != nil {
		return nil, fmt.Errorf("rf: 打开串口失败: %w", err)
	}
	defer port.Close()
	_ = port.SetReadTimeout(timeout)

	start := time.Now()
	resp := &core.Response{Protocol: "rf", Status: "ok", LatencyMS: 0}

	// 默认载荷已含 \r\n 则直接发送; 否则追加 \r\n (常见 AT 行结尾)
	data := payload
	if len(data) > 0 && !strings.HasSuffix(strings.ToUpper(string(data)), "\r\n") {
		data = append(data, '\r', '\n')
	}
	if len(data) > 0 {
		if _, werr := port.Write(data); werr != nil {
			resp.Status = "error"
			resp.Error = werr.Error()
			return resp, nil
		}
	}

	// 读取模块返回/下行数据(直到超时或模块静默)
	buf := make([]byte, 4096)
	var total []byte
	for {
		n, rerr := port.Read(buf)
		if n > 0 {
			total = append(total, buf[:n]...)
			if len(total) >= 1<<20 {
				break
			}
			// 读到后再等一小段, 收集连续返回
			continue
		}
		if rerr != nil {
			break
		}
		if time.Since(start) > timeout {
			break
		}
	}
	resp.LatencyMS = time.Since(start).Milliseconds()
	if len(total) > 0 {
		resp.Data = total
		resp.DataHex = core.FormatDataHex(total)
		resp.DataTxt = strings.TrimRight(string(total), "\r\n\x00")
	} else {
		resp.DataTxt = "(已发送, 无返回)"
	}
	_ = ctx
	return resp, nil
}
