// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
// Package registry 汇总注册全部发送协议驱动。
package registry

import (
	"github.com/FasterEdge/MiniGreat-Sender/internal/core"
	"github.com/FasterEdge/MiniGreat-Sender/internal/driver/bledrv"
	"github.com/FasterEdge/MiniGreat-Sender/internal/driver/candrv"
	"github.com/FasterEdge/MiniGreat-Sender/internal/driver/httpdrv"
	"github.com/FasterEdge/MiniGreat-Sender/internal/driver/i2cdrv"
	"github.com/FasterEdge/MiniGreat-Sender/internal/driver/modbusdrv"
	"github.com/FasterEdge/MiniGreat-Sender/internal/driver/mqttdrv"
	"github.com/FasterEdge/MiniGreat-Sender/internal/driver/netdrv"
	"github.com/FasterEdge/MiniGreat-Sender/internal/driver/rfdrv"
	"github.com/FasterEdge/MiniGreat-Sender/internal/driver/serialdrv"
	"github.com/FasterEdge/MiniGreat-Sender/internal/driver/spidrv"
	"github.com/FasterEdge/MiniGreat-Sender/internal/driver/wsdrv"
)

// New 创建并注册全部驱动。
func New() *core.Registry {
	r := core.NewRegistry()
	drivers := []core.Driver{
		netdrv.TCPDriver{},
		netdrv.UDPDriver{},
		httpdrv.HTTPDriver{},
		wsdrv.WSDriver{},
		mqttdrv.MQTTDriver{},
		modbusdrv.ModbusDriver{},
		serialdrv.SerialDriver{},
		rfdrv.RFDriver{},
		candrv.CANDriver{},
		spidrv.SPIDriver{},
		i2cdrv.I2CDriver{},
		bledrv.BLEDriver{},
	}
	for _, d := range drivers {
		r.Register(d)
	}
	return r
}
