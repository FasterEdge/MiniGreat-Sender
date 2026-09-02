// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
package serialdrv

import (
	"context"
	"fmt"
	"time"

	"go.bug.st/serial"

	"github.com/FasterEdge/MiniGreat-Sender/internal/core"
)

// SerialDriver 实现串口(UART/RS232/RS485)发送。
type SerialDriver struct{}

// Name 返回协议名。
func (SerialDriver) Name() string { return "serial" }

// Description 返回描述。
func (SerialDriver) Description() string {
	return "串口(UART/RS232/RS485): 打开串口发送数据并读取响应"
}

// Validate 校验参数。
func (SerialDriver) Validate(req *core.Request) error {
	if req.SerialDevice == "" {
		return fmt.Errorf("serial: serialDevice 不能为空 (如 /dev/ttyUSB0)")
	}
	return nil
}

// Send 执行一次串口发送。
func (SerialDriver) Send(ctx context.Context, req *core.Request) (*core.Response, error) {
	payload, err := core.ResolvePayload(req)
	if err != nil {
		return nil, err
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	baud := req.SerialBaud
	if baud == 0 {
		baud = 115200
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
	case "M", "m":
		parity = serial.MarkParity
	case "S", "s":
		parity = serial.SpaceParity
	}

	mode := &serial.Mode{
		BaudRate: baud,
		DataBits: databits,
		Parity:   parity,
		StopBits: serial.StopBits(stopbits),
	}
	port, err := serial.Open(req.SerialDevice, mode)
	if err != nil {
		return nil, fmt.Errorf("serial: 打开串口失败: %w", err)
	}
	defer port.Close()
	_ = port.SetReadTimeout(timeout)

	start := time.Now()
	resp := &core.Response{Protocol: "serial", Status: "ok", LatencyMS: 0}
	if len(payload) > 0 {
		if _, werr := port.Write(payload); werr != nil {
			resp.Status = "error"
			resp.Error = werr.Error()
			return resp, nil
		}
	}
	buf := make([]byte, 4096)
	var total []byte
	for {
		n, rerr := port.Read(buf)
		if n > 0 {
			total = append(total, buf[:n]...)
			if len(total) >= 1<<20 {
				break
			}
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
		resp.DataTxt = core.FormatDataTxt(total)
	} else {
		resp.DataTxt = "(已发送, 无响应)"
	}
	_ = ctx
	return resp, nil
}
