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
