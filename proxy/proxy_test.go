package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
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
