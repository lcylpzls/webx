# webx 架构设计

## 1. 定位

webx 是基于 Go 标准库自研的 HTTP/HTTPS 服务组件库，核心设计：

| 维度 | 说明 |
| --- | --- |
| 路由 | 自研 radix 匹配树 |
| Handler 签名 | `func(*webx.Context)`（自研） |
| 上下文/中间件链 | 自研 Context / 自研索引式链 |
| 依赖 | **仅 quic-go**（直接依赖） |
| 日志 | logx（外部注入） |
| 错误 | errx 结构化错误 |
| 配置 | Config + confx TOML 加载 |

## 2. 分层架构

```
┌──────────────────────────────────────────────────────────┐
│ transport 传输层                                           │
│   TCP/TLS(HTTP/1.1+HTTP/2) · QUIC(HTTP/3) · Unix Socket │
└──────────────────────────┬───────────────────────────────┘
                           │
┌──────────────────────────▼───────────────────────────────┐
│ server 服务层                                             │
│   生命周期 · 多通道监听 · 优雅关闭 · 健康检查 · 配置校验    │
└──────────────────────────┬───────────────────────────────┘
                           │
┌──────────────────────────▼───────────────────────────────┐
│ routing 路由层                                            │
│   ServeMux 包装 · :id→{id} 翻译 · 路由组 · 404/405 兜底    │
└──────────────────────────┬───────────────────────────────┘
                           │
┌──────────────────────────▼───────────────────────────────┐
│ context 上下文层                                          │
│   请求上下文 · 中间件链(Next/Abort) · 参数 · 响应 Writer   │
└──────────────────────────┬───────────────────────────────┘
                           │
┌──────────────────────────▼───────────────────────────────┐
│ middleware 中间件层                                       │
│   Recovery · RequestID · Timeout · CORS · Validation ·   │
│   RateLimit（+ 可选 AccessLog）                           │
└──────────────────────────────────────────────────────────┘

支撑层（横向）：logx 适配 · errx 错误码 · confx 配置 · 标准化响应
```

## 3. 核心机制

### 3.1 Context（自研）

每个请求创建一个 `*Context`，持有：

- `http.ResponseWriter`（可被 Timeout 等中间件包装）；
- `*http.Request`；
- 路由参数（`map[string]string`，从 `r.PathValue` 收集）；
- 请求级 KV（`Set/Get`，用于 requestId 等）；
- 中间件链状态：`handlers []HandlerFunc`、`index int`、`aborted bool`。

中间件链采用与 gin 一致的索引模型：

```go
func (c *Context) Next() {
    c.index++
    for c.index < len(c.handlers) {
        c.handlers[c.index](c)
        c.index++
        if c.aborted { return }
    }
}

func (c *Context) Abort() { c.aborted = true }
```

中间件链语义与主流框架一致（Next/Abort），迁移与协作成本低。

### 3.2 路由（自研 radix 匹配树）

webx 对外接受框架风格路由语法：`:id`（单段参数）与 `*filepath`（通配剩余路径），
内部由 radix 匹配树按字面段 > 参数段 > 通配段的优先级匹配与分发。

设计要点：

- 注册时做冲突检测：重复方法、参数名冲突、精确/子树冲突均返回错误；
- `NoRoute`：未命中任何模式时返回自定义 JSON 404；
- `NoMethod`：路径存在但方法不匹配时返回标准化 JSON 405 与 `Allow` 响应头；
- 静态文件与 `/health` 共存无路由冲突问题。

### 3.3 响应信封

统一采用 `StandardizedResponse` 信封：

```json
{ "code": 0, "msg": "ok", "data": {}, "requestId": "...", "timestamp": 1750000000000 }
```

`code` 语义（int）与 `msg` 文案（简体中文）统一约定，
取值策略见 [api-design.md](api-design.md) 的 D-6。

### 3.4 传输与生命周期

- **HTTP/2**：标准库 `http.Server` + `tls.Config{NextProtos: ["h2","http/1.1"]}`；
- **HTTP/3**：quic-go 的 `http3.Server`，UDP 监听，与 TCP 同端口不冲突；
- **Unix Socket**：`net.Listen("unix", path)`，含残留文件清理与权限设置；
  Windows 要求 build 1803（10.0.17134）以上（AF_UNIX 双工通信），
  `Start()` 会先做版本检测，不达标拒绝初始化；
- **优雅关闭**：信号捕获（SIGINT/SIGTERM）+ `Stop(ctx)`，逐一 Shutdown
  所有 `http.Server` 与 `http3.Server`，最后执行清理函数；
- **启动失败回滚**：任一通道监听失败时，关闭已创建的监听器并返回 errx 错误。

### 3.5 配置（confx 集成）

`Config` 结构体同时支持两种用法：

```go
// 方式一：代码直接构造
srv := webx.NewServer(webx.Config{ ... })

// 方式二：TOML 文件加载（confx）
cfg, err := webx.LoadConfig("config.toml") // 内部 confx.Parse + Config.Validate()
srv := webx.NewServer(cfg)
```

`Config` 字段带 `toml` tag，样例见 [api-design.md](api-design.md)。

### 3.6 日志与错误

- 日志：`logx.Logger` 由调用方在 `NewServer(cfg, logger)` 时注入，
  webx 内部只使用、不创建日志器；
- 错误：所有失败路径返回 `errx` 结构化错误，错误码启动期注册
  （`WEBX_*`），见 [api-design.md](api-design.md) 的错误码表。

## 4. 并发与安全设计

- `Server` 配置字段在 `Start()` 后不可变，变更方法仅记录 Warn 日志；
- 配置修改与读取使用 `sync.Mutex`；关闭使用 `sync.Once`；
- 优雅关闭会依次排空 HTTP/2/Unix 连接、关闭 QUIC 监听器并调用
  `http3.Server.Close()` 终止活动 HTTP/3 连接，避免 goroutine 泄漏；
- 启动失败会回收限流清理 goroutine 与信号监听注册；
- Timeout 中间件通过包装 Writer 丢弃超时写入，不引入额外 goroutine，
  避免 Context 并发读写；
- RateLimiter 使用互斥锁 + 令牌桶，定期清理过期桶；
- 所有响应头/响应体写入均在 handler goroutine 内完成。

> 注意：不要在请求 Handler 内调用 `Stop()`——`http.Server.Shutdown`
> 会等待当前连接变为空闲，Handler 自身阻塞将导致关闭等待直到超时。

> 注意：`Stop()` 后同一 Server 实例不能再次 `Start()`（需新建实例）；
> `logger` 必须在 `NewServer` 时注入，`WithLogger(nil)` 会让 `Start()` 失败。

## 5. 设计取舍

- 保持标准库兼容：所有能力基于 `net/http`，可嵌入任意 HTTP 服务；
- 路由性能与表达能力兼顾：radix 树 + 参数/通配，注册期冲突检测；
- 业务代码只接触 `*webx.Context`，不依赖任何第三方框架类型。
