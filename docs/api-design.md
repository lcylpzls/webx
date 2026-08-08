# webx API 设计草案

> 本文是评审稿。所有签名在实现前都可能调整；
> **第 6 节的"待决策点"需要你确认后冻结 API。**

## 1. 包结构

```
webx/
├── config.go        # Config 结构体 + Validate() + LoadConfig()
├── server.go        # Server 生命周期与链式 API
├── context.go       # Context 与中间件链
├── router.go        # ServeMux 包装 + 路由语法翻译
├── route.go         # Route / RouteGroup / HandlerFunc
├── response.go      # StandardizedResponse 与状态码
├── errors.go        # errx 错误码注册
├── listener.go      # TLS / QUIC / Unix 监听器工厂
├── graceful.go      # 信号捕获与优雅关闭
├── health.go        # /health 处理器
├── static.go        # 静态文件与 SPA
├── logadapter.go    # logx 适配（默认 logger 与字段转换）
├── middleware/
│   ├── recovery.go
│   ├── requestid.go
│   ├── timeout.go
│   ├── cors.go
│   ├── validation.go
│   ├── ratelimit.go
│   └── manager.go
└── examples/
```

## 2. 核心类型

### 2.1 HandlerFunc 与 Context

```go
// HandlerFunc 是 webx 的业务处理器签名，不依赖任何第三方类型。
type HandlerFunc func(*Context)

type Context struct {
    // 未导出字段：writer、request、params、values、handlers、index、aborted 等
}

// --- 请求 ---
func (c *Context) Request() *http.Request
func (c *Context) Writer() http.ResponseWriter
func (c *Context) Param(key string) string      // 路由参数
func (c *Context) Query(key string) string
func (c *Context) GetHeader(key string) string
func (c *Context) RemoteIP() string

// --- 请求级 KV ---
func (c *Context) Set(key string, val any)
func (c *Context) Get(key string) (any, bool)
func (c *Context) GetString(key string) string
func (c *Context) RequestID() string            // requestId 快捷读取

// --- 响应 ---
func (c *Context) Status(code int)
func (c *Context) Header(key, value string)
func (c *Context) JSON(code int, data any) error
func (c *Context) String(code int, format string, args ...any) error
func (c *Context) JSONResponse(httpStatus int, msg string, data any)  // 标准信封
func (c *Context) Success(msg string, data any)
func (c *Context) Fail(httpStatus, code int, msg string)

// --- 绑定 ---
func (c *Context) BindJSON(out any) error

// --- 中间件链 ---
func (c *Context) Next()
func (c *Context) Abort()
func (c *Context) AbortWithStatusJSON(httpStatus int, msg string, data any)
func (c *Context) IsAborted() bool
```

### 2.2 Server 链式 API（与 ginx 对齐）

```go
func NewServer(cfg Config, logger logx.Logger) *Server
func (s *Server) WithLogger(l logx.Logger) *Server
func (s *Server) UseHttp2Listen(addr string) *Server
func (s *Server) UseHttp3Listen(addr string) *Server
func (s *Server) UseUnixSocketListen(path string, perm os.FileMode) *Server
func (s *Server) RegisterRoute(r Route) *Server
func (s *Server) RegisterRoutes(routes []Route) *Server
func (s *Server) RegisterRouteGroup(prefix string, fn func(*RouteGroup)) *Server
func (s *Server) UseGlobalMiddleware(mw ...HandlerFunc) *Server
func (s *Server) OverrideMiddleware(mt MiddlewareType, mw HandlerFunc) *Server
func (s *Server) DisableMiddleware(mt ...MiddlewareType) *Server
func (s *Server) EnableMiddleware(mt ...MiddlewareType) *Server
func (s *Server) EnableRateLimit(opts RateLimitOptions) *Server
func (s *Server) DisableRateLimit() *Server
func (s *Server) ServeStaticDir(prefix, root string) *Server
func (s *Server) ServeStaticFS(prefix string, fs http.FileSystem) *Server
func (s *Server) EnableSPA(fs http.FileSystem, indexPath string) *Server
func (s *Server) ListenerAddr() string
func (s *Server) Start() error
func (s *Server) Stop(ctx context.Context) error
```

### 2.3 Route 与 RouteGroup

```go
type Route struct {
    Method     string        // GET/POST/PUT/DELETE/PATCH/HEAD/OPTIONS
    Path       string        // 支持 "/api/users/:id" 与 "/assets/*filepath"
    Handler    HandlerFunc
    Middleware []HandlerFunc
}

type RouteGroup struct { /* 未导出字段 */ }

func (rg *RouteGroup) GET(path string, h HandlerFunc, mw ...HandlerFunc)
func (rg *RouteGroup) POST(path string, h HandlerFunc, mw ...HandlerFunc)
func (rg *RouteGroup) PUT(path string, h HandlerFunc, mw ...HandlerFunc)
func (rg *RouteGroup) DELETE(path string, h HandlerFunc, mw ...HandlerFunc)
func (rg *RouteGroup) PATCH(path string, h HandlerFunc, mw ...HandlerFunc)
func (rg *RouteGroup) Use(mw ...HandlerFunc)
func (rg *RouteGroup) Group(relativePath string) *RouteGroup
```

### 2.4 Config（TOML 可加载）

```go
type Config struct {
    TLSCertFile string        `toml:"tls_cert_file"`
    TLSKeyFile  string        `toml:"tls_key_file"`
    ReadTimeout time.Duration `toml:"read_timeout"`
    // ... 其余字段与 ginx 对齐，全部带 toml tag
}

func (c *Config) Validate() error
func LoadConfig(path string) (Config, error)   // confx 加载 + Validate
```

## 3. 标准化响应

```go
type StandardizedResponse struct {
    Code      int    `json:"code"`
    Msg       string `json:"msg"`
    Data      any    `json:"data,omitempty"`
    RequestID string `json:"requestId"`
    Timestamp int64  `json:"timestamp"`
}

const (
    CodeSuccess           = 0
    CodeBadRequest        = 400
    CodeNotFound          = 404
    CodeMethodNotAllowed  = 405
    CodeTooManyRequests   = 429
    CodeInternalError     = 500
    CodeServiceUnavailable = 503
)
```

## 4. 内置中间件

与 ginx 完全对齐，顺序固定：

`Recovery → RequestID → Timeout → CORS → Validation → RateLimit`

每个中间件提供 `middleware.XXX() webx.HandlerFunc` 工厂，
由 `middleware.Manager` 统一管理启用/禁用/覆盖。

## 5. errx 错误码表（草案）

| 错误码 | Kind | 场景 |
| --- | --- | --- |
| `WEBX_CONFIG_INVALID` | KindInvalid | Config.Validate 失败 |
| `WEBX_CONFIG_LOAD_FAILED` | KindUnavailable | confx 加载失败 |
| `WEBX_LISTEN_FAILED` | KindUnavailable | 监听器创建失败 |
| `WEBX_START_FAILED` | KindUnavailable | 启动阶段失败 |
| `WEBX_SHUTDOWN_FAILED` | KindUnavailable | 优雅关闭失败 |
| `WEBX_PANIC` | KindInternal | Recovery 捕获 panic |

错误码在 `init()` 中通过 `errx.RegisterCode` 注册，可用 `errx.Markdown()` 生成文档。

## 6. 待决策点（请确认）

| 编号 | 问题 | 方案 A（推荐） | 方案 B |
| --- | --- | --- | --- |
| D-1 | Handler 签名 | `func(*webx.Context)` | 保留 `*gin.Context`（需依赖 gin，否决） |
| D-2 | 路由写法 | 对外兼容 `:id`/`*filepath`，内部翻译为 `{id}`/`{path...}` | 对外直接要求 `{id}` |
| D-3 | HTTP/3 默认 | 默认关闭，`UseHttp3Listen` 显式开启 | 只要配置了证书就自动开启 |
| D-4 | 配置来源 | Config 直接构造 + `LoadConfig` TOML 都支持 | 仅 confx TOML |
| D-5 | 日志抽象 | 直接用 logx.Logger（不另设接口） | 保留 ginx 式 Logger 接口 |
| D-6 | 响应 code | 沿用 int（0=成功，HTTP 语义错误码），与 ginx 兼容 | 改为 errx.Code 字符串（更语义化，但破坏兼容） |
| D-7 | AccessLog | 新增可选的请求日志中间件（ginx 没有） | 不新增，保持与 ginx 完全一致 |

> 我的建议：D-1/D-2/D-3/D-4/D-5/D-6 全部选方案 A；D-7 建议选 A
> （服务级访问日志是生产刚需，logx 已具备完整能力）。
