package middleware

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/lcylpzls/webx/internal/core"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

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
				t.Errorf("生成的 UUID v4 格式不符：%s", c.RequestID())
			}
			if got := req.Header.Get("X-Request-ID"); got != c.RequestID() {
				t.Errorf("出站请求头未透传：%s", got)
			}
		},
	})
	c.Run()
}

func TestNewUUIDV4(t *testing.T) {
	for i := 0; i < 100; i++ {
		id := newUUIDV4()
		if !uuidPattern.MatchString(id) {
			t.Fatalf("newUUIDV4 格式不符：%s", id)
		}
	}
}
