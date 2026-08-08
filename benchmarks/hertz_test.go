package benchmarks

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol/suite"
	"github.com/hertz-contrib/http2/factory"
	"github.com/lcylpzls/webx"
	"github.com/quic-go/quic-go/http3"
)

func init() {
	// 静音 hertz 系统日志，避免基准输出噪音（全局仅设置一次）。
	hlog.SetOutput(io.Discard)
}

// TestHertzTLSProtocol 验证 hertz TLS 服务在 ALPN 下协商 HTTP/2。
func TestHertzTLSProtocol(t *testing.T) {
	_, _, cert := writeBenchCert(t)
	base, stop := startHertzTLS(t, cert, true)
	defer stop()
	client := benchClientH2()
	var proto string
	for i := 0; i < 200; i++ {
		resp, err := client.Get(base + "/ping")
		if err == nil {
			proto = resp.Proto
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if proto != "HTTP/2.0" {
		t.Fatalf("hertz h2 基准未协商到 HTTP/2，实际协议=%q", proto)
	}
}

// startHertzTLS 启动 hertz TLS 服务（HTTP/1.1 或 HTTP/2），返回 HTTPS 地址与关闭函数。
func startHertzTLS(b testing.TB, cert tls.Certificate, enableH2 bool) (string, func()) {
	b.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	h := server.New(server.WithListener(ln), server.WithTLS(cfg), server.WithALPN(true))
	if enableH2 {
		h.AddProtocol(suite.HTTP2, factory.NewServerFactory())
		cfg.NextProtos = []string{suite.HTTP2}
	}
	h.GET("/ping", func(_ context.Context, c *app.RequestContext) {
		c.SetBodyString("hello")
	})
	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()
	base := "https://" + ln.Addr().String()
	return base, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.Shutdown(ctx)
	}
}

// BenchmarkServerTLSHertz 对比 hertz HTTPS + HTTP/1.1 端到端吞吐。
func BenchmarkServerTLSHertz(b *testing.B) {
	_, _, cert := writeBenchCert(b)
	base, stop := startHertzTLS(b, cert, false)
	defer stop()
	runBenchRequests(b, base, 200)
}

// BenchmarkServerH2Hertz 对比 hertz HTTPS + HTTP/2 端到端吞吐。
func BenchmarkServerH2Hertz(b *testing.B) {
	_, _, cert := writeBenchCert(b)
	base, stop := startHertzTLS(b, cert, true)
	defer stop()
	runBenchRequestsWithClient(b, base, 200, benchClientH2())
}

// webxConfigForBench 构造 webx 基准配置。
func webxConfigForBench(certFile, keyFile string) webx.Config {
	return webx.Config{
		TLSCertFile:     certFile,
		TLSKeyFile:      keyFile,
		ShutdownTimeout: 5 * time.Second,
	}
}

// newWebxServer 启动 webx 服务并等待监听就绪；useH3 为 true 时启用 HTTP/3（QUIC over UDP）。
func newWebxServer(b *testing.B, cfg webx.Config, useH3 bool) *webx.Server {
	b.Helper()
	s := webx.NewServer(cfg, benchLogger())
	if useH3 {
		s.UseHttp3Listen("127.0.0.1:0")
	} else {
		s.UseHttp2Listen("127.0.0.1:0")
	}
	s.RegisterRoute(webx.Route{
		Method:  http.MethodGet,
		Path:    "/ping",
		Handler: func(c *webx.Context) { _ = c.String(http.StatusOK, "hello") },
	})
	errCh := make(chan error, 1)
	go func() { errCh <- s.Start() }()
	for i := 0; i < 500; i++ {
		if addr := s.ListenerAddr(); addr != "" {
			return s
		}
		select {
		case err := <-errCh:
			b.Fatal(err)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	b.Fatal("webx 启动超时")
	return nil
}

// newBenchClientH3 返回 HTTP/3（QUIC）客户端与关闭函数。
func newBenchClientH3() (*http.Client, func()) {
	tr := &http3.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{Transport: tr}, func() { _ = tr.Close() }
}

// BenchmarkServerH2Webx 对比 webx HTTPS + HTTP/2 端到端吞吐。
// 该基准与 hertz 的 h2 基准共享同一客户端与预热逻辑。
func BenchmarkServerH2Webx(b *testing.B) {
	certFile, keyFile, _ := writeBenchCert(b)
	cfg := webxConfigForBench(certFile, keyFile)
	s := newWebxServer(b, cfg, false)
	defer s.Stop(context.Background())
	runBenchRequestsWithClient(b, "https://"+s.ListenerAddr(), 200, benchClientH2())
}

// BenchmarkServerH3Webx 对比 webx HTTP/3（QUIC over UDP）端到端吞吐。
// 客户端使用 quic-go http3.Transport，与 h2 一样为单连接多路复用。
func BenchmarkServerH3Webx(b *testing.B) {
	certFile, keyFile, _ := writeBenchCert(b)
	cfg := webxConfigForBench(certFile, keyFile)
	s := newWebxServer(b, cfg, true)
	defer s.Stop(context.Background())
	client, closeClient := newBenchClientH3()
	defer closeClient()
	runBenchRequestsWithClient(b, "https://"+s.ListenerAddr(), 200, client)
}

// TestWebxH3Protocol 验证 webx HTTP/3 基准真实走 QUIC + HTTP/3。
func TestWebxH3Protocol(t *testing.T) {
	certFile, keyFile, _ := writeBenchCert(t)
	cfg := webxConfigForBench(certFile, keyFile)
	s := webx.NewServer(cfg, benchLogger())
	s.UseHttp3Listen("127.0.0.1:0")
	s.RegisterRoute(webx.Route{
		Method:  http.MethodGet,
		Path:    "/ping",
		Handler: func(c *webx.Context) { _ = c.String(http.StatusOK, "hello") },
	})
	errCh := make(chan error, 1)
	go func() { errCh <- s.Start() }()
	base := ""
	for i := 0; i < 500; i++ {
		if addr := s.ListenerAddr(); addr != "" {
			base = "https://" + addr
			break
		}
		select {
		case err := <-errCh:
			t.Fatal(err)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if base == "" {
		t.Fatal("webx HTTP/3 启动超时")
	}
	client, closeClient := newBenchClientH3()
	defer closeClient()
	var proto string
	for i := 0; i < 200; i++ {
		resp, err := client.Get(base + "/ping")
		if err == nil {
			proto = resp.Proto
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if proto != "HTTP/3.0" {
		t.Fatalf("webx HTTP/3 基准未协商到 HTTP/3，实际协议=%q", proto)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}
