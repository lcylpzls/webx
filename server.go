// Package webx 提供基于 Go 标准库的工业级 HTTP/HTTPS 服务组件库。
// 路由基于 http.ServeMux，上下文与中间件链自研，日志/错误/配置
// 分别接入 logx / errx / confx，HTTP/3 使用 quic-go。
package webx

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
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
	config        Config
	logger        logx.Logger
	routes        []Route
	routeGroups   []routeGroupEntry
	staticEntries []staticEntry
	spa           *spaConfig
	mwManager     *middleware.Manager
	rateLimiter   *middleware.RateLimiter
	router        *Router
	startTime     time.Time
	started       bool
	mu            sync.Mutex
	shutdownOnce  sync.Once
	cleanupFuncs  []func()
	signalCtx     context.Context
	signalCancel  context.CancelFunc

	listeners     []net.Listener
	quicListeners []*quic.Listener
	http3Servers  []*http3.Server
	listenersMu   sync.Mutex
	httpServers   []*http.Server

	http2Enabled   bool
	http2Addr      string
	http3Enabled   bool
	http3Addr      string
	unixEnabled    bool
	unixSocketPath string
	unixSocketPerm os.FileMode
}

// checkUnixSocket 是可注入的 Unix Socket 平台检查（测试可替换）。
var checkUnixSocket = unixSocketSupported

// NewServer 创建 webx Server 实例。
func NewServer(cfg Config) *Server {
	return &Server{
		config:    cfg,
		logger:    DefaultLogger(parseLogLevel(cfg.LogLevel)),
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

// ListenerAddr 返回第一个 Listener 的监听地址（port 0 动态端口时可用）。
func (s *Server) ListenerAddr() string {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	if len(s.listeners) > 0 {
		return s.listeners[0].Addr().String()
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

	noRoute := core.NoRouteHandler
	if s.spa != nil {
		noRoute = spaNoRoute(s.spa.fs, s.spa.indexPath)
	}
	s.router = NewRouter(noRoute, core.NoMethodHandler)

	ctx := context.Background()
	globalChain := s.mwManager.Build(ctx)
	buildChain := func(route Route) []core.HandlerFunc {
		chain := append([]core.HandlerFunc{}, globalChain...)
		chain = append(chain, route.Middleware...)
		chain = append(chain, route.Handler)
		return chain
	}

	for _, entry := range s.staticEntries {
		if err := s.router.HandleStatic(entry.prefix, entry.fs); err != nil {
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
		entry.fn(rg)
		for _, route := range rg.flatten() {
			if err := s.router.Handle(route.Method, route.Path, buildChain(route)); err != nil {
				return errx.Wrap(err, errx.KindInvalid, CodeStartFailed, "路由注册失败："+route.Method+" "+route.Path)
			}
		}
	}
	if !s.hasRoute(s.config.HealthPath) {
		if err := s.router.Handle("GET", s.config.HealthPath, []core.HandlerFunc{healthHandler(s.startTime)}); err != nil {
			return errx.Wrap(err, errx.KindInvalid, CodeStartFailed, "健康检查路由注册失败")
		}
	}

	var wg sync.WaitGroup
	if s.http2Enabled {
		ln, err := createTLSListener(s.http2Addr, s.config.TLSCertFile, s.config.TLSKeyFile)
		if err != nil {
			s.closeListeners()
			return err
		}
		s.addListener(ln)
		srv := &http.Server{
			Handler:        s.router,
			ReadTimeout:    s.config.ReadTimeout,
			WriteTimeout:   s.config.WriteTimeout,
			IdleTimeout:    s.config.IdleTimeout,
			MaxHeaderBytes: s.config.MaxHeaderBytes,
		}
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
		qln, err := createQUICListener(s.http3Addr, s.config.TLSCertFile, s.config.TLSKeyFile)
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
				s.logger.WithContext(ctx).Error("webx：HTTP/3 服务异常退出", logx.Fields(logx.Any("error", err)))
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
			Handler:        s.router,
			ReadTimeout:    s.config.ReadTimeout,
			WriteTimeout:   s.config.WriteTimeout,
			IdleTimeout:    s.config.IdleTimeout,
			MaxHeaderBytes: s.config.MaxHeaderBytes,
		}
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
	for _, h3s := range h3Servers {
		_ = h3s.Close()
	}
	if err != nil {
		return errx.Wrap(err, errx.KindUnavailable, CodeShutdownFailed, "优雅关闭失败")
	}
	return nil
}

// hasRoute 判断用户是否已注册指定路径（任意方法），用于跳过自动健康检查注册。
func (s *Server) hasRoute(path string) bool {
	for _, route := range s.routes {
		if route.Path == path {
			return true
		}
	}
	for _, entry := range s.routeGroups {
		rg := &RouteGroup{prefix: entry.prefix}
		entry.fn(rg)
		for _, route := range rg.flatten() {
			if route.Path == path {
				return true
			}
		}
	}
	return false
}

// registerBuiltinMiddleware 按配置注册内置中间件。
func (s *Server) registerBuiltinMiddleware() {
	s.mwManager.RegisterBuiltin("recovery", middleware.Recovery())
	if !s.config.MiddlewareRecovery {
		s.mwManager.Disable("recovery")
	}
	s.mwManager.RegisterBuiltin("request_id", middleware.RequestID())
	if !s.config.MiddlewareRequestID {
		s.mwManager.Disable("request_id")
	}
	s.mwManager.RegisterBuiltin("timeout", middleware.Timeout(s.config.RequestTimeout))
	if !s.config.MiddlewareTimeout {
		s.mwManager.Disable("timeout")
	}
	corsCfg := middleware.CORSConfig{
		AllowedOrigins: s.config.CORSAllowedOrigins,
		AllowedMethods: s.config.CORSAllowedMethods,
		AllowedHeaders: s.config.CORSAllowedHeaders,
		MaxAge:         int(s.config.CORSMaxAge.Seconds()),
	}
	if len(corsCfg.AllowedOrigins) == 0 {
		corsCfg = middleware.DefaultCORSConfig()
	}
	s.mwManager.RegisterBuiltin("cors", middleware.CORS(corsCfg))
	if !s.config.MiddlewareCORS {
		s.mwManager.Disable("cors")
	}
	s.mwManager.RegisterBuiltin("validation", middleware.Validation())
	if !s.config.MiddlewareValidation {
		s.mwManager.Disable("validation")
	}
	s.mwManager.RegisterBuiltin("access_log", middleware.AccessLog(s.logger, s.config.LogSuccessReq))
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
	s.logger.WithContext(context.Background()).Warn("webx：服务已启动，不允许"+action, logx.Fields())
}

// serveHTTP3 在 QUIC Listener 上运行 HTTP/3 服务。
func serveHTTP3(ctx context.Context, h3s *http3.Server, qln *quic.Listener) error {
	for {
		conn, err := qln.Accept(ctx)
		if err != nil {
			return err
		}
		go h3s.ServeQUICConn(conn)
	}
}
