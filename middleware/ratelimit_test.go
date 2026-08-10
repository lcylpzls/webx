package middleware

import (
	testx "github.com/lcylpzls/testx"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lcylpzls/webx/v2/internal/core"
)

func TestRateLimiterAllowAndLimit(t *testing.T) {
	rl := NewRateLimiter(1, time.Second, nil)
	if !rl.Allow("1.2.3.4") {
		t.Error("首次请求应允许")
	}
	if rl.Allow("1.2.3.4") {
		t.Error("超限请求应拒绝")
	}
	if got := rl.Rejected(); got != 1 {
		t.Errorf("拒绝计数不符：%d", got)
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

func TestRateLimiterRetryAfter(t *testing.T) {
	rl := NewRateLimiter(2, time.Second, nil)
	if got := rl.RetryAfter("1.1.1.1"); got != 0 {
		t.Errorf("无桶应返回 0：%v", got)
	}
	rl.Allow("1.1.1.1") // tokens 2→1
	if got := rl.RetryAfter("1.1.1.1"); got != 0 {
		t.Errorf("令牌充足应返回 0：%v", got)
	}
	rl.Allow("1.1.1.1") // tokens 1→0
	if got := rl.RetryAfter("1.1.1.1"); got != time.Second {
		t.Errorf("缺 1 枚令牌应返回 1s：%v", got)
	}
	rlZero := NewRateLimiter(0, time.Second, nil)
	rlZero.Allow("8.8.8.8")
	if got := rlZero.RetryAfter("8.8.8.8"); got != 0 {
		t.Errorf("qps=0 应返回 0：%v", got)
	}
}

func TestRateLimiterMaxBuckets(t *testing.T) {
	rl := NewRateLimiter(10, time.Second, nil)
	rl.SetMaxBuckets(1)
	if !rl.Allow("1.1.1.1") {
		t.Fatal("首个 IP 应允许")
	}
	if rl.Allow("2.2.2.2") {
		t.Error("超出桶上限应拒绝")
	}
	if got := rl.Rejected(); got != 1 {
		t.Errorf("拒绝计数不符：%d", got)
	}
	rl.SetMaxBuckets(0) // 无效值不生效
	if rl.Allow("3.3.3.3") {
		t.Error("无效上限设置后应保持原上限（新 IP 拒绝）")
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
	testx.False(t, ok)

	// 活跃桶不应被清理
	rl.Allow("2.2.2.2")
	rl.Cleanup(time.Hour)
	rl.mu.Lock()
	_, ok = rl.buckets["2.2.2.2"]
	rl.mu.Unlock()
	testx.True(t, ok)

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
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.1.1.1:1234"
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{RateLimit(rl), func(c *core.Context) { c.Success("ok", nil) }})
	c.Run()
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After 头不符：%s", got)
	}
}

func TestRateLimitCustomMessage(t *testing.T) {
	rl := NewRateLimiter(1, time.Second, nil)
	rl.SetRejectMessage("自定义限流文案")
	run := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.1.1.1:1234"
		rec := httptest.NewRecorder()
		c := core.NewContext(rec, req)
		c.SetHandlers([]core.HandlerFunc{RateLimit(rl), func(c *core.Context) { c.Success("ok", nil) }})
		c.Run()
		return rec
	}
	run()
	rec := run()
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), "自定义限流文案") {
		t.Errorf("自定义文案未生效：%d %s", rec.Code, rec.Body.String())
	}
}

func TestRateLimitMiddlewareNoRetryAfter(t *testing.T) {
	rl := NewRateLimiter(10, time.Second, nil)
	rl.SetMaxBuckets(1)
	run := func(ip string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		c := core.NewContext(rec, req)
		c.SetHandlers([]core.HandlerFunc{RateLimit(rl), func(c *core.Context) { c.Success("ok", nil) }})
		c.Run()
		return rec
	}
	run("1.1.1.1")
	rec := run("2.2.2.2")
	testx.Equal(t, rec.Code, http.StatusTooManyRequests)

	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Errorf("无桶时不应设置 Retry-After：%s", got)
	}
}

func TestRateLimitKeyFunc(t *testing.T) {
	rl := NewRateLimiter(1, time.Second, nil)
	rl.SetKeyFunc(func(c *core.Context) string { return c.Query("user") })
	run := func(user string) int {
		req := httptest.NewRequest(http.MethodGet, "/?user="+user, nil)
		req.RemoteAddr = "1.1.1.1:80"
		rec := httptest.NewRecorder()
		c := core.NewContext(rec, req)
		c.SetHandlers([]core.HandlerFunc{RateLimit(rl), func(c *core.Context) { c.Success("ok", nil) }})
		c.Run()
		return rec.Code
	}
	if got := run("a"); got != http.StatusOK {
		t.Errorf("用户 a 首次应放行：%d", got)
	}
	if got := run("a"); got != http.StatusTooManyRequests {
		t.Errorf("用户 a 再次应限流：%d", got)
	}
	if got := run("b"); got != http.StatusOK {
		t.Errorf("不同用户同 IP 应放行：%d", got)
	}
}

func TestRateLimitTrustedProxy(t *testing.T) {
	run := func(rl *RateLimiter, remote, xff string, trusted bool) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = remote
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		rec := httptest.NewRecorder()
		c := core.NewContext(rec, req)
		if trusted {
			_, ipNet, _ := net.ParseCIDR("9.9.9.0/24")
			c.SetTrustedProxies([]*net.IPNet{ipNet})
		}
		c.SetHandlers([]core.HandlerFunc{RateLimit(rl), func(c *core.Context) { c.Success("ok", nil) }})
		c.Run()
		return rec.Code
	}

	// 未信任代理：XFF 被忽略，按 RemoteAddr 限流
	rl := NewRateLimiter(1, time.Second, nil)
	if got := run(rl, "9.9.9.9:80", "1.1.1.1", false); got != http.StatusOK {
		t.Errorf("未信任代理首次应放行：%d", got)
	}
	if got := run(rl, "9.9.9.9:80", "2.2.2.2", false); got != http.StatusTooManyRequests {
		t.Errorf("未信任代理应仍按 RemoteAddr 限流：%d", got)
	}

	// 信任代理：按 XFF 首个 IP 限流
	rl2 := NewRateLimiter(1, time.Second, nil)
	if got := run(rl2, "9.9.9.9:80", "1.1.1.1", true); got != http.StatusOK {
		t.Errorf("信任代理首次应放行：%d", got)
	}
	if got := run(rl2, "9.9.9.9:80", "1.1.1.1", true); got != http.StatusTooManyRequests {
		t.Errorf("信任代理同 XFF 应限流：%d", got)
	}
	if got := run(rl2, "9.9.9.9:80", "5.5.5.5", true); got != http.StatusOK {
		t.Errorf("信任代理不同 XFF 应放行：%d", got)
	}
}
