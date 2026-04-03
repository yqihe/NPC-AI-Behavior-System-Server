//go:build experiment

package experiment

import (
	"encoding/json"
	"fmt"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/rule"
)

// ScaleConfig 某个规模档位的配置集合
type ScaleConfig struct {
	BehaviorCount int

	// PureFSM: N 状态 + ~3N 转换规则
	PureFSMConfig *fsm.FSMConfig
	FSMTransCount int // 统计用

	// PureBT: 一棵大 Selector，N 个分支
	PureBTTreeJSON []byte
	BTNodeCount    int // 统计用

	// Hybrid: 固定 5 FSM 状态 + 5 棵 BT 子树，每棵 N/5 节点
	HybridFSM       *fsm.FSMConfig
	HybridBTrees    map[string][]byte // 状态名 → BT JSON
	HybridFSMTrans  int               // 统计用
	HybridBTTotal   int               // 统计用（所有子树节点总数）
}

// GenerateScaleConfig 为指定行为数生成三种模式的配置
func GenerateScaleConfig(behaviorCount int) *ScaleConfig {
	cfg := &ScaleConfig{BehaviorCount: behaviorCount}
	cfg.generatePureFSM(behaviorCount)
	cfg.generatePureBT(behaviorCount)
	cfg.generateHybrid(behaviorCount)
	return cfg
}

// --- PureFSM: N 状态 + ~3N 转换规则 ---

func (c *ScaleConfig) generatePureFSM(n int) {
	states := make([]fsm.StateConfig, n)
	for i := 0; i < n; i++ {
		states[i] = fsm.StateConfig{Name: fmt.Sprintf("S%d", i)}
	}

	var transitions []fsm.TransitionConfig
	for i := 0; i < n; i++ {
		// 每个状态到下一个状态（环形）
		next := (i + 1) % n
		transitions = append(transitions, fsm.TransitionConfig{
			From: fmt.Sprintf("S%d", i), To: fmt.Sprintf("S%d", next),
			Priority: 5,
			Condition: rule.Condition{
				Key: "threat_level", Op: ">=",
				Value: json.RawMessage(fmt.Sprintf("%d", 10+i)),
			},
		})
		// 每个状态到 S0（回归初始）
		if i > 0 {
			transitions = append(transitions, fsm.TransitionConfig{
				From: fmt.Sprintf("S%d", i), To: "S0",
				Priority: 1,
				Condition: rule.Condition{
					Key: "threat_level", Op: "<", Value: json.RawMessage("5"),
				},
			})
		}
		// 高优先级跳转到最后一个状态（类似 Flee）
		if i < n-1 {
			transitions = append(transitions, fsm.TransitionConfig{
				From: fmt.Sprintf("S%d", i), To: fmt.Sprintf("S%d", n-1),
				Priority: 10,
				Condition: rule.Condition{
					Key: "threat_level", Op: ">=", Value: json.RawMessage("90"),
				},
			})
		}
	}

	c.PureFSMConfig = &fsm.FSMConfig{
		InitialState: "S0",
		States:       states,
		Transitions:  transitions,
	}
	c.FSMTransCount = len(transitions)
}

// --- PureBT: 一棵大 Selector，N 个分支 ---

func (c *ScaleConfig) generatePureBT(n int) {
	type treeNode struct {
		Type     string      `json:"type"`
		Params   interface{} `json:"params,omitempty"`
		Children []treeNode  `json:"children,omitempty"`
	}

	children := make([]treeNode, n)
	nodeCount := 0
	for i := 0; i < n; i++ {
		threshold := 90 - (i * 80 / n) // 从 90 递减到 ~10
		children[i] = treeNode{
			Type: "sequence",
			Children: []treeNode{
				{Type: "check_bb_float", Params: map[string]interface{}{
					"key": "threat_level", "op": ">=", "value": threshold,
				}},
				{Type: "set_bb_value", Params: map[string]interface{}{
					"key": "fsm_state", "value": fmt.Sprintf("S%d", i),
				}},
				{Type: "stub_action", Params: map[string]interface{}{
					"name": fmt.Sprintf("action_%d", i), "result": "success",
				}},
			},
		}
		nodeCount += 4 // sequence + check + set + stub
	}

	root := treeNode{Type: "selector", Children: children}
	nodeCount++ // selector 自身

	data, _ := json.Marshal(root)
	c.PureBTTreeJSON = data
	c.BTNodeCount = nodeCount
}

// --- Hybrid: 5 FSM 状态 + 5 棵 BT 子树 ---

func (c *ScaleConfig) generateHybrid(n int) {
	hybridStates := []string{"Idle", "Patrol", "Alarmed", "Search", "Flee"}
	states := make([]fsm.StateConfig, len(hybridStates))
	for i, s := range hybridStates {
		states[i] = fsm.StateConfig{Name: s}
	}

	// 固定 ~10 条转换规则（与 N 无关）
	transitions := []fsm.TransitionConfig{
		{From: "Idle", To: "Alarmed", Priority: 10, Condition: rule.Condition{Key: "last_event_type", Op: "!=", Value: json.RawMessage(`""`)}},
		{From: "Idle", To: "Patrol", Priority: 3, Condition: rule.Condition{Key: "threat_level", Op: ">=", Value: json.RawMessage("5")}},
		{From: "Patrol", To: "Alarmed", Priority: 10, Condition: rule.Condition{Key: "threat_level", Op: ">=", Value: json.RawMessage("20")}},
		{From: "Patrol", To: "Idle", Priority: 1, Condition: rule.Condition{Key: "threat_level", Op: "<", Value: json.RawMessage("5")}},
		{From: "Alarmed", To: "Flee", Priority: 10, Condition: rule.Condition{
			And: []rule.Condition{
				{Key: "threat_level", Op: ">=", Value: json.RawMessage("50")},
				{Key: "threat_expire_at", Op: ">", RefKey: "current_time"},
			},
		}},
		{From: "Alarmed", To: "Search", Priority: 5, Condition: rule.Condition{Key: "threat_level", Op: ">=", Value: json.RawMessage("30")}},
		{From: "Alarmed", To: "Idle", Priority: 1, Condition: rule.Condition{Key: "last_event_type", Op: "==", Value: json.RawMessage(`""`)}},
		{From: "Search", To: "Flee", Priority: 10, Condition: rule.Condition{Key: "threat_level", Op: ">=", Value: json.RawMessage("50")}},
		{From: "Search", To: "Idle", Priority: 1, Condition: rule.Condition{Key: "threat_level", Op: "<", Value: json.RawMessage("10")}},
		{From: "Flee", To: "Idle", Priority: 5, Condition: rule.Condition{
			Or: []rule.Condition{
				{Key: "threat_level", Op: "<", Value: json.RawMessage("20")},
				{Key: "threat_expire_at", Op: "<=", RefKey: "current_time"},
			},
		}},
	}

	c.HybridFSM = &fsm.FSMConfig{
		InitialState: "Idle",
		States:       states,
		Transitions:  transitions,
	}
	c.HybridFSMTrans = len(transitions)

	// 每个状态一棵 BT 子树，每棵 N/5 个行为节点
	type treeNode struct {
		Type     string      `json:"type"`
		Params   interface{} `json:"params,omitempty"`
		Children []treeNode  `json:"children,omitempty"`
	}

	perState := n / 5
	if perState < 1 {
		perState = 1
	}
	c.HybridBTrees = make(map[string][]byte)
	totalNodes := 0

	for _, state := range hybridStates {
		children := make([]treeNode, perState)
		for j := 0; j < perState; j++ {
			children[j] = treeNode{
				Type: "stub_action",
				Params: map[string]interface{}{
					"name": fmt.Sprintf("%s_action_%d", state, j), "result": "success",
				},
			}
		}
		root := treeNode{Type: "sequence", Children: children}
		totalNodes += perState + 1 // sequence + N stubs

		data, _ := json.Marshal(root)
		c.HybridBTrees[state] = data
	}
	c.HybridBTTotal = totalNodes
}
