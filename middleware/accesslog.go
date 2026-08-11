package middleware

import (
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/webx/internal/core"
)

var countingWriterPool = sync.Pool{New: func() any { return &countingWriter{} }}

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
	// HeaderKeys 需要写入日志的请求头白名单（命中 RedactKeys 的值会脱敏）。
	HeaderKeys []string
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
func AccessLog(logger logx.Logger, opts AccessLogOptions) func(http.Handler) http.Handler {
	headerKeys := make([]string, len(opts.HeaderKeys))
	for i, k := range opts.HeaderKeys {
		headerKeys[i] = core.CanonicalHeaderKey(k)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := core.From(r.Context())
			start := time.Now()
			orig := c.Writer()
			cw := countingWriterPool.Get().(*countingWriter)
			cw.ResponseWriter = orig
			cw.n = 0
			c.SetWriter(cw)
			next.ServeHTTP(cw, r)
			c.SetWriter(orig)
			bytes := cw.n
			countingWriterPool.Put(cw)
			status := c.StatusCode()
			elapsed := time.Since(start)
			if opts.SlowThreshold > 0 && elapsed >= opts.SlowThreshold {
				logger.Warn("慢请求", logx.Fields(
					logx.String("method", r.Method),
					logx.String("path", r.URL.Path),
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
			query := redactQuery(r.URL.RawQuery, opts.RedactKeys)
			logFields := []logx.Field{
				logx.String("method", r.Method),
				logx.String("path", r.URL.Path),
				logx.Int("status", status),
				logx.String("requestId", c.RequestID()),
				logx.String("ip", c.RemoteIP()),
				logx.String("host", r.Host),
				logx.String("scheme", requestScheme(r)),
				logx.String("proto", friendlyProto(r.Proto)),
				logx.String("query", query),
				logx.String("user_agent", r.Header.Get(canonicalUserAgent)),
				logx.Any("duration", elapsed.String()),
				logx.Int64("duration_ms", elapsed.Milliseconds()),
				logx.Int64("bytes", bytes),
			}
			for i, key := range opts.HeaderKeys {
				if key == "" {
					continue
				}
				value := r.Header.Get(headerKeys[i])
				if value == "" {
					continue
				}
				if isRedactKey(opts.RedactKeys, key) {
					value = "***"
				}
				logFields = append(logFields, logx.String(headerFieldName(key), value))
			}
			fields := logx.Fields(logFields...)
			if status >= 400 {
				logger.Warn("访问日志", fields)
				return
			}
			logger.Info("访问日志", fields)
		})
	}
}

// headerFieldName 将请求头名转为日志字段名（X-Request-ID → header_x_request_id）。
func headerFieldName(key string) string {
	return "header_" + strings.ToLower(strings.ReplaceAll(key, "-", "_"))
}

// isRedactKey 判断指定键是否命中脱敏列表。
func isRedactKey(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}

// requestScheme 返回请求协议：TLS 或 HTTP/3 记为 https，否则为 http。
func requestScheme(r *http.Request) string {
	if r.TLS != nil || r.ProtoMajor >= 3 {
		return "https"
	}
	return "http"
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
