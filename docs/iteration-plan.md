# webx 迭代计划与质量门槛

## 1. 迭代阶段

### P0 项目骨架

- `go.mod`（module github.com/lcylpzls/webx，go 1.26）；
- 目录结构、`.gitignore`、CI 工作流（三平台矩阵 + staticcheck + fuzz）；
- 依赖引入：logx、errx、confx、quic-go。

**验收**：空包测试通过，CI 全绿。

### P1 基础支撑

- `errors.go`：WEBX_* 错误码注册（errx）；
- `logadapter.go`：logx 适配与默认 logger；
- `config.go`：Config 结构体（toml tag）、`Validate()`、`LoadConfig()`（confx）。

**验收**：100% 覆盖率，配置校验全路径测试。

### P2 上下文与中间件链

- `context.go`：Context 全部方法面；
- 中间件链 Next/Abort 语义；
- `response.go`：标准化响应信封。

**验收**：链式语义专项测试（嵌套中间件、Abort 提前终止、超时 Writer）。

### P3 路由

- `router.go`：ServeMux 包装、`:id`/`*filepath` 语法翻译；
- `route.go`：Route / RouteGroup（含嵌套分组与分组中间件）；
- 404/405 标准化 JSON 兜底。

**验收**：路由优先级、冲突、参数提取、405/404 全路径测试 + FuzzRoute。

### P4 传输与生命周期

- `listener.go`：TLS（HTTP/2）、QUIC（HTTP/3）、Unix Socket；
- `server.go`：Start/Stop、信号捕获、优雅关闭、启动失败回滚；
- `health.go`：/health。

**验收**：集成测试（真实监听 + 请求），race 全绿。

### P5 内置中间件

- Recovery / RequestID / Timeout / CORS / Validation / RateLimit；
- `middleware.Manager` 的启用/禁用/覆盖。

**验收**：每个中间件独立测试 + 组合测试，100% 覆盖率。

### P6 静态文件与示例

- `static.go`：静态目录 / embed / SPA 回退；
- `examples/`：basic、fluent、custom-middleware、spa；
- README/文档收尾。

**验收**：示例可运行，文档与代码一致。

### P7 发布 v0.1.0

- CHANGELOG 定版、API 基线（docs/api-v0.1.0.md）、tag、Release 工作流；
- 与 ginx 功能对照表核对。

## 2. 质量门槛（每个阶段强制）

- 语句覆盖率 **100%**（`go test -cover`）；
- `go vet ./...`、`staticcheck ./...` 零告警；
- Linux `-race` 全绿；
- fuzz：路由翻译、JSON 绑定、限流至少各 1 个目标（CI 10s 短跑）；
- 三平台 CI：ubuntu / windows / macos × Go 1.26.x；
- v0.1.0 起维护 API 基线，CI 增加 apidiff 检查；
- 所有日志、注释、文档使用简体中文。

## 3. 依赖策略

- **直接依赖仅 quic-go**（HTTP/3 必需）；
- logx / errx / confx 为自家库；
- 禁止为小功能引入第三方（如 UUID 自研、CORS 自研、限流自研）；
- confx 内部使用 BurntSushi/toml 解析，webx 不直接感知。
