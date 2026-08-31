//go:build !linux

// Package candrv 在非 Linux 平台提供占位实现。
package candrv

import (
	"context"
	"fmt"

	"minigreat-sender/internal/core"
)

// CANDriver 非 Linux 平台占位。
type CANDriver struct{}

// Name 返回协议名。
func (CANDriver) Name() string { return "can" }

// Description 返回描述。
func (CANDriver) Description() string { return "CAN 总线 (仅 Linux SocketCAN 支持)" }

// Validate 校验参数。
func (CANDriver) Validate(req *core.Request) error {
	return fmt.Errorf("can: 仅 Linux 平台支持 SocketCAN")
}

// Send 返回不支持错误。
func (CANDriver) Send(ctx context.Context, req *core.Request) (*core.Response, error) {
	return nil, fmt.Errorf("can: 当前平台不支持 SocketCAN, 请在 Linux 下运行")
}