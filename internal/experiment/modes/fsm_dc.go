//go:build experiment

package modes

import (
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/experiment"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/decision"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/perception"
)

// FSMDCNPC FSM + 决策中心，无 BT
type FSMDCNPC struct {
	bb       *blackboard.Blackboard
	fsm      *fsm.FSM
	decision *decision.Center
	percCfg  *perception.PerceptionConfig
	position event.Vec3
}

func NewFSMDCNPC(id string, pos event.Vec3, src config.Source) (*FSMDCNPC, error) {
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
	registerLifecycleCallbacks(f, bb)
	percCfg := perception.PerceptionConfig{VisualRange: 200, AuditoryRange: 500}
	return &FSMDCNPC{bb: bb, fsm: f, decision: decision.NewCenter(10.0), percCfg: &percCfg, position: pos}, nil
}

func NewFSMDCFromScale(id string, cfg *experiment.ScaleConfig) (*FSMDCNPC, error) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(0))
	f, err := fsm.NewFSM(cfg.PureFSMConfig, bb)
	if err != nil {
		return nil, err
	}
	percCfg := perception.PerceptionConfig{VisualRange: 200, AuditoryRange: 500}
	return &FSMDCNPC{bb: bb, fsm: f, decision: decision.NewCenter(10.0), percCfg: &percCfg}, nil
}

func (n *FSMDCNPC) Tick(events []*event.Event, evtTypes map[string]*event.EventTypeConfig, dt float64) string {
	perceived := filterPerceived(n.position, n.percCfg, events, evtTypes)
	n.decision.Evaluate(n.bb, n.position, perceived, evtTypes, dt)
	n.fsm.Tick(n.bb)
	return n.fsm.Current()
}

func (n *FSMDCNPC) State() string                    { return n.fsm.Current() }
func (n *FSMDCNPC) BB() *blackboard.Blackboard { return n.bb }
