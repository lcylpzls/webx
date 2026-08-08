package webx

import (
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
	if cfg.LogLevel != "info" || cfg.HealthPath != "/health" {
		t.Errorf("默认值未填充：%+v", cfg)
	}
	if len(cfg.CORSAllowedOrigins) == 0 || len(cfg.CORSAllowedMethods) == 0 {
		t.Error("CORS 默认值未填充")
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
		{"空闲超时负数", Config{TLSCertFile: cert, TLSKeyFile: key, IdleTimeout: -1}, "空闲超时时间不能为负数"},
		{"请求头负数", Config{TLSCertFile: cert, TLSKeyFile: key, MaxHeaderBytes: -1}, "最大请求头字节数不能为负数"},
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

func TestLoadConfigSuccess(t *testing.T) {
	cert, key := writeTestCert(t)
	path := filepath.Join(t.TempDir(), "config.toml")
	toml := `
tls_cert_file = ` + quote(cert) + `
tls_key_file = ` + quote(key) + `
read_timeout = "5s"
request_timeout = "3s"
health_path = "/ready"
log_level = "debug"
middleware_request_id = true
middleware_recovery = true
access_log_enabled = true
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
	if !cfg.MiddlewareRequestID || !cfg.MiddlewareRecovery || !cfg.AccessLogEnabled {
		t.Errorf("开关配置不符：%+v", cfg)
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
