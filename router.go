package webx

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/lcylpzls/webx/internal/core"
)

// Router 基于标准库 http.ServeMux 实现路由：
// 负责 gin 风格语法（:id / *filepath）到 ServeMux 模式（{id} / {path...}）的翻译，
// 以及 404/405 标准化 JSON 响应。路径匹配由内置轻量匹配器完成，
// 实际分发交给 ServeMux（保留其冲突检测与 PathValue 能力）。
type Router struct {
	mux      *http.ServeMux
	patterns []*routePattern
	noRoute  core.HandlerFunc
	noMethod core.HandlerFunc
	maxBody  int64
}

// routePattern 是一条已翻译的 ServeMux 模式及其注册信息。
type routePattern struct {
	pattern  string
	segments []segment
	params   []string
	methods  map[string]bool
	subtree  bool
}

// segment 是路径模式中的一段。
type segment struct {
	literal  string
	param    string
	wildcard bool
}

// NewRouter 创建路由，并指定 404/405 兜底处理器。
func NewRouter(noRoute, noMethod core.HandlerFunc) *Router {
	return &Router{
		mux:      http.NewServeMux(),
		noRoute:  noRoute,
		noMethod: noMethod,
	}
}

// SetMaxBodyBytes 设置路由处理链中 BindJSON 的最大请求体字节数。
func (rt *Router) SetMaxBodyBytes(n int64) {
	rt.maxBody = n
}

// Handle 注册一条路由（chain 为全局中间件 + 路由中间件 + 最终处理器的完整链）。
// 路径冲突或语法非法时返回错误。
func (rt *Router) Handle(method, path string, chain []core.HandlerFunc) error {
	translated, params, err := translateGinPattern(path)
	if err != nil {
		return err
	}
	if method == "" {
		return fmt.Errorf("webx：路由方法不能为空：%s", path)
	}
	// translateGinPattern 已保证模式合法，这里不再重复校验。
	segs, _ := parsePattern(translated)
	p := &routePattern{
		pattern:  translated,
		segments: segs,
		params:   params,
		methods:  map[string]bool{method: true},
		subtree:  strings.HasSuffix(translated, "/"),
	}
	if err := rt.safeHandle(method+" "+translated, http.HandlerFunc(rt.wrap(chain, params))); err != nil {
		return err
	}
	rt.patterns = append(rt.patterns, p)
	return nil
}

// HandleStatic 注册静态文件服务（支持子树路径）。
// 使用无方法模式注册，避免 ServeMux 中 GET 隐式匹配 HEAD 导致
// "静态根 + 具体 GET 路由" 的冲突；方法判定由匹配器负责。
func (rt *Router) HandleStatic(prefix string, fs http.FileSystem) error {
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
	handler := http.StripPrefix(strip, http.FileServer(fs))
	p := &routePattern{
		pattern:  pattern,
		segments: segs,
		methods:  map[string]bool{"GET": true, "HEAD": true},
		subtree:  true,
	}
	if err := rt.safeHandle(pattern, handler); err != nil {
		return err
	}
	rt.patterns = append(rt.patterns, p)
	return nil
}

// safeHandle 包装 mux.Handle，将冲突/非法模式 panic 转为错误，遵守"绝不 panic"铁律。
func (rt *Router) safeHandle(pattern string, handler http.Handler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("webx：路由注册失败：%v", r)
		}
	}()
	rt.mux.Handle(pattern, handler)
	return nil
}

// wrap 将 webx 处理器链包装为标准 http.HandlerFunc。
func (rt *Router) wrap(chain []core.HandlerFunc, params []string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		c := core.NewContext(w, r)
		if len(params) > 0 {
			values := make(map[string]string, len(params))
			for _, name := range params {
				values[name] = r.PathValue(name)
			}
			c.SetParams(values)
		}
		if rt.maxBody > 0 {
			c.SetMaxBodyBytes(rt.maxBody)
		}
		c.SetHandlers(chain)
		c.Run()
	}
}

// ServeHTTP 实现 http.Handler：先做 404/405 判定，再交给 ServeMux 分发。
func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	matches := rt.match(r.URL.Path)
	if len(matches) == 0 {
		// 兼容 ServeMux 的尾斜杠重定向：/path → /path/
		if !strings.HasSuffix(r.URL.Path, "/") {
			if len(rt.match(r.URL.Path+"/")) > 0 {
				w.Header().Set("Location", r.URL.Path+"/")
				w.WriteHeader(http.StatusMovedPermanently)
				return
			}
		}
		rt.runFallback(w, r, rt.noRoute)
		return
	}
	best := mostSpecific(matches)
	if !methodAllowedAny(best, r.Method) {
		w.Header().Set("Allow", allowHeader(best))
		rt.runFallback(w, r, rt.noMethod)
		return
	}
	rt.mux.ServeHTTP(w, r)
}

// runFallback 执行 404/405 兜底处理器。
func (rt *Router) runFallback(w http.ResponseWriter, r *http.Request, h core.HandlerFunc) {
	c := core.NewContext(w, r)
	c.SetHandlers([]core.HandlerFunc{h})
	c.Run()
}

// match 返回与路径匹配的全部已注册模式。
func (rt *Router) match(path string) []*routePattern {
	var out []*routePattern
	for _, p := range rt.patterns {
		// 子树模式要求请求以 "/" 结尾或路径更长，避免与 ServeMux 的
		// 尾斜杠重定向行为冲突。
		if p.subtree && len(splitPath(path)) == len(p.segments) && !strings.HasSuffix(path, "/") {
			continue
		}
		if matchSegments(p.segments, splitPath(path)) {
			out = append(out, p)
		}
	}
	return out
}

// methodAllowedAny 判断方法是否被同优先级模式组中的任一模式允许；
// GET 路由同时响应 HEAD。
func methodAllowedAny(patterns []*routePattern, method string) bool {
	for _, p := range patterns {
		if p.methods[method] {
			return true
		}
		if method == http.MethodHead && p.methods[http.MethodGet] {
			return true
		}
	}
	return false
}

// mostSpecific 返回最具体的匹配模式组，与 ServeMux 的选择保持一致：
// 字面段 > 参数段 > 通配/子树，段数越多越具体。
// 同路径不同方法注册会得到相同分数，作为一组参与方法判定。
func mostSpecific(matches []*routePattern) []*routePattern {
	bestScore := patternScore(matches[0])
	best := []*routePattern{matches[0]}
	for _, p := range matches[1:] {
		score := patternScore(p)
		switch {
		case score > bestScore:
			bestScore = score
			best = []*routePattern{p}
		case score == bestScore:
			best = append(best, p)
		}
	}
	return best
}

// patternScore 计算模式优先级分数（越大越具体）。
func patternScore(p *routePattern) int {
	score := 0
	for _, s := range p.segments {
		switch {
		case s.wildcard:
			score += 1
		case s.param != "":
			score += 2
		default:
			score += 4
		}
	}
	return score
}

// allowHeader 汇总匹配模式的全部方法，生成 Allow 响应头。
func allowHeader(matches []*routePattern) string {
	set := make(map[string]bool)
	for _, p := range matches {
		for m := range p.methods {
			set[m] = true
		}
	}
	// HTTP 语义：GET 路由同时响应 HEAD，Allow 一并列出。
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

// translateGinPattern 将 gin 风格路径翻译为 ServeMux 模式：
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
				return "", nil, fmt.Errorf("webx：路由参数名非法：%q（路径 %s）", name, path)
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
				return "", nil, fmt.Errorf("webx：通配参数名非法：%q（路径 %s）", name, path)
			}
			if j < len(path) {
				return "", nil, fmt.Errorf("webx：通配参数必须是路径最后一段：%s", path)
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

// parsePattern 将已翻译的 ServeMux 模式拆分为段。
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
	// 校验：通配段必须是最后一段
	for i, s := range segs {
		if s.wildcard && i != len(segs)-1 {
			return nil, fmt.Errorf("webx：通配参数必须是路径最后一段：%s", pattern)
		}
	}
	return segs, nil
}

// splitPath 将请求路径拆分为段（保留尾斜杠语义）。
func splitPath(path string) []string {
	parts := strings.Split(path, "/")
	segs := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		segs = append(segs, part)
	}
	return segs
}

// matchSegments 判断模式段与路径段是否匹配。
// 支持：字面段、参数段（单段）、通配段（匹配剩余全部）、子树模式（前缀匹配）。
func matchSegments(pattern []segment, path []string) bool {
	if len(pattern) == 0 {
		return true
	}
	// 通配段：匹配剩余全部路径
	if pattern[len(pattern)-1].wildcard {
		if len(path) < len(pattern)-1 {
			return false
		}
		for i := 0; i < len(pattern)-1; i++ {
			if !matchSegment(pattern[i], path[i]) {
				return false
			}
		}
		return true
	}
	// 子树模式：路径段数多于模式段数，且前缀匹配
	if len(path) > len(pattern) {
		for i := 0; i < len(pattern); i++ {
			if !matchSegment(pattern[i], path[i]) {
				return false
			}
		}
		return true
	}
	if len(path) != len(pattern) {
		return false
	}
	for i := 0; i < len(pattern); i++ {
		if !matchSegment(pattern[i], path[i]) {
			return false
		}
	}
	return true
}

// matchSegment 匹配单个段。
func matchSegment(p segment, s string) bool {
	if p.param != "" {
		return s != ""
	}
	return p.literal == s
}
