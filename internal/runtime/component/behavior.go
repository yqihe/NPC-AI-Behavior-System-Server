package component

import (
	"encoding/json"
	"fmt"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
)

// BehaviorComponent AI 行为组件（FSM + BT）
// 工厂只解析 JSON 填 FSMRef/BTRefs，FSM 和 BTrees 在 Instance 创建时由 BuildRuntime 构建。
type BehaviorComponent struct {
	FSMRef string            `json:"fsm_ref"`
	BTRefs map[string]string `json:"bt_refs"`

	// 运行时字段，不序列化
	FSM    *fsm.FSM          `json:"-"`
	BTrees map[string]bt.Node `json:"-"`
}

func (c *BehaviorComponent) Name() string { return "behavior" }

// BehaviorFactory 从 JSON 创建 BehaviorComponent（只解析配置引用，不加载 FSM/BT）
func BehaviorFactory(raw json.RawMessage) (Component, error) {
	var c BehaviorComponent
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("behavior: %w", err)
	}
	if c.FSMRef == "" {
		return nil, fmt.Errorf("behavior: fsm_ref is required")
	}
	if len(c.BTRefs) == 0 {
		return nil, fmt.Errorf("behavior: bt_refs must have at least one entry")
	}
	return &c, nil
}
