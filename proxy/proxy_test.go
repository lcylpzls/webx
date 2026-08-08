package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

func TestHandlerErrorResponse(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	tu, _ := url.Parse(target.URL)
	target.Close() // 上游已关闭 → 代理错误

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://webx/", nil)
	c := webx.NewContext(rec, req)
	Handler(tu)(c)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("上游不可用应 502：%d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应体不是 JSON：%v", err)
	}
	if body["code"] != float64(502) || !strings.Contains(body["msg"].(string), "上游") {
		t.Errorf("默认错误体不符：%s", rec.Body.String())
	}

	// 自定义错误处理器
	rec = httptest.NewRecorder()
	c = webx.NewContext(rec, httptest.NewRequest(http.MethodGet, "http://webx/", nil))
	Handler(tu, WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("custom"))
	}))(c)
	if rec.Code != http.StatusServiceUnavailable || rec.Body.String() != "custom" {
		t.Errorf("自定义错误处理器不符：%d %s", rec.Code, rec.Body.String())
	}
}

func TestNewProxyDefaults(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer target.Close()
	tu, _ := url.Parse(target.URL)
	rp := newProxy(tu)
	if rp.ErrorHandler == nil {
		t.Error("默认错误处理器未设置")
	}
	if rp.FlushInterval != 0 {
		t.Errorf("默认 FlushInterval 应为 0：%v", rp.FlushInterval)
	}
	if rp.Transport != nil {
		t.Error("默认 Transport 应为 nil")
	}
}

func TestWithFlushInterval(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer target.Close()
	tu, _ := url.Parse(target.URL)
	rp := newProxy(tu, WithFlushInterval(-1))
	if rp.FlushInterval != -1 {
		t.Errorf("FlushInterval 未生效：%v", rp.FlushInterval)
	}
}

func TestWithTimeout(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer target.Close()
	tu, _ := url.Parse(target.URL)

	rp := newProxy(tu, WithTimeout(50*time.Millisecond))
	if rp.Transport == nil {
		t.Fatal("WithTimeout 应包装 Transport")
	}
	start := time.Now()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://webx/", nil)
	rp.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("超时应输出 502：%d", rec.Code)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("超时未生效：%v", elapsed)
	}

	// 叠加选项：第二次包装时 base 已存在
	rp2 := newProxy(tu, WithTimeout(10*time.Millisecond), WithTimeout(20*time.Millisecond))
	if rp2.Transport == nil {
		t.Fatal("叠加 WithTimeout 应保留包装")
	}
	if _, err := rp2.Transport.RoundTrip(req.WithContext(context.Background())); err == nil {
		t.Error("超时 RoundTrip 应返回错误")
	}
}

func TestWithTimeoutDisabled(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer target.Close()
	tu, _ := url.Parse(target.URL)
	rp := newProxy(tu, WithTimeout(0))
	if rp.Transport != nil {
		t.Error("超时 <=0 不应包装 Transport")
	}
}
