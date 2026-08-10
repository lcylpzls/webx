package middleware

import (
	"fmt"
	"time"

	"github.com/lcylpzls/webx/internal/core"
)

// 安全响应头的预计算规范化键。
var (
	canonicalContentTypeOptions = core.CanonicalHeaderKey("X-Content-Type-Options")
	canonicalFrameOptions       = core.CanonicalHeaderKey("X-Frame-Options")
	canonicalReferrerPolicy     = core.CanonicalHeaderKey("Referrer-Policy")
	canonicalHSTS               = core.CanonicalHeaderKey("Strict-Transport-Security")
	canonicalPermissionsPolicy  = core.CanonicalHeaderKey("Permissions-Policy")
	canonicalCOOP               = core.CanonicalHeaderKey("Cross-Origin-Opener-Policy")
	canonicalCORP               = core.CanonicalHeaderKey("Cross-Origin-Resource-Policy")
	canonicalCOEP               = core.CanonicalHeaderKey("Cross-Origin-Embedder-Policy")
	canonicalCSP                = core.CanonicalHeaderKey("Content-Security-Policy")
	canonicalCSPReportOnly      = core.CanonicalHeaderKey("Content-Security-Policy-Report-Only")
	canonicalOriginAgentCluster = core.CanonicalHeaderKey("Origin-Agent-Cluster")
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
	// ContentSecurityPolicy 设置 Content-Security-Policy（空则不设置）。
	ContentSecurityPolicy string
	// ContentSecurityPolicyReportOnly 设置 Content-Security-Policy-Report-Only（空则不设置）。
	ContentSecurityPolicyReportOnly string
	// HSTSIncludeSubDomains HSTS 指令附加 includeSubDomains。
	HSTSIncludeSubDomains bool
	// HSTSPreload HSTS 指令附加 preload。
	HSTSPreload bool
	// OriginAgentCluster 设置 Origin-Agent-Cluster: ?1（站点隔离）。
	OriginAgentCluster bool
}

// SecurityHeaders 返回安全响应头中间件。
func SecurityHeaders(opts SecurityHeadersOptions) core.HandlerFunc {
	return func(c *core.Context) {
		if opts.ContentTypeNoSniff {
			c.SetHeaderCanonical(canonicalContentTypeOptions, "nosniff")
		}
		if opts.FrameDeny {
			c.SetHeaderCanonical(canonicalFrameOptions, "DENY")
		}
		if opts.ReferrerPolicy != "" {
			c.SetHeaderCanonical(canonicalReferrerPolicy, opts.ReferrerPolicy)
		}
		if opts.HSTSMaxAge > 0 {
			hsts := fmt.Sprintf("max-age=%d", int64(opts.HSTSMaxAge.Seconds()))
			if opts.HSTSIncludeSubDomains {
				hsts += "; includeSubDomains"
			}
			if opts.HSTSPreload {
				hsts += "; preload"
			}
			c.SetHeaderCanonical(canonicalHSTS, hsts)
		}
		if opts.PermissionsPolicy != "" {
			c.SetHeaderCanonical(canonicalPermissionsPolicy, opts.PermissionsPolicy)
		}
		if opts.CrossOriginOpenerPolicy != "" {
			c.SetHeaderCanonical(canonicalCOOP, opts.CrossOriginOpenerPolicy)
		}
		if opts.CrossOriginResourcePolicy != "" {
			c.SetHeaderCanonical(canonicalCORP, opts.CrossOriginResourcePolicy)
		}
		if opts.CrossOriginEmbedderPolicy != "" {
			c.SetHeaderCanonical(canonicalCOEP, opts.CrossOriginEmbedderPolicy)
		}
		if opts.ContentSecurityPolicy != "" {
			c.SetHeaderCanonical(canonicalCSP, opts.ContentSecurityPolicy)
		}
		if opts.ContentSecurityPolicyReportOnly != "" {
			c.SetHeaderCanonical(canonicalCSPReportOnly, opts.ContentSecurityPolicyReportOnly)
		}
		if opts.OriginAgentCluster {
			c.SetHeaderCanonical(canonicalOriginAgentCluster, "?1")
		}
		c.Next()
	}
}
