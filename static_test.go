package webx

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lcylpzls/webx/internal/core"
)

func TestSPANoRouteServeFileAndFallback(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "app.js"), []byte("js"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "index.html"), []byte("index"), 0o600)
	h := spaNoRoute(http.Dir(dir), "index.html")

	// 文件存在
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/app.js", nil))
	c.SetHandlers([]core.HandlerFunc{h})
	c.Run()
	if rec.Code != http.StatusOK || rec.Body.String() != "js" {
		t.Errorf("文件服务不符：%d %s", rec.Code, rec.Body.String())
	}

	// 文件不存在 → 回退 index
	rec = httptest.NewRecorder()
	c = core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/route/x", nil))
	c.SetHandlers([]core.HandlerFunc{h})
	c.Run()
	if rec.Code != http.StatusOK || rec.Body.String() != "index" {
		t.Errorf("SPA 回退不符：%d %s", rec.Code, rec.Body.String())
	}

	// 非 GET/HEAD → 404
	rec = httptest.NewRecorder()
	c = core.NewContext(rec, httptest.NewRequest(http.MethodPost, "/route/x", nil))
	c.SetHandlers([]core.HandlerFunc{h})
	c.Run()
	if rec.Code != http.StatusNotFound {
		t.Errorf("非 GET 应 404：%d", rec.Code)
	}
}

func TestSPANoRouteIndexMissing(t *testing.T) {
	dir := t.TempDir()
	h := spaNoRoute(http.Dir(dir), "index.html")
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	c.SetHandlers([]core.HandlerFunc{h})
	c.Run()
	if rec.Code != http.StatusNotFound {
		t.Errorf("index 缺失应 404：%d", rec.Code)
	}
}

func TestSPANoRouteStatError(t *testing.T) {
	h := spaNoRoute(errStatFS{}, "index.html")
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	c.SetHandlers([]core.HandlerFunc{h})
	c.Run()
	if rec.Code != http.StatusNotFound {
		t.Errorf("Stat 失败应 404：%d", rec.Code)
	}
}

// errStatFS 返回 Stat 失败的假文件系统。
type errStatFS struct{}

func (errStatFS) Open(name string) (http.File, error) {
	return errStatFile{}, nil
}

type errStatFile struct{}

func (errStatFile) Read(p []byte) (int, error) { return 0, io.EOF }
func (errStatFile) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}
func (errStatFile) Close() error { return nil }
func (errStatFile) Stat() (os.FileInfo, error) {
	return nil, errors.New("stat 失败")
}
func (errStatFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, nil
}

func TestSPANoRouteRequestPathIsDir(t *testing.T) {
	dir := t.TempDir()
	_ = os.Mkdir(filepath.Join(dir, "sub"), 0o700)
	_ = os.WriteFile(filepath.Join(dir, "index.html"), []byte("index"), 0o600)
	h := spaNoRoute(http.Dir(dir), "index.html")
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/sub", nil))
	c.SetHandlers([]core.HandlerFunc{h})
	c.Run()
	if rec.Code != http.StatusOK || rec.Body.String() != "index" {
		t.Errorf("目录请求应回退 index：%d %s", rec.Code, rec.Body.String())
	}
	// index 为目录 → 404
	dir2 := t.TempDir()
	_ = os.Mkdir(filepath.Join(dir2, "index.html"), 0o700)
	h2 := spaNoRoute(http.Dir(dir2), "index.html")
	rec = httptest.NewRecorder()
	c = core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	c.SetHandlers([]core.HandlerFunc{h2})
	c.Run()
	if rec.Code != http.StatusNotFound {
		t.Errorf("index 为目录应 404：%d", rec.Code)
	}
}

func TestServeStaticDirAndFS(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "f.txt"), []byte("f"), 0o600)
	s := newTestServer(t, validConfig(t))
	if got := s.ServeStaticDir("/d", dir); got != s {
		t.Error("ServeStaticDir 应返回自身")
	}
	if got := s.ServeStaticFS("/fs", http.Dir(dir)); got != s {
		t.Error("ServeStaticFS 应返回自身")
	}
	if len(s.staticEntries) != 2 {
		t.Errorf("静态条目数量不符：%d", len(s.staticEntries))
	}
	if got := s.ServeStaticDirWithOptions("/d2", dir, StaticOptions{MaxAge: 60 * time.Second, DisableIndex: true}); got != s {
		t.Error("ServeStaticDirWithOptions 应返回自身")
	}
	if got := s.ServeStaticFSWithOptions("/fs2", http.Dir(dir), StaticOptions{MaxAge: 60 * time.Second}); got != s {
		t.Error("ServeStaticFSWithOptions 应返回自身")
	}
	if len(s.staticEntries) != 4 {
		t.Errorf("带选项静态条目数量不符：%d", len(s.staticEntries))
	}
	if s.staticEntries[2].opts.MaxAge != 60*time.Second || !s.staticEntries[2].opts.DisableIndex {
		t.Errorf("选项未保存：%+v", s.staticEntries[2].opts)
	}
}
