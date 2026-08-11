package middleware

import (
	"fmt"
	"net/http"
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
func SecurityHeaders(opts SecurityHeadersOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if opts.ContentTypeNoSniff {
				w.Header().Set(canonicalContentTypeOptions, "nosniff")
			}
			if opts.FrameDeny {
				w.Header().Set(canonicalFrameOptions, "DENY")
			}
			if opts.ReferrerPolicy != "" {
				w.Header().Set(canonicalReferrerPolicy, opts.ReferrerPolicy)
			}
			if opts.HSTSMaxAge > 0 {
				hsts := fmt.Sprintf("max-age=%d", int64(opts.HSTSMaxAge.Seconds()))
				if opts.HSTSIncludeSubDomains {
					hsts += "; includeSubDomains"
				}
				if opts.HSTSPreload {
					hsts += "; preload"
				}
				w.Header().Set(canonicalHSTS, hsts)
			}
			if opts.PermissionsPolicy != "" {
				w.Header().Set(canonicalPermissionsPolicy, opts.PermissionsPolicy)
			}
			if opts.CrossOriginOpenerPolicy != "" {
				w.Header().Set(canonicalCOOP, opts.CrossOriginOpenerPolicy)
			}
			if opts.CrossOriginResourcePolicy != "" {
				w.Header().Set(canonicalCORP, opts.CrossOriginResourcePolicy)
			}
			if opts.CrossOriginEmbedderPolicy != "" {
				w.Header().Set(canonicalCOEP, opts.CrossOriginEmbedderPolicy)
			}
			if opts.ContentSecurityPolicy != "" {
				w.Header().Set(canonicalCSP, opts.ContentSecurityPolicy)
			}
			if opts.ContentSecurityPolicyReportOnly != "" {
				w.Header().Set(canonicalCSPReportOnly, opts.ContentSecurityPolicyReportOnly)
			}
			if opts.OriginAgentCluster {
				w.Header().Set(canonicalOriginAgentCluster, "?1")
			}
			next.ServeHTTP(w, r)
		})
	}
}
