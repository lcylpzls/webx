package webx

import "github.com/lcylpzls/webx/internal/core"

// HandlerFunc 是 webx 的业务处理器签名，不依赖任何第三方类型。
type HandlerFunc = core.HandlerFunc

// Route 定义一条 HTTP 路由。
type Route struct {
	// Method HTTP 方法，如 GET、POST、PUT、DELETE、PATCH。
	Method string
	// Path 路由路径，支持 gin 风格 "/api/users/:id" 与 "/assets/*filepath"。
	Path string
	// Handler 路由处理器。
	Handler HandlerFunc
	// Middleware 路由专属中间件（可选），仅对当前路由生效。
	Middleware []HandlerFunc
}

// RouteGroup 路由分组，支持嵌套分组和分组级中间件。
// 仅缓冲注册，Start() 时一次性挂载。
type RouteGroup struct {
	prefix     string
	routes     []Route
	groups     []*RouteGroup
	middleware []HandlerFunc
}

// GET 注册一条 GET 方法路由。
func (rg *RouteGroup) GET(path string, handler HandlerFunc, mw ...HandlerFunc) {
	rg.register("GET", path, handler, mw...)
}

// POST 注册一条 POST 方法路由。
func (rg *RouteGroup) POST(path string, handler HandlerFunc, mw ...HandlerFunc) {
	rg.register("POST", path, handler, mw...)
}

// PUT 注册一条 PUT 方法路由。
func (rg *RouteGroup) PUT(path string, handler HandlerFunc, mw ...HandlerFunc) {
	rg.register("PUT", path, handler, mw...)
}

// DELETE 注册一条 DELETE 方法路由。
func (rg *RouteGroup) DELETE(path string, handler HandlerFunc, mw ...HandlerFunc) {
	rg.register("DELETE", path, handler, mw...)
}

// PATCH 注册一条 PATCH 方法路由。
func (rg *RouteGroup) PATCH(path string, handler HandlerFunc, mw ...HandlerFunc) {
	rg.register("PATCH", path, handler, mw...)
}

// HEAD 注册一条 HEAD 方法路由。
func (rg *RouteGroup) HEAD(path string, handler HandlerFunc, mw ...HandlerFunc) {
	rg.register("HEAD", path, handler, mw...)
}

// OPTIONS 注册一条 OPTIONS 方法路由。
func (rg *RouteGroup) OPTIONS(path string, handler HandlerFunc, mw ...HandlerFunc) {
	rg.register("OPTIONS", path, handler, mw...)
}

// register 追加一条路由，继承分组中间件。
func (rg *RouteGroup) register(method, path string, handler HandlerFunc, mw ...HandlerFunc) {
	rg.routes = append(rg.routes, Route{
		Method:     method,
		Path:       rg.prefix + path,
		Handler:    handler,
		Middleware: append(append([]HandlerFunc{}, rg.middleware...), mw...),
	})
}

// Use 向当前分组追加中间件，影响该分组内所有已注册和后续注册的路由。
func (rg *RouteGroup) Use(middleware ...HandlerFunc) {
	rg.middleware = append(rg.middleware, middleware...)
}

// Group 创建子分组，继承父分组 prefix 与中间件。
func (rg *RouteGroup) Group(relativePath string) *RouteGroup {
	sub := &RouteGroup{
		prefix:     rg.prefix + relativePath,
		middleware: append([]HandlerFunc{}, rg.middleware...),
	}
	rg.groups = append(rg.groups, sub)
	return sub
}

// flatten 递归展平分组中所有路由。
func (rg *RouteGroup) flatten() []Route {
	var all []Route
	all = append(all, rg.routes...)
	for _, g := range rg.groups {
		all = append(all, g.flatten()...)
	}
	return all
}
