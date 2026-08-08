package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lcylpzls/webx/internal/core"
)

func TestMetricsHandler(t *testing.T) {
	m := NewMetrics()

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
	m := NewMetrics()
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
	m := NewMetrics()
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
	if got := m.InFlight(); got != 1 {
		t.Errorf("处理中 InFlight 应为 1：%d", got)
	}
	close(release)
	<-done
	if got := m.InFlight(); got != 0 {
		t.Errorf("处理完成 InFlight 应为 0：%d", got)
	}
}

func TestMetricsStatusCodes(t *testing.T) {
	m := NewMetrics()
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
	m := NewMetrics()
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
	fresh := NewMetrics().ProtocolStats()
	if fresh.HTTP1AvgMs != 0 || fresh.HTTP2AvgMs != 0 || fresh.HTTP3AvgMs != 0 {
		t.Errorf("未采样协议平均耗时应为 0：%+v", fresh)
	}
}
