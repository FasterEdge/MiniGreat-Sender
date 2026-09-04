// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
//go:build linux

// Package i2cdrv 实现 Linux I2C 主设备读写驱动 (/dev/i2c-N)。
package i2cdrv

import (
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"

	"github.com/FasterEdge/MiniGreat-Sender/internal/core"
)

// I2CDriver 实现 I2C 从机读写。
type I2CDriver struct{}

// Name 返回协议名。
func (I2CDriver) Name() string { return "i2c" }

// Description 返回描述。
func (I2CDriver) Description() string {
	return "I2C 主设备: /dev/i2c-N, 指定从机地址读写(支持寄存器寻址)"
}

// Validate 校验参数。
func (I2CDriver) Validate(req *core.Request) error {
	if req.I2CBus < 0 {
		return fmt.Errorf("i2c: i2cBus 必须 >= 0 (如 1 对应 /dev/i2c-1)")
	}
	if req.I2CAddr <= 0 || req.I2CAddr > 0x7F {
		return fmt.Errorf("i2c: i2cAddr 必须在 1~127 之间(7位地址)")
	}
	return nil
}

// ioctl 常量
const (
	i2cSlave  = 0x0703
	i2cFunCs  = 0x0705
	i2cRDFunc = 0x0001
	i2cWrFunc = 0x0002
)

// Send 执行 I2C 写(可选带寄存器地址), 并读回(若 Quantity>0)。
// 规则:
//   - I2CRegister >= 0 时: 先写寄存器地址, 再写载荷; 若 Quantity>0 则从寄存器读 Quantity 字节。
//   - I2CRegister < 0 时: 载荷直接作为写数据; Quantity>0 时直接读。
func (I2CDriver) Send(ctx context.Context, req *core.Request) (*core.Response, error) {
	payload, err := core.ResolvePayload(req)
	if err != nil {
		return nil, err
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	// 单次读上限提前校验: I2C 单次传输受总线限制 (~32KB), 打开设备前
	// 拒绝超大 Quantity — 防用户可控参数造成超大一次性分配 (内存耗尽
	// DoS), 且避免无谓的设备打开。
	if req.ModbusQuantity > 4096 {
		return nil, fmt.Errorf("i2c: 单次读取上限 4096 字节, 当前 %d", req.ModbusQuantity)
	}

	dev := fmt.Sprintf("/dev/i2c-%d", req.I2CBus)
	f, err := os.OpenFile(dev, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("i2c: 打开 %s 失败: %w", dev, err)
	}
	defer f.Close()

	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), uintptr(i2cSlave), uintptr(req.I2CAddr)); errno != 0 {
		return nil, fmt.Errorf("i2c: 设置从机地址 0x%02X 失败: %v", req.I2CAddr, errno)
	}
	_ = timeout
	start := time.Now()
	resp := &core.Response{Protocol: "i2c", Status: "ok", LatencyMS: 0}

	var writeBuf []byte
	if req.I2CRegister >= 0 {
		writeBuf = append(writeBuf, byte(req.I2CRegister))
	}
	writeBuf = append(writeBuf, payload...)

	if len(writeBuf) > 0 {
		if _, werr := f.Write(writeBuf); werr != nil {
			resp.Status = "error"
			resp.Error = werr.Error()
			resp.LatencyMS = time.Since(start).Milliseconds()
			return resp, nil
		}
	}

	var rx []byte
	if req.ModbusQuantity > 0 { // 复用 ModbusQuantity 字段作为读取字节数
		rx = make([]byte, req.ModbusQuantity)
		n, rerr := f.Read(rx)
		if rerr != nil && n == 0 {
			resp.Status = "error"
			resp.Error = rerr.Error()
			resp.LatencyMS = time.Since(start).Milliseconds()
			return resp, nil
		}
		rx = rx[:n]
	}
	resp.LatencyMS = time.Since(start).Milliseconds()
	resp.Data = rx
	resp.DataHex = core.FormatDataHex(rx)
	resp.DataTxt = core.FormatDataTxt(rx)
	resp.Meta = map[string]any{
		"bus":      req.I2CBus,
		"addr":     fmt.Sprintf("0x%02X", req.I2CAddr),
		"register": req.I2CRegister,
		"written":  len(writeBuf),
		"read":     len(rx),
	}
	if len(rx) == 0 {
		resp.DataTxt = "(已写入, 未读)"
	}
	_ = ctx
	return resp, nil
}
