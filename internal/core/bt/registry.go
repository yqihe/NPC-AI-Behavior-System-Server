package bt

import (
	"encoding/json"
	"fmt"
)

// NodeFactory 节点工厂函数，从 JSON 参数创建节点
type NodeFactory func(params json.RawMessage) (Node, error)

// Registry 节点类型注册表
type Registry struct {
	factories map[string]NodeFactory
}

// NewRegistry 创建空的注册表
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]NodeFactory),
	}
}

// Register 注册一个节点类型
func (r *Registry) Register(typeName string, factory NodeFactory) {
	r.factories[typeName] = factory
}

// Get 获取节点工厂，不存在返回 error
func (r *Registry) Get(typeName string) (NodeFactory, error) {
	f, ok := r.factories[typeName]
	if !ok {
		return nil, fmt.Errorf("bt: unknown node type %q", typeName)
	}
	return f, nil
}

// DefaultRegistry 创建一个包含所有内置节点的注册表
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register("sequence", func(params json.RawMessage) (Node, error) {
		return &Sequence{}, nil // children 由 builder 填充
	})
	r.Register("selector", func(params json.RawMessage) (Node, error) {
		return &Selector{}, nil
	})
	r.Register("parallel", func(params json.RawMessage) (Node, error) {
		var cfg struct {
			Policy string `json:"policy"`
		}
		policy := RequireAll
		if len(params) > 0 {
			if err := json.Unmarshal(params, &cfg); err != nil {
				return nil, fmt.Errorf("bt: parallel params: %w", err)
			}
			if cfg.Policy == "require_one" {
				policy = RequireOne
			}
		}
		return &Parallel{Policy: policy}, nil
	})
	r.Register("inverter", func(params json.RawMessage) (Node, error) {
		return &Inverter{}, nil // child 由 builder 填充
	})
	r.Register("check_bb_float", checkBBFloatFactory)
	r.Register("check_bb_string", checkBBStringFactory)
	r.Register("set_bb_value", setBBValueFactory)
	r.Register("stub_action", stubActionFactory)
	return r
}
