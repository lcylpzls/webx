package middleware

import (
	"sync/atomic"

	"github.com/lcylpzls/webx/internal/core"
)

// Metrics 统计请求与 5xx 错误数量，供监控面板对接。
type Metrics struct {
	requests atomic.Uint64
	errors5x atomic.Uint64
}

// NewMetrics 创建指标计数器。
func NewMetrics() *Metrics {
	return &Metrics{}
}

// MetricsHandler 返回指标采集中间件。
func MetricsHandler(m *Metrics) core.HandlerFunc {
	return func(c *core.Context) {
		m.requests.Add(1)
		c.Next()
		if c.StatusCode() >= 500 {
			m.errors5x.Add(1)
		}
	}
}

// Snapshot 返回当前计数快照。
func (m *Metrics) Snapshot() (requests, errors5x uint64) {
	return m.requests.Load(), m.errors5x.Load()
}
