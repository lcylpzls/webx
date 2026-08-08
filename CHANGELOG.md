# Changelog

本项目遵循语义化版本（SemVer）。值得记录的变更统一维护在此文件。

## [v0.6.0] - 2026-08-08

### API 调整

- `Server.Metrics()` 改为返回 `webx.Metrics` 结构体（`Requests`/`Errors5xx`）；
- `RouteGroup` 新增 `HEAD`/`OPTIONS` 注册方法。

### 文档

- 新增 ginx → webx 迁移指南，README 增加功能清单与性能数据。

## [v0.5.0] - 2026-08-08

### 新增

- Gzip 响应压缩中间件（`Config.MiddlewareGzip`），按 Accept-Encoding 协商；
- 请求/5xx 计数中间件（`Config.MiddlewareMetrics`）与 `Server.Metrics()`；
- SPA embed 示例（examples/spa）。

### 增强

- AccessLog 增加 `duration_ms`、`user_agent`、`host`、`query` 字段。

## [v0.4.0] - 2026-08-08

### 性能

- 路由匹配改为按需分段、零分配实现，消除 `splitPath` 的 `strings.Split`
  分配（此前占路由路径全部分配的 94%）；
- 基准提升：路由 `5159ns/op、107 allocs` → `1523ns/op、6 allocs`。

### 修复

- 精确路由不再前缀匹配更长路径或尾斜杠路径（与 ServeMux 语义对齐）。

## [v0.3.0] - 2026-08-08

### 新增

- `BindJSON` 请求体大小限制：`Config.MaxBodyBytes`（默认 10MB，`toml:"max_body_bytes"`）；
- `FuzzRouterServeHTTP` 模糊目标：随机方法/路径/模式，确保路由处理不 panic。

### 修复

- 路由分组回调 panic 不再导致启动崩溃，转为 `WEBX_START_FAILED` 错误；
- HTTP/3 预期关闭（`quic.ErrServerClosed`）降级为 Info 日志，不再误报 Error；
- 文档补充使用禁忌：Handler 内调 Stop、Stop 后不可重启、logger 注入要求。

### 工程

- CI 新增 apidiff API 兼容检查，以 v0.2.0 为冻结基线。

## [v0.2.0] - 2026-08-08

### 破坏性变更

- `NewServer(cfg, logger)`：logger 改为由调用方注入 `logx.Logger`，
  webx 内部不再创建日志器，只负责使用；`logger` 为 nil 时 `Start()` 返回错误。

### 修复

- HEAD 请求正确命中 GET 路由（HTTP 语义），不再误报 405；
- 405 判定改为"最具体模式优先"，与 ServeMux 的选择一致，避免纯文本 405 混入；
- Timeout 中间件的 Writer 支持 `Unwrap()`，`http.ResponseController` 的
  Flush/Hijack 在 SSE、WebSocket 场景可用。

### 工程

- 新增基准测试：路由匹配、中间件链、JSON 响应。

## [v0.1.0] - 2026-08-08

### 规划

- 项目立项：基于标准库实现与 ginx 对等的 HTTP/HTTPS 服务组件库；
- 技术栈确定：net/http + logx + errx + confx + quic-go（唯一直接第三方依赖）；
- 文档阶段：架构、API 草案、迭代计划、ADR 已就绪。

### 实现

- 自研 `Context` 与中间件链（Next/Abort），零第三方类型泄漏；
- 基于 `http.ServeMux` 的路由：gin 风格 `:id`/`*filepath` 自动翻译、
  404/405 标准化 JSON、尾斜杠重定向、路由冲突安全报错；
- 多通道监听：HTTP/2（TLS）、HTTP/3（QUIC）、Unix Socket，支持启动回滚；
- 内置中间件：Recovery / RequestID / Timeout / CORS / Validation /
  RateLimit / AccessLog；
- 标准化响应信封、健康检查、静态文件与 SPA 回退、优雅关闭；
- 配置经 confx 从 TOML 加载，错误统一 errx，日志接入 logx；
- 质量：三包语句覆盖率 100%、fuzz 三个目标、CI 三平台矩阵（待仓库推送后生效）。

### 修复

- 优雅关闭：HTTP/3 活动连接通过 `http3.Server.Close()` 终止，不再遗留连接 goroutine；
- 信号监听注册在 waitSignal 退出时注销（`signal.Stop`），避免重复注册泄漏；
- `Start()` 启动失败时回收限流清理 goroutine，避免后台 goroutine 泄漏。
