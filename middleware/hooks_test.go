package middleware

import (
	testx "github.com/lcylpzls/testx"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lcylpzls/webx/internal/core"
)

func TestHooks(t *testing.T) {
	var order []string
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		stdToCore(Hooks(
			func(c *core.Context) { order = append(order, "request") },
			func(c *core.Context) { order = append(order, "response") },
		)),
		func(c *core.Context) { order = append(order, "handler"); c.Success("ok", nil) },
	})
	c.Run()
	if len(order) != 3 || order[0] != "request" || order[1] != "handler" || order[2] != "response" {
		t.Errorf("钩子顺序不符：%v", order)
	}
}

func TestHooksNil(t *testing.T) {
	var called bool
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		stdToCore(Hooks(nil, nil)),
		func(c *core.Context) { called = true; c.Success("ok", nil) },
	})
	c.Run()
	testx.True(t, called)

}
