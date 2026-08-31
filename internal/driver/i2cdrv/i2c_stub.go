//go:build !linux

// Package i2cdrv 在非 Linux 平台提供占位实现。
package i2cdrv

import (
	"context"
	"fmt"

	"minigreat-sender/internal/core"
)

// I2CDriver 非 Linux 平台占位。
type I2CDriver struct{}

// Name 返回协议名。
func (I2CDriver) Name() string { return "i2c" }

// Description 返回描述。
func (I2CDriver) Description() string { return "I2C 主设备 (仅 Linux /dev/i2c-N 支持)" }

// Validate 校验参数。
func (I2CDriver) Validate(req *core.Request) error {
	return fmt.Errorf("i2c: 仅 Linux 平台支持 /dev/i2c-N")
}

// Send 返回不支持错误。
func (I2CDriver) Send(ctx context.Context, req *core.Request) (*core.Response, error) {
	return nil, fmt.Errorf("i2c: 当前平台不支持, 请在 Linux 下运行")
}