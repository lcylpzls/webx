<!-- v0.3.0 API 基线；生成方式：go doc -all . / ./middleware -->

## 包 webx

package webx // import "github.com/lcylpzls/webx"

Package webx 提供基于 Go 标准库的工业级 HTTP/HTTPS 服务组件库。 路由基于
http.ServeMux，上下文与中间件链自研，日志/错误/配置 分别接入 logx / errx / confx，HTTP/3 使用 quic-go。

CONSTANTS

const (
	CodeSuccess            = core.CodeSuccess
	CodeBadRequest         = core.CodeBadRequest
	CodeNotFound           = core.CodeNotFound
	CodeMethodNotAllowed   = core.CodeMethodNotAllowed
	CodeTooManyRequests    = core.CodeTooManyRequests
	CodeInternalError      = core.CodeInternalError
	CodeServiceUnavailable = core.CodeServiceUnavailable
)
    标准化响应业务码，与 ginx 保持一致。

const (
	// CodeConfigInvalid 配置校验失败。
	CodeConfigInvalid errx.Code = "WEBX_CONFIG_INVALID"
	// CodeConfigLoadFailed 配置文件加载失败。
	CodeConfigLoadFailed errx.Code = "WEBX_CONFIG_LOAD_FAILED"
	// CodeListenFailed 监听器创建失败。
	CodeListenFailed errx.Code = "WEBX_LISTEN_FAILED"
	// CodeStartFailed 服务启动失败。
	CodeStartFailed errx.Code = "WEBX_START_FAILED"
	// CodeShutdownFailed 优雅关闭失败。
	CodeShutdownFailed errx.Code = "WEBX_SHUTDOWN_FAILED"
	// CodePanic 请求处理发生 panic（Recovery 中间件捕获）。
	CodePanic errx.Code = "WEBX_PANIC"
)
    webx 错误码：统一使用 errx 结构化错误。


FUNCTIONS

func GracefulShutdown(
	ctx context.Context,
	logger logx.Logger,
	httpServer *http.Server,
	listener net.Listener,
	shutdownTimeout time.Duration,
	unixSocketPath string,
	cleanupFuncs []func(),
) error
    GracefulShutdown 监听系统信号并执行优雅关闭（公开 API，兼容 ginx 用法）。 收到 SIGINT/SIGTERM 后调用
    httpServer.Shutdown 排空请求。


TYPES

type Config struct {
	// TLSCertFile TLS 证书文件路径（PEM 格式），必填。
	TLSCertFile string `toml:"tls_cert_file"`
	// TLSKeyFile TLS 私钥文件路径（PEM 格式），必填。
	TLSKeyFile string `toml:"tls_key_file"`

	// ReadTimeout HTTP 读取超时时间。
	ReadTimeout time.Duration `toml:"read_timeout"`
	// WriteTimeout HTTP 写入超时时间。
	WriteTimeout time.Duration `toml:"write_timeout"`
	// IdleTimeout HTTP 空闲连接超时时间。
	IdleTimeout time.Duration `toml:"idle_timeout"`
	// RequestTimeout 单个请求的超时时间，由 Timeout 中间件使用。
	RequestTimeout time.Duration `toml:"request_timeout"`
	// ShutdownTimeout 优雅关闭的最大等待时间。
	ShutdownTimeout time.Duration `toml:"shutdown_timeout"`
	// MaxHeaderBytes 请求头的最大字节数。
	MaxHeaderBytes int `toml:"max_header_bytes"`
	// MaxBodyBytes BindJSON 的最大请求体字节数，0 表示默认 10MB。
	MaxBodyBytes int64 `toml:"max_body_bytes"`

	// HealthPath 健康检查端点路径，默认为 "/health"。
	HealthPath string `toml:"health_path"`

	// LogLevel 日志级别，可选 debug、info、warn、error，为空默认 info。
	LogLevel string `toml:"log_level"`
	// AccessLogEnabled 是否启用访问日志中间件。
	AccessLogEnabled bool `toml:"access_log_enabled"`
	// LogSuccessReq 访问日志是否记录成功请求（默认仅记录非 2xx）。
	LogSuccessReq bool `toml:"log_success_req"`

	// CORSAllowedOrigins CORS 允许的来源列表，为空使用默认值。
	CORSAllowedOrigins []string `toml:"cors_allowed_origins"`
	// CORSAllowedMethods CORS 允许的 HTTP 方法列表。
	CORSAllowedMethods []string `toml:"cors_allowed_methods"`
	// CORSAllowedHeaders CORS 允许的请求头列表。
	CORSAllowedHeaders []string `toml:"cors_allowed_headers"`
	// CORSMaxAge CORS 预检请求的缓存时间。
	CORSMaxAge time.Duration `toml:"cors_max_age"`

	// MiddlewareRequestID 是否启用 RequestID 中间件。
	MiddlewareRequestID bool `toml:"middleware_request_id"`
	// MiddlewareCORS 是否启用 CORS 中间件。
	MiddlewareCORS bool `toml:"middleware_cors"`
	// MiddlewareTimeout 是否启用 Timeout 中间件。
	MiddlewareTimeout bool `toml:"middleware_timeout"`
	// MiddlewareRecovery 是否启用 Recovery 中间件。
	MiddlewareRecovery bool `toml:"middleware_recovery"`
	// MiddlewareValidation 是否启用 Validation 中间件。
	MiddlewareValidation bool `toml:"middleware_validation"`
}
    Config 定义 webx Server 的全部配置项，通过 confx 从 TOML 文件加载。 所有校验在 Validate()
    中集中进行，失败返回 errx 结构化错误。

func LoadConfig(path string) (Config, error)
    LoadConfig 通过 confx 从 TOML 文件加载配置并校验。 文件不存在、TOML 非法、存在未声明字段或校验失败时返回 errx 错误。

func (c *Config) Validate() error
    Validate 校验配置完整性并填充默认值。 校验规则与 ginx 对齐：证书/私钥必填且可配对、超时非负、日志级别合法。

type Context = core.Context
    Context 是单个请求的上下文。

type HandlerFunc = core.HandlerFunc
    HandlerFunc 是 webx 的业务处理器签名，不依赖任何第三方类型。

type MiddlewareType string
    MiddlewareType 标识内置中间件的类型。

const (
	// MiddlewareRequestID 请求 ID 生成中间件。
	MiddlewareRequestID MiddlewareType = "request_id"
	// MiddlewareCORS 跨域处理中间件。
	MiddlewareCORS MiddlewareType = "cors"
	// MiddlewareTimeout 请求超时中间件。
	MiddlewareTimeout MiddlewareType = "timeout"
	// MiddlewareRecovery Panic 捕获中间件。
	MiddlewareRecovery MiddlewareType = "recovery"
	// MiddlewareValidation 请求参数校验中间件。
	MiddlewareValidation MiddlewareType = "validation"
	// MiddlewareRateLimit IP 令牌桶限流中间件。
	MiddlewareRateLimit MiddlewareType = "rate_limit"
	// MiddlewareAccessLog 访问日志中间件。
	MiddlewareAccessLog MiddlewareType = "access_log"
)
type RateLimitOptions struct {
	// QPS 每 IP 每秒允许的请求数（必填，> 0）。
	QPS int
	// Window 限流窗口时长（必填，> 0）。
	Window time.Duration
	// Whitelist 白名单 IP/CIDR 列表（可选）。
	Whitelist []string
	// CleanupInterval 过期桶清理间隔（可选，0 = 默认 5 分钟）。
	CleanupInterval time.Duration
}
    RateLimitOptions 定义 IP 限流中间件的配置参数。

type Route struct {
	// Method HTTP 方法，如 GET、POST、PUT、DELETE、PATCH。
	Method string
	// Path 路由路径，支持 gin 风格 "/api/users/:id" 与 "/assets/*filepath"。
	Path string
	// Handler 路由处理器。
	Handler HandlerFunc
	// Middleware 路由专属中间件（可选），仅对当前路由生效。
	Middleware []HandlerFunc
}
    Route 定义一条 HTTP 路由。

type RouteGroup struct {
	// Has unexported fields.
}
    RouteGroup 路由分组，支持嵌套分组和分组级中间件。 仅缓冲注册，Start() 时一次性挂载。

func (rg *RouteGroup) DELETE(path string, handler HandlerFunc, mw ...HandlerFunc)
    DELETE 注册一条 DELETE 方法路由。

func (rg *RouteGroup) GET(path string, handler HandlerFunc, mw ...HandlerFunc)
    GET 注册一条 GET 方法路由。

func (rg *RouteGroup) Group(relativePath string) *RouteGroup
    Group 创建子分组，继承父分组 prefix 与中间件。

func (rg *RouteGroup) PATCH(path string, handler HandlerFunc, mw ...HandlerFunc)
    PATCH 注册一条 PATCH 方法路由。

func (rg *RouteGroup) POST(path string, handler HandlerFunc, mw ...HandlerFunc)
    POST 注册一条 POST 方法路由。

func (rg *RouteGroup) PUT(path string, handler HandlerFunc, mw ...HandlerFunc)
    PUT 注册一条 PUT 方法路由。

func (rg *RouteGroup) Use(middleware ...HandlerFunc)
    Use 向当前分组追加中间件，影响该分组内所有已注册和后续注册的路由。

type Router struct {
	// Has unexported fields.
}
    Router 基于标准库 http.ServeMux 实现路由： 负责 gin 风格语法（:id / *filepath）到 ServeMux
    模式（{id} / {path...}）的翻译， 以及 404/405 标准化 JSON 响应。路径匹配由内置轻量匹配器完成， 实际分发交给
    ServeMux（保留其冲突检测与 PathValue 能力）。

func NewRouter(noRoute, noMethod core.HandlerFunc) *Router
    NewRouter 创建路由，并指定 404/405 兜底处理器。

func (rt *Router) Handle(method, path string, chain []core.HandlerFunc) error
    Handle 注册一条路由（chain 为全局中间件 + 路由中间件 + 最终处理器的完整链）。 路径冲突或语法非法时返回错误。

func (rt *Router) HandleStatic(prefix string, fs http.FileSystem) error
    HandleStatic 注册静态文件服务（支持子树路径）。 使用无方法模式注册，避免 ServeMux 中 GET 隐式匹配 HEAD 导致 "静态根
    + 具体 GET 路由" 的冲突；方法判定由匹配器负责。

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request)
    ServeHTTP 实现 http.Handler：先做 404/405 判定，再交给 ServeMux 分发。

func (rt *Router) SetMaxBodyBytes(n int64)
    SetMaxBodyBytes 设置路由处理链中 BindJSON 的最大请求体字节数。

type Server struct {
	// Has unexported fields.
}
    Server 是 webx 的核心类型，提供多通道 HTTPS 服务能力。 通过链式 API 配置，Start() 启动，Stop(ctx) 优雅关闭。

func NewServer(cfg Config, logger logx.Logger) *Server
    NewServer 创建 webx Server 实例。 logger 由调用方注入（logx.Logger），webx 内部只使用、不创建日志器；
    logger 为 nil 时 Start() 会返回错误。

func (s *Server) DisableMiddleware(mt ...MiddlewareType) *Server
    DisableMiddleware 禁用指定类型的内置中间件。

func (s *Server) DisableRateLimit() *Server
    DisableRateLimit 禁用 IP 限流中间件。

func (s *Server) EnableMiddleware(mt ...MiddlewareType) *Server
    EnableMiddleware 重新启用指定类型的内置中间件（RateLimit 除外）。

func (s *Server) EnableRateLimit(opts RateLimitOptions) *Server
    EnableRateLimit 启用 IP 限流中间件。

func (s *Server) EnableSPA(filesys http.FileSystem, indexPath string) *Server
    EnableSPA 启用 SPA 回退：未匹配路由的 GET/HEAD 请求先尝试文件，再回退 index。

func (s *Server) ListenerAddr() string
    ListenerAddr 返回第一个 Listener 的监听地址（port 0 动态端口时可用）。

func (s *Server) OverrideMiddleware(mt MiddlewareType, mw HandlerFunc) *Server
    OverrideMiddleware 使用自定义 Handler 覆盖指定类型的内置中间件。

func (s *Server) RegisterRoute(r Route) *Server
    RegisterRoute 注册单条路由。

func (s *Server) RegisterRouteGroup(prefix string, fn func(*RouteGroup)) *Server
    RegisterRouteGroup 注册路由分组。

func (s *Server) RegisterRoutes(routes []Route) *Server
    RegisterRoutes 批量注册路由。

func (s *Server) ServeStaticDir(prefix, root string) *Server
    ServeStaticDir 从本地目录提供静态文件。

func (s *Server) ServeStaticFS(prefix string, filesys http.FileSystem) *Server
    ServeStaticFS 从 http.FileSystem 提供静态文件，配合 embed 使用。

func (s *Server) Start() error
    Start 启动服务：校验配置、装配中间件、注册路由、创建各通道监听器。 调用后阻塞直到服务关闭或发生错误。

func (s *Server) Stop(ctx context.Context) error
    Stop 优雅关闭服务（幂等，可重复调用）。

func (s *Server) UseGlobalMiddleware(mw ...HandlerFunc) *Server
    UseGlobalMiddleware 追加外部全局中间件。

func (s *Server) UseHttp2Listen(addr string) *Server
    UseHttp2Listen 启用 HTTP/2 TLS 监听（含 HTTP/1.1 兼容）。

func (s *Server) UseHttp3Listen(addr string) *Server
    UseHttp3Listen 启用 HTTP/3 QUIC 监听。

func (s *Server) UseUnixSocketListen(path string, perm os.FileMode) *Server
    UseUnixSocketListen 启用 Unix Socket 监听。 Windows 需 build 1803+，兼容性检查在 Start()
    时执行。

func (s *Server) WithLogger(l logx.Logger) *Server
    WithLogger 注入自定义 logx.Logger。

type StandardizedResponse = core.StandardizedResponse
    StandardizedResponse 是统一的标准 JSON 响应体。


## 包 webx/middleware

package middleware // import "github.com/lcylpzls/webx/middleware"

Package middleware 提供 webx 内置的 HTTP 中间件实现。 包含
Recovery、RequestID、Timeout、CORS、Validation、RateLimit 与 AccessLog。

FUNCTIONS

func AccessLog(logger logx.Logger, successOnly bool) core.HandlerFunc
    AccessLog 返回访问日志中间件。 successOnly 为 true 时记录全部请求；为 false 时仅记录非 2xx 请求。

func CORS(cfg CORSConfig) core.HandlerFunc
    CORS 返回 CORS 跨域处理中间件。

func RateLimit(rl *RateLimiter) core.HandlerFunc
    RateLimit 返回 IP 令牌桶限流中间件，超限返回标准化 429。

func Recovery() core.HandlerFunc
    Recovery 返回 Panic 捕获中间件，这是组件库中唯一调用 recover() 的位置。

func RequestID() core.HandlerFunc
    RequestID 返回请求 ID 生成中间件。 优先使用请求头 X-Request-ID，否则生成 UUID v4。

func Timeout(timeout time.Duration) core.HandlerFunc
    Timeout 返回请求超时中间件。 向请求注入带超时的 Context；超时后丢弃 Handler 写入并返回 503。

func Validation() core.HandlerFunc
    Validation 返回请求参数校验中间件： 校验 Content-Type 是否为 JSON、Content-Length 是否超过 10MB。


TYPES

type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAge         int
}
    CORSConfig 定义 CORS 中间件的配置参数。

func DefaultCORSConfig() CORSConfig
    DefaultCORSConfig 返回常用 CORS 默认配置。

type Manager struct {
	// Has unexported fields.
}
    Manager 管理内置中间件的注册表、执行顺序和启用状态。

func NewManager() *Manager
    NewManager 创建中间件管理器。 默认启用全部内置中间件（RateLimit 注册但默认禁用）。

func (m *Manager) Append(handler ...core.HandlerFunc)
    Append 追加外部全局中间件到中间件链末尾（路由专属中间件之前）。

func (m *Manager) Build(ctx context.Context) []core.HandlerFunc
    Build 构建最终执行的中间件链。 返回顺序：内置（启用）→ 外部全局。

func (m *Manager) Disable(mt ...string)
    Disable 禁用指定类型的内置中间件。

func (m *Manager) DisableRateLimit()
    DisableRateLimit 禁用限流中间件并移除其 Handler。

func (m *Manager) Enable(mt ...string)
    Enable 启用指定类型的内置中间件。 注意：RateLimit 必须通过 EnableRateLimit 激活，Enable 对其无效。

func (m *Manager) EnableRateLimit(handler core.HandlerFunc)
    EnableRateLimit 启用限流中间件并注册其 Handler。

func (m *Manager) Override(mt string, handler core.HandlerFunc)
    Override 覆盖指定类型的内置中间件。

func (m *Manager) RegisterBuiltin(key string, handler core.HandlerFunc)
    RegisterBuiltin 注册一个内置中间件到管理器。

type RateLimiter struct {
	// Has unexported fields.
}
    RateLimiter 实现基于 IP 的令牌桶限流。

func NewRateLimiter(qps int, window time.Duration, whitelistCIDRs []string) *RateLimiter
    NewRateLimiter 创建 IP 限流器。

func (rl *RateLimiter) Allow(ip string) bool
    Allow 检查指定 IP 是否被允许通过。

func (rl *RateLimiter) Cleanup(interval time.Duration)
    Cleanup 清理超过 window*10 未活动的桶。

