package webx

import "time"

// MiddlewareType 标识内置中间件的类型。
type MiddlewareType string

// KeyFunc 定义限流维度的提取函数（默认按客户端 IP）。
type KeyFunc func(*Context) string

const (
	// MiddlewareRequestID 请求 ID 生成中间件。
	MiddlewareRequestID MiddlewareType = "request_id"
	// MiddlewareCORS 跨域处理中间件。
	MiddlewareCORS MiddlewareType = "cors"
	// MiddlewareTimeout 请求超时中间件。
	MiddlewareTimeout MiddlewareType = "timeout"
	// MiddlewareRecovery Panic 捕获中间件。
	MiddlewareRecovery MiddlewareType = "recovery"
	// MiddlewareValidation 请求参数校验中间件。
	MiddlewareValidation MiddlewareType = "validation"
	// MiddlewareRateLimit IP 令牌桶限流中间件。
	MiddlewareRateLimit MiddlewareType = "rate_limit"
	// MiddlewareGzip 响应压缩中间件。
	MiddlewareGzip MiddlewareType = "gzip"
	// MiddlewareMetrics 请求/5xx 计数中间件。
	MiddlewareMetrics MiddlewareType = "metrics"
	// MiddlewareAccessLog 访问日志中间件。
	MiddlewareAccessLog MiddlewareType = "access_log"
)

// RateLimitOptions 定义 IP 限流中间件的配置参数。
type RateLimitOptions struct {
	// QPS 每 IP 每秒允许的请求数（必填，> 0）。
	QPS int
	// Window 限流窗口时长（必填，> 0）。
	Window time.Duration
	// Whitelist 白名单 IP/CIDR 列表（可选）。
	Whitelist []string
	// CleanupInterval 过期桶清理间隔（可选，0 = 默认 5 分钟）。
	CleanupInterval time.Duration
	// KeyFunc 限流维度提取函数（可选，默认按客户端 IP）。
	KeyFunc KeyFunc
}
