// Package core 提供 webx 的请求上下文、中间件链与标准化响应等核心原语。
// 独立于根包，避免根包与 middleware 子包之间的循环依赖。
package core

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
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
	params   []Param
	values   map[string]any
	route    string
	group    string
	trusted  []*net.IPNet
	handlers []HandlerFunc
	index    int
	aborted  bool
	status   int
	wrote    bool
	maxBody  int64
}

// Param 是单个路由参数（名称 + 值）。
type Param struct {
	Name  string
	Value string
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
func (c *Context) SetParams(params []Param) {
	c.params = params
}

// Param 返回路由参数值；同名参数存在多个时返回最后一次匹配值。
func (c *Context) Param(key string) string {
	value := ""
	for _, p := range c.params {
		if p.Name == key {
			value = p.Value
		}
	}
	return value
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

// SetTrustedProxies 设置可信代理网段（由服务层在进入处理器链前调用）。
// 仅当对端地址位于可信网段时才信任 X-Forwarded-For / X-Real-IP。
func (c *Context) SetTrustedProxies(proxies []*net.IPNet) {
	c.trusted = append([]*net.IPNet(nil), proxies...)
}

// RemoteIP 提取客户端 IP。
// 对端位于可信代理网段时优先 X-Forwarded-For → X-Real-IP，否则一律取 RemoteAddr，
// 防止客户端伪造代理头污染限流、审计与日志。
func (c *Context) RemoteIP() string {
	remote := c.remoteHost()
	if c.isTrustedProxy(remote) {
		if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[0])
		}
		if xri := c.GetHeader("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}
	return remote
}

// remoteHost 返回对端主机（去掉端口）。
func (c *Context) remoteHost() string {
	host, _, err := net.SplitHostPort(c.request.RemoteAddr)
	if err != nil {
		return c.request.RemoteAddr
	}
	return host
}

// isTrustedProxy 判断对端 IP 是否位于可信代理网段。
func (c *Context) isTrustedProxy(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, p := range c.trusted {
		if p.Contains(parsed) {
			return true
		}
	}
	return false
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

// Redirect 返回重定向响应；非 3xx 状态码统一转为 302。
func (c *Context) Redirect(status int, location string) {
	if status < 300 || status > 399 {
		status = http.StatusFound
	}
	c.Header("Location", location)
	c.writeHeader(status)
	_, _ = fmt.Fprintf(c.writer, "<a href=\"%s\">%s</a>", html.EscapeString(location), http.StatusText(status))
}

// Cookie 返回请求中指定名称的 Cookie。
func (c *Context) Cookie(name string) (*http.Cookie, error) {
	return c.request.Cookie(name)
}

// SetCookie 向响应写入 Cookie。
func (c *Context) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.writer, cookie)
}

// SetSecureCookie 写入 Cookie 并补齐安全属性：
// 未显式设置时强制 Secure、HttpOnly 与 SameSite=Lax。
func (c *Context) SetSecureCookie(cookie *http.Cookie) {
	if !cookie.Secure {
		cookie.Secure = true
	}
	if !cookie.HttpOnly {
		cookie.HttpOnly = true
	}
	if cookie.SameSite == 0 { // 未显式设置时补齐 SameSite=Lax（DefaultMode 保持不输出）
		cookie.SameSite = http.SameSiteLaxMode
	}
	http.SetCookie(c.writer, cookie)
}

// JSON 以 JSON 格式写入响应。
func (c *Context) JSON(code int, data any) error {
	setContentType(c.writer, "application/json; charset=utf-8")
	c.writeHeader(code)
	return json.NewEncoder(c.writer).Encode(data)
}

// String 以纯文本格式写入响应。
func (c *Context) String(code int, format string, args ...any) error {
	setContentType(c.writer, "text/plain; charset=utf-8")
	c.writeHeader(code)
	if len(args) == 0 && !strings.ContainsRune(format, '%') {
		_, err := io.WriteString(c.writer, format)
		return err
	}
	_, err := fmt.Fprintf(c.writer, format, args...)
	return err
}

// setContentType 直接写入规范化键 Content-Type，
// 避免 textproto 每次调用 Set 时的键规范化分配（热路径零分配）。
func setContentType(w http.ResponseWriter, value string) {
	h := w.Header()
	if len(h["Content-Type"]) == 0 {
		h["Content-Type"] = []string{value}
		return
	}
	h["Content-Type"][0] = value
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

// Bind 按 Content-Type 自动分派绑定方式：
// application/json → BindJSON；multipart/form-data 或 urlencoded → BindForm；
// 其余（如 GET）→ BindQuery。
func (c *Context) Bind(out any) error {
	ct := strings.ToLower(c.request.Header.Get("Content-Type"))
	switch {
	case strings.Contains(ct, "application/json"):
		return c.BindJSON(out)
	case strings.Contains(ct, "multipart/form-data"), strings.Contains(ct, "application/x-www-form-urlencoded"):
		return c.BindForm(out)
	default:
		return c.BindQuery(out)
	}
}

// openFile 打开待服务文件（测试可替换以覆盖异常分支）。
var openFile = os.Open

// statFile 查询文件信息（测试可替换以覆盖异常分支）。
var statFile = os.Stat

// statusRecorder 记录首个写入的 HTTP 状态码。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader 记录状态码并透传。
func (w *statusRecorder) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

// File 输出单个文件（支持 304 与 Range）。
func (c *Context) File(path string) {
	f, err := openFile(path)
	if err != nil {
		c.writeHeader(http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := statFile(path)
	if err != nil {
		c.writeHeader(http.StatusInternalServerError)
		return
	}
	rec := &statusRecorder{ResponseWriter: c.writer}
	http.ServeContent(rec, c.request, filepath.Base(path), info.ModTime(), f)
	c.status = rec.status
	c.wrote = true
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
	c.trusted = nil
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
