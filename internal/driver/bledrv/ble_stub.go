//go:build !linux

// Package bledrv 在非 Linux 平台提供占位实现。
package bledrv

import (
	"context"
	"fmt"

	"minigreat-sender/internal/core"
)

// BLEDriver 非 Linux 平台占位。
type BLEDriver struct{}

// Name 返回协议名。
func (BLEDriver) Name() string { return "ble" }

// Description 返回描述。
func (BLEDriver) Description() string { return "BLE 蓝牙 (仅 Linux BlueZ 支持)" }

// Validate 校验参数。
func (BLEDriver) Validate(req *core.Request) error {
	return fmt.Errorf("ble: 仅 Linux 平台支持 BlueZ")
}

// Send 返回不支持错误。
func (BLEDriver) Send(ctx context.Context, req *core.Request) (*core.Response, error) {
	return nil, fmt.Errorf("ble: 当前平台不支持, 请在 Linux 下运行")
}