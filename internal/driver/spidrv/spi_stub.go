// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
//go:build !linux

// Package spidrv 在非 Linux 平台提供占位实现。
package spidrv

import (
	"context"
	"fmt"

	"github.com/FasterEdge/MiniGreat-Sender/internal/core"
)

// SPIDriver 非 Linux 平台占位。
type SPIDriver struct{}

// Name 返回协议名。
func (SPIDriver) Name() string { return "spi" }

// Description 返回描述。
func (SPIDriver) Description() string { return "SPI 主设备 (仅 Linux /dev/spidev 支持)" }

// Validate 校验参数。
func (SPIDriver) Validate(req *core.Request) error {
	return fmt.Errorf("spi: 仅 Linux 平台支持 /dev/spidev")
}

// Send 返回不支持错误。
func (SPIDriver) Send(ctx context.Context, req *core.Request) (*core.Response, error) {
	return nil, fmt.Errorf("spi: 当前平台不支持, 请在 Linux 下运行")
}
