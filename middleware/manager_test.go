package middleware

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/lcylpzls/testx"
)

func TestManagerDefaultState(t *testing.T) {
	m := NewManager()
	chain := m.Build(context.Background())
	if len(chain) != 0 {
		t.Errorf("默认应无中间件（rate_limit/access_log 禁用）：%d", len(chain))
	}
}

func TestManagerRegisterOverrideDisableEnable(t *testing.T) {
	m := NewManager()
	a := pass()
	b := pass()
	override := pass()

	m.RegisterBuiltin("recovery", a)
	m.RegisterBuiltin("request_id", b)
	testx.RequireEqual(t, len(m.Build(context.Background())), 2)

	m.Disable("recovery")
	testx.RequireEqual(t, len(m.Build(context.Background())), 1)
	m.Enable("recovery")
	testx.RequireEqual(t, len(m.Build(context.Background())), 2)

	m.Override("recovery", override)
	chain := m.Build(context.Background())
	if len(chain) != 2 {
		t.Fatalf("覆盖后链长度不符：%d", len(chain))
	}
	if !sameFunc(chain[0], override) {
		t.Error("覆盖未生效")
	}
	m.Override("missing", override)
	testx.RequireEqual(t, len(m.Build(context.Background())), 2)
}

func TestManagerRateLimit(t *testing.T) {
	m := NewManager()
	rl := NewRateLimiter(10, time.Second, nil)
	m.EnableRateLimit(RateLimit(rl))
	chain := m.Build(context.Background())
	if len(chain) != 1 {
		t.Fatalf("EnableRateLimit 后链长度不符：%d", len(chain))
	}
	// Enable 对 rate_limit 无效
	m.DisableRateLimit()
	m.Enable("rate_limit")
	testx.RequireEqual(t, len(m.Build(context.Background())), 0)
	m.EnableRateLimit(RateLimit(rl))
	testx.RequireEqual(t, len(m.Build(context.Background())), 1)
	m.DisableRateLimit()
	testx.RequireEqual(t, len(m.Build(context.Background())), 0)
}

func TestManagerAppendAndOrder(t *testing.T) {
	m := NewManager()
	recovery := pass()
	requestID := pass()
	extra := pass()
	m.RegisterBuiltin("recovery", recovery)
	m.RegisterBuiltin("request_id", requestID)
	m.Append(extra)
	chain := m.Build(context.Background())
	if len(chain) != 3 {
		t.Fatalf("链长度不符：%d", len(chain))
	}
	if !sameFunc(chain[0], recovery) || !sameFunc(chain[1], requestID) || !sameFunc(chain[2], extra) {
		t.Error("链顺序不符：应为 recovery → request_id → 外部中间件")
	}
}

func TestManagerSetOrder(t *testing.T) {
	m := NewManager()
	m.RegisterBuiltin("recovery", pass())
	m.RegisterBuiltin("request_id", pass())
	m.SetOrder("request_id", "recovery", "unknown")
	chain := m.Build(context.Background())
	if len(chain) != 2 {
		t.Fatalf("链长度不符：%d", len(chain))
	}
	if !sameFunc(chain[0], m.builtins["request_id"]) || !sameFunc(chain[1], m.builtins["recovery"]) {
		t.Error("自定义顺序未生效")
	}
	m.SetOrder()
	testx.RequireEqual(t, len(m.Build(context.Background())), 2)
}

// sameFunc 判断两个函数值是否指向同一函数。
func sameFunc(a, b Middleware) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

// pass 构造一个直通的标准中间件（测试辅助）。
func pass() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}
