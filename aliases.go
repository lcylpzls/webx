package webx

import (
	"net/http"

	"github.com/lcylpzls/webx/v2/internal/core"
)

// 对外类型别名：业务代码只接触 webx 包，不感知 internal/core。
type (
	// Context 是单个请求的上下文。
	Context = core.Context
	// StandardizedResponse 是统一的标准 JSON 响应体。
	StandardizedResponse = core.StandardizedResponse
)

// 标准化响应业务码。
const (
	CodeSuccess            = core.CodeSuccess
	CodeBadRequest         = core.CodeBadRequest
	CodeNotFound           = core.CodeNotFound
	CodeMethodNotAllowed   = core.CodeMethodNotAllowed
	CodeTooManyRequests    = core.CodeTooManyRequests
	CodeInternalError      = core.CodeInternalError
	CodeServiceUnavailable = core.CodeServiceUnavailable
)

// NoRouteHandler 404 兜底处理器（嵌入自定义路由器时使用）。
var NoRouteHandler = core.NoRouteHandler

// NoMethodHandler 405 兜底处理器。
var NoMethodHandler = core.NoMethodHandler

// NewContext 创建请求上下文（用于在自定义路由器中嵌入 webx Handler）。
func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	return core.NewContext(w, r)
}
