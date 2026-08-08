// metrics 示例：Prometheus 指标端点 + 路由/分组级统计。
package main

import (
	"net/http"

	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/webx"
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
	// 启用 Prometheus 文本格式指标端点（绕过业务中间件链，避免自采集反馈）。
	s.EnableMetricsEndpoint("/metrics")
	s.RegisterRouteGroup("/api", func(rg *webx.RouteGroup) {
		rg.GET("/users/:id", func(c *webx.Context) {
			c.Success("ok", map[string]string{"id": c.Param("id")})
		})
	})
	s.RegisterRoute(webx.Route{
		Method: http.MethodGet,
		Path:   "/ping",
		Handler: func(c *webx.Context) {
			c.Success("pong", nil)
		},
	})
	if err := s.Start(); err != nil {
		panic(err)
	}
}
