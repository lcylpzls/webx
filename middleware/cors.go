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
func CORS(cfg CORSConfig) core.HandlerFunc {
	allowAll := false
	for _, o := range cfg.AllowedOrigins {
		if o == "*" {
			allowAll = true
			break
		}
	}
	return func(c *core.Context) {
		origin := c.GetHeaderCanonical(canonicalOrigin)
		if allowAll {
			c.SetHeaderCanonical(canonicalAllowOrigin, "*")
		} else if origin != "" {
			for _, o := range cfg.AllowedOrigins {
				if o == origin {
					c.SetHeaderCanonical(canonicalAllowOrigin, origin)
					c.SetHeaderCanonical(canonicalVary, "Origin")
					break
				}
			}
		}
		if len(cfg.AllowedMethods) > 0 {
			c.SetHeaderCanonical(canonicalAllowMethods, strings.Join(cfg.AllowedMethods, ", "))
		}
		if len(cfg.AllowedHeaders) > 0 {
			c.SetHeaderCanonical(canonicalAllowHeaders, strings.Join(cfg.AllowedHeaders, ", "))
		}
		if len(cfg.ExposeHeaders) > 0 {
			c.SetHeaderCanonical(canonicalExposeHeaders, strings.Join(cfg.ExposeHeaders, ", "))
		}
		if cfg.AllowCredentials {
			c.SetHeaderCanonical(canonicalAllowCredentials, "true")
		}
		if cfg.AllowPrivateNetwork {
			c.SetHeaderCanonical(canonicalAllowPrivateNetwork, "true")
		}
		if cfg.MaxAge > 0 {
			c.SetHeaderCanonical(canonicalMaxAge, strconv.Itoa(cfg.MaxAge))
		}
		if c.Request().Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
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
