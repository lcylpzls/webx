package middleware

import (
	"time"

	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/webx/internal/core"
)

// AccessLog 返回访问日志中间件。
// successOnly 为 true 时记录全部请求；为 false 时仅记录非 2xx 请求。
func AccessLog(logger logx.Logger, successOnly bool) core.HandlerFunc {
	return func(c *core.Context) {
		start := time.Now()
		c.Next()
		status := c.StatusCode()
		if status >= 200 && status < 300 && !successOnly {
			return
		}
		fields := logx.Fields(
			logx.String("method", c.Request().Method),
			logx.String("path", c.Request().URL.Path),
			logx.Int("status", status),
			logx.String("requestId", c.RequestID()),
			logx.String("ip", c.RemoteIP()),
			logx.String("host", c.Request().Host),
			logx.String("query", c.Request().URL.RawQuery),
			logx.String("user_agent", c.GetHeader("User-Agent")),
			logx.Any("duration", time.Since(start).String()),
			logx.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
		if status >= 400 {
			logger.Warn("访问日志", fields)
			return
		}
		logger.Info("访问日志", fields)
	}
}
