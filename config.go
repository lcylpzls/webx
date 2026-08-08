package webx

import (
	"crypto/tls"
	"os"
	"strings"
	"time"

	"github.com/lcylpzls/confx"
	"github.com/lcylpzls/errx"
)

// statPath 是可注入的文件状态查询函数（测试可替换以覆盖异常分支）。
var statPath = os.Stat

// Config 定义 webx Server 的全部配置项，通过 confx 从 TOML 文件加载。
// 所有校验在 Validate() 中集中进行，失败返回 errx 结构化错误。
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

// isRegularFile 检查给定路径是否存在且为普通文件（非目录）。
func isRegularFile(path string) error {
	info, err := statPath(path)
	if err != nil {
		if os.IsNotExist(err) {
			return errx.Newf(errx.KindInvalid, CodeConfigInvalid, "文件不存在：%s", path)
		}
		return errx.Wrap(err, errx.KindInvalid, CodeConfigInvalid, "无法访问文件："+path)
	}
	if info.IsDir() {
		return errx.Newf(errx.KindInvalid, CodeConfigInvalid, "路径是目录而非文件：%s", path)
	}
	return nil
}

// Validate 校验配置完整性并填充默认值。
// 校验规则：证书/私钥必填且可配对、超时非负、日志级别合法。
func (c *Config) Validate() error {
	if c.TLSCertFile == "" {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "TLS 证书文件路径不能为空")
	}
	if err := isRegularFile(c.TLSCertFile); err != nil {
		return err
	}
	if c.TLSKeyFile == "" {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "TLS 私钥文件路径不能为空")
	}
	if err := isRegularFile(c.TLSKeyFile); err != nil {
		return err
	}
	if _, err := tls.LoadX509KeyPair(c.TLSCertFile, c.TLSKeyFile); err != nil {
		return errx.Wrap(err, errx.KindInvalid, CodeConfigInvalid, "TLS 证书加载失败")
	}
	if c.MinTLSVersion == 0 {
		c.MinTLSVersion = tls.VersionTLS12
	}
	switch c.MinTLSVersion {
	case tls.VersionTLS12, tls.VersionTLS13:
	default:
		return errx.Newf(errx.KindInvalid, CodeConfigInvalid, "最低 TLS 版本无效：%d，必须是 TLS1.2 或 TLS1.3", c.MinTLSVersion)
	}
	if c.ShutdownTimeout < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "关闭超时时间不能为负数")
	}
	if c.RequestTimeout < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "请求超时时间不能为负数")
	}
	if c.ReadTimeout < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "读取超时时间不能为负数")
	}
	if c.WriteTimeout < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "写入超时时间不能为负数")
	}
	if c.ReadHeaderTimeout < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "请求头读取超时时间不能为负数")
	}
	if c.IdleTimeout < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "空闲超时时间不能为负数")
	}
	if c.MaxHeaderBytes < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "最大请求头字节数不能为负数")
	}
	if c.MaxBodyBytes < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "最大请求体字节数不能为负数")
	}
	if c.AccessLogSampleRate < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "访问日志采样率不能为负数")
	}
	if c.SlowRequestThreshold < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "慢请求阈值不能为负数")
	}
	if c.GzipMinSize < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "gzip 最小字节数不能为负数")
	}
	if c.GzipLevel < 0 || c.GzipLevel > 9 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "gzip 压缩级别必须在 0-9 之间")
	}
	if c.SecurityHSTSMaxAge < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "HSTS 缓存秒数不能为负数")
	}
	if c.MaxBodyBytes == 0 {
		c.MaxBodyBytes = 10 * 1024 * 1024
	}
	if c.QUICMaxIdleTimeout < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "QUIC 空闲超时不能为负数")
	}
	if c.QUICMaxIdleTimeout == 0 {
		c.QUICMaxIdleTimeout = 30 * time.Second
	}
	if c.QUICMaxIncomingStreams < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "QUIC 最大入站流数不能为负数")
	}
	if c.QUICMaxIncomingStreams == 0 {
		c.QUICMaxIncomingStreams = 100
	}
	if c.QUICDrainTimeout < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "QUIC 排空超时不能为负数")
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return errx.Newf(errx.KindInvalid, CodeConfigInvalid, "日志级别无效：%s，必须是 debug、info、warn 或 error", c.LogLevel)
	}
	if c.HealthPath == "" {
		c.HealthPath = "/health"
	}
	if c.LivenessPath == "" {
		c.LivenessPath = "/healthz"
	}
	if c.ReadinessPath == "" {
		c.ReadinessPath = "/readyz"
	}
	if c.MetricsEnabled && c.MetricsPath == "" {
		c.MetricsPath = "/metrics"
	}
	if c.MetricsPath != "" && !strings.HasPrefix(c.MetricsPath, "/") {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "指标端点路径必须以 / 开头")
	}
	if c.ReadHeaderTimeout == 0 {
		c.ReadHeaderTimeout = 10 * time.Second
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = 60 * time.Second
	}
	if len(c.CORSAllowedOrigins) == 0 {
		c.CORSAllowedOrigins = []string{"*"}
		c.CORSAllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"}
		c.CORSAllowedHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"}
		c.CORSExposeHeaders = []string{"X-Request-ID"}
	}
	return nil
}

// LoadConfig 通过 confx 从 TOML 文件加载配置并校验。
// 文件不存在、TOML 非法、存在未声明字段或校验失败时返回 errx 错误。
func LoadConfig(path string) (Config, error) {
	var cfg Config
	if err := confx.Load(path, &cfg); err != nil {
		return cfg, errx.Wrap(err, errx.KindUnavailable, CodeConfigLoadFailed, "加载配置文件失败")
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}
