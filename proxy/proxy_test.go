package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/lcylpzls/webx"
)

func TestHandler(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Upstream", "yes")
		_, _ = w.Write([]byte("目标响应"))
	}))
	defer target.Close()
	tu, err := url.Parse(target.URL)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://webx/path?q=1", nil)
	c := webx.NewContext(rec, req)
	Handler(tu)(c)
	if rec.Code != http.StatusOK || rec.Body.String() != "目标响应" {
		t.Errorf("代理响应不符：%d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Upstream") != "yes" {
		t.Error("上游响应头未透传")
	}
}

func TestHandlerWithOptions(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer target.Close()
	tu, _ := url.Parse(target.URL)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://webx/", nil)
	c := webx.NewContext(rec, req)
	Handler(tu, func(rp *httputil.ReverseProxy) {
		rp.ModifyResponse = func(resp *http.Response) error {
			resp.Header.Set("X-Modified", "1")
			return nil
		}
	})(c)
	if rec.Header().Get("X-Modified") != "1" {
		t.Errorf("选项未生效：%v", rec.Header())
	}
}

func TestHandlerUpgrade(t *testing.T) {
	var hit atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			t.Error("上游未收到 Upgrade 头")
			return
		}
		hit.Store(true)
		hj := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		defer conn.Close()
		_, _ = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n"))
	}))
	defer target.Close()
	tu, _ := url.Parse(target.URL)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://webx/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	c := webx.NewContext(rec, req)
	Handler(tu)(c)
	if !hit.Load() {
		t.Error("Upgrade 请求未透传到上游")
	}
}
