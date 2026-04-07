//go:build experiment

package modes

import (
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/experiment"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/decision"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/perception"
)

// BTDCNPC BT + 决策中心，无 FSM
type BTDCNPC struct {
	bb       *blackboard.Blackboard
	tree     bt.Node
	decision *decision.Center
	percCfg  *perception.PerceptionConfig
	position event.Vec3
}

func NewBTDCNPC(id string, pos event.Vec3, src config.Source, btReg *bt.Registry) (*BTDCNPC, error) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(0))
	blackboard.Set(bb, blackboard.KeyFSMState, "Idle")
	treeData, err := src.LoadBTTree("civilian/pure_bt")
	if err != nil {
		return nil, err
	}
	tree, err := bt.BuildFromJSON(treeData, btReg)
	if err != nil {
		return nil, err
	}
	percCfg := perception.PerceptionConfig{VisualRange: 200, AuditoryRange: 500}
	return &BTDCNPC{bb: bb, tree: tree, decision: decision.NewCenter(10.0), percCfg: &percCfg, position: pos}, nil
}

func NewBTDCFromScale(id string, cfg *experiment.ScaleConfig, btReg *bt.Registry) (*BTDCNPC, error) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(0))
	blackboard.Set(bb, blackboard.KeyFSMState, "S0")
	tree, err := bt.BuildFromJSON(cfg.PureBTTreeJSON, btReg)
	if err != nil {
		return nil, err
	}
	percCfg := perception.PerceptionConfig{VisualRange: 200, AuditoryRange: 500}
	return &BTDCNPC{bb: bb, tree: tree, decision: decision.NewCenter(10.0), percCfg: &percCfg}, nil
}

func (n *BTDCNPC) Tick(events []*event.Event, evtTypes map[string]*event.EventTypeConfig, dt float64) string {
	perceived := filterPerceived(n.position, n.percCfg, events, evtTypes)
	n.decision.Evaluate(n.bb, n.position, decision.DecisionInput{Perceived: perceived, Weights: decision.DefaultWeights}, evtTypes, dt)
	ctx := &bt.Context{BB: n.bb}
	n.tree.Tick(ctx)
	return n.State()
}

func (n *BTDCNPC) State() string {
	s, ok := blackboard.Get(n.bb, blackboard.KeyFSMState)
	if !ok {
		return "Idle"
	}
	return s
}

func (n *BTDCNPC) BB() *blackboard.Blackboard { return n.bb }
