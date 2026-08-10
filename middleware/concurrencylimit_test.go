package middleware

import (
	testx "github.com/lcylpzls/testx"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/lcylpzls/webx/internal/core"
)

func TestConcurrencyLimiter(t *testing.T) {
	l := NewConcurrencyLimiter(2)
	if !l.TryAcquire() {
		t.Fatal("第一个额度应获取成功")
	}
	if !l.TryAcquire() {
		t.Fatal("第二个额度应获取成功")
	}
	if l.TryAcquire() {
		t.Error("额度已满应拒绝")
	}
	if got := l.Rejected(); got != 1 {
		t.Errorf("拒绝计数不符：%d", got)
	}
	if got := l.Active(); got != 2 {
		t.Errorf("Active 不符：%d", got)
	}
	l.Release()
	if !l.TryAcquire() {
		t.Error("释放后应可重新获取")
	}
	if got := l.Active(); got != 2 {
		t.Errorf("释放后 Active 不符：%d", got)
	}

	unlimited := NewConcurrencyLimiter(0)
	if !unlimited.TryAcquire() {
		t.Error("不限制时应总是可获取")
	}
	if got := unlimited.Rejected(); got != 0 {
		t.Errorf("不限制时拒绝计数应为 0：%d", got)
	}
}

func TestConcurrencyLimitMiddleware(t *testing.T) {
	l := NewConcurrencyLimiter(1)
	entered := make(chan struct{})
	release := make(chan struct{})
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		ConcurrencyLimit(l),
		func(c *core.Context) {
			close(entered)
			<-release
			c.Success("ok", nil)
		},
	})
	done := make(chan struct{})
	go func() {
		c.Run()
		close(done)
	}()
	<-entered

	rec2 := httptest.NewRecorder()
	c2 := core.NewContext(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	c2.SetHandlers([]core.HandlerFunc{
		ConcurrencyLimit(l),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c2.Run()
	testx.Equal(t, rec2.Code, http.StatusServiceUnavailable)

	if got := rec2.Header().Get("Retry-After"); got != "1" {
		t.Errorf("应携带 Retry-After：%s", got)
	}
	if got := l.Active(); got != 1 {
		t.Errorf("拒绝请求不应占用额度：%d", got)
	}

	close(release)
	<-done
	if got := l.Active(); got != 0 {
		t.Errorf("处理完成后额度应释放：%d", got)
	}
	testx.Equal(t, rec.Code, http.StatusOK)

}

func TestConcurrencyLimitCustomMessage(t *testing.T) {
	l := NewConcurrencyLimiter(0)
	l.SetRejectMessage("自定义繁忙文案")
	if got := l.rejectMsg.Load(); got == nil || *got != "自定义繁忙文案" {
		t.Error("拒绝文案未保存")
	}

	l2 := NewConcurrencyLimiter(1)
	l2.SetRejectMessage("自定义繁忙文案")
	entered := make(chan struct{})
	release := make(chan struct{})
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		ConcurrencyLimit(l2),
		func(c *core.Context) {
			close(entered)
			<-release
			c.Success("ok", nil)
		},
	})
	done := make(chan struct{})
	go func() {
		c.Run()
		close(done)
	}()
	<-entered
	rec2 := httptest.NewRecorder()
	c2 := core.NewContext(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	c2.SetHandlers([]core.HandlerFunc{
		ConcurrencyLimit(l2),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c2.Run()
	close(release)
	<-done
	if rec2.Code != http.StatusServiceUnavailable || !strings.Contains(rec2.Body.String(), "自定义繁忙文案") {
		t.Errorf("自定义文案未生效：%d %s", rec2.Code, rec2.Body.String())
	}
}

func TestConcurrencyLimitPanicRelease(t *testing.T) {
	l := NewConcurrencyLimiter(1)
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetHandlers([]core.HandlerFunc{
		ConcurrencyLimit(l),
		func(c *core.Context) { panic("测试 panic") },
	})
	func() {
		defer func() { recover() }()
		c.Run()
	}()
	if got := l.Active(); got != 0 {
		t.Errorf("panic 后额度应释放：%d", got)
	}
}

func TestConcurrencyLimitConcurrent(t *testing.T) {
	l := NewConcurrencyLimiter(4)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if l.TryAcquire() {
					if l.Active() > 4 {
						t.Errorf("并发数超过上限：%d", l.Active())
					}
					l.Release()
				}
			}
		}()
	}
	wg.Wait()
	if got := l.Active(); got != 0 {
		t.Errorf("并发测试后额度应归零：%d", got)
	}
}
