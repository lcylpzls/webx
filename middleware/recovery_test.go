package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcylpzls/webx/internal/core"
)

func TestRecoveryPanic(t *testing.T) {
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		Recovery(),
		func(c *core.Context) { panic("boom") },
	})
	c.Run()
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("panic 应返回 500：%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "服务器内部错误") {
		t.Errorf("500 响应体不符：%s", rec.Body.String())
	}
	if _, ok := c.Get("recoveryError"); !ok {
		t.Error("recoveryError 应写入上下文")
	}
	if _, ok := c.Get("recoveryStack"); !ok {
		t.Error("recoveryStack 应写入上下文")
	}
}

func TestRecoveryNormal(t *testing.T) {
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		Recovery(),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	if rec.Code != http.StatusOK {
		t.Errorf("正常请求应通过：%d", rec.Code)
	}
	if _, ok := c.Get("recoveryError"); ok {
		t.Error("正常请求不应有 recoveryError")
	}
}

func TestRecoveryWithMetrics(t *testing.T) {
	m := NewMetrics()
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		RecoveryWithMetrics(m),
		func(c *core.Context) { panic("boom") },
	})
	c.Run()
	if got := m.Panics(); got != 1 {
		t.Errorf("panic 计数不符：%d", got)
	}
}
