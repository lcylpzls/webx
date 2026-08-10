package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lcylpzls/testx"
	"github.com/lcylpzls/webx/internal/core"
)

func TestMetricsHandler(t *testing.T) {
	m := NewMetrics(nil)

	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/ok", nil))
	c.SetHandlers([]core.HandlerFunc{
		MetricsHandler(m),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	requests, errors5x := m.Snapshot()
	if requests != 1 || errors5x != 0 {
		t.Errorf("成功请求计数不符：req=%d err5x=%d", requests, errors5x)
	}

	rec = httptest.NewRecorder()
	c = core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/fail", nil))
	c.SetHandlers([]core.HandlerFunc{
		MetricsHandler(m),
		func(c *core.Context) { c.JSONResponse(http.StatusInternalServerError, "内部错误", nil) },
	})
	c.Run()
	requests, errors5x = m.Snapshot()
	if requests != 2 || errors5x != 1 {
		t.Errorf("失败请求计数不符：req=%d err5x=%d", requests, errors5x)
	}
}

func TestMetricsDurations(t *testing.T) {
	m := NewMetrics(nil)
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/slow", nil))
	c.SetHandlers([]core.HandlerFunc{
		MetricsHandler(m),
		func(c *core.Context) {
			time.Sleep(2 * time.Millisecond)
			c.Success("ok", nil)
		},
	})
	c.Run()
	totalNs, samples := m.Durations()
	if samples != 1 || totalNs == 0 {
		t.Errorf("耗时统计不符：totalNs=%d samples=%d", totalNs, samples)
	}
}

func TestMetricsInFlight(t *testing.T) {
	m := NewMetrics(nil)
	entered := make(chan struct{})
	release := make(chan struct{})
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		MetricsHandler(m),
		func(c *core.Context) {
			close(entered)
			<-release
			c.Success("ok", nil)
		},
	})
	done := make(chan struct{})
	go func() {
		c.Run()
		close(done)
	}()
	<-entered
	testx.RequireEqual(t, m.InFlight(), int64(1))
	close(release)
	<-done
	testx.RequireEqual(t, m.InFlight(), int64(0))
}

func TestMetricsStatusCodes(t *testing.T) {
	m := NewMetrics(nil)
	for _, code := range []int{199, 200, 302, 404, 500} {
		rec := httptest.NewRecorder()
		c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		c.SetHandlers([]core.HandlerFunc{
			MetricsHandler(m),
			func(c *core.Context) { c.JSONResponse(code, "resp", nil) },
		})
		c.Run()
	}
	s1, s2, s3, s4, s5 := m.StatusCodes()
	if s1 != 1 || s2 != 1 || s3 != 1 || s4 != 1 || s5 != 1 {
		t.Errorf("状态码分布不符：1xx=%d 2xx=%d 3xx=%d 4xx=%d 5xx=%d", s1, s2, s3, s4, s5)
	}
	requests, errors5x := m.Snapshot()
	if requests != 5 || errors5x != 1 {
		t.Errorf("总量统计不符：req=%d err5x=%d", requests, errors5x)
	}
}

func TestMetricsProtocolStats(t *testing.T) {
	m := NewMetrics(nil)
	for _, proto := range []string{"HTTP/1.0", "HTTP/1.1", "HTTP/2.0", "HTTP/3.0"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Proto = proto
		c := core.NewContext(rec, req)
		c.SetHandlers([]core.HandlerFunc{
			MetricsHandler(m),
			func(c *core.Context) {
				time.Sleep(2 * time.Millisecond)
				c.Success("ok", nil)
			},
		})
		c.Run()
	}
	ps := m.ProtocolStats()
	if ps.HTTP1Requests != 2 || ps.HTTP2Requests != 1 || ps.HTTP3Requests != 1 {
		t.Errorf("协议请求数不符：%+v", ps)
	}
	if ps.HTTP1AvgMs == 0 || ps.HTTP2AvgMs == 0 || ps.HTTP3AvgMs == 0 {
		t.Errorf("协议平均耗时应为正数：%+v", ps)
	}
	fresh := NewMetrics(nil).ProtocolStats()
	if fresh.HTTP1AvgMs != 0 || fresh.HTTP2AvgMs != 0 || fresh.HTTP3AvgMs != 0 {
		t.Errorf("未采样协议平均耗时应为 0：%+v", fresh)
	}
}

func TestMetricsPanicSampling(t *testing.T) {
	m := NewMetrics(nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.Proto = "HTTP/1.1"
	c := core.NewContext(rec, req)
	c.SetRoute("/boom")
	c.SetHandlers([]core.HandlerFunc{
		RecoveryWithOptions(nil, m, false),
		MetricsHandler(m),
		func(c *core.Context) { panic("测试 panic") },
	})
	func() {
		defer func() { recover() }()
		c.Run()
	}()
	requests, errors5x := m.Snapshot()
	if requests != 1 || errors5x != 1 {
		t.Errorf("panic 请求应计入总量且归为 5xx：req=%d err5x=%d", requests, errors5x)
	}
	_, _, _, _, s5 := m.StatusCodes()
	if s5 != 1 {
		t.Errorf("panic 应计入 5xx 分布：%d", s5)
	}
	testx.RequireEqual(t, m.InFlight(), int64(0))
	rs := m.RouteStats()
	if len(rs) != 1 || rs[0].Path != "/boom" || rs[0].Requests != 1 || rs[0].Errors5xx != 1 {
		t.Errorf("panic 路由统计不符：%+v", rs)
	}
}

func TestMetricsRouteAndGroupStats(t *testing.T) {
	m := NewMetrics(nil)
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/api/users/1", nil))
		c.SetRoute("/api/users/:id")
		c.SetGroup("/api")
		c.SetHandlers([]core.HandlerFunc{
			MetricsHandler(m),
			func(c *core.Context) {
				if i == 2 {
					c.JSONResponse(http.StatusInternalServerError, "内部错误", nil)
					return
				}
				time.Sleep(2 * time.Millisecond)
				c.Success("ok", nil)
			},
		})
		c.Run()
	}

	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/api/v2/x", nil))
	c.SetRoute("/api/v2/:id")
	c.SetGroup("/api/v2")
	c.SetHandlers([]core.HandlerFunc{
		MetricsHandler(m),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()

	rs := m.RouteStats()
	if len(rs) != 2 || rs[0].Path != "/api/users/:id" || rs[1].Path != "/api/v2/:id" {
		t.Fatalf("路由统计应按路径排序：%+v", rs)
	}
	if rs[0].Requests != 3 || rs[0].Errors5xx != 1 || rs[0].AvgDurationMs == 0 {
		t.Errorf("路由统计计数不符：%+v", rs[0])
	}
	if rs[1].Requests != 1 || rs[1].Errors5xx != 0 {
		t.Errorf("第二路由统计不符：%+v", rs[1])
	}
	gs := m.GroupStats()
	if len(gs) != 2 || gs[0].Prefix != "/api" || gs[1].Prefix != "/api/v2" {
		t.Fatalf("分组统计应按前缀排序：%+v", gs)
	}
	if gs[0].Requests != 3 || gs[0].Errors5xx != 1 || gs[0].AvgDurationMs == 0 {
		t.Errorf("分组统计计数不符：%+v", gs[0])
	}
	if gs[1].Requests != 1 || gs[1].Errors5xx != 0 {
		t.Errorf("第二分组统计不符：%+v", gs[1])
	}

	// 零样本条目平均耗时为 0
	m.stats.routes.Store("/zero", &routeStat{})
	m.stats.groups.Store("/zero", &routeStat{})
	rs = m.RouteStats()
	gs = m.GroupStats()
	var routeZero, groupZero bool
	for _, st := range rs {
		if st.Path == "/zero" && st.AvgDurationMs == 0 {
			routeZero = true
		}
	}
	for _, st := range gs {
		if st.Prefix == "/zero" && st.AvgDurationMs == 0 {
			groupZero = true
		}
	}
	if !routeZero || !groupZero {
		t.Errorf("零样本平均耗时应为 0：routes=%+v groups=%+v", rs, gs)
	}

	// 空快照
	fresh := NewMetrics(nil)
	if got := fresh.RouteStats(); len(got) != 0 {
		t.Errorf("空路由快照应为空：%+v", got)
	}
	if got := fresh.GroupStats(); len(got) != 0 {
		t.Errorf("空分组快照应为空：%+v", got)
	}
}

func TestMetricsZeroValueSafe(t *testing.T) {
	var m Metrics
	testx.RequireNil(t, m.RouteStats())
	testx.RequireNil(t, m.GroupStats())
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	c.SetRoute("/x")
	c.SetHandlers([]core.HandlerFunc{
		MetricsHandler(&m),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	testx.RequireNil(t, m.RouteStats())
}

// fakeSink 是外部指标接收器测试替身，同时实现瞬时量接口。
type fakeSink struct {
	counters map[string][]string
	hist     map[string][]string
	gauges   map[string]float64
}

func newFakeSink() *fakeSink {
	return &fakeSink{
		counters: make(map[string][]string),
		hist:     make(map[string][]string),
		gauges:   make(map[string]float64),
	}
}

func (f *fakeSink) IncCounter(name string, labels []string) {
	f.counters[name] = append(f.counters[name], name+":"+joinLabels(labels))
}

func (f *fakeSink) AddCounter(name string, delta float64, labels []string) {
	f.counters[name] = append(f.counters[name], name+":"+joinLabels(labels))
}

func (f *fakeSink) ObserveDuration(name string, seconds float64, labels []string) {
	f.hist[name] = append(f.hist[name], name+":"+joinLabels(labels))
}

func (f *fakeSink) AddGauge(name string, delta float64, labels []string) {
	f.gauges[name] += delta
}

func (f *fakeSink) SetGauge(name string, value float64, labels []string) {
	f.gauges[name] = value
}

func (f *fakeSink) RegisterMetric(name, help string, labelNames []string) error {
	return nil
}

func joinLabels(labels []string) string {
	return strings.Join(labels, "|")
}

func TestMetricsHandlerExternalSink(t *testing.T) {
	sink := newFakeSink()
	m := NewMetrics(sink)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
	req.Proto = "HTTP/2.0"
	c := core.NewContext(rec, req)
	c.SetRoute("/api/users/:id")
	c.SetGroup("/api")
	c.SetHandlers([]core.HandlerFunc{
		MetricsHandler(m),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()

	if len(sink.counters["webx.requests"]) != 1 {
		t.Fatalf("外部请求计数缺失：%+v", sink.counters)
	}
	testx.RequireEqual(t, sink.counters["webx.requests"][0], "webx.requests:/api/users/:id|/api|2xx|HTTP/2")
	if len(sink.hist["webx.request_duration"]) != 1 {
		t.Fatalf("外部耗时观测缺失：%+v", sink.hist)
	}
	if sink.gauges["webx.requests_in_flight"] != 0 {
		t.Errorf("瞬时量应回归 0：%v", sink.gauges)
	}
}

func TestMetricsHandlerExternalSinkPanic(t *testing.T) {
	sink := newFakeSink()
	m := NewMetrics(sink)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.Proto = "HTTP/1.1"
	c := core.NewContext(rec, req)
	c.SetRoute("/boom")
	c.SetHandlers([]core.HandlerFunc{
		RecoveryWithOptions(nil, m, false),
		MetricsHandler(m),
		func(c *core.Context) { panic("测试 panic") },
	})
	func() {
		defer func() { recover() }()
		c.Run()
	}()
	if len(sink.counters["webx.panics"]) != 1 {
		t.Errorf("外部 panic 计数缺失：%+v", sink.counters)
	}
	testx.RequireEqual(t, sink.counters["webx.requests"][0], "webx.requests:/boom||5xx|HTTP/1")
}

func TestMetricsHandlerExternalSinkNoGauge(t *testing.T) {
	sink := &counterOnlySink{counters: make(map[string][]string)}
	m := NewMetrics(sink)
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	c.SetHandlers([]core.HandlerFunc{
		MetricsHandler(m),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	if len(sink.counters["webx.requests"]) != 1 {
		t.Fatalf("仅计数接收器也应收到请求事件：%+v", sink.counters)
	}
}

func TestStatusClassAndProtocolName(t *testing.T) {
	cases := []struct {
		status int
		proto  string
		class  string
		pt     string
	}{
		{199, "HTTP/1.0", "1xx", "HTTP/1"},
		{200, "HTTP/1.1", "2xx", "HTTP/1"},
		{302, "HTTP/2.0", "3xx", "HTTP/2"},
		{404, "HTTP/3.0", "4xx", "HTTP/3"},
		{500, "h2c", "5xx", "unknown"},
	}
	for _, tc := range cases {
		testx.RequireEqual(t, statusClass(tc.status), tc.class)
		testx.RequireEqual(t, protocolName(tc.proto), tc.pt)
	}
}

func TestLimiterMetricsSinkExternal(t *testing.T) {
	sink := newFakeSink()
	rl := NewRateLimiter(1, time.Second, nil)
	rl.SetMetricsSink(sink)
	if !rl.Allow("127.0.0.1") {
		t.Fatal("首个请求应放行")
	}
	if rl.Allow("127.0.0.1") {
		t.Fatal("第二个请求应被限流拒绝")
	}
	if len(sink.counters["webx.rate_limited"]) != 1 {
		t.Errorf("外部限流拒绝事件缺失：%+v", sink.counters)
	}

	cl := NewConcurrencyLimiter(1)
	cl.SetMetricsSink(sink)
	if !cl.TryAcquire() {
		t.Fatal("首个并发额度应获取成功")
	}
	if cl.TryAcquire() {
		t.Fatal("额度已满应拒绝")
	}
	if len(sink.counters["webx.concurrency_rejected"]) != 1 {
		t.Errorf("外部并发拒绝事件缺失：%+v", sink.counters)
	}
}

// counterOnlySink 实现 MetricsSink，Gauge 相关方法为 no-op。
type counterOnlySink struct {
	counters map[string][]string
}

func (f *counterOnlySink) IncCounter(name string, labels []string) {
	f.counters[name] = append(f.counters[name], name+":"+joinLabels(labels))
}

func (f *counterOnlySink) AddCounter(string, float64, []string) {}

func (f *counterOnlySink) ObserveDuration(string, float64, []string) {}

func (f *counterOnlySink) AddGauge(string, float64, []string) {}

func (f *counterOnlySink) SetGauge(string, float64, []string) {}

func (f *counterOnlySink) RegisterMetric(string, string, []string) error {
	return nil
}
