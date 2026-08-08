package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/lcylpzls/webx/internal/core"
)

// timeoutWriter 包装 http.ResponseWriter，超时后丢弃写入。
// 不引入额外 goroutine，避免 Context 并发读写。
type timeoutWriter struct {
	http.ResponseWriter
	ctx         context.Context
	timedOut    bool
	wroteHeader bool
}

// WriteHeader 写入状态码；Context 已超时则丢弃。
func (w *timeoutWriter) WriteHeader(code int) {
	select {
	case <-w.ctx.Done():
		w.timedOut = true
	default:
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(code)
	}
}

// Write 写入响应体；Context 已超时则丢弃。
func (w *timeoutWriter) Write(b []byte) (int, error) {
	select {
	case <-w.ctx.Done():
		w.timedOut = true
		return len(b), nil
	default:
		return w.ResponseWriter.Write(b)
	}
}

// Timeout 返回请求超时中间件。
// 向请求注入带超时的 Context；超时后丢弃 Handler 写入并返回 503。
func Timeout(timeout time.Duration) core.HandlerFunc {
	return func(c *core.Context) {
		if timeout <= 0 {
			c.Next()
			return
		}
		ctx, cancel := context.WithTimeout(c.Request().Context(), timeout)
		defer cancel()
		c.SetRequest(c.Request().WithContext(ctx))

		origWriter := c.Writer()
		tw := &timeoutWriter{ResponseWriter: origWriter, ctx: ctx}
		c.SetWriter(tw)

		c.Next()
		c.SetWriter(origWriter)

		if tw.timedOut && !tw.wroteHeader {
			c.ResetWriteState()
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, "请求处理超时", nil)
		}
	}
}
