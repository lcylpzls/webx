package webx

import (
	"context"
	"errors"
	testx "github.com/lcylpzls/testx"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lcylpzls/webx/internal/core"
)

func TestFormatUptime(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{5 * time.Second, "5秒"},
		{time.Minute, "1分钟"},
		{time.Minute + 5*time.Second, "1分钟5秒"},
		{time.Hour, "1小时"},
		{time.Hour + 5*time.Minute, "1小时5分钟"},
	}
	for _, tc := range cases {
		if got := formatUptime(tc.in); got != tc.want {
			t.Errorf("formatUptime(%v) 不符：got %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestHealthHandler(t *testing.T) {
	start := time.Now().Add(-2 * time.Minute)
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	c.SetHandlers([]core.HandlerFunc{healthHandler(start, nil, nil)})
	c.Run()
	testx.Equal(t, rec.Code, http.StatusOK)

	body := rec.Body.String()
	for _, want := range []string{"运行中", "2分钟", "code"} {
		testx.RequireTrue(t, strings.Contains(body, want))
	}
}

func TestHealthHandlerWithChecks(t *testing.T) {
	start := time.Now()
	checks := []healthCheck{
		{name: "db", fn: func(ctx context.Context) error { return nil }},
		{name: "redis", fn: func(ctx context.Context) error { return errors.New("连接失败") }},
	}
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	c.SetHandlers([]core.HandlerFunc{healthHandler(start, checks, nil)})
	c.Run()
	testx.Equal(t, rec.Code, http.StatusServiceUnavailable)

	body := rec.Body.String()
	if !strings.Contains(body, "异常") || !strings.Contains(body, "redis") || !strings.Contains(body, "连接失败") {
		t.Errorf("检查结果不符：%s", body)
	}
}

func TestHealthHandlerShuttingDown(t *testing.T) {
	start := time.Now().Add(-time.Minute)
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	c.SetHandlers([]core.HandlerFunc{healthHandler(start, nil, func() bool { return true })})
	c.Run()
	testx.Equal(t, rec.Code, http.StatusServiceUnavailable)

	if !strings.Contains(rec.Body.String(), "关闭中") {
		t.Errorf("关闭中响应体不符：%s", rec.Body.String())
	}
}
