package webx

// Metrics 是 webx 运行指标快照，可接入监控面板。
type Metrics struct {
	// Requests 请求总数（需启用 MiddlewareMetrics）。
	Requests uint64
	// Errors5xx 5xx 响应数（需启用 MiddlewareMetrics）。
	Errors5xx uint64
}

// Metrics 返回运行指标快照；未启用 MiddlewareMetrics 时全为 0。
func (s *Server) Metrics() Metrics {
	if s.metrics == nil {
		return Metrics{}
	}
	requests, errors5x := s.metrics.Snapshot()
	return Metrics{Requests: requests, Errors5xx: errors5x}
}
