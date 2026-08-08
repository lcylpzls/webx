package middleware

import (
	"sync/atomic"
	"time"

	"github.com/lcylpzls/webx/internal/core"
)

// Metrics 统计请求与 5xx 错误数量，供监控面板对接。
type Metrics struct {
	requests   atomic.Uint64
	errors5x   atomic.Uint64
	panics     atomic.Uint64
	durationNs atomic.Uint64
	samples    atomic.Uint64
	inFlight   atomic.Int64
}

// NewMetrics 创建指标计数器。
func NewMetrics() *Metrics {
	return &Metrics{}
}

// MetricsHandler 返回指标采集中间件。
func MetricsHandler(m *Metrics) core.HandlerFunc {
	return func(c *core.Context) {
		start := time.Now()
		m.requests.Add(1)
		m.inFlight.Add(1)
		c.Next()
		m.inFlight.Add(-1)
		m.durationNs.Add(uint64(time.Since(start)))
		m.samples.Add(1)
		if c.StatusCode() >= 500 {
			m.errors5x.Add(1)
		}
	}
}

// Snapshot 返回当前计数快照。
func (m *Metrics) Snapshot() (requests, errors5x uint64) {
	return m.requests.Load(), m.errors5x.Load()
}

// Panics 返回 Recovery 捕获的 panic 数量。
func (m *Metrics) Panics() uint64 {
	return m.panics.Load()
}

// Durations 返回累计耗时（纳秒）与样本数。
func (m *Metrics) Durations() (totalNs, samples uint64) {
	return m.durationNs.Load(), m.samples.Load()
}

// InFlight 返回当前活跃请求数。
func (m *Metrics) InFlight() int64 {
	return m.inFlight.Load()
}
