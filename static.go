package webx

import (
	"net/http"

	"github.com/lcylpzls/webx/internal/core"
)

// staticEntry 缓存单条静态文件服务配置。
type staticEntry struct {
	prefix string
	fs     http.FileSystem
}

// spaConfig 缓存 SPA 回退配置。
type spaConfig struct {
	fs        http.FileSystem
	indexPath string
}

// ServeStaticDir 从本地目录提供静态文件。
func (s *Server) ServeStaticDir(prefix, root string) *Server {
	return s.ServeStaticFS(prefix, http.Dir(root))
}

// ServeStaticFS 从 http.FileSystem 提供静态文件，配合 embed 使用。
func (s *Server) ServeStaticFS(prefix string, filesys http.FileSystem) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.logWarn("webx：服务已启动，不允许修改配置")
		return s
	}
	s.staticEntries = append(s.staticEntries, staticEntry{prefix: prefix, fs: filesys})
	return s
}

// EnableSPA 启用 SPA 回退：未匹配路由的 GET/HEAD 请求先尝试文件，再回退 index。
func (s *Server) EnableSPA(filesys http.FileSystem, indexPath string) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.logWarn("webx：服务已启动，不允许修改配置")
		return s
	}
	s.spa = &spaConfig{fs: filesys, indexPath: indexPath}
	return s
}

// spaNoRoute 返回 SPA 模式的 NoRoute 处理器。
func spaNoRoute(filesys http.FileSystem, indexPath string) core.HandlerFunc {
	return func(c *core.Context) {
		if c.Request().Method != http.MethodGet && c.Request().Method != http.MethodHead {
			c.JSONResponse(http.StatusNotFound, "请求的资源不存在", nil)
			return
		}
		reqPath := c.Request().URL.Path
		if f, err := filesys.Open(reqPath); err == nil {
			stat, statErr := f.Stat()
			f.Close()
			if statErr == nil && !stat.IsDir() {
				if f2, err2 := filesys.Open(reqPath); err2 == nil {
					defer f2.Close()
					stat2, _ := f2.Stat()
					http.ServeContent(c.Writer(), c.Request(), reqPath, stat2.ModTime(), f2)
					return
				}
			}
		}
		file, err := filesys.Open(indexPath)
		if err != nil {
			c.JSONResponse(http.StatusNotFound, "请求的资源不存在", nil)
			return
		}
		defer file.Close()
		stat, err := file.Stat()
		if err != nil || stat.IsDir() {
			c.JSONResponse(http.StatusNotFound, "请求的资源不存在", nil)
			return
		}
		http.ServeContent(c.Writer(), c.Request(), indexPath, stat.ModTime(), file)
	}
}
