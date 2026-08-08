package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lcylpzls/webx/internal/core"
)

func TestGzipCompresses(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{
		Gzip(),
		func(c *core.Context) { _ = c.String(http.StatusOK, "压缩内容") },
	})
	c.Run()
	if rec.Code != http.StatusOK {
		t.Fatalf("状态不符：%d", rec.Code)
	}
	if rec.Header().Get("Content-Encoding") != "gzip" || rec.Header().Get("Vary") != "Accept-Encoding" {
		t.Errorf("响应头不符：%v", rec.Header())
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("响应体不是合法 gzip：%v", err)
	}
	got, _ := io.ReadAll(zr)
	zr.Close()
	if string(got) != "压缩内容" {
		t.Errorf("解压内容不符：%s", got)
	}
}

func TestGzipNoAcceptEncoding(t *testing.T) {
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		Gzip(),
		func(c *core.Context) { _ = c.String(http.StatusOK, "明文") },
	})
	c.Run()
	if rec.Body.String() != "明文" || rec.Header().Get("Content-Encoding") != "" {
		t.Errorf("未协商 gzip 时应明文输出：%s %v", rec.Body.String(), rec.Header())
	}
}

func TestAcceptsGzip(t *testing.T) {
	cases := map[string]bool{
		"gzip":     true,
		"br, gzip": true,
		"gzip;q=1": false,
		"br":       false,
		"":         false,
	}
	for in, want := range cases {
		if got := acceptsGzip(in); got != want {
			t.Errorf("acceptsGzip(%q) 不符：got %v, want %v", in, got, want)
		}
	}
}

func TestGzipWriterMethods(t *testing.T) {
	rec := httptest.NewRecorder()
	gz := gzip.NewWriter(rec)
	w := &gzipWriter{ResponseWriter: rec, gz: gz}
	w.WriteHeader(http.StatusCreated)
	w.WriteHeader(http.StatusOK) // 二次写入无效
	w.Flush()
	_, err := w.Write([]byte("x"))
	if err != nil {
		t.Fatalf("Write 失败：%v", err)
	}
	gz.Close()
	if rec.Code != http.StatusCreated {
		t.Errorf("状态码不符：%d", rec.Code)
	}
	if w.Unwrap() != rec {
		t.Error("Unwrap 不符")
	}
}
