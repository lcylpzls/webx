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
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return BodyLimitWithOptions(maxBytes, BodyLimitOptions{})
}

// BodyLimitWithOptions 返回带文案选项的请求体大小限制中间件。
// Content-Length 明确超限时直接返回 413；chunked 请求体由 MaxBytesReader 兜底。
func BodyLimitWithOptions(maxBytes int64, opts BodyLimitOptions) func(http.Handler) http.Handler {
	message := opts.Message
	if message == "" {
		message = "请求体过大"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := core.From(r.Context())
			if maxBytes <= 0 {
				next.ServeHTTP(w, r)
				return
			}
			if r.ContentLength > maxBytes {
				c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, message, nil)
				return
			}
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}
