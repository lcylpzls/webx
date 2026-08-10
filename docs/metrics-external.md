# v2 外置指标接入指南

webx v2.0.0 起不再内置 Prometheus 指标端点。指标统一由家族
metricsx 底座采集，webx 只负责把请求事件转发给外部注入的实例。

## 快速接入

```go
m, err := metricsx.New(metricsx.WithNamespace("myapp"))
if err != nil {
	panic(err)
}

s := webx.NewServer(cfg, logger)
s.WithMetrics(m)

// 自行挂载 Prometheus 暴露路由（示例使用 promhttp）
s.RegisterRoute(webx.Route{
	Method: http.MethodGet,
	Path:   "/metrics",
	Handler: func(c *webx.Context) {
		promhttp.Handler().ServeHTTP(c.Writer(), c.Request())
	},
})
```

## 上报的指标

| 指标名 | 类型 | 标签 | 说明 |
| --- | --- | --- | --- |
| `webx.requests` | Counter | route, group, status, protocol | 请求数 |
| `webx.request_duration` | Histogram | route, group, status, protocol | 请求耗时（秒） |
| `webx.panics` | Counter | 无 | Recovery 捕获的 panic 数 |
| `webx.rate_limited` | Counter | 无 | 限流拒绝数 |
| `webx.concurrency_rejected` | Counter | 无 | 并发限制拒绝数 |
| `webx.requests_in_flight` | Gauge | 无 | 活跃请求水位 |
| `webx.active_connections` | Gauge | 无 | 当前连接数 |

瞬时量仅在注入的接收器实现 `webx.GaugeMetrics` 时上报；
只实现 `webx.Metrics` 的接收器自动跳过水位指标。

## 与 v1 的差异

- 删除 `EnableMetricsEndpoint`、`metrics_enabled` / `metrics_path`；
- `Metrics` 快照结构更名为 `MetricsSnapshot`；
- 快照 API（`Metrics()` / `RouteStats()` / `GroupStats()`）保留，
  用于运行状态查看，不再承担 Prometheus 暴露职责。
