package webx

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcylpzls/errx"
)

func TestConfigValidateSuccessWithDefaults(t *testing.T) {
	cert, key := writeTestCert(t)
	cfg := Config{TLSCertFile: cert, TLSKeyFile: key}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate 失败：%v", err)
	}
	if cfg.LogLevel != "info" || cfg.HealthPath != "/health" ||
		cfg.LivenessPath != "/healthz" || cfg.ReadinessPath != "/readyz" {
		t.Errorf("默认值未填充：%+v", cfg)
	}
	if len(cfg.CORSAllowedOrigins) == 0 || len(cfg.CORSAllowedMethods) == 0 ||
		len(cfg.CORSExposeHeaders) == 0 || cfg.CORSExposeHeaders[0] != "X-Request-ID" {
		t.Error("CORS 默认值未填充")
	}
	if cfg.MaxBodyBytes != 10*1024*1024 {
		t.Errorf("MaxBodyBytes 默认值不符：%d", cfg.MaxBodyBytes)
	}
	if cfg.ReadHeaderTimeout != 10*time.Second || cfg.IdleTimeout != 60*time.Second {
		t.Errorf("超时默认值不符：%+v", cfg)
	}
	if cfg.MinTLSVersion != tls.VersionTLS12 || cfg.QUICMaxIdleTimeout != 30*time.Second || cfg.QUICMaxIncomingStreams != 100 {
		t.Errorf("TLS/QUIC 默认值不符：%+v", cfg)
	}
	if cfg.QUICDrainTimeout != 0 {
		t.Errorf("QUIC 排空默认值不符：%+v", cfg)
	}
}

func TestConfigValidateErrors(t *testing.T) {
	cert, key := writeTestCert(t)
	dir := t.TempDir()
	cases := []struct {
		name string
		cfg  Config
		msg  string
	}{
		{"证书为空", Config{}, "TLS 证书文件路径不能为空"},
		{"证书不存在", Config{TLSCertFile: filepath.Join(dir, "no.pem"), TLSKeyFile: key}, "文件不存在"},
		{"证书是目录", Config{TLSCertFile: dir, TLSKeyFile: key}, "路径是目录而非文件"},
		{"私钥为空", Config{TLSCertFile: cert}, "TLS 私钥文件路径不能为空"},
		{"私钥不存在", Config{TLSCertFile: cert, TLSKeyFile: filepath.Join(dir, "no.key")}, "文件不存在"},
		{"证书私钥不配对", Config{TLSCertFile: cert, TLSKeyFile: cert}, "TLS 证书加载失败"},
		{"关闭超时负数", Config{TLSCertFile: cert, TLSKeyFile: key, ShutdownTimeout: -1}, "关闭超时时间不能为负数"},
		{"请求超时负数", Config{TLSCertFile: cert, TLSKeyFile: key, RequestTimeout: -1}, "请求超时时间不能为负数"},
		{"读取超时负数", Config{TLSCertFile: cert, TLSKeyFile: key, ReadTimeout: -1}, "读取超时时间不能为负数"},
		{"写入超时负数", Config{TLSCertFile: cert, TLSKeyFile: key, WriteTimeout: -1}, "写入超时时间不能为负数"},
		{"读头超时负数", Config{TLSCertFile: cert, TLSKeyFile: key, ReadHeaderTimeout: -1}, "请求头读取超时时间不能为负数"},
		{"TLS 版本无效", Config{TLSCertFile: cert, TLSKeyFile: key, MinTLSVersion: 0x0301}, "最低 TLS 版本无效"},
		{"QUIC 空闲负数", Config{TLSCertFile: cert, TLSKeyFile: key, QUICMaxIdleTimeout: -1}, "QUIC 空闲超时不能为负数"},
		{"QUIC 流数负数", Config{TLSCertFile: cert, TLSKeyFile: key, QUICMaxIncomingStreams: -1}, "QUIC 最大入站流数不能为负数"},
		{"QUIC 排空负数", Config{TLSCertFile: cert, TLSKeyFile: key, QUICDrainTimeout: -1}, "QUIC 排空超时不能为负数"},
		{"空闲超时负数", Config{TLSCertFile: cert, TLSKeyFile: key, IdleTimeout: -1}, "空闲超时时间不能为负数"},
		{"请求头负数", Config{TLSCertFile: cert, TLSKeyFile: key, MaxHeaderBytes: -1}, "最大请求头字节数不能为负数"},
		{"请求体负数", Config{TLSCertFile: cert, TLSKeyFile: key, MaxBodyBytes: -1}, "最大请求体字节数不能为负数"},
		{"采样率负数", Config{TLSCertFile: cert, TLSKeyFile: key, AccessLogSampleRate: -1}, "访问日志采样率不能为负数"},
		{"gzip 最小负数", Config{TLSCertFile: cert, TLSKeyFile: key, GzipMinSize: -1}, "gzip 最小字节数不能为负数"},
		{"HSTS 负数", Config{TLSCertFile: cert, TLSKeyFile: key, SecurityHSTSMaxAge: -1}, "HSTS 缓存秒数不能为负数"},
		{"日志级别非法", Config{TLSCertFile: cert, TLSKeyFile: key, LogLevel: "verbose"}, "日志级别无效"},
	}
	for _, tc := range cases {
		err := tc.cfg.Validate()
		if err == nil {
			t.Errorf("%s：应返回错误", tc.name)
			continue
		}
		if !errx.Is(err, CodeConfigInvalid) {
			t.Errorf("%s：错误码不符：%v", tc.name, err)
		}
		if !strings.Contains(err.Error(), tc.msg) {
			t.Errorf("%s：错误消息不符：%v", tc.name, err)
		}
	}
}

func TestConfigValidateExplicitLevelAndCORS(t *testing.T) {
	cert, key := writeTestCert(t)
	cfg := Config{
		TLSCertFile:        cert,
		TLSKeyFile:         key,
		LogLevel:           "debug",
		CORSAllowedOrigins: []string{"https://example.com"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate 失败：%v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("显式日志级别被覆盖：%s", cfg.LogLevel)
	}
	if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "https://example.com" {
		t.Errorf("显式 CORS 配置被覆盖：%v", cfg.CORSAllowedOrigins)
	}
}

func TestConfigMetricsPath(t *testing.T) {
	cfg := validConfig(t)
	cfg.MetricsEnabled = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("启用指标端点应校验通过：%v", err)
	}
	if cfg.MetricsPath != "/metrics" {
		t.Errorf("默认指标路径不符：%s", cfg.MetricsPath)
	}
	cfg.MetricsPath = "metrics"
	if err := cfg.Validate(); err == nil {
		t.Error("指标路径不以 / 开头应报错")
	}
	cfg.MetricsPath = "/custom"
	cfg.MetricsEnabled = false
	if err := cfg.Validate(); err != nil {
		t.Errorf("未启用指标端点不应校验失败：%v", err)
	}
}

func TestConfigSlowRequestThreshold(t *testing.T) {
	cfg := validConfig(t)
	cfg.SlowRequestThreshold = -time.Second
	if err := cfg.Validate(); err == nil {
		t.Error("负的慢请求阈值应报错")
	}
	cfg.SlowRequestThreshold = time.Second
	if err := cfg.Validate(); err != nil {
		t.Errorf("合法慢请求阈值不应报错：%v", err)
	}
}

func TestConfigGzipLevel(t *testing.T) {
	for _, bad := range []int{-1, 10} {
		cfg := validConfig(t)
		cfg.GzipLevel = bad
		if err := cfg.Validate(); err == nil {
			t.Errorf("GzipLevel=%d 应报错", bad)
		}
	}
	cfg := validConfig(t)
	cfg.GzipLevel = 9
	if err := cfg.Validate(); err != nil {
		t.Errorf("GzipLevel=9 不应报错：%v", err)
	}
}

func TestConfigTrustedProxies(t *testing.T) {
	cfg := validConfig(t)
	cfg.TrustedProxies = []string{"10.0.0.0/8", "192.168.1.5"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("合法可信代理应通过：%v", err)
	}
	if len(cfg.trustedNets) != 2 {
		t.Fatalf("可信代理解析数量不符：%d", len(cfg.trustedNets))
	}
	cfg.TrustedProxies = []string{"bad-cidr"}
	if err := cfg.Validate(); err == nil {
		t.Error("非法可信代理应报错")
	}
}

func TestLoadConfigSuccess(t *testing.T) {
	cert, key := writeTestCert(t)
	path := filepath.Join(t.TempDir(), "config.toml")
	toml := `
tls_cert_file = ` + quote(cert) + `
tls_key_file = ` + quote(key) + `
read_timeout = "5s"
read_header_timeout = "8s"
request_timeout = "3s"
health_path = "/ready"
log_level = "debug"
middleware_request_id = true
middleware_recovery = true
middleware_gzip = true
middleware_metrics = true
access_log_enabled = true
access_log_sample_rate = 10
access_log_redact = ["token"]
access_log_headers = ["X-Trace-ID"]
min_tls_version = 771
quic_max_idle_timeout = "45s"
quic_max_incoming_streams = 200
quic_drain_timeout = "5s"
middleware_security = true
security_hsts_max_age = 3600
security_referrer_policy = "no-referrer"
security_permissions_policy = "camera=()"
security_cross_origin_opener_policy = "same-origin"
gzip_min_size = 2048
debug = true
cors_allow_credentials = true

[error_messages]
not_found = "页面不存在"
`
	if err := os.WriteFile(path, []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig 失败：%v", err)
	}
	if cfg.HealthPath != "/ready" || cfg.LogLevel != "debug" || cfg.ReadTimeout != 5*time.Second {
		t.Errorf("配置加载不符：%+v", cfg)
	}
	if cfg.ReadHeaderTimeout != 8*time.Second {
		t.Errorf("读头超时加载不符：%+v", cfg)
	}
	if !cfg.MiddlewareRequestID || !cfg.MiddlewareRecovery || !cfg.AccessLogEnabled {
		t.Errorf("开关配置不符：%+v", cfg)
	}
	if !cfg.MiddlewareGzip || !cfg.MiddlewareMetrics {
		t.Errorf("新增开关配置不符：%+v", cfg)
	}
	if cfg.AccessLogSampleRate != 10 || len(cfg.AccessLogRedact) != 1 || cfg.AccessLogRedact[0] != "token" {
		t.Errorf("访问日志配置不符：%+v", cfg)
	}
	if len(cfg.AccessLogHeaders) != 1 || cfg.AccessLogHeaders[0] != "X-Trace-ID" {
		t.Errorf("访问日志请求头白名单不符：%+v", cfg.AccessLogHeaders)
	}
	if cfg.ErrorMessages["not_found"] != "页面不存在" {
		t.Errorf("错误文案配置加载不符：%+v", cfg.ErrorMessages)
	}
	if cfg.MinTLSVersion != tls.VersionTLS12 || cfg.QUICMaxIdleTimeout != 45*time.Second || cfg.QUICMaxIncomingStreams != 200 {
		t.Errorf("TLS/QUIC 配置加载不符：%+v", cfg)
	}
	if cfg.QUICDrainTimeout != 5*time.Second {
		t.Errorf("QUIC 排空加载不符：%+v", cfg)
	}
	if !cfg.MiddlewareSecurity || cfg.SecurityHSTSMaxAge != 3600 ||
		cfg.SecurityReferrerPolicy != "no-referrer" || cfg.GzipMinSize != 2048 {
		t.Errorf("安全/gzip 配置加载不符：%+v", cfg)
	}
	if !cfg.Debug {
		t.Errorf("debug 配置加载不符：%+v", cfg)
	}
	if !cfg.CORSAllowCredentials {
		t.Errorf("CORS 凭据配置加载不符：%+v", cfg)
	}
	if cfg.SecurityPermissionsPolicy != "camera=()" || cfg.SecurityCrossOriginOpenerPolicy != "same-origin" {
		t.Errorf("安全头扩展配置加载不符：%+v", cfg)
	}
}

func TestLoadConfigErrors(t *testing.T) {
	cert, key := writeTestCert(t)
	dir := t.TempDir()

	// 文件不存在
	if _, err := LoadConfig(filepath.Join(dir, "missing.toml")); !errx.Is(err, CodeConfigLoadFailed) {
		t.Errorf("缺失文件错误码不符：%v", err)
	}
	// 非法 TOML
	bad := filepath.Join(dir, "bad.toml")
	_ = os.WriteFile(bad, []byte("tls_cert_file = ["), 0o600)
	if _, err := LoadConfig(bad); !errx.Is(err, CodeConfigLoadFailed) {
		t.Errorf("非法 TOML 错误码不符：%v", err)
	}
	// 未声明字段（confx 严格模式）
	unknown := filepath.Join(dir, "unknown.toml")
	_ = os.WriteFile(unknown, []byte("tls_cert_file = \"x\"\nunknown_key = 1\n"), 0o600)
	if _, err := LoadConfig(unknown); !errx.Is(err, CodeConfigLoadFailed) {
		t.Errorf("未声明字段错误码不符：%v", err)
	}
	// 加载成功但校验失败
	invalid := filepath.Join(dir, "invalid.toml")
	_ = os.WriteFile(invalid, []byte("tls_cert_file = \"x\"\ntls_key_file = \"y\"\n"), 0o600)
	if _, err := LoadConfig(invalid); !errx.Is(err, CodeConfigInvalid) {
		t.Errorf("校验失败错误码不符：%v", err)
	}
	// 成功路径快速确认
	ok := filepath.Join(dir, "ok.toml")
	_ = os.WriteFile(ok, []byte("tls_cert_file = "+quote(cert)+"\ntls_key_file = "+quote(key)+"\n"), 0o600)
	if _, err := LoadConfig(ok); err != nil {
		t.Errorf("合法配置加载失败：%v", err)
	}
}

func TestIsRegularFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.txt")
	_ = os.WriteFile(file, []byte("x"), 0o600)
	if err := isRegularFile(file); err != nil {
		t.Errorf("普通文件应通过：%v", err)
	}
	if err := isRegularFile(dir); err == nil {
		t.Error("目录应报错")
	}
	if err := isRegularFile(filepath.Join(dir, "missing")); err == nil {
		t.Error("缺失文件应报错")
	}
	parentFile := filepath.Join(dir, "p.txt")
	_ = os.WriteFile(parentFile, []byte("x"), 0o600)
	if err := isRegularFile(filepath.Join(parentFile, "child")); err == nil {
		t.Error("父路径为文件应报错")
	}
	// 注入 Stat 异常覆盖"无法访问文件"分支
	origStat := statPath
	statPath = func(string) (os.FileInfo, error) {
		return nil, os.ErrPermission
	}
	defer func() { statPath = origStat }()
	if err := isRegularFile(file); err == nil || !strings.Contains(err.Error(), "无法访问文件") {
		t.Errorf("Stat 异常应报无法访问：%v", err)
	}
}

// quote 返回 TOML 字符串字面量。
func quote(s string) string {
	return "\"" + strings.ReplaceAll(s, "\\", "\\\\") + "\""
}
