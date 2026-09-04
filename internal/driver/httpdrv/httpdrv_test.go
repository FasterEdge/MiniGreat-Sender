// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
package httpdrv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FasterEdge/MiniGreat-Sender/internal/core"
)

// TestSendResponseBodyLimited: 服务器返回超大响应体时, 客户端必须截断到 1MiB
// (回归: 修复前 io.ReadAll 无上限, 恶意/异常服务器可耗尽内存)。
func TestSendResponseBodyLimited(t *testing.T) {
	big := make([]byte, 3<<20) // 3 MiB > 1 MiB 上限
	for i := range big {
		big[i] = byte(i % 251)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(big)
	}))
	defer srv.Close()

	drv := HTTPDriver{}
	resp, err := drv.Send(context.Background(), &core.Request{
		URL:     srv.URL,
		Method:  http.MethodGet,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Send unexpected error: %v", err)
	}
	if len(resp.Data) != 1<<20 {
		t.Fatalf("resp.Data len = %d, want 1MiB (limited)", len(resp.Data))
	}
	// 截断内容应与源一致 (前 1MiB)
	for i := 0; i < 1<<20; i++ {
		if resp.Data[i] != big[i] {
			t.Fatalf("byte %d mismatch", i)
		}
	}
}