// Package proxy 提供基于标准库 httputil.ReverseProxy 的上游代理封装。
package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/lcylpzls/webx"
)

// Option 配置 ReverseProxy 的选项。
type Option func(*httputil.ReverseProxy)

// Handler 返回反向代理处理器，将请求转发到 target。
func Handler(target *url.URL, opts ...Option) webx.HandlerFunc {
	rp := httputil.NewSingleHostReverseProxy(target)
	if rp.ErrorHandler == nil {
		rp.ErrorHandler = DefaultErrorHandler
	}
	for _, o := range opts {
		o(rp)
	}
	return func(c *webx.Context) {
		rp.ServeHTTP(c.Writer(), c.Request())
	}
}

// WithErrorHandler 设置上游错误处理器。
func WithErrorHandler(fn func(http.ResponseWriter, *http.Request, error)) Option {
	return func(rp *httputil.ReverseProxy) {
		rp.ErrorHandler = fn
	}
}

// DefaultErrorHandler 输出统一 JSON 502 错误响应。
func DefaultErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	_, _ = fmt.Fprintf(w, `{"code":502,"msg":"上游服务不可用"}`)
}
