package webx

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcylpzls/webx/internal/core"
)

func TestTranslateGinPattern(t *testing.T) {
	cases := []struct {
		in       string
		want     string
		wantParm []string
	}{
		{"/api/users", "/api/users", nil},
		{"/api/users/:id", "/api/users/{id}", []string{"id"}},
		{"/api/:ver/users/:id", "/api/{ver}/users/{id}", []string{"ver", "id"}},
		{"/assets/*filepath", "/assets/{filepath...}", []string{"filepath"}},
	}
	for _, tc := range cases {
		got, params, err := translateGinPattern(tc.in)
		if err != nil {
			t.Errorf("translate(%s) 失败：%v", tc.in, err)
			continue
		}
		if got != tc.want || strings.Join(params, ",") != strings.Join(tc.wantParm, ",") {
			t.Errorf("translate(%s) 不符：got %s %v, want %s %v", tc.in, got, params, tc.want, tc.wantParm)
		}
	}
}

func TestTranslateGinPatternErrors(t *testing.T) {
	cases := []string{
		"/x/:",
		"/x/:bad-name",
		"/x/*",
		"/x/*bad-name",
		"/x/*p/rest",
	}
	for _, in := range cases {
		if _, _, err := translateGinPattern(in); err == nil {
			t.Errorf("translate(%s) 应返回错误", in)
		}
	}
}

func TestParsePatternWildcardNotLast(t *testing.T) {
	if _, err := parsePattern("/a/{p...}/b"); err == nil {
		t.Error("通配段不在末尾应返回错误")
	}
	if _, err := parsePattern("/a/{p...}"); err != nil {
		t.Errorf("合法通配模式应通过：%v", err)
	}
}

func TestRouterParamsAndMethods(t *testing.T) {
	rt := NewRouter(core.NoRouteHandler, core.NoMethodHandler)
	echo := func(c *core.Context) {
		_ = c.String(http.StatusOK, "%s|%s", c.Param("id"), c.Param("filepath"))
	}
	if err := rt.Handle("GET", "/api/users/:id", []core.HandlerFunc{echo}); err != nil {
		t.Fatal(err)
	}
	if err := rt.Handle("POST", "/api/users/:id", []core.HandlerFunc{echo}); err != nil {
		t.Fatal(err)
	}
	if err := rt.Handle("GET", "/assets/*filepath", []core.HandlerFunc{echo}); err != nil {
		t.Fatal(err)
	}

	// GET 参数
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/users/42", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "42|" {
		t.Errorf("GET 参数不符：%d %s", rec.Code, rec.Body.String())
	}
	// POST
	rec = httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/users/7", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "7|" {
		t.Errorf("POST 参数不符：%d %s", rec.Code, rec.Body.String())
	}
	// 通配
	rec = httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/css/app.css", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "|css/app.css" {
		t.Errorf("通配参数不符：%d %s", rec.Code, rec.Body.String())
	}
}

func TestRouterMethodNotAllowed(t *testing.T) {
	rt := NewRouter(core.NoRouteHandler, core.NoMethodHandler)
	if err := rt.Handle("GET", "/api/x", []core.HandlerFunc{noopCore}); err != nil {
		t.Fatal(err)
	}
	if err := rt.Handle("POST", "/api/x", []core.HandlerFunc{noopCore}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/x", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("405 状态不符：%d", rec.Code)
	}
	if rec.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Errorf("Allow 头不符：%s", rec.Header().Get("Allow"))
	}
	if !strings.Contains(rec.Body.String(), "不支持的请求方法") {
		t.Errorf("405 响应体不符：%s", rec.Body.String())
	}
}

func TestRouterHeadOnGetRoute(t *testing.T) {
	rt := NewRouter(core.NoRouteHandler, core.NoMethodHandler)
	if err := rt.Handle("GET", "/api/x", []core.HandlerFunc{
		func(c *core.Context) { _ = c.String(http.StatusOK, "ok") },
	}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodHead, "/api/x", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("HEAD 应命中 GET 路由：%d", rec.Code)
	}
}

func TestRouterSpecificityMethodDecision(t *testing.T) {
	rt := NewRouter(core.NoRouteHandler, core.NoMethodHandler)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "other.txt"), []byte("static"), 0o600)
	if err := rt.HandleStatic("", http.Dir(dir)); err != nil {
		t.Fatal(err)
	}
	if err := rt.Handle("POST", "/api/{id}", []core.HandlerFunc{noopCore}); err != nil {
		t.Fatal(err)
	}

	// 最具体模式（POST 参数路由）不允许 GET → 405，而非落到静态根
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/42", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("最具体模式方法判定不符：%d", rec.Code)
	}
	if rec.Header().Get("Allow") != "POST" {
		t.Errorf("Allow 头不符：%s", rec.Header().Get("Allow"))
	}
	// 未命中任何具体路由 → 静态根服务
	rec = httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/other.txt", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "static") {
		t.Errorf("静态根服务不符：%d %s", rec.Code, rec.Body.String())
	}
}

func TestRouterExactNoPrefixMatch(t *testing.T) {
	rt := NewRouter(core.NoRouteHandler, core.NoMethodHandler)
	if err := rt.Handle("GET", "/api/users/:id", []core.HandlerFunc{noopCore}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/users/42/extra", "/api/users/42/"} {
		rec := httptest.NewRecorder()
		rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("精确路由不应前缀匹配 %s：%d", path, rec.Code)
		}
	}
}

func TestRouterNotFoundAndRedirect(t *testing.T) {
	rt := NewRouter(core.NoRouteHandler, core.NoMethodHandler)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o600)
	if err := rt.HandleStatic("/static", http.Dir(dir)); err != nil {
		t.Fatal(err)
	}

	// 404
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "请求的资源不存在") {
		t.Errorf("404 不符：%d %s", rec.Code, rec.Body.String())
	}
	// 静态文件
	rec = httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/a.txt", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "hello" {
		t.Errorf("静态文件不符：%d %s", rec.Code, rec.Body.String())
	}
	// 子树模式精确命中（尾斜杠）
	rec = httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("子树模式尾斜杠应命中：%d", rec.Code)
	}
	// 尾斜杠重定向
	rec = httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static", nil))
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/static/" {
		t.Errorf("重定向不符：%d %s", rec.Code, rec.Header().Get("Location"))
	}
	// 静态目录 405
	rec = httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/static/a.txt", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("静态 405 不符：%d", rec.Code)
	}
}

func TestRouterHandleErrors(t *testing.T) {
	rt := NewRouter(core.NoRouteHandler, core.NoMethodHandler)
	chain := []core.HandlerFunc{noopCore}
	if err := rt.Handle("", "/x", chain); err == nil {
		t.Error("空方法应报错")
	}
	if err := rt.Handle("GET", "/x/:", chain); err == nil {
		t.Error("非法参数应报错")
	}
	if err := rt.Handle("GET", "/x/*a/b", chain); err == nil {
		t.Error("通配不在末尾应报错")
	}
	if err := rt.Handle("GET", "/same", chain); err != nil {
		t.Fatal(err)
	}
	if err := rt.Handle("GET", "/same", chain); err == nil {
		t.Error("重复注册同路径应报错")
	}
	if err := rt.Handle("POST", "/same", chain); err != nil {
		t.Errorf("不同方法同路径应允许：%v", err)
	}
	if err := rt.Handle("GET", "/conflict/:a", chain); err != nil {
		t.Fatal(err)
	}
	if err := rt.Handle("GET", "/conflict/:b", chain); err == nil {
		t.Error("参数名冲突应报错")
	}
	if err := rt.HandleStatic("/static", http.Dir(t.TempDir())); err != nil {
		t.Errorf("静态注册失败：%v", err)
	}
	// 静态路径重复注册：GET 冲突
	if err := rt.HandleStatic("/static", http.Dir(t.TempDir())); err == nil {
		t.Error("重复注册静态路径应报错")
	}
}

func TestRouterStaticRootAndShortWildcard(t *testing.T) {
	rt := NewRouter(core.NoRouteHandler, core.NoMethodHandler)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "index.html"), []byte("root"), 0o600)
	if err := rt.HandleStatic("", http.Dir(dir)); err != nil {
		t.Fatal(err)
	}
	if err := rt.Handle("GET", "/a/b/*filepath", []core.HandlerFunc{noopCore}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "root") {
		t.Errorf("根静态服务不符：%d %s", rec.Code, rec.Body.String())
	}
	// 通配模式路径过短 → 404
	rec = httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/a", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("通配模式路径过短应 404：%d", rec.Code)
	}
}

func TestRouterCustomFallbacks(t *testing.T) {
	noRoute := func(c *core.Context) {
		_ = c.String(http.StatusTeapot, "自定义404")
	}
	noMethod := func(c *core.Context) {
		_ = c.String(http.StatusTeapot, "自定义405")
	}
	rt := NewRouter(noRoute, noMethod)
	if err := rt.Handle("GET", "/x", []core.HandlerFunc{noopCore}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rec.Code != http.StatusTeapot || rec.Body.String() != "自定义404" {
		t.Errorf("自定义 404 不符：%d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", nil))
	if rec.Code != http.StatusTeapot || rec.Body.String() != "自定义405" {
		t.Errorf("自定义 405 不符：%d %s", rec.Code, rec.Body.String())
	}
}

func noopCore(*core.Context) {}
