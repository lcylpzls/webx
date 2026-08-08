package core

import (
	"net/http"
	"time"
)

// 标准化响应业务码，与 ginx 保持一致。
const (
	// CodeSuccess 表示请求成功。
	CodeSuccess = 0
	// CodeBadRequest 表示请求参数校验失败。
	CodeBadRequest = 400
	// CodeNotFound 表示请求的资源不存在。
	CodeNotFound = 404
	// CodeMethodNotAllowed 表示不支持的请求方法。
	CodeMethodNotAllowed = 405
	// CodeTooManyRequests 表示请求频率超限。
	CodeTooManyRequests = 429
	// CodeInternalError 表示服务器内部错误。
	CodeInternalError = 500
	// CodeServiceUnavailable 表示服务暂时不可用（如请求超时）。
	CodeServiceUnavailable = 503
)

// StandardizedResponse 是 webx 统一的标准 JSON 响应体。
// msg 字段使用简体中文。
type StandardizedResponse struct {
	Code      int    `json:"code"`
	Msg       string `json:"msg"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"requestId"`
	Timestamp int64  `json:"timestamp"`
}

// JSONResponse 以标准化格式写入响应，code 取 HTTP 状态码。
func (c *Context) JSONResponse(httpStatus int, msg string, data any) {
	_ = c.JSON(httpStatus, StandardizedResponse{
		Code:      httpStatus,
		Msg:       msg,
		Data:      data,
		RequestID: c.RequestID(),
		Timestamp: time.Now().UnixMilli(),
	})
}

// Success 以标准化格式写入成功响应，code 固定为 0。
func (c *Context) Success(msg string, data any) {
	_ = c.JSON(http.StatusOK, StandardizedResponse{
		Code:      CodeSuccess,
		Msg:       msg,
		Data:      data,
		RequestID: c.RequestID(),
		Timestamp: time.Now().UnixMilli(),
	})
}

// Fail 以标准化格式写入失败响应，code 由调用方指定。
func (c *Context) Fail(httpStatus, code int, msg string) {
	_ = c.JSON(httpStatus, StandardizedResponse{
		Code:      code,
		Msg:       msg,
		RequestID: c.RequestID(),
		Timestamp: time.Now().UnixMilli(),
	})
}

// AbortWithStatusJSON 终止处理器链并写入标准化响应。
func (c *Context) AbortWithStatusJSON(httpStatus int, msg string, data any) {
	c.Abort()
	c.JSONResponse(httpStatus, msg, data)
}

// NoRouteHandler 返回 404 兜底处理器。
func NoRouteHandler(c *Context) {
	c.JSONResponse(http.StatusNotFound, "请求的资源不存在", nil)
}

// NoMethodHandler 返回 405 兜底处理器。
func NoMethodHandler(c *Context) {
	c.JSONResponse(http.StatusMethodNotAllowed, "不支持的请求方法", nil)
}
