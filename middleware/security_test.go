package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lcylpzls/webx/internal/core"
)

func TestSecurityHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		SecurityHeaders(SecurityHeadersOptions{
			ContentTypeNoSniff:              true,
			FrameDeny:                       true,
			ReferrerPolicy:                  "no-referrer",
			HSTSMaxAge:                      3600 * time.Second,
			HSTSIncludeSubDomains:           true,
			HSTSPreload:                     true,
			PermissionsPolicy:               "camera=()",
			CrossOriginOpenerPolicy:         "same-origin",
			CrossOriginResourcePolicy:       "same-origin",
			CrossOriginEmbedderPolicy:       "require-corp",
			ContentSecurityPolicy:           "default-src 'self'",
			ContentSecurityPolicyReportOnly: "default-src 'self'; report-uri /csp",
			OriginAgentCluster:              true,
		}),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("nosniff 头缺失")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("frame deny 头缺失")
	}
	if rec.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Error("referrer 头缺失")
	}
	if rec.Header().Get("Strict-Transport-Security") != "max-age=3600; includeSubDomains; preload" {
		t.Errorf("HSTS 头不符：%s", rec.Header().Get("Strict-Transport-Security"))
	}
	if rec.Header().Get("Permissions-Policy") != "camera=()" {
		t.Error("Permissions-Policy 头缺失")
	}
	if rec.Header().Get("Cross-Origin-Opener-Policy") != "same-origin" {
		t.Error("Cross-Origin-Opener-Policy 头缺失")
	}
	if rec.Header().Get("Cross-Origin-Resource-Policy") != "same-origin" {
		t.Error("Cross-Origin-Resource-Policy 头缺失")
	}
	if rec.Header().Get("Cross-Origin-Embedder-Policy") != "require-corp" {
		t.Error("Cross-Origin-Embedder-Policy 头缺失")
	}
	if rec.Header().Get("Content-Security-Policy") != "default-src 'self'" {
		t.Error("Content-Security-Policy 头缺失")
	}
	if rec.Header().Get("Content-Security-Policy-Report-Only") != "default-src 'self'; report-uri /csp" {
		t.Error("Content-Security-Policy-Report-Only 头缺失")
	}
	if rec.Header().Get("Origin-Agent-Cluster") != "?1" {
		t.Error("Origin-Agent-Cluster 头缺失")
	}
}

func TestSecurityHeadersDisabled(t *testing.T) {
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		SecurityHeaders(SecurityHeadersOptions{}),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	if rec.Header().Get("X-Content-Type-Options") != "" || rec.Header().Get("Strict-Transport-Security") != "" ||
		rec.Header().Get("Cross-Origin-Resource-Policy") != "" || rec.Header().Get("Cross-Origin-Embedder-Policy") != "" ||
		rec.Header().Get("Content-Security-Policy") != "" || rec.Header().Get("Origin-Agent-Cluster") != "" {
		t.Errorf("未启用项不应设置响应头：%v", rec.Header())
	}
}
