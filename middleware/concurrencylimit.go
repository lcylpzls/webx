package middleware

import (
	"net/http"
	"sync/atomic"

	"github.com/lcylpzls/webx/internal/core"
)

// ConcurrencyLimiter 限制同一时刻处理的请求数。
type ConcurrencyLimiter struct {
	max       int64
	active    atomic.Int64
	rejected  atomic.Uint64
	rejectMsg atomic.Pointer[string]
	sink      MetricsSink
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
			l.rejected.Add(1)
			l.emitRejected()
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

// Rejected 返回因额度已满被拒绝的请求数。
func (l *ConcurrencyLimiter) Rejected() uint64 {
	return l.rejected.Load()
}

// SetRejectMessage 设置拒绝响应文案；空字符串使用默认文案。
func (l *ConcurrencyLimiter) SetRejectMessage(msg string) {
	l.rejectMsg.Store(&msg)
}

// SetMetricsSink 注入外部指标接收器（启动前调用，可为 nil）。
func (l *ConcurrencyLimiter) SetMetricsSink(sink MetricsSink) {
	l.sink = sink
}

// emitRejected 转发并发拒绝事件。
func (l *ConcurrencyLimiter) emitRejected() {
	if l.sink != nil {
		l.sink.IncCounter("webx.concurrency_rejected")
	}
}

// ConcurrencyLimit 返回并发限制中间件；额度已满时返回 503 并携带 Retry-After。
func ConcurrencyLimit(l *ConcurrencyLimiter) core.HandlerFunc {
	return func(c *core.Context) {
		if !l.TryAcquire() {
			c.SetHeaderCanonical(canonicalRetryAfter, "1")
			if p := l.rejectMsg.Load(); p != nil && *p != "" {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, *p, nil)
				return
			}
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, "请求过于繁忙，请稍后重试", nil)
			return
		}
		defer l.Release()
		c.Next()
	}
}
