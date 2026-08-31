// Package web 提供 MiniGreat-Sender 本地 Web 调试面板:
//  - 静态页面 (内嵌)
//  - POST /api/send      执行一次发送 (JSON body = core.Request)
//  - GET  /api/protocols 列出协议
//  - WS   /api/ws        实时日志/历史推送
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

	"minigreat-sender/internal/core"
	"minigreat-sender/internal/registry"
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
	clients  map[*websocket.Conn]bool
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
		clients: make(map[*websocket.Conn]bool),
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
	s.mu.Lock()
	s.clients[conn] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
	}()
	// 简单心跳/关闭检测
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

// push 记录历史并广播到所有 WS 客户端。
func (s *Server) push(e HistoryEntry) {
	s.mu.Lock()
	s.history = append(s.history, e)
	if len(s.history) > 500 {
		s.history = s.history[len(s.history)-500:]
	}
	clients := make([]*websocket.Conn, 0, len(s.clients))
	for c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	for _, c := range clients {
		_ = c.WriteMessage(websocket.TextMessage, b) // #nosec G104 -- 面板推送失败可忽略
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v) // #nosec G104
}