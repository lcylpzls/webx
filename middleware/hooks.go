package middleware

import "github.com/lcylpzls/webx/v2/internal/core"

// Hooks 返回请求钩子中间件：进入时调用 onRequest，处理结束后调用 onResponse。
// 可用于 OpenTelemetry 适配等观测场景；回调可传 nil。
func Hooks(onRequest, onResponse func(*core.Context)) core.HandlerFunc {
	return func(c *core.Context) {
		if onRequest != nil {
			onRequest(c)
		}
		c.Next()
		if onResponse != nil {
			onResponse(c)
		}
	}
}
