# webx 架构决策记录（ADR）

## ADR-001 路由：标准库 ServeMux 而非自研 radix tree

- **状态**：已定
- **背景**：gin 的路由树强大但复杂度高，自研成本大。
- **决策**：使用 Go 1.22+ 的 `http.ServeMux`，内部做 `:id → {id}`、
  `*filepath → {path...}` 语法翻译；404/405 由 webx 包一层输出 JSON。
- **后果**：路由能力受限于 ServeMux 模式（不支持自定义优先级），
  但对常规业务完全够用；静态/健康路由共存更简单。

## ADR-002 Context 自研，零类型泄漏

- **状态**：已定
- **决策**：业务 Handler 签名 `func(*webx.Context)`，不导出任何第三方类型；
  中间件链语义（Next/Abort）与 gin 对齐，降低迁移成本。
- **后果**：与 ginx 的 Handler 签名不兼容，属于新库而非 ginx v2。

## ADR-003 依赖策略：仅 quic-go

- **状态**：已定
- **决策**：直接依赖仅 quic-go（HTTP/3 是刚需）；UUID、CORS、限流等
  一律自研；日志/错误/配置使用自家 logx/errx/confx。
- **后果**：依赖面最小化，但 HTTP/3 升级节奏受 quic-go 上游影响。

## ADR-004 配置：Config 结构体 + confx TOML

- **状态**：已定
- **决策**：Config 带 toml tag，支持代码直接构造与 `LoadConfig(path)`
  两种方式；`Validate()` 是所有构造路径的统一校验入口。
- **后果**：新增配置项时必须同步维护 toml tag 与 Validate。

## ADR-005 日志：直接使用 logx

- **状态**：已定
- **决策**：Server 内部直接使用 `logx.Logger`，不另设 Logger 接口；
  调用方可用 `WithLogger` 注入自定义 logx.Logger。
- **后果**：webx 与 logx 版本绑定，但避免 ginx 中接口空转的问题。

## ADR-006 中间件语义与 gin 对齐

- **状态**：已定
- **决策**：Next/Abort/AbortWithStatusJSON 语义逐一对齐 gin，
  内置中间件顺序固定：Recovery → RequestID → Timeout → CORS →
  Validation → RateLimit。
- **后果**：从 ginx 迁移中间件的心智成本最低。

## ADR-007 质量门槛：100% 覆盖 + CI 全绿才可发版

- **状态**：已定
- **决策**：每个迭代阶段强制 100% 语句覆盖、vet/staticcheck 零告警、
  race 全绿、fuzz 短跑；v0.1.0 起维护 API 基线。
- **后果**：迭代速度略慢，但每个版本都可放心作为生产依赖。
