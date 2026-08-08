# webx 架构设计

## 1. 定位

webx 是 [ginx](https://github.com/lcylpzls/ginx) 的标准库后继：功能对等，
但移除 Gin 依赖。核心差异：

| 维度 | ginx | webx |
| --- | --- | --- |
| 路由 | Gin radix tree | 标准库 `http.ServeMux`（Go 1.22+ pattern）+ 语法翻译层 |
| Handler 签名 | `func(*gin.Context)` | `func(*webx.Context)`（自研） |
| 上下文/中间件链 | gin.Context / gin 链 | 自研 Context / 自研索引式链 |
| 依赖 | gin + uuid + quic-go（间接约 30 个模块） | **仅 quic-go**（直接依赖） |
| 日志 | 自定义 Logger 接口 | logx |
| 错误 | 普通 error + int code | errx 结构化错误 |
| 配置 | 纯 Config 结构体 | Config 结构体 + confx TOML 加载 |

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

这样 ginx 现有的中间件迁移成本最低，语义完全一致。

### 3.2 路由（标准库 ServeMux）

Go 1.22+ 的 `http.ServeMux` 已支持方法 + 路径参数：

| ginx/gin 语法 | ServeMux 语法 |
| --- | --- |
| `/api/users/:id` | `/api/users/{id}` |
| `/*filepath` | `/{filepath...}` |

webx 在内部做语法翻译，**对外仍接受 ginx 风格的 `:id` 与 `*filepath`**，
降低迁移成本。

设计要点：

- 路由注册时统一翻译并交给 `http.ServeMux`，冲突检测由标准库负责；
- `NoRoute`：ServeMux 未命中时返回自定义 JSON 404（与 ginx 一致）；
- `NoMethod`：ServeMux 对"路径存在但方法不匹配"返回 405，webx 包一层
  输出标准化 JSON 405 与 `Allow` 响应头；
- 静态文件与 `/health` 共存不再有 ginx 的 radix tree 冲突问题，
  可删除 ginx 中"根路径静态服务跳过 health"的 workaround。

### 3.3 响应信封

沿用 ginx 的 `StandardizedResponse`：

```json
{ "code": 0, "msg": "ok", "data": {}, "requestId": "...", "timestamp": 1750000000000 }
```

`code` 语义（int）与 `msg` 文案（简体中文）与 ginx 保持一致，
具体取值策略见 [api-design.md](api-design.md) 的待决策点 D-6。

### 3.4 传输与生命周期

- **HTTP/2**：标准库 `http.Server` + `tls.Config{NextProtos: ["h2","http/1.1"]}`；
- **HTTP/3**：quic-go 的 `http3.Server`，UDP 监听，与 TCP 同端口不冲突；
- **Unix Socket**：`net.Listen("unix", path)`，含残留文件清理与权限设置；
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

- 日志：内部默认使用 logx（控制台输出，级别可配置），也接受调用方注入
  `logx.Logger`（`WithLogger`）；
- 错误：所有失败路径返回 `errx` 结构化错误，错误码启动期注册
  （`WEBX_*`），见 [api-design.md](api-design.md) 的错误码表。

## 4. 并发与安全设计

- `Server` 配置字段在 `Start()` 后不可变，变更方法仅记录 Warn 日志；
- 配置修改与读取使用 `sync.Mutex`；关闭使用 `sync.Once`；
- Timeout 中间件通过包装 Writer 丢弃超时写入，不引入额外 goroutine，
  避免 Context 并发读写；
- RateLimiter 使用互斥锁 + 令牌桶，定期清理过期桶；
- 所有响应头/响应体写入均在 handler goroutine 内完成。

## 5. 与 ginx 的迁移关系

- ginx 保持现状继续维护，webx 是新项目、新模块，不共享历史包袱；
- 业务迁移成本集中在 Handler 签名（`*gin.Context` → `*webx.Context`）
  与路由语法（webx 保留 `:id` 写法，差异最小化）；
- 标准化响应体、中间件顺序、配置字段尽量对齐，方便逐服务平滑迁移。
