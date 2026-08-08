package webx

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRenderMetricsFull(t *testing.T) {
	m := Metrics{
		Requests:                  10,
		Errors5xx:                 2,
		Status1xx:                 1,
		Status2xx:                 5,
		Status3xx:                 1,
		Status4xx:                 1,
		RateLimited:               3,
		ConcurrencyRejected:       5,
		Panics:                    1,
		AvgRequestDurationMs:      12,
		HTTP1Requests:             7,
		HTTP2Requests:             2,
		HTTP3Requests:             1,
		AvgHTTP1RequestDurationMs: 9,
		AvgHTTP2RequestDurationMs: 2,
		AvgHTTP3RequestDurationMs: 1,
		ActiveConnections:         4,
		RequestsInFlight:          2,
	}
	routes := []RouteStat{{
		Path:                 "/a",
		Requests:             6,
		Errors5xx:            1,
		AvgRequestDurationMs: 8,
	}}
	groups := []GroupStat{{
		Prefix:               "/g",
		Requests:             6,
		Errors5xx:            1,
		AvgRequestDurationMs: 8,
	}}
	got := renderMetrics(m, routes, groups)
	for _, want := range []string{
		"# HELP webx_requests_total 请求总数",
		"# TYPE webx_requests_total counter",
		"webx_requests_total 10",
		"webx_requests_5xx_total 2",
		"webx_requests_1xx_total 1",
		"webx_requests_2xx_total 5",
		"webx_requests_3xx_total 1",
		"webx_requests_4xx_total 1",
		"webx_http1_requests_total 7",
		"webx_http2_requests_total 2",
		"webx_http3_requests_total 1",
		"webx_rate_limited_total 3",
		"webx_concurrency_rejected_total 5",
		"webx_panics_total 1",
		"# TYPE webx_active_connections gauge",
		"webx_active_connections 4",
		"webx_requests_in_flight 2",
		"webx_goroutines",
		"webx_mem_heap_alloc_bytes",
		"webx_gc_count_total",
		`webx_route_requests_total{path="/a"} 6`,
		`webx_route_errors5xx_total{path="/a"} 1`,
		`webx_route_avg_duration_ms{path="/a"} 8`,
		`webx_group_requests_total{prefix="/g"} 6`,
		`webx_group_errors5xx_total{prefix="/g"} 1`,
		`webx_group_avg_duration_ms{prefix="/g"} 8`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("渲染结果缺少指标：%s\n完整输出：\n%s", want, got)
		}
	}
}

func TestRenderMetricsEmpty(t *testing.T) {
	got := renderMetrics(Metrics{}, nil, nil)
	if !strings.Contains(got, "webx_requests_total 0") {
		t.Errorf("空快照应输出 0 值：%s", got)
	}
	if strings.Contains(got, "webx_route_") {
		t.Errorf("空路由不应输出路由指标：%s", got)
	}
}

func TestServerMetricsEndpoint(t *testing.T) {
	cfg := validConfig(t)
	cfg.MiddlewareMetrics = true
	cfg.MiddlewareRequestID = true
	s := newTestServer(t, cfg)
	s.EnableMetricsEndpoint("/metrics")
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/ok",
		Handler: func(c *Context) { c.Success("ok", nil) },
	})
	startServer(t, s)
	client := testHTTPClient()
	base := "https://" + s.ListenerAddr()

	for i := 0; i < 2; i++ {
		resp, err := client.Get(base + "/ok")
		if err != nil {
			t.Fatalf("业务请求失败：%v", err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	resp, err := client.Get(base + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics 失败：%v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("指标端点状态不符：%d", resp.StatusCode)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/plain") {
		t.Errorf("指标端点 Content-Type 不符：%s", resp.Header.Get("Content-Type"))
	}
	if resp.Header.Get("X-Request-ID") != "" {
		t.Error("指标端点不应经过业务中间件链")
	}
	for _, want := range []string{"webx_requests_total 2", `webx_route_requests_total{path="/ok"} 2`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("指标内容缺少：%s\n%s", want, body)
		}
	}

	// 指标端点自身不计入统计，避免自采集反馈
	client.Get(base + "/metrics")
	resp, _ = client.Get(base + "/metrics")
	body2, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body2), "webx_requests_total 2") {
		t.Errorf("指标端点请求不应计入统计：%s", body2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerMetricsEndpointConfig(t *testing.T) {
	cfg := validConfig(t)
	cfg.MetricsEnabled = true // 默认路径 /metrics
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/ok",
		Handler: func(c *Context) { c.Success("ok", nil) },
	})
	startServer(t, s)
	resp, err := testHTTPClient().Get("https://" + s.ListenerAddr() + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics 失败：%v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("配置启用的指标端点应可用：%d", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}

func TestServerMetricsEndpointInvalidPath(t *testing.T) {
	cfg := validConfig(t)
	s := newTestServer(t, cfg)
	s.EnableMetricsEndpoint("/x/:")
	s.UseHttp2Listen("127.0.0.1:0")
	if err := s.Start(); err == nil {
		t.Fatal("非法指标路径应导致启动失败")
	}
}

func TestEnableMetricsEndpointStartedGuard(t *testing.T) {
	s := &Server{started: true}
	if got := s.EnableMetricsEndpoint("/metrics"); got != s {
		t.Error("EnableMetricsEndpoint 应返回自身")
	}
}

func TestMetricsEndpointDisabledByDefault(t *testing.T) {
	cfg := validConfig(t)
	s := newTestServer(t, cfg)
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/ok",
		Handler: func(c *Context) { c.Success("ok", nil) },
	})
	startServer(t, s)
	resp, err := testHTTPClient().Get("https://" + s.ListenerAddr() + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics 失败：%v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("默认应禁用指标端点：%d", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Stop(ctx)
}
