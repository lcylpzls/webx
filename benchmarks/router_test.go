package benchmarks

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/buaazp/fasthttprouter"
	"github.com/gin-gonic/gin"
	"github.com/labstack/echo/v4"
	"github.com/lcylpzls/webx/v2"
	"github.com/valyala/fasthttp"
)

// routePaths500 生成 500 条参数化路由（与 webx CI 基准同构）。
func routePaths500() []string {
	paths := make([]string, 0, 500)
	for i := 0; i < 500; i++ {
		paths = append(paths, fmt.Sprintf("/api/v1/resource/%d/:id", i))
	}
	return paths
}

const benchRequestPath = "/api/v1/resource/25/42"

func BenchmarkRouterServeMux500(b *testing.B) {
	mux := http.NewServeMux()
	for i := 0; i < 500; i++ {
		mux.HandleFunc("GET "+fmt.Sprintf("/api/v1/resource/%d/{id}", i), func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "hello")
		})
	}
	req := httptest.NewRequest(http.MethodGet, benchRequestPath, nil)
	rec := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(rec, req)
	}
}

func BenchmarkRouterWebx500(b *testing.B) {
	rt := webx.NewRouter(webx.NoRouteHandler, webx.NoMethodHandler)
	handler := func(c *webx.Context) { _ = c.String(http.StatusOK, "hello") }
	for _, p := range routePaths500() {
		if err := rt.Handle(http.MethodGet, p, []webx.HandlerFunc{handler}); err != nil {
			b.Fatal(err)
		}
	}
	req := httptest.NewRequest(http.MethodGet, benchRequestPath, nil)
	rec := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.ServeHTTP(rec, req)
	}
}

func BenchmarkRouterGin500(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	handler := func(c *gin.Context) { c.String(http.StatusOK, "hello") }
	for _, p := range routePaths500() {
		r.GET(p, handler)
	}
	req := httptest.NewRequest(http.MethodGet, benchRequestPath, nil)
	rec := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(rec, req)
	}
}

func BenchmarkRouterEcho500(b *testing.B) {
	e := echo.New()
	handler := func(c echo.Context) error { return c.String(http.StatusOK, "hello") }
	for _, p := range routePaths500() {
		e.GET(p, handler)
	}
	req := httptest.NewRequest(http.MethodGet, benchRequestPath, nil)
	rec := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ServeHTTP(rec, req)
	}
}

func BenchmarkRouterFasthttp500(b *testing.B) {
	r := fasthttprouter.New()
	handler := func(ctx *fasthttp.RequestCtx) { _, _ = ctx.WriteString("hello") }
	for _, p := range routePaths500() {
		r.GET(p, handler)
	}
	var ctx fasthttp.RequestCtx
	ctx.Init(&fasthttp.Request{}, nil, nil)
	ctx.Request.SetRequestURI(benchRequestPath)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.SetRequestURI(benchRequestPath)
		r.Handler(&ctx)
	}
}
