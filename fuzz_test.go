package webx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lcylpzls/webx/internal/core"
)

func FuzzTranslatePattern(f *testing.F) {
	f.Add("/api/users/:id")
	f.Add("/assets/*filepath")
	f.Add("/x/:")
	f.Add("/x/*p/rest")

	f.Fuzz(func(t *testing.T, path string) {
		_, _, _ = translateGinPattern(path)
	})
}

func FuzzRouterServeHTTP(f *testing.F) {
	f.Add("GET", "/api/users/:id", "/api/users/42")
	f.Add("POST", "/assets/*filepath", "/assets/a/b.txt")
	f.Add("", "", "")

	f.Fuzz(func(t *testing.T, method, pattern, path string) {
		rt := NewRouter(core.NoRouteHandler, core.NoMethodHandler)
		_ = rt.Handle(method, pattern, []core.HandlerFunc{noopCore})
		req, err := http.NewRequest(method, path, nil)
		if err != nil {
			return
		}
		rt.ServeHTTP(httptest.NewRecorder(), req)
	})
}
