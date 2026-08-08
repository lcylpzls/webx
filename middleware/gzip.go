package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"

	"github.com/lcylpzls/webx/internal/core"
)

// Gzip 返回响应压缩中间件：客户端 Accept-Encoding 含 gzip 时启用。
func Gzip() core.HandlerFunc {
	return func(c *core.Context) {
		if !acceptsGzip(c.GetHeader("Accept-Encoding")) {
			c.Next()
			return
		}
		orig := c.Writer()
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")
		gz := gzip.NewWriter(orig)
		c.SetWriter(&gzipWriter{ResponseWriter: orig, gz: gz})

		c.Next()
		_ = gz.Close()
		c.SetWriter(orig)
	}
}

// acceptsGzip 判断 Accept-Encoding 是否包含 gzip。
func acceptsGzip(header string) bool {
	for _, part := range strings.Split(header, ",") {
		if strings.TrimSpace(part) == "gzip" {
			return true
		}
	}
	return false
}

// gzipWriter 包装 ResponseWriter，写入前经 gzip 压缩。
type gzipWriter struct {
	http.ResponseWriter
	gz        *gzip.Writer
	wroteHead bool
}

// WriteHeader 记录状态码并透传。
func (w *gzipWriter) WriteHeader(code int) {
	if !w.wroteHead {
		w.wroteHead = true
		w.ResponseWriter.WriteHeader(code)
	}
}

// Write 写入压缩数据。
func (w *gzipWriter) Write(p []byte) (int, error) {
	return w.gz.Write(p)
}

// Flush 刷新压缩流。
func (w *gzipWriter) Flush() {
	_ = w.gz.Flush()
}

// Unwrap 返回底层 Writer。
func (w *gzipWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
