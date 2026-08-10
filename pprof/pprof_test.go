package pprof

import (
	testx "github.com/lcylpzls/testx"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lcylpzls/webx"
)

type fakeRegistrar struct {
	routes []webx.Route
}

func (f *fakeRegistrar) RegisterRoute(r webx.Route) *webx.Server {
	f.routes = append(f.routes, r)
	return nil
}

func TestRegister(t *testing.T) {
	f := &fakeRegistrar{}
	Register(f)
	if len(f.routes) != 3 {
		t.Fatalf("注册路由数量不符：%d", len(f.routes))
	}
	rt := webx.NewRouter(webx.NoRouteHandler, webx.NoMethodHandler)
	for _, r := range f.routes {
		if err := rt.Handle(r.Method, r.Path, []webx.HandlerFunc{r.Handler}); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"/debug/pprof", "/debug/pprof/cmdline", "/debug/pprof/symbol"} {
		rec := httptest.NewRecorder()
		rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		testx.Equal(t, rec.Code, http.StatusOK)

	}
}
