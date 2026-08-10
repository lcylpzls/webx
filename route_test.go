package webx

import (
	"testing"

	"github.com/lcylpzls/webx/v2/internal/core"
)

func TestRouteGroupMethods(t *testing.T) {
	rg := &RouteGroup{prefix: "/api"}
	rg.GET("/a", noopHandler)
	rg.POST("/b", noopHandler)
	rg.PUT("/c", noopHandler)
	rg.DELETE("/d", noopHandler)
	rg.PATCH("/e", noopHandler)
	rg.HEAD("/f", noopHandler)
	rg.OPTIONS("/g", noopHandler)

	routes := rg.flatten()
	want := []struct {
		method, path string
	}{
		{"GET", "/api/a"}, {"POST", "/api/b"}, {"PUT", "/api/c"},
		{"DELETE", "/api/d"}, {"PATCH", "/api/e"},
		{"HEAD", "/api/f"}, {"OPTIONS", "/api/g"},
	}
	if len(routes) != len(want) {
		t.Fatalf("路由数量不符：%d", len(routes))
	}
	for i, w := range want {
		if routes[i].Method != w.method || routes[i].Path != w.path {
			t.Errorf("第 %d 条路由不符：%+v", i, routes[i])
		}
	}
}

func TestRouteGroupMiddlewareInheritance(t *testing.T) {
	rg := &RouteGroup{prefix: "/api"}
	rg.Use(mwA)
	rg.GET("/before", noopHandler)
	rg.GET("/after", noopHandler, mwB)

	routes := rg.flatten()
	if len(routes[0].Middleware) != 1 || routes[0].Middleware[0] == nil {
		t.Error("分组中间件未继承")
	}
	if len(routes[1].Middleware) != 2 {
		t.Errorf("路由级中间件未追加：%d", len(routes[1].Middleware))
	}

	rg.Use(mwC)
	routes = rg.flatten()
	if len(routes[0].Middleware) != 1 {
		t.Error("Use 不应影响已注册路由")
	}
}

func TestRouteGroupNested(t *testing.T) {
	root := &RouteGroup{prefix: ""}
	root.Use(mwA)
	admin := root.Group("/admin")
	admin.Use(mwB)
	admin.GET("/users", noopHandler)

	routes := root.flatten()
	if len(routes) != 1 {
		t.Fatalf("嵌套路由数量不符：%d", len(routes))
	}
	r := routes[0]
	if r.Path != "/admin/users" {
		t.Errorf("嵌套前缀不符：%s", r.Path)
	}
	if len(r.Middleware) != 2 {
		t.Errorf("嵌套中间件继承不符：%d", len(r.Middleware))
	}
}

func TestRouteGroupEmpty(t *testing.T) {
	rg := &RouteGroup{prefix: "/x"}
	if routes := rg.flatten(); len(routes) != 0 {
		t.Errorf("空分组应无路由：%v", routes)
	}
}

func noopHandler(*core.Context) {}

func mwA(c *core.Context) { c.Next() }
func mwB(c *core.Context) { c.Next() }
func mwC(c *core.Context) { c.Next() }
