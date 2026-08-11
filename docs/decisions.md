# webx 架构决策记录（ADR）

## ADR-001 路由：自研 radix 匹配树

- **状态**：已定
- **背景**：v0.17 前使用 `http.ServeMux`，性能和模式能力受限。
- **决策**：自研 radix 匹配树，`:id → {id}`、`*filepath → {path...}`
  语法翻译；404/405 由 webx 统一输出 JSON。
- **后果**：500 条路由匹配约 0.22µs/op，匹配与分发完全自主。

## ADR-002 Context 自研，零类型泄漏

- **状态**：已定
- **决策**：业务 Handler 签名 `func(*webx.Context)`，不导出任何第三方类型；
  中间件链语义（Next/Abort）与 gin 对齐，降低迁移成本。
- **后果**：业务 Handler 使用自研 Context，不依赖任何第三方框架类型。

## ADR-003 依赖策略：quic-go + google/uuid

- **状态**：已定
- **决策**：直接依赖 quic-go（HTTP/3 是刚需）与 google/uuid（UUID v7
  标准实现，`uuid.Must` 稳定生成）；CORS、限流、路由等一律自研；
  日志/错误/配置使用自家 logx/errx/confx。
- **后果**：第三方依赖面控制在 2 个，升级节奏受上游影响。

## ADR-004 配置：Config 结构体 + confx TOML

- **状态**：已定
- **决策**：Config 带 toml tag，支持代码直接构造与 `LoadConfig(path)`
  两种方式；`Validate()` 是所有构造路径的统一校验入口。
- **后果**：新增配置项时必须同步维护 toml tag 与 Validate。

## ADR-005 日志：直接使用 logx

- **状态**：已定
- **决策**：Server 内部直接使用 `logx.Logger`，不另设 Logger 接口；
  调用方可用 `WithLogger` 注入自定义 logx.Logger。
- **后果**：webx 与 logx 版本绑定，减少接口间接层。

## ADR-006 中间件语义与 gin 对齐、顺序可配置

- **状态**：已定
- **决策**：Next/Abort/AbortWithStatusJSON 语义逐一对齐 gin，
  默认顺序 Recovery → RequestID → BodyLimit → ConcurrencyLimit →
  Timeout → CORS → Validation → Security → Gzip → RateLimit →
  Metrics → AccessLog，且可通过 `SetMiddlewareOrder` 调整。
- **后果**：迁移心智成本低，同时保留灵活定制能力。

> 补充（v1.6.0）：全局中间件与内置中间件统一为标准库形态
> `func(http.Handler) http.Handler`；路由/分组中间件保留 core 链
> 的 Next/Abort 语义，标准中间件通过 request context 透传
> `*Context`（`core.From / RouteFrom / GroupFrom`）。

## ADR-007 质量门槛：100% 覆盖 + CI 全绿才可发版

- **状态**：已定
- **决策**：每个迭代阶段强制 100% 语句覆盖、vet/staticcheck 零告警、
  race 全绿、fuzz 短跑；v0.20.0 起维护 API 基线，v1.0.0 起冻结。
- **后果**：迭代速度略慢，但每个版本都可放心作为生产依赖。

## ADR-008 v1.0.0 定版

- **状态**：已定
- **背景**：v0.1.0 ~ v0.32.0 迭代完成，功能面覆盖路由、中间件、安全基线、
  可观测性、绑定/上传、代理与生命周期。
- **决策**：发布 v1.0.0，公开 API 冻结；apidiff 基线升级至 v1.0.0；
  API 终审记录见 [api-design.md](api-design.md)。
- **后果**：v1 之后破坏性变更需走主版本升级，0.x 时代的快速迭代节奏结束。

> 补充（家族统一约定）：自后续批次起，破坏性变更统一走 minor 版本
> （不强制主版本升级），apidiff 门禁不再作为发布条件；
> 具体变更以各库 CHANGELOG 为准。

## ADR-009 性能路线：零分配核心 + 传输层保持现状

- **状态**：已定
- **背景**：横向基准显示 webx 路由分发分配偏高（参数 map 占 ~99.6%），
  端到端 TLS 与 gin/echo 持平但落后 fasthttp 约 35%。
- **决策**：
  - P0：保持 net/http 与 quic-go 传输，零分配改造自研热路径
    （路由参数槽化、手写标准化响应信封、Content-Type 免规范化分配）；
  - P1：不自研 HTTP/1.1 解析层（自用场景主链路为 HTTP/2/3，
    收益低且自研 HTTP 解析安全风险高）；
  - P2：HTTP/2 / HTTP/3 不自研，沿用 net/http 与 quic-go http3，
    仅做连接级缓冲适配与上游版本跟进；
  - QUIC 传输层继续使用 quic-go，不自行实现 QUIC。
- **后果**：路由分发与标准化响应达到 0 allocs/op，端到端性能保持
  与 gin/echo 持平并持续向 fasthttp 靠拢；CI 增加分配数门禁防回退。

## ADR-010 指标统一外置（metricsx）

- **状态**：已定
- **背景**：家族统一可观测性要求 metrics 只由一个底座（metricsx）
  采集，webx 不再内置 Prometheus 文本端点与 /metrics 路由。
- **决策**：
  - 删除 `EnableMetricsEndpoint`、`metrics_enabled` / `metrics_path`
    配置与文本渲染；快照 API（`Metrics()` / `RouteStats()` /
    `GroupStats()`）保留，仅作为运行状态查看；
  - 新增 `WithMetrics(webx.Metrics)` 外置转发：请求/耗时/panic/
    限流/并发拒绝事件，活跃请求与连接水位走可选 `GaugeMetrics`；
  - 暴露路由由调用方自行实现（示例使用 promhttp）。
- **后果**：Prometheus 采集链路唯一化，webx 保持零指标采集依赖；
  迁移只需删除配置字段并替换一行 API。

> 落地说明：该决策最终在 v1.5.x 系列以无后缀模块路径落地
> （v2 实验线未保留），详见 CHANGELOG。
