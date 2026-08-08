package webx

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lcylpzls/webx/internal/core"
)

// discardWriter 是零分配的最小响应写入器，用于基准测试。
type discardWriter struct {
	h http.Header
}

func (d *discardWriter) Header() http.Header         { return d.h }
func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }
func (d *discardWriter) WriteHeader(int)             {}

func BenchmarkRouterServeHTTP(b *testing.B) {
	rt := NewRouter(core.NoRouteHandler, core.NoMethodHandler)
	for i := 0; i < 50; i++ {
		if err := rt.Handle("GET", fmt.Sprintf("/api/v1/resource/%d/:id", i), []core.HandlerFunc{noopCore}); err != nil {
			b.Fatal(err)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/resource/25/42", nil)
	dw := &discardWriter{h: make(http.Header)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.ServeHTTP(dw, req)
	}
}

func BenchmarkMiddlewareChain(b *testing.B) {
	mw := func(c *core.Context) { c.Next() }
	chain := []core.HandlerFunc{mw, mw, mw, func(c *core.Context) {}}
	dw := &discardWriter{h: make(http.Header)}
	c := core.NewContext(dw, httptest.NewRequest(http.MethodGet, "/", nil))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.SetHandlers(chain)
		c.Run()
	}
}

func BenchmarkJSONResponse(b *testing.B) {
	dw := &discardWriter{h: make(http.Header)}
	c := core.NewContext(dw, httptest.NewRequest(http.MethodGet, "/", nil))
	resp := StandardizedResponse{Code: CodeSuccess, Msg: "ok", RequestID: "r", Timestamp: 1}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.JSON(http.StatusOK, resp)
	}
}

func BenchmarkSuccessResponse(b *testing.B) {
	dw := &discardWriter{h: make(http.Header)}
	c := core.NewContext(dw, httptest.NewRequest(http.MethodGet, "/", nil))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Success("ok", nil)
	}
}

func BenchmarkServerRequest(b *testing.B) {
	cert, key := writeTestCert(b)
	cfg := Config{
		TLSCertFile:     cert,
		TLSKeyFile:      key,
		ShutdownTimeout: 5 * time.Second,
	}
	cfg.MiddlewareRequestID = true
	cfg.MiddlewareRecovery = true
	s := NewServer(cfg, newTestLogger(b))
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method: "GET",
		Path:   "/ping",
		Handler: func(c *core.Context) {
			c.Success("pong", nil)
		},
	})
	errCh := make(chan error, 1)
	go func() { errCh <- s.Start() }()
	deadline := time.After(10 * time.Second)
	for s.ListenerAddr() == "" {
		select {
		case err := <-errCh:
			b.Fatalf("Start 失败：%v", err)
		case <-deadline:
			b.Fatal("启动超时")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Stop(ctx)
	}()

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			MaxIdleConns:    1,
		},
	}
	url := "https://" + s.ListenerAddr() + "/ping"
	// 预热并建立连接
	resp, err := client.Get(url)
	if err != nil {
		b.Fatalf("预热请求失败：%v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url)
		if err != nil {
			b.Fatalf("请求失败：%v", err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func BenchmarkServerRequestFull(b *testing.B) {
	cert, key := writeTestCert(b)
	cfg := Config{
		TLSCertFile:     cert,
		TLSKeyFile:      key,
		ShutdownTimeout: 5 * time.Second,
		RequestTimeout:  5 * time.Second,
	}
	cfg.MiddlewareRequestID = true
	cfg.MiddlewareRecovery = true
	cfg.MiddlewareTimeout = true
	cfg.MiddlewareCORS = true
	cfg.MiddlewareValidation = true
	cfg.MiddlewareSecurity = true
	cfg.MiddlewareGzip = true
	cfg.MiddlewareMetrics = true
	cfg.AccessLogEnabled = true
	cfg.LogSuccessReq = true
	s := NewServer(cfg, newTestLogger(b))
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method: "GET",
		Path:   "/ping",
		Handler: func(c *core.Context) {
			c.Success("pong", nil)
		},
	})
	errCh := make(chan error, 1)
	go func() { errCh <- s.Start() }()
	deadline := time.After(10 * time.Second)
	for s.ListenerAddr() == "" {
		select {
		case err := <-errCh:
			b.Fatalf("Start 失败：%v", err)
		case <-deadline:
			b.Fatal("启动超时")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Stop(ctx)
	}()

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			MaxIdleConns:    1,
		},
	}
	url := "https://" + s.ListenerAddr() + "/ping"
	resp, err := client.Get(url)
	if err != nil {
		b.Fatalf("预热请求失败：%v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url)
		if err != nil {
			b.Fatalf("请求失败：%v", err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func BenchmarkRouterServeHTTP100(b *testing.B) {
	benchRouter(b, 100)
}

func BenchmarkRouterServeHTTP500(b *testing.B) {
	benchRouter(b, 500)
}

func benchRouter(b *testing.B, n int) {
	rt := NewRouter(core.NoRouteHandler, core.NoMethodHandler)
	for i := 0; i < n; i++ {
		if err := rt.Handle("GET", fmt.Sprintf("/api/v1/resource/%d/:id", i), []core.HandlerFunc{noopCore}); err != nil {
			b.Fatal(err)
		}
	}
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/resource/%d/42", n-1), nil)
	dw := &discardWriter{h: make(http.Header)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.ServeHTTP(dw, req)
	}
}
