// Package webx 提供基于 Go 标准库的工业级 HTTP/HTTPS 服务组件库。
// 路由基于自研 radix 匹配树，上下文与中间件链自研，日志/错误/配置
// 分别接入 logx / errx / confx，HTTP/3 使用 quic-go。
package webx

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/lcylpzls/errx"
	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/webx/internal/core"
	"github.com/lcylpzls/webx/middleware"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// routeGroupEntry 记录路由分组的配置，在 Start() 时展开注册。
type routeGroupEntry struct {
	prefix string
	fn     func(*RouteGroup)
}

// Server 是 webx 的核心类型，提供多通道 HTTPS 服务能力。
// 通过链式 API 配置，Start() 启动，Stop(ctx) 优雅关闭。
type Server struct {
	config             Config
	logger             logx.Logger
	routes             []Route
	routeGroups        []routeGroupEntry
	staticEntries      []staticEntry
	spa                *spaConfig
	healthChecks       []healthCheck
	mwManager          *middleware.Manager
	rateLimiter        *middleware.RateLimiter
	metrics            *middleware.Metrics
	concurrencyLimiter *middleware.ConcurrencyLimiter
	router             *Router
	startTime          time.Time
	started            bool
	mu                 sync.Mutex
	shutdownOnce       sync.Once
	cleanupFuncs       []func()
	signalCtx          context.Context
	signalCancel       context.CancelFunc

	listeners     []net.Listener
	quicListeners []*quic.Listener
	http3Servers  []*http3.Server
	listenersMu   sync.Mutex
	httpServers   []*http.Server

	http2Enabled    bool
	http2Addr       string
	http3Enabled    bool
	http3Addr       string
	unixEnabled     bool
	unixSocketPath  string
	unixSocketPerm  os.FileMode
	certLoader      func(*tls.ClientHelloInfo) (*tls.Certificate, error)
	sniCerts        []SNICertificate
	livenessChecks  []healthCheck
	readinessChecks []healthCheck
	shuttingDown    atomic.Bool
	activeConns     atomic.Int64
	connCtx         func(context.Context, net.Conn) context.Context
	onShutdown      []func()
	mwOrder         []MiddlewareType
	requestIDOpts   RequestIDOptions
	metricsPath     string
	maxConcurrent   int
}

// SNICertificate 是按 ServerName（SNI）指定的证书。
type SNICertificate struct {
	// ServerName 客户端 SNI 主机名（如 "api.example.com"）。
	ServerName string
	// CertFile 该域名证书文件。
	CertFile string
	// KeyFile 该域名私钥文件。
	KeyFile string
}

// checkUnixSocket 是可注入的 Unix Socket 平台检查（测试可替换）。
var checkUnixSocket = unixSocketSupported

// NewServer 创建 webx Server 实例。
// logger 由调用方注入（logx.Logger），webx 内部只使用、不创建日志器；
// logger 为 nil 时 Start() 会返回错误。
func NewServer(cfg Config, logger logx.Logger) *Server {
	return &Server{
		config:    cfg,
		logger:    logger,
		mwManager: middleware.NewManager(),
	}
}

// WithLogger 注入自定义 logx.Logger。
func (s *Server) WithLogger(l logx.Logger) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.warnStarted("修改 Logger")
		return s
	}
	s.logger = l
	return s
}

// UseGlobalMiddleware 追加外部全局中间件。
func (s *Server) UseGlobalMiddleware(mw ...HandlerFunc) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.warnStarted("修改全局中间件")
		return s
	}
	s.mwManager.Append(mw...)
	return s
}

// OverrideMiddleware 使用自定义 Handler 覆盖指定类型的内置中间件。
func (s *Server) OverrideMiddleware(mt MiddlewareType, mw HandlerFunc) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.warnStarted("覆盖中间件")
		return s
	}
	s.mwManager.Override(string(mt), mw)
	return s
}

// DisableMiddleware 禁用指定类型的内置中间件。
func (s *Server) DisableMiddleware(mt ...MiddlewareType) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.warnStarted("禁用中间件")
		return s
	}
	keys := make([]string, len(mt))
	for i, t := range mt {
		keys[i] = string(t)
	}
	s.mwManager.Disable(keys...)
	return s
}

// EnableMiddleware 重新启用指定类型的内置中间件（RateLimit 除外）。
func (s *Server) EnableMiddleware(mt ...MiddlewareType) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.warnStarted("启用中间件")
		return s
	}
	keys := make([]string, len(mt))
	for i, t := range mt {
		keys[i] = string(t)
	}
	s.mwManager.Enable(keys...)
	return s
}

// RegisterRoute 注册单条路由。
func (s *Server) RegisterRoute(r Route) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.warnStarted("注册路由")
		return s
	}
	s.routes = append(s.routes, r)
	return s
}

// RegisterRoutes 批量注册路由。
func (s *Server) RegisterRoutes(routes []Route) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.warnStarted("批量注册路由")
		return s
	}
	s.routes = append(s.routes, routes...)
	return s
}

// RegisterRouteGroup 注册路由分组。
func (s *Server) RegisterRouteGroup(prefix string, fn func(*RouteGroup)) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.warnStarted("注册路由分组")
		return s
	}
	s.routeGroups = append(s.routeGroups, routeGroupEntry{prefix: prefix, fn: fn})
	return s
}

// UseHttp2Listen 启用 HTTP/2 TLS 监听（含 HTTP/1.1 兼容）。
func (s *Server) UseHttp2Listen(addr string) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.warnStarted("启用 HTTP/2 监听")
		return s
	}
	s.http2Enabled = true
	s.http2Addr = addr
	return s
}

// UseHttp3Listen 启用 HTTP/3 QUIC 监听。
func (s *Server) UseHttp3Listen(addr string) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.warnStarted("启用 HTTP/3 监听")
		return s
	}
	s.http3Enabled = true
	s.http3Addr = addr
	return s
}

// UseUnixSocketListen 启用 Unix Socket 监听。
// Windows 需 build 1803+，兼容性检查在 Start() 时执行。
func (s *Server) UseUnixSocketListen(path string, perm os.FileMode) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.warnStarted("启用 Unix Socket 监听")
		return s
	}
	s.unixEnabled = true
	s.unixSocketPath = path
	if perm == 0 {
		perm = 0660
	}
	s.unixSocketPerm = perm
	return s
}

// SetCertificateLoader 设置自定义证书加载器（用于 SNI 多证书、KMS 等场景）。
// 未设置时默认从 Config 的证书/私钥文件按需加载并缓存（文件变化自动重载）。
func (s *Server) SetCertificateLoader(fn func(*tls.ClientHelloInfo) (*tls.Certificate, error)) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.logWarn("webx：服务已启动，不允许设置证书加载器")
		return s
	}
	s.certLoader = fn
	return s
}

// SetConnContext 设置每连接上下文注入函数（供链路/连接级数据传播）。
func (s *Server) SetConnContext(fn func(context.Context, net.Conn) context.Context) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.logWarn("webx：服务已启动，不允许设置连接上下文函数")
		return s
	}
	if fn != nil {
		s.connCtx = fn
	}
	return s
}

// RegisterOnShutdown 注册关闭钩子（http.Server.Shutdown 触发时执行）。
func (s *Server) RegisterOnShutdown(fn func()) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.logWarn("webx：服务已启动，不允许注册关闭钩子")
		return s
	}
	if fn != nil {
		s.onShutdown = append(s.onShutdown, fn)
	}
	return s
}

// SetMiddlewareOrder 设置内置中间件执行顺序（默认顺序保持不变）。
func (s *Server) SetMiddlewareOrder(order []MiddlewareType) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.logWarn("webx：服务已启动，不允许设置中间件顺序")
		return s
	}
	s.mwOrder = append([]MiddlewareType(nil), order...)
	return s
}

// SetSNICertificates 设置按 SNI 域名区分的多证书；未匹配域名回退到默认证书。
func (s *Server) SetSNICertificates(certs []SNICertificate) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.logWarn("webx：服务已启动，不允许设置 SNI 证书")
		return s
	}
	valid := make([]SNICertificate, 0, len(certs))
	for _, c := range certs {
		if c.ServerName != "" && c.CertFile != "" && c.KeyFile != "" {
			valid = append(valid, c)
		}
	}
	s.sniCerts = valid
	return s
}

// SetRequestIDOptions 设置请求 ID 中间件的配置（启动前调用）。
func (s *Server) SetRequestIDOptions(opts RequestIDOptions) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.warnStarted("设置请求 ID 选项")
		return s
	}
	s.requestIDOpts = opts
	return s
}

// EnableMetricsEndpoint 启用 Prometheus 文本格式指标端点（启动前调用）。
// path 为空表示禁用；绕过业务中间件链，避免自采集反馈。
func (s *Server) EnableMetricsEndpoint(path string) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.warnStarted("启用指标端点")
		return s
	}
	s.metricsPath = path
	return s
}

// SetMaxConcurrentRequests 设置同时处理的请求数上限（启动前调用）。
// n <= 0 表示不限制；超限请求返回 503 并携带 Retry-After。
func (s *Server) SetMaxConcurrentRequests(n int) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.warnStarted("设置并发上限")
		return s
	}
	s.maxConcurrent = n
	return s
}

// EnableRateLimit 启用 IP 限流中间件。
func (s *Server) EnableRateLimit(opts RateLimitOptions) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.warnStarted("启用限流")
		return s
	}
	if opts.QPS <= 0 || opts.Window <= 0 {
		return s
	}
	rl := middleware.NewRateLimiter(opts.QPS, opts.Window, opts.Whitelist)
	if opts.KeyFunc != nil {
		rl.SetKeyFunc(func(c *core.Context) string {
			return opts.KeyFunc(c)
		})
	}
	s.rateLimiter = rl
	s.mwManager.EnableRateLimit(middleware.RateLimit(rl))

	cleanupInterval := opts.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cleanupFuncs = append(s.cleanupFuncs, cancel)
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				rl.Cleanup(cleanupInterval)
			}
		}
	}()
	return s
}

// DisableRateLimit 禁用 IP 限流中间件。
func (s *Server) DisableRateLimit() *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.warnStarted("禁用限流")
		return s
	}
	s.mwManager.DisableRateLimit()
	s.rateLimiter = nil
	return s
}

// RegisterHealthCheck 注册自定义健康检查项，/health 会执行全部检查项。
func (s *Server) RegisterHealthCheck(name string, fn func(context.Context) error) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.logWarn("webx：服务已启动，不允许注册健康检查")
		return s
	}
	if name == "" || fn == nil {
		return s
	}
	s.healthChecks = append(s.healthChecks, healthCheck{name: name, fn: fn})
	return s
}

// RegisterLivenessCheck 注册存活探针检查项，/healthz 会执行全部存活检查项。
func (s *Server) RegisterLivenessCheck(name string, fn func(context.Context) error) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.logWarn("webx：服务已启动，不允许注册存活检查")
		return s
	}
	if name == "" || fn == nil {
		return s
	}
	s.livenessChecks = append(s.livenessChecks, healthCheck{name: name, fn: fn})
	return s
}

// RegisterReadinessCheck 注册就绪探针检查项，/readyz 会执行全部就绪检查项。
// 服务进入优雅关闭后，就绪探针直接返回 503。
func (s *Server) RegisterReadinessCheck(name string, fn func(context.Context) error) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.logWarn("webx：服务已启动，不允许注册就绪检查")
		return s
	}
	if name == "" || fn == nil {
		return s
	}
	s.readinessChecks = append(s.readinessChecks, healthCheck{name: name, fn: fn})
	return s
}

// ListenerAddr 返回第一个 Listener 的监听地址（port 0 动态端口时可用）。
func (s *Server) ListenerAddr() string {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	if len(s.listeners) > 0 {
		return s.listeners[0].Addr().String()
	}
	if len(s.quicListeners) > 0 {
		return s.quicListeners[0].Addr().String()
	}
	return ""
}

// Start 启动服务：校验配置、装配中间件、注册路由、创建各通道监听器。
// 调用后阻塞直到服务关闭或发生错误。
func (s *Server) Start() error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errx.New(errx.KindInvalid, CodeStartFailed, "服务已启动，不允许重复启动")
	}
	s.started = true
	s.mu.Unlock()

	if s.logger == nil {
		return errx.New(errx.KindInvalid, CodeStartFailed, "logger 不能为 nil，请在 NewServer 时注入 logx.Logger")
	}

	// 启动失败时回收限流清理 goroutine，避免泄漏。
	startedOK := false
	defer func() {
		if !startedOK {
			s.cancelCleanup()
		}
	}()

	if !s.http2Enabled && !s.http3Enabled && !s.unixEnabled {
		return errx.New(errx.KindInvalid, CodeStartFailed, "至少需要启用一种监听方式（HTTP/2、HTTP/3 或 Unix Socket）")
	}
	if err := s.config.Validate(); err != nil {
		return err
	}
	if s.unixEnabled {
		if err := checkUnixSocket(); err != nil {
			return errx.Wrap(err, errx.KindInvalid, CodeStartFailed, "Unix Socket 平台检查失败")
		}
	}
	s.startTime = time.Now()
	s.signalCtx, s.signalCancel = context.WithCancel(context.Background())
	s.registerBuiltinMiddleware()
	if len(s.mwOrder) > 0 {
		keys := make([]string, len(s.mwOrder))
		for i, t := range s.mwOrder {
			keys[i] = string(t)
		}
		s.mwManager.SetOrder(keys...)
	}

	noRoute := core.NoRouteHandler
	if s.spa != nil {
		noRoute = spaNoRoute(s.spa.fs, s.spa.indexPath)
	}
	s.router = NewRouter(noRoute, core.NoMethodHandler)
	s.router.SetMaxBodyBytes(s.config.MaxBodyBytes)

	ctx := context.Background()
	globalChain := s.mwManager.Build(ctx)
	buildChain := func(route Route) []core.HandlerFunc {
		// 首个处理器注入路由/分组元数据，供 Metrics 等中间件做路由级聚合。
		chain := []core.HandlerFunc{
			func(c *core.Context) {
				c.SetRoute(route.Path)
				c.SetGroup(route.Group)
				c.Next()
			},
		}
		chain = append(chain, globalChain...)
		chain = append(chain, route.Middleware...)
		chain = append(chain, route.Handler)
		return chain
	}

	for _, entry := range s.staticEntries {
		if err := s.router.HandleStaticWithOptions(entry.prefix, entry.fs, entry.opts); err != nil {
			return errx.Wrap(err, errx.KindInvalid, CodeStartFailed, "静态文件路由注册失败")
		}
	}
	for _, route := range s.routes {
		if err := s.router.Handle(route.Method, route.Path, buildChain(route)); err != nil {
			return errx.Wrap(err, errx.KindInvalid, CodeStartFailed, "路由注册失败："+route.Method+" "+route.Path)
		}
	}
	for _, entry := range s.routeGroups {
		rg := &RouteGroup{prefix: entry.prefix}
		if err := callRouteGroupFn(entry.fn, rg); err != nil {
			return errx.Wrap(err, errx.KindInvalid, CodeStartFailed, "路由分组回调执行失败")
		}
		for _, route := range rg.flatten() {
			if err := s.router.Handle(route.Method, route.Path, buildChain(route)); err != nil {
				return errx.Wrap(err, errx.KindInvalid, CodeStartFailed, "路由注册失败："+route.Method+" "+route.Path)
			}
		}
	}
	probeEndpoints := []struct {
		path    string
		handler HandlerFunc
	}{
		{s.config.HealthPath, healthHandler(s.startTime, s.healthChecks, nil)},
		{s.config.LivenessPath, healthHandler(s.startTime, s.livenessChecks, nil)},
		{s.config.ReadinessPath, healthHandler(s.startTime, s.readinessChecks, func() bool { return s.shuttingDown.Load() })},
	}
	for _, ep := range probeEndpoints {
		has, err := s.hasRoute(ep.path)
		if err != nil {
			return errx.Wrap(err, errx.KindInvalid, CodeStartFailed, "健康探针路由检查失败")
		}
		if !has {
			if err := s.router.Handle("GET", ep.path, []core.HandlerFunc{ep.handler}); err != nil {
				return errx.Wrap(err, errx.KindInvalid, CodeStartFailed, "健康探针路由注册失败")
			}
		}
	}
	if path := s.metricsEndpointPath(); path != "" {
		if err := s.router.Handle("GET", path, []core.HandlerFunc{s.serveMetrics}); err != nil {
			return errx.Wrap(err, errx.KindInvalid, CodeStartFailed, "指标端点路由注册失败")
		}
	}

	var wg sync.WaitGroup
	if s.http2Enabled {
		getCert := s.buildGetCertificate()
		ln, err := createTLSListener(s.http2Addr, getCert, s.config.MinTLSVersion)
		if err != nil {
			s.closeListeners()
			return err
		}
		s.addListener(ln)
		srv := &http.Server{
			Handler:           s.router,
			ReadTimeout:       s.config.ReadTimeout,
			WriteTimeout:      s.config.WriteTimeout,
			ReadHeaderTimeout: s.config.ReadHeaderTimeout,
			IdleTimeout:       s.config.IdleTimeout,
			MaxHeaderBytes:    s.config.MaxHeaderBytes,
			ConnState:         s.connState,
			ConnContext:       s.connCtx,
		}
		s.applyServerHooks(srv)
		s.addHTTPServer(srv)
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.logger.WithContext(ctx).Info("webx：HTTP/2 服务已启动", logx.Fields(logx.String("地址", s.http2Addr)))
			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.logger.WithContext(ctx).Error("webx：HTTP/2 服务异常退出", logx.Fields(logx.Any("error", err)))
			}
		}()
	}
	if s.http3Enabled {
		getCert := s.buildGetCertificate()
		qln, err := createQUICListener(s.http3Addr, getCert, s.config.MinTLSVersion,
			s.config.QUICMaxIdleTimeout, s.config.QUICMaxIncomingStreams)
		if err != nil {
			s.closeListeners()
			return err
		}
		s.addQUICListener(qln)
		h3s := &http3.Server{Handler: s.router}
		s.addHTTP3Server(h3s)
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.logger.WithContext(ctx).Info("webx：HTTP/3 (QUIC) 服务已启动", logx.Fields(logx.String("地址", s.http3Addr)))
			if err := serveHTTP3(ctx, h3s, qln); err != nil {
				logHTTP3Exit(s.logger, ctx, err)
			}
		}()
	}
	if s.unixEnabled {
		ln, err := createUnixListener(s.unixSocketPath, s.unixSocketPerm)
		if err != nil {
			s.closeListeners()
			return err
		}
		s.addListener(ln)
		srv := &http.Server{
			Handler:           s.router,
			ReadTimeout:       s.config.ReadTimeout,
			WriteTimeout:      s.config.WriteTimeout,
			ReadHeaderTimeout: s.config.ReadHeaderTimeout,
			IdleTimeout:       s.config.IdleTimeout,
			MaxHeaderBytes:    s.config.MaxHeaderBytes,
			ConnState:         s.connState,
			ConnContext:       s.connCtx,
		}
		s.applyServerHooks(srv)
		s.addHTTPServer(srv)
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.logger.WithContext(ctx).Info("webx：Unix Socket 服务已启动", logx.Fields(logx.String("路径", s.unixSocketPath)))
			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.logger.WithContext(ctx).Error("webx：Unix Socket 服务异常退出", logx.Fields(logx.Any("error", err)))
			}
		}()
	}

	startedOK = true
	go s.waitSignal(listenSignals())
	wg.Wait()
	return nil
}

// buildGetCertificate 返回当前证书加载器；默认使用带 mtime 缓存的文件提供器。
func (s *Server) buildGetCertificate() func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	if s.certLoader != nil {
		return s.certLoader
	}
	defaultCert := newCertificateProvider(s.config.TLSCertFile, s.config.TLSKeyFile)
	if len(s.sniCerts) == 0 {
		return defaultCert.getCertificate
	}
	providers := make(map[string]*certificateProvider, len(s.sniCerts))
	for _, c := range s.sniCerts {
		providers[c.ServerName] = newCertificateProvider(c.CertFile, c.KeyFile)
	}
	return func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if chi != nil {
			if p, ok := providers[chi.ServerName]; ok {
				return p.getCertificate(chi)
			}
		}
		return defaultCert.getCertificate(chi)
	}
}

// listenSignals 创建系统信号监听通道。
func listenSignals() chan os.Signal {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	return quit
}

// waitSignal 监听系统信号并执行优雅关闭。
func (s *Server) waitSignal(quit chan os.Signal) {
	defer signal.Stop(quit)
	select {
	case sig := <-quit:
		s.logger.WithContext(s.signalCtx).Info("webx：收到系统信号，开始优雅关闭", logx.Fields(logx.String("信号", sig.String())))
		s.requestShutdown(context.Background())
	case <-s.signalCtx.Done():
	}
}

// Stop 优雅关闭服务（幂等，可重复调用）。
func (s *Server) Stop(ctx context.Context) error {
	var err error
	s.shutdownOnce.Do(func() {
		err = s.shutdownAll(ctx)
	})
	return err
}

// requestShutdown 幂等触发优雅关闭（信号路径与 Stop 共用）。
func (s *Server) requestShutdown(ctx context.Context) {
	s.shutdownOnce.Do(func() {
		_ = s.shutdownAll(ctx)
	})
}

// shutdownAll 关闭全部 Server 与 Listener，清理 Unix Socket 与清理函数。
func (s *Server) shutdownAll(ctx context.Context) error {
	s.shuttingDown.Store(true)
	if s.signalCancel != nil {
		s.signalCancel()
	}
	s.listenersMu.Lock()
	servers := make([]*http.Server, len(s.httpServers))
	copy(servers, s.httpServers)
	listeners := make([]net.Listener, len(s.listeners))
	copy(listeners, s.listeners)
	quicListeners := make([]*quic.Listener, len(s.quicListeners))
	copy(quicListeners, s.quicListeners)
	h3Servers := make([]*http3.Server, len(s.http3Servers))
	copy(h3Servers, s.http3Servers)
	s.listenersMu.Unlock()

	unixPath := ""
	if s.unixEnabled && s.unixSocketPath != "" {
		unixPath = s.unixSocketPath
	}
	err := shutdownServers(ctx, s.logger, servers, listeners, s.config.ShutdownTimeout, unixPath, s.cleanupFuncs)
	for _, qln := range quicListeners {
		qln.Close()
	}
	if s.config.QUICDrainTimeout > 0 {
		time.Sleep(s.config.QUICDrainTimeout)
	}
	for _, h3s := range h3Servers {
		_ = h3s.Close()
	}
	if err != nil {
		return errx.Wrap(err, errx.KindUnavailable, CodeShutdownFailed, "优雅关闭失败")
	}
	return nil
}

// hasRoute 判断用户是否已注册指定路径（任意方法），用于跳过自动健康检查注册。
// 分组回调 panic 会转为错误返回，避免启动崩溃。
func (s *Server) hasRoute(path string) (bool, error) {
	for _, route := range s.routes {
		if route.Path == path {
			return true, nil
		}
	}
	for _, entry := range s.routeGroups {
		rg := &RouteGroup{prefix: entry.prefix}
		if err := callRouteGroupFn(entry.fn, rg); err != nil {
			return false, err
		}
		for _, route := range rg.flatten() {
			if route.Path == path {
				return true, nil
			}
		}
	}
	return false, nil
}

// callRouteGroupFn 执行路由分组回调，panic 转为错误。
func callRouteGroupFn(fn func(*RouteGroup), rg *RouteGroup) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("webx：路由分组回调 panic：%v", r)
		}
	}()
	fn(rg)
	return nil
}

// registerBuiltinMiddleware 按配置注册内置中间件。
func (s *Server) registerBuiltinMiddleware() {
	s.metrics = middleware.NewMetrics()
	s.mwManager.RegisterBuiltin("recovery", middleware.RecoveryWithOptions(s.logger, s.metrics, s.config.Debug))
	if !s.config.MiddlewareRecovery {
		s.mwManager.Disable("recovery")
	}
	s.mwManager.RegisterBuiltin("request_id", middleware.RequestIDWithOptions(middleware.RequestIDOptions{
		Header:    s.requestIDOpts.Header,
		Generator: s.requestIDOpts.Generator,
	}))
	if !s.config.MiddlewareRequestID {
		s.mwManager.Disable("request_id")
	}
	s.mwManager.RegisterBuiltin("body_limit", middleware.BodyLimit(s.config.MaxBodyBytes))
	if s.config.MaxBodyBytes <= 0 {
		s.mwManager.Disable("body_limit")
	}
	s.concurrencyLimiter = middleware.NewConcurrencyLimiter(s.maxConcurrent)
	s.mwManager.RegisterBuiltin("concurrency_limit", middleware.ConcurrencyLimit(s.concurrencyLimiter))
	if s.maxConcurrent <= 0 {
		s.mwManager.Disable("concurrency_limit")
	}
	s.mwManager.RegisterBuiltin("timeout", middleware.Timeout(s.config.RequestTimeout))
	if !s.config.MiddlewareTimeout {
		s.mwManager.Disable("timeout")
	}
	corsCfg := middleware.CORSConfig{
		AllowedOrigins:   s.config.CORSAllowedOrigins,
		AllowedMethods:   s.config.CORSAllowedMethods,
		AllowedHeaders:   s.config.CORSAllowedHeaders,
		ExposeHeaders:    s.config.CORSExposeHeaders,
		MaxAge:           int(s.config.CORSMaxAge.Seconds()),
		AllowCredentials: s.config.CORSAllowCredentials,
	}
	if len(corsCfg.AllowedOrigins) == 0 {
		corsCfg = middleware.DefaultCORSConfig()
		if len(s.config.CORSExposeHeaders) > 0 {
			corsCfg.ExposeHeaders = s.config.CORSExposeHeaders
		}
	}
	s.mwManager.RegisterBuiltin("cors", middleware.CORS(corsCfg))
	if !s.config.MiddlewareCORS {
		s.mwManager.Disable("cors")
	}
	s.mwManager.RegisterBuiltin("validation", middleware.Validation())
	if !s.config.MiddlewareValidation {
		s.mwManager.Disable("validation")
	}
	s.mwManager.RegisterBuiltin("security", middleware.SecurityHeaders(middleware.SecurityHeadersOptions{
		ContentTypeNoSniff:        true,
		FrameDeny:                 true,
		ReferrerPolicy:            s.config.SecurityReferrerPolicy,
		HSTSMaxAge:                time.Duration(s.config.SecurityHSTSMaxAge) * time.Second,
		PermissionsPolicy:         s.config.SecurityPermissionsPolicy,
		CrossOriginOpenerPolicy:   s.config.SecurityCrossOriginOpenerPolicy,
		CrossOriginResourcePolicy: s.config.SecurityCrossOriginResourcePolicy,
		CrossOriginEmbedderPolicy: s.config.SecurityCrossOriginEmbedderPolicy,
	}))
	if !s.config.MiddlewareSecurity {
		s.mwManager.Disable("security")
	}
	s.mwManager.RegisterBuiltin("gzip", middleware.GzipWithOptions(middleware.GzipOptions{
		MinSize: s.config.GzipMinSize,
		Level:   s.config.GzipLevel,
	}))
	if !s.config.MiddlewareGzip {
		s.mwManager.Disable("gzip")
	}
	s.mwManager.RegisterBuiltin("metrics", middleware.MetricsHandler(s.metrics))
	if !s.config.MiddlewareMetrics {
		s.mwManager.Disable("metrics")
	}
	s.mwManager.RegisterBuiltin("access_log", middleware.AccessLog(s.logger, middleware.AccessLogOptions{
		LogSuccess:    s.config.LogSuccessReq,
		SampleRate:    s.config.AccessLogSampleRate,
		RedactKeys:    s.config.AccessLogRedact,
		SlowThreshold: s.config.SlowRequestThreshold,
	}))
	if !s.config.AccessLogEnabled {
		s.mwManager.Disable("access_log")
	}
}

// addListener 注册 Listener。
func (s *Server) addListener(ln net.Listener) {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	s.listeners = append(s.listeners, ln)
}

// addHTTPServer 注册 http.Server。
func (s *Server) addHTTPServer(srv *http.Server) {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	s.httpServers = append(s.httpServers, srv)
}

// addQUICListener 注册 QUIC 监听器。
func (s *Server) addQUICListener(ln *quic.Listener) {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	s.quicListeners = append(s.quicListeners, ln)
}

// addHTTP3Server 注册 HTTP/3 服务实例（关闭时用于终止活动连接）。
func (s *Server) addHTTP3Server(srv *http3.Server) {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	s.http3Servers = append(s.http3Servers, srv)
}

// closeListeners 关闭已创建的监听器（启动失败回滚）。
func (s *Server) closeListeners() {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	for _, ln := range s.listeners {
		ln.Close()
	}
	for _, qln := range s.quicListeners {
		qln.Close()
	}
	for _, h3s := range s.http3Servers {
		_ = h3s.Close()
	}
}

// cancelCleanup 执行全部清理函数（幂等，用于启动失败路径回收后台 goroutine）。
func (s *Server) cancelCleanup() {
	for _, fn := range s.cleanupFuncs {
		if fn != nil {
			fn()
		}
	}
}

// warnStarted 记录启动后修改配置的警告。
func (s *Server) warnStarted(action string) {
	s.logWarn("webx：服务已启动，不允许" + action)
}

// logWarn 输出警告日志；logger 未注入时静默跳过。
func (s *Server) logWarn(msg string) {
	if s.logger != nil {
		s.logger.WithContext(context.Background()).Warn(msg, logx.Fields())
	}
}

// quicAccepter 抽象 QUIC 监听器的 Accept，便于测试注入异常。
type quicAccepter interface {
	Accept(context.Context) (*quic.Conn, error)
}

// serveHTTP3 在 QUIC Listener 上运行 HTTP/3 服务。
func serveHTTP3(ctx context.Context, h3s *http3.Server, qln quicAccepter) error {
	for {
		conn, err := qln.Accept(ctx)
		if err != nil {
			return err
		}
		go h3s.ServeQUICConn(conn)
	}
}

// logHTTP3Exit 记录 HTTP/3 服务退出日志：预期关闭记 Info，异常退出记 Error。
func logHTTP3Exit(logger logx.Logger, ctx context.Context, err error) {
	if errors.Is(err, quic.ErrServerClosed) {
		logger.WithContext(ctx).Info("webx：HTTP/3 服务已关闭", logx.Fields())
		return
	}
	logger.WithContext(ctx).Error("webx：HTTP/3 服务异常退出", logx.Fields(logx.Any("error", err)))
}
