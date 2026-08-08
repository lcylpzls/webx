# webx

**基于 Go 标准库的工业级 HTTP/HTTPS 服务组件库**：
路由、上下文、中间件链全部自研，传输层基于 `net/http`。

[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.26-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

> 当前状态：**v1.2.0 已发布，API 冻结**。各包语句覆盖率 100%，
> 三平台 CI + race + fuzz + apidiff 全绿。

## 技术栈

| 组件 | 用途 | 说明 |
| --- | --- | --- |
| Go 标准库 `net/http` | HTTP/1.1、HTTP/2、路由、生命周期 | 唯一运行时基座 |
| [logx](https://github.com/lcylpzls/logx) | 结构化日志 | 自家库，零第三方依赖 |
| [errx](https://github.com/lcylpzls/errx) | 结构化错误 | 自家库，统一错误码 |
| [confx](https://github.com/lcylpzls/confx) | TOML 配置加载 | 自家库 |
| [quic-go](https://github.com/quic-go/quic-go) | HTTP/3 (QUIC) 传输 | 第三方直接依赖之一 |
| [google/uuid](https://github.com/google/uuid) | UUID v7 请求 ID | 标准实现，`uuid.Must` 稳定生成 |

> 第三方直接依赖：`quic-go`（HTTP/3）与 `google/uuid`（UUID v7），其余全部自研。

## 快速开始

1. 复制 `config.example.toml` 为 `config.toml`，填入 TLS 证书路径并按需修改；
2. 从 [examples/basic](examples/basic) 起步体验最小服务；
3. 生产部署直接参考 [examples/production](examples/production)：
   探针、可信代理、限流、并发限制、指标端点、上传与优雅关闭全配置模板。

## 设计原则

- **零泄漏**：不导出任何第三方类型，业务 Handler 只接触自研 `*Context`；
- **链式 API**：Builder 风格配置，配置即文档；
- **启动后不可变**：`Start()` 之后修改配置仅记录警告；
- **绝不 panic**：唯一 `recover()` 位于 Recovery 中间件；
- **绝不吞错**：所有错误路径返回 errx 结构化错误并记录日志；
- **简体中文**：日志、打印、注释、文档统一简体中文；
- **工业级门槛**：语句覆盖率 100%、fuzz、race、三平台 CI、API 基线。

## 功能清单

- 🔒 强制 TLS：HTTP/2（TLS over TCP）+ HTTP/3（QUIC over UDP）+ Unix Socket 多通道同时监听；
- 🔗 链式配置：路由、分组、中间件、限流、静态文件一键组装；
- 🗺️ 路由：`:id` / `*filepath` 参数语法、404/405 JSON、尾斜杠重定向；
- 🧱 内置中间件：Recovery / RequestID / Timeout / CORS / Validation / RateLimit /
  Gzip / Metrics / AccessLog / Security / BodyLimit（顺序可配置）；
- 📤 文件上传：`FormFile` / `SaveUploadedFile`（沿用请求体大小限制）；
- 📏 全局请求体限制：`max_body_bytes` 超限直接 413；
- 🛡️ 并发限制：`SetMaxConcurrentRequests` 超限返回 503 + Retry-After（防雪崩）；
- 🔐 浏览器安全基线：CSP、HSTS 全指令、COOP/CORP/COEP、Origin-Agent-Cluster、PNA；
- 🐢 慢请求日志：`slow_request_threshold` 超阈值自动 Warn；
- ⚡ 静态资源 ETag：可选弱 ETag，`If-None-Match` 命中返回 304；
- 🕵️ 可信代理：`trusted_proxies` 配置，代理头防伪造（限流/审计/日志统一）；
- 📊 运行时指标：goroutine、堆内存、GC 计数进入 `/metrics`；
- ✏️ 错误文案可定制：`error_messages` 覆盖内置 404/405/413/429/503；
- 📋 AccessLog 请求头白名单：`access_log_headers` + 复用脱敏；
- 📊 标准化响应：`{code, msg, data, requestId, timestamp}`；
- 🩺 健康检查：`/health`，路径可配置；
- 🔍 探针分离：`/healthz` 存活 / `/readyz` 就绪（优雅关闭中就绪自动 503）；
- 📈 Prometheus 指标端点：`/metrics` 文本格式输出（零第三方依赖）；
- 🪵 日志接入 logx；错误接入 errx；配置接入 confx（TOML）；
- 🧹 优雅关闭：SIGINT/SIGTERM 信号捕获 + `Stop(ctx)`；
- 🗂️ 静态文件服务与 SPA 回退（支持 `embed`）。

## 文档索引

- [docs/README.md](docs/README.md) — 文档索引与阅读顺序
- [docs/architecture.md](docs/architecture.md) — 架构设计
- [docs/api-design.md](docs/api-design.md) — API 设计定稿（决策点已冻结）
- [docs/iteration-plan.md](docs/iteration-plan.md) — 迭代计划与质量门槛
- [docs/decisions.md](docs/decisions.md) — 架构决策记录（ADR）

## 性能数据（Benchmark 实测）

| 基准 | 结果 |
| --- | --- |
| 路由匹配+分发（50 条路由） | 0.21µs/op，4 allocs |
| 路由匹配+分发（100 条路由） | 0.21µs/op，4 allocs |
| 路由匹配+分发（500 条路由） | 0.22µs/op，4 allocs |
| 中间件链（3 层） | 10.6ns/op，0 allocs |
| 标准化 JSON 响应 | 148ns/op，1 alloc |
| 端到端 HTTPS 请求（TLS+中间件+JSON） | 41µs/op，90 allocs |

与 gin / echo / fasthttp 的横向对比（HTTPS、单核/多核）见
[benchmarks/BENCHMARKS.md](benchmarks/BENCHMARKS.md)。

## API 速查

| 能力 | API |
| --- | --- |
| 创建服务 | `webx.NewServer(cfg, logger)` |
| 加载配置 | `webx.LoadConfig("config.toml")` |
| 多通道监听 | `UseHttp2Listen / UseHttp3Listen / UseUnixSocketListen` |
| 路由/分组 | `RegisterRoute / RegisterRouteGroup / RouteGroup.GET...` |
| 中间件管理 | `UseGlobalMiddleware / OverrideMiddleware / Disable/EnableMiddleware` |
| 请求 ID | `SetRequestIDOptions`（自定义头名与生成器，默认 UUID v7） |
| 指标端点 | `EnableMetricsEndpoint("/metrics")`（Prometheus 文本格式） |
| 并发限制 | `SetMaxConcurrentRequests(n)`（超限 503 + Retry-After） |
| 内置中间件 | Recovery、RequestID、BodyLimit、Timeout、CORS、Validation、RateLimit、Gzip、Metrics、AccessLog、Security |
| 标准化响应 | `c.Success / c.Fail / c.JSONResponse / c.AbortWithStatusJSON` |
| 参数绑定 | `c.BindJSON / c.BindForm / c.BindQuery` |
| 自动绑定 | `c.Bind(out)`（按 Content-Type 自动分派 JSON/Form/Query） |
| 嵌套绑定 | `form` / `query` tag 支持嵌套结构体与结构体指针 |
| 文件上传 | `c.FormFile(name)` / `c.SaveUploadedFile(fh, dest)` |
| Web 便捷方法 | `c.Redirect / c.Cookie / c.SetCookie / c.SetSecureCookie / c.File` |
| errx 集成 | `webx.RespondError / StatusForError` |
| 健康检查 | `RegisterHealthCheck`（/health 聚合输出） |
| 存活/就绪探针 | `RegisterLivenessCheck`（/healthz）/ `RegisterReadinessCheck`（/readyz） |
| 静态/SPA | `ServeStaticDir / ServeStaticFS / ServeStatic*WithOptions（MaxAge/DisableIndex/EnableETag）/ EnableSPA` |
| 反向代理 | `webx/proxy.Handler(target, opts...)` |
| 代理选项 | `WithErrorHandler / WithTimeout / WithFlushInterval / WithDirector / WithModifyResponse` |
| 指标 | `Server.Metrics()`（状态码分布、协议维度、路由/分组级聚合、活跃请求/连接、限流/Panic） |
| 路由/分组统计 | `Server.RouteStats()` / `Server.GroupStats()` |
| 优雅关闭 | `Stop(ctx)` / 信号自动关闭 |
| 证书热重载 | `SetCertificateLoader`（默认按文件 mtime 自动重载） |

## 浏览器安全基线

- 已覆盖：`X-Content-Type-Options`、`X-Frame-Options`、`Referrer-Policy`、
  `Strict-Transport-Security`（max-age + includeSubDomains + preload）、
  `Permissions-Policy`、`Cross-Origin-Opener-Policy`、`Cross-Origin-Resource-Policy`、
  `Cross-Origin-Embedder-Policy`、`Content-Security-Policy`（含 Report-Only）、
  `Origin-Agent-Cluster`、CORS Private Network Access；
- 刻意不提供：`X-XSS-Protection` 与 `Expect-CT`（已被现代浏览器废弃）。

## 平台限制

- Windows 上使用 Unix Socket 监听要求 Windows 10 build 1803（10.0.17134）
  或更高版本；低于该版本时 `Start()` 会拒绝初始化并返回
  `WEBX_START_FAILED`。

## License

MIT © [lcylpzls](https://github.com/lcylpzls)
