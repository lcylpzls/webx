package core

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	// X-Forwarded-For 优先
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
	c.SetParams(map[string]string{"id": "7"})
	if got := c.Param("id"); got != "7" {
		t.Errorf("Param 不符：%s", got)
	}
	if got := c.Param("missing"); got != "" {
		t.Errorf("缺失参数应为空：%s", got)
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
	if rec.Code != http.StatusCreated {
		t.Errorf("状态码不符：%d", rec.Code)
	}
	if got := c.StatusCode(); got != http.StatusCreated {
		t.Errorf("StatusCode 不符：%d", got)
	}
	c.Header("X-Test", "yes")
	if rec.Header().Get("X-Test") != "yes" {
		t.Error("Header 设置不符")
	}
}

func TestContextJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if err := c.JSON(http.StatusOK, map[string]string{"k": "v"}); err != nil {
		t.Fatalf("JSON 写入失败：%v", err)
	}
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
	if err := c.String(http.StatusOK, "hello %s", "webx"); err != nil {
		t.Fatalf("String 写入失败：%v", err)
	}
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
