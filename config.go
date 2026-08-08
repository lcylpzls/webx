package webx

import (
	"crypto/tls"
	"os"
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
// 校验规则与 ginx 对齐：证书/私钥必填且可配对、超时非负、日志级别合法。
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
	if c.IdleTimeout < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "空闲超时时间不能为负数")
	}
	if c.MaxHeaderBytes < 0 {
		return errx.New(errx.KindInvalid, CodeConfigInvalid, "最大请求头字节数不能为负数")
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
	if len(c.CORSAllowedOrigins) == 0 {
		c.CORSAllowedOrigins = []string{"*"}
		c.CORSAllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"}
		c.CORSAllowedHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"}
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
