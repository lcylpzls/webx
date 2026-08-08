package webx

import (
	"context"
	"errors"
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
	c.SetHandlers([]core.HandlerFunc{healthHandler(start, nil)})
	c.Run()
	if rec.Code != http.StatusOK {
		t.Errorf("健康检查状态不符：%d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"运行中", "2分钟", "code"} {
		if !strings.Contains(body, want) {
			t.Errorf("健康检查响应缺少 %s：%s", want, body)
		}
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
	c.SetHandlers([]core.HandlerFunc{healthHandler(start, checks)})
	c.Run()
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("检查失败应 503：%d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "异常") || !strings.Contains(body, "redis") || !strings.Contains(body, "连接失败") {
		t.Errorf("检查结果不符：%s", body)
	}
}
