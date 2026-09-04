// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
// Package web 提供 MiniGreat-Sender 本地 Web 调试面板:
//   - 静态页面 (内嵌)
//   - POST /api/send      执行一次发送 (JSON body = core.Request)
//   - GET  /api/protocols 列出协议
//   - WS   /api/ws        实时日志/历史推送
package web

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/FasterEdge/MiniGreat-Sender/internal/core"
	"github.com/FasterEdge/MiniGreat-Sender/internal/registry"
)

//go:embed static/*
var staticFS embed.FS

// Server Web 面板服务器。
type Server struct {
	reg      *core.Registry
	logger   *slog.Logger
	upgrader websocket.Upgrader
	mu       sync.Mutex
	history  []HistoryEntry
	clients  map[*wsClient]bool
}

// wsClient 为每个 WS 客户端持有的连接与其专属写锁。
// gorilla/websocket 不允许并发写同一连接, 事件推送可能并发,
// 因此每个连接必须有独立写锁 (否则数据竞态/panic)。
type wsClient struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// HistoryEntry 一条历史记录。
type HistoryEntry struct {
	Time     string         `json:"time"`
	Request  *core.Request  `json:"request"`
	Response *core.Response `json:"response"`
}

// New 创建 Web 服务器。
func New(logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		reg:     registry.New(),
		logger:  logger,
		history: make([]HistoryEntry, 0, 200),
		clients: make(map[*wsClient]bool),
	}
}

// Handler 返回 HTTP 路由。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/protocols", s.handleProtocols)
	mux.HandleFunc("/api/send", s.handleSend)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/ws", s.handleWS)
	return mux
}

func (s *Server) handleProtocols(w http.ResponseWriter, r *http.Request) {
	type info struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	out := []info{}
	for _, n := range s.reg.Names() {
		d, _ := s.reg.Get(n)
		out = append(out, info{n, d.Description()})
	}
	writeJSON(w, out)
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	// timeout 字段前端以字符串("300ms")或数字(ms)传入, 需要特殊解析
	var raw struct {
		core.Request
		Timeout json.RawMessage `json:"timeout"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&raw); err != nil {
		http.Error(w, "JSON 解析失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	req := raw.Request
	if len(raw.Timeout) > 0 && string(raw.Timeout) != "null" && string(raw.Timeout) != `""` {
		var s string
		if err := json.Unmarshal(raw.Timeout, &s); err == nil && s != "" {
			d, perr := time.ParseDuration(s)
			if perr != nil {
				http.Error(w, "timeout 解析失败: "+perr.Error(), http.StatusBadRequest)
				return
			}
			req.Timeout = d
		} else {
			var ms int64
			if err := json.Unmarshal(raw.Timeout, &ms); err == nil {
				req.Timeout = time.Duration(ms) * time.Millisecond
			}
		}
	}
	d, ok := s.reg.Get(req.Protocol)
	if !ok {
		http.Error(w, "未知协议: "+req.Protocol, http.StatusBadRequest)
		return
	}
	if err := d.Validate(&req); err != nil {
		http.Error(w, "参数校验失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	resp, err := d.Send(ctx, &req)
	if err != nil {
		resp = &core.Response{Protocol: req.Protocol, Status: "error", Error: err.Error()}
	}
	// 记录历史
	entry := HistoryEntry{Time: time.Now().Format("15:04:05.000"), Request: &req, Response: resp}
	s.push(entry)
	writeJSON(w, resp)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, s.history)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	// 面板 WS 只读客户端消息用于检测断线, 16KiB 上限 + 60s 读超时,
	// 防恶意客户端发超大帧或连上不发数据永久占用 goroutine。
	conn.SetReadLimit(1 << 16)
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	wc := &wsClient{conn: conn}
	s.mu.Lock()
	s.clients[wc] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, wc)
		s.mu.Unlock()
	}()
	// 简单心跳/关闭检测
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	}
}

// push 记录历史并广播到所有 WS 客户端。
// 每个连接用独立写锁串行化写 (gorilla 禁止并发写), 并设 2s 写超时;
// 慢客户端不再阻塞整个事件链, 写失败立即断开移除。
func (s *Server) push(e HistoryEntry) {
	s.mu.Lock()
	s.history = append(s.history, e)
	if len(s.history) > 500 {
		s.history = s.history[len(s.history)-500:]
	}
	clients := make([]*wsClient, 0, len(s.clients))
	for c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	for _, c := range clients {
		c.mu.Lock()
		_ = c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		werr := c.conn.WriteMessage(websocket.TextMessage, b)
		c.mu.Unlock()
		if werr != nil {
			// 写失败 (慢/断连客户端): 移除, 避免累积阻塞后续推送
			s.mu.Lock()
			delete(s.clients, c)
			s.mu.Unlock()
		}
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v) // #nosec G104
}
