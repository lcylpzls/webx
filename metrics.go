package webx

// Metrics 是 webx 运行指标快照，可接入监控面板。
type Metrics struct {
	// Requests 请求总数（需启用 MiddlewareMetrics）。
	Requests uint64
	// Errors5xx 5xx 响应数（需启用 MiddlewareMetrics）。
	Errors5xx uint64
	// RateLimited 限流拒绝数（启用 EnableRateLimit 后统计）。
	RateLimited uint64
	// Panics Recovery 捕获的 panic 数（启用 MiddlewareRecovery 后统计）。
	Panics uint64
	// AvgRequestDurationMs 平均请求耗时（毫秒，需启用 MiddlewareMetrics）。
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
	}
	if s.rateLimiter != nil {
		m.RateLimited = s.rateLimiter.Rejected()
	}
	return m
}
