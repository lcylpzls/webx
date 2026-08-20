// basic 示例：最小 HTTP/HTTPS 服务。
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
	// 配置通过 confx 从 TOML 加载（见同目录 config.toml）。
	cfg, err := webx.LoadConfig("config.toml")
	if err != nil {
		panic(err)
	}
	if err := webx.NewServer(cfg, logger).
		UseHttp1or2Listen(":8443", true).
		RegisterRoute(webx.Route{
			Method: "GET",
			Path:   "/ping",
			Handler: func(c *webx.Context) {
				c.Success("pong", nil)
			},
		}).
		Start(); err != nil {
		panic(err)
	}
}
