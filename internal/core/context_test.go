package core

import (
	"bytes"
	"encoding/json"
	"errors"
	testx "github.com/lcylpzls/testx"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextAccessors(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x?a=1", nil)
	req.RemoteAddr = "192.168.1.1:8080"
	req.Header.Set("X-Request-ID", "req-123")
	rec := httptest.NewRecorder()
	c := NewContext(rec, req)

	if c.Request() != req || c.Writer() != rec {
		t.Error("Request/Writer 访问器不符")
	}
	if got := c.Query("a"); got != "1" {
		t.Errorf("Query 不符：%s", got)
	}
	if got := c.GetHeader("X-Request-ID"); got != "req-123" {
		t.Errorf("GetHeader 不符：%s", got)
	}
	if got := c.RemoteIP(); got != "192.168.1.1" {
		t.Errorf("RemoteIP 不符：%s", got)
	}
	if got := c.RequestID(); got != "" {
		t.Errorf("未设置 requestId 时应为空：%s", got)
	}

	// 未配置可信代理时，代理头一律不信任
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
	if got := c.RemoteIP(); got != "192.168.1.1" {
		t.Errorf("未信任代理时不应采用 XFF：%s", got)
	}
	req.Header.Del("X-Forwarded-For")

	// 配置可信代理后，X-Forwarded-For 优先
	c.SetTrustedProxies([]*net.IPNet{mustParseCIDR(t, "192.168.1.0/24")})
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
	if got := c.RemoteIP(); got != "10.0.0.1" {
		t.Errorf("X-Forwarded-For 提取不符：%s", got)
	}
	req.Header.Del("X-Forwarded-For")
	req.Header.Set("X-Real-IP", "10.0.0.9")
	if got := c.RemoteIP(); got != "10.0.0.9" {
		t.Errorf("X-Real-IP 提取不符：%s", got)
	}
	// RemoteAddr 无端口
	req.Header.Del("X-Real-IP")
	req.RemoteAddr = "10.0.0.8"
	if got := c.RemoteIP(); got != "10.0.0.8" {
		t.Errorf("RemoteAddr 无端口提取不符：%s", got)
	}

	// 对端不在可信网段时，代理头不信任
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	req.RemoteAddr = "203.0.113.5:80"
	if got := c.RemoteIP(); got != "203.0.113.5" {
		t.Errorf("非可信代理不应采用 XFF：%s", got)
	}

	// 对端地址不可解析时按原样返回
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	req.RemoteAddr = "no-port"
	if got := c.RemoteIP(); got != "no-port" {
		t.Errorf("非法对端地址应按原样返回：%s", got)
	}
}

func mustParseCIDR(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, ipNet, err := net.ParseCIDR(cidr)
	testx.RequireNoError(t, err)

	return ipNet
}

func TestContextValues(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	c.Set("requestId", "abc")
	c.Set("count", 42)
	if got := c.RequestID(); got != "abc" {
		t.Errorf("RequestID 不符：%s", got)
	}
	if got := c.GetString("requestId"); got != "abc" {
		t.Errorf("GetString 不符：%s", got)
	}
	if got := c.GetString("missing"); got != "" {
		t.Errorf("缺失键 GetString 应为空：%s", got)
	}
	if got := c.GetString("count"); got != "" {
		t.Errorf("非字符串 GetString 应为空：%s", got)
	}
	if _, ok := c.Get("missing"); ok {
		t.Error("缺失键 Get 应返回 false")
	}
	if val, ok := c.Get("count"); !ok || val.(int) != 42 {
		t.Errorf("Get 不符：%v %v", val, ok)
	}
}

func TestContextParams(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := c.Param("id"); got != "" {
		t.Errorf("未设置参数时应为空：%s", got)
	}
	c.SetParams([]Param{{Name: "id", Value: "7"}})
	if got := c.Param("id"); got != "7" {
		t.Errorf("Param 不符：%s", got)
	}
	c.SetParams([]Param{{Name: "id", Value: "7"}, {Name: "id", Value: "9"}})
	if got := c.Param("id"); got != "9" {
		t.Errorf("Param 应返回最后一次匹配值：%s", got)
	}
	if got := c.Param("missing"); got != "" {
		t.Errorf("缺失参数应为空：%s", got)
	}
}

func TestContextRouteGroup(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := c.Route(); got != "" {
		t.Errorf("未设置路由时应为空：%s", got)
	}
	if got := c.Group(); got != "" {
		t.Errorf("未设置分组时应为空：%s", got)
	}
	c.SetRoute("/api/users/:id")
	c.SetGroup("/api")
	if got := c.Route(); got != "/api/users/:id" {
		t.Errorf("Route 不符：%s", got)
	}
	if got := c.Group(); got != "/api" {
		t.Errorf("Group 不符：%s", got)
	}
}

func TestContextStatusAndHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := c.StatusCode(); got != http.StatusOK {
		t.Errorf("未写入状态码时应为 200：%d", got)
	}
	c.Status(http.StatusCreated)
	c.Status(http.StatusInternalServerError) // 二次写入无效
	testx.Equal(t, rec.Code, http.StatusCreated)

	if got := c.StatusCode(); got != http.StatusCreated {
		t.Errorf("StatusCode 不符：%d", got)
	}
	c.Header("X-Test", "yes")
	if rec.Header().Get("X-Test") != "yes" {
		t.Error("Header 设置不符")
	}
}

func TestContextCanonicalHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header[CanonicalHeaderKey("X-Request-ID")] = []string{"incoming"}
	rec := httptest.NewRecorder()
	c := NewContext(rec, req)

	if got := CanonicalHeaderKey("Content-Type"); got != "Content-Type" {
		t.Errorf("规范化键不符：%s", got)
	}
	key := CanonicalHeaderKey("X-Request-ID")
	if got := c.GetHeaderCanonical(key); got != "incoming" {
		t.Errorf("GetHeaderCanonical 不符：%s", got)
	}
	if got := c.GetHeaderCanonical(CanonicalHeaderKey("X-Missing")); got != "" {
		t.Errorf("缺失头应返回空：%s", got)
	}

	ct := CanonicalHeaderKey("Content-Type")
	c.SetHeaderCanonical(ct, "application/json")
	c.SetHeaderCanonical(ct, "text/plain") // 复用已有切片
	if got := rec.Header().Get("Content-Type"); got != "text/plain" {
		t.Errorf("SetHeaderCanonical 复用不符：%s", got)
	}

	traceKey := CanonicalHeaderKey("X-Trace-ID")
	c.SetRequestHeaderCanonical(traceKey, "out-1") // 首次创建
	c.SetRequestHeaderCanonical(traceKey, "out-2") // 复用已有切片
	if got := req.Header.Get("X-Trace-ID"); got != "out-2" {
		t.Errorf("SetRequestHeaderCanonical 复用不符：%s", got)
	}
}

func TestContextStringFastPath(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	testx.RequireNoError(t, c.String(http.StatusOK, "hello"))
	if rec.Body.String() != "hello" {
		t.Errorf("无格式 String 不符：%s", rec.Body.String())
	}

	rec2 := httptest.NewRecorder()
	c2 := NewContext(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	testx.RequireNoError(t, c2.String(http.StatusOK, "%s", "格式化"))
	if rec2.Body.String() != "格式化" {
		t.Errorf("带参数 String 不符：%s", rec2.Body.String())
	}

	rec3 := httptest.NewRecorder()
	c3 := NewContext(rec3, httptest.NewRequest(http.MethodGet, "/", nil))
	_ = c3.String(http.StatusOK, "100%%") // 含 % 且无参数，走 Fprintf 分支
	if rec3.Body.String() != "100%" {
		t.Errorf("含 %% 无参数 String 不符：%s", rec3.Body.String())
	}
}

func TestContextRedirect(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Redirect(http.StatusFound, "/next")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/next" {
		t.Errorf("重定向不符：%d %v", rec.Code, rec.Header())
	}
	if !strings.Contains(rec.Body.String(), "/next") {
		t.Error("重定向响应体应包含目标地址")
	}
	if got := c.StatusCode(); got != http.StatusFound {
		t.Errorf("StatusCode 未记录：%d", got)
	}

	rec = httptest.NewRecorder()
	c = NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Redirect(http.StatusOK, "/coerced") // 非 3xx 强制转 302
	testx.Equal(t, rec.Code, http.StatusFound)

}

func TestContextCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "sid", Value: "abc"})
	rec := httptest.NewRecorder()
	c := NewContext(rec, req)
	cookie, err := c.Cookie("sid")
	if err != nil || cookie.Value != "abc" {
		t.Errorf("Cookie 读取不符：%v %v", cookie, err)
	}
	if _, err := c.Cookie("missing"); err == nil {
		t.Error("缺失 Cookie 应报错")
	}
	c.SetCookie(&http.Cookie{Name: "k", Value: "v"})
	if got := rec.Result().Cookies(); len(got) != 1 || got[0].Name != "k" || got[0].Value != "v" {
		t.Errorf("SetCookie 不符：%+v", got)
	}
}

func TestContextSetSecureCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetSecureCookie(&http.Cookie{Name: "sid", Value: "abc"})
	raw := rec.Header().Get("Set-Cookie")
	if !strings.Contains(raw, "Secure") || !strings.Contains(raw, "HttpOnly") ||
		!strings.Contains(raw, "SameSite=Lax") {
		t.Errorf("安全属性未补齐：%s", raw)
	}

	rec = httptest.NewRecorder()
	c = NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetSecureCookie(&http.Cookie{
		Name:     "custom",
		Value:    "x",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	raw = rec.Header().Get("Set-Cookie")
	if !strings.Contains(raw, "SameSite=Strict") || !strings.Contains(raw, "Secure") ||
		!strings.Contains(raw, "HttpOnly") {
		t.Errorf("显式属性不应被覆盖：%s", raw)
	}
}

func TestContextFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(filePath, []byte("文件内容"), 0o600); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/a.txt", nil)
	c := NewContext(rec, req)
	c.File(filePath)
	if rec.Code != http.StatusOK || rec.Body.String() != "文件内容" {
		t.Errorf("文件响应不符：%d %s", rec.Code, rec.Body.String())
	}
	if got := c.StatusCode(); got != http.StatusOK {
		t.Errorf("StatusCode 未记录：%d", got)
	}

	// 304：If-Modified-Since 命中
	info, err := os.Stat(filePath)
	testx.RequireNoError(t, err)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/a.txt", nil)
	req.Header.Set("If-Modified-Since", info.ModTime().UTC().Format(http.TimeFormat))
	c = NewContext(rec, req)
	c.File(filePath)
	testx.Equal(t, rec.Code, http.StatusNotModified)

	// 文件不存在 → 404
	rec = httptest.NewRecorder()
	c = NewContext(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
	c.File(filepath.Join(dir, "missing.txt"))
	testx.Equal(t, rec.Code, http.StatusNotFound)

	// stat 失败 → 500
	origStat := statFile
	statFile = func(string) (os.FileInfo, error) { return nil, errors.New("stat 失败") }
	rec = httptest.NewRecorder()
	c = NewContext(rec, httptest.NewRequest(http.MethodGet, "/a.txt", nil))
	c.File(filePath)
	statFile = origStat
	testx.Equal(t, rec.Code, http.StatusInternalServerError)

}

func TestStatusRecorder(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusRecorder{ResponseWriter: rec}
	sw.WriteHeader(http.StatusCreated)
	sw.WriteHeader(http.StatusInternalServerError)
	testx.Equal(t, sw.status, http.StatusCreated)

	testx.Equal(t, rec.Code, http.StatusCreated)

}

func TestContextJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	testx.RequireNoError(t, c.JSON(http.StatusOK, map[string]string{"k": "v"}))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("JSON 响应头/状态不符：%d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), `"k":"v"`) {
		t.Errorf("JSON 响应体不符：%s", rec.Body.String())
	}
}

func TestContextString(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	testx.RequireNoError(t, c.String(http.StatusOK, "hello %s", "webx"))
	if rec.Body.String() != "hello webx" {
		t.Errorf("String 响应体不符：%s", rec.Body.String())
	}
}

func TestContextBindJSON(t *testing.T) {
	var body struct {
		Name string `json:"name"`
	}
	c := NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/",
		bytes.NewBufferString(`{"name":"webx"}`)))
	if err := c.BindJSON(&body); err != nil || body.Name != "webx" {
		t.Errorf("BindJSON 不符：%v %+v", err, body)
	}
	bad := NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/",
		bytes.NewBufferString(`{bad`)))
	if err := bad.BindJSON(&body); err == nil {
		t.Error("非法 JSON 应返回错误")
	}
}

func TestContextBindJSONMaxBytes(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/",
		bytes.NewBufferString(`{"name":"这是一个超长请求体"}`))
	rec := httptest.NewRecorder()
	c := NewContext(rec, req)
	c.SetMaxBodyBytes(4)
	var body struct {
		Name string `json:"name"`
	}
	if err := c.BindJSON(&body); err == nil {
		t.Error("超过最大请求体应返回错误")
	}
	// 未设置限制时正常解析
	req2 := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"name":"ok"}`))
	c2 := NewContext(httptest.NewRecorder(), req2)
	if err := c2.BindJSON(&body); err != nil || body.Name != "ok" {
		t.Errorf("无限制 BindJSON 不符：%v %+v", err, body)
	}
}

func TestContextChain(t *testing.T) {
	var order []string
	mw1 := func(c *Context) {
		order = append(order, "mw1-before")
		c.Next()
		order = append(order, "mw1-after")
	}
	mw2 := func(c *Context) {
		order = append(order, "mw2")
		c.Next()
	}
	final := func(c *Context) {
		order = append(order, "final")
	}
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]HandlerFunc{mw1, mw2, final})
	c.Run()
	want := []string{"mw1-before", "mw2", "final", "mw1-after"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Errorf("链执行顺序不符：%v", order)
	}
	if c.IsAborted() {
		t.Error("正常执行不应 aborted")
	}
}

func TestContextChainAbort(t *testing.T) {
	var order []string
	abort := func(c *Context) {
		order = append(order, "abort")
		c.Abort()
		c.Next()
	}
	skip := func(c *Context) {
		order = append(order, "skip")
	}
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]HandlerFunc{abort, skip})
	c.Run()
	if strings.Join(order, ",") != "abort" {
		t.Errorf("Abort 后不应继续执行：%v", order)
	}
	if !c.IsAborted() {
		t.Error("IsAborted 应为 true")
	}
}

func TestContextResponseHelpers(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Set("requestId", "r1")
	c.Success("ok", "data")
	var resp StandardizedResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Code != CodeSuccess || resp.Msg != "ok" || resp.Data != "data" || resp.RequestID != "r1" {
		t.Errorf("Success 不符：%+v", resp)
	}

	rec = httptest.NewRecorder()
	c = NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Fail(http.StatusBadRequest, 40001, "参数错误")
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Code != 40001 || resp.Msg != "参数错误" {
		t.Errorf("Fail 不符：%+v", resp)
	}

	rec = httptest.NewRecorder()
	c = NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.JSONResponse(http.StatusNotFound, "不存在", nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Code != http.StatusNotFound || resp.Msg != "不存在" {
		t.Errorf("JSONResponse 不符：%+v", resp)
	}

	rec = httptest.NewRecorder()
	c = NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.AbortWithStatusJSON(http.StatusInternalServerError, "内部错误", nil)
	if !c.IsAborted() {
		t.Error("AbortWithStatusJSON 应终止链")
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Code != http.StatusInternalServerError || resp.Msg != "内部错误" {
		t.Errorf("AbortWithStatusJSON 不符：%+v", resp)
	}
}

func TestNoRouteAndNoMethodHandlers(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]HandlerFunc{NoRouteHandler})
	c.Run()
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "请求的资源不存在") {
		t.Errorf("NoRoute 不符：%d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	c = NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]HandlerFunc{NoMethodHandler})
	c.Run()
	if rec.Code != http.StatusMethodNotAllowed || !strings.Contains(rec.Body.String(), "不支持的请求方法") {
		t.Errorf("NoMethod 不符：%d %s", rec.Code, rec.Body.String())
	}
}

func TestContextWriterSwap(t *testing.T) {
	rec := httptest.NewRecorder()
	swapped := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetWriter(swapped)
	if c.Writer() != swapped {
		t.Error("SetWriter 不符")
	}
	req2 := httptest.NewRequest(http.MethodPost, "/", nil)
	c.SetRequest(req2)
	if c.Request() != req2 {
		t.Error("SetRequest 不符")
	}
}

func TestContextResetWriteState(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Status(http.StatusBadGateway)
	c.ResetWriteState()
	if got := c.StatusCode(); got != http.StatusOK {
		t.Errorf("重置后 StatusCode 应为 200：%d", got)
	}
	c.Status(http.StatusCreated)
	if got := c.StatusCode(); got != http.StatusCreated {
		t.Errorf("重置后应可重新写状态码：%d", got)
	}
}

func TestContextPoolReuse(t *testing.T) {
	c := Acquire(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	c.Set("k", "v")
	c.SetParams([]Param{{Name: "id", Value: "1"}})
	c.SetRoute("/r")
	c.SetGroup("/g")
	c.SetTrustedProxies([]*net.IPNet{mustParseCIDR(t, "10.0.0.0/8")})
	c.Status(http.StatusCreated)
	c.SetMaxBodyBytes(5)
	c.SetHandlers([]HandlerFunc{func(c *Context) {}})
	Release(c)

	c2 := Acquire(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	defer Release(c2)
	if c2.values != nil || c2.params != nil || c2.handlers != nil {
		t.Error("Reset 未清空状态")
	}
	if c2.route != "" || c2.group != "" || c2.trusted != nil {
		t.Error("Reset 未清空路由/分组/可信代理信息")
	}
	if c2.StatusCode() != http.StatusOK || c2.maxBody != 0 || c2.IsAborted() {
		t.Errorf("Reset 后默认状态不符：%d %d %v", c2.StatusCode(), c2.maxBody, c2.IsAborted())
	}
}

func TestContextRunReset(t *testing.T) {
	var n int
	h := func(c *Context) { n++ }
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]HandlerFunc{h, h})
	c.Run()
	c.Run() // 再次从头执行
	if n != 4 {
		t.Errorf("Run 应重置索引：%d", n)
	}
}

func TestNextWithoutHandlers(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Run()
	if c.IsAborted() {
		t.Error("空链不应 aborted")
	}
}
