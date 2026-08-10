package middleware

import (
	"net/http"

	"github.com/lcylpzls/webx/internal/core"
)

// BodyLimitOptions 定义请求体限制中间件的配置。
type BodyLimitOptions struct {
	// Message 超限响应文案（默认 "请求体过大"）。
	Message string
}

// BodyLimit 返回请求体大小限制中间件（默认文案）。
// Content-Length 明确超限时直接返回 413；chunked 请求体由 MaxBytesReader 兜底。
func BodyLimit(maxBytes int64) core.HandlerFunc {
	return BodyLimitWithOptions(maxBytes, BodyLimitOptions{})
}

// BodyLimitWithOptions 返回带文案选项的请求体大小限制中间件。
// Content-Length 明确超限时直接返回 413；chunked 请求体由 MaxBytesReader 兜底。
func BodyLimitWithOptions(maxBytes int64, opts BodyLimitOptions) core.HandlerFunc {
	message := opts.Message
	if message == "" {
		message = "请求体过大"
	}
	return func(c *core.Context) {
		if maxBytes <= 0 {
			c.Next()
			return
		}
		if c.Request().ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, message, nil)
			return
		}
		if c.Request().Body != nil {
			c.Request().Body = http.MaxBytesReader(c.Writer(), c.Request().Body, maxBytes)
		}
		c.Next()
	}
}
