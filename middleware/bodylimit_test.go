package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcylpzls/webx/internal/core"
)

func TestBodyLimitOverContentLength(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 100)))
	c := core.NewContext(rec, req)
	aborted := true
	c.SetHandlers([]core.HandlerFunc{
		BodyLimit(10),
		func(c *core.Context) { aborted = false },
	})
	c.Run()
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("超限应 413：%d", rec.Code)
	}
	if !aborted {
		t.Error("超限应终止链")
	}
}

func TestBodyLimitCustomMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 100)))
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{
		BodyLimitWithOptions(10, BodyLimitOptions{Message: "自定义超限文案"}),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	if rec.Code != http.StatusRequestEntityTooLarge || !strings.Contains(rec.Body.String(), "自定义超限文案") {
		t.Errorf("自定义文案未生效：%d %s", rec.Code, rec.Body.String())
	}
}

func TestBodyLimitChunkedWrapped(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 100)))
	req.ContentLength = -1 // 模拟 chunked 传输
	c := core.NewContext(rec, req)
	readErr := error(nil)
	c.SetHandlers([]core.HandlerFunc{
		BodyLimit(10),
		func(c *core.Context) {
			_, readErr = io.ReadAll(c.Request().Body)
		},
	})
	c.Run()
	if readErr == nil {
		t.Error("chunked 超限读取应报错")
	}
}

func TestBodyLimitUnderLimit(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("ok"))
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{
		BodyLimit(10),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	if rec.Code != http.StatusOK {
		t.Errorf("未超限应放行：%d", rec.Code)
	}
}

func TestBodyLimitDisabledAndNilBody(t *testing.T) {
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		BodyLimit(0),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	if rec.Code != http.StatusOK {
		t.Errorf("禁用时应放行：%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Body = nil
	c = core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{
		BodyLimit(10),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	if rec.Code != http.StatusOK {
		t.Errorf("nil body 应放行：%d", rec.Code)
	}
}
