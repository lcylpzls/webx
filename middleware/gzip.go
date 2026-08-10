package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/lcylpzls/webx/internal/core"
)

// setHeader 以预计算键写入响应头（复用已有切片，热路径零分配）。
func setHeader(h http.Header, key, value string) {
	if len(h[key]) == 0 {
		h[key] = []string{value}
		return
	}
	h[key][0] = value
}

// gzipPools 按压缩级别复用 gzip.Writer，降低每请求分配。
var gzipPools sync.Map // int → *sync.Pool

// GzipOptions 定义响应压缩中间件的选项。
type GzipOptions struct {
	// MinSize 未显式写状态码时，小于该字节数的响应不压缩（0=默认 1024）。
	MinSize int
	// Level 压缩级别（0=标准库默认；1-9 对应 BestSpeed-BestCompression）。
	Level int
}

// gzipPoolForLevel 返回指定压缩级别的 Writer 池。
func gzipPoolForLevel(level int) *sync.Pool {
	if v, ok := gzipPools.Load(level); ok {
		return v.(*sync.Pool)
	}
	pool := &sync.Pool{New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, level)
		return w
	}}
	actual, _ := gzipPools.LoadOrStore(level, pool)
	return actual.(*sync.Pool)
}

// Gzip 返回响应压缩中间件：客户端 Accept-Encoding 含 gzip 时启用。
func Gzip() core.HandlerFunc {
	return GzipWithOptions(GzipOptions{})
}

// GzipWithOptions 返回带选项的响应压缩中间件。
func GzipWithOptions(opts GzipOptions) core.HandlerFunc {
	minSize := opts.MinSize
	if minSize <= 0 {
		minSize = 1024
	}
	level := opts.Level
	if level == 0 || level < gzip.HuffmanOnly || level > gzip.BestCompression {
		level = gzip.DefaultCompression
	}
	pool := gzipPoolForLevel(level)
	return func(c *core.Context) {
		if !acceptsGzip(c.GetHeaderCanonical(canonicalAcceptEncoding)) {
			c.Next()
			return
		}
		orig := c.Writer()
		gz := pool.Get().(*gzip.Writer)
		gw := &gzipWriter{ResponseWriter: orig, gz: gz, minSize: minSize}
		c.SetWriter(gw)

		c.Next()
		_ = gw.Close()
		pool.Put(gz)
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
	started   bool
	decided   bool
	compress  bool
	minSize   int
	buf       []byte
}

// WriteHeader 记录状态码并透传。
func (w *gzipWriter) WriteHeader(code int) {
	if !w.wroteHead {
		w.wroteHead = true
		if !w.compressible() {
			delete(w.Header(), canonicalContentEncoding)
			w.ResponseWriter.WriteHeader(code)
			return
		}
		setHeader(w.Header(), canonicalContentEncoding, "gzip")
		setHeader(w.Header(), canonicalVary, "Accept-Encoding")
		w.gz.Reset(w.ResponseWriter)
		w.started = true
		w.ResponseWriter.WriteHeader(code)
	}
}

// Write 写入压缩数据。
func (w *gzipWriter) Write(p []byte) (int, error) {
	if !w.compressible() {
		return w.ResponseWriter.Write(p)
	}
	if !w.started {
		w.buf = append(w.buf, p...)
		if len(w.buf) >= w.minSize {
			w.startGzip(200)
			if w.started {
				return len(p), nil
			}
		}
		return len(p), nil
	}
	return w.gz.Write(p)
}

// Close 结束压缩；小响应（未达 MinSize 且未显式写状态码）按明文输出。
func (w *gzipWriter) Close() error {
	if w.started {
		return w.gz.Close()
	}
	if len(w.buf) > 0 {
		delete(w.Header(), canonicalContentEncoding)
		_, err := w.ResponseWriter.Write(w.buf)
		w.buf = nil
		return err
	}
	return nil
}

// compressible 判断响应内容类型是否值得压缩。
func (w *gzipWriter) compressible() bool {
	if w.decided {
		return w.compress
	}
	w.decided = true
	var ct string
	if v := w.Header()[canonicalContentType]; len(v) > 0 {
		ct = v[0]
	}
	if ct == "" {
		w.compress = true
		return true
	}
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	switch {
	case strings.HasPrefix(ct, "text/"),
		ct == "application/json",
		strings.HasPrefix(ct, "application/javascript"),
		strings.HasPrefix(ct, "application/xml"),
		ct == "application/xhtml+xml",
		ct == "image/svg+xml":
		w.compress = true
	default:
		w.compress = false
	}
	return w.compress
}

// startGzip 在首次隐式写入时启动压缩流。
func (w *gzipWriter) startGzip(code int) {
	if !w.wroteHead {
		w.wroteHead = true
		setHeader(w.Header(), canonicalContentEncoding, "gzip")
		setHeader(w.Header(), canonicalVary, "Accept-Encoding")
		w.gz.Reset(w.ResponseWriter)
		w.started = true
		w.ResponseWriter.WriteHeader(code)
		if len(w.buf) > 0 {
			_, _ = w.gz.Write(w.buf)
			w.buf = nil
		}
	}
}

// Flush 刷新压缩流。
func (w *gzipWriter) Flush() {
	if !w.started && len(w.buf) > 0 {
		w.startGzip(200)
	}
	if w.started {
		_ = w.gz.Flush()
	}
}

// Unwrap 返回底层 Writer。
func (w *gzipWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
