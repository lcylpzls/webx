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

// Unwrap 返回底层 Writer，供 http.ResponseController 透传 Flush/Hijack 等能力
// （SSE、WebSocket 等场景依赖该接口）。
func (w *timeoutWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
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

// TimeoutOptions 定义请求超时中间件的配置。
type TimeoutOptions struct {
	// Message 超时响应文案（默认 "请求处理超时"）。
	Message string
}

// Timeout 返回请求超时中间件（默认文案）。
// 向请求注入带超时的 Context；超时后丢弃 Handler 写入并返回 503。
func Timeout(timeout time.Duration) core.HandlerFunc {
	return TimeoutWithOptions(timeout, TimeoutOptions{})
}

// TimeoutWithOptions 返回带文案选项的请求超时中间件。
func TimeoutWithOptions(timeout time.Duration, opts TimeoutOptions) core.HandlerFunc {
	message := opts.Message
	if message == "" {
		message = "请求处理超时"
	}
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
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, message, nil)
		}
	}
}
