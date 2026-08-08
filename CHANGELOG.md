# Changelog

本项目遵循语义化版本（SemVer）。值得记录的变更统一维护在此文件。

## [v0.14.0] - 2026-08-08

### 新增

- `config.example.toml` 全字段参考配置；
- 静态文件选项：`ServeStatic*WithOptions`（Cache-Control、禁用目录索引）；
- 限流维度可配置：`RateLimitOptions.KeyFunc`（默认按 IP）；
- README API 速查表。

## [v0.13.0] - 2026-08-08

### 新增

- TLS 配置化与证书热重载：`MinTLSVersion` 可配置；
  `SetCertificateLoader` 支持自定义加载器（SNI/KMS），默认按文件 mtime 自动重载；
- QUIC 参数配置化：`QUICMaxIdleTimeout`（默认 30s）、
  `QUICMaxIncomingStreams`（默认 100）；
- proxy 子包补充 Upgrade（WebSocket）透传测试；
- HTTP/3 并发冒烟提升至 50 并发。

## [v0.12.0] - 2026-08-08

### 依赖

- quic-go 升级至 v0.61.0（qpack v0.6.0），适配 `Accept` 返回
  `*quic.Conn` 的新 API；HTTP/3 全链路测试通过。

## [v0.11.0] - 2026-08-08

### 增强

- 路由分组前缀支持参数（如 `/api/:ver`），补齐专项测试；
- Validation 中间件支持 multipart/form-data 请求体大小限制（10MB）；
- 新增 godoc 示例 `ExampleStatusForError`；
- 新增 Dependabot（Go 模块与 GitHub Actions 每周更新检查）。

## [v0.10.0] - 2026-08-08

### 新增

- errx 集成：`RespondError`/`StatusForError` 将 errx 错误自动映射为
  标准化 HTTP 响应；导出 `NewContext` 便于自定义路由器嵌入；
- 健康检查注册：`RegisterHealthCheck(name, fn)`，/health 输出各检查项状态，
  失败时返回 503；
- `webx/proxy` 子包：基于标准库 `httputil.ReverseProxy` 的上游代理封装。

## [v0.9.0] - 2026-08-08

### 增强

- Slowloris 防护：`Config.ReadHeaderTimeout` 默认 10s、`IdleTimeout` 默认 60s；
- Recovery 中间件支持注入 logger，panic 输出 requestId 与调用栈；
- 限流器增加 IP 桶数量上限（`SetMaxBuckets`，默认 10 万）；
- `ListenerAddr()` 支持 HTTP/3-only 动态端口；
- Metrics 增加平均请求耗时（`AvgRequestDurationMs`）；
- gzip.Writer 池化复用，降低压缩路径分配。

### 工程

- CI 的 apidiff 基线更新至 v0.8.0（v0.7.0 的 AccessLog 签名调整属于 0.x 合法破坏性变更）。

## [v0.8.0] - 2026-08-08

### 性能

- 请求上下文（Context）池化复用，降低每请求分配；
- 新增并发池化路由测试（race 下验证）。

### 示例

- 新增 examples/demo：errx + logx + confx + webx 完整服务模板。

## [v0.7.0] - 2026-08-08

### 增强

- Metrics 扩展：`RateLimited`（限流拒绝数）与 `Panics`（Recovery 捕获数）；
- AccessLog 采样（`AccessLogSampleRate`）与 query 脱敏（`AccessLogRedact`）；
- 新增真实请求级基准 `BenchmarkServerRequest`；
- 新增组合场景测试：中间件全开、SSE Flush 透传（Timeout+Gzip）、HTTP/3 并发。

### 工程

- CI 的 apidiff 基线更新至 v0.6.0。

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
