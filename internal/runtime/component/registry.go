package component

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// Factory 组件工厂函数，从 JSON 配置创建组件实例
type Factory func(raw json.RawMessage) (Component, error)

// Registry 组件注册表，管理组件名 → 工厂函数的映射
type Registry struct {
	factories map[string]Factory
}

// NewRegistry 创建空的组件注册表
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]Factory),
	}
}

// Register 注册一个组件工厂。重复注册同名组件会 panic。
func (r *Registry) Register(name string, factory Factory) {
	if _, exists := r.factories[name]; exists {
		panic(fmt.Sprintf("component: duplicate registration for %q", name))
	}
	r.factories[name] = factory
	slog.Debug("component.registered", "name", name)
}

// Create 通过名称和 JSON 配置创建组件实例
func (r *Registry) Create(name string, raw json.RawMessage) (Component, error) {
	factory, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("component: unknown type %q", name)
	}
	comp, err := factory(raw)
	if err != nil {
		return nil, fmt.Errorf("component: create %q: %w", name, err)
	}
	return comp, nil
}

// Has 检查是否已注册指定名称的组件
func (r *Registry) Has(name string) bool {
	_, ok := r.factories[name]
	return ok
}
