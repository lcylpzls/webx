package webx

import (
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/lcylpzls/errx"
	"github.com/lcylpzls/webx/internal/core"
)

// Router 基于自研 radix 匹配树实现路由：
// 支持 gin 风格语法（:id / *filepath）、404/405 标准化 JSON 与尾斜杠重定向。
// 匹配与分发均由自身完成，不依赖 http.ServeMux。
type Router struct {
	root     *routeNode
	noRoute  core.HandlerFunc
	noMethod core.HandlerFunc
	maxBody  int64
	paramBuf sync.Pool
}

// routeNode 是匹配树中的一个节点。
type routeNode struct {
	literal    string
	param      string
	wildcard   bool
	subtree    bool
	handlers   map[string]requestHandler
	children   map[string]*routeNode
	paramChild *routeNode
	wildChild  *routeNode
}

// requestHandler 是带路由参数的最终处理器。
// 参数以切片传入（路由层维护缓冲池，热路径零分配）。
type requestHandler func(http.ResponseWriter, *http.Request, []core.Param)

// segment 是路径模式中的一段。
type segment struct {
	literal  string
	param    string
	wildcard bool
}

// NewRouter 创建路由，并指定 404/405 兜底处理器。
func NewRouter(noRoute, noMethod core.HandlerFunc) *Router {
	rt := &Router{
		noRoute:  noRoute,
		noMethod: noMethod,
	}
	rt.paramBuf.New = func() any {
		buf := make([]core.Param, 0, 8)
		return &buf
	}
	return rt
}

// SetMaxBodyBytes 设置路由处理链中 BindJSON 的最大请求体字节数。
func (rt *Router) SetMaxBodyBytes(n int64) {
	rt.maxBody = n
}

// Handle 注册一条路由（chain 为全局中间件 + 路由中间件 + 最终处理器的完整链）。
func (rt *Router) Handle(method, path string, chain []core.HandlerFunc) error {
	translated, params, err := translateGinPattern(path)
	if err != nil {
		return err
	}
	if method == "" {
		return errx.NewCodef(CodeRouteInvalid, "webx：路由方法不能为空：%s", path)
	}
	// translateGinPattern 已保证规范形式合法，这里不再重复校验。
	segs, _ := parsePattern(translated)
	return rt.insert(method, segs, wrapRequestHandler(rt, chain, params), false)
}

// HandleStatic 注册静态文件服务（支持子树路径）。
func (rt *Router) HandleStatic(prefix string, fs http.FileSystem) error {
	return rt.HandleStaticWithOptions(prefix, fs, StaticOptions{})
}

// HandleStaticWithOptions 注册静态文件服务（含缓存头/目录索引选项）。
func (rt *Router) HandleStaticWithOptions(prefix string, fs http.FileSystem, opts StaticOptions) error {
	pattern := prefix
	if pattern == "" {
		pattern = "/"
	}
	if !strings.HasSuffix(pattern, "/") {
		pattern += "/"
	}
	segs, err := parsePattern(pattern)
	if err != nil {
		return err
	}
	strip := strings.TrimSuffix(pattern, "/")
	h := http.StripPrefix(strip, staticOptionsFileServer(fs, opts))
	handler := func(w http.ResponseWriter, r *http.Request, _ []core.Param) {
		h.ServeHTTP(w, r)
	}
	if err := rt.insert("GET", segs, handler, true); err != nil {
		return err
	}
	return rt.insert("HEAD", segs, handler, true)
}

// insert 将模式段插入匹配树。
func (rt *Router) insert(method string, segs []segment, handler requestHandler, subtree bool) error {
	if rt.root == nil {
		rt.root = &routeNode{children: map[string]*routeNode{}}
	}
	node := rt.root
	for _, seg := range segs {
		switch {
		case seg.wildcard:
			if node.wildChild == nil {
				node.wildChild = &routeNode{wildcard: true, param: seg.param, children: map[string]*routeNode{}}
			}
			if node.wildChild.param != seg.param {
				return errx.NewCodef(CodeRouteInvalid, "webx：通配参数名冲突：%s 与 %s", node.wildChild.param, seg.param)
			}
			node = node.wildChild
		case seg.param != "":
			if node.paramChild == nil {
				node.paramChild = &routeNode{param: seg.param, children: map[string]*routeNode{}}
			}
			if node.paramChild.param != seg.param {
				return errx.NewCodef(CodeRouteInvalid, "webx：路由参数名冲突：%s 与 %s", node.paramChild.param, seg.param)
			}
			node = node.paramChild
		default:
			if node.children[seg.literal] == nil {
				node.children[seg.literal] = &routeNode{literal: seg.literal, children: map[string]*routeNode{}}
			}
			node = node.children[seg.literal]
		}
	}
	if node.handlers == nil {
		node.handlers = map[string]requestHandler{}
	}
	if _, dup := node.handlers[method]; dup {
		return errx.NewCodef(CodeRouteInvalid, "webx：路由重复注册：%s", method)
	}
	if len(node.handlers) > 0 && node.subtree != subtree {
		return errx.NewCode(CodeRouteInvalid, "webx：精确路由与子树路由冲突")
	}
	node.handlers[method] = handler
	if subtree {
		node.subtree = true
	}
	return nil
}

// ServeHTTP 实现 http.Handler：树匹配 + 方法判定 + 分发。
func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	buf := rt.paramBuf.Get().(*[]core.Param)
	params := (*buf)[:0]
	defer func() {
		*buf = params
		rt.paramBuf.Put(buf)
	}()
	node, ok := rt.lookup(r.URL.Path, &params)
	if !ok {
		if !strings.HasSuffix(r.URL.Path, "/") {
			if n, ok2 := rt.lookup(r.URL.Path+"/", &params); ok2 && n.subtree {
				w.Header().Set("Location", r.URL.Path+"/")
				w.WriteHeader(http.StatusMovedPermanently)
				return
			}
		}
		rt.runFallback(w, r, rt.noRoute)
		return
	}
	handler, ok := node.handlers[r.Method]
	if !ok && r.Method == http.MethodHead {
		handler, ok = node.handlers[http.MethodGet]
	}
	if !ok {
		if r.Method == http.MethodOptions {
			// 未显式注册 OPTIONS 时按 Allow 自动响应 204。
			w.Header().Set("Allow", allowHeader(node))
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Allow", allowHeader(node))
		rt.runFallback(w, r, rt.noMethod)
		return
	}
	handler(w, r, params)
}

// lookup 在匹配树中查找路径对应的节点，并把沿途参数追加到 params。
func (rt *Router) lookup(path string, params *[]core.Param) (*routeNode, bool) {
	if rt.root == nil {
		return nil, false
	}
	cur := rt.root
	idx := 0
	var best *routeNode

	for cur != nil {
		// 子树候选：路径还有内容（含尾斜杠）时记录最浅匹配
		if cur.subtree && (idx < len(path) || strings.HasSuffix(path, "/")) {
			best = cur
		}
		if idx >= len(path) {
			if len(cur.handlers) > 0 {
				if cur.subtree && strings.HasSuffix(path, "/") {
					return cur, true
				}
				if !cur.subtree && !strings.HasSuffix(path, "/") {
					return cur, true
				}
			}
			if cur.wildChild != nil && strings.HasSuffix(path, "/") {
				*params = append(*params, core.Param{Name: cur.wildChild.param})
				return cur.wildChild, true
			}
			break
		}
		seg, next := nextSegment(path, idx)
		if child := cur.children[seg]; child != nil {
			cur = child
			idx = next
			if idx < 0 {
				idx = len(path)
			}
			continue
		}
		if cur.paramChild != nil {
			*params = append(*params, core.Param{Name: cur.paramChild.param, Value: seg})
			cur = cur.paramChild
			idx = next
			if idx < 0 {
				idx = len(path)
			}
			continue
		}
		if cur.wildChild != nil {
			*params = append(*params, core.Param{Name: cur.wildChild.param, Value: path[idx:]})
			return cur.wildChild, true
		}
		break
	}
	if best != nil {
		return best, true
	}
	return nil, false
}

// allowHeader 汇总节点方法，生成 Allow 响应头（GET 附带 HEAD）。
func allowHeader(node *routeNode) string {
	set := make(map[string]bool, len(node.handlers)+1)
	for m := range node.handlers {
		set[m] = true
	}
	if set[http.MethodGet] {
		set[http.MethodHead] = true
	}
	methods := make([]string, 0, len(set))
	for m := range set {
		methods = append(methods, m)
	}
	sort.Strings(methods)
	return strings.Join(methods, ", ")
}

// wrapRequestHandler 将 webx 处理器链包装为带参数的最终处理器。
func wrapRequestHandler(rt *Router, chain []core.HandlerFunc, params []string) requestHandler {
	return func(w http.ResponseWriter, r *http.Request, matched []core.Param) {
		c := core.From(r.Context())
		if c == nil {
			c = core.Acquire(w, r)
			defer core.Release(c)
		}
		c.SetWriter(w)
		c.SetRequest(r)
		if len(params) > 0 {
			c.SetParams(matched)
		}
		if rt.maxBody > 0 {
			c.SetMaxBodyBytes(rt.maxBody)
		}
		c.SetHandlers(chain)
		c.Run()
	}
}

// runFallback 执行 404/405 兜底处理器。
func (rt *Router) runFallback(w http.ResponseWriter, r *http.Request, h core.HandlerFunc) {
	c := core.From(r.Context())
	if c == nil {
		c = core.Acquire(w, r)
		defer core.Release(c)
	}
	c.SetWriter(w)
	c.SetRequest(r)
	c.SetHandlers([]core.HandlerFunc{h})
	c.Run()
}

// translateGinPattern 将 gin 风格路径翻译为规范形式：
// ":name" → "{name}"，"*name" → "{name...}"。
func translateGinPattern(path string) (string, []string, error) {
	var b strings.Builder
	var params []string
	i := 0
	for i < len(path) {
		switch path[i] {
		case ':':
			j := i + 1
			for j < len(path) && path[j] != '/' && path[j] != ':' && path[j] != '*' {
				j++
			}
			name := path[i+1 : j]
			if !validParamName(name) {
				return "", nil, errx.NewCodef(CodeRouteInvalid, "webx：路由参数名非法：%q（路径 %s）", name, path)
			}
			params = append(params, name)
			b.WriteByte('{')
			b.WriteString(name)
			b.WriteByte('}')
			i = j
		case '*':
			j := i + 1
			for j < len(path) && path[j] != '/' {
				j++
			}
			name := path[i+1 : j]
			if !validParamName(name) {
				return "", nil, errx.NewCodef(CodeRouteInvalid, "webx：通配参数名非法：%q（路径 %s）", name, path)
			}
			if j < len(path) {
				return "", nil, errx.NewCodef(CodeRouteInvalid, "webx：通配参数必须是路径最后一段：%s", path)
			}
			params = append(params, name)
			b.WriteByte('{')
			b.WriteString(name)
			b.WriteString("...}")
			i = j
		default:
			b.WriteByte(path[i])
			i++
		}
	}
	return b.String(), params, nil
}

// validParamName 校验参数名：字母、数字、下划线。
func validParamName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	return true
}

// parsePattern 将规范路径拆分为段。
func parsePattern(pattern string) ([]segment, error) {
	parts := strings.Split(pattern, "/")
	segs := make([]segment, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			if strings.HasSuffix(name, "...") {
				segs = append(segs, segment{param: strings.TrimSuffix(name, "..."), wildcard: true})
				continue
			}
			segs = append(segs, segment{param: name})
			continue
		}
		segs = append(segs, segment{literal: part})
	}
	for i, s := range segs {
		if s.wildcard && i != len(segs)-1 {
			return nil, errx.NewCodef(CodeRouteInvalid, "webx：通配参数必须是路径最后一段：%s", pattern)
		}
	}
	return segs, nil
}

// nextSegment 返回从 idx 开始的下一段及下一段起始位置。
func nextSegment(path string, idx int) (string, int) {
	for idx < len(path) && path[idx] == '/' {
		idx++
	}
	if idx >= len(path) {
		return "", -1
	}
	start := idx
	for idx < len(path) && path[idx] != '/' {
		idx++
	}
	if idx >= len(path) {
		return path[start:idx], -1
	}
	return path[start:idx], idx + 1
}
