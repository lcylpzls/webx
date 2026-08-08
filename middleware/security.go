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
	// PermissionsPolicy 设置 Permissions-Policy（空则不设置）。
	PermissionsPolicy string
	// CrossOriginOpenerPolicy 设置 Cross-Origin-Opener-Policy（空则不设置）。
	CrossOriginOpenerPolicy string
	// CrossOriginResourcePolicy 设置 Cross-Origin-Resource-Policy（空则不设置）。
	CrossOriginResourcePolicy string
	// CrossOriginEmbedderPolicy 设置 Cross-Origin-Embedder-Policy（空则不设置）。
	CrossOriginEmbedderPolicy string
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
		if opts.PermissionsPolicy != "" {
			c.Header("Permissions-Policy", opts.PermissionsPolicy)
		}
		if opts.CrossOriginOpenerPolicy != "" {
			c.Header("Cross-Origin-Opener-Policy", opts.CrossOriginOpenerPolicy)
		}
		if opts.CrossOriginResourcePolicy != "" {
			c.Header("Cross-Origin-Resource-Policy", opts.CrossOriginResourcePolicy)
		}
		if opts.CrossOriginEmbedderPolicy != "" {
			c.Header("Cross-Origin-Embedder-Policy", opts.CrossOriginEmbedderPolicy)
		}
		c.Next()
	}
}
