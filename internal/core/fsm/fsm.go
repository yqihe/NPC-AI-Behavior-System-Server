package fsm

import (
	"fmt"
	"sort"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/rule"
)

// --- 配置结构 ---

// StateConfig 一个状态的配置
type StateConfig struct {
	Name string `json:"name"`
}

// TransitionConfig 一条转换规则的配置
type TransitionConfig struct {
	From      string         `json:"from"`
	To        string         `json:"to"`
	Priority  int            `json:"priority"`
	Condition rule.Condition `json:"condition"`
}

// FSMConfig 完整的 FSM 配置
type FSMConfig struct {
	InitialState string             `json:"initial_state"`
	States       []StateConfig      `json:"states"`
	Transitions  []TransitionConfig `json:"transitions"`
}

// --- 回调 ---

// TransitionCallback FSM 状态转换时的回调
type TransitionCallback func(from, to string)

// StateCallback 进入/退出状态时的回调
type StateCallback func(state string)

// --- FSM ---

// FSM 配置驱动的有限状态机
type FSM struct {
	current     string
	stateSet    map[string]bool                // 合法状态集合
	transitions map[string][]TransitionConfig  // from_state → 按 priority 降序排列的转换列表

	onTransition TransitionCallback
	onEnter      StateCallback
	onExit       StateCallback
}

// NewFSM 从配置创建 FSM 实例，校验配置合法性
func NewFSM(cfg *FSMConfig, bb *blackboard.Blackboard) (*FSM, error) {
	if cfg == nil {
		return nil, fmt.Errorf("fsm: config is nil")
	}
	if bb == nil {
		return nil, fmt.Errorf("fsm: blackboard is nil")
	}
	if len(cfg.States) == 0 {
		return nil, fmt.Errorf("fsm: no states defined")
	}

	// 构建合法状态集合
	stateSet := make(map[string]bool, len(cfg.States))
	for _, s := range cfg.States {
		if s.Name == "" {
			return nil, fmt.Errorf("fsm: empty state name")
		}
		if stateSet[s.Name] {
			return nil, fmt.Errorf("fsm: duplicate state name %q", s.Name)
		}
		stateSet[s.Name] = true
	}

	// 校验初始状态
	if !stateSet[cfg.InitialState] {
		return nil, fmt.Errorf("fsm: initial state %q not in state set", cfg.InitialState)
	}

	// 构建转换表并校验
	transitions := make(map[string][]TransitionConfig)
	for _, t := range cfg.Transitions {
		if !stateSet[t.From] {
			return nil, fmt.Errorf("fsm: transition from unknown state %q", t.From)
		}
		if !stateSet[t.To] {
			return nil, fmt.Errorf("fsm: transition to unknown state %q", t.To)
		}
		if err := t.Condition.Validate(); err != nil {
			return nil, fmt.Errorf("fsm: transition %s→%s: %w", t.From, t.To, err)
		}
		transitions[t.From] = append(transitions[t.From], t)
	}

	// 按 priority 降序排列每个状态的转换列表
	for state := range transitions {
		sort.Slice(transitions[state], func(i, j int) bool {
			return transitions[state][i].Priority > transitions[state][j].Priority
		})
	}

	f := &FSM{
		current:     cfg.InitialState,
		stateSet:    stateSet,
		transitions: transitions,
	}

	// 写入初始状态到 Blackboard
	blackboard.Set(bb, blackboard.KeyFSMState, cfg.InitialState)

	return f, nil
}

// --- 回调注册 ---

// OnTransition 注册状态转换回调
func (f *FSM) OnTransition(cb TransitionCallback) {
	f.onTransition = cb
}

// OnEnter 注册进入状态回调
func (f *FSM) OnEnter(cb StateCallback) {
	f.onEnter = cb
}

// OnExit 注册退出状态回调
func (f *FSM) OnExit(cb StateCallback) {
	f.onExit = cb
}

// --- 运行时 ---

// Current 返回当前状态
func (f *FSM) Current() string {
	return f.current
}

// Tick 评估当前状态的所有转换条件，触发第一个匹配的转换
func (f *FSM) Tick(bb *blackboard.Blackboard) {
	trans, ok := f.transitions[f.current]
	if !ok {
		return
	}

	for i := range trans {
		if trans[i].Condition.Evaluate(bb) {
			f.transitionTo(trans[i].To, bb)
			return
		}
	}
}

// transitionTo 执行状态转换
func (f *FSM) transitionTo(to string, bb *blackboard.Blackboard) {
	from := f.current

	if f.onExit != nil {
		f.onExit(from)
	}

	f.current = to
	blackboard.Set(bb, blackboard.KeyFSMState, to)

	if f.onEnter != nil {
		f.onEnter(to)
	}

	if f.onTransition != nil {
		f.onTransition(from, to)
	}
}

// States 返回所有合法状态名（用于调试）
func (f *FSM) States() []string {
	states := make([]string, 0, len(f.stateSet))
	for s := range f.stateSet {
		states = append(states, s)
	}
	return states
}
