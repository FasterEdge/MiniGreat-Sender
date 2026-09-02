//go:build linux

// Package candrv 实现 SocketCAN 发送驱动 (Linux only)。
package candrv

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/FasterEdge/MiniGreat-Sender/internal/core"
)

// CANDriver 实现 CAN 总线报文发送。
type CANDriver struct{}

// Name 返回协议名。
func (CANDriver) Name() string { return "can" }

// Description 返回描述。
func (CANDriver) Description() string {
	return "CAN 总线 (SocketCAN): can0/vcan0 发送标准/扩展/远程帧"
}

// Validate 校验参数。
func (CANDriver) Validate(req *core.Request) error {
	if req.CANInterface == "" {
		return fmt.Errorf("can: canInterface 不能为空 (如 can0/vcan0)")
	}
	return nil
}

// canFrame 与内核 struct can_frame 对齐(16字节)。
type canFrame struct {
	ID   uint32
	DLC  uint8
	Pad  [3]uint8
	Data [8]uint8
}

const (
	CAN_EFF_FLAG       = 0x80000000 // 扩展帧标志
	CAN_RTR_FLAG       = 0x40000000 // 远程帧标志
	CAN_ERR_FLAG       = 0x20000000
	SOL_CAN_BASE       = 100
	CAN_RAW            = 1
	CAN_RAW_FILTER     = 1
	CAN_RAW_ERR_FILTER = 2
)

// Send 发送一帧 CAN 报文。
func (CANDriver) Send(ctx context.Context, req *core.Request) (*core.Response, error) {
	payload, err := core.ResolvePayload(req)
	if err != nil {
		return nil, err
	}
	if len(payload) > 8 {
		return nil, fmt.Errorf("can: 一帧数据最多 8 字节, 当前 %d 字节 (需要拆帧)", len(payload))
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	fd, err := unix.Socket(unix.AF_CAN, unix.SOCK_RAW, CAN_RAW)
	if err != nil {
		return nil, fmt.Errorf("can: 打开 SocketCAN 失败: %w", err)
	}
	defer unix.Close(fd)

	iface, err := net.InterfaceByName(req.CANInterface)
	if err != nil {
		return nil, fmt.Errorf("can: 找不到接口 %s: %w", req.CANInterface, err)
	}
	addr := &unix.SockaddrCAN{Ifindex: iface.Index}
	if err := unix.Bind(fd, addr); err != nil {
		return nil, fmt.Errorf("can: 绑定 %s 失败: %w", req.CANInterface, err)
	}

	// 构造帧
	var frame canFrame
	id := req.CANID & 0x1FFFFFFF
	if req.CANExt {
		frame.ID = id | CAN_EFF_FLAG
	} else {
		if id > 0x7FF {
			return nil, fmt.Errorf("can: 标准帧 ID 不能超过 0x7FF")
		}
		frame.ID = id
	}
	if req.CANRTR {
		frame.ID |= CAN_RTR_FLAG
		frame.DLC = uint8(len(payload))
	} else {
		frame.DLC = uint8(len(payload))
		copy(frame.Data[:], payload)
	}
	buf := (*[unsafe.Sizeof(canFrame{})]byte)(unsafe.Pointer(&frame))[:]

	start := time.Now()
	if _, err := unix.Write(fd, buf); err != nil {
		return nil, fmt.Errorf("can: 发送失败: %w", err)
	}
	latency := time.Since(start).Milliseconds()
	_ = ctx

	resp := &core.Response{
		Protocol:  "can",
		Status:    "ok",
		LatencyMS: latency,
		Meta: map[string]any{
			"interface": req.CANInterface,
			"id":        fmt.Sprintf("0x%X", req.CANID),
			"extended":  req.CANExt,
			"rtr":       req.CANRTR,
			"dlc":       len(payload),
		},
	}
	if len(payload) > 0 {
		resp.Data = payload
		resp.DataHex = core.FormatDataHex(payload)
		resp.DataTxt = core.FormatDataTxt(payload)
	} else {
		resp.DataTxt = "(已发送空帧)"
	}
	return resp, nil
}

// 供其它平台编译占位
var _ = binary.LittleEndian
var _ = os.ErrClosed
