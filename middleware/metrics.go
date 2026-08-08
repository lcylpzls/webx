package middleware

import (
	"sync/atomic"
	"time"

	"github.com/lcylpzls/webx/internal/core"
)

// Metrics 统计请求、状态码分布与协议维度指标，供监控面板对接。
type Metrics struct {
	requests     atomic.Uint64
	errors5x     atomic.Uint64
	panics       atomic.Uint64
	durationNs   atomic.Uint64
	samples      atomic.Uint64
	inFlight     atomic.Int64
	status1xx    atomic.Uint64
	status2xx    atomic.Uint64
	status3xx    atomic.Uint64
	status4xx    atomic.Uint64
	status5xx    atomic.Uint64
	http1Req     atomic.Uint64
	http2Req     atomic.Uint64
	http3Req     atomic.Uint64
	http1Ns      atomic.Uint64
	http2Ns      atomic.Uint64
	http3Ns      atomic.Uint64
	http1Samples atomic.Uint64
	http2Samples atomic.Uint64
	http3Samples atomic.Uint64
}

// NewMetrics 创建指标计数器。
func NewMetrics() *Metrics {
	return &Metrics{}
}

// ProtocolStats 协议维度请求统计快照。
type ProtocolStats struct {
	// HTTP1Requests HTTP/1.0 与 HTTP/1.1 请求数。
	HTTP1Requests uint64
	// HTTP2Requests HTTP/2 请求数。
	HTTP2Requests uint64
	// HTTP3Requests HTTP/3 请求数。
	HTTP3Requests uint64
	// HTTP1AvgMs HTTP/1.x 平均耗时（毫秒）。
	HTTP1AvgMs uint64
	// HTTP2AvgMs HTTP/2 平均耗时（毫秒）。
	HTTP2AvgMs uint64
	// HTTP3AvgMs HTTP/3 平均耗时（毫秒）。
	HTTP3AvgMs uint64
}

// MetricsHandler 返回指标采集中间件。
func MetricsHandler(m *Metrics) core.HandlerFunc {
	return func(c *core.Context) {
		start := time.Now()
		m.requests.Add(1)
		m.inFlight.Add(1)
		c.Next()
		m.inFlight.Add(-1)
		elapsed := uint64(time.Since(start))
		m.durationNs.Add(elapsed)
		m.samples.Add(1)
		status := c.StatusCode()
		switch {
		case status < 200:
			m.status1xx.Add(1)
		case status < 300:
			m.status2xx.Add(1)
		case status < 400:
			m.status3xx.Add(1)
		case status < 500:
			m.status4xx.Add(1)
		default:
			m.status5xx.Add(1)
		}
		if status >= 500 {
			m.errors5x.Add(1)
		}
		switch c.Request().Proto {
		case "HTTP/1.0", "HTTP/1.1":
			m.http1Req.Add(1)
			m.http1Ns.Add(elapsed)
			m.http1Samples.Add(1)
		case "HTTP/2.0":
			m.http2Req.Add(1)
			m.http2Ns.Add(elapsed)
			m.http2Samples.Add(1)
		case "HTTP/3.0":
			m.http3Req.Add(1)
			m.http3Ns.Add(elapsed)
			m.http3Samples.Add(1)
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

// StatusCodes 返回按状态码分类的请求计数（1xx/2xx/3xx/4xx/5xx）。
func (m *Metrics) StatusCodes() (s1xx, s2xx, s3xx, s4xx, s5xx uint64) {
	return m.status1xx.Load(), m.status2xx.Load(), m.status3xx.Load(), m.status4xx.Load(), m.status5xx.Load()
}

// ProtocolStats 返回各协议（HTTP/1.x、HTTP/2、HTTP/3）的请求数与平均耗时（毫秒）。
func (m *Metrics) ProtocolStats() ProtocolStats {
	var ps ProtocolStats
	ps.HTTP1Requests = m.http1Req.Load()
	ps.HTTP2Requests = m.http2Req.Load()
	ps.HTTP3Requests = m.http3Req.Load()
	if s := m.http1Samples.Load(); s > 0 {
		ps.HTTP1AvgMs = m.http1Ns.Load() / s / 1_000_000
	}
	if s := m.http2Samples.Load(); s > 0 {
		ps.HTTP2AvgMs = m.http2Ns.Load() / s / 1_000_000
	}
	if s := m.http3Samples.Load(); s > 0 {
		ps.HTTP3AvgMs = m.http3Ns.Load() / s / 1_000_000
	}
	return ps
}

// InFlight 返回当前活跃请求数。
func (m *Metrics) InFlight() int64 {
	return m.inFlight.Load()
}
