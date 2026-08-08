package middleware

import (
	"net/http"
	"sync/atomic"

	"github.com/lcylpzls/webx/internal/core"
)

// ConcurrencyLimiter 限制同一时刻处理的请求数。
type ConcurrencyLimiter struct {
	max    int64
	active atomic.Int64
}

// NewConcurrencyLimiter 创建并发限制器；max <= 0 表示不限制。
func NewConcurrencyLimiter(max int) *ConcurrencyLimiter {
	return &ConcurrencyLimiter{max: int64(max)}
}

// TryAcquire 尝试占用一个并发额度；额度已满返回 false。
func (l *ConcurrencyLimiter) TryAcquire() bool {
	if l.max <= 0 {
		return true
	}
	for {
		current := l.active.Load()
		if current >= l.max {
			return false
		}
		if l.active.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

// Release 释放一个并发额度。
func (l *ConcurrencyLimiter) Release() {
	l.active.Add(-1)
}

// Active 返回当前占用的并发额度。
func (l *ConcurrencyLimiter) Active() int64 {
	return l.active.Load()
}

// ConcurrencyLimit 返回并发限制中间件；额度已满时返回 503 并携带 Retry-After。
func ConcurrencyLimit(l *ConcurrencyLimiter) core.HandlerFunc {
	return func(c *core.Context) {
		if !l.TryAcquire() {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, "请求过于繁忙，请稍后重试", nil)
			return
		}
		defer l.Release()
		c.Next()
	}
}
