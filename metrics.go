package webx

import (
	"net"
	"net/http"
)

// Metrics 是 webx 运行指标快照，可接入监控面板。
type Metrics struct {
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
func (s *Server) Metrics() Metrics {
	m := Metrics{}
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
	case http.StateClosed, http.StateHijacked:
		s.activeConns.Add(-1)
	}
}

// applyServerHooks 为 http.Server 注册关闭钩子。
func (s *Server) applyServerHooks(srv *http.Server) {
	for _, fn := range s.onShutdown {
		srv.RegisterOnShutdown(fn)
	}
}
