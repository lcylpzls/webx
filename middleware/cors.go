package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/lcylpzls/webx/internal/core"
)

// CORS 响应头的预计算规范化键。
var (
	canonicalOrigin              = core.CanonicalHeaderKey("Origin")
	canonicalAllowOrigin         = core.CanonicalHeaderKey("Access-Control-Allow-Origin")
	canonicalAllowMethods        = core.CanonicalHeaderKey("Access-Control-Allow-Methods")
	canonicalAllowHeaders        = core.CanonicalHeaderKey("Access-Control-Allow-Headers")
	canonicalExposeHeaders       = core.CanonicalHeaderKey("Access-Control-Expose-Headers")
	canonicalAllowCredentials    = core.CanonicalHeaderKey("Access-Control-Allow-Credentials")
	canonicalAllowPrivateNetwork = core.CanonicalHeaderKey("Access-Control-Allow-Private-Network")
	canonicalMaxAge              = core.CanonicalHeaderKey("Access-Control-Max-Age")
)

// CORSConfig 定义 CORS 中间件的配置参数。
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposeHeaders    []string
	MaxAge           int
	AllowCredentials bool
	// AllowPrivateNetwork 预检响应输出 Access-Control-Allow-Private-Network: true。
	AllowPrivateNetwork bool
}

// CORS 返回 CORS 跨域处理中间件。
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	allowAll := false
	for _, o := range cfg.AllowedOrigins {
		if o == "*" {
			allowAll = true
			break
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get(canonicalOrigin)
			if allowAll {
				w.Header().Set(canonicalAllowOrigin, "*")
			} else if origin != "" {
				for _, o := range cfg.AllowedOrigins {
					if o == origin {
						w.Header().Set(canonicalAllowOrigin, origin)
						w.Header().Set(canonicalVary, "Origin")
						break
					}
				}
			}
			if len(cfg.AllowedMethods) > 0 {
				w.Header().Set(canonicalAllowMethods, strings.Join(cfg.AllowedMethods, ", "))
			}
			if len(cfg.AllowedHeaders) > 0 {
				w.Header().Set(canonicalAllowHeaders, strings.Join(cfg.AllowedHeaders, ", "))
			}
			if len(cfg.ExposeHeaders) > 0 {
				w.Header().Set(canonicalExposeHeaders, strings.Join(cfg.ExposeHeaders, ", "))
			}
			if cfg.AllowCredentials {
				w.Header().Set(canonicalAllowCredentials, "true")
			}
			if cfg.AllowPrivateNetwork {
				w.Header().Set(canonicalAllowPrivateNetwork, "true")
			}
			if cfg.MaxAge > 0 {
				w.Header().Set(canonicalMaxAge, strconv.Itoa(cfg.MaxAge))
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// DefaultCORSConfig 返回常用 CORS 默认配置。
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"},
		ExposeHeaders:  []string{"X-Request-ID"},
		MaxAge:         86400,
	}
}
