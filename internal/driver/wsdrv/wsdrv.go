// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
package wsdrv

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/gorilla/websocket"

	"github.com/FasterEdge/MiniGreat-Sender/internal/core"
)

// WSDriver 实现 WebSocket 客户端: 连接 -> 发送文本/二进制消息 -> 读取响应。
type WSDriver struct{}

// Name 返回协议名。
func (WSDriver) Name() string { return "ws" }

// Description 返回描述。
func (WSDriver) Description() string {
	return "WebSocket 客户端: ws://wss:// 连接, 发送文本或二进制消息"
}

// Validate 校验参数。
func (WSDriver) Validate(req *core.Request) error {
	if req.URL == "" {
		return fmt.Errorf("ws: url 不能为空 (ws:// 或 wss://)")
	}
	return nil
}

// Send 执行一次 WebSocket 发送。
func (WSDriver) Send(ctx context.Context, req *core.Request) (*core.Response, error) {
	payload, err := core.ResolvePayload(req)
	if err != nil {
		return nil, err
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	dialer := websocket.Dialer{
		HandshakeTimeout: timeout,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
	}
	if req.Insecure {
		if dialer.TLSClientConfig == nil {
			dialer.TLSClientConfig = &tls.Config{} // #nosec G402 -- 调试工具
		}
		dialer.TLSClientConfig.InsecureSkipVerify = true
	}

	start := time.Now()
	conn, _, err := dialer.DialContext(ctx, req.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("ws: 连接失败: %w", err)
	}
	defer conn.Close()

	resp := &core.Response{Protocol: "ws", Status: "ok", LatencyMS: 0}
	if len(payload) > 0 {
		// 二进制消息与文本消息由调用方控制, 这里统一按二进制发送, 兼容大部分调试场景。
		if err := conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
			resp.Status = "error"
			resp.Error = "发送失败: " + err.Error()
			resp.LatencyMS = time.Since(start).Milliseconds()
			return resp, nil
		}
	}
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	mt, data, rerr := conn.ReadMessage()
	resp.LatencyMS = time.Since(start).Milliseconds()
	if rerr != nil {
		resp.Status = "timeout"
		resp.DataTxt = "(无响应或连接关闭)"
		return resp, nil
	}
	resp.Data = data
	resp.DataHex = core.FormatDataHex(data)
	resp.DataTxt = core.FormatDataTxt(data)
	if mt == websocket.TextMessage {
		resp.DataTxt = string(data)
		resp.Meta = map[string]any{"messageType": "text"}
	} else {
		resp.Meta = map[string]any{"messageType": "binary"}
	}
	return resp, nil
}
