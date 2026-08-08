// Package proxy 提供基于标准库 httputil.ReverseProxy 的上游代理封装。
package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/lcylpzls/webx"
)

// Option 配置 ReverseProxy 的选项。
type Option func(*httputil.ReverseProxy)

// Handler 返回反向代理处理器，将请求转发到 target。
func Handler(target *url.URL, opts ...Option) webx.HandlerFunc {
	rp := newProxy(target, opts...)
	return func(c *webx.Context) {
		rp.ServeHTTP(c.Writer(), c.Request())
	}
}

// newProxy 创建配置完成的 ReverseProxy（测试可直接校验选项效果）。
func newProxy(target *url.URL, opts ...Option) *httputil.ReverseProxy {
	rp := httputil.NewSingleHostReverseProxy(target)
	if rp.ErrorHandler == nil {
		rp.ErrorHandler = DefaultErrorHandler
	}
	for _, o := range opts {
		o(rp)
	}
	return rp
}

// WithErrorHandler 设置上游错误处理器。
func WithErrorHandler(fn func(http.ResponseWriter, *http.Request, error)) Option {
	return func(rp *httputil.ReverseProxy) {
		rp.ErrorHandler = fn
	}
}

// WithTimeout 设置上游请求整体超时；<=0 表示不限制。
// 超时覆盖连接与响应头阶段，超时后交由 ErrorHandler 输出错误响应。
func WithTimeout(d time.Duration) Option {
	return func(rp *httputil.ReverseProxy) {
		if d <= 0 {
			return
		}
		base := rp.Transport
		if base == nil {
			base = http.DefaultTransport
		}
		rp.Transport = &timeoutRoundTripper{base: base, timeout: d}
	}
}

// WithFlushInterval 设置流式响应刷新间隔。
// 负数表示每次写入后立即刷新（SSE 等长连接场景）；0 使用标准库默认行为。
func WithFlushInterval(d time.Duration) Option {
	return func(rp *httputil.ReverseProxy) {
		rp.FlushInterval = d
	}
}

// timeoutRoundTripper 为上游请求注入整体超时上下文。
type timeoutRoundTripper struct {
	base    http.RoundTripper
	timeout time.Duration
}

// RoundTrip 带超时执行上游请求。
func (t *timeoutRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(r.Context(), t.timeout)
	defer cancel()
	return t.base.RoundTrip(r.WithContext(ctx))
}

// DefaultErrorHandler 输出统一 JSON 502 错误响应。
func DefaultErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	_, _ = fmt.Fprintf(w, `{"code":502,"msg":"上游服务不可用"}`)
}
