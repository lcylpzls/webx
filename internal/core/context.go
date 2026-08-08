// Package core 提供 webx 的请求上下文、中间件链与标准化响应等核心原语。
// 独立于根包，避免根包与 middleware 子包之间的循环依赖。
package core

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
)

// HandlerFunc 是 webx 的业务处理器签名，不依赖任何第三方类型。
type HandlerFunc func(*Context)

// Context 是单个请求的上下文，自研实现。
// 中间件链语义与 gin 对齐：Next 继续执行、Abort 提前终止。
type Context struct {
	writer   http.ResponseWriter
	request  *http.Request
	params   map[string]string
	values   map[string]any
	route    string
	group    string
	handlers []HandlerFunc
	index    int
	aborted  bool
	status   int
	wrote    bool
	maxBody  int64
}

// ctxPool 是请求上下文的复用池。
var ctxPool = sync.Pool{
	New: func() any {
		return NewContext(nil, nil)
	},
}

// Acquire 从池中获取一个上下文并绑定请求。
func Acquire(w http.ResponseWriter, r *http.Request) *Context {
	c := ctxPool.Get().(*Context)
	c.Reset()
	c.writer = w
	c.request = r
	return c
}

// Release 将上下文归还池中复用。
func Release(c *Context) {
	ctxPool.Put(c)
}

// NewContext 创建请求上下文。
func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{
		writer:  w,
		request: r,
		index:   -1,
	}
}

// Request 返回原始请求。
func (c *Context) Request() *http.Request {
	return c.request
}

// Writer 返回响应写入器。
func (c *Context) Writer() http.ResponseWriter {
	return c.writer
}

// SetWriter 替换响应写入器（供 Timeout 等中间件包装使用）。
func (c *Context) SetWriter(w http.ResponseWriter) {
	c.writer = w
}

// SetMaxBodyBytes 设置 BindJSON 的最大请求体字节数；<=0 表示不限制。
func (c *Context) SetMaxBodyBytes(n int64) {
	c.maxBody = n
}

// SetRequest 替换请求（供 Timeout 等中间件注入带超时的 Context）。
func (c *Context) SetRequest(r *http.Request) {
	c.request = r
}

// SetParams 设置路由参数（由路由层在进入处理器链前调用）。
func (c *Context) SetParams(params map[string]string) {
	c.params = params
}

// Param 返回路由参数值，不存在时返回空字符串。
func (c *Context) Param(key string) string {
	if c.params == nil {
		return ""
	}
	return c.params[key]
}

// SetRoute 记录当前请求匹配的路由模式（由路由层在进入处理器链前调用）。
func (c *Context) SetRoute(route string) {
	c.route = route
}

// Route 返回当前请求匹配的路由模式；未匹配（404/405 等）时为空字符串。
func (c *Context) Route() string {
	return c.route
}

// SetGroup 记录当前请求所属分组前缀（由路由层在进入处理器链前调用）。
func (c *Context) SetGroup(group string) {
	c.group = group
}

// Group 返回当前请求所属分组前缀；未分组时为空字符串。
func (c *Context) Group() string {
	return c.group
}

// Query 返回查询参数值。
func (c *Context) Query(key string) string {
	return c.request.URL.Query().Get(key)
}

// GetHeader 返回请求头值。
func (c *Context) GetHeader(key string) string {
	return c.request.Header.Get(key)
}

// RemoteIP 提取客户端 IP。
// 优先级：X-Forwarded-For → X-Real-IP → RemoteAddr。
func (c *Context) RemoteIP() string {
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(c.request.RemoteAddr)
	if err != nil {
		return c.request.RemoteAddr
	}
	return host
}

// Set 保存请求级 KV。
func (c *Context) Set(key string, val any) {
	if c.values == nil {
		c.values = make(map[string]any)
	}
	c.values[key] = val
}

// Get 读取请求级 KV。
func (c *Context) Get(key string) (any, bool) {
	val, ok := c.values[key]
	return val, ok
}

// GetString 读取请求级 KV 并转为字符串，不存在或类型不符时返回空字符串。
func (c *Context) GetString(key string) string {
	if val, ok := c.Get(key); ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// RequestID 返回请求 ID（RequestID 中间件写入的 "requestId" 值）。
func (c *Context) RequestID() string {
	return c.GetString("requestId")
}

// Status 写入 HTTP 状态码（仅首次写入生效）。
func (c *Context) Status(code int) {
	c.writeHeader(code)
}

// StatusCode 返回已写入的状态码；未写入时视为 200。
func (c *Context) StatusCode() int {
	if c.status == 0 {
		return http.StatusOK
	}
	return c.status
}

// Header 设置响应头。
func (c *Context) Header(key, value string) {
	c.writer.Header().Set(key, value)
}

// JSON 以 JSON 格式写入响应。
func (c *Context) JSON(code int, data any) error {
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.writeHeader(code)
	return json.NewEncoder(c.writer).Encode(data)
}

// String 以纯文本格式写入响应。
func (c *Context) String(code int, format string, args ...any) error {
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.writeHeader(code)
	_, err := fmt.Fprintf(c.writer, format, args...)
	return err
}

// BindJSON 解析请求体 JSON 到 out。
func (c *Context) BindJSON(out any) error {
	body := c.request.Body
	if c.maxBody > 0 {
		body = http.MaxBytesReader(c.writer, body, c.maxBody)
	}
	decoder := json.NewDecoder(body)
	return decoder.Decode(out)
}

// SetHandlers 设置待执行的处理器链（中间件 + 最终处理器）。
func (c *Context) SetHandlers(handlers []HandlerFunc) {
	c.handlers = handlers
}

// Run 从头开始执行处理器链。
func (c *Context) Run() {
	c.index = -1
	c.Next()
}

// Next 继续执行处理器链中的下一个处理器。
func (c *Context) Next() {
	c.index++
	for c.index < len(c.handlers) && !c.aborted {
		c.handlers[c.index](c)
		c.index++
	}
}

// Abort 终止处理器链，后续处理器不再执行。
func (c *Context) Abort() {
	c.aborted = true
}

// IsAborted 返回链是否已终止。
func (c *Context) IsAborted() bool {
	return c.aborted
}

// ResetWriteState 重置响应头写入状态（供 Timeout 中间件在超时后回写 503 使用）。
func (c *Context) ResetWriteState() {
	c.wrote = false
	c.status = 0
}

// Reset 清空全部状态，供池化复用。
func (c *Context) Reset() {
	c.writer = nil
	c.request = nil
	c.params = nil
	c.values = nil
	c.route = ""
	c.group = ""
	c.handlers = nil
	c.index = -1
	c.aborted = false
	c.status = 0
	c.wrote = false
	c.maxBody = 0
}

// writeHeader 仅写入一次响应头。
func (c *Context) writeHeader(code int) {
	if c.wrote {
		return
	}
	c.wrote = true
	c.status = code
	c.writer.WriteHeader(code)
}
