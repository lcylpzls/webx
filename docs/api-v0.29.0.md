<!-- v0.29.0 API 基线；生成方式：go doc -all . / ./middleware / ./proxy / ./pprof -->

package webx // import "github.com/lcylpzls/webx"

Package webx 提供基于 Go 标准库的工业级 HTTP/HTTPS 服务组件库。 路由基于自研 radix
匹配树，上下文与中间件链自研，日志/错误/配置 分别接入 logx / errx / confx，HTTP/3 使用 quic-go。

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
    标准化响应业务码。

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


VARIABLES

var NoMethodHandler = core.NoMethodHandler
    NoMethodHandler 405 兜底处理器。

var NoRouteHandler = core.NoRouteHandler
    NoRouteHandler 404 兜底处理器（嵌入自定义路由器时使用）。


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
    GracefulShutdown 监听系统信号并执行优雅关闭。 收到 SIGINT/SIGTERM 后调用 httpServer.Shutdown
    排空请求。

func RespondError(c *Context, err error)
    RespondError 将 errx 错误映射为标准化错误响应。 状态码由 Kind 映射（如 KindNotFound → 404），响应体为统一
    JSON 信封。

func RespondErrorWithData(c *Context, err error, data any)
    RespondErrorWithData 将 errx 错误映射为标准化错误响应，并附带业务数据。

func StatusForError(err error) int
    StatusForError 返回 errx 错误对应的 HTTP 状态码；非 errx 错误返回 500。


TYPES

type Config struct {
	// TLSCertFile TLS 证书文件路径（PEM 格式），必填。
	TLSCertFile string `toml:"tls_cert_file"`
	// TLSKeyFile TLS 私钥文件路径（PEM 格式），必填。
	TLSKeyFile string `toml:"tls_key_file"`
	// MinTLSVersion 最低 TLS 版本，0 表示默认 TLS 1.2（仅允许 TLS1.2/1.3）。
	MinTLSVersion uint16 `toml:"min_tls_version"`

	// ReadTimeout HTTP 读取超时时间。
	ReadTimeout time.Duration `toml:"read_timeout"`
	// WriteTimeout HTTP 写入超时时间。
	WriteTimeout time.Duration `toml:"write_timeout"`
	// ReadHeaderTimeout 请求头读取超时时间，0 表示默认 10s（Slowloris 防护）。
	ReadHeaderTimeout time.Duration `toml:"read_header_timeout"`
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
	// QUICMaxIdleTimeout HTTP/3 空闲连接超时，0 表示默认 30s。
	QUICMaxIdleTimeout time.Duration `toml:"quic_max_idle_timeout"`
	// QUICMaxIncomingStreams HTTP/3 单连接最大入站流数，0 表示默认 100。
	QUICMaxIncomingStreams int64 `toml:"quic_max_incoming_streams"`
	// QUICDrainTimeout HTTP/3 关闭前等待活动连接排空的时间，0 表示不等待。
	QUICDrainTimeout time.Duration `toml:"quic_drain_timeout"`

	// HealthPath 健康检查端点路径，默认为 "/health"。
	HealthPath string `toml:"health_path"`
	// LivenessPath 存活探针端点路径，默认为 "/healthz"。
	LivenessPath string `toml:"liveness_path"`
	// ReadinessPath 就绪探针端点路径，默认为 "/readyz"。
	ReadinessPath string `toml:"readiness_path"`
	// MetricsEnabled 是否启用 Prometheus 文本格式指标端点。
	MetricsEnabled bool `toml:"metrics_enabled"`
	// MetricsPath 指标端点路径，启用时默认为 "/metrics"。
	MetricsPath string `toml:"metrics_path"`

	// LogLevel 日志级别，可选 debug、info、warn、error，为空默认 info。
	LogLevel string `toml:"log_level"`
	// AccessLogEnabled 是否启用访问日志中间件。
	AccessLogEnabled bool `toml:"access_log_enabled"`
	// LogSuccessReq 访问日志是否记录成功请求（默认仅记录非 2xx）。
	LogSuccessReq bool `toml:"log_success_req"`
	// AccessLogSampleRate 访问日志采样率：0=全部记录，N>0 平均每 N 条记录 1 条。
	AccessLogSampleRate int `toml:"access_log_sample_rate"`
	// AccessLogRedact 访问日志 query 参数中需要脱敏的键。
	AccessLogRedact []string `toml:"access_log_redact"`
	// SlowRequestThreshold 慢请求日志阈值（0=关闭）。
	SlowRequestThreshold time.Duration `toml:"slow_request_threshold"`

	// CORSAllowedOrigins CORS 允许的来源列表，为空使用默认值。
	CORSAllowedOrigins []string `toml:"cors_allowed_origins"`
	// CORSAllowedMethods CORS 允许的 HTTP 方法列表。
	CORSAllowedMethods []string `toml:"cors_allowed_methods"`
	// CORSAllowedHeaders CORS 允许的请求头列表。
	CORSAllowedHeaders []string `toml:"cors_allowed_headers"`
	// CORSExposeHeaders CORS 允许浏览器读取的响应头列表。
	CORSExposeHeaders []string `toml:"cors_expose_headers"`
	// CORSMaxAge CORS 预检请求的缓存时间。
	CORSMaxAge time.Duration `toml:"cors_max_age"`
	// CORSAllowCredentials 是否允许携带凭据。
	CORSAllowCredentials bool `toml:"cors_allow_credentials"`

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
	// MiddlewareGzip 是否启用响应压缩中间件。
	MiddlewareGzip bool `toml:"middleware_gzip"`
	// MiddlewareMetrics 是否启用请求/5xx 计数中间件。
	MiddlewareMetrics bool `toml:"middleware_metrics"`
	// MiddlewareSecurity 是否启用安全响应头中间件。
	MiddlewareSecurity bool `toml:"middleware_security"`
	// SecurityHSTSMaxAge HSTS 缓存秒数（0=不启用 HSTS）。
	SecurityHSTSMaxAge int `toml:"security_hsts_max_age"`
	// SecurityReferrerPolicy Referrer-Policy 取值（空=不设置）。
	SecurityReferrerPolicy string `toml:"security_referrer_policy"`
	// SecurityPermissionsPolicy Permissions-Policy 取值（空=不设置）。
	SecurityPermissionsPolicy string `toml:"security_permissions_policy"`
	// SecurityCrossOriginOpenerPolicy Cross-Origin-Opener-Policy 取值（空=不设置）。
	SecurityCrossOriginOpenerPolicy string `toml:"security_cross_origin_opener_policy"`
	// SecurityCrossOriginResourcePolicy Cross-Origin-Resource-Policy 取值（空=不设置）。
	SecurityCrossOriginResourcePolicy string `toml:"security_cross_origin_resource_policy"`
	// SecurityCrossOriginEmbedderPolicy Cross-Origin-Embedder-Policy 取值（空=不设置）。
	SecurityCrossOriginEmbedderPolicy string `toml:"security_cross_origin_embedder_policy"`
	// GzipMinSize 响应压缩最小字节数（0=默认 1024）。
	GzipMinSize int `toml:"gzip_min_size"`
	// GzipLevel 响应压缩级别（0=标准库默认，1-9 对应 BestSpeed-BestCompression）。
	GzipLevel int `toml:"gzip_level"`
	// Debug 调试模式：Recovery 响应携带 panic 摘要（生产环境保持 false）。
	Debug bool `toml:"debug"`
}
    Config 定义 webx Server 的全部配置项，通过 confx 从 TOML 文件加载。 所有校验在 Validate()
    中集中进行，失败返回 errx 结构化错误。

func LoadConfig(path string) (Config, error)
    LoadConfig 通过 confx 从 TOML 文件加载配置并校验。 文件不存在、TOML 非法、存在未声明字段或校验失败时返回 errx 错误。

func (c *Config) Validate() error
    Validate 校验配置完整性并填充默认值。 校验规则：证书/私钥必填且可配对、超时非负、日志级别合法。

type Context = core.Context
    Context 是单个请求的上下文。

func NewContext(w http.ResponseWriter, r *http.Request) *Context
    NewContext 创建请求上下文（用于在自定义路由器中嵌入 webx Handler）。

type GroupStat struct {
	// Prefix 分组前缀。
	Prefix string
	// Requests 请求数。
	Requests uint64
	// Errors5xx 5xx 响应数。
	Errors5xx uint64
	// AvgRequestDurationMs 平均请求耗时（毫秒）。
	AvgRequestDurationMs uint64
}
    GroupStat 单个路由分组的指标统计。

type HandlerFunc = core.HandlerFunc
    HandlerFunc 是 webx 的业务处理器签名，不依赖任何第三方类型。

type KeyFunc func(*Context) string
    KeyFunc 定义限流维度的提取函数（默认按客户端 IP）。

type Metrics struct {
	// Requests 请求总数（需启用 MiddlewareMetrics）。
	Requests uint64
	// Errors5xx 5xx 响应数（需启用 MiddlewareMetrics）。
	Errors5xx uint64
	// Status1xx 1xx 响应数（需启用 MiddlewareMetrics）。
	Status1xx uint64
	// Status2xx 2xx 响应数（需启用 MiddlewareMetrics）。
	Status2xx uint64
	// Status3xx 3xx 响应数（需启用 MiddlewareMetrics）。
	Status3xx uint64
	// Status4xx 4xx 响应数（需启用 MiddlewareMetrics）。
	Status4xx uint64
	// Status5xx 5xx 响应数（需启用 MiddlewareMetrics）。
	Status5xx uint64
	// RateLimited 限流拒绝数（启用 EnableRateLimit 后统计）。
	RateLimited uint64
	// Panics Recovery 捕获的 panic 数（启用 MiddlewareRecovery 后统计）。
	Panics uint64
	// ConcurrencyRejected 并发限制拒绝数（启用 SetMaxConcurrentRequests 后统计）。
	ConcurrencyRejected uint64
	// AvgRequestDurationMs 平均请求耗时（毫秒，需启用 MiddlewareMetrics）。
	AvgRequestDurationMs uint64
	// HTTP1Requests HTTP/1.x 请求数（需启用 MiddlewareMetrics）。
	HTTP1Requests uint64
	// HTTP2Requests HTTP/2 请求数（需启用 MiddlewareMetrics）。
	HTTP2Requests uint64
	// HTTP3Requests HTTP/3 请求数（需启用 MiddlewareMetrics）。
	HTTP3Requests uint64
	// AvgHTTP1RequestDurationMs HTTP/1.x 平均请求耗时（毫秒，需启用 MiddlewareMetrics）。
	AvgHTTP1RequestDurationMs uint64
	// AvgHTTP2RequestDurationMs HTTP/2 平均请求耗时（毫秒，需启用 MiddlewareMetrics）。
	AvgHTTP2RequestDurationMs uint64
	// AvgHTTP3RequestDurationMs HTTP/3 平均请求耗时（毫秒，需启用 MiddlewareMetrics）。
	AvgHTTP3RequestDurationMs uint64
	// ActiveConnections 当前打开的连接数。
	ActiveConnections int64
	// RequestsInFlight 当前活跃请求数（需启用 MiddlewareMetrics）。
	RequestsInFlight int64
}
    Metrics 是 webx 运行指标快照，可接入监控面板。

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
	// MiddlewareGzip 响应压缩中间件。
	MiddlewareGzip MiddlewareType = "gzip"
	// MiddlewareMetrics 请求/5xx 计数中间件。
	MiddlewareMetrics MiddlewareType = "metrics"
	// MiddlewareSecurity 安全响应头中间件。
	MiddlewareSecurity MiddlewareType = "security"
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
	// KeyFunc 限流维度提取函数（可选，默认按客户端 IP）。
	KeyFunc KeyFunc
}
    RateLimitOptions 定义 IP 限流中间件的配置参数。

type RequestIDOptions struct {
	// Header 请求 ID 头名（默认 X-Request-ID）。
	Header string
	// Generator 请求 ID 生成函数（默认 UUID v7）。
	Generator func() string
}
    RequestIDOptions 定义请求 ID 中间件的配置参数。

type Route struct {
	// Method HTTP 方法，如 GET、POST、PUT、DELETE、PATCH。
	Method string
	// Path 路由路径，支持 gin 风格 "/api/users/:id" 与 "/assets/*filepath"。
	Path string
	// Handler 路由处理器。
	Handler HandlerFunc
	// Middleware 路由专属中间件（可选），仅对当前路由生效。
	Middleware []HandlerFunc
	// Group 路由所属分组前缀（由 RouteGroup 自动填充，供分组级指标聚合；直接注册的路由留空）。
	Group string
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

func (rg *RouteGroup) HEAD(path string, handler HandlerFunc, mw ...HandlerFunc)
    HEAD 注册一条 HEAD 方法路由。

func (rg *RouteGroup) OPTIONS(path string, handler HandlerFunc, mw ...HandlerFunc)
    OPTIONS 注册一条 OPTIONS 方法路由。

func (rg *RouteGroup) PATCH(path string, handler HandlerFunc, mw ...HandlerFunc)
    PATCH 注册一条 PATCH 方法路由。

func (rg *RouteGroup) POST(path string, handler HandlerFunc, mw ...HandlerFunc)
    POST 注册一条 POST 方法路由。

func (rg *RouteGroup) PUT(path string, handler HandlerFunc, mw ...HandlerFunc)
    PUT 注册一条 PUT 方法路由。

func (rg *RouteGroup) Use(middleware ...HandlerFunc)
    Use 向当前分组追加中间件，影响该分组内所有已注册和后续注册的路由。

type RouteStat struct {
	// Path 路由注册路径。
	Path string
	// Requests 请求数。
	Requests uint64
	// Errors5xx 5xx 响应数。
	Errors5xx uint64
	// AvgRequestDurationMs 平均请求耗时（毫秒）。
	AvgRequestDurationMs uint64
}
    RouteStat 单条路由的指标统计。

type Router struct {
	// Has unexported fields.
}
    Router 基于自研 radix 匹配树实现路由： 支持 gin 风格语法（:id / *filepath）、404/405 标准化 JSON
    与尾斜杠重定向。 匹配与分发均由自身完成，不依赖 http.ServeMux。

func NewRouter(noRoute, noMethod core.HandlerFunc) *Router
    NewRouter 创建路由，并指定 404/405 兜底处理器。

func (rt *Router) Handle(method, path string, chain []core.HandlerFunc) error
    Handle 注册一条路由（chain 为全局中间件 + 路由中间件 + 最终处理器的完整链）。

func (rt *Router) HandleStatic(prefix string, fs http.FileSystem) error
    HandleStatic 注册静态文件服务（支持子树路径）。

func (rt *Router) HandleStaticWithOptions(prefix string, fs http.FileSystem, opts StaticOptions) error
    HandleStaticWithOptions 注册静态文件服务（含缓存头/目录索引选项）。

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request)
    ServeHTTP 实现 http.Handler：树匹配 + 方法判定 + 分发。

func (rt *Router) SetMaxBodyBytes(n int64)
    SetMaxBodyBytes 设置路由处理链中 BindJSON 的最大请求体字节数。

type SNICertificate struct {
	// ServerName 客户端 SNI 主机名（如 "api.example.com"）。
	ServerName string
	// CertFile 该域名证书文件。
	CertFile string
	// KeyFile 该域名私钥文件。
	KeyFile string
}
    SNICertificate 是按 ServerName（SNI）指定的证书。

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

func (s *Server) EnableMetricsEndpoint(path string) *Server
    EnableMetricsEndpoint 启用 Prometheus 文本格式指标端点（启动前调用）。 path
    为空表示禁用；绕过业务中间件链，避免自采集反馈。

func (s *Server) EnableMiddleware(mt ...MiddlewareType) *Server
    EnableMiddleware 重新启用指定类型的内置中间件（RateLimit 除外）。

func (s *Server) EnableRateLimit(opts RateLimitOptions) *Server
    EnableRateLimit 启用 IP 限流中间件。

func (s *Server) EnableSPA(filesys http.FileSystem, indexPath string) *Server
    EnableSPA 启用 SPA 回退：未匹配路由的 GET/HEAD 请求先尝试文件，再回退 index。

func (s *Server) GroupStats() []GroupStat
    GroupStats 返回分组级统计快照（按分组前缀排序；需启用 MiddlewareMetrics）。

func (s *Server) ListenerAddr() string
    ListenerAddr 返回第一个 Listener 的监听地址（port 0 动态端口时可用）。

func (s *Server) Metrics() Metrics
    Metrics 返回运行指标快照；未启用对应能力时字段为 0。

func (s *Server) OverrideMiddleware(mt MiddlewareType, mw HandlerFunc) *Server
    OverrideMiddleware 使用自定义 Handler 覆盖指定类型的内置中间件。

func (s *Server) RegisterHealthCheck(name string, fn func(context.Context) error) *Server
    RegisterHealthCheck 注册自定义健康检查项，/health 会执行全部检查项。

func (s *Server) RegisterLivenessCheck(name string, fn func(context.Context) error) *Server
    RegisterLivenessCheck 注册存活探针检查项，/healthz 会执行全部存活检查项。

func (s *Server) RegisterOnShutdown(fn func()) *Server
    RegisterOnShutdown 注册关闭钩子（http.Server.Shutdown 触发时执行）。

func (s *Server) RegisterReadinessCheck(name string, fn func(context.Context) error) *Server
    RegisterReadinessCheck 注册就绪探针检查项，/readyz 会执行全部就绪检查项。 服务进入优雅关闭后，就绪探针直接返回 503。

func (s *Server) RegisterRoute(r Route) *Server
    RegisterRoute 注册单条路由。

func (s *Server) RegisterRouteGroup(prefix string, fn func(*RouteGroup)) *Server
    RegisterRouteGroup 注册路由分组。

func (s *Server) RegisterRoutes(routes []Route) *Server
    RegisterRoutes 批量注册路由。

func (s *Server) RouteStats() []RouteStat
    RouteStats 返回路由级统计快照（按注册路径排序；需启用 MiddlewareMetrics）。

func (s *Server) ServeStaticDir(prefix, root string) *Server
    ServeStaticDir 从本地目录提供静态文件。

func (s *Server) ServeStaticDirWithOptions(prefix, root string, opts StaticOptions) *Server
    ServeStaticDirWithOptions 从本地目录提供静态文件，并应用选项。

func (s *Server) ServeStaticFS(prefix string, filesys http.FileSystem) *Server
    ServeStaticFS 从 http.FileSystem 提供静态文件，配合 embed 使用。

func (s *Server) ServeStaticFSWithOptions(prefix string, filesys http.FileSystem, opts StaticOptions) *Server
    ServeStaticFSWithOptions 从 http.FileSystem 提供静态文件，并应用选项。

func (s *Server) SetCertificateLoader(fn func(*tls.ClientHelloInfo) (*tls.Certificate, error)) *Server
    SetCertificateLoader 设置自定义证书加载器（用于 SNI 多证书、KMS 等场景）。 未设置时默认从 Config
    的证书/私钥文件按需加载并缓存（文件变化自动重载）。

func (s *Server) SetConnContext(fn func(context.Context, net.Conn) context.Context) *Server
    SetConnContext 设置每连接上下文注入函数（供链路/连接级数据传播）。

func (s *Server) SetMaxConcurrentRequests(n int) *Server
    SetMaxConcurrentRequests 设置同时处理的请求数上限（启动前调用）。 n <= 0 表示不限制；超限请求返回 503 并携带
    Retry-After。

func (s *Server) SetMiddlewareOrder(order []MiddlewareType) *Server
    SetMiddlewareOrder 设置内置中间件执行顺序（默认顺序保持不变）。

func (s *Server) SetRequestIDOptions(opts RequestIDOptions) *Server
    SetRequestIDOptions 设置请求 ID 中间件的配置（启动前调用）。

func (s *Server) SetSNICertificates(certs []SNICertificate) *Server
    SetSNICertificates 设置按 SNI 域名区分的多证书；未匹配域名回退到默认证书。

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

type StaticOptions struct {
	// MaxAge 设置 Cache-Control: max-age（0 表示不设置）。
	MaxAge time.Duration
	// DisableIndex 禁用目录索引：无 index.html 的目录返回 404。
	DisableIndex bool
	// EnableETag 按文件 mtime 与大小生成弱 ETag，支持 If-None-Match 返回 304。
	EnableETag bool
}
    StaticOptions 定义静态文件服务的选项。


## 包 webx/middleware

package middleware // import "github.com/lcylpzls/webx/middleware"

Package middleware 提供 webx 内置的 HTTP 中间件实现。 包含
Recovery、RequestID、Timeout、CORS、Validation、RateLimit 与 AccessLog。

FUNCTIONS

func AccessLog(logger logx.Logger, opts AccessLogOptions) core.HandlerFunc
    AccessLog 返回访问日志中间件。

func BodyLimit(maxBytes int64) core.HandlerFunc
    BodyLimit 返回请求体大小限制中间件。 Content-Length 明确超限时直接返回 413；chunked 请求体由
    MaxBytesReader 兜底。

func CORS(cfg CORSConfig) core.HandlerFunc
    CORS 返回 CORS 跨域处理中间件。

func ConcurrencyLimit(l *ConcurrencyLimiter) core.HandlerFunc
    ConcurrencyLimit 返回并发限制中间件；额度已满时返回 503 并携带 Retry-After。

func Gzip() core.HandlerFunc
    Gzip 返回响应压缩中间件：客户端 Accept-Encoding 含 gzip 时启用。

func GzipWithOptions(opts GzipOptions) core.HandlerFunc
    GzipWithOptions 返回带选项的响应压缩中间件。

func Hooks(onRequest, onResponse func(*core.Context)) core.HandlerFunc
    Hooks 返回请求钩子中间件：进入时调用 onRequest，处理结束后调用 onResponse。 可用于 OpenTelemetry
    适配等观测场景；回调可传 nil。

func MetricsHandler(m *Metrics) core.HandlerFunc
    MetricsHandler 返回指标采集中间件。 panic 安全：请求处理发生 panic 时仍会记录请求数、耗时与 5xx 分布， 随后重新抛出
    panic 交由 Recovery 中间件处理。

func RateLimit(rl *RateLimiter) core.HandlerFunc
    RateLimit 返回 IP 令牌桶限流中间件，超限返回标准化 429。

func Recovery() core.HandlerFunc
    Recovery 返回 Panic 捕获中间件，这是组件库中唯一调用 recover() 的位置。

func RecoveryWith(logger logx.Logger, m *Metrics) core.HandlerFunc
    RecoveryWith 返回 Panic 捕获中间件，统计 panic 数量并输出日志。

func RecoveryWithMetrics(m *Metrics) core.HandlerFunc
    RecoveryWithMetrics 返回 Panic 捕获中间件，并统计 panic 数量。

func RecoveryWithOptions(logger logx.Logger, m *Metrics, debugMode bool) core.HandlerFunc
    RecoveryWithOptions 返回 Panic 捕获中间件；debugMode 为 true 时响应携带 panic 摘要。

func RequestID() core.HandlerFunc
    RequestID 返回请求 ID 生成中间件。 优先使用请求头 X-Request-ID，否则生成 UUID v7。

func RequestIDWithOptions(opts RequestIDOptions) core.HandlerFunc
    RequestIDWithOptions 返回按选项配置的请求 ID 生成中间件。

func SecurityHeaders(opts SecurityHeadersOptions) core.HandlerFunc
    SecurityHeaders 返回安全响应头中间件。

func Timeout(timeout time.Duration) core.HandlerFunc
    Timeout 返回请求超时中间件。 向请求注入带超时的 Context；超时后丢弃 Handler 写入并返回 503。

func Validation() core.HandlerFunc
    Validation 返回请求参数校验中间件： 校验 Content-Type 是否为 JSON、Content-Length 是否超过 10MB。


TYPES

type AccessLogOptions struct {
	// LogSuccess 是否记录成功请求（默认仅记录非 2xx）。
	LogSuccess bool
	// SampleRate 采样率：0 表示记录全部；N>0 表示平均每 N 条记录 1 条。
	SampleRate int
	// RedactKeys query 参数中需要脱敏的键。
	RedactKeys []string
	// SlowThreshold 慢请求阈值；>0 且请求耗时达到阈值时额外记录 Warn（默认关闭）。
	SlowThreshold time.Duration
}
    AccessLogOptions 定义访问日志中间件的配置。

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposeHeaders    []string
	MaxAge           int
	AllowCredentials bool
}
    CORSConfig 定义 CORS 中间件的配置参数。

func DefaultCORSConfig() CORSConfig
    DefaultCORSConfig 返回常用 CORS 默认配置。

type ConcurrencyLimiter struct {
	// Has unexported fields.
}
    ConcurrencyLimiter 限制同一时刻处理的请求数。

func NewConcurrencyLimiter(max int) *ConcurrencyLimiter
    NewConcurrencyLimiter 创建并发限制器；max <= 0 表示不限制。

func (l *ConcurrencyLimiter) Active() int64
    Active 返回当前占用的并发额度。

func (l *ConcurrencyLimiter) Rejected() uint64
    Rejected 返回因额度已满被拒绝的请求数。

func (l *ConcurrencyLimiter) Release()
    Release 释放一个并发额度。

func (l *ConcurrencyLimiter) TryAcquire() bool
    TryAcquire 尝试占用一个并发额度；额度已满返回 false。

type GroupStat struct {
	// Prefix 分组前缀。
	Prefix string
	// Requests 请求数。
	Requests uint64
	// Errors5xx 5xx 响应数。
	Errors5xx uint64
	// AvgDurationMs 平均请求耗时（毫秒）。
	AvgDurationMs uint64
}
    GroupStat 单个路由分组的指标统计快照。

type GzipOptions struct {
	// MinSize 未显式写状态码时，小于该字节数的响应不压缩（0=默认 1024）。
	MinSize int
	// Level 压缩级别（0=标准库默认；1-9 对应 BestSpeed-BestCompression）。
	Level int
}
    GzipOptions 定义响应压缩中间件的选项。

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

func (m *Manager) SetOrder(keys ...string)
    SetOrder 设置内置中间件的执行顺序（未知键与空顺序忽略）。

type Metrics struct {
	// Has unexported fields.
}
    Metrics 统计请求、状态码分布与协议维度指标，供监控面板对接。

func NewMetrics() *Metrics
    NewMetrics 创建指标计数器。

func (m *Metrics) Durations() (totalNs, samples uint64)
    Durations 返回累计耗时（纳秒）与样本数。

func (m *Metrics) GroupStats() []GroupStat
    GroupStats 返回分组级统计快照（按分组前缀排序）。

func (m *Metrics) InFlight() int64
    InFlight 返回当前活跃请求数。

func (m *Metrics) Panics() uint64
    Panics 返回 Recovery 捕获的 panic 数量。

func (m *Metrics) ProtocolStats() ProtocolStats
    ProtocolStats 返回各协议（HTTP/1.x、HTTP/2、HTTP/3）的请求数与平均耗时（毫秒）。

func (m *Metrics) RouteStats() []RouteStat
    RouteStats 返回路由级统计快照（按注册路径排序）。

func (m *Metrics) Snapshot() (requests, errors5x uint64)
    Snapshot 返回当前计数快照。

func (m *Metrics) StatusCodes() (s1xx, s2xx, s3xx, s4xx, s5xx uint64)
    StatusCodes 返回按状态码分类的请求计数（1xx/2xx/3xx/4xx/5xx）。

type ProtocolStats struct {
	// HTTP1Requests HTTP/1.0 与 HTTP/1.1 请求数。
	HTTP1Requests uint64
	// HTTP2Requests HTTP/2 请求数。
	HTTP2Requests uint64
	// HTTP3Requests HTTP/3 请求数。
	HTTP3Requests uint64
	// HTTP1AvgMs HTTP/1.x 平均耗时（毫秒）。
	HTTP1AvgMs uint64
	// HTTP2AvgMs HTTP/2 平均耗时（毫秒）。
	HTTP2AvgMs uint64
	// HTTP3AvgMs HTTP/3 平均耗时（毫秒）。
	HTTP3AvgMs uint64
}
    ProtocolStats 协议维度请求统计快照。

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

func (rl *RateLimiter) Rejected() uint64
    Rejected 返回被拒绝的请求数。

func (rl *RateLimiter) RetryAfter(key string) time.Duration
    RetryAfter 返回指定 key 恢复 1 枚令牌所需的等待时间（秒，向上取整）。

func (rl *RateLimiter) SetKeyFunc(fn func(*core.Context) string)
    SetKeyFunc 设置限流维度提取函数（默认按客户端 IP）。

func (rl *RateLimiter) SetMaxBuckets(n int)
    SetMaxBuckets 设置 IP 桶数量上限；达到上限后新 IP 直接拒绝。

type RequestIDOptions struct {
	// Header 请求 ID 头名（默认 X-Request-ID）。
	Header string
	// Generator 请求 ID 生成函数（默认 UUID v7）。
	Generator func() string
}
    RequestIDOptions 定义请求 ID 中间件的配置参数。

type RouteStat struct {
	// Path 路由注册路径。
	Path string
	// Requests 请求数。
	Requests uint64
	// Errors5xx 5xx 响应数。
	Errors5xx uint64
	// AvgDurationMs 平均请求耗时（毫秒）。
	AvgDurationMs uint64
}
    RouteStat 单条路由的指标统计快照。

type SecurityHeadersOptions struct {
	// ContentTypeNoSniff 设置 X-Content-Type-Options: nosniff。
	ContentTypeNoSniff bool
	// FrameDeny 设置 X-Frame-Options: DENY。
	FrameDeny bool
	// ReferrerPolicy 设置 Referrer-Policy（空则不设置）。
	ReferrerPolicy string
	// HSTSMaxAge 大于 0 时设置 Strict-Transport-Security。
	HSTSMaxAge time.Duration
	// PermissionsPolicy 设置 Permissions-Policy（空则不设置）。
	PermissionsPolicy string
	// CrossOriginOpenerPolicy 设置 Cross-Origin-Opener-Policy（空则不设置）。
	CrossOriginOpenerPolicy string
	// CrossOriginResourcePolicy 设置 Cross-Origin-Resource-Policy（空则不设置）。
	CrossOriginResourcePolicy string
	// CrossOriginEmbedderPolicy 设置 Cross-Origin-Embedder-Policy（空则不设置）。
	CrossOriginEmbedderPolicy string
}
    SecurityHeadersOptions 定义安全响应头中间件的配置。


## 包 webx/proxy

package proxy // import "github.com/lcylpzls/webx/proxy"

Package proxy 提供基于标准库 httputil.ReverseProxy 的上游代理封装。

FUNCTIONS

func DefaultErrorHandler(w http.ResponseWriter, r *http.Request, err error)
    DefaultErrorHandler 输出统一 JSON 502 错误响应。

func Handler(target *url.URL, opts ...Option) webx.HandlerFunc
    Handler 返回反向代理处理器，将请求转发到 target。


TYPES

type Option func(*httputil.ReverseProxy)
    Option 配置 ReverseProxy 的选项。

func WithErrorHandler(fn func(http.ResponseWriter, *http.Request, error)) Option
    WithErrorHandler 设置上游错误处理器。

func WithFlushInterval(d time.Duration) Option
    WithFlushInterval 设置流式响应刷新间隔。 负数表示每次写入后立即刷新（SSE 等长连接场景）；0 使用标准库默认行为。

func WithTimeout(d time.Duration) Option
    WithTimeout 设置上游请求整体超时；<=0 表示不限制。 超时覆盖连接与响应头阶段，超时后交由 ErrorHandler 输出错误响应。


## 包 webx/pprof

package pprof // import "github.com/lcylpzls/webx/pprof"

Package pprof 注册标准库 net/http/pprof 处理器，便于线上性能诊断。

FUNCTIONS

func Register(s Registrar) *webx.Server
    Register 注册 /debug/pprof 相关处理器。


TYPES

type Registrar interface {
	RegisterRoute(webx.Route) *webx.Server
}
    Registrar 抽象路由注册能力（*webx.Server 满足）。

