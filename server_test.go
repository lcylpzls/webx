package webx

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	testx "github.com/lcylpzls/testx"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/lcylpzls/errx"
	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/webx/internal/core"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// newTestLogger 返回写入 io.Discard 的测试日志器。
func newTestLogger(t testHelper) logx.Logger {
	t.Helper()
	l, err := logx.NewBuilder().EnableWriter(io.Discard, logx.InfoLevel).Build()
	testx.RequireNoError(t, err)

	t.Cleanup(func() { _ = l.Close() })
	return l
}

// newTestServer 创建注入测试日志器的 Server。
func newTestServer(t *testing.T, cfg Config) *Server {
	t.Helper()
	return NewServer(cfg, newTestLogger(t))
}

// noopMW 返回一个直通的标准中间件（测试辅助）。
func noopMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// stdChainToCore 将标准中间件链适配为 core 处理器链（测试辅助）。
func stdChainToCore(chain []func(http.Handler) http.Handler, final core.HandlerFunc) []core.HandlerFunc {
	handlers := make([]core.HandlerFunc, 0, len(chain)+1)
	for _, mw := range chain {
		mw := mw
		handlers = append(handlers, func(c *core.Context) {
			r := c.Request().WithContext(core.NewContextWith(c.Request().Context(), c))
			c.SetRequest(r)
			called := false
			var next http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r2 *http.Request) {
				called = true
				c.SetWriter(w)
				c.SetRequest(r2)
				c.Next()
			})
			mw(next).ServeHTTP(c.Writer(), r)
			if !called {
				c.Abort()
			}
		})
	}
	return append(handlers, final)
}

func validConfig(t *testing.T) Config {
	t.Helper()
	cert, key := writeTestCert(t)
	return Config{
		TLSCertFile:     cert,
		TLSKeyFile:      key,
		ShutdownTimeout: 3 * time.Second,
		RequestTimeout:  3 * time.Second,
	}
}

// startServer 在 goroutine 中启动服务并等待监听就绪。
func startServer(t *testing.T, s *Server) {
	t.Helper()
	errCh := make(chan error, 1)
	go func() { errCh <- s.Start() }()
	deadline := time.After(5 * time.Second)
	for {
		if s.ListenerAddr() != "" {
			return
		}
		select {
		case err := <-errCh:
			t.Fatalf("Start 失败：%v", err)
		case <-deadline:
			t.Fatal("服务启动超时")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func testHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

func TestServerChainAPI(t *testing.T) {
	s := newTestServer(t, validConfig(t))
	testx.RequireNotNil(t, s)

	if got := s.WithLogger(newTestLogger(t)); got != s {
		t.Error("链式方法应返回自身")
	}
	if got := s.UseGlobalMiddleware(noopMW); got != s {
		t.Error("UseGlobalMiddleware 应返回自身")
	}
	if got := s.OverrideMiddleware(MiddlewareRequestID, noopMW); got != s {
		t.Error("OverrideMiddleware 应返回自身")
	}
	if got := s.DisableMiddleware(MiddlewareCORS); got != s {
		t.Error("DisableMiddleware 应返回自身")
	}
	if got := s.EnableMiddleware(MiddlewareCORS); got != s {
		t.Error("EnableMiddleware 应返回自身")
	}
	if got := s.RegisterRoute(Route{Method: "GET", Path: "/a", Handler: noopHandler}); got != s {
		t.Error("RegisterRoute 应返回自身")
	}
	if got := s.RegisterRoutes([]Route{{Method: "GET", Path: "/b", Handler: noopHandler}}); got != s {
		t.Error("RegisterRoutes 应返回自身")
	}
	if got := s.RegisterRouteGroup("/g", func(rg *RouteGroup) { rg.GET("/c", noopHandler) }); got != s {
		t.Error("RegisterRouteGroup 应返回自身")
	}
	if got := s.RegisterHealthCheck("ok", func(ctx context.Context) error { return nil }); got != s {
		t.Error("RegisterHealthCheck 应返回自身")
	}
	if got := s.RegisterLivenessCheck("", nil); got != s {
		t.Error("非法存活检查应直接返回")
	}
	if got := s.RegisterReadinessCheck("", nil); got != s {
		t.Error("非法就绪检查应直接返回")
	}
	if got := s.UseHttp2Listen(":0"); got != s {
		t.Error("UseHttp2Listen 应返回自身")
	}
	if got := s.UseHttp3Listen(":0"); got != s {
		t.Error("UseHttp3Listen 应返回自身")
	}
	if got := s.UseUnixSocketListen("test.sock", 0); got != s {
		t.Error("UseUnixSocketListen 应返回自身")
	}
	if got := s.EnableRateLimit(RateLimitOptions{QPS: 0, Window: 0}); got != s {
		t.Error("非法限流参数应直接返回")
	}
	if got := s.EnableRateLimit(RateLimitOptions{QPS: 10, Window: time.Second}); got != s {
		t.Error("EnableRateLimit 应返回自身")
	}
	if got := s.SetRequestIDOptions(RequestIDOptions{Header: "X-Trace-ID"}); got != s {
		t.Error("SetRequestIDOptions 应返回自身")
	}
	if got := s.WithMetrics(nil); got != s {
		t.Error("WithMetrics 应返回自身")
	}
	if got := s.SetMaxConcurrentRequests(10); got != s {
		t.Error("SetMaxConcurrentRequests 应返回自身")
	}
	if got := s.SetErrorMessages(map[string]string{ErrorMessageNotFound: "x"}); got != s {
		t.Error("SetErrorMessages 应返回自身")
	}
	if got := s.DisableRateLimit(); got != s {
		t.Error("DisableRateLimit 应返回自身")
	}
	if got := s.ServeStaticDir("/static", t.TempDir()); got != s {
		t.Error("ServeStaticDir 应返回自身")
	}
	if got := s.ServeStaticFS("/embed", http.Dir(t.TempDir())); got != s {
		t.Error("ServeStaticFS 应返回自身")
	}
	if got := s.EnableSPA(http.Dir(t.TempDir()), "index.html"); got != s {
		t.Error("EnableSPA 应返回自身")
	}
	if got := s.ListenerAddr(); got != "" {
		t.Errorf("未启动时 ListenerAddr 应为空：%s", got)
	}
	if m := s.Metrics(); m.Requests != 0 || m.Errors5xx != 0 {
		t.Errorf("未启动时 Metrics 应为 0：%+v", m)
	}
	if got := s.RouteStats(); got != nil {
		t.Errorf("未启动时 RouteStats 应为 nil：%+v", got)
	}
	if got := s.GroupStats(); got != nil {
		t.Errorf("未启动时 GroupStats 应为 nil：%+v", got)
	}
}

func TestWarnStartedWithNilLogger(t *testing.T) {
	s := &Server{started: true}
	if got := s.RegisterRoute(Route{Method: "GET", Path: "/x", Handler: noopHandler}); got != s {
		t.Error("RegisterRoute 应返回自身")
	}
}

func TestServerStartErrors(t *testing.T) {
	// logger 为 nil
	s := NewServer(validConfig(t), nil)
	s.UseHttp2Listen("127.0.0.1:0")
	if err := s.Start(); err == nil {
		t.Error("nil logger 应报错")
	}
	// 无监听方式
	s = newTestServer(t, validConfig(t))
	if err := s.Start(); !errx.Is(err, CodeStartFailed) {
		t.Errorf("无监听方式错误不符：%v", err)
	}
	// 配置校验失败
	s = newTestServer(t, Config{})
	s.UseHttp2Listen(":0")
	if err := s.Start(); err == nil {
		t.Error("非法配置应报错")
	}
	// Unix 平台检查失败
	orig := checkUnixSocket
	checkUnixSocket = func() error { return errors.New("平台不支持") }
	defer func() { checkUnixSocket = orig }()
	s = newTestServer(t, validConfig(t))
	s.UseUnixSocketListen("x.sock", 0o600)
	if err := s.Start(); err == nil {
		t.Error("平台检查失败应报错")
	}
	// HTTP/2 监听失败
	s = newTestServer(t, validConfig(t))
	s.UseHttp2Listen("bad-addr")
	if err := s.Start(); err == nil {
		t.Error("非法 HTTP/2 地址应报错")
	}
	// 启动失败时回收限流清理 goroutine
	s = newTestServer(t, validConfig(t))
	s.EnableRateLimit(RateLimitOptions{QPS: 10, Window: time.Second})
	s.UseHttp2Listen("bad-addr")
	if err := s.Start(); err == nil {
		t.Error("限流场景下非法地址应启动失败")
	}
	// 路由注册失败（非法参数名）
	s = newTestServer(t, validConfig(t))
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{Method: "GET", Path: "/x/:", Handler: noopHandler})
	if err := s.Start(); err == nil {
		t.Error("非法路由应报错")
	}
	// 路由组注册失败
	s = newTestServer(t, validConfig(t))
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRouteGroup("/g", func(rg *RouteGroup) { rg.GET("/x/:", noopHandler) })
	if err := s.Start(); err == nil {
		t.Error("非法分组路由应报错")
	}
	// 静态路由注册失败（非法前缀）
	s = newTestServer(t, validConfig(t))
	s.UseHttp2Listen("127.0.0.1:0")
	s.ServeStaticFS("/a/{p...}/b", http.Dir(t.TempDir()))
	if err := s.Start(); err == nil {
		t.Error("非法静态前缀应报错")
	}
	// 健康检查注册失败（非法路径）
	cfg := validConfig(t)
	cfg.HealthPath = "/x/:"
	s = newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	if err := s.Start(); err == nil {
		t.Error("非法健康检查路径应报错")
	}
	// 路由分组回调 panic（注册阶段）
	s = newTestServer(t, validConfig(t))
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRouteGroup("/g", func(rg *RouteGroup) {
		panic("分组回调 panic")
	})
	if err := s.Start(); !errx.Is(err, CodeStartFailed) {
		t.Errorf("分组回调 panic 应转为启动错误：%v", err)
	}
	// 路由分组回调 panic（hasRoute 二次执行阶段）
	s = newTestServer(t, validConfig(t))
	s.UseHttp2Listen("127.0.0.1:0")
	calls := 0
	s.RegisterRouteGroup("/g", func(rg *RouteGroup) {
		calls++
		if calls == 2 {
			panic("第二次执行 panic")
		}
		rg.GET("/x", noopHandler)
	})
	if err := s.Start(); !errx.Is(err, CodeStartFailed) {
		t.Errorf("hasRoute 阶段 panic 应转为启动错误：%v", err)
	}
}

func TestServerMaxBodyBytes(t *testing.T) {
	cfg := validConfig(t)
	cfg.MaxBodyBytes = 16
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method: "POST",
		Path:   "/echo",
		Handler: func(c *core.Context) {
			var body map[string]string
			if err := c.BindJSON(&body); err != nil {
				c.Fail(http.StatusBadRequest, http.StatusBadRequest, "请求体过大或非法")
				return
			}
			c.Success("ok", body)
		},
	})
	startServer(t, s)
	client := testHTTPClient()
	base := "https://" + s.ListenerAddr()

	resp, err := client.Post(base+"/echo", "application/json",
		strings.NewReader(`{"data":"这是一个超过十六字节的请求体内容"}`))
	testx.RequireNoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("超大请求体应 413：%d %s", resp.StatusCode, body)
	}
	resp, err = client.Post(base+"/echo", "application/json", strings.NewReader(`{"a":"b"}`))
	testx.RequireNoError(t, err)

	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"a":"b"`) {
		t.Errorf("合法请求体应成功：%d %s", resp.StatusCode, body)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestLogHTTP3Exit(t *testing.T) {
	logger := newTestLogger(t)
	ctx := context.Background()
	logHTTP3Exit(logger, ctx, quic.ErrServerClosed)
	logHTTP3Exit(logger, ctx, errors.New("自定义异常"))
}

func TestServeHTTP3AcceptError(t *testing.T) {
	h3s := &http3.Server{}
	accepter := &errAccepter{err: errors.New("自定义 Accept 错误")}
	if err := serveHTTP3(context.Background(), h3s, accepter); err == nil {
		t.Error("Accept 失败应返回错误")
	}
}

// errAccepter 是 Accept 必然失败的假 QUIC 监听器。
type errAccepter struct {
	err error
}

func (a *errAccepter) Accept(context.Context) (*quic.Conn, error) {
	return nil, a.err
}

func TestServerIntegration(t *testing.T) {
	cfg := validConfig(t)
	cfg.MiddlewareRecovery = true
	cfg.MiddlewareRequestID = true
	cfg.MiddlewareTimeout = true
	cfg.MiddlewareCORS = true
	cfg.MiddlewareValidation = true
	cfg.AccessLogEnabled = true
	cfg.LogSuccessReq = true

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello-webx"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method: "GET",
		Path:   "/ping",
		Handler: func(c *core.Context) {
			c.Success("pong", map[string]string{"id": c.Param("id")})
		},
	})
	s.RegisterRouteGroup("/api/v2", func(rg *RouteGroup) {
		rg.GET("/items", func(c *core.Context) { c.Success("ok", "items") })
	})
	s.ServeStaticDir("/static", dir)
	startServer(t, s)

	client := testHTTPClient()
	base := "https://" + s.ListenerAddr()

	// 正常路由 + 请求 ID
	resp, err := client.Get(base + "/ping")
	testx.RequireNoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "pong") {
		t.Errorf("/ping 不符：%d %s", resp.StatusCode, body)
	}
	if resp.Header.Get("X-Request-ID") == "" {
		t.Error("请求 ID 响应头缺失")
	}

	// 路由组
	resp, err = client.Get(base + "/api/v2/items")
	testx.RequireNoError(t, err)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	testx.Equal(t, resp.StatusCode, http.StatusOK)

	// 健康检查
	resp, err = client.Get(base + "/health")
	testx.RequireNoError(t, err)

	hb, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(hb), "运行中") {
		t.Errorf("/health 不符：%d %s", resp.StatusCode, hb)
	}

	// 404 / 405
	resp, err = client.Get(base + "/nope")
	testx.RequireNoError(t, err)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	testx.Equal(t, resp.StatusCode, http.StatusNotFound)

	req, _ := http.NewRequest(http.MethodPost, base+"/ping", nil)
	resp, err = client.Do(req)
	testx.RequireNoError(t, err)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed || resp.Header.Get("Allow") != "GET, HEAD" {
		t.Errorf("405 不符：%d %s", resp.StatusCode, resp.Header.Get("Allow"))
	}

	// 静态文件
	resp, err = client.Get(base + "/static/a.txt")
	testx.RequireNoError(t, err)

	sb, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(sb) != "hello-webx" {
		t.Errorf("静态文件内容不符：%s", sb)
	}

	// 启动后修改配置仅告警
	if got := s.RegisterRoute(Route{Method: "GET", Path: "/late", Handler: noopHandler}); got != s {
		t.Error("RegisterRoute 应返回自身")
	}
	_ = s.WithLogger(s.logger)
	_ = s.UseGlobalMiddleware(noopMW)
	_ = s.OverrideMiddleware(MiddlewareCORS, noopMW)
	_ = s.DisableMiddleware(MiddlewareCORS)
	_ = s.EnableMiddleware(MiddlewareCORS)
	_ = s.RegisterRoutes(nil)
	_ = s.RegisterRouteGroup("/x", func(rg *RouteGroup) {})
	_ = s.UseHttp2Listen(":0")
	_ = s.UseHttp3Listen(":0")
	_ = s.UseUnixSocketListen("x.sock", 0)
	_ = s.EnableRateLimit(RateLimitOptions{QPS: 1, Window: time.Second})
	_ = s.DisableRateLimit()
	_ = s.ServeStaticDir("/s2", dir)
	_ = s.ServeStaticFS("/s3", http.Dir(dir))
	_ = s.EnableSPA(http.Dir(dir), "index.html")

	// 重复启动
	if err := s.Start(); !errx.Is(err, CodeStartFailed) {
		t.Errorf("重复启动错误不符：%v", err)
	}

	// 优雅关闭（幂等）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	testx.RequireNoError(t, s.Stop(ctx))
	testx.RequireNoError(t, s.Stop(ctx))
}

func TestServerRecoveryAndRateLimit(t *testing.T) {
	cfg := validConfig(t)
	cfg.MiddlewareRecovery = true
	cfg.MiddlewareRequestID = true
	cfg.MiddlewareMetrics = true
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.EnableRateLimit(RateLimitOptions{QPS: 3, Window: time.Second, CleanupInterval: time.Millisecond})
	s.RegisterRoute(Route{
		Method: "GET",
		Path:   "/boom",
		Handler: func(c *core.Context) {
			panic("测试 panic")
		},
	})
	s.RegisterRoute(Route{Method: "GET", Path: "/ok", Handler: func(c *core.Context) { c.Success("ok", nil) }})
	startServer(t, s)

	client := testHTTPClient()
	base := "https://" + s.ListenerAddr()

	// 放行两次 /ok（消耗限流令牌，验证正常路由统计）
	for i := 0; i < 2; i++ {
		resp, err := client.Get(base + "/ok")
		testx.RequireNoError(t, err)

		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		testx.Equal(t, resp.StatusCode, http.StatusOK)

	}

	// /boom 消耗最后一枚令牌并触发 panic
	resp, err := client.Get(base + "/boom")
	testx.RequireNoError(t, err)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	testx.Equal(t, resp.StatusCode, http.StatusInternalServerError)

	// 限流：令牌耗尽后应 429
	resp, _ = client.Get(base + "/ok")
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	testx.Equal(t, resp.StatusCode, http.StatusTooManyRequests)

	if got := resp.Header.Get("Retry-After"); got != "1" {
		t.Errorf("限流响应应带 Retry-After：%s", got)
	}
	m := s.Metrics()
	if m.Panics < 1 || m.RateLimited < 1 {
		t.Errorf("Metrics 扩展计数不符：%+v", m)
	}
	if m.Status5xx < 1 || m.Errors5xx < 1 {
		t.Errorf("panic 应计入 5xx 指标：%+v", m)
	}
	var boomOK, okOK bool
	for _, rs := range s.RouteStats() {
		switch rs.Path {
		case "/boom":
			boomOK = rs.Requests == 1 && rs.Errors5xx == 1
		case "/ok":
			// 被限流拦截的请求不会进入 metrics（限流在外层直接 Abort）。
			okOK = rs.Requests == 2 && rs.Errors5xx == 0
		}
	}
	if !boomOK || !okOK {
		t.Errorf("路由级统计不符：%+v", s.RouteStats())
	}
	if len(s.GroupStats()) != 0 {
		t.Errorf("直接注册路由不应有分组统计：%+v", s.GroupStats())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	testx.RequireNoError(t, s.Stop(ctx))
}

func TestServerRouteGroupStats(t *testing.T) {
	cfg := validConfig(t)
	cfg.MiddlewareMetrics = true
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRouteGroup("/api", func(rg *RouteGroup) {
		rg.GET("/users/:id", func(c *core.Context) { c.Success("ok", nil) })
		rg.GET("/admin", func(c *core.Context) {
			c.Fail(http.StatusInternalServerError, http.StatusInternalServerError, "内部错误")
		})
	})
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/ping",
		Handler: func(c *core.Context) { c.Success("ok", nil) },
	})
	startServer(t, s)
	client := testHTTPClient()
	base := "https://" + s.ListenerAddr()
	for _, path := range []string{"/api/users/1", "/api/users/2", "/api/admin", "/ping"} {
		resp, err := client.Get(base + path)
		testx.RequireNoError(t, err)

		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	routes := s.RouteStats()
	if len(routes) != 3 {
		t.Fatalf("路由统计数量不符：%+v", routes)
	}
	var users, admin, ping bool
	for _, rs := range routes {
		switch rs.Path {
		case "/api/users/:id":
			users = rs.Requests == 2 && rs.Errors5xx == 0
		case "/api/admin":
			admin = rs.Requests == 1 && rs.Errors5xx == 1
		case "/ping":
			ping = rs.Requests == 1 && rs.Errors5xx == 0
		}
	}
	if !users || !admin || !ping {
		t.Errorf("路由级统计不符：%+v", routes)
	}
	groups := s.GroupStats()
	if len(groups) != 1 || groups[0].Prefix != "/api" ||
		groups[0].Requests != 3 || groups[0].Errors5xx != 1 {
		t.Errorf("分组级统计不符：%+v", groups)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerRateLimitKeyFunc(t *testing.T) {
	cfg := validConfig(t)
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.EnableRateLimit(RateLimitOptions{
		QPS:     1,
		Window:  time.Second,
		KeyFunc: func(c *core.Context) string { return c.Query("user") },
	})
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/rl",
		Handler: func(c *core.Context) { c.Success("ok", nil) },
	})
	startServer(t, s)
	client := testHTTPClient()
	base := "https://" + s.ListenerAddr() + "/rl?user="

	for i := 0; i < 2; i++ {
		resp, err := client.Get(base + "a")
		testx.RequireNoError(t, err)

		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		want := http.StatusOK
		if i == 1 {
			want = http.StatusTooManyRequests
		}
		testx.Equal(t, resp.StatusCode, want)

	}
	resp, err := client.Get(base + "b")
	testx.RequireNoError(t, err)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	testx.Equal(t, resp.StatusCode, http.StatusOK)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerGzipAndMetrics(t *testing.T) {
	cfg := validConfig(t)
	cfg.MiddlewareGzip = true
	cfg.MiddlewareMetrics = true
	cfg.MiddlewareRecovery = true
	cfg.GzipLevel = 9
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method: "GET",
		Path:   "/ok",
		Handler: func(c *core.Context) {
			_ = c.String(http.StatusOK, "你好 webx")
		},
	})
	s.RegisterRoute(Route{
		Method: "GET",
		Path:   "/err",
		Handler: func(c *core.Context) {
			c.Fail(http.StatusInternalServerError, http.StatusInternalServerError, "内部错误")
		},
	})
	startServer(t, s)
	client := testHTTPClient()
	base := "https://" + s.ListenerAddr()

	// gzip 压缩
	req, _ := http.NewRequest(http.MethodGet, base+"/ok", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := client.Do(req)
	testx.RequireNoError(t, err)

	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("应返回 gzip：%v", resp.Header)
	}
	zr, err := gzip.NewReader(resp.Body)
	testx.RequireNoError(t, err)

	got, _ := io.ReadAll(zr)
	zr.Close()
	resp.Body.Close()
	if string(got) != "你好 webx" {
		t.Errorf("解压内容不符：%s", got)
	}

	// 未协商 gzip → 明文
	resp, _ = client.Get(base + "/ok")
	plain, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(plain) != "你好 webx" {
		t.Errorf("明文响应不符：%s", plain)
	}

	// 5xx 计数
	resp, _ = client.Get(base + "/err")
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	testx.Equal(t, resp.StatusCode, http.StatusInternalServerError)

	m := s.Metrics()
	if m.Requests < 3 || m.Errors5xx < 1 {
		t.Errorf("Metrics 不符：%+v", m)
	}
	if m.Status2xx < 2 || m.Status5xx < 1 || m.Status4xx != 0 {
		t.Errorf("状态码分布不符：%+v", m)
	}
	if m.HTTP1Requests < 3 {
		t.Errorf("HTTP/1 请求计数不符：%+v", m)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestMetricsProtocolHTTP2(t *testing.T) {
	cfg := validConfig(t)
	cfg.MiddlewareMetrics = true
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/ok",
		Handler: func(c *core.Context) { c.Success("ok", nil) },
	})
	startServer(t, s)
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			ForceAttemptHTTP2: true,
		},
	}
	resp, err := client.Get("https://" + s.ListenerAddr() + "/ok")
	testx.RequireNoError(t, err)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.ProtoMajor != 2 {
		t.Errorf("客户端应协商 HTTP/2：%s", resp.Proto)
	}
	m := s.Metrics()
	if m.HTTP2Requests < 1 || m.Status2xx < 1 || m.HTTP1Requests != 0 {
		t.Errorf("HTTP/2 指标不符：%+v", m)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestMetricsActiveConnections(t *testing.T) {
	cfg := validConfig(t)
	cfg.MiddlewareMetrics = true
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/ok",
		Handler: func(c *core.Context) { c.Success("ok", nil) },
	})
	startServer(t, s)
	resp, err := testHTTPClient().Get("https://" + s.ListenerAddr() + "/ok")
	testx.RequireNoError(t, err)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if m := s.Metrics(); m.ActiveConnections < 1 {
		t.Errorf("ActiveConnections 应 >= 1：%+v", m)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerConnContextAndOnShutdown(t *testing.T) {
	type ctxKey struct{}
	cfg := validConfig(t)
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.SetConnContext(func(ctx context.Context, _ net.Conn) context.Context {
		return context.WithValue(ctx, ctxKey{}, "conn-value")
	})
	shutdownCh := make(chan struct{}, 1)
	s.RegisterOnShutdown(func() { shutdownCh <- struct{}{} })
	s.RegisterRoute(Route{
		Method: "GET",
		Path:   "/ctx",
		Handler: func(c *core.Context) {
			_ = c.String(http.StatusOK, "%s", c.Request().Context().Value(ctxKey{}))
		},
	})
	startServer(t, s)
	resp, err := testHTTPClient().Get("https://" + s.ListenerAddr() + "/ctx")
	testx.RequireNoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "conn-value" {
		t.Errorf("连接上下文未注入：%s", body)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
	select {
	case <-shutdownCh:
	case <-time.After(2 * time.Second):
		t.Error("关闭钩子未执行")
	}
}

func TestServerSecurityHeaders(t *testing.T) {
	cfg := validConfig(t)
	cfg.MiddlewareSecurity = true
	cfg.SecurityHSTSMaxAge = 3600
	cfg.SecurityReferrerPolicy = "no-referrer"
	cfg.SecurityCrossOriginResourcePolicy = "same-origin"
	cfg.SecurityCrossOriginEmbedderPolicy = "require-corp"
	cfg.SecurityContentSecurityPolicy = "default-src 'self'"
	cfg.SecurityHSTSIncludeSubDomains = true
	cfg.SecurityOriginAgentCluster = true
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/ok",
		Handler: func(c *core.Context) { c.Success("ok", nil) },
	})
	startServer(t, s)
	resp, err := testHTTPClient().Get("https://" + s.ListenerAddr() + "/ok")
	testx.RequireNoError(t, err)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" ||
		resp.Header.Get("X-Frame-Options") != "DENY" ||
		resp.Header.Get("Referrer-Policy") != "no-referrer" ||
		resp.Header.Get("Strict-Transport-Security") != "max-age=3600; includeSubDomains" ||
		resp.Header.Get("Cross-Origin-Resource-Policy") != "same-origin" ||
		resp.Header.Get("Cross-Origin-Embedder-Policy") != "require-corp" ||
		resp.Header.Get("Content-Security-Policy") != "default-src 'self'" ||
		resp.Header.Get("Origin-Agent-Cluster") != "?1" {
		t.Errorf("安全头缺失：%v", resp.Header)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerCORSPNA(t *testing.T) {
	cfg := validConfig(t)
	cfg.MiddlewareCORS = true
	cfg.CORSAllowPrivateNetwork = true
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/ok",
		Handler: func(c *core.Context) { c.Success("ok", nil) },
	})
	startServer(t, s)
	req, err := http.NewRequest(http.MethodGet, "https://"+s.ListenerAddr()+"/ok", nil)
	testx.RequireNoError(t, err)

	req.Header.Set("Origin", "https://a.com")
	resp, err := testHTTPClient().Do(req)
	testx.RequireNoError(t, err)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Private-Network"); got != "true" {
		t.Errorf("内网预检头缺失：%s", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerTrustedProxies(t *testing.T) {
	registerIPRoute := func(s *Server) {
		s.RegisterRoute(Route{
			Method: "GET",
			Path:   "/ip",
			Handler: func(c *core.Context) {
				c.Success("ok", c.RemoteIP())
			},
		})
	}

	// 未配置可信代理：XFF 被忽略，返回 RemoteAddr
	cfg := validConfig(t)
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	registerIPRoute(s)
	startServer(t, s)
	client := testHTTPClient()
	req, _ := http.NewRequest(http.MethodGet, "https://"+s.ListenerAddr()+"/ip", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	resp, err := client.Do(req)
	testx.RequireNoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	testx.RequireTrue(t, strings.Contains(string(body), "127.0.0.1"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)

	// 配置可信代理：XFF 生效
	cfg2 := validConfig(t)
	cfg2.TrustedProxies = []string{"127.0.0.1/32"}
	s2 := newTestServer(t, cfg2)
	s2.UseHttp2Listen("127.0.0.1:0")
	registerIPRoute(s2)
	startServer(t, s2)
	req, _ = http.NewRequest(http.MethodGet, "https://"+s2.ListenerAddr()+"/ip", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	resp, err = client.Do(req)
	testx.RequireNoError(t, err)

	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	testx.RequireTrue(t, strings.Contains(string(body), "203.0.113.9"))
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s2.Stop(ctx)
}

func TestServerMiddlewareCombo(t *testing.T) {
	cfg := validConfig(t)
	cfg.MiddlewareRecovery = true
	cfg.MiddlewareRequestID = true
	cfg.MiddlewareTimeout = true
	cfg.MiddlewareGzip = true
	cfg.MiddlewareMetrics = true
	cfg.AccessLogEnabled = true
	cfg.LogSuccessReq = true
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method: "GET",
		Path:   "/ok",
		Handler: func(c *core.Context) {
			_ = c.String(http.StatusOK, "组合测试")
		},
	})
	startServer(t, s)
	client := testHTTPClient()
	base := "https://" + s.ListenerAddr()

	req, _ := http.NewRequest(http.MethodGet, base+"/ok", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := client.Do(req)
	testx.RequireNoError(t, err)

	if resp.Header.Get("X-Request-ID") == "" || resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("组合响应头不符：%v", resp.Header)
	}
	zr, err := gzip.NewReader(resp.Body)
	testx.RequireNoError(t, err)

	got, _ := io.ReadAll(zr)
	zr.Close()
	resp.Body.Close()
	if string(got) != "组合测试" {
		t.Errorf("解压内容不符：%s", got)
	}
	if m := s.Metrics(); m.Requests < 1 {
		t.Errorf("组合 Metrics 不符：%+v", m)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerSSEFlush(t *testing.T) {
	cfg := validConfig(t)
	cfg.MiddlewareTimeout = true
	cfg.MiddlewareGzip = true
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method: "GET",
		Path:   "/sse",
		Handler: func(c *core.Context) {
			rc := http.NewResponseController(c.Writer())
			_, _ = c.Writer().Write([]byte("a"))
			_ = rc.Flush()
			_, _ = c.Writer().Write([]byte("b"))
			_ = rc.Flush()
		},
	})
	startServer(t, s)
	client := testHTTPClient()
	req, _ := http.NewRequest(http.MethodGet, "https://"+s.ListenerAddr()+"/sse", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := client.Do(req)
	testx.RequireNoError(t, err)

	zr, err := gzip.NewReader(resp.Body)
	testx.RequireNoError(t, err)

	got, _ := io.ReadAll(zr)
	zr.Close()
	resp.Body.Close()
	if string(got) != "ab" {
		t.Errorf("Flush 透传内容不符：%s", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerHTTP3Concurrent(t *testing.T) {
	cfg := validConfig(t)
	h3Addr := freeUDPAddr(t)
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.UseHttp3Listen(h3Addr)
	s.RegisterRoute(Route{
		Method: "GET",
		Path:   "/ping",
		Handler: func(c *core.Context) {
			c.Success("h3", nil)
		},
	})
	startServer(t, s)

	transport := &http3.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	defer transport.Close()
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second}

	var wg sync.WaitGroup
	errCh := make(chan error, 15)
	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				resp, err := client.Get("https://" + h3Addr + "/ping")
				if err != nil {
					errCh <- err
					return
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					errCh <- fmt.Errorf("状态码不符：%d", resp.StatusCode)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("HTTP/3 并发请求失败：%v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerSetCertificateLoader(t *testing.T) {
	cert, key := writeTestCert(t)
	s := newTestServer(t, validConfig(t))
	s.UseHttp2Listen("127.0.0.1:0")
	s.SetCertificateLoader(func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		c, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return nil, err
		}
		return &c, nil
	})
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/ping",
		Handler: func(c *core.Context) { c.Success("pong", nil) },
	})
	startServer(t, s)
	resp, err := testHTTPClient().Get("https://" + s.ListenerAddr() + "/ping")
	testx.RequireNoError(t, err)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	testx.Equal(t, resp.StatusCode, http.StatusOK)

	// 启动后设置不生效（仅告警）
	if got := s.SetCertificateLoader(nil); got != s {
		t.Error("SetCertificateLoader 应返回自身")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerSNICertificates(t *testing.T) {
	certA, keyA := writeTestCert(t)
	certB, keyB := writeTestCert(t)
	cfg := validConfig(t)
	s := newTestServer(t, cfg)
	s.SetSNICertificates([]SNICertificate{
		{ServerName: "a.example.com", CertFile: certA, KeyFile: keyA},
		{ServerName: "b.example.com", CertFile: certB, KeyFile: keyB},
		{ServerName: "", CertFile: certA, KeyFile: keyA}, // 非法条目被过滤
	})
	if len(s.sniCerts) != 2 {
		t.Fatalf("SNI 条目过滤不符：%d", len(s.sniCerts))
	}
	getCert := s.buildGetCertificate()
	ca, err := getCert(&tls.ClientHelloInfo{ServerName: "a.example.com"})
	testx.RequireNoError(t, err)

	cb, err := getCert(&tls.ClientHelloInfo{ServerName: "b.example.com"})
	testx.RequireNoError(t, err)

	if string(ca.Certificate[0]) == string(cb.Certificate[0]) {
		t.Error("不同 SNI 应返回不同证书")
	}
	defCert := newCertificateProvider(cfg.TLSCertFile, cfg.TLSKeyFile)
	// 未匹配 SNI 回退默认证书
	fallback, err := getCert(&tls.ClientHelloInfo{ServerName: "unknown.example.com"})
	testx.RequireNoError(t, err)

	defaultLoaded, err := defCert.getCertificate(nil)
	testx.RequireNoError(t, err)

	if string(fallback.Certificate[0]) != string(defaultLoaded.Certificate[0]) {
		t.Error("未匹配 SNI 应回退默认证书")
	}
}

func TestServerQUICDrain(t *testing.T) {
	cfg := validConfig(t)
	cfg.QUICDrainTimeout = 10 * time.Millisecond
	s := newTestServer(t, cfg)
	s.UseHttp3Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/ping",
		Handler: func(c *core.Context) { c.Success("h3", nil) },
	})
	startServer(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	testx.RequireNoError(t, s.Stop(ctx))
}

func TestSetSNIStartedGuard(t *testing.T) {
	s := &Server{started: true}
	if got := s.SetSNICertificates(nil); got != s {
		t.Error("SetSNICertificates 应返回自身")
	}
}

func TestServerHooksStartedGuard(t *testing.T) {
	s := &Server{started: true}
	if got := s.SetConnContext(nil); got != s {
		t.Error("SetConnContext 应返回自身")
	}
	if got := s.RegisterOnShutdown(nil); got != s {
		t.Error("RegisterOnShutdown 应返回自身")
	}
}

func TestSetMiddlewareOrderStartedGuard(t *testing.T) {
	s := &Server{started: true}
	if got := s.SetMiddlewareOrder(nil); got != s {
		t.Error("SetMiddlewareOrder 应返回自身")
	}
}

func TestSetRequestIDOptionsStartedGuard(t *testing.T) {
	s := &Server{started: true}
	if got := s.SetRequestIDOptions(RequestIDOptions{Header: "X"}); got != s {
		t.Error("SetRequestIDOptions 应返回自身")
	}
}

func TestSetMaxConcurrentRequestsStartedGuard(t *testing.T) {
	s := &Server{started: true}
	if got := s.SetMaxConcurrentRequests(1); got != s {
		t.Error("SetMaxConcurrentRequests 应返回自身")
	}
}

func TestSetErrorMessagesStartedGuard(t *testing.T) {
	s := &Server{started: true}
	if got := s.SetErrorMessages(nil); got != s {
		t.Error("SetErrorMessages 应返回自身")
	}
}

func TestServerRequestIDOptions(t *testing.T) {
	cfg := validConfig(t)
	cfg.MiddlewareRequestID = true
	s := newTestServer(t, cfg)
	s.SetRequestIDOptions(RequestIDOptions{
		Header:    "X-Trace-ID",
		Generator: func() string { return "trace-abc" },
	})
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/ok",
		Handler: func(c *core.Context) { c.Success("ok", nil) },
	})
	startServer(t, s)
	resp, err := testHTTPClient().Get("https://" + s.ListenerAddr() + "/ok")
	testx.RequireNoError(t, err)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-Trace-ID") != "trace-abc" {
		t.Errorf("自定义请求 ID 未生效：%d %v", resp.StatusCode, resp.Header)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerMaxConcurrentRequests(t *testing.T) {
	cfg := validConfig(t)
	s := newTestServer(t, cfg)
	s.SetMaxConcurrentRequests(1)
	s.UseHttp2Listen("127.0.0.1:0")
	entered := make(chan struct{})
	release := make(chan struct{})
	var enteredOnce sync.Once
	s.RegisterRoute(Route{
		Method: "GET",
		Path:   "/slow",
		Handler: func(c *core.Context) {
			enteredOnce.Do(func() { close(entered) })
			<-release
			c.Success("ok", nil)
		},
	})
	startServer(t, s)
	client := testHTTPClient()
	base := "https://" + s.ListenerAddr()

	slowDone := make(chan struct{})
	go func() {
		defer close(slowDone)
		resp, err := client.Get(base + "/slow")
		if err != nil {
			t.Errorf("慢请求失败：%v", err)
			return
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	<-entered

	resp, err := client.Get(base + "/slow")
	testx.RequireNoError(t, err)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	testx.Equal(t, resp.StatusCode, http.StatusServiceUnavailable)

	if got := resp.Header.Get("Retry-After"); got != "1" {
		t.Errorf("应携带 Retry-After：%s", got)
	}
	if got := s.Metrics().ConcurrencyRejected; got < 1 {
		t.Errorf("并发拒绝计数不符：%d", got)
	}
	close(release)
	<-slowDone

	resp, err = client.Get(base + "/slow")
	testx.RequireNoError(t, err)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	testx.Equal(t, resp.StatusCode, http.StatusOK)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerErrorMessages(t *testing.T) {
	cfg := validConfig(t)
	cfg.MaxBodyBytes = 16
	cfg.RequestTimeout = 50 * time.Millisecond
	cfg.MiddlewareTimeout = true
	cfg.MiddlewareRecovery = true
	cfg.ErrorMessages = map[string]string{
		ErrorMessageMethodNotAllowed: "自定义 405",
		ErrorMessageBodyTooLarge:     "自定义 413",
		ErrorMessageRateLimited:      "自定义 429",
		ErrorMessageTooBusy:          "自定义 503 繁忙",
		ErrorMessageTimeout:          "自定义 503 超时",
	}
	s := newTestServer(t, cfg)
	s.SetMaxConcurrentRequests(1)
	s.EnableRateLimit(RateLimitOptions{
		QPS:     2,
		Window:  time.Second,
		KeyFunc: func(c *core.Context) string { return c.Request().URL.Path },
	})
	s.SetErrorMessages(map[string]string{ErrorMessageNotFound: "流式 404"})
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{Method: "GET", Path: "/ok", Handler: func(c *core.Context) { c.Success("ok", nil) }})
	s.RegisterRoute(Route{
		Method: "POST",
		Path:   "/ok",
		Handler: func(c *core.Context) {
			_, _ = io.ReadAll(c.Request().Body)
			c.Success("ok", nil)
		},
	})
	s.RegisterRoute(Route{
		Method: "GET",
		Path:   "/slow",
		Handler: func(c *core.Context) {
			time.Sleep(500 * time.Millisecond)
			c.Success("ok", nil)
		},
	})
	startServer(t, s)
	client := testHTTPClient()
	base := "https://" + s.ListenerAddr()

	check := func(name string, resp *http.Response, wantCode int, wantMsg string) {
		t.Helper()
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != wantCode || !strings.Contains(string(body), wantMsg) {
			t.Errorf("%s 不符：%d %s（期望 %d 含 %s）", name, resp.StatusCode, body, wantCode, wantMsg)
		}
	}

	resp, err := client.Get(base + "/missing")
	testx.RequireNoError(t, err)

	check("404", resp, http.StatusNotFound, "流式 404")

	patchReq, _ := http.NewRequest(http.MethodPatch, base+"/ok", nil)
	resp, err = client.Do(patchReq)
	testx.RequireNoError(t, err)

	check("405", resp, http.StatusMethodNotAllowed, "自定义 405")

	resp, err = client.Post(base+"/ok", "application/json", strings.NewReader(strings.Repeat("x", 100)))
	testx.RequireNoError(t, err)

	check("413", resp, http.StatusRequestEntityTooLarge, "自定义 413")

	for i := 0; i < 3; i++ {
		resp, err = client.Get(base + "/ok")
		testx.RequireNoError(t, err)

		if i == 2 {
			check("429", resp, http.StatusTooManyRequests, "自定义 429")
		} else {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}

	slowDone := make(chan struct{})
	go func() {
		defer close(slowDone)
		resp, err := client.Get(base + "/slow")
		if err != nil {
			return
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	time.Sleep(100 * time.Millisecond)
	resp, err = client.Get(base + "/slow")
	testx.RequireNoError(t, err)

	check("503 繁忙", resp, http.StatusServiceUnavailable, "自定义 503 繁忙")
	<-slowDone

	resp, err = client.Get(base + "/slow")
	testx.RequireNoError(t, err)

	check("503 超时", resp, http.StatusServiceUnavailable, "自定义 503 超时")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerMiddlewareOrder(t *testing.T) {
	cfg := validConfig(t)
	cfg.MiddlewareRequestID = true
	cfg.MiddlewareRecovery = true
	s := newTestServer(t, cfg)
	s.SetMiddlewareOrder([]MiddlewareType{MiddlewareRequestID, MiddlewareRecovery})
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/ok",
		Handler: func(c *core.Context) { c.Success("ok", nil) },
	})
	startServer(t, s)
	resp, err := testHTTPClient().Get("https://" + s.ListenerAddr() + "/ok")
	testx.RequireNoError(t, err)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-Request-ID") == "" {
		t.Errorf("自定义顺序服务不符：%d %v", resp.StatusCode, resp.Header)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerHTTP3(t *testing.T) {
	cfg := validConfig(t)
	cfg.MiddlewareRequestID = true
	cfg.MiddlewareMetrics = true
	h3Addr := freeUDPAddr(t)
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.UseHttp3Listen(h3Addr)
	s.RegisterRoute(Route{Method: "GET", Path: "/ping", Handler: func(c *core.Context) { c.Success("h3", nil) }})
	startServer(t, s)

	transport := &http3.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	defer transport.Close()
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	resp, err := client.Get("https://" + h3Addr + "/ping")
	testx.RequireNoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "h3") {
		t.Errorf("HTTP/3 响应不符：%d %s", resp.StatusCode, body)
	}
	m := s.Metrics()
	if m.HTTP3Requests < 1 || m.Status2xx < 1 || m.HTTP1Requests != 0 {
		t.Errorf("HTTP/3 指标不符：%+v", m)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	testx.RequireNoError(t, s.Stop(ctx))
}

func TestServerHTTP3OnlyListenerAddr(t *testing.T) {
	cfg := validConfig(t)
	s := newTestServer(t, cfg)
	s.UseHttp3Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/ping",
		Handler: func(c *core.Context) { c.Success("h3", nil) },
	})
	startServer(t, s)
	if addr := s.ListenerAddr(); addr == "" {
		t.Error("HTTP/3-only 时 ListenerAddr 不应为空")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerUnixSocket(t *testing.T) {
	if err := unixSocketSupported(); err != nil {
		t.Skipf("当前平台不支持 Unix Socket：%v", err)
	}
	path := filepath.Join(t.TempDir(), "webx.sock")
	s := newTestServer(t, validConfig(t))
	s.UseUnixSocketListen(path, 0o600)
	s.RegisterRoute(Route{Method: "GET", Path: "/ping", Handler: func(c *core.Context) { c.Success("unix", nil) }})
	startServer(t, s)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", path)
			},
		},
	}
	resp, err := client.Get("http://unix/ping")
	testx.RequireNoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "unix") {
		t.Errorf("Unix 响应不符：%d %s", resp.StatusCode, body)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	testx.RequireNoError(t, s.Stop(ctx))
	if runtime.GOOS != "windows" {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("Socket 文件应被清理：%v", err)
		}
	}
}

func TestServerStartRollback(t *testing.T) {
	s := newTestServer(t, validConfig(t))
	s.UseHttp2Listen("127.0.0.1:0")
	s.UseHttp3Listen("bad-addr")
	if err := s.Start(); err == nil {
		t.Error("非法 QUIC 地址应启动失败")
	}
	// HTTP/2 + HTTP/3 成功、Unix 失败 → 回滚关闭全部监听器
	s = newTestServer(t, validConfig(t))
	s.UseHttp2Listen("127.0.0.1:0")
	s.UseHttp3Listen(freeUDPAddr(t))
	dir := t.TempDir()
	_ = os.Mkdir(filepath.Join(dir, "sub"), 0o700)
	_ = os.WriteFile(filepath.Join(dir, "sub", "x"), []byte("x"), 0o600)
	s.UseUnixSocketListen(dir, 0o600) // 目录 → 创建失败
	if err := s.Start(); err == nil {
		t.Error("Unix 目录路径应启动失败")
	}
}

func TestServerHealthUserRegistered(t *testing.T) {
	s := newTestServer(t, validConfig(t))
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method: "GET",
		Path:   "/health",
		Handler: func(c *core.Context) {
			_ = c.String(http.StatusOK, "custom-health")
		},
	})
	startServer(t, s)
	resp, err := testHTTPClient().Get("https://" + s.ListenerAddr() + "/health")
	testx.RequireNoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "custom-health" {
		t.Errorf("用户自定义健康检查应生效：%s", body)
	}
	_ = body
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)

	// 通过路由分组注册 /health 同样跳过自动注册
	s2 := newTestServer(t, validConfig(t))
	s2.UseHttp2Listen("127.0.0.1:0")
	s2.RegisterRouteGroup("", func(rg *RouteGroup) {
		rg.GET("/health", func(c *core.Context) { _ = c.String(http.StatusOK, "group-health") })
	})
	startServer(t, s2)
	resp, err = testHTTPClient().Get("https://" + s2.ListenerAddr() + "/health")
	testx.RequireNoError(t, err)

	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "group-health" {
		t.Errorf("分组自定义健康检查应生效：%s", body)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	_ = s2.Stop(ctx2)
}

func TestServerHealthChecks(t *testing.T) {
	s := newTestServer(t, validConfig(t))
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterHealthCheck("db", func(ctx context.Context) error { return nil })
	s.RegisterHealthCheck("redis", func(ctx context.Context) error { return errors.New("连接失败") })
	startServer(t, s)
	client := testHTTPClient()
	resp, err := client.Get("https://" + s.ListenerAddr() + "/health")
	testx.RequireNoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	testx.Equal(t, resp.StatusCode, http.StatusServiceUnavailable)

	if !strings.Contains(string(body), "redis") || !strings.Contains(string(body), "连接失败") {
		t.Errorf("检查结果不符：%s", body)
	}
	// 启动后注册不生效（仅告警）
	if got := s.RegisterHealthCheck("late", func(ctx context.Context) error { return nil }); got != s {
		t.Error("RegisterHealthCheck 应返回自身")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerLivenessReadiness(t *testing.T) {
	cfg := validConfig(t)
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterLivenessCheck("live", func(ctx context.Context) error { return nil })
	s.RegisterReadinessCheck("db", func(ctx context.Context) error { return errors.New("数据库未就绪") })
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/ok",
		Handler: func(c *core.Context) { c.Success("ok", nil) },
	})
	startServer(t, s)
	client := testHTTPClient()
	base := "https://" + s.ListenerAddr()

	resp, err := client.Get(base + "/healthz")
	testx.RequireNoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "运行中") {
		t.Errorf("存活探针不符：%d %s", resp.StatusCode, body)
	}

	resp, err = client.Get(base + "/readyz")
	testx.RequireNoError(t, err)

	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(body), "数据库未就绪") {
		t.Errorf("就绪探针不符：%d %s", resp.StatusCode, body)
	}

	resp, err = client.Get(base + "/health")
	testx.RequireNoError(t, err)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	testx.Equal(t, resp.StatusCode, http.StatusOK)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestRegisterProbeStartedGuard(t *testing.T) {
	s := &Server{started: true}
	if got := s.RegisterLivenessCheck("x", func(ctx context.Context) error { return nil }); got != s {
		t.Error("RegisterLivenessCheck 应返回自身")
	}
	if got := s.RegisterReadinessCheck("x", func(ctx context.Context) error { return nil }); got != s {
		t.Error("RegisterReadinessCheck 应返回自身")
	}
}

func TestShutdownMarksReadinessDown(t *testing.T) {
	cfg := validConfig(t)
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/ok",
		Handler: func(c *core.Context) { c.Success("ok", nil) },
	})
	startServer(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
	if !s.shuttingDown.Load() {
		t.Error("Stop 后应标记服务关闭中")
	}
}

func TestRegisterHealthCheckInvalid(t *testing.T) {
	s := newTestServer(t, validConfig(t))
	if got := s.RegisterHealthCheck("", func(ctx context.Context) error { return nil }); got != s {
		t.Error("空名称应返回自身")
	}
	if got := s.RegisterHealthCheck("x", nil); got != s {
		t.Error("nil 函数应返回自身")
	}
	if len(s.healthChecks) != 0 {
		t.Error("非法检查项不应注册")
	}
}

func TestServerShutdownErrorWrapped(t *testing.T) {
	logger := newTestLogger(t)
	defer logger.Close()
	nonEmptyDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(nonEmptyDir, "x"), []byte("x"), 0o600)
	// 直接构造未运行的 Server 并触发关闭错误，避免与 Start 并发读写字段。
	s := &Server{
		logger:         logger,
		config:         Config{ShutdownTimeout: time.Second},
		unixEnabled:    true,
		unixSocketPath: nonEmptyDir,
	}
	if err := s.shutdownAll(context.Background()); !errx.Is(err, CodeShutdownFailed) {
		t.Errorf("关闭错误应包装为 WEBX_SHUTDOWN_FAILED：%v", err)
	}
}

func TestServerUnixServeErrorLogged(t *testing.T) {
	s := newTestServer(t, validConfig(t))
	s.UseUnixSocketListen(filepath.Join(t.TempDir(), "x.sock"), 0o600)
	errCh := make(chan error, 1)
	go func() { errCh <- s.Start() }()
	deadline := time.After(5 * time.Second)
	for s.ListenerAddr() == "" {
		select {
		case err := <-errCh:
			t.Fatalf("Start 失败：%v", err)
		case <-deadline:
			t.Fatal("启动超时")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	// 直接关闭监听器，使 Serve 返回非 ErrServerClosed 错误（触发错误日志分支）
	s.closeListeners()
	select {
	case err := <-errCh:
		testx.NoError(t, err)

	case <-time.After(2 * time.Second):
		t.Fatal("服务未退出")
	}
}

func TestRegisterBuiltinMiddlewareDefaults(t *testing.T) {
	s := newTestServer(t, Config{})
	s.registerBuiltinMiddleware()
	chain := s.mwManager.Build(context.Background())
	if len(chain) != 0 {
		t.Errorf("默认配置下不应启用任何中间件：%d", len(chain))
	}
}

func TestRegisterBuiltinMiddlewareCORSExposeOverride(t *testing.T) {
	s := newTestServer(t, Config{
		MiddlewareCORS:          true,
		CORSExposeHeaders:       []string{"X-Trace-ID"},
		CORSAllowPrivateNetwork: true,
	})
	s.registerBuiltinMiddleware()
	chain := s.mwManager.Build(context.Background())
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers(stdChainToCore(chain, func(c *core.Context) { c.Success("ok", nil) }))
	c.Run()
	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "X-Trace-ID" {
		t.Errorf("默认 CORS 未保留自定义 ExposeHeaders：%s", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Private-Network"); got != "true" {
		t.Errorf("默认 CORS 未保留内网预检开关：%s", got)
	}
}

func TestServerStaticAndSPA(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log(1)"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>spa</html>"), 0o600)

	s := newTestServer(t, validConfig(t))
	s.UseHttp2Listen("127.0.0.1:0")
	s.ServeStaticDir("/static", dir)
	s.EnableSPA(http.Dir(dir), "index.html")
	startServer(t, s)
	client := testHTTPClient()
	base := "https://" + s.ListenerAddr()

	resp, _ := client.Get(base + "/static/app.js")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "console.log(1)" {
		t.Errorf("静态文件不符：%s", body)
	}
	resp, _ = client.Get(base + "/app/route")
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "<html>spa</html>" {
		t.Errorf("SPA 回退不符：%s", body)
	}
	req, _ := http.NewRequest(http.MethodPost, base+"/app/route", nil)
	resp, _ = client.Do(req)
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	testx.Equal(t, resp.StatusCode, http.StatusNotFound)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerWaitSignal(t *testing.T) {
	logger := newTestLogger(t)
	defer logger.Close()
	s := &Server{logger: logger, config: Config{ShutdownTimeout: time.Second}}
	s.signalCtx, s.signalCancel = context.WithCancel(context.Background())
	quit := make(chan os.Signal, 1)
	done := make(chan struct{})
	go func() {
		s.waitSignal(quit)
		close(done)
	}()
	quit <- syscall.SIGTERM
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("waitSignal 未在信号后返回")
	}

	s2 := &Server{logger: logger, config: Config{ShutdownTimeout: time.Second}}
	s2.signalCtx, s2.signalCancel = context.WithCancel(context.Background())
	done2 := make(chan struct{})
	go func() {
		s2.waitSignal(make(chan os.Signal))
		close(done2)
	}()
	s2.signalCancel()
	select {
	case <-done2:
	case <-time.After(2 * time.Second):
		t.Fatal("waitSignal 未在取消后返回")
	}
}

// freeUDPAddr 返回一个空闲的 UDP 地址（127.0.0.1:port）。
func freeUDPAddr(t *testing.T) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	testx.RequireNoError(t, err)

	addr := pc.LocalAddr().String()
	pc.Close()
	return addr
}

// TestGlobalMiddlewareCoversFallbacks 验证全局中间件覆盖 404/405。
func TestGlobalMiddlewareCoversFallbacks(t *testing.T) {
	s := newTestServer(t, validConfig(t))
	var hits atomic.Int32
	s.UseGlobalMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			next.ServeHTTP(w, r)
		})
	})
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{Method: http.MethodGet, Path: "/hello",
		Handler: func(c *Context) { _ = c.String(http.StatusOK, "ok") }})
	startServer(t, s)
	defer s.Stop(context.Background())

	base := "https://" + s.ListenerAddr()
	resp, err := testHTTPClient().Get(base + "/missing")
	testx.RequireNoError(t, err)

	_ = resp.Body.Close()
	testx.RequireEqual(t, resp.StatusCode, http.StatusNotFound)

	req, _ := http.NewRequest(http.MethodPost, base+"/hello", nil)
	resp2, err := testHTTPClient().Do(req)
	testx.RequireNoError(t, err)

	_ = resp2.Body.Close()
	testx.RequireEqual(t, resp2.StatusCode, http.StatusMethodNotAllowed)

	if hits.Load() != 2 {
		t.Fatalf("全局中间件应覆盖 404/405，实际命中：%d", hits.Load())
	}
}
