package middleware

import (
	"net/http"

	"github.com/lcylpzls/webx/internal/core"
)

// Validation 返回请求参数校验中间件：
// 校验 Content-Type 是否为 JSON、Content-Length 是否超过 10MB。
func Validation() core.HandlerFunc {
	return func(c *core.Context) {
		method := c.Request().Method
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			c.Next()
			return
		}
		contentType := c.GetHeader("Content-Type")
		if contentType == "" {
			c.Next()
			return
		}
		if !isJSONContentType(contentType) {
			c.AbortWithStatusJSON(http.StatusBadRequest, "请求参数校验失败：Content-Type 必须为 application/json", nil)
			return
		}
		if c.Request().ContentLength > 10*1024*1024 {
			c.AbortWithStatusJSON(http.StatusBadRequest, "请求参数校验失败：请求体过大", nil)
			return
		}
		c.Next()
	}
}

// isJSONContentType 检查 Content-Type 是否为 JSON 类型。
func isJSONContentType(ct string) bool {
	if i := stringsIndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return ct == "application/json"
}

// stringsIndexByte 返回字节在字符串中的位置，避免额外导入 strings。
func stringsIndexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
