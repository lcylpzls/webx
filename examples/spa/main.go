// spa 示例：embed 前端资源 + SPA 回退。
package main

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/webx"
)

//go:embed dist
var dist embed.FS

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
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	s := webx.NewServer(cfg, logger)
	s.ServeStaticFS("/", http.FS(sub))
	s.EnableSPA(http.FS(sub), "index.html")
	if err := s.Start(); err != nil {
		panic(err)
	}
}
