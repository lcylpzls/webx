package middleware

import (
	"context"
	"reflect"
	"testing"

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
	if got := len(m.Build(context.Background())); got != 2 {
		t.Fatalf("注册后链长度不符：%d", got)
	}

	m.Disable("recovery")
	if got := len(m.Build(context.Background())); got != 1 {
		t.Fatalf("禁用后链长度不符：%d", got)
	}
	m.Enable("recovery")
	if got := len(m.Build(context.Background())); got != 2 {
		t.Fatalf("启用后链长度不符：%d", got)
	}

	m.Override("recovery", override)
	chain := m.Build(context.Background())
	if len(chain) != 2 {
		t.Fatalf("覆盖后链长度不符：%d", len(chain))
	}
	if !sameFunc(chain[0], override) {
		t.Error("覆盖未生效")
	}
	m.Override("missing", override)
	if got := len(m.Build(context.Background())); got != 2 {
		t.Errorf("覆盖未注册键不应改变链：%d", got)
	}
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
	if got := len(m.Build(context.Background())); got != 0 {
		t.Errorf("Enable 不应激活 rate_limit：%d", got)
	}
	m.EnableRateLimit(rl)
	if got := len(m.Build(context.Background())); got != 1 {
		t.Errorf("重新启用后链长度不符：%d", got)
	}
	m.DisableRateLimit()
	if got := len(m.Build(context.Background())); got != 0 {
		t.Errorf("DisableRateLimit 后链长度不符：%d", got)
	}
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

// sameFunc 判断两个函数值是否指向同一函数。
func sameFunc(a, b core.HandlerFunc) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}
