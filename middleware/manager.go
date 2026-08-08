// Package middleware 提供 webx 内置的 HTTP 中间件实现。
// 包含 Recovery、RequestID、Timeout、CORS、Validation、RateLimit 与 AccessLog。
package middleware

import (
	"context"
	"sync"

	"github.com/lcylpzls/webx/internal/core"
)

// Manager 管理内置中间件的注册表、执行顺序和启用状态。
type Manager struct {
	builtins  map[string]core.HandlerFunc
	overrides map[string]core.HandlerFunc
	disabled  map[string]bool
	order     []string
	extras    []core.HandlerFunc
	mu        sync.RWMutex
}

// NewManager 创建中间件管理器。
// 默认启用全部内置中间件（RateLimit 注册但默认禁用）。
func NewManager() *Manager {
	m := &Manager{
		builtins:  make(map[string]core.HandlerFunc),
		overrides: make(map[string]core.HandlerFunc),
		disabled:  make(map[string]bool),
		order: []string{
			"recovery",
			"request_id",
			"timeout",
			"cors",
			"validation",
			"security",
			"gzip",
			"rate_limit",
			"metrics",
			"access_log",
		},
		extras: make([]core.HandlerFunc, 0),
	}
	m.disabled["rate_limit"] = true
	return m
}

// RegisterBuiltin 注册一个内置中间件到管理器。
func (m *Manager) RegisterBuiltin(key string, handler core.HandlerFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.builtins[key] = handler
}

// Override 覆盖指定类型的内置中间件。
func (m *Manager) Override(mt string, handler core.HandlerFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.overrides[mt] = handler
}

// Disable 禁用指定类型的内置中间件。
func (m *Manager) Disable(mt ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range mt {
		m.disabled[t] = true
	}
}

// Enable 启用指定类型的内置中间件。
// 注意：RateLimit 必须通过 EnableRateLimit 激活，Enable 对其无效。
func (m *Manager) Enable(mt ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range mt {
		if t == "rate_limit" {
			continue
		}
		delete(m.disabled, t)
	}
}

// EnableRateLimit 启用限流中间件并注册其 Handler。
func (m *Manager) EnableRateLimit(handler core.HandlerFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.builtins["rate_limit"] = handler
	delete(m.disabled, "rate_limit")
}

// DisableRateLimit 禁用限流中间件并移除其 Handler。
func (m *Manager) DisableRateLimit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disabled["rate_limit"] = true
	delete(m.builtins, "rate_limit")
}

// Append 追加外部全局中间件到中间件链末尾（路由专属中间件之前）。
func (m *Manager) Append(handler ...core.HandlerFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.extras = append(m.extras, handler...)
}

// SetOrder 设置内置中间件的执行顺序（未知键与空顺序忽略）。
func (m *Manager) SetOrder(keys ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	known := make(map[string]bool, len(m.order))
	for _, k := range m.order {
		known[k] = true
	}
	order := make([]string, 0, len(keys))
	for _, k := range keys {
		if known[k] {
			order = append(order, k)
		}
	}
	if len(order) > 0 {
		m.order = order
	}
}

// Build 构建最终执行的中间件链。
// 返回顺序：内置（启用）→ 外部全局。
func (m *Manager) Build(ctx context.Context) []core.HandlerFunc {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var chain []core.HandlerFunc
	for _, key := range m.order {
		if m.disabled[key] {
			continue
		}
		if h, ok := m.overrides[key]; ok {
			chain = append(chain, h)
			continue
		}
		if h, ok := m.builtins[key]; ok {
			chain = append(chain, h)
		}
	}
	chain = append(chain, m.extras...)
	return chain
}
