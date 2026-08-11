package middleware

import (
	"net/http"

	"github.com/lcylpzls/webx/internal/core"
)

// Hooks 返回请求钩子中间件：进入时调用 onRequest，处理结束后调用 onResponse。
// 可用于 OpenTelemetry 适配等观测场景；回调可传 nil。
func Hooks(onRequest, onResponse func(*core.Context)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := core.From(r.Context())
			if onRequest != nil {
				onRequest(c)
			}
			next.ServeHTTP(w, r)
			if onResponse != nil {
				onResponse(c)
			}
		})
	}
}
