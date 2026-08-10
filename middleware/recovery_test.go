package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcylpzls/logx"
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
	m := NewMetrics(nil)
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

func TestRecoveryWithLogger(t *testing.T) {
	var buf bytes.Buffer
	logger, err := logx.NewBuilder().EnableWriter(&buf, logx.ErrorLevel).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "req-1")
	c := core.NewContext(rec, req)
	c.Set("requestId", "req-1")
	c.SetHandlers([]core.HandlerFunc{
		RecoveryWith(logger, nil),
		func(c *core.Context) { panic("boom") },
	})
	c.Run()
	if !strings.Contains(buf.String(), "请求处理发生 panic") || !strings.Contains(buf.String(), "boom") {
		t.Errorf("panic 日志缺失：%s", buf.String())
	}
	if !strings.Contains(buf.String(), "req-1") || !strings.Contains(buf.String(), "goroutine") {
		t.Errorf("panic 日志应含 requestId 与调用栈：%s", buf.String())
	}
}

func TestRecoveryWithOptionsDebug(t *testing.T) {
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		RecoveryWithOptions(nil, nil, true),
		func(c *core.Context) { panic("boom") },
	})
	c.Run()
	if !strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("debug 模式响应应含 panic 摘要：%s", rec.Body.String())
	}
}
