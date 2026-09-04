// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
package mqttdrv

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/FasterEdge/MiniGreat-Sender/internal/core"
)

// MQTTDriver 实现 MQTT 客户端: 连接 broker 并向主题发布消息。
type MQTTDriver struct{}

// Name 返回协议名。
func (MQTTDriver) Name() string { return "mqtt" }

// Description 返回描述。
func (MQTTDriver) Description() string {
	return "MQTT 客户端: 连接 broker, 向主题发布消息 (QoS 0/1/2)"
}

// Validate 校验参数。
func (MQTTDriver) Validate(req *core.Request) error {
	if req.Broker == "" {
		return fmt.Errorf("mqtt: broker 不能为空 (如 tcp://127.0.0.1:1883)")
	}
	if req.Topic == "" {
		return fmt.Errorf("mqtt: topic 不能为空")
	}
	if req.QoS > 2 {
		return fmt.Errorf("mqtt: qos 必须在 0~2 之间")
	}
	return nil
}

// randHex 返回 n 字节随机数的十六进制 (用于生成低碰撞概率的默认 ClientID)。
func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Send 执行一次 MQTT 发布。
func (MQTTDriver) Send(ctx context.Context, req *core.Request) (*core.Response, error) {
	payload, err := core.ResolvePayload(req)
	if err != nil {
		return nil, err
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	// 尊重调用方 ctx (如 HTTP 请求取消/服务端超时), 取更早的截止时间
	if dl, ok := ctx.Deadline(); ok {
		if d := time.Until(dl); d < timeout {
			timeout = d
		}
	}
	clientID := req.ClientID
	if clientID == "" {
		// 时间戳+随机数: 仅时间戳取模碰撞空间小, 并发发布易被 broker 踢旧连接
		clientID = fmt.Sprintf("minigreat-sender-%d-%s", time.Now().UnixNano(), randHex(4))
	}
	opts := mqtt.NewClientOptions().
		AddBroker(req.Broker).
		SetClientID(clientID).
		SetConnectTimeout(timeout).
		SetAutoReconnect(false)
	if req.Username != "" {
		opts.SetUsername(req.Username)
		opts.SetPassword(req.Password)
	}
	client := mqtt.NewClient(opts)
	tok := client.Connect()
	if err := waitToken(ctx, tok, timeout); err != nil {
		return nil, fmt.Errorf("mqtt: 连接失败: %w", err)
	}
	defer client.Disconnect(100)

	start := time.Now()
	ptok := client.Publish(req.Topic, req.QoS, req.Retain, payload)
	if err := waitToken(ctx, ptok, timeout); err != nil {
		return nil, fmt.Errorf("mqtt: 发布失败: %w", err)
	}
	resp := &core.Response{
		Protocol:  "mqtt",
		Status:    "ok",
		LatencyMS: time.Since(start).Milliseconds(),
		Data:      payload,
		DataHex:   core.FormatDataHex(payload),
		DataTxt:   core.FormatDataTxt(payload),
		Meta: map[string]any{
			"broker":    req.Broker,
			"topic":     req.Topic,
			"qos":       req.QoS,
			"retain":    req.Retain,
			"clientId":  clientID,
			"byteCount": len(payload),
		},
	}
	return resp, nil
}

// waitToken 等待 MQTT token 完成; ctx 取消/超时优先于 token, 保证调用方中止能立即返回。
func waitToken(ctx context.Context, tok mqtt.Token, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-tok.Done():
		return tok.Error()
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("timeout after %v", timeout)
	}
}

// ParseBroker 简单规范化 broker 地址。
func ParseBroker(b string) string {
	b = strings.TrimSpace(b)
	if !strings.Contains(b, "://") {
		b = "tcp://" + b
	}
	return b
}
