package middleware

import (
	"bytes"
	"compress/gzip"
	testx "github.com/lcylpzls/testx"
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
	testx.RequireEqual(t, rec.Code, http.StatusOK)

	if rec.Header().Get("Content-Encoding") != "gzip" || rec.Header().Get("Vary") != "Accept-Encoding" {
		t.Errorf("响应头不符：%v", rec.Header())
	}
	zr, err := gzip.NewReader(rec.Body)
	testx.RequireNoError(t, err)

	got, _ := io.ReadAll(zr)
	zr.Close()
	if string(got) != "压缩内容" {
		t.Errorf("解压内容不符：%s", got)
	}
}

func TestGzipCustomLevel(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{
		GzipWithOptions(GzipOptions{Level: 9}),
		func(c *core.Context) { _ = c.String(http.StatusOK, "自定义级别压缩内容") },
	})
	c.Run()
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("自定义级别未启用 gzip：%v", rec.Header())
	}
	zr, err := gzip.NewReader(rec.Body)
	testx.RequireNoError(t, err)

	got, _ := io.ReadAll(zr)
	zr.Close()
	if string(got) != "自定义级别压缩内容" {
		t.Errorf("解压内容不符：%s", got)
	}
}

func TestGzipInvalidLevelNormalized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{
		GzipWithOptions(GzipOptions{Level: 99}),
		func(c *core.Context) { _ = c.String(http.StatusOK, "非法级别回退默认") },
	})
	c.Run()
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("非法级别应回退默认并压缩：%v", rec.Header())
	}
	zr, err := gzip.NewReader(rec.Body)
	testx.RequireNoError(t, err)

	got, _ := io.ReadAll(zr)
	zr.Close()
	if string(got) != "非法级别回退默认" {
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

func TestSetHeaderReuse(t *testing.T) {
	h := make(http.Header)
	key := core.CanonicalHeaderKey("X-Test")
	setHeader(h, key, "1")
	setHeader(h, key, "2") // 复用已有切片
	if got := h.Get("X-Test"); got != "2" {
		t.Errorf("setHeader 复用不符：%s", got)
	}
	if len(h[key]) != 1 {
		t.Errorf("setHeader 不应追加切片：%d", len(h[key]))
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
	testx.RequireNoError(t, err)

	gz.Close()
	testx.Equal(t, rec.Code, http.StatusCreated)

	if w.Unwrap() != rec {
		t.Error("Unwrap 不符")
	}
}

func TestGzipSkipsBinaryContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{
		Gzip(),
		func(c *core.Context) {
			c.Header("Content-Type", "image/png")
			_, _ = c.Writer().Write([]byte("PNG"))
		},
	})
	c.Run()
	if rec.Header().Get("Content-Encoding") != "" {
		t.Errorf("二进制内容不应压缩：%v", rec.Header())
	}
	if rec.Body.String() != "PNG" {
		t.Errorf("响应体应为明文：%s", rec.Body.String())
	}

	// 显式写状态码的二进制响应同样不压缩
	rec = httptest.NewRecorder()
	c = core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Request().Header.Set("Accept-Encoding", "gzip")
	c.SetHandlers([]core.HandlerFunc{
		Gzip(),
		func(c *core.Context) {
			c.Header("Content-Type", "image/png")
			c.Status(http.StatusOK)
			_, _ = c.Writer().Write([]byte("PNG"))
		},
	})
	c.Run()
	if rec.Header().Get("Content-Encoding") != "" || rec.Body.String() != "PNG" {
		t.Errorf("显式状态码二进制响应不应压缩：%v %s", rec.Header(), rec.Body.String())
	}
}

func TestGzipMinSize(t *testing.T) {
	run := func(size int) (*httptest.ResponseRecorder, error) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		c := core.NewContext(rec, req)
		data := bytes.Repeat([]byte("x"), size)
		c.SetHandlers([]core.HandlerFunc{
			Gzip(),
			func(c *core.Context) { _, _ = c.Writer().Write(data) },
		})
		c.Run()
		return rec, nil
	}
	// 小响应（隐式写）：明文
	rec, _ := run(10)
	if rec.Header().Get("Content-Encoding") != "" || rec.Body.Len() != 10 {
		t.Errorf("小响应不应压缩：%v %d", rec.Header(), rec.Body.Len())
	}
	// 大响应：压缩
	rec, _ = run(2048)
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("大响应应压缩：%v", rec.Header())
	}
	zr, err := gzip.NewReader(rec.Body)
	testx.RequireNoError(t, err)

	got, _ := io.ReadAll(zr)
	zr.Close()
	if len(got) != 2048 {
		t.Errorf("解压长度不符：%d", len(got))
	}
}

func TestGzipFlushStarts(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{
		Gzip(),
		func(c *core.Context) {
			_, _ = c.Writer().Write([]byte("ab"))
			if gw, ok := c.Writer().(*gzipWriter); ok {
				gw.Flush()
			} else {
				t.Error("Writer 应为 gzipWriter")
			}
			_, _ = c.Writer().Write([]byte("cd"))
		},
	})
	c.Run()
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("Flush 应启动压缩：%v", rec.Header())
	}
	zr, err := gzip.NewReader(rec.Body)
	testx.RequireNoError(t, err)

	got, _ := io.ReadAll(zr)
	zr.Close()
	if string(got) != "abcd" {
		t.Errorf("解压内容不符：%s", got)
	}
}
