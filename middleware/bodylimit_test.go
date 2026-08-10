package middleware

import (
	testx "github.com/lcylpzls/testx"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcylpzls/webx/v2/internal/core"
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
	testx.Equal(t, rec.Code, http.StatusRequestEntityTooLarge)

	testx.True(t, aborted)

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
	testx.NotNil(t, readErr)

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
	testx.Equal(t, rec.Code, http.StatusOK)

}

func TestBodyLimitDisabledAndNilBody(t *testing.T) {
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		BodyLimit(0),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	testx.Equal(t, rec.Code, http.StatusOK)

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Body = nil
	c = core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{
		BodyLimit(10),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	testx.Equal(t, rec.Code, http.StatusOK)

}
