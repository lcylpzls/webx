// Package pprof 注册标准库 net/http/pprof 处理器，便于线上性能诊断。
package pprof

import (
	"net/http/pprof"

	"github.com/lcylpzls/webx"
)

// Registrar 抽象路由注册能力（*webx.Server 满足）。
type Registrar interface {
	RegisterRoute(webx.Route) *webx.Server
}

// Register 注册 /debug/pprof 相关处理器。
func Register(s Registrar) *webx.Server {
	handlers := []webx.Route{
		{Method: "GET", Path: "/debug/pprof", Handler: func(c *webx.Context) {
			pprof.Index(c.Writer(), c.Request())
		}},
		{Method: "GET", Path: "/debug/pprof/cmdline", Handler: func(c *webx.Context) {
			pprof.Cmdline(c.Writer(), c.Request())
		}},
		{Method: "GET", Path: "/debug/pprof/symbol", Handler: func(c *webx.Context) {
			pprof.Symbol(c.Writer(), c.Request())
		}},
	}
	var srv *webx.Server
	for _, r := range handlers {
		srv = s.RegisterRoute(r)
	}
	return srv
}
