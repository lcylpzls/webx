// production 示例：探针、可信代理、限流、并发限制、外置指标、上传与优雅关闭的综合生产模板。
package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/metricsx"
	"github.com/lcylpzls/webx"
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
		logger.Error("加载配置失败", logx.Fields(logx.Any("error", err)))
		return
	}

	metrics, err := metricsx.New(metricsx.WithNamespace("myapp"))
	if err != nil {
		logger.Error("创建指标适配器失败", logx.Fields(logx.Any("error", err)))
		return
	}

	s := webx.NewServer(cfg, logger)
	s.WithMetrics(metrics)
	s.RegisterRoute(webx.Route{
		Method: http.MethodGet,
		Path:   "/metrics",
		Handler: func(c *webx.Context) {
			promhttp.Handler().ServeHTTP(c.Writer(), c.Request())
		},
	})
	s.SetMaxConcurrentRequests(200)
	s.EnableRateLimit(webx.RateLimitOptions{QPS: 100, Window: time.Second})
	s.RegisterLivenessCheck("live", func(ctx context.Context) error { return nil })
	s.RegisterReadinessCheck("db", func(ctx context.Context) error { return nil })
	_ = os.MkdirAll("uploads", 0o755)
	s.RegisterRouteGroup("/api", func(rg *webx.RouteGroup) {
		rg.GET("/users/:id", func(c *webx.Context) {
			c.Success("ok", map[string]string{"id": c.Param("id")})
		})
		rg.POST("/upload", func(c *webx.Context) {
			fh, err := c.FormFile("file")
			if err != nil {
				c.Fail(http.StatusBadRequest, http.StatusBadRequest, "缺少上传文件")
				return
			}
			dest := filepath.Join("uploads", fh.Filename)
			if err := c.SaveUploadedFile(fh, dest); err != nil {
				c.Fail(http.StatusInternalServerError, http.StatusInternalServerError, "保存上传文件失败")
				return
			}
			c.Success("ok", map[string]string{"file": dest})
		})
	})
	if err := s.Start(); err != nil {
		logger.Error("服务异常退出", logx.Fields(logx.Any("error", err)))
	}
}
