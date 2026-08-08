package middleware

import (
	"math/rand/v2"
	"net/http"
	"net/url"
	"time"

	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/webx/internal/core"
)

// AccessLogOptions 定义访问日志中间件的配置。
type AccessLogOptions struct {
	// LogSuccess 是否记录成功请求（默认仅记录非 2xx）。
	LogSuccess bool
	// SampleRate 采样率：0 表示记录全部；N>0 表示平均每 N 条记录 1 条。
	SampleRate int
	// RedactKeys query 参数中需要脱敏的键。
	RedactKeys []string
	// SlowThreshold 慢请求阈值；>0 且请求耗时达到阈值时额外记录 Warn（默认关闭）。
	SlowThreshold time.Duration
}

// countingWriter 统计响应体字节数。
type countingWriter struct {
	http.ResponseWriter
	n int64
}

// Write 写入并计数。
func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.n += int64(n)
	return n, err
}

// Unwrap 返回底层 Writer。
func (w *countingWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// sampleRand 可注入的随机函数（测试可替换）。
var sampleRand = rand.IntN

// AccessLog 返回访问日志中间件。
func AccessLog(logger logx.Logger, opts AccessLogOptions) core.HandlerFunc {
	return func(c *core.Context) {
		start := time.Now()
		orig := c.Writer()
		cw := &countingWriter{ResponseWriter: orig}
		c.SetWriter(cw)
		c.Next()
		c.SetWriter(orig)
		status := c.StatusCode()
		elapsed := time.Since(start)
		if opts.SlowThreshold > 0 && elapsed >= opts.SlowThreshold {
			logger.Warn("慢请求", logx.Fields(
				logx.String("method", c.Request().Method),
				logx.String("path", c.Request().URL.Path),
				logx.Int("status", status),
				logx.String("requestId", c.RequestID()),
				logx.String("duration", elapsed.String()),
			))
		}
		if status >= 200 && status < 300 && !opts.LogSuccess {
			return
		}
		if opts.SampleRate > 0 && sampleRand(opts.SampleRate) != 0 {
			return
		}
		query := redactQuery(c.Request().URL.RawQuery, opts.RedactKeys)
		fields := logx.Fields(
			logx.String("method", c.Request().Method),
			logx.String("path", c.Request().URL.Path),
			logx.Int("status", status),
			logx.String("requestId", c.RequestID()),
			logx.String("ip", c.RemoteIP()),
			logx.String("host", c.Request().Host),
			logx.String("proto", friendlyProto(c.Request().Proto)),
			logx.String("query", query),
			logx.String("user_agent", c.GetHeader("User-Agent")),
			logx.Any("duration", elapsed.String()),
			logx.Int64("duration_ms", elapsed.Milliseconds()),
			logx.Int64("bytes", cw.n),
		)
		if status >= 400 {
			logger.Warn("访问日志", fields)
			return
		}
		logger.Info("访问日志", fields)
	}
}

// friendlyProto 将请求协议转为可读名称：HTTP/2.0 → HTTP/2，HTTP/3.0 → HTTP/3。
func friendlyProto(proto string) string {
	switch proto {
	case "HTTP/2.0":
		return "HTTP/2"
	case "HTTP/3.0":
		return "HTTP/3"
	default:
		return proto
	}
}

// redactQuery 对 query 中指定键的值进行脱敏。
func redactQuery(raw string, keys []string) string {
	if len(keys) == 0 || raw == "" {
		return raw
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return raw
	}
	for key := range values {
		for _, k := range keys {
			if key == k {
				values.Set(key, "***")
			}
		}
	}
	return values.Encode()
}
