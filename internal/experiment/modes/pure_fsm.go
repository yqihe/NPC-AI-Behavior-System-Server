//go:build experiment

package modes

import (
	"math"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/experiment"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
)

// PureFSMNPC FSM only，无 BT，无决策中心
// 公平实现：取最高 severity（但无距离衰减——架构固有限制）
type PureFSMNPC struct {
	bb  *blackboard.Blackboard
	fsm *fsm.FSM
}

func NewPureFSMNPC(id string, src config.Source) (*PureFSMNPC, error) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(0))
	fsmCfg, err := src.LoadFSMConfig("civilian")
	if err != nil {
		return nil, err
	}
	f, err := fsm.NewFSM(fsmCfg, bb)
	if err != nil {
		return nil, err
	}
	return &PureFSMNPC{bb: bb, fsm: f}, nil
}

func NewPureFSMFromScale(id string, cfg *experiment.ScaleConfig) (*PureFSMNPC, error) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(0))
	f, err := fsm.NewFSM(cfg.PureFSMConfig, bb)
	if err != nil {
		return nil, err
	}
	return &PureFSMNPC{bb: bb, fsm: f}, nil
}

func (p *PureFSMNPC) Tick(events []*event.Event, evtTypes map[string]*event.EventTypeConfig, dt float64) string {
	if len(events) > 0 {
		evt := events[0]
		for _, e := range events[1:] {
			if e.Severity > evt.Severity {
				evt = e
			}
		}
		blackboard.Set(p.bb, blackboard.KeyThreatLevel, evt.Severity)
		blackboard.Set(p.bb, blackboard.KeyThreatSource, evt.ID)
		blackboard.Set(p.bb, blackboard.KeyLastEventType, evt.Type)
		ct, _ := blackboard.Get(p.bb, blackboard.KeyCurrentTime)
		blackboard.Set(p.bb, blackboard.KeyThreatExpireAt, ct+int64(evt.TTL*1000))
	} else {
		level, ok := blackboard.Get(p.bb, blackboard.KeyThreatLevel)
		if ok && level > 0 {
			newLevel := math.Max(0, level-10*dt)
			blackboard.Set(p.bb, blackboard.KeyThreatLevel, newLevel)
			if newLevel == 0 {
				blackboard.Set(p.bb, blackboard.KeyThreatSource, "")
				blackboard.Set(p.bb, blackboard.KeyLastEventType, "")
			}
		}
	}
	p.fsm.Tick(p.bb)
	return p.fsm.Current()
}

func (p *PureFSMNPC) State() string                    { return p.fsm.Current() }
func (p *PureFSMNPC) BB() *blackboard.Blackboard { return p.bb }
