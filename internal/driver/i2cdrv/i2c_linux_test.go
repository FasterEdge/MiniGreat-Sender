// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
//go:build linux

package i2cdrv

import (
	"context"
	"strings"
	"testing"

	"github.com/FasterEdge/MiniGreat-Sender/internal/core"
)

// TestSendQuantityLimit: 超大 ModbusQuantity 必须在打开设备前被拒绝
// (回归: 修复前 make([]byte, Quantity) 用户可控无上限分配)。
// 无需真实 /dev/i2c-N — 校验发生在打开设备之前。
func TestSendQuantityLimit(t *testing.T) {
	drv := I2CDriver{}
	_, err := drv.Send(context.Background(), &core.Request{
		I2CBus:        1,
		I2CAddr:       0x50,
		ModbusQuantity: 4097,
	})
	if err == nil {
		t.Fatal("expected quantity limit error, got nil")
	}
	if !strings.Contains(err.Error(), "4096") {
		t.Fatalf("error = %q, want mention of 4096 limit", err)
	}

	// 上限以内 (4096) 且无设备: 应报设备打开错误而不是上限错误 —
	// 证明合法数量没有被误拒 (设备缺失是环境问题)。
	_, err = drv.Send(context.Background(), &core.Request{
		I2CBus:        1,
		I2CAddr:       0x50,
		ModbusQuantity: 4096,
	})
	if err == nil {
		t.Fatal("expected device open error (no /dev/i2c-1 in test env), got nil")
	}
	if strings.Contains(err.Error(), "4096") {
		t.Fatalf("in-range quantity rejected: %v", err)
	}
}