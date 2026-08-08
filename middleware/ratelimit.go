package middleware

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lcylpzls/webx/internal/core"
)

// RateLimiter 实现基于 IP 的令牌桶限流。
type RateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*tokenBucket
	qps        int
	window     time.Duration
	whitelist  []*net.IPNet
	rejected   atomic.Uint64
	maxBuckets int
	keyFunc    func(*core.Context) string
	rejectMsg  string
}

type tokenBucket struct {
	tokens   float64
	lastTime time.Time
}

// NewRateLimiter 创建 IP 限流器。
func NewRateLimiter(qps int, window time.Duration, whitelistCIDRs []string) *RateLimiter {
	rl := &RateLimiter{
		buckets:    make(map[string]*tokenBucket),
		qps:        qps,
		window:     window,
		maxBuckets: 100000,
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

// SetMaxBuckets 设置 IP 桶数量上限；达到上限后新 IP 直接拒绝。
func (rl *RateLimiter) SetMaxBuckets(n int) {
	if n > 0 {
		rl.maxBuckets = n
	}
}

// SetKeyFunc 设置限流维度提取函数（默认按客户端 IP）。
func (rl *RateLimiter) SetKeyFunc(fn func(*core.Context) string) {
	if fn != nil {
		rl.keyFunc = fn
	}
}

// SetRejectMessage 设置拒绝响应文案；空字符串使用默认文案。
func (rl *RateLimiter) SetRejectMessage(msg string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.rejectMsg = msg
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
		if len(rl.buckets) >= rl.maxBuckets {
			rl.rejected.Add(1)
			return false
		}
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
	rl.rejected.Add(1)
	return false
}

// Rejected 返回被拒绝的请求数。
func (rl *RateLimiter) Rejected() uint64 {
	return rl.rejected.Load()
}

// RetryAfter 返回指定 key 恢复 1 枚令牌所需的等待时间（秒，向上取整）。
func (rl *RateLimiter) RetryAfter(key string) time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.qps <= 0 {
		return 0
	}
	bucket, ok := rl.buckets[key]
	if !ok {
		return 0
	}
	missing := 1 - bucket.tokens
	if missing <= 0 {
		return 0
	}
	return time.Duration(math.Ceil(missing/float64(rl.qps))) * time.Second
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

// RateLimit 返回 IP 令牌桶限流中间件，超限返回标准化 429。
func RateLimit(rl *RateLimiter) core.HandlerFunc {
	return func(c *core.Context) {
		key := c.RemoteIP()
		if rl.keyFunc != nil {
			key = rl.keyFunc(c)
		}
		if !rl.Allow(key) {
			if retryAfter := rl.RetryAfter(key); retryAfter > 0 {
				c.SetHeaderCanonical(canonicalRetryAfter, strconv.FormatInt(int64(retryAfter.Seconds()), 10))
			}
			rl.mu.Lock()
			message := rl.rejectMsg
			rl.mu.Unlock()
			if message == "" {
				message = "请求过于频繁，请稍后重试"
			}
			c.AbortWithStatusJSON(http.StatusTooManyRequests, message, nil)
			return
		}
		c.Next()
	}
}
