package middleware

import (
	"net/http"

	"github.com/lcylpzls/webx/internal/core"
)

// BodyLimit 返回请求体大小限制中间件。
// Content-Length 明确超限时直接返回 413；chunked 请求体由 MaxBytesReader 兜底。
func BodyLimit(maxBytes int64) core.HandlerFunc {
	return func(c *core.Context) {
		if maxBytes <= 0 {
			c.Next()
			return
		}
		if c.Request().ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, "请求体过大", nil)
			return
		}
		if c.Request().Body != nil {
			c.Request().Body = http.MaxBytesReader(c.Writer(), c.Request().Body, maxBytes)
		}
		c.Next()
	}
}
