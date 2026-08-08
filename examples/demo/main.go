// demo 示例：errx + logx + confx + webx 完整服务模板。
package main

import (
	"github.com/lcylpzls/errx"
	errxlogx "github.com/lcylpzls/errx/logx"
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
		logger.Error("加载配置失败", logx.Fields(logx.Any("error", err)))
		return
	}

	s := webx.NewServer(cfg, logger)
	s.RegisterRoute(webx.Route{
		Method: "GET",
		Path:   "/api/users/:id",
		Handler: func(c *webx.Context) {
			if c.Param("id") == "0" {
				err := errx.New(errx.KindNotFound, "USER_NOT_FOUND", "用户不存在")
				logger.Warn("业务错误", errxlogx.Fields(err))
				c.Fail(404, 404, err.Message())
				return
			}
			c.Success("ok", map[string]string{"id": c.Param("id")})
		},
	})
	if err := s.Start(); err != nil {
		logger.Error("服务异常退出", logx.Fields(logx.Any("error", err)))
	}
}
