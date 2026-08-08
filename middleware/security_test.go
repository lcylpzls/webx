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
			ContentTypeNoSniff:      true,
			FrameDeny:               true,
			ReferrerPolicy:          "no-referrer",
			HSTSMaxAge:              3600 * time.Second,
			PermissionsPolicy:       "camera=()",
			CrossOriginOpenerPolicy: "same-origin",
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
	if rec.Header().Get("Strict-Transport-Security") != "max-age=3600" {
		t.Errorf("HSTS 头不符：%s", rec.Header().Get("Strict-Transport-Security"))
	}
	if rec.Header().Get("Permissions-Policy") != "camera=()" {
		t.Error("Permissions-Policy 头缺失")
	}
	if rec.Header().Get("Cross-Origin-Opener-Policy") != "same-origin" {
		t.Error("Cross-Origin-Opener-Policy 头缺失")
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
	if rec.Header().Get("X-Content-Type-Options") != "" || rec.Header().Get("Strict-Transport-Security") != "" {
		t.Errorf("未启用项不应设置响应头：%v", rec.Header())
	}
}
