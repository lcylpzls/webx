package benchmarks

import (
	"crypto/tls"
	testx "github.com/lcylpzls/testx"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labstack/echo/v4"
	"github.com/quic-go/quic-go/http3"
)

// startH3Handler 将任意标准库 http.Handler 挂到 quic-go http3.Server 上
// （与 webx 的 HTTP/3 同为 quic-go 协议栈），返回 HTTPS 地址与关闭函数。
func startH3Handler(b testing.TB, handler http.Handler, cert tls.Certificate) (string, func()) {
	b.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	testx.RequireNoError(b, err)

	srv := &http3.Server{
		Handler: handler,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
		},
	}
	go func() { _ = srv.Serve(pc) }()
	base := "https://" + pc.LocalAddr().String()
	return base, func() {
		_ = srv.Close()
		_ = pc.Close()
	}
}

// BenchmarkServerH3ServeMux 对比标准库裸 ServeMux HTTP/3 端到端吞吐。
func BenchmarkServerH3ServeMux(b *testing.B) {
	_, _, cert := writeBenchCert(b)
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	base, stop := startH3Handler(b, mux, cert)
	defer stop()
	client, closeClient := newBenchClientH3()
	defer closeClient()
	runBenchRequestsWithClient(b, base, 200, client)
}

// BenchmarkServerH3Gin 对比 gin HTTP/3 端到端吞吐。
func BenchmarkServerH3Gin(b *testing.B) {
	_, _, cert := writeBenchCert(b)
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "hello") })
	base, stop := startH3Handler(b, r, cert)
	defer stop()
	client, closeClient := newBenchClientH3()
	defer closeClient()
	runBenchRequestsWithClient(b, base, 200, client)
}

// BenchmarkServerH3Echo 对比 echo HTTP/3 端到端吞吐。
func BenchmarkServerH3Echo(b *testing.B) {
	_, _, cert := writeBenchCert(b)
	e := echo.New()
	e.GET("/ping", func(c echo.Context) error { return c.String(http.StatusOK, "hello") })
	base, stop := startH3Handler(b, e, cert)
	defer stop()
	client, closeClient := newBenchClientH3()
	defer closeClient()
	runBenchRequestsWithClient(b, base, 200, client)
}

// TestStdH3Protocol 验证标准库 http.Handler 挂到 http3.Server 后协商 HTTP/3。
func TestStdH3Protocol(t *testing.T) {
	_, _, cert := writeBenchCert(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	base, stop := startH3Handler(t, mux, cert)
	defer stop()
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
	testx.RequireEqual(t, proto, "HTTP/3.0")

}
