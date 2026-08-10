// metrics 示例：外置 metricsx 指标接入 + 路由/分组级统计。
package main

import (
	"net/http"

	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/metricsx"
	"github.com/lcylpzls/webx/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	metrics, err := metricsx.New(metricsx.WithNamespace("myapp"))
	if err != nil {
		panic(err)
	}

	s := webx.NewServer(cfg, logger)
	// v2 不再内置指标端点：外部注入 metricsx，并自行挂载 Prometheus 暴露路由。
	s.WithMetrics(metrics)
	s.RegisterRoute(webx.Route{
		Method: http.MethodGet,
		Path:   "/metrics",
		Handler: func(c *webx.Context) {
			promhttp.Handler().ServeHTTP(c.Writer(), c.Request())
		},
	})
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
