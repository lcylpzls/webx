// order 示例：通过 SetMiddlewareOrder 自定义内置中间件执行顺序。
package main

import (
	"net/http"

	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/webx/v2"
)

func main() {
	logger, err := logx.NewBuilder().EnableConsole(logx.InfoLevel).Build()
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	cfg, err := webx.LoadConfig("config.toml")
	if err != nil {
		panic(err)
	}

	s := webx.NewServer(cfg, logger)
	// 自定义顺序：请求 ID 最先（供后续中间件使用），限流提前拦截，
	// 访问日志与指标最后收尾（保证统计到完整处理耗时）。
	s.SetMiddlewareOrder([]webx.MiddlewareType{
		webx.MiddlewareRequestID,
		webx.MiddlewareRateLimit,
		webx.MiddlewareTimeout,
		webx.MiddlewareRecovery,
		webx.MiddlewareCORS,
		webx.MiddlewareValidation,
		webx.MiddlewareSecurity,
		webx.MiddlewareGzip,
		webx.MiddlewareMetrics,
		webx.MiddlewareAccessLog,
	})
	s.RegisterRoute(webx.Route{
		Method: http.MethodGet,
		Path:   "/",
		Handler: func(c *webx.Context) {
			c.Success("ok", map[string]string{"requestId": c.RequestID()})
		},
	})
	if err := s.Start(); err != nil {
		panic(err)
	}
}
