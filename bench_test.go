package webx

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lcylpzls/webx/internal/core"
)

// discardWriter 是零分配的最小响应写入器，用于基准测试。
type discardWriter struct {
	h http.Header
}

func (d *discardWriter) Header() http.Header         { return d.h }
func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }
func (d *discardWriter) WriteHeader(int)             {}

func BenchmarkRouterServeHTTP(b *testing.B) {
	rt := NewRouter(core.NoRouteHandler, core.NoMethodHandler)
	for i := 0; i < 50; i++ {
		if err := rt.Handle("GET", fmt.Sprintf("/api/v1/resource/%d/:id", i), []core.HandlerFunc{noopCore}); err != nil {
			b.Fatal(err)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/resource/25/42", nil)
	dw := &discardWriter{h: make(http.Header)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.ServeHTTP(dw, req)
	}
}

func BenchmarkMiddlewareChain(b *testing.B) {
	mw := func(c *core.Context) { c.Next() }
	chain := []core.HandlerFunc{mw, mw, mw, func(c *core.Context) {}}
	dw := &discardWriter{h: make(http.Header)}
	c := core.NewContext(dw, httptest.NewRequest(http.MethodGet, "/", nil))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.SetHandlers(chain)
		c.Run()
	}
}

func BenchmarkJSONResponse(b *testing.B) {
	dw := &discardWriter{h: make(http.Header)}
	c := core.NewContext(dw, httptest.NewRequest(http.MethodGet, "/", nil))
	resp := StandardizedResponse{Code: CodeSuccess, Msg: "ok", RequestID: "r", Timestamp: 1}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.JSON(http.StatusOK, resp)
	}
}
