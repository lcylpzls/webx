package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lcylpzls/webx/internal/core"
)

// RateLimiter 实现基于 IP 的令牌桶限流。
type RateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*tokenBucket
	qps       int
	window    time.Duration
	whitelist []*net.IPNet
}

type tokenBucket struct {
	tokens   float64
	lastTime time.Time
}

// NewRateLimiter 创建 IP 限流器。
func NewRateLimiter(qps int, window time.Duration, whitelistCIDRs []string) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*tokenBucket),
		qps:     qps,
		window:  window,
	}
	for _, cidr := range whitelistCIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			ip := net.ParseIP(cidr)
			if ip != nil {
				_, ipNet, _ = net.ParseCIDR(cidr + "/32")
			}
		}
		if ipNet != nil {
			rl.whitelist = append(rl.whitelist, ipNet)
		}
	}
	return rl
}

// Allow 检查指定 IP 是否被允许通过。
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	parsedIP := net.ParseIP(ip)
	if parsedIP != nil {
		for _, wl := range rl.whitelist {
			if wl.Contains(parsedIP) {
				return true
			}
		}
	}

	now := time.Now()
	bucket, exists := rl.buckets[ip]
	if !exists {
		bucket = &tokenBucket{tokens: float64(rl.qps), lastTime: now}
		rl.buckets[ip] = bucket
	}
	elapsed := now.Sub(bucket.lastTime).Seconds()
	bucket.tokens += elapsed * float64(rl.qps)
	if bucket.tokens > float64(rl.qps) {
		bucket.tokens = float64(rl.qps)
	}
	bucket.lastTime = now
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}
	return false
}

// Cleanup 清理超过 window*10 未活动的桶。
func (rl *RateLimiter) Cleanup(interval time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	for ip, bucket := range rl.buckets {
		if now.Sub(bucket.lastTime) > interval*10 {
			delete(rl.buckets, ip)
		}
	}
}

// extractClientIP 提取客户端 IP：X-Forwarded-For → X-Real-IP → RemoteAddr。
func extractClientIP(c *core.Context) string {
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(c.Request().RemoteAddr)
	if err != nil {
		return c.Request().RemoteAddr
	}
	return host
}

// RateLimit 返回 IP 令牌桶限流中间件，超限返回标准化 429。
func RateLimit(rl *RateLimiter) core.HandlerFunc {
	return func(c *core.Context) {
		if !rl.Allow(extractClientIP(c)) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, "请求过于频繁，请稍后重试", nil)
			return
		}
		c.Next()
	}
}
