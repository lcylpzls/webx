// fluent 示例：多通道监听 + 路由分组 + 限流。
package main

import (
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
	s.UseHttp2Listen(":8443").
		UseHttp3Listen(":8443"). // TCP/UDP 同端口不冲突
		RegisterRoute(webx.Route{
			Method: "GET",
			Path:   "/api/users/:id",
			Handler: func(c *webx.Context) {
				c.Success("ok", map[string]string{"id": c.Param("id")})
			},
		}).
		RegisterRouteGroup("/api/v2", func(rg *webx.RouteGroup) {
			rg.GET("/products", func(c *webx.Context) { c.Success("ok", "products") })
		}).
		EnableRateLimit(webx.RateLimitOptions{
			QPS:    200,
			Window: 1,
		})
	if err := s.Start(); err != nil {
		panic(err)
	}
}
