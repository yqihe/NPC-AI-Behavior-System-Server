package component

import (
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
)

// Component 所有 NPC 组件的基接口。
// 每个组件必须返回唯一的名称标识符。
type Component interface {
	Name() string
}

// Tickable 需要每帧更新的组件。
// behavior 组件不走此接口——AI 管线（感知→决策→FSM→BT）由 Scheduler 显式编排。
// movement/needs/emotion/memory 通过此接口泛化 Tick。
type Tickable interface {
	Component
	Tick(bb *blackboard.Blackboard, dt float64)
}
