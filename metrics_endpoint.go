package webx

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/lcylpzls/webx/internal/core"
)

// metricsEndpointPath 返回指标端点路径；未启用时为空字符串。
func (s *Server) metricsEndpointPath() string {
	if s.metricsPath != "" {
		return s.metricsPath
	}
	if s.config.MetricsEnabled {
		return s.config.MetricsPath
	}
	return ""
}

// serveMetrics 输出 Prometheus 文本格式指标。
// 该端点绕过业务中间件链（不计入指标、不写访问日志），避免自采集反馈。
func (s *Server) serveMetrics(c *core.Context) {
	w := c.Writer()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(renderMetrics(s.Metrics(), s.RouteStats(), s.GroupStats())))
}

// renderMetrics 将指标快照渲染为 Prometheus 文本格式（exposition 0.0.4）。
func renderMetrics(m Metrics, routes []RouteStat, groups []GroupStat) string {
	var b strings.Builder
	writeCounter(&b, "webx_requests_total", "请求总数", m.Requests)
	writeCounter(&b, "webx_requests_5xx_total", "5xx 响应数", m.Errors5xx)
	writeCounter(&b, "webx_requests_1xx_total", "1xx 响应数", m.Status1xx)
	writeCounter(&b, "webx_requests_2xx_total", "2xx 响应数", m.Status2xx)
	writeCounter(&b, "webx_requests_3xx_total", "3xx 响应数", m.Status3xx)
	writeCounter(&b, "webx_requests_4xx_total", "4xx 响应数", m.Status4xx)
	writeCounter(&b, "webx_http1_requests_total", "HTTP/1.x 请求数", m.HTTP1Requests)
	writeCounter(&b, "webx_http2_requests_total", "HTTP/2 请求数", m.HTTP2Requests)
	writeCounter(&b, "webx_http3_requests_total", "HTTP/3 请求数", m.HTTP3Requests)
	writeCounter(&b, "webx_rate_limited_total", "限流拒绝数", m.RateLimited)
	writeCounter(&b, "webx_panics_total", "Recovery 捕获的 panic 数", m.Panics)
	writeGauge(&b, "webx_active_connections", "当前打开的连接数", m.ActiveConnections)
	writeGauge(&b, "webx_requests_in_flight", "当前活跃请求数", m.RequestsInFlight)
	for _, st := range routes {
		writeLabeledCounter(&b, "webx_route_requests_total", "路由级请求数", "path", st.Path, st.Requests)
		writeLabeledGauge(&b, "webx_route_errors5xx_total", "路由级 5xx 响应数", "path", st.Path, st.Errors5xx)
		writeLabeledGauge(&b, "webx_route_avg_duration_ms", "路由级平均耗时（毫秒）", "path", st.Path, st.AvgRequestDurationMs)
	}
	for _, st := range groups {
		writeLabeledCounter(&b, "webx_group_requests_total", "分组级请求数", "prefix", st.Prefix, st.Requests)
		writeLabeledGauge(&b, "webx_group_errors5xx_total", "分组级 5xx 响应数", "prefix", st.Prefix, st.Errors5xx)
		writeLabeledGauge(&b, "webx_group_avg_duration_ms", "分组级平均耗时（毫秒）", "prefix", st.Prefix, st.AvgRequestDurationMs)
	}
	return b.String()
}

// writeCounter 输出 counter 类型指标。
func writeCounter(b *strings.Builder, name, help string, value uint64) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s counter\n", name)
	fmt.Fprintf(b, "%s %d\n", name, value)
}

// writeGauge 输出 gauge 类型指标。
func writeGauge(b *strings.Builder, name, help string, value int64) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s gauge\n", name)
	fmt.Fprintf(b, "%s %d\n", name, value)
}

// writeLabeledCounter 输出带标签的 counter 类型指标。
func writeLabeledCounter(b *strings.Builder, name, help, label, value string, n uint64) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s counter\n", name)
	fmt.Fprintf(b, "%s{%s=%q} %d\n", name, label, value, n)
}

// writeLabeledGauge 输出带标签的 gauge 类型指标。
func writeLabeledGauge(b *strings.Builder, name, help, label, value string, n uint64) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s gauge\n", name)
	fmt.Fprintf(b, "%s{%s=%q} %d\n", name, label, value, n)
}
