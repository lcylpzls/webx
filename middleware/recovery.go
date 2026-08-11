package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/webx/internal/core"
)

// Recovery 返回 Panic 捕获中间件，这是组件库中唯一调用 recover() 的位置。
func Recovery() func(http.Handler) http.Handler {
	return RecoveryWithOptions(nil, nil, false)
}

// RecoveryWithMetrics 返回 Panic 捕获中间件，并统计 panic 数量。
func RecoveryWithMetrics(m *Metrics) func(http.Handler) http.Handler {
	return RecoveryWithOptions(nil, m, false)
}

// RecoveryWith 返回 Panic 捕获中间件，统计 panic 数量并输出日志。
func RecoveryWith(logger logx.Logger, m *Metrics) func(http.Handler) http.Handler {
	return RecoveryWithOptions(logger, m, false)
}

// RecoveryWithOptions 返回 Panic 捕获中间件；debugMode 为 true 时响应携带 panic 摘要。
func RecoveryWithOptions(logger logx.Logger, m *Metrics, debugMode bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := core.From(r.Context())
			defer func() {
				if rec := recover(); rec != nil {
					if m != nil {
						m.recordPanic()
					}
					stack := debug.Stack()
					if logger != nil {
						logger.WithField("requestId", c.RequestID()).
							WithField("stack", string(stack)).
							Error("webx：请求处理发生 panic", logx.Fields(logx.Any("panic", rec)))
					}
					c.Set("recoveryError", fmt.Sprintf("webx：请求处理发生 panic：%v", rec))
					c.Set("recoveryStack", string(stack))
					msg := "服务器内部错误"
					if debugMode {
						msg = fmt.Sprintf("服务器内部错误：%v", rec)
					}
					c.AbortWithStatusJSON(http.StatusInternalServerError, msg, nil)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
