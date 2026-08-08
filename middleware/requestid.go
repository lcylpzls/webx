package middleware

import (
	"github.com/google/uuid"
	"github.com/lcylpzls/webx/internal/core"
)

// RequestIDOptions 定义请求 ID 中间件的配置参数。
type RequestIDOptions struct {
	// Header 请求 ID 头名（默认 X-Request-ID）。
	Header string
	// Generator 请求 ID 生成函数（默认 UUID v7）。
	Generator func() string
}

// RequestID 返回请求 ID 生成中间件。
// 优先使用请求头 X-Request-ID，否则生成 UUID v7。
func RequestID() core.HandlerFunc {
	return RequestIDWithOptions(RequestIDOptions{})
}

// RequestIDWithOptions 返回按选项配置的请求 ID 生成中间件。
func RequestIDWithOptions(opts RequestIDOptions) core.HandlerFunc {
	header := opts.Header
	if header == "" {
		header = "X-Request-ID"
	}
	generator := opts.Generator
	if generator == nil {
		generator = newUUIDV7
	}
	canonical := core.CanonicalHeaderKey(header)
	return func(c *core.Context) {
		requestID := c.GetHeaderCanonical(canonical)
		if requestID == "" {
			requestID = generator()
		}
		c.Set("requestId", requestID)
		c.SetHeaderCanonical(canonical, requestID)
		// 出站透传：转发到上游时保留同一条请求 ID。
		c.SetRequestHeaderCanonical(canonical, requestID)
		c.Next()
	}
}

// newUUIDV7 生成 UUID v7（时间有序，适合分布式链路 ID）。
func newUUIDV7() string {
	return uuid.Must(uuid.NewV7()).String()
}
