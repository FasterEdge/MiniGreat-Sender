package httpdrv

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/FasterEdge/MiniGreat-Sender/internal/core"
)

// HTTPDriver 实现 HTTP/HTTPS 请求发送, 支持全部请求方法。
type HTTPDriver struct{}

// Name 返回协议名。
func (HTTPDriver) Name() string { return "http" }

// Description 返回描述。
func (HTTPDriver) Description() string {
	return "HTTP/HTTPS 客户端: 任意请求方法 + 自定义头 + 载荷"
}

// Methods 返回 http 支持的方法列表(供 UI 提示)。
func (HTTPDriver) Methods() []string {
	return []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE"}
}

// Validate 校验参数。
func (HTTPDriver) Validate(req *core.Request) error {
	if req.URL == "" {
		return fmt.Errorf("http: url 不能为空")
	}
	if req.Method == "" {
		req.Method = http.MethodGet
	}
	return nil
}

// Send 执行一次 HTTP 请求。
func (HTTPDriver) Send(ctx context.Context, req *core.Request) (*core.Response, error) {
	if req.Method == "" {
		req.Method = http.MethodGet
	}
	payload, err := core.ResolvePayload(req)
	if err != nil {
		return nil, err
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx2, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var body io.Reader
	if len(payload) > 0 {
		body = bytes.NewReader(payload)
	}
	hreq, err := http.NewRequestWithContext(ctx2, strings.ToUpper(req.Method), req.URL, body)
	if err != nil {
		return nil, fmt.Errorf("http: 构造请求失败: %w", err)
	}
	for k, v := range req.Headers {
		if strings.EqualFold(k, "Host") {
			hreq.Host = v
			continue
		}
		hreq.Header.Set(k, v)
	}
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: req.Insecure}, // #nosec G402 -- 调试工具允许跳过校验
		},
	}
	start := time.Now()
	hresp, err := client.Do(hreq)
	if err != nil {
		return nil, fmt.Errorf("http: 请求失败: %w", err)
	}
	defer hresp.Body.Close()
	data, _ := io.ReadAll(hresp.Body)

	resp := &core.Response{
		Protocol:  "http",
		Status:    "ok",
		LatencyMS: time.Since(start).Milliseconds(),
		Data:      data,
		DataHex:   core.FormatDataHex(data),
		DataTxt:   core.FormatDataTxt(data),
		Meta: map[string]any{
			"statusCode": hresp.StatusCode,
			"statusText": hresp.Status,
			"headers":    flattenHeaders(hresp.Header),
		},
	}
	if hresp.StatusCode >= 400 {
		resp.Status = "error"
	}
	return resp, nil
}

func flattenHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			out[k] = strings.Join(v, ", ")
		}
	}
	return out
}
