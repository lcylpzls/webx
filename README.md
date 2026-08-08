# webx

**基于 Go 标准库的工业级 HTTP/HTTPS 服务组件库**，功能与
[ginx](https://github.com/lcylpzls/ginx) 对等，但不再依赖 Gin：
路由、上下文、中间件链全部自研，传输层基于 `net/http`。

[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.26-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

> 当前状态：**核心实现完成**。功能与 ginx 对等，三包语句覆盖率 100%，
> 当前版本：v0.5.0（v0.6.0 迭代中）。

## 技术栈

| 组件 | 用途 | 说明 |
| --- | --- | --- |
| Go 标准库 `net/http` | HTTP/1.1、HTTP/2、路由、生命周期 | 唯一运行时基座 |
| [logx](https://github.com/lcylpzls/logx) | 结构化日志 | 自家库，零第三方依赖 |
| [errx](https://github.com/lcylpzls/errx) | 结构化错误 | 自家库，统一错误码 |
| [confx](https://github.com/lcylpzls/confx) | TOML 配置加载 | 自家库 |
| [quic-go](https://github.com/quic-go/quic-go) | HTTP/3 (QUIC) 传输 | **唯一直接第三方依赖** |

## 设计原则

- **零泄漏**：不导出任何第三方类型，业务 Handler 只接触自研 `*Context`；
- **链式 API**：Builder 风格配置，配置即文档；
- **启动后不可变**：`Start()` 之后修改配置仅记录警告；
- **绝不 panic**：唯一 `recover()` 位于 Recovery 中间件；
- **绝不吞错**：所有错误路径返回 errx 结构化错误并记录日志；
- **简体中文**：日志、打印、注释、文档统一简体中文；
- **工业级门槛**：语句覆盖率 100%、fuzz、race、三平台 CI、API 基线。

## 功能清单（与 ginx 对等）

- 🔒 强制 TLS：HTTP/2（TLS over TCP）+ HTTP/3（QUIC over UDP）+ Unix Socket 多通道同时监听；
- 🔗 链式配置：路由、分组、中间件、限流、静态文件一键组装；
- 🧱 内置中间件：Recovery / RequestID / Timeout / CORS / Validation / RateLimit；
- 📊 标准化响应：`{code, msg, data, requestId, timestamp}`；
- 🩺 健康检查：`/health`，路径可配置；
- 🪵 日志接入 logx；错误接入 errx；配置接入 confx（TOML）；
- 🧹 优雅关闭：SIGINT/SIGTERM 信号捕获 + `Stop(ctx)`；
- 🗂️ 静态文件服务与 SPA 回退（支持 `embed`）。

## 文档索引

- [docs/README.md](docs/README.md) — 文档索引与阅读顺序
- [docs/architecture.md](docs/architecture.md) — 架构设计
- [docs/api-design.md](docs/api-design.md) — API 草案与待确认决策点
- [docs/iteration-plan.md](docs/iteration-plan.md) — 迭代计划与质量门槛
- [docs/decisions.md](docs/decisions.md) — 架构决策记录（ADR）
- [docs/migration.md](docs/migration.md) — ginx 迁移指南

## 性能数据（v0.4.0 基线）

| 基准 | 结果 |
| --- | --- |
| 路由匹配+分发（50 条路由） | 1.5µs/op，6 allocs |
| 中间件链（3 层） | 10.6ns/op，0 allocs |
| 标准化 JSON 响应 | 148ns/op，1 alloc |

## 功能清单

- 多通道监听：HTTP/2（TLS）、HTTP/3（QUIC）、Unix Socket；
- 路由：gin 风格 `:id`/`*filepath` 语法、404/405 JSON、尾斜杠重定向；
- 中间件：Recovery / RequestID / Timeout / CORS / Validation / RateLimit /
  Gzip / Metrics / AccessLog；
- 标准化响应、健康检查、静态文件与 SPA、优雅关闭；
- 配置 confx（TOML）、错误 errx、日志 logx（外部注入）。

## License

MIT © [lcylpzls](https://github.com/lcylpzls)
