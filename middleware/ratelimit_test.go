package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lcylpzls/webx/internal/core"
)

func TestRateLimiterAllowAndLimit(t *testing.T) {
	rl := NewRateLimiter(1, time.Second, nil)
	if !rl.Allow("1.2.3.4") {
		t.Error("首次请求应允许")
	}
	if rl.Allow("1.2.3.4") {
		t.Error("超限请求应拒绝")
	}
	if !rl.Allow("5.6.7.8") {
		t.Error("不同 IP 应允许")
	}
}

func TestRateLimiterTokenCap(t *testing.T) {
	rl := NewRateLimiter(100, time.Second, nil)
	if !rl.Allow("1.1.1.1") {
		t.Fatal("首次请求应允许")
	}
	time.Sleep(20 * time.Millisecond)
	if !rl.Allow("1.1.1.1") {
		t.Error("恢复令牌后应允许（触发令牌上限分支）")
	}
}

func TestRateLimiterWhitelist(t *testing.T) {
	rl := NewRateLimiter(0, time.Second, []string{"10.0.0.0/8", "192.168.1.5", "bad-cidr"})
	if !rl.Allow("10.1.2.3") {
		t.Error("CIDR 白名单应放行")
	}
	if !rl.Allow("192.168.1.5") {
		t.Error("单 IP 白名单应放行")
	}
	// qps=0 时非白名单必被拒
	if rl.Allow("8.8.8.8") {
		t.Error("非白名单应被拒")
	}
}

func TestRateLimiterCleanup(t *testing.T) {
	rl := NewRateLimiter(10, time.Second, nil)
	rl.Allow("1.1.1.1")
	time.Sleep(time.Millisecond)
	rl.Cleanup(50 * time.Microsecond)
	rl.mu.Lock()
	_, ok := rl.buckets["1.1.1.1"]
	rl.mu.Unlock()
	if ok {
		t.Error("过期桶应被清理")
	}
	// 活跃桶不应被清理
	rl.Allow("2.2.2.2")
	rl.Cleanup(time.Hour)
	rl.mu.Lock()
	_, ok = rl.buckets["2.2.2.2"]
	rl.mu.Unlock()
	if !ok {
		t.Error("活跃桶不应被清理")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := NewRateLimiter(1, time.Second, nil)
	run := func(ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		c := core.NewContext(rec, req)
		c.SetHandlers([]core.HandlerFunc{RateLimit(rl), func(c *core.Context) { c.Success("ok", nil) }})
		c.Run()
		return rec.Code
	}
	if got := run("1.1.1.1"); got != http.StatusOK {
		t.Errorf("首次应放行：%d", got)
	}
	if got := run("1.1.1.1"); got != http.StatusTooManyRequests {
		t.Errorf("超限应 429：%d", got)
	}
}

func TestExtractClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "9.9.9.9:80"
	req.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2")
	c := core.NewContext(httptest.NewRecorder(), req)
	if got := extractClientIP(c); got != "1.1.1.1" {
		t.Errorf("XFF 提取不符：%s", got)
	}
	req.Header.Del("X-Forwarded-For")
	req.Header.Set("X-Real-IP", "3.3.3.3")
	if got := extractClientIP(c); got != "3.3.3.3" {
		t.Errorf("X-Real-IP 提取不符：%s", got)
	}
	req.Header.Del("X-Real-IP")
	if got := extractClientIP(c); got != "9.9.9.9" {
		t.Errorf("RemoteAddr 提取不符：%s", got)
	}
	req.RemoteAddr = "no-port"
	if got := extractClientIP(c); got != "no-port" {
		t.Errorf("无端口 RemoteAddr 不符：%s", got)
	}
}
