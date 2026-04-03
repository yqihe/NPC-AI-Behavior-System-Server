//go:build experiment

package modes

import (
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/experiment"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/decision"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/perception"
)

// HybridNPC FSM + BT + DC 完整架构
type HybridNPC struct {
	inst     *npc.Instance
	decision *decision.Center
}

// NewHybridNPC 从 JSON 配置创建（定性场景用）
func NewHybridNPC(id string, pos event.Vec3, src config.Source, btReg *bt.Registry) (*HybridNPC, error) {
	rawCfg, err := src.LoadNPCTypeConfig("civilian")
	if err != nil {
		return nil, err
	}
	typeCfg, err := npc.ParseNPCTypeConfig(rawCfg)
	if err != nil {
		return nil, err
	}
	inst, err := npc.NewInstance(id, pos, typeCfg, src, btReg)
	if err != nil {
		return nil, err
	}
	registerLifecycleCallbacks(inst.FSM, inst.BB)
	return &HybridNPC{inst: inst, decision: decision.NewCenter(10.0)}, nil
}

// NewHybridFromScale 从规模配置创建（定量测试用）
func NewHybridFromScale(id string, cfg *experiment.ScaleConfig, btReg *bt.Registry) (*HybridNPC, error) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(0))
	f, err := fsm.NewFSM(cfg.HybridFSM, bb)
	if err != nil {
		return nil, err
	}
	btrees := make(map[string]bt.Node)
	for state, data := range cfg.HybridBTrees {
		tree, err := bt.BuildFromJSON(data, btReg)
		if err != nil {
			return nil, err
		}
		btrees[state] = tree
	}
	percCfg := perception.PerceptionConfig{VisualRange: 200, AuditoryRange: 500}
	inst := &npc.Instance{
		ID: id, TypeName: "hybrid_scale", Position: event.Vec3{},
		BB: bb, FSM: f, BTrees: btrees, Perception: &percCfg,
	}
	registerLifecycleCallbacks(f, bb)
	return &HybridNPC{inst: inst, decision: decision.NewCenter(10.0)}, nil
}

func (h *HybridNPC) Tick(events []*event.Event, evtTypes map[string]*event.EventTypeConfig, dt float64) string {
	perceived := filterPerceived(h.inst.Position, h.inst.Perception, events, evtTypes)
	h.decision.Evaluate(h.inst.BB, h.inst.Position, perceived, evtTypes, dt)
	h.inst.Tick()
	return h.inst.FSM.Current()
}

func (h *HybridNPC) State() string { return h.inst.FSM.Current() }
func (h *HybridNPC) BB() *blackboard.Blackboard { return h.inst.BB }

// --- 共用工具 ---

func filterPerceived(pos event.Vec3, cfg *perception.PerceptionConfig, events []*event.Event, evtTypes map[string]*event.EventTypeConfig) []*event.Event {
	perceived := make([]*event.Event, 0, len(events))
	for _, evt := range events {
		typeCfg, ok := evtTypes[evt.Type]
		if !ok {
			continue
		}
		if perception.CanPerceive(pos, cfg, evt, typeCfg) {
			perceived = append(perceived, evt)
		}
	}
	return perceived
}

func registerLifecycleCallbacks(f *fsm.FSM, bb *blackboard.Blackboard) {
	f.OnEnter(func(state string) {
		if state == "Alarmed" {
			ct, _ := blackboard.Get(bb, blackboard.KeyCurrentTime)
			blackboard.Set(bb, blackboard.KeyAlertStartTick, ct)
		}
	})
	f.OnExit(func(state string) {
		if state == "Alarmed" {
			blackboard.Delete(bb, blackboard.KeyAlertStartTick)
			blackboard.Set(bb, blackboard.KeyExitCleanupDone, "alarmed_cleaned")
		}
	})
}
