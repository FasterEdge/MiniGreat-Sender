package mqttdrv

import (
	"context"
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
	clientID := req.ClientID
	if clientID == "" {
		clientID = fmt.Sprintf("minigreat-sender-%d", time.Now().UnixNano()%100000)
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
	if !tok.WaitTimeout(timeout) || tok.Error() != nil {
		return nil, fmt.Errorf("mqtt: 连接失败: %v", tok.Error())
	}
	defer client.Disconnect(100)

	start := time.Now()
	ptok := client.Publish(req.Topic, req.QoS, req.Retain, payload)
	if !ptok.WaitTimeout(timeout) || ptok.Error() != nil {
		return nil, fmt.Errorf("mqtt: 发布失败: %v", ptok.Error())
	}
	_ = ctx
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

// ParseBroker 简单规范化 broker 地址。
func ParseBroker(b string) string {
	b = strings.TrimSpace(b)
	if !strings.Contains(b, "://") {
		b = "tcp://" + b
	}
	return b
}
