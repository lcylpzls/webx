# Changelog

本项目遵循语义化版本（SemVer）。值得记录的变更统一维护在此文件。

## [v2.0.0] - 2026-08-10

### 破坏性变更

- Go 模块路径升级为 `github.com/lcylpzls/webx/v2`（v2 主版本规范），
  所有导入需追加 `/v2` 后缀；
- 移除内置 Prometheus 指标端点：`EnableMetricsEndpoint`、
  配置项 `metrics_enabled` / `metrics_path` 与 `/metrics` 文本渲染全部删除；
- 新增 `WithMetrics(webx.Metrics)`：外部注入统一指标接收器
  （metricsx 等家族底座天然满足），请求数/耗时/panic/限流/并发拒绝
  事件全部转发，活跃请求与连接水位通过可选 `GaugeMetrics` 接口上报；
- `Metrics` 快照结构更名为 `MetricsSnapshot`，`Server.Metrics()` /
  `RouteStats()` / `GroupStats()` 快照能力保留；
- `middleware.NewMetrics(sink)` 签名变更，`middleware.MetricsSink` /
  `GaugeSink` 为新的外部接收器接口。

### 迁移指引

- 删除配置中的 `metrics_enabled` / `metrics_path`；
- `s.EnableMetricsEndpoint("/metrics")` 改为
  `s.WithMetrics(m)` + 自行注册 promhttp 暴露路由（见 examples/metrics）。

### 质量

- 新增外部指标转发测试（含限流/并发拒绝事件与无 Gauge 实现降级）；
- 覆盖率保持 100%；race / vet / staticcheck / fuzz 全绿。

## [v1.2.5] - 2026-08-10

### 修复与规范

- 全局中间件链覆盖 404/405 兜底请求（此前只覆盖已匹配路由），
  保证 trace / 限流 / 安全等中间件全请求生效；
- 明确规范：webx 不内置分布式追踪中间件，链路追踪统一通过
  tracex 基座 + `github.com/lcylpzls/tracex/adapters/webx` 接入；
  `X-Request-ID` 请求 ID 能力保留（与链路追踪无关）。

## [v1.2.6] - 2026-08-10

### 文档修正

- 请求 ID 头名文档统一为 `X-Request-ID`（默认值本就是
  X-Request-ID，此前文档误写为 X-Trace-ID）。

## [v1.2.2] - 2026-08-09

### 性能文档

- 新增 Linux 虚拟机复测数据（Debian 13 / 2 核 / 1.9GiB / Go 1.26.5）：
  HTTP/1.1、HTTP/2、HTTP/3 三协议矩阵、QPS 换算与结论，含复现命令；
- 复测结论：HTTP/1.1 与 Windows 一致（webx 与 gin/echo/ServeMux 同档），
  HTTP/2 下 webx 仍领先 gin/echo 约 15%。

## [v1.2.1] - 2026-08-09

### 基准与性能文档

- 基准模块新增 hertz（v0.10.6 + `hertz-contrib/http2` v0.1.8）的
  HTTPS HTTP/1.1 与 HTTP/2 对比；
- 基准模块新增 HTTP/3 矩阵：ServeMux / gin / echo / webx 统一挂载
  quic-go `http3.Server`，客户端使用 `http3.Transport`；
- 修复基准模块 `startStdTLS` 未启用 HTTP/2 的问题（改为 `ServeTLS`），
  并为 h2/h3 各协议新增协商断言测试（防止静默回退 HTTP/1.1）；
- `BENCHMARKS.md` 重构：方法学、标准库阵营 h1/h2/h3 三协议矩阵、
  三协议总览、结论与限制；
- README 性能段同步更新为最新实测摘要。

## [v1.2.0] - 2026-08-08

### 性能（热路径深度优化）

- 响应/请求头写入全部改为预计算规范化键（`textproto` 键只计算一次），
  RequestID / CORS / Security / Gzip / Validation / AccessLog / 限流
  中间件全部切换，消除每请求的键规范化分配；
- `RemoteIP` 请求头读取免规范化；
- AccessLog `countingWriter` 池化；
- 新增全中间件基准（`BenchmarkServerRequestFull` /
  `BenchmarkServerTLSWebxFull`）与 CI 参考输出。

### 基准

- 全中间件并行：约 12.5µs/op（≈80k req/s），相对裸路径 9.3µs 增加约 35%
  （日志、压缩、安全头、超时上下文等中间件固有成本）。

### 依赖

- confx 跟进 v0.3.1（纯文档修订：明确解析失败不修改 out 的行为保证，
  无功能变更）。

## [v1.1.0] - 2026-08-08

### 性能（P0 零分配核心）

- 路由参数槽化：消除参数 map 分配，路由分发达到 **0 allocs/op**
  （500 条路由约 0.10µs，改造前约 0.34µs / 5 allocs）；
- 手写标准化响应信封编码：Success / Fail / JSONResponse 热路径
  零反射、零 fmt（0 allocs/op）；
- `Context.String` 无格式参数零分配快速路径；
- Content-Type 免 textproto 规范化分配；
- CI 增加分配数门禁：路由分发与标准化响应均要求 ≤1 allocs/op。

### 依赖

- confx 升级至 v0.3.0：适配 `ConfigManager` 统一入口
  （`LoadConfig` 改用 `confx.NewConfigManager(confx.Toml).Load`）。

## [v1.0.0] - 2026-08-08

### 定版

- 公开 API 冻结，apidiff 基线升级至 v1.0.0；
- API 设计决策点（D-1 ~ D-7）全部确认，终审记录见 docs/api-design.md；
- 文档清理：过时的迭代计划、ServeMux 表述与「待决策点」移除；
- 并发安全修补：限流/并发限制的拒绝文案改为原子读写，消除潜在数据竞争。

## [v0.32.0] - 2026-08-08

### 新增

- 内置错误文案可定制：`error_messages` 配置或 `SetErrorMessages`，覆盖
  404 / 405 / 413 / 429 / 503（并发繁忙与请求超时分开）；
- AccessLog 请求头白名单：`access_log_headers` 按白名单记录请求头，
  命中 `access_log_redact` 的值自动脱敏；
- 嵌套结构体绑定：`form` / `query` 支持嵌套结构与结构体指针递归绑定，
  带循环引用防护；
- 新增 `examples/production` 生产综合模板与 README 快速开始。

## [v0.31.0] - 2026-08-08

### 新增

- 可信代理：`trusted_proxies` 配置（CIDR 或 IP），仅来自可信网段的请求
  才采用 `X-Forwarded-For` / `X-Real-IP`；限流中间件统一复用
  `Context.RemoteIP()`，防止伪造源 IP 污染限流、审计与日志；
- Prometheus 端点增加运行时指标：`webx_goroutines`、
  `webx_mem_heap_alloc_bytes`、`webx_gc_count_total`；
- `Context.Bind`：按 Content-Type 自动分派 JSON / Form / Query；
- proxy：`WithDirector`（改写上游请求）与 `WithModifyResponse`（改写上游响应）。

### 变更

- `RemoteIP()` 默认不再信任代理头（安全默认），需显式配置 `trusted_proxies`。

## [v0.30.0] - 2026-08-08

### 新增（现代浏览器安全基线补全）

- `Content-Security-Policy` 与 `Content-Security-Policy-Report-Only`
  （`security_content_security_policy` / `security_content_security_policy_report_only`）；
- HSTS 指令升级：`security_hsts_include_subdomains`、`security_hsts_preload`；
- `Origin-Agent-Cluster: ?1`（`security_origin_agent_cluster`，站点隔离）；
- CORS 内网预检：`cors_allow_private_network` 输出
  `Access-Control-Allow-Private-Network: true`；
- `Context.SetSecureCookie`：自动补齐 Secure + HttpOnly + SameSite=Lax。

### 新增（其他）

- 绑定默认值：`form` / `query` tag 支持 `,default=xxx` 修饰；
- 优雅关闭强制兜底：`Shutdown` 超时后调用 `srv.Close()` 强制断开残余连接并合并错误；
- `FuzzBindQuery` 模糊目标并接入 CI。

### 说明

- 刻意不提供已废弃的 `X-XSS-Protection` 与 `Expect-CT`（现代浏览器已移除/忽略）。

## [v0.29.0] - 2026-08-08

### 新增

- 探针分离：`RegisterLivenessCheck`（`/healthz`）与 `RegisterReadinessCheck`
  （`/readyz`），路径可通过 `liveness_path` / `readiness_path` 配置；
  服务进入优雅关闭后就绪探针直接返回 503；
- Gzip 压缩级别配置：`gzip_level`（0=标准库默认，1-9 对应
  BestSpeed-BestCompression），按级别复用 Writer 池；
- Metrics 增加 `ConcurrencyRejected`（并发限制拒绝数），Prometheus 端点同步输出
  `webx_concurrency_rejected_total`；
- AccessLog 增加 `scheme` 字段（TLS 与 HTTP/3 记为 https）。

## [v0.28.0] - 2026-08-08

### 新增

- 并发限制：`SetMaxConcurrentRequests(n)`，同一时刻请求数超限返回 503 +
  `Retry-After`（防雪崩，panic 时额度自动释放）；
- 慢请求日志：`slow_request_threshold` 配置，请求耗时达到阈值时额外记录 Warn
  （默认关闭，不受成功日志开关与采样率影响）；
- Context 便捷方法：`Redirect`、`Cookie`、`SetCookie`、`File`
  （单文件输出支持 304 与 Range）；
- 静态服务可选弱 ETag：`StaticOptions.EnableETag`，`If-None-Match` 命中返回 304。

### 变更

- 内置中间件默认顺序新增 `concurrency_limit`（位于 body_limit 之后）。

## [v0.27.0] - 2026-08-08

### 新增

- 文件上传：`Context.FormFile(name)` 获取上传文件、
  `Context.SaveUploadedFile(fh, dest)` 安全落盘（沿用 `max_body_bytes` 限制）；
- 全局请求体限制：`max_body_bytes` 对所有请求生效，Content-Length 超限直接 413，
  chunked 请求体由 MaxBytesReader 兜底；
- CORS 增加 `ExposeHeaders`（默认暴露 X-Request-ID）；
- 安全头增加 `Cross-Origin-Resource-Policy` 与 `Cross-Origin-Embedder-Policy`；
- 限流拒绝响应增加 `Retry-After` 头（按令牌桶恢复时间向上取整）。

### 变更

- 超大请求体由 Bind 阶段返回 400 改为全局 413（行为变更）。

## [v0.26.0] - 2026-08-08

### 新增

- Prometheus 指标端点：`EnableMetricsEndpoint(path)` 或配置 `metrics_enabled` /
  `metrics_path`（默认 `/metrics`），输出文本格式指标（请求数、状态码分布、
  协议维度、路由/分组级、限流、panic、连接数、活跃请求）；
  端点绕过业务中间件链，避免自采集反馈；
- `Context.BindQuery`：查询参数绑定（`query` tag），与 BindJSON/BindForm 对齐；
- proxy：`WithTimeout`（上游整体超时）与 `WithFlushInterval`（SSE 流式刷新）；
- `SetRequestIDOptions`：自定义请求 ID 头名与生成函数。

### 变更

- 请求 ID 生成器由自研 UUID v4 升级为 google/uuid 的 UUID v7
  （`uuid.Must(uuid.NewV7())`，时间有序，适合分布式链路 ID）；
- 新增第三方直接依赖：`github.com/google/uuid`。

## [v0.25.0] - 2026-08-08

### 新增

- 新增 `Server.RouteStats()`：按注册路径聚合请求数、5xx 数与平均耗时；
- 新增 `Server.GroupStats()`：按分组前缀聚合（直接注册的路由不计入分组）；
- Metrics panic 安全采样：处理器 panic 时仍完整记录请求数、耗时与 5xx 分布，
  随后重新抛出交由 Recovery 中间件处理；
- AccessLog 协议字段可读化：`HTTP/2.0` → `HTTP/2`、`HTTP/3.0` → `HTTP/3`。

## [v0.24.0] - 2026-08-08

### 新增

- Metrics 增加状态码分布：`Status1xx` ~ `Status5xx`；
- Metrics 增加协议维度：HTTP/1.x、HTTP/2、HTTP/3 分别统计请求数与平均耗时
  （`HTTP1Requests` / `HTTP2Requests` / `HTTP3Requests` 与
  `AvgHTTP1RequestDurationMs` / `AvgHTTP2RequestDurationMs` / `AvgHTTP3RequestDurationMs`）；
- 新增 `examples/order`：`SetMiddlewareOrder` 自定义中间件顺序示例。

### 文档

- README 补充 Windows Unix Socket 版本限制（build 1803 / 10.0.17134）；
- `config.example.toml` 补充中间件默认顺序与 `SetMiddlewareOrder` 说明；
- README 合并重复的功能清单，指标说明更新。

## [v0.23.0] - 2026-08-08

### 新增

- 中间件顺序可配置：`SetMiddlewareOrder`（默认顺序保持不变，非法键忽略）；
- CI：race 检测扩展到 Windows / macOS 全平台。

## [v0.22.0] - 2026-08-08

### 新增

- Metrics 增加 `RequestsInFlight`（活跃请求数）；
- AccessLog 增加协议字段（HTTP/1.1、HTTP/2、HTTP/3）；
- `SetConnContext` 每连接上下文注入、`RegisterOnShutdown` 关闭钩子；
- 安全头扩展：`Permissions-Policy`、`Cross-Origin-Opener-Policy`；
- `FuzzBindForm` 模糊目标。

## [v0.21.0] - 2026-08-08

### 工程

- API 终审第二轮：公开 API 无待调整项，命名与签名保持稳定；
- apidiff 冻结基线更新至 v0.20.0；
- README 当前版本信息完善。

## [v0.20.0] - 2026-08-08

### 新增

- `Context.BindForm`：multipart/urlencoded 表单绑定（form tag，含大小限制）；
- CORS `AllowCredentials` 支持（`CORSAllowCredentials`）；
- `RespondErrorWithData`：errx 错误响应附带业务数据；
- 未显式注册 OPTIONS 时按 Allow 自动响应 204。

### 工程

- 移除既有框架迁移指南及相关字样，webx 作为独立新项目定位。

## [v0.19.0] - 2026-08-08

### 新增

- Metrics 增加 `ActiveConnections`（连接数实时统计）；
- AccessLog 增加响应字节数（`bytes`）；
- 新增 `webx/pprof` 子包：一键注册 /debug/pprof 性能诊断端点；
- CI Benchmark 任务增加路由性能回退阈值（500 条路由 > 1000ns 即失败）。

## [v0.18.0] - 2026-08-08

### 新增

- Debug 模式：`Config.Debug` 开启后 Recovery 响应携带 panic 摘要；
- proxy：`WithErrorHandler` 自定义上游错误处理，默认输出统一 JSON 502；
- 导出 `NoRouteHandler`/`NoMethodHandler`，新增 `ExampleNewRouter` godoc 示例。

## [v0.17.0] - 2026-08-08

### 性能

- 移除 http.ServeMux，路由改为自研 radix 匹配树（匹配与分发完全自主）；
- 基准：50/100/500 条路由均由线性扫描+ServeMux 提升到 ~0.21µs/op，
  500 条路由提升约 50 倍（11.2µs → 0.22µs），分配 4 allocs/op。

## [v0.16.0] - 2026-08-08

### 新增

- 安全响应头中间件（`MiddlewareSecurity` + HSTS/Referrer-Policy 配置）；
- Gzip 增强：跳过已压缩内容类型（图片/二进制等）、`GzipMinSize` 小响应不压缩；
- RequestID 出站透传：X-Request-ID 同步写入请求头，上游链路可关联；
- CI 覆盖率门槛：语句覆盖率非 100% 直接失败。

## [v0.15.0] - 2026-08-08

### 新增

- SNI 多证书：`SetSNICertificates` 按域名选择证书，未匹配回退默认证书；
- 请求钩子中间件 `middleware.Hooks`（OnRequest/OnResponse，可作 OTel 适配点）；
- 大路由基准：100/500 条路由；
- CI 新增 Benchmark 任务；
- HTTP/3 排空：`QUICDrainTimeout` 配置关闭前等待时间。

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

- README 增加功能清单与性能数据。

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

- 项目立项：基于标准库实现工业级 HTTP/HTTPS 服务组件库；
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
