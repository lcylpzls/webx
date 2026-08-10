package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/lcylpzls/testx"
	"github.com/lcylpzls/webx/internal/core"
)

var requestIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

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
			if !requestIDPattern.MatchString(c.RequestID()) {
				t.Errorf("生成的随机请求 ID 格式不符：%s", c.RequestID())
			}
			testx.RequireEqual(t, req.Header.Get("X-Request-ID"), c.RequestID())
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
			testx.RequireEqual(t, req.Header.Get("X-Trace-ID"), "trace-123")
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

func TestNewRequestID(t *testing.T) {
	for i := 0; i < 100; i++ {
		id := newRequestID()
		if !requestIDPattern.MatchString(id) {
			t.Fatalf("newRequestID 格式不符：%s", id)
		}
	}
}

func TestNewRequestIDFallback(t *testing.T) {
	orig := requestIDRand
	requestIDRand = func(int) (string, error) { return "", errors.New("随机源故障") }
	defer func() { requestIDRand = orig }()
	id := newRequestID()
	if !strings.HasPrefix(id, "req-") {
		t.Fatalf("随机源失败应回退时间戳前缀：%s", id)
	}
}
