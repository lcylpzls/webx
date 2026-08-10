package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lcylpzls/testx"
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
	testx.RequireEqual(t, rec.Header().Get("X-Content-Type-Options"), "nosniff")
	testx.RequireEqual(t, rec.Header().Get("X-Frame-Options"), "DENY")
	testx.RequireEqual(t, rec.Header().Get("Referrer-Policy"), "no-referrer")
	if rec.Header().Get("Strict-Transport-Security") != "max-age=3600; includeSubDomains; preload" {
		t.Errorf("HSTS 头不符：%s", rec.Header().Get("Strict-Transport-Security"))
	}
	testx.RequireEqual(t, rec.Header().Get("Permissions-Policy"), "camera=()")
	testx.RequireEqual(t, rec.Header().Get("Cross-Origin-Opener-Policy"), "same-origin")
	testx.RequireEqual(t, rec.Header().Get("Cross-Origin-Resource-Policy"), "same-origin")
	testx.RequireEqual(t, rec.Header().Get("Cross-Origin-Embedder-Policy"), "require-corp")
	testx.RequireEqual(t, rec.Header().Get("Content-Security-Policy"), "default-src 'self'")
	testx.RequireEqual(t, rec.Header().Get("Content-Security-Policy-Report-Only"), "default-src 'self'; report-uri /csp")
	testx.RequireEqual(t, rec.Header().Get("Origin-Agent-Cluster"), "?1")
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
