package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/lcylpzls/webx/internal/core"
)

// Recovery 返回 Panic 捕获中间件，这是组件库中唯一调用 recover() 的位置。
func Recovery() core.HandlerFunc {
	return RecoveryWithMetrics(nil)
}

// RecoveryWithMetrics 返回 Panic 捕获中间件，并统计 panic 数量。
func RecoveryWithMetrics(m *Metrics) core.HandlerFunc {
	return func(c *core.Context) {
		defer func() {
			if r := recover(); r != nil {
				if m != nil {
					m.panics.Add(1)
				}
				stack := debug.Stack()
				c.Set("recoveryError", fmt.Sprintf("webx：请求处理发生 panic：%v", r))
				c.Set("recoveryStack", string(stack))
				c.AbortWithStatusJSON(http.StatusInternalServerError, "服务器内部错误", nil)
			}
		}()
		c.Next()
	}
}
