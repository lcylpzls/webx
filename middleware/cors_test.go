package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lcylpzls/webx/v2/internal/core"
)

func TestCORSAllowAll(t *testing.T) {
	cfg := DefaultCORSConfig()
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://a.com")
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{CORS(cfg), func(c *core.Context) { c.Success("ok", nil) }})
	c.Run()
	if rec.Code != http.StatusNoContent {
		t.Errorf("OPTIONS 预检应返回 204：%d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Allow-Origin 不符")
	}
	if rec.Header().Get("Access-Control-Max-Age") != "86400" {
		t.Error("Max-Age 不符")
	}
	if !c.IsAborted() {
		t.Error("预检应终止链")
	}
}

func TestCORSAllowedOrigin(t *testing.T) {
	cfg := CORSConfig{
		AllowedOrigins: []string{"https://trusted.com"},
		AllowedMethods: []string{"GET"},
		AllowedHeaders: []string{"X-Test"},
		MaxAge:         60,
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://trusted.com")
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{CORS(cfg), func(c *core.Context) { c.Success("ok", nil) }})
	c.Run()
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://trusted.com" {
		t.Error("可信来源未回显")
	}
	if rec.Header().Get("Vary") != "Origin" {
		t.Error("Vary 头不符")
	}
	if rec.Header().Get("Access-Control-Allow-Methods") != "GET" {
		t.Error("Methods 头不符")
	}
	if rec.Header().Get("Access-Control-Allow-Headers") != "X-Test" {
		t.Error("Headers 头不符")
	}
	if rec.Header().Get("Access-Control-Max-Age") != "60" {
		t.Error("Max-Age 头不符")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("非预检应放行：%d", rec.Code)
	}
}

func TestCORSDisallowedAndNoOrigin(t *testing.T) {
	cfg := CORSConfig{AllowedOrigins: []string{"https://trusted.com"}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{CORS(cfg), func(c *core.Context) { c.Success("ok", nil) }})
	c.Run()
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("不可信来源不应回显")
	}

	rec = httptest.NewRecorder()
	c = core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{CORS(cfg), func(c *core.Context) { c.Success("ok", nil) }})
	c.Run()
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("无 Origin 不应设置响应头")
	}
}

func TestCORSAllowCredentials(t *testing.T) {
	cfg := CORSConfig{
		AllowedOrigins:   []string{"https://trusted.com"},
		AllowCredentials: true,
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://trusted.com")
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{CORS(cfg), func(c *core.Context) { c.Success("ok", nil) }})
	c.Run()
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("凭据头缺失")
	}
}

func TestCORSExposeHeaders(t *testing.T) {
	cfg := CORSConfig{ExposeHeaders: []string{"X-Request-ID", "X-Trace-ID"}}
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{CORS(cfg), func(c *core.Context) { c.Success("ok", nil) }})
	c.Run()
	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "X-Request-ID, X-Trace-ID" {
		t.Errorf("Expose-Headers 不符：%s", got)
	}

	rec = httptest.NewRecorder()
	c = core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{CORS(DefaultCORSConfig()), func(c *core.Context) { c.Success("ok", nil) }})
	c.Run()
	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "X-Request-ID" {
		t.Errorf("默认 Expose-Headers 应含 X-Request-ID：%s", got)
	}

	rec = httptest.NewRecorder()
	c = core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{CORS(CORSConfig{}), func(c *core.Context) { c.Success("ok", nil) }})
	c.Run()
	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "" {
		t.Errorf("空 ExposeHeaders 不应设置响应头：%s", got)
	}
}

func TestCORSAllowPrivateNetwork(t *testing.T) {
	cfg := CORSConfig{AllowPrivateNetwork: true}
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{CORS(cfg), func(c *core.Context) { c.Success("ok", nil) }})
	c.Run()
	if got := rec.Header().Get("Access-Control-Allow-Private-Network"); got != "true" {
		t.Errorf("内网预检头缺失：%s", got)
	}

	rec = httptest.NewRecorder()
	c = core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{CORS(DefaultCORSConfig()), func(c *core.Context) { c.Success("ok", nil) }})
	c.Run()
	if got := rec.Header().Get("Access-Control-Allow-Private-Network"); got != "" {
		t.Errorf("默认不应输出内网预检头：%s", got)
	}
}
