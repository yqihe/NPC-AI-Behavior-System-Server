package runtime_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/decision"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc/npctest"
)

// --- 需求优先：NPC 饥饿且无威胁 ---

func TestDecisionIntegration_NeedsPriority_NoThreat(t *testing.T) {
	src, btReg, compReg, evtTypes := newTestEnv(t)

	// 创建一个有 needs + personality 的 NPC（wolf ADMIN 模板 + extras 注入）
	extras := map[string]json.RawMessage{
		"needs":       []byte(`{"need_types":[{"name":"hunger","current":10,"max":100,"decay_rate":5}]}`),
		"personality": []byte(`{"personality_type":"docile","decision_weights":{"threat":0.3,"needs":0.5,"emotion":0.2}}`),
	}
	inst, err := npctest.NewInstanceWithExtras("npc_hungry", event.Vec3{X: 100, Z: 100},
		wolfADMINTemplate(nil), extras, src, btReg, compReg)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	bus := event.NewBus()
	reg := npc.NewRegistry()
	reg.Add(inst)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)

	blackboard.Set(inst.BB, blackboard.KeyCurrentTime, int64(10000))
	// Tick 1：needs 组件写 BB（need_lowest_val），decision 读到旧值（0）
	sched.Tick(0.1)
	// Tick 2：decision 读到 Tick 1 写入的 need_lowest_val
	sched.Tick(0.1)

	// 无威胁时，decision_winner 应为 "needs"（hunger urgency=(100-current)/100*100）
	winner, _ := blackboard.Get(inst.BB, blackboard.KeyDecisionWinner)
	if winner != "needs" {
		t.Errorf("decision_winner = %q, want %q (hungry NPC, no threat)", winner, "needs")
	}

	ns, _ := blackboard.Get(inst.BB, blackboard.KeyNeedScore)
	if ns < 80 {
		t.Errorf("need_score = %f, want >= 80", ns)
	}
}

// --- 情绪优先：timid NPC 高恐惧 ---

func TestDecisionIntegration_EmotionPriority_Timid(t *testing.T) {
	src, btReg, compReg, evtTypes := newTestEnv(t)

	extras := map[string]json.RawMessage{
		"personality": []byte(`{"personality_type":"timid","decision_weights":{"threat":0.2,"needs":0.2,"emotion":0.6},"flee_threshold":30}`),
		"emotion":     []byte(`{"emotion_states":[{"name":"fear","value":80,"accumulate_rate":10,"decay_rate":1}]}`),
	}
	inst, err := npctest.NewInstanceWithExtras("npc_timid", event.Vec3{X: 100, Z: 100},
		wolfADMINTemplate(nil), extras, src, btReg, compReg)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	bus := event.NewBus()
	// 小威胁事件（远距离）
	explosionCfg := evtTypes["explosion"]
	evt := event.NewEvent(explosionCfg, event.Vec3{X: 400, Y: 0, Z: 100}, "bomb", 30, "")
	bus.Publish(evt)

	reg := npc.NewRegistry()
	reg.Add(inst)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)

	blackboard.Set(inst.BB, blackboard.KeyCurrentTime, int64(10000))
	// Tick 1：emotion 组件写 BB，decision 读到旧值
	sched.Tick(0.1)
	// Tick 2：decision 读到 Tick 1 写入的 emotion_dominant_val
	sched.Tick(0.1)

	// 情绪权重 0.6，恐惧经衰减后仍较高 → emotion 应该赢
	winner, _ := blackboard.Get(inst.BB, blackboard.KeyDecisionWinner)
	if winner != "emotion" {
		// 查看各维度分数辅助调试
		ts, _ := blackboard.Get(inst.BB, blackboard.KeyThreatScore)
		es, _ := blackboard.Get(inst.BB, blackboard.KeyEmotionScore)
		t.Errorf("decision_winner = %q, want %q (timid + high fear). threat_score=%f emotion_score=%f", winner, "emotion", ts, es)
	}
}

// --- 高威胁压制 ---

func TestDecisionIntegration_ThreatOverride(t *testing.T) {
	src, btReg, compReg, evtTypes := newTestEnv(t)

	extras := map[string]json.RawMessage{
		"personality": []byte(`{"personality_type":"docile","decision_weights":{"threat":0.4,"needs":0.3,"emotion":0.3}}`),
		"needs":       []byte(`{"need_types":[{"name":"hunger","current":30,"max":100,"decay_rate":5}]}`),
		"emotion":     []byte(`{"emotion_states":[{"name":"fear","value":40,"accumulate_rate":10,"decay_rate":1}]}`),
	}
	inst, err := npctest.NewInstanceWithExtras("npc_balanced", event.Vec3{X: 100, Z: 100},
		wolfADMINTemplate(nil), extras, src, btReg, compReg)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	bus := event.NewBus()
	// 近距离高威胁事件
	explosionCfg := evtTypes["explosion"]
	evt := event.NewEvent(explosionCfg, event.Vec3{X: 110, Y: 0, Z: 100}, "bomb", 80, "")
	bus.Publish(evt)

	reg := npc.NewRegistry()
	reg.Add(inst)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)

	blackboard.Set(inst.BB, blackboard.KeyCurrentTime, int64(10000))
	sched.Tick(0.1)

	winner, _ := blackboard.Get(inst.BB, blackboard.KeyDecisionWinner)
	if winner != "threat" {
		ts, _ := blackboard.Get(inst.BB, blackboard.KeyThreatScore)
		ns, _ := blackboard.Get(inst.BB, blackboard.KeyNeedScore)
		es, _ := blackboard.Get(inst.BB, blackboard.KeyEmotionScore)
		t.Errorf("decision_winner = %q, want %q (close explosion). scores: threat=%f need=%f emotion=%f", winner, "threat", ts, ns, es)
	}
}

// --- 默认权重（无 personality）始终 threat ---

func TestDecisionIntegration_DefaultWeights(t *testing.T) {
	src, btReg, compReg, evtTypes := newTestEnv(t)

	// 无 personality 组件（ADMIN shape + 不 opt-in、不 extras 注入）→ 走默认权重
	// 通过 fields 去掉 aggression 避免触发任何 personality 推断
	tmplNoPersonality := wolfADMINTemplate(nil)
	delete(tmplNoPersonality.Fields, "aggression")
	extras := map[string]json.RawMessage{
		"needs":   []byte(`{"need_types":[{"name":"hunger","current":5,"max":100,"decay_rate":5}]}`),
		"emotion": []byte(`{"emotion_states":[{"name":"fear","value":90,"accumulate_rate":10,"decay_rate":1}]}`),
	}
	inst, err := npctest.NewInstanceWithExtras("npc_nopersonality", event.Vec3{X: 100, Z: 100},
		tmplNoPersonality, extras, src, btReg, compReg)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	bus := event.NewBus()
	// 微弱威胁
	shoutCfg := evtTypes["shout"]
	evt := event.NewEvent(shoutCfg, event.Vec3{X: 110, Y: 0, Z: 100}, "npc2", 10, "")
	bus.Publish(evt)

	reg := npc.NewRegistry()
	reg.Add(inst)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)

	blackboard.Set(inst.BB, blackboard.KeyCurrentTime, int64(10000))
	sched.Tick(0.1)

	// 默认权重 {1,0,0} → needs 和 emotion 的加权分=0 → 即使微弱威胁也赢
	winner, _ := blackboard.Get(inst.BB, blackboard.KeyDecisionWinner)
	if winner != "threat" {
		t.Errorf("decision_winner = %q, want %q (default weights, no personality)", winner, "threat")
	}
}
