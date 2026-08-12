// metrics 示例：外置 metricsx 指标接入 + 路由/分组级统计 + Prometheus 暴露端点。
package main

import (
	"net/http"

	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/metricsx"
	"github.com/lcylpzls/metricsx/prometheus"
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

	metrics, err := metricsx.New(prometheus.WithPrometheus(
		prometheus.WithNamespace("webx_metrics"),
	))
	if err != nil {
		panic(err)
	}

	s := webx.NewServer(cfg, logger)
	// 指标事件转发给 metricsx 的 Sink（webx.Metrics = metricsx.Sink 契约）。
	s.WithMetrics(metrics.Sink())
	// 指标端点统一走 metricsx/prometheus.HTTPHandler，业务侧不直接依赖 promhttp。
	s.RegisterRoute(webx.Route{
		Method: http.MethodGet,
		Path:   "/metrics",
		Handler: func(c *webx.Context) {
			prometheus.HTTPHandler(metrics).ServeHTTP(c.Writer(), c.Request())
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
