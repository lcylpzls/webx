package middleware

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/lcylpzls/webx/internal/core"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestRequestIDFromHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "custom-id")
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{
		RequestID(),
		func(c *core.Context) {
			if c.RequestID() != "custom-id" {
				t.Errorf("请求 ID 不符：%s", c.RequestID())
			}
			if rec.Header().Get("X-Request-ID") != "custom-id" {
				t.Error("响应头未透传")
			}
		},
	})
	c.Run()
}

func TestRequestIDGenerated(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{
		RequestID(),
		func(c *core.Context) {
			if !uuidPattern.MatchString(c.RequestID()) {
				t.Errorf("生成的 UUID v7 格式不符：%s", c.RequestID())
			}
			if got := req.Header.Get("X-Request-ID"); got != c.RequestID() {
				t.Errorf("出站请求头未透传：%s", got)
			}
		},
	})
	c.Run()
}

func TestRequestIDWithOptions(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{
		RequestIDWithOptions(RequestIDOptions{
			Header:    "X-Trace-ID",
			Generator: func() string { return "trace-123" },
		}),
		func(c *core.Context) {
			if c.RequestID() != "trace-123" {
				t.Errorf("自定义生成器未生效：%s", c.RequestID())
			}
			if rec.Header().Get("X-Trace-ID") != "trace-123" {
				t.Error("自定义头未写入响应")
			}
			if got := req.Header.Get("X-Trace-ID"); got != "trace-123" {
				t.Errorf("自定义头未透传上游：%s", got)
			}
		},
	})
	c.Run()

	// 入站请求头优先
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Trace-ID", "incoming-id")
	c = core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{
		RequestIDWithOptions(RequestIDOptions{Header: "X-Trace-ID"}),
		func(c *core.Context) {
			if c.RequestID() != "incoming-id" {
				t.Errorf("入站自定义头未优先：%s", c.RequestID())
			}
		},
	})
	c.Run()
}

func TestNewUUIDV7(t *testing.T) {
	for i := 0; i < 100; i++ {
		id := newUUIDV7()
		if !uuidPattern.MatchString(id) {
			t.Fatalf("newUUIDV7 格式不符：%s", id)
		}
	}
}
