package middleware

import (
	"fmt"
	"time"

	"github.com/lcylpzls/webx/internal/core"
)

// SecurityHeadersOptions 定义安全响应头中间件的配置。
type SecurityHeadersOptions struct {
	// ContentTypeNoSniff 设置 X-Content-Type-Options: nosniff。
	ContentTypeNoSniff bool
	// FrameDeny 设置 X-Frame-Options: DENY。
	FrameDeny bool
	// ReferrerPolicy 设置 Referrer-Policy（空则不设置）。
	ReferrerPolicy string
	// HSTSMaxAge 大于 0 时设置 Strict-Transport-Security。
	HSTSMaxAge time.Duration
}

// SecurityHeaders 返回安全响应头中间件。
func SecurityHeaders(opts SecurityHeadersOptions) core.HandlerFunc {
	return func(c *core.Context) {
		if opts.ContentTypeNoSniff {
			c.Header("X-Content-Type-Options", "nosniff")
		}
		if opts.FrameDeny {
			c.Header("X-Frame-Options", "DENY")
		}
		if opts.ReferrerPolicy != "" {
			c.Header("Referrer-Policy", opts.ReferrerPolicy)
		}
		if opts.HSTSMaxAge > 0 {
			c.Header("Strict-Transport-Security",
				fmt.Sprintf("max-age=%d", int64(opts.HSTSMaxAge.Seconds())))
		}
		c.Next()
	}
}
