// Package proxy 提供基于标准库 httputil.ReverseProxy 的上游代理封装。
package proxy

import (
	"net/http/httputil"
	"net/url"

	"github.com/lcylpzls/webx"
)

// Option 配置 ReverseProxy 的选项。
type Option func(*httputil.ReverseProxy)

// Handler 返回反向代理处理器，将请求转发到 target。
func Handler(target *url.URL, opts ...Option) webx.HandlerFunc {
	rp := httputil.NewSingleHostReverseProxy(target)
	for _, o := range opts {
		o(rp)
	}
	return func(c *webx.Context) {
		rp.ServeHTTP(c.Writer(), c.Request())
	}
}
