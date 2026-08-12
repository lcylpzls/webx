// production 示例：探针、可信代理、限流、并发限制、外置指标、上传与优雅关闭的综合生产模板。
package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/lcylpzls/clix"
	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/metricsx"
	"github.com/lcylpzls/metricsx/prometheus"
	"github.com/lcylpzls/webx"
)

func main() {
	app, err := clix.New("webx-production", "2.1.0",
		clix.WithDescription("webx 生产模板示例"),
		clix.WithIO(os.Stdout, os.Stderr),
		clix.WithGlobalFlags(
			clix.StringFlag("config", "配置文件路径（TOML）").Default("config.toml"),
			clix.StringFlag("uploads", "上传文件保存目录").Default("uploads"),
		),
		clix.WithRootAction(run),
	)
	if err != nil {
		panic(err)
	}
	os.Exit(app.Execute(context.Background(), os.Args[1:]))
}

// run 启动生产模板服务（clix 根 Action）。
func run(_ context.Context, c *clix.Context) error {
	logger, err := logx.NewBuilder().EnableConsole(logx.InfoLevel).Build()
	if err != nil {
		return err
	}
	defer logger.Close()

	cfg, err := webx.LoadConfig(c.GlobalString("config"))
	if err != nil {
		logger.Error("加载配置失败", logx.Fields(logx.Any("error", err)))
		return err
	}

	metrics, err := metricsx.New(prometheus.WithPrometheus(
		prometheus.WithNamespace("webx_production"),
	))
	if err != nil {
		logger.Error("创建指标适配器失败", logx.Fields(logx.Any("error", err)))
		return err
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
	s.SetMaxConcurrentRequests(200)
	s.EnableRateLimit(webx.RateLimitOptions{QPS: 100, Window: time.Second})
	s.RegisterLivenessCheck("live", func(ctx context.Context) error { return nil })
	s.RegisterReadinessCheck("db", func(ctx context.Context) error { return nil })
	uploadDir := c.GlobalString("uploads")
	_ = os.MkdirAll(uploadDir, 0o755)
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
			dest := filepath.Join(uploadDir, fh.Filename)
			if err := c.SaveUploadedFile(fh, dest); err != nil {
				c.Fail(http.StatusInternalServerError, http.StatusInternalServerError, "保存上传文件失败")
				return
			}
			c.Success("ok", map[string]string{"file": dest})
		})
	})
	if err := s.Start(); err != nil {
		logger.Error("服务异常退出", logx.Fields(logx.Any("error", err)))
		return err
	}
	return nil
}
