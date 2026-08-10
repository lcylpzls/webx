package webx

import (
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/lcylpzls/webx/internal/core"
)

// staticEntry 缓存单条静态文件服务配置。
type staticEntry struct {
	prefix string
	fs     http.FileSystem
	opts   StaticOptions
}

// StaticOptions 定义静态文件服务的选项。
type StaticOptions struct {
	// MaxAge 设置 Cache-Control: max-age（0 表示不设置）。
	MaxAge time.Duration
	// DisableIndex 禁用目录索引：无 index.html 的目录返回 404。
	DisableIndex bool
	// EnableETag 按文件 mtime 与大小生成弱 ETag，支持 If-None-Match 返回 304。
	EnableETag bool
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
	return s.ServeStaticFSWithOptions(prefix, filesys, StaticOptions{})
}

// ServeStaticFSWithOptions 从 http.FileSystem 提供静态文件，并应用选项。
func (s *Server) ServeStaticFSWithOptions(prefix string, filesys http.FileSystem, opts StaticOptions) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.logWarn("webx：服务已启动，不允许修改配置")
		return s
	}
	s.staticEntries = append(s.staticEntries, staticEntry{prefix: prefix, fs: filesys, opts: opts})
	return s
}

// ServeStaticDirWithOptions 从本地目录提供静态文件，并应用选项。
func (s *Server) ServeStaticDirWithOptions(prefix, root string, opts StaticOptions) *Server {
	return s.ServeStaticFSWithOptions(prefix, http.Dir(root), opts)
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

// staticOptionsFileServer 包装 http.FileServer，应用缓存头与目录索引选项。
func staticOptionsFileServer(fs http.FileSystem, opts StaticOptions) http.Handler {
	next := http.FileServer(fs)
	if opts.MaxAge <= 0 && !opts.DisableIndex && !opts.EnableETag {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if opts.MaxAge > 0 {
			w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", int64(opts.MaxAge.Seconds())))
		}
		if opts.DisableIndex {
			if f, err := fs.Open(r.URL.Path); err == nil {
				if info, statErr := f.Stat(); statErr == nil && info.IsDir() {
					f.Close()
					if idx, err := fs.Open(path.Join(r.URL.Path, "index.html")); err != nil {
						http.NotFound(w, r)
						return
					} else {
						idx.Close()
					}
				} else {
					f.Close()
				}
			}
		}
		if opts.EnableETag {
			if f, err := fs.Open(r.URL.Path); err == nil {
				if info, statErr := f.Stat(); statErr == nil && !info.IsDir() {
					defer f.Close()
					w.Header().Set("Etag", weakETag(info.ModTime(), info.Size()))
					http.ServeContent(w, r, path.Base(r.URL.Path), info.ModTime(), f)
					return
				}
				f.Close()
			}
		}
		next.ServeHTTP(w, r)
	})
}

// weakETag 按文件 mtime 与大小生成弱 ETag。
func weakETag(modTime time.Time, size int64) string {
	return fmt.Sprintf("W/\"%x-%x\"", modTime.Unix(), size)
}
