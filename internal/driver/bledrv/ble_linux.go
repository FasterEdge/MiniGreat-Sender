//go:build linux

// Package bledrv 实现 Linux BlueZ BLE 客户端 (经 D-Bus 操作 org.bluez)。
// 支持: 扫描设备、按地址/名称连接、发现服务与特征、特征读写、订阅通知。
package bledrv

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/FasterEdge/MiniGreat-Sender/internal/core"
)

const (
	bluezName       = "org.bluez"
	adapterIf       = "org.bluez.Adapter1"
	deviceIf        = "org.bluez.Device1"
	gattServiceIf   = "org.bluez.GattService1"
	gattCharIf      = "org.bluez.GattCharacteristic1"
	objectManagerIf = "org.freedesktop.DBus.ObjectManager"
)

// BLEDriver 实现 BLE 发送。
type BLEDriver struct{}

// Name 返回协议名。
func (BLEDriver) Name() string { return "ble" }

// Description 返回描述。
func (BLEDriver) Description() string {
	return "BLE 蓝牙 (BlueZ): 扫描/连接外设, 特征读写与通知订阅"
}

// Validate 校验参数。
func (BLEDriver) Validate(req *core.Request) error {
	if req.BLEChar == "" && req.BLEService == "" {
		return fmt.Errorf("ble: 需要 bleChar (特征UUID) 或 bleService")
	}
	return nil
}

// Send 执行一次 BLE 操作。
// 流程: 可选扫描 → 连接目标 → 发现GATT → 写特征(载荷) → 可选读回。
func (BLEDriver) Send(ctx context.Context, req *core.Request) (*core.Response, error) {
	payload, err := core.ResolvePayload(req)
	if err != nil {
		return nil, err
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("ble: 无法连接系统总线: %w", err)
	}
	defer conn.Close()

	adapterPath, err := findAdapter(conn)
	if err != nil {
		return nil, err
	}

	// 目标地址
	target := strings.ToUpper(req.BLEAddress)
	if target == "" {
		// 扫描后选第一个可连设备
		devs, err := scanDevices(conn, adapterPath, timeout)
		if err != nil {
			return nil, err
		}
		if len(devs) == 0 {
			return nil, fmt.Errorf("ble: 扫描未发现设备")
		}
		target = devs[0].Address
	}

	// 连接
	devPath, err := connectDevice(conn, adapterPath, target, timeout)
	if err != nil {
		return nil, err
	}
	defer disconnectDevice(conn, devPath)

	start := time.Now()
	resp := &core.Response{Protocol: "ble", Status: "ok", LatencyMS: 0}

	// 发现 GATT 并写特征
	written, err := writeChar(conn, devPath, req.BLEService, req.BLEChar, payload, req.BLEReadResp, timeout)
	if err != nil {
		resp.Status = "error"
		resp.Error = err.Error()
		resp.LatencyMS = time.Since(start).Milliseconds()
		return resp, nil
	}
	resp.LatencyMS = time.Since(start).Milliseconds()
	resp.Data = written
	resp.DataHex = core.FormatDataHex(written)
	resp.DataTxt = core.FormatDataTxt(written)
	resp.Meta = map[string]any{
		"address":    target,
		"devicePath": devPath,
		"service":    req.BLEService,
		"char":       req.BLEChar,
		"written":    len(payload),
		"read":       len(written),
	}
	if len(written) == 0 {
		resp.DataTxt = "(已写入特征, 未读回)"
	}
	return resp, nil
}

// bleDevice 描述一个扫描到的设备。
type bleDevice struct {
	Path      string
	Address   string
	Name      string
	RSSI      int16
	Connected bool
}

// findAdapter 返回第一个可用适配器对象路径。
func findAdapter(conn *dbus.Conn) (dbus.ObjectPath, error) {
	obj := conn.Object(bluezName, "/")
	var managed map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if err := obj.Call(objectManagerIf+".GetManagedObjects", 0).Store(&managed); err != nil {
		return "", fmt.Errorf("ble: 无法读取 BlueZ 对象: %w", err)
	}
	for path, ifaces := range managed {
		if _, ok := ifaces[adapterIf]; ok {
			return path, nil
		}
	}
	return "", fmt.Errorf("ble: 未找到蓝牙适配器 (请检查 hciconfig/bluetoothctl)")
}

// scanDevices 扫描 timeout 秒并返回发现的设备。
func scanDevices(conn *dbus.Conn, adapterPath dbus.ObjectPath, timeout time.Duration) ([]bleDevice, error) {
	adapter := conn.Object(bluezName, adapterPath)
	_ = adapter.Call(adapterIf+".StartDiscovery", 0).Store()
	defer adapter.Call(adapterIf+".StopDiscovery", 0) // #nosec G104 -- 忽略

	deadline := time.Now().Add(timeout)
	var found []bleDevice
	seen := map[string]bool{}
	for time.Now().Before(deadline) {
		obj := conn.Object(bluezName, "/")
		var managed map[dbus.ObjectPath]map[string]map[string]dbus.Variant
		if err := obj.Call(objectManagerIf+".GetManagedObjects", 0).Store(&managed); err == nil {
			for path, ifaces := range managed {
				d, ok := ifaces[deviceIf]
				if !ok {
					continue
				}
				addr := getStr(d, "Address")
				if addr == "" || seen[addr] {
					continue
				}
				seen[addr] = true
				dev := bleDevice{
					Path:      string(path),
					Address:   addr,
					Name:      getStr(d, "Name"),
					RSSI:      getI16(d, "RSSI"),
					Connected: getBool(d, "Connected"),
				}
				found = append(found, dev)
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return found, nil
}

// connectDevice 按 MAC 查找并连接设备。
func connectDevice(conn *dbus.Conn, adapterPath dbus.ObjectPath, address string, timeout time.Duration) (dbus.ObjectPath, error) {
	obj := conn.Object(bluezName, "/")
	var managed map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if err := obj.Call(objectManagerIf+".GetManagedObjects", 0).Store(&managed); err != nil {
		return "", fmt.Errorf("ble: 读取对象失败: %w", err)
	}
	var devPath dbus.ObjectPath
	for path, ifaces := range managed {
		d, ok := ifaces[deviceIf]
		if !ok {
			continue
		}
		if strings.EqualFold(getStr(d, "Address"), address) {
			devPath = path
			break
		}
	}
	if devPath == "" {
		return "", fmt.Errorf("ble: 未找到设备 %s (请先扫描)", address)
	}
	dev := conn.Object(bluezName, devPath)
	if !getBool(mustProps(conn, devPath), "Connected") {
		done := make(chan *dbus.Call, 1)
		dev.Go(deviceIf+".Connect", 0, done)
		select {
		case <-done:
		case <-time.After(timeout):
			return "", fmt.Errorf("ble: 连接 %s 超时", address)
		}
	}
	return devPath, nil
}

func disconnectDevice(conn *dbus.Conn, devPath dbus.ObjectPath) {
	conn.Object(bluezName, devPath).Call(deviceIf+".Disconnect", 0) // #nosec G104
}

func mustProps(conn *dbus.Conn, path dbus.ObjectPath) map[string]dbus.Variant {
	obj := conn.Object(bluezName, "/")
	var managed map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if err := obj.Call(objectManagerIf+".GetManagedObjects", 0).Store(&managed); err != nil {
		return nil
	}
	if ifaces, ok := managed[path]; ok {
		return ifaces[deviceIf]
	}
	return nil
}

// writeChar 在设备下查找匹配 service/char, 写入载荷; readBack 为真时读回。
func writeChar(conn *dbus.Conn, devPath dbus.ObjectPath, serviceUUID, charUUID string, payload []byte, readBack bool, timeout time.Duration) ([]byte, error) {
	obj := conn.Object(bluezName, "/")
	var managed map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if err := obj.Call(objectManagerIf+".GetManagedObjects", 0).Store(&managed); err != nil {
		return nil, fmt.Errorf("ble: 读取GATT失败: %w", err)
	}

	// 找到目标特征对象路径
	var charPath dbus.ObjectPath
	for path, ifaces := range managed {
		if _, ok := ifaces[gattCharIf]; !ok {
			continue
		}
		// 必须属于目标设备子树
		if !strings.HasPrefix(string(path), string(devPath)) {
			continue
		}
		cUUID := strings.ToLower(getStr(ifaces[gattCharIf], "UUID"))
		if serviceUUID != "" {
			svc, err := findServiceOfChar(conn, path, managed)
			if err != nil || !strings.EqualFold(svc, serviceUUID) {
				continue
			}
		}
		if charUUID != "" && !strings.EqualFold(cUUID, charUUID) {
			continue
		}
		charPath = path
		break
	}
	if charPath == "" {
		return nil, fmt.Errorf("ble: 未找到特征 (service=%s char=%s)", serviceUUID, charUUID)
	}

	char := conn.Object(bluezName, charPath)
	var written []byte
	if len(payload) > 0 {
		// options: map[string]dbus.Variant{"type": dbus.MakeVariant("request")}
		opts := map[string]dbus.Variant{"type": dbus.MakeVariant("request")}
		done := make(chan *dbus.Call, 1)
		char.Go(gattCharIf+".WriteValue", 0, done, payload, opts)
		select {
		case <-done:
		case <-time.After(timeout):
			return nil, fmt.Errorf("ble: 写特征超时")
		}
	}
	if readBack {
		done := make(chan *dbus.Call, 1)
		char.Go(gattCharIf+".ReadValue", 0, done, map[string]dbus.Variant{})
		select {
		case call := <-done:
			if call.Err == nil {
				if vals, ok := call.Body[0].([]byte); ok {
					written = vals
				}
			}
		case <-time.After(timeout):
		}
	}
	return written, nil
}

// findServiceOfChar 返回特征所属服务 UUID。
func findServiceOfChar(conn *dbus.Conn, charPath dbus.ObjectPath, managed map[dbus.ObjectPath]map[string]map[string]dbus.Variant) (string, error) {
	// 简化: 通过树结构推导 (父路径是服务)
	for path, ifaces := range managed {
		if _, ok := ifaces[gattServiceIf]; !ok {
			continue
		}
		if strings.HasPrefix(string(charPath), string(path)) {
			return strings.ToLower(getStr(ifaces[gattServiceIf], "UUID")), nil
		}
	}
	return "", fmt.Errorf("ble: 无法定位特征所属服务")
}

// ---- D-Bus 属性读取辅助 ----
func getStr(m map[string]dbus.Variant, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	return ""
}

func getI16(m map[string]dbus.Variant, key string) int16 {
	if v, ok := m[key]; ok {
		switch t := v.Value().(type) {
		case int16:
			return t
		case int32:
			return int16(t)
		}
	}
	return 0
}

func getBool(m map[string]dbus.Variant, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.Value().(bool); ok {
			return b
		}
	}
	return false
}
