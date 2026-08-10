package middleware

import (
	"fmt"
	"time"

	"github.com/lcylpzls/idgenx"
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
		generator = newRequestID
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

// requestIDRand 可替换的随机 ID 生成函数（测试注入失败场景用）。
var requestIDRand = idgenx.RandomHex

// newRequestID 生成 16 字节随机十六进制请求 ID（32 位小写 hex）。
// 随机源失败时回退时间戳前缀，保证请求 ID 始终可用。
func newRequestID() string {
	id, err := requestIDRand(16)
	if err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return id
}
