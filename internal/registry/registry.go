// Package registry 汇总注册全部发送协议驱动。
package registry

import (
	"minigreat-sender/internal/core"
	"minigreat-sender/internal/driver/bledrv"
	"minigreat-sender/internal/driver/candrv"
	"minigreat-sender/internal/driver/httpdrv"
	"minigreat-sender/internal/driver/i2cdrv"
	"minigreat-sender/internal/driver/modbusdrv"
	"minigreat-sender/internal/driver/mqttdrv"
	"minigreat-sender/internal/driver/netdrv"
	"minigreat-sender/internal/driver/rfdrv"
	"minigreat-sender/internal/driver/serialdrv"
	"minigreat-sender/internal/driver/spidrv"
	"minigreat-sender/internal/driver/wsdrv"
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