package bt

import (
	"encoding/json"
	"fmt"
)

// TreeConfig BT 树的 JSON 配置结构
type TreeConfig struct {
	Type     string           `json:"type"`
	Params   json.RawMessage  `json:"params,omitempty"`
	Children []TreeConfig     `json:"children,omitempty"`
	Child    *TreeConfig      `json:"child,omitempty"` // 装饰节点用
}

// Build 从 TreeConfig 递归构建节点树
func Build(cfg *TreeConfig, reg *Registry) (Node, error) {
	if cfg == nil {
		return nil, fmt.Errorf("bt: tree config is nil")
	}

	factory, err := reg.Get(cfg.Type)
	if err != nil {
		return nil, err
	}

	node, err := factory(cfg.Params)
	if err != nil {
		return nil, fmt.Errorf("bt: create node %q: %w", cfg.Type, err)
	}

	// 填充子节点
	switch n := node.(type) {
	case *Sequence:
		children, err := buildChildren(cfg.Children, reg)
		if err != nil {
			return nil, err
		}
		n.Children = children
	case *Selector:
		children, err := buildChildren(cfg.Children, reg)
		if err != nil {
			return nil, err
		}
		n.Children = children
	case *Parallel:
		children, err := buildChildren(cfg.Children, reg)
		if err != nil {
			return nil, err
		}
		n.Children = children
	case *Inverter:
		if cfg.Child == nil {
			return nil, fmt.Errorf("bt: inverter requires a child node")
		}
		child, err := Build(cfg.Child, reg)
		if err != nil {
			return nil, err
		}
		n.Child = child
	}

	return node, nil
}

// BuildFromJSON 从 JSON 字节构建节点树
func BuildFromJSON(data []byte, reg *Registry) (Node, error) {
	var cfg TreeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("bt: parse tree config: %w", err)
	}
	return Build(&cfg, reg)
}

func buildChildren(cfgs []TreeConfig, reg *Registry) ([]Node, error) {
	children := make([]Node, len(cfgs))
	for i := range cfgs {
		child, err := Build(&cfgs[i], reg)
		if err != nil {
			return nil, err
		}
		children[i] = child
	}
	return children, nil
}
