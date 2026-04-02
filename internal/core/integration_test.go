package core_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
)

func configsDir(t *testing.T) string {
	t.Helper()
	// internal/core/ → 项目根目录
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "..", "..", "configs")
}

// TestIntegration_FSM_FromConfig 完整链路：JSON 配置 → Config 加载 → FSM 创建 → BB 写入 → Tick → 状态转换
func TestIntegration_FSM_FromConfig(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))

	// 1. 加载 civilian FSM 配置
	fsmCfg, err := src.LoadFSMConfig("civilian")
	if err != nil {
		t.Fatalf("load FSM config: %v", err)
	}

	// 2. 创建 Blackboard 和 FSM
	bb := blackboard.New()
	f, err := fsm.NewFSM(fsmCfg, bb)
	if err != nil {
		t.Fatalf("create FSM: %v", err)
	}

	// 3. 初始状态应该是 Idle
	if f.Current() != "Idle" {
		t.Fatalf("expected Idle, got %s", f.Current())
	}

	// 4. 设置事件 → Tick → 应该转到 Alarmed
	blackboard.Set(bb, blackboard.KeyLastEventType, "E01")
	f.Tick(bb)
	if f.Current() != "Alarmed" {
		t.Fatalf("expected Alarmed after event, got %s", f.Current())
	}

	// 5. 设置高威胁 + 未过期 → Tick → 应该转到 Flee
	blackboard.Set(bb, blackboard.KeyThreatLevel, 75.0)
	blackboard.Set(bb, blackboard.KeyThreatExpireAt, int64(9999))
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(5000))
	f.Tick(bb)
	if f.Current() != "Flee" {
		t.Fatalf("expected Flee after high threat, got %s", f.Current())
	}

	// 6. 威胁过期 → Tick → 应该回到 Idle
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(99999))
	f.Tick(bb)
	if f.Current() != "Idle" {
		t.Fatalf("expected Idle after threat expired, got %s", f.Current())
	}
}

// TestIntegration_BT_WithBB BT 节点读写 Blackboard 的完整链路
func TestIntegration_BT_WithBB(t *testing.T) {
	treeJSON := []byte(`{
		"type": "sequence",
		"children": [
			{"type": "check_bb_float", "params": {"key": "threat_level", "op": ">=", "value": 50}},
			{"type": "set_bb_value", "params": {"key": "fsm_state", "value": "Flee"}}
		]
	}`)

	reg := bt.DefaultRegistry()
	tree, err := bt.BuildFromJSON(treeJSON, reg)
	if err != nil {
		t.Fatalf("build BT: %v", err)
	}

	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 75.0)

	ctx := &bt.Context{BB: bb}
	status := tree.Tick(ctx)
	if status != bt.Success {
		t.Fatalf("expected Success, got %s", status)
	}

	// 验证 BT 写入的值
	state, ok := bb.GetRaw("fsm_state")
	if !ok || state != "Flee" {
		t.Fatalf("expected fsm_state=Flee, got %v", state)
	}
}

// TestIntegration_FSM_BT_Combined FSM 和 BT 通过 Blackboard 协同工作
func TestIntegration_FSM_BT_Combined(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))

	fsmCfg, err := src.LoadFSMConfig("civilian")
	if err != nil {
		t.Fatal(err)
	}

	bb := blackboard.New()
	f, err := fsm.NewFSM(fsmCfg, bb)
	if err != nil {
		t.Fatal(err)
	}

	// 构建一棵简单的 BT：检查威胁等级
	treeJSON := []byte(`{
		"type": "check_bb_float",
		"params": {"key": "threat_level", "op": ">=", "value": 50}
	}`)
	reg := bt.DefaultRegistry()
	tree, err := bt.BuildFromJSON(treeJSON, reg)
	if err != nil {
		t.Fatal(err)
	}

	// 场景：事件到来 → FSM 转换 → BT 检查威胁等级
	blackboard.Set(bb, blackboard.KeyLastEventType, "E01")
	f.Tick(bb)

	blackboard.Set(bb, blackboard.KeyThreatLevel, 75.0)
	blackboard.Set(bb, blackboard.KeyThreatExpireAt, int64(9999))
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(5000))
	f.Tick(bb) // → Flee

	// BT 在 Flee 状态下检查威胁等级
	ctx := &bt.Context{BB: bb}
	status := tree.Tick(ctx)
	if status != bt.Success {
		t.Fatalf("expected BT Success in Flee state, got %s", status)
	}
	if f.Current() != "Flee" {
		t.Fatalf("expected FSM in Flee, got %s", f.Current())
	}
}
