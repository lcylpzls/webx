package webx

import (
	"net"
	"net/http"
)

// Metrics 是最小指标接口，与 dbx/httpx/cachex/resiliencex 等
// 家族底座签名一致，metricsx 天然满足。
// webx 本身不采集 Prometheus，只把事件转发给外部注入的实例。
type Metrics interface {
	// IncCounter 增加一个计数指标。
	IncCounter(name string, labels ...string)
	// ObserveDuration 记录一次耗时观测（秒）。
	ObserveDuration(name string, seconds float64, labels ...string)
}

// GaugeMetrics 是可选的瞬时量扩展接口。
// 注入的指标实例支持时，webx 会上报活跃请求与连接水位；
// 不支持则自动跳过，不影响主流程。
type GaugeMetrics interface {
	// AddGauge 按增量调整瞬时量（如 +1/-1）。
	AddGauge(name string, delta float64, labels ...string)
	// SetGauge 设置瞬时量绝对值。
	SetGauge(name string, value float64, labels ...string)
}

// MetricsSnapshot 是 webx 运行指标快照，可接入监控面板。
type MetricsSnapshot struct {
	// Requests 请求总数（需启用 MiddlewareMetrics）。
	Requests uint64
	// Errors5xx 5xx 响应数（需启用 MiddlewareMetrics）。
	Errors5xx uint64
	// Status1xx 1xx 响应数（需启用 MiddlewareMetrics）。
	Status1xx uint64
	// Status2xx 2xx 响应数（需启用 MiddlewareMetrics）。
	Status2xx uint64
	// Status3xx 3xx 响应数（需启用 MiddlewareMetrics）。
	Status3xx uint64
	// Status4xx 4xx 响应数（需启用 MiddlewareMetrics）。
	Status4xx uint64
	// Status5xx 5xx 响应数（需启用 MiddlewareMetrics）。
	Status5xx uint64
	// RateLimited 限流拒绝数（启用 EnableRateLimit 后统计）。
	RateLimited uint64
	// Panics Recovery 捕获的 panic 数（启用 MiddlewareRecovery 后统计）。
	Panics uint64
	// ConcurrencyRejected 并发限制拒绝数（启用 SetMaxConcurrentRequests 后统计）。
	ConcurrencyRejected uint64
	// AvgRequestDurationMs 平均请求耗时（毫秒，需启用 MiddlewareMetrics）。
	AvgRequestDurationMs uint64
	// HTTP1Requests HTTP/1.x 请求数（需启用 MiddlewareMetrics）。
	HTTP1Requests uint64
	// HTTP2Requests HTTP/2 请求数（需启用 MiddlewareMetrics）。
	HTTP2Requests uint64
	// HTTP3Requests HTTP/3 请求数（需启用 MiddlewareMetrics）。
	HTTP3Requests uint64
	// AvgHTTP1RequestDurationMs HTTP/1.x 平均请求耗时（毫秒，需启用 MiddlewareMetrics）。
	AvgHTTP1RequestDurationMs uint64
	// AvgHTTP2RequestDurationMs HTTP/2 平均请求耗时（毫秒，需启用 MiddlewareMetrics）。
	AvgHTTP2RequestDurationMs uint64
	// AvgHTTP3RequestDurationMs HTTP/3 平均请求耗时（毫秒，需启用 MiddlewareMetrics）。
	AvgHTTP3RequestDurationMs uint64
	// ActiveConnections 当前打开的连接数。
	ActiveConnections int64
	// RequestsInFlight 当前活跃请求数（需启用 MiddlewareMetrics）。
	RequestsInFlight int64
}

// RouteStat 单条路由的指标统计。
type RouteStat struct {
	// Path 路由注册路径。
	Path string
	// Requests 请求数。
	Requests uint64
	// Errors5xx 5xx 响应数。
	Errors5xx uint64
	// AvgRequestDurationMs 平均请求耗时（毫秒）。
	AvgRequestDurationMs uint64
}

// GroupStat 单个路由分组的指标统计。
type GroupStat struct {
	// Prefix 分组前缀。
	Prefix string
	// Requests 请求数。
	Requests uint64
	// Errors5xx 5xx 响应数。
	Errors5xx uint64
	// AvgRequestDurationMs 平均请求耗时（毫秒）。
	AvgRequestDurationMs uint64
}

// Metrics 返回运行指标快照；未启用对应能力时字段为 0。
// 快照来自 webx 内部轻量计数器，与外部 metricsx 转发互不干扰。
func (s *Server) Metrics() MetricsSnapshot {
	m := MetricsSnapshot{}
	if s.metrics != nil {
		m.Requests, m.Errors5xx = s.metrics.Snapshot()
		m.Panics = s.metrics.Panics()
		totalNs, samples := s.metrics.Durations()
		if samples > 0 {
			m.AvgRequestDurationMs = totalNs / samples / 1_000_000
		}
		m.Status1xx, m.Status2xx, m.Status3xx, m.Status4xx, m.Status5xx = s.metrics.StatusCodes()
		ps := s.metrics.ProtocolStats()
		m.HTTP1Requests = ps.HTTP1Requests
		m.HTTP2Requests = ps.HTTP2Requests
		m.HTTP3Requests = ps.HTTP3Requests
		m.AvgHTTP1RequestDurationMs = ps.HTTP1AvgMs
		m.AvgHTTP2RequestDurationMs = ps.HTTP2AvgMs
		m.AvgHTTP3RequestDurationMs = ps.HTTP3AvgMs
		m.RequestsInFlight = s.metrics.InFlight()
	}
	if s.rateLimiter != nil {
		m.RateLimited = s.rateLimiter.Rejected()
	}
	if s.concurrencyLimiter != nil {
		m.ConcurrencyRejected = s.concurrencyLimiter.Rejected()
	}
	m.ActiveConnections = s.activeConns.Load()
	return m
}

// RouteStats 返回路由级统计快照（按注册路径排序；需启用 MiddlewareMetrics）。
func (s *Server) RouteStats() []RouteStat {
	if s.metrics == nil {
		return nil
	}
	stats := s.metrics.RouteStats()
	out := make([]RouteStat, len(stats))
	for i, st := range stats {
		out[i] = RouteStat{
			Path:                 st.Path,
			Requests:             st.Requests,
			Errors5xx:            st.Errors5xx,
			AvgRequestDurationMs: st.AvgDurationMs,
		}
	}
	return out
}

// GroupStats 返回分组级统计快照（按分组前缀排序；需启用 MiddlewareMetrics）。
func (s *Server) GroupStats() []GroupStat {
	if s.metrics == nil {
		return nil
	}
	stats := s.metrics.GroupStats()
	out := make([]GroupStat, len(stats))
	for i, st := range stats {
		out[i] = GroupStat{
			Prefix:               st.Prefix,
			Requests:             st.Requests,
			Errors5xx:            st.Errors5xx,
			AvgRequestDurationMs: st.AvgDurationMs,
		}
	}
	return out
}

// connState 统计当前打开的连接数。
func (s *Server) connState(_ net.Conn, state http.ConnState) {
	switch state {
	case http.StateNew:
		s.activeConns.Add(1)
		s.emitConnGauge(1)
	case http.StateClosed, http.StateHijacked:
		s.activeConns.Add(-1)
		s.emitConnGauge(-1)
	}
}

// emitConnGauge 向外部瞬时量接口上报当前连接数变化。
func (s *Server) emitConnGauge(delta int64) {
	if g, ok := s.metricsSink.(GaugeMetrics); ok {
		g.AddGauge("webx.active_connections", float64(delta))
	}
}

// applyServerHooks 为 http.Server 注册关闭钩子。
func (s *Server) applyServerHooks(srv *http.Server) {
	for _, fn := range s.onShutdown {
		srv.RegisterOnShutdown(fn)
	}
}
