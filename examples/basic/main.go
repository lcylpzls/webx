// basic 示例：最小 HTTP/2 服务。
package main

import (
	"github.com/lcylpzls/webx"
)

func main() {
	// 配置通过 confx 从 TOML 加载（见同目录 config.toml）。
	cfg, err := webx.LoadConfig("config.toml")
	if err != nil {
		panic(err)
	}
	if err := webx.NewServer(cfg).
		UseHttp2Listen(":8443").
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
