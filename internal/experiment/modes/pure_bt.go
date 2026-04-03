//go:build experiment

package modes

import (
	"math"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/experiment"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
)

// PureBTNPC BT only，无 FSM，无决策中心
// 公平实现：取最高 severity（但无距离衰减——架构固有限制）
type PureBTNPC struct {
	bb   *blackboard.Blackboard
	tree bt.Node
}

func NewPureBTNPC(id string, src config.Source, btReg *bt.Registry) (*PureBTNPC, error) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(0))
	blackboard.Set(bb, blackboard.KeyFSMState, "Idle")
	treeData, err := src.LoadBTTree("civilian_pure_bt")
	if err != nil {
		return nil, err
	}
	tree, err := bt.BuildFromJSON(treeData, btReg)
	if err != nil {
		return nil, err
	}
	return &PureBTNPC{bb: bb, tree: tree}, nil
}

func NewPureBTFromScale(id string, cfg *experiment.ScaleConfig, btReg *bt.Registry) (*PureBTNPC, error) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(0))
	blackboard.Set(bb, blackboard.KeyFSMState, "S0")
	tree, err := bt.BuildFromJSON(cfg.PureBTTreeJSON, btReg)
	if err != nil {
		return nil, err
	}
	return &PureBTNPC{bb: bb, tree: tree}, nil
}

func (p *PureBTNPC) Tick(events []*event.Event, evtTypes map[string]*event.EventTypeConfig, dt float64) string {
	if len(events) > 0 {
		best := events[0]
		for _, e := range events[1:] {
			if e.Severity > best.Severity {
				best = e
			}
		}
		blackboard.Set(p.bb, blackboard.KeyThreatLevel, best.Severity)
		blackboard.Set(p.bb, blackboard.KeyLastEventType, best.Type)
		blackboard.Set(p.bb, blackboard.KeyThreatSource, best.ID)
	} else {
		level, ok := blackboard.Get(p.bb, blackboard.KeyThreatLevel)
		if ok && level > 0 {
			newLevel := math.Max(0, level-10*dt)
			blackboard.Set(p.bb, blackboard.KeyThreatLevel, newLevel)
			if newLevel == 0 {
				blackboard.Set(p.bb, blackboard.KeyLastEventType, "")
				blackboard.Set(p.bb, blackboard.KeyThreatSource, "")
			}
		}
	}
	ctx := &bt.Context{BB: p.bb}
	p.tree.Tick(ctx)
	return p.State()
}

func (p *PureBTNPC) State() string {
	s, ok := blackboard.Get(p.bb, blackboard.KeyFSMState)
	if !ok {
		return "Idle"
	}
	return s
}

func (p *PureBTNPC) BB() *blackboard.Blackboard { return p.bb }
