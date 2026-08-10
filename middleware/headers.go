package middleware

import "github.com/lcylpzls/webx/v2/internal/core"

// 常用请求/响应头的预计算规范化键（热路径零规范化分配）。
var (
	canonicalAcceptEncoding  = core.CanonicalHeaderKey("Accept-Encoding")
	canonicalContentEncoding = core.CanonicalHeaderKey("Content-Encoding")
	canonicalContentType     = core.CanonicalHeaderKey("Content-Type")
	canonicalVary            = core.CanonicalHeaderKey("Vary")
	canonicalRetryAfter      = core.CanonicalHeaderKey("Retry-After")
	canonicalUserAgent       = core.CanonicalHeaderKey("User-Agent")
)
