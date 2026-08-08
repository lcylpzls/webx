package benchmarks

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labstack/echo/v4"
	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/webx"
	"github.com/valyala/fasthttp"
)

// writeBenchCert 生成自签名证书（PEM 文件 + 内存证书）。
func writeBenchCert(tb testing.TB) (certFile, keyFile string, cert tls.Certificate) {
	tb.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		tb.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		tb.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		tb.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	dir := tb.TempDir()
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		tb.Fatal(err)
	}
	cert, err = tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		tb.Fatal(err)
	}
	return certFile, keyFile, cert
}

// benchLogger 返回写入 io.Discard 的日志器。
func benchLogger() logx.Logger {
	logger, err := logx.NewBuilder().EnableWriter(io.Discard, logx.ErrorLevel).Build()
	if err != nil {
		panic(err)
	}
	return logger
}

// benchClient 返回 HTTP/1.1 + TLS 的 keep-alive 客户端。
func benchClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			MaxIdleConns:        128,
			MaxIdleConnsPerHost: 64,
			MaxConnsPerHost:     64,
			IdleConnTimeout:     60 * time.Second,
			DisableCompression:  true,
		},
	}
}

// benchClientH2 返回启用 HTTP/2 的客户端（webx 默认协议路径）。
func benchClientH2() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			ForceAttemptHTTP2:   true,
			MaxIdleConns:        128,
			MaxIdleConnsPerHost: 64,
			MaxConnsPerHost:     64,
			IdleConnTimeout:     60 * time.Second,
			DisableCompression:  true,
		},
	}
}

// runBenchRequests 并行压测指定地址（预热后计时）。
// 公平性约定：所有框架统一使用 HTTPS（webx 仅支持 HTTPS，不做明文对比）。
func runBenchRequests(b *testing.B, base string, warmup int) {
	runBenchRequestsWithClient(b, base, warmup, benchClient())
}

// runBenchRequestsWithClient 使用指定客户端并行压测（预热后计时）。
func runBenchRequestsWithClient(b *testing.B, base string, warmup int, client *http.Client) {
	b.Helper()
	// 并行预热：让连接池预先建立多条 keep-alive 连接，避免计时阶段混入握手开销。
	var wg sync.WaitGroup
	for g := 0; g < 12; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < warmup; i++ {
				resp, err := client.Get(base + "/ping")
				if err != nil {
					b.Error(err)
					return
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}()
	}
	wg.Wait()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Get(base + "/ping")
			if err != nil {
				b.Error(err)
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})
	b.StopTimer() // 关闭服务不计入基准结果
}

// startStdTLS 启动标准 net/http TLS 服务（HTTP/1.1）。
func startStdTLS(b testing.TB, handler http.Handler, cert tls.Certificate) (string, func()) {
	b.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	// 使用 ServeTLS + TLSConfig：标准库会在此路径上自动启用 HTTP/2（ALPN h2），
	// 同时保持 TLS 1.2 下限与外部 tls.Listen 行为一致。
	srv := &http.Server{
		Handler: handler,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
		ErrorLog: log.New(io.Discard, "", 0),
	}
	go func() { _ = srv.ServeTLS(ln, "", "") }()
	return "https://" + ln.Addr().String(), func() { _ = srv.Close() }
}

func BenchmarkServerTLSWebx(b *testing.B) {
	certFile, keyFile, _ := writeBenchCert(b)
	cfg := webx.Config{
		TLSCertFile:     certFile,
		TLSKeyFile:      keyFile,
		ShutdownTimeout: 5 * time.Second,
	}
	s := webx.NewServer(cfg, benchLogger())
	s.UseHttp2Listen("127.0.0.1:0")
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
			b.Fatal(err)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if base == "" {
		b.Fatal("webx 启动超时")
	}
	runBenchRequests(b, base, 200)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func BenchmarkServerTLSWebxFull(b *testing.B) {
	certFile, keyFile, _ := writeBenchCert(b)
	cfg := webx.Config{
		TLSCertFile:          certFile,
		TLSKeyFile:           keyFile,
		ShutdownTimeout:      5 * time.Second,
		RequestTimeout:       5 * time.Second,
		MiddlewareRequestID:  true,
		MiddlewareRecovery:   true,
		MiddlewareTimeout:    true,
		MiddlewareCORS:       true,
		MiddlewareValidation: true,
		MiddlewareSecurity:   true,
		MiddlewareGzip:       true,
		MiddlewareMetrics:    true,
		AccessLogEnabled:     true,
		LogSuccessReq:        true,
	}
	s := webx.NewServer(cfg, benchLogger())
	s.UseHttp2Listen("127.0.0.1:0")
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
			b.Fatal(err)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if base == "" {
		b.Fatal("webx 启动超时")
	}
	runBenchRequests(b, base, 200)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func BenchmarkServerTLSGin(b *testing.B) {
	_, _, cert := writeBenchCert(b)
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "hello") })
	base, stop := startStdTLS(b, r, cert)
	defer stop()
	runBenchRequests(b, base, 200)
}

// BenchmarkServerH2Gin 对比 gin HTTPS + HTTP/2 端到端吞吐（标准库封装阵营）。
func BenchmarkServerH2Gin(b *testing.B) {
	_, _, cert := writeBenchCert(b)
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "hello") })
	base, stop := startStdTLS(b, r, cert)
	defer stop()
	runBenchRequestsWithClient(b, base, 200, benchClientH2())
}

func BenchmarkServerTLSEcho(b *testing.B) {
	_, _, cert := writeBenchCert(b)
	e := echo.New()
	e.GET("/ping", func(c echo.Context) error { return c.String(http.StatusOK, "hello") })
	base, stop := startStdTLS(b, e, cert)
	defer stop()
	runBenchRequests(b, base, 200)
}

// BenchmarkServerH2Echo 对比 echo HTTPS + HTTP/2 端到端吞吐（标准库封装阵营）。
func BenchmarkServerH2Echo(b *testing.B) {
	_, _, cert := writeBenchCert(b)
	e := echo.New()
	e.GET("/ping", func(c echo.Context) error { return c.String(http.StatusOK, "hello") })
	base, stop := startStdTLS(b, e, cert)
	defer stop()
	runBenchRequestsWithClient(b, base, 200, benchClientH2())
}

// BenchmarkServerTLSServeMux 对比标准库裸 ServeMux HTTPS + HTTP/1.1（下限参照）。
func BenchmarkServerTLSServeMux(b *testing.B) {
	_, _, cert := writeBenchCert(b)
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	base, stop := startStdTLS(b, mux, cert)
	defer stop()
	runBenchRequests(b, base, 200)
}

// BenchmarkServerH2ServeMux 对比标准库裸 ServeMux HTTPS + HTTP/2（下限参照）。
func BenchmarkServerH2ServeMux(b *testing.B) {
	_, _, cert := writeBenchCert(b)
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	base, stop := startStdTLS(b, mux, cert)
	defer stop()
	runBenchRequestsWithClient(b, base, 200, benchClientH2())
}

// TestStdTLSServeMuxH2 验证标准库 TLS 服务在 h2 客户端下协商 HTTP/2。
func TestStdTLSServeMuxH2(t *testing.T) {
	_, _, cert := writeBenchCert(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	base, stop := startStdTLS(t, mux, cert)
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
		t.Fatalf("标准库 h2 基准未协商到 HTTP/2，实际协议=%q", proto)
	}
}

func BenchmarkServerTLSFasthttp(b *testing.B) {
	_, _, cert := writeBenchCert(b)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		b.Fatal(err)
	}
	fs := &fasthttp.Server{Handler: func(ctx *fasthttp.RequestCtx) {
		_, _ = ctx.WriteString("hello")
	}, Logger: discardLogger{}}
	go func() { _ = fs.Serve(ln) }()
	base := "https://" + ln.Addr().String()
	defer func() { _ = ln.Close() }()
	runBenchRequests(b, base, 200)
}

// discardLogger 静默 fasthttp 服务端日志。
type discardLogger struct{}

func (discardLogger) Printf(string, ...interface{}) {}
