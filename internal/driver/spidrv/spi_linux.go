// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
//go:build linux

// Package spidrv 实现 Linux SPI 主设备发送驱动 (/dev/spidevX.Y)。
package spidrv

import (
	"context"
	"fmt"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/FasterEdge/MiniGreat-Sender/internal/core"
)

// SPIDriver 实现 SPI 半双工写 + 读(全双工)。
type SPIDriver struct{}

// Name 返回协议名。
func (SPIDriver) Name() string { return "spi" }

// Description 返回描述。
func (SPIDriver) Description() string {
	return "SPI 主设备: /dev/spidevX.Y, 模式/频率/位宽可配, 全双工读写"
}

// Validate 校验参数。
func (SPIDriver) Validate(req *core.Request) error {
	if req.SPIDevice == "" {
		return fmt.Errorf("spi: spiDevice 不能为空 (如 /dev/spidev0.0)")
	}
	return nil
}

// SPI 相关 ioctl 常量 (linux/include/uapi/linux/spi/spidev.h)
const (
	spiIOCMagic    = 0x6B
	spiIocRdMode   = 0x80016B01
	spiIocWrMode   = 0x40016B01
	spiIocRdBits   = 0x80016B03
	spiIocWrBits   = 0x40016B03
	spiIocRdSpeed  = 0x80046B04
	spiIocWrSpeed  = 0x40046B04
	spiIocMessage1 = 0x40006B00
	spiIocMessage2 = 0x40086B00
)

type spiIocTransfer struct {
	TxBuf       uint64
	RxBuf       uint64
	Len         uint32
	SpeedHz     uint32
	DelayUsecs  uint16
	BitsPerWord uint8
	CsChange    uint8
	TxNBits     uint8
	RxNBits     uint8
	Pad         uint16
}

// Send 执行一次 SPI 传输: 写入 tx 同时读回 rx。
func (SPIDriver) Send(ctx context.Context, req *core.Request) (*core.Response, error) {
	tx, err := core.ResolvePayload(req)
	if err != nil {
		return nil, err
	}
	if len(tx) == 0 {
		return nil, fmt.Errorf("spi: 载荷不能为空")
	}
	mode := req.SPIMode
	bits := req.SPIBits
	if bits == 0 {
		bits = 8
	}
	speed := req.SPISpeed
	if speed == 0 {
		speed = 1000000 // 1MHz 默认
	}

	f, err := os.OpenFile(req.SPIDevice, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("spi: 打开设备失败: %w", err)
	}
	defer f.Close()

	fd := f.Fd()
	_ = fd
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, uintptr(spiIocWrMode), uintptr(unsafe.Pointer(&mode))); errno != 0 {
		return nil, fmt.Errorf("spi: 设置模式失败: %v", errno)
	}
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), uintptr(spiIocWrBits), uintptr(unsafe.Pointer(&bits))); errno != 0 {
		return nil, fmt.Errorf("spi: 设置位宽失败: %v", errno)
	}
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), uintptr(spiIocWrSpeed), uintptr(unsafe.Pointer(&speed))); errno != 0 {
		return nil, fmt.Errorf("spi: 设置频率失败: %v", errno)
	}

	rx := make([]byte, len(tx))
	tr := spiIocTransfer{
		TxBuf:       uint64(uintptr(unsafe.Pointer(&tx[0]))),
		RxBuf:       uint64(uintptr(unsafe.Pointer(&rx[0]))),
		Len:         uint32(len(tx)),
		SpeedHz:     uint32(speed),
		DelayUsecs:  0,
		BitsPerWord: bits,
	}
	start := time.Now()
	ptr := unsafe.Pointer(&tr)
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), uintptr(spiIocMessage1), uintptr(ptr)); errno != 0 {
		return nil, fmt.Errorf("spi: 传输失败: %v", errno)
	}
	latency := time.Since(start).Microseconds()
	_ = ctx

	resp := &core.Response{
		Protocol:  "spi",
		Status:    "ok",
		LatencyMS: latency / 1000,
		Data:      rx,
		DataHex:   core.FormatDataHex(rx),
		DataTxt:   core.FormatDataTxt(rx),
		Meta: map[string]any{
			"device":  req.SPIDevice,
			"mode":    mode,
			"bits":    bits,
			"speedHz": speed,
			"txLen":   len(tx),
			"rxLen":   len(rx),
		},
	}
	return resp, nil
}
