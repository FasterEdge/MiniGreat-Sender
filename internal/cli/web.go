// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
package cli

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/FasterEdge/MiniGreat-Sender/internal/web"
)

// openBrowserCmd 尝试用系统默认浏览器打开 URL。
func openBrowserCmd(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	_ = cmd.Start() // #nosec G204 -- 本地面板固定URL
}

// cmdWeb 启动本地 Web 调试面板。
func cmdWeb(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("web", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:8080", "监听地址")
	openBrowser := fs.Bool("open", false, "启动后尝试打开浏览器")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintln(stderr, "监听失败:", err)
		return 1
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	srv := web.New(logger)

	// 打印面板地址; 0.0.0.0/:: 时显示真实暴露面并警告无鉴权
	port := ln.Addr().(*net.TCPAddr).Port
	host := "127.0.0.1"
	exposed := false
	if h, _, e := net.SplitHostPort(*addr); e == nil && h != "" {
		if h == "0.0.0.0" || h == "::" {
			exposed = true
			host = h
		} else {
			host = h
		}
	}
	panelURL := fmt.Sprintf("http://%s:%d", host, port)
	fmt.Fprintf(stdout, "MiniGreat-Sender Web 调试面板: %s\n", panelURL)
	if exposed {
		// 面板无鉴权: /api/send 可远程触发任意协议发送, /api/history 回显
		// 请求(含 MQTT 密码等凭据字段)——显式暴露必须伴随醒目警告。
		fmt.Fprintln(stdout, "警告: 面板监听 0.0.0.0/:: 且无鉴权——所在网络内的任何主机都可调用 /api/send 触发发送、读取 /api/history, 请确认部署网络可信, 或加访问控制(防火墙/iptables)。")
	}
	if *openBrowser {
		openBrowserCmd(panelURL)
	}

	// ReadHeaderTimeout 防慢速客户端无限占用 handler goroutine
	// (慢连接 DoS: 连上不发完整请求头即可挂住一个 goroutine)。
	httpSrv := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := httpSrv.Serve(ln); err != nil {
		fmt.Fprintln(stderr, "Web 服务异常:", err)
		return 1
	}
	return 0
}
