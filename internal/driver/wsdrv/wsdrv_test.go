// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
package wsdrv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/FasterEdge/MiniGreat-Sender/internal/core"
)

// TestSendReadLimit: 服务器发送超大消息时, 客户端 ReadMessage 必须因
// SetReadLimit(1MiB) 报错, 而不是无限分配内存 (回归: 修复前无上限)。
func TestSendReadLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := websocket.Upgrader{}
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		big := make([]byte, 2<<20) // 2 MiB > 1 MiB 上限
		for i := range big {
			big[i] = 0xAB
		}
		_ = conn.WriteMessage(websocket.BinaryMessage, big)
	}))
	defer srv.Close()
	wsURL := "ws" + srv.URL[len("http"):]

	drv := WSDriver{}
	resp, err := drv.Send(context.Background(), &core.Request{
		URL:     wsURL,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Send unexpected error: %v", err)
	}
	if resp.Status != "timeout" {
		t.Fatalf("resp.Status = %q, want timeout (read limit hit)", resp.Status)
	}
}