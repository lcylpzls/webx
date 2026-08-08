package middleware

import (
	"net/http"
	"sort"
	"sync"
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
	stats        *routeGroupStore
}

// routeGroupStore 持有路由级与分组级的计数器集合。
// 独立为指针结构，保持 Metrics 可比较（不直接引入 map 字段）。
type routeGroupStore struct {
	routes sync.Map
	groups sync.Map
}

// NewMetrics 创建指标计数器。
func NewMetrics() *Metrics {
	return &Metrics{stats: &routeGroupStore{}}
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

// RouteStat 单条路由的指标统计快照。
type RouteStat struct {
	// Path 路由注册路径。
	Path string
	// Requests 请求数。
	Requests uint64
	// Errors5xx 5xx 响应数。
	Errors5xx uint64
	// AvgDurationMs 平均请求耗时（毫秒）。
	AvgDurationMs uint64
}

// GroupStat 单个路由分组的指标统计快照。
type GroupStat struct {
	// Prefix 分组前缀。
	Prefix string
	// Requests 请求数。
	Requests uint64
	// Errors5xx 5xx 响应数。
	Errors5xx uint64
	// AvgDurationMs 平均请求耗时（毫秒）。
	AvgDurationMs uint64
}

// routeStat 单条路由或分组的内部计数器。
type routeStat struct {
	requests   atomic.Uint64
	errors5x   atomic.Uint64
	durationNs atomic.Uint64
	samples    atomic.Uint64
}

// MetricsHandler 返回指标采集中间件。
// panic 安全：请求处理发生 panic 时仍会记录请求数、耗时与 5xx 分布，
// 随后重新抛出 panic 交由 Recovery 中间件处理。
func MetricsHandler(m *Metrics) core.HandlerFunc {
	return func(c *core.Context) {
		start := time.Now()
		m.requests.Add(1)
		m.inFlight.Add(1)
		defer func() {
			m.inFlight.Add(-1)
			elapsed := uint64(time.Since(start))
			m.durationNs.Add(elapsed)
			m.samples.Add(1)
			status := c.StatusCode()
			if r := recover(); r != nil {
				status = http.StatusInternalServerError
				m.status5xx.Add(1)
				m.errors5x.Add(1)
				m.recordRoute(c, status, elapsed)
				panic(r)
			}
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
			m.recordRoute(c, status, elapsed)
		}()
		c.Next()
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

// RouteStats 返回路由级统计快照（按注册路径排序）。
func (m *Metrics) RouteStats() []RouteStat {
	if m.stats == nil {
		return nil
	}
	var out []RouteStat
	m.stats.routes.Range(func(key, value any) bool {
		s := value.(*routeStat)
		out = append(out, RouteStat{
			Path:          key.(string),
			Requests:      s.requests.Load(),
			Errors5xx:     s.errors5x.Load(),
			AvgDurationMs: avgMs(s.durationNs.Load(), s.samples.Load()),
		})
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// GroupStats 返回分组级统计快照（按分组前缀排序）。
func (m *Metrics) GroupStats() []GroupStat {
	if m.stats == nil {
		return nil
	}
	var out []GroupStat
	m.stats.groups.Range(func(key, value any) bool {
		s := value.(*routeStat)
		out = append(out, GroupStat{
			Prefix:        key.(string),
			Requests:      s.requests.Load(),
			Errors5xx:     s.errors5x.Load(),
			AvgDurationMs: avgMs(s.durationNs.Load(), s.samples.Load()),
		})
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Prefix < out[j].Prefix })
	return out
}

// recordRoute 更新当前请求的路由级与分组级统计。
func (m *Metrics) recordRoute(c *core.Context, status int, elapsed uint64) {
	if m.stats == nil {
		return
	}
	if route := c.Route(); route != "" {
		s := m.routeStat(route)
		s.requests.Add(1)
		s.durationNs.Add(elapsed)
		s.samples.Add(1)
		if status >= 500 {
			s.errors5x.Add(1)
		}
	}
	if group := c.Group(); group != "" {
		s := m.groupStat(group)
		s.requests.Add(1)
		s.durationNs.Add(elapsed)
		s.samples.Add(1)
		if status >= 500 {
			s.errors5x.Add(1)
		}
	}
}

// routeStat 返回指定路由的计数器，不存在时创建。
func (m *Metrics) routeStat(route string) *routeStat {
	if v, ok := m.stats.routes.Load(route); ok {
		return v.(*routeStat)
	}
	s := &routeStat{}
	v, _ := m.stats.routes.LoadOrStore(route, s)
	return v.(*routeStat)
}

// groupStat 返回指定分组的计数器，不存在时创建。
func (m *Metrics) groupStat(group string) *routeStat {
	if v, ok := m.stats.groups.Load(group); ok {
		return v.(*routeStat)
	}
	s := &routeStat{}
	v, _ := m.stats.groups.LoadOrStore(group, s)
	return v.(*routeStat)
}

// avgMs 计算平均耗时（毫秒），无样本时返回 0。
func avgMs(totalNs, samples uint64) uint64 {
	if samples == 0 {
		return 0
	}
	return totalNs / samples / 1_000_000
}

// InFlight 返回当前活跃请求数。
func (m *Metrics) InFlight() int64 {
	return m.inFlight.Load()
}
