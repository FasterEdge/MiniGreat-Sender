// Package netdrv 提供 TCP / UDP 网络发送驱动。
package netdrv

import (
	"context"
	"fmt"
	"net"
	"time"

	"minigreat-sender/internal/core"
)

// TCPDriver 实现 TCP 发送: 连接 -> 发送 -> 读取响应(直到超时)。
type TCPDriver struct{}

// Name 返回协议名。
func (TCPDriver) Name() string { return "tcp" }

// Description 返回描述。
func (TCPDriver) Description() string { return "TCP 客户端: 连接远程端口并发送数据, 读取响应" }

// Validate 校验参数。
func (TCPDriver) Validate(req *core.Request) error {
	if req.RemoteAddr == "" {
		return fmt.Errorf("tcp: remoteAddr 不能为空 (host:port)")
	}
	return nil
}

// Send 执行一次 TCP 发送。
func (TCPDriver) Send(ctx context.Context, req *core.Request) (*core.Response, error) {
	payload, err := core.ResolvePayload(req)
	if err != nil {
		return nil, err
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	d := net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := d.DialContext(ctx, "tcp", req.RemoteAddr)
	if err != nil {
		return nil, fmt.Errorf("tcp: 连接失败: %w", err)
	}
	defer conn.Close()

	resp := &core.Response{Protocol: "tcp", Status: "ok", LatencyMS: 0}
	if len(payload) > 0 {
		if _, err := conn.Write(payload); err != nil {
			resp.Status = "error"
			resp.Error = err.Error()
			return resp, nil
		}
	}
	// 读取响应直到 ctx 超时/取消
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 4096)
	var total []byte
	for {
		n, rerr := conn.Read(buf)
		if n > 0 {
			total = append(total, buf[:n]...)
			if len(total) >= 1<<20 { // 1MB 上限
				break
			}
		}
		if rerr != nil {
			break
		}
	}
	resp.LatencyMS = time.Since(start).Milliseconds()
	if len(total) > 0 {
		resp.Data = total
		resp.DataHex = core.FormatDataHex(total)
		resp.DataTxt = core.FormatDataTxt(total)
	} else {
		resp.Status = "timeout"
		resp.DataTxt = "(无响应)"
	}
	return resp, nil
}

// UDPDriver 实现 UDP 发送: 直接向目标发送数据报并等待可选响应。
type UDPDriver struct{}

// Name 返回协议名。
func (UDPDriver) Name() string { return "udp" }

// Description 返回描述。
func (UDPDriver) Description() string { return "UDP 客户端: 发送数据报, 可选读取响应" }

// Validate 校验参数。
func (UDPDriver) Validate(req *core.Request) error {
	if req.RemoteAddr == "" {
		return fmt.Errorf("udp: remoteAddr 不能为空 (host:port)")
	}
	return nil
}

// Send 执行一次 UDP 发送。
func (UDPDriver) Send(ctx context.Context, req *core.Request) (*core.Response, error) {
	payload, err := core.ResolvePayload(req)
	if err != nil {
		return nil, err
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	start := time.Now()
	conn, err := net.Dial("udp", req.RemoteAddr)
	if err != nil {
		return nil, fmt.Errorf("udp: 连接失败: %w", err)
	}
	defer conn.Close()

	resp := &core.Response{Protocol: "udp", Status: "ok", LatencyMS: 0}
	if len(payload) > 0 {
		if _, err := conn.Write(payload); err != nil {
			resp.Status = "error"
			resp.Error = err.Error()
			return resp, nil
		}
	}
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 4096)
	n, rerr := conn.Read(buf)
	resp.LatencyMS = time.Since(start).Milliseconds()
	if n > 0 {
		resp.Data = buf[:n]
		resp.DataHex = core.FormatDataHex(buf[:n])
		resp.DataTxt = core.FormatDataTxt(buf[:n])
	} else if rerr != nil {
		resp.Status = "timeout"
		resp.DataTxt = "(无响应)"
	} else {
		resp.DataTxt = "(已发送, 无响应数据)"
	}
	return resp, nil
}