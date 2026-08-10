package middleware

import (
	"context"
	"reflect"
	"testing"

	"github.com/lcylpzls/testx"
	"github.com/lcylpzls/webx/internal/core"
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
	a := func(c *core.Context) { c.Next() }
	b := func(c *core.Context) { c.Next() }
	override := func(c *core.Context) { c.Next() }

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
	rl := func(c *core.Context) { c.Next() }
	m.EnableRateLimit(rl)
	chain := m.Build(context.Background())
	if len(chain) != 1 {
		t.Fatalf("EnableRateLimit 后链长度不符：%d", len(chain))
	}
	// Enable 对 rate_limit 无效
	m.DisableRateLimit()
	m.Enable("rate_limit")
	testx.RequireEqual(t, len(m.Build(context.Background())), 0)
	m.EnableRateLimit(rl)
	testx.RequireEqual(t, len(m.Build(context.Background())), 1)
	m.DisableRateLimit()
	testx.RequireEqual(t, len(m.Build(context.Background())), 0)
}

func TestManagerAppendAndOrder(t *testing.T) {
	m := NewManager()
	recovery := func(c *core.Context) { c.Next() }
	requestID := func(c *core.Context) { c.Next() }
	extra := func(c *core.Context) { c.Next() }
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
	m.RegisterBuiltin("recovery", func(c *core.Context) { c.Next() })
	m.RegisterBuiltin("request_id", func(c *core.Context) { c.Next() })
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
func sameFunc(a, b core.HandlerFunc) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}
