package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lcylpzls/webx/internal/core"
)

func TestTimeoutTriggers(t *testing.T) {
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		Timeout(10 * time.Millisecond),
		func(c *core.Context) {
			<-c.Request().Context().Done()
			_ = c.String(http.StatusOK, "迟到响应")
		},
	})
	done := make(chan struct{})
	go func() {
		c.Run()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("处理超时")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("超时应返回 503：%d", rec.Code)
	}
	if !contains(rec.Body.String(), "请求处理超时") {
		t.Errorf("503 响应体不符：%s", rec.Body.String())
	}
}

func TestTimeoutWroteHeaderBeforeExpiry(t *testing.T) {
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		Timeout(5 * time.Millisecond),
		func(c *core.Context) {
			c.Status(http.StatusOK)
			<-c.Request().Context().Done()
			_, _ = c.Writer().Write([]byte("late"))
		},
	})
	c.Run()
	if rec.Code != http.StatusOK {
		t.Errorf("已写状态码后不应返回 503：%d", rec.Code)
	}
}

func TestTimeoutWritesBeforeExpiry(t *testing.T) {
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		Timeout(time.Second),
		func(c *core.Context) {
			_ = c.String(http.StatusOK, "正常响应")
		},
	})
	c.Run()
	if rec.Code != http.StatusOK || rec.Body.String() != "正常响应" {
		t.Errorf("未超时写入不符：%d %s", rec.Code, rec.Body.String())
	}
}

func TestTimeoutDisabled(t *testing.T) {
	rec := httptest.NewRecorder()
	var called bool
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		Timeout(0),
		func(c *core.Context) { called = true; c.Success("ok", nil) },
	})
	c.Run()
	if !called || rec.Code != http.StatusOK {
		t.Errorf("timeout<=0 应直接放行：%v %d", called, rec.Code)
	}
}

func TestTimeoutWriterUnwrap(t *testing.T) {
	rec := httptest.NewRecorder()
	tw := &timeoutWriter{ResponseWriter: rec, ctx: context.Background()}
	rc := http.NewResponseController(tw)
	if err := rc.Flush(); err != nil {
		t.Errorf("经 Unwrap 的 Flush 应可用：%v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
