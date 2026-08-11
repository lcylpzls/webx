package middleware

import (
	"context"
	testx "github.com/lcylpzls/testx"
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
		stdToCore(Timeout(10 * time.Millisecond)),
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
	testx.Equal(t, rec.Code, http.StatusServiceUnavailable)

	if !contains(rec.Body.String(), "请求处理超时") {
		t.Errorf("503 响应体不符：%s", rec.Body.String())
	}
}

func TestTimeoutCustomMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		stdToCore(TimeoutWithOptions(10*time.Millisecond, TimeoutOptions{Message: "自定义超时文案"})),
		func(c *core.Context) {
			<-c.Request().Context().Done()
			_ = c.String(http.StatusOK, "迟到响应")
		},
	})
	c.Run()
	if rec.Code != http.StatusServiceUnavailable || !contains(rec.Body.String(), "自定义超时文案") {
		t.Errorf("自定义文案未生效：%d %s", rec.Code, rec.Body.String())
	}
}

func TestTimeoutWroteHeaderBeforeExpiry(t *testing.T) {
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		stdToCore(Timeout(5 * time.Millisecond)),
		func(c *core.Context) {
			c.Status(http.StatusOK)
			<-c.Request().Context().Done()
			_, _ = c.Writer().Write([]byte("late"))
		},
	})
	c.Run()
	testx.Equal(t, rec.Code, http.StatusOK)

}

func TestTimeoutWritesBeforeExpiry(t *testing.T) {
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		stdToCore(Timeout(time.Second)),
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
		stdToCore(Timeout(0)),
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
	testx.RequireNoError(t, rc.Flush())
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
