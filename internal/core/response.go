package core

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"time"
)

// responseBufPool 复用标准化响应编码缓冲。
var responseBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, 256)
		return &buf
	},
}

const hexDigits = "0123456789abcdef"

// appendJSONString 手动编码 JSON 字符串，转义规则与 encoding/json 默认一致
// （引号、反斜杠、控制字符与 HTML 敏感字符）。
func appendJSONString(b []byte, s string) []byte {
	b = append(b, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			b = append(b, '\\', c)
		case c == '<':
			b = append(b, '\\', 'u', '0', '0', '3', 'c')
		case c == '>':
			b = append(b, '\\', 'u', '0', '0', '3', 'e')
		case c == '&':
			b = append(b, '\\', 'u', '0', '0', '2', '6')
		case c < 0x20:
			b = append(b, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xf])
		default:
			b = append(b, c)
		}
	}
	return append(b, '"')
}

// 标准化响应业务码。
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
	c.writeStandardized(httpStatus, msg, data, httpStatus)
}

// Success 以标准化格式写入成功响应，code 固定为 0。
func (c *Context) Success(msg string, data any) {
	c.writeStandardized(http.StatusOK, msg, data, CodeSuccess)
}

// Fail 以标准化格式写入失败响应，code 由调用方指定。
func (c *Context) Fail(httpStatus, code int, msg string) {
	c.writeStandardized(httpStatus, msg, nil, code)
}

// writeStandardized 手写编码标准化响应信封（热路径零反射、零 fmt）。
// 字段顺序与编码规则与 StandardizedResponse 的 encoding/json 输出一致。
func (c *Context) writeStandardized(httpStatus int, msg string, data any, code int) {
	buf := responseBufPool.Get().(*[]byte)
	b := (*buf)[:0]
	defer func() {
		*buf = b
		responseBufPool.Put(buf)
	}()
	b = append(b, `{"code":`...)
	b = strconv.AppendInt(b, int64(code), 10)
	b = append(b, `,"msg":`...)
	b = appendJSONString(b, msg)
	if data != nil && !isEmptyValue(reflect.ValueOf(data)) {
		b = append(b, `,"data":`...)
		dataBytes, err := json.Marshal(data)
		if err != nil {
			dataBytes = []byte("null")
		}
		b = append(b, dataBytes...)
	}
	b = append(b, `,"requestId":`...)
	b = appendJSONString(b, c.RequestID())
	b = append(b, `,"timestamp":`...)
	b = strconv.AppendInt(b, time.Now().UnixMilli(), 10)
	b = append(b, '}')
	setContentType(c.writer, "application/json; charset=utf-8")
	c.writeHeader(httpStatus)
	_, _ = c.writer.Write(b)
}

// isEmptyValue 与 encoding/json 的 omitempty 判定一致（结构体恒为不空）。
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Interface, reflect.Pointer:
		return v.IsZero()
	}
	return false
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
