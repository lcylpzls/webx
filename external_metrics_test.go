package webx

import (
	testx "github.com/lcylpzls/testx"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// externalSink 是外部指标接收器测试替身，同时实现 GaugeMetrics。
type externalSink struct {
	mu       sync.Mutex
	counters map[string][]string
	hist     map[string][]string
	gauges   map[string]float64
}

func newExternalSink() *externalSink {
	return &externalSink{
		counters: make(map[string][]string),
		hist:     make(map[string][]string),
		gauges:   make(map[string]float64),
	}
}

func (f *externalSink) IncCounter(name string, labels ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counters[name] = append(f.counters[name], strings.Join(labels, "|"))
}

func (f *externalSink) ObserveDuration(name string, seconds float64, labels ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hist[name] = append(f.hist[name], strings.Join(labels, "|"))
}

func (f *externalSink) AddGauge(name string, delta float64, labels ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gauges[name] += delta
}

func (f *externalSink) SetGauge(name string, value float64, labels ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gauges[name] = value
}

func (f *externalSink) counterCount(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.counters[name])
}

func (f *externalSink) hasGauge(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.gauges[name]
	return ok
}

func TestServerExternalMetrics(t *testing.T) {
	cfg := validConfig(t)
	cfg.MiddlewareMetrics = true
	cfg.MiddlewareRequestID = true
	s := newTestServer(t, cfg)
	sink := newExternalSink()
	s.WithMetrics(sink)
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
		testx.RequireNoError(t, err)

		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	if got := sink.counterCount("webx.requests"); got != 2 {
		t.Errorf("外部请求计数不符：%d", got)
	}
	if !sink.hasGauge("webx.requests_in_flight") {
		t.Error("外部应收到活跃请求瞬时量")
	}
	if !sink.hasGauge("webx.active_connections") {
		t.Error("外部应收到连接数瞬时量")
	}
	// v2 不再内置 /metrics 端点，请求应 404。
	resp, err := client.Get(base + "/metrics")
	testx.RequireNoError(t, err)

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	testx.Equal(t, resp.StatusCode, http.StatusNotFound)

}

func TestServerExternalMetricsRateLimited(t *testing.T) {
	cfg := validConfig(t)
	cfg.MiddlewareMetrics = true
	s := newTestServer(t, cfg)
	sink := newExternalSink()
	s.WithMetrics(sink)
	s.EnableRateLimit(RateLimitOptions{QPS: 1, Window: time.Second})
	s.UseHttp2Listen("127.0.0.1:0")
	s.RegisterRoute(Route{
		Method:  "GET",
		Path:    "/ok",
		Handler: func(c *Context) { c.Success("ok", nil) },
	})
	startServer(t, s)

	client := testHTTPClient()
	base := "https://" + s.ListenerAddr()
	var last int
	for i := 0; i < 2; i++ {
		resp, err := client.Get(base + "/ok")
		testx.RequireNoError(t, err)

		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		last = resp.StatusCode
	}
	testx.RequireEqual(t, last, http.StatusTooManyRequests)

	if got := sink.counterCount("webx.rate_limited"); got < 1 {
		t.Errorf("外部应收到限流拒绝事件：%d", got)
	}
}

func TestWithMetricsStartedGuard(t *testing.T) {
	s := &Server{started: true}
	if got := s.WithMetrics(newExternalSink()); got != s {
		t.Error("启动后 WithMetrics 应返回自身")
	}
}
