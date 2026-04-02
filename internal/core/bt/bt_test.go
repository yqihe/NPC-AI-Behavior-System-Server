package bt_test

import (
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
)

// --- 辅助节点 ---

type fixedNode struct {
	status bt.Status
}

func (f *fixedNode) Tick(ctx *bt.Context) bt.Status { return f.status }

func fixed(s bt.Status) bt.Node { return &fixedNode{status: s} }

func newCtx() *bt.Context {
	return &bt.Context{BB: blackboard.New()}
}

// --- Status ---

func TestStatus_String(t *testing.T) {
	if bt.Success.String() != "Success" {
		t.Fatal("expected Success")
	}
	if bt.Failure.String() != "Failure" {
		t.Fatal("expected Failure")
	}
	if bt.Running.String() != "Running" {
		t.Fatal("expected Running")
	}
}

// --- Sequence ---

func TestSequence_AllSuccess(t *testing.T) {
	seq := &bt.Sequence{Children: []bt.Node{fixed(bt.Success), fixed(bt.Success), fixed(bt.Success)}}
	if seq.Tick(newCtx()) != bt.Success {
		t.Fatal("expected Success when all children succeed")
	}
}

func TestSequence_FirstFailure(t *testing.T) {
	seq := &bt.Sequence{Children: []bt.Node{fixed(bt.Success), fixed(bt.Failure), fixed(bt.Success)}}
	if seq.Tick(newCtx()) != bt.Failure {
		t.Fatal("expected Failure when any child fails")
	}
}

func TestSequence_Running(t *testing.T) {
	seq := &bt.Sequence{Children: []bt.Node{fixed(bt.Success), fixed(bt.Running), fixed(bt.Success)}}
	if seq.Tick(newCtx()) != bt.Running {
		t.Fatal("expected Running when child returns Running")
	}
}

func TestSequence_Empty(t *testing.T) {
	seq := &bt.Sequence{}
	if seq.Tick(newCtx()) != bt.Success {
		t.Fatal("expected Success for empty sequence")
	}
}

// --- Selector ---

func TestSelector_FirstSuccess(t *testing.T) {
	sel := &bt.Selector{Children: []bt.Node{fixed(bt.Failure), fixed(bt.Success), fixed(bt.Failure)}}
	if sel.Tick(newCtx()) != bt.Success {
		t.Fatal("expected Success when any child succeeds")
	}
}

func TestSelector_AllFailure(t *testing.T) {
	sel := &bt.Selector{Children: []bt.Node{fixed(bt.Failure), fixed(bt.Failure)}}
	if sel.Tick(newCtx()) != bt.Failure {
		t.Fatal("expected Failure when all children fail")
	}
}

func TestSelector_Running(t *testing.T) {
	sel := &bt.Selector{Children: []bt.Node{fixed(bt.Failure), fixed(bt.Running), fixed(bt.Success)}}
	if sel.Tick(newCtx()) != bt.Running {
		t.Fatal("expected Running when child returns Running")
	}
}

func TestSelector_Empty(t *testing.T) {
	sel := &bt.Selector{}
	if sel.Tick(newCtx()) != bt.Failure {
		t.Fatal("expected Failure for empty selector")
	}
}

// --- Parallel ---

func TestParallel_RequireAll_AllSuccess(t *testing.T) {
	p := &bt.Parallel{
		Children: []bt.Node{fixed(bt.Success), fixed(bt.Success)},
		Policy:   bt.RequireAll,
	}
	if p.Tick(newCtx()) != bt.Success {
		t.Fatal("expected Success")
	}
}

func TestParallel_RequireAll_OneFailure(t *testing.T) {
	p := &bt.Parallel{
		Children: []bt.Node{fixed(bt.Success), fixed(bt.Failure)},
		Policy:   bt.RequireAll,
	}
	if p.Tick(newCtx()) != bt.Failure {
		t.Fatal("expected Failure")
	}
}

func TestParallel_RequireAll_OneRunning(t *testing.T) {
	p := &bt.Parallel{
		Children: []bt.Node{fixed(bt.Success), fixed(bt.Running)},
		Policy:   bt.RequireAll,
	}
	if p.Tick(newCtx()) != bt.Running {
		t.Fatal("expected Running")
	}
}

func TestParallel_RequireOne_OneSuccess(t *testing.T) {
	p := &bt.Parallel{
		Children: []bt.Node{fixed(bt.Failure), fixed(bt.Success)},
		Policy:   bt.RequireOne,
	}
	if p.Tick(newCtx()) != bt.Success {
		t.Fatal("expected Success")
	}
}

func TestParallel_RequireOne_AllFailure(t *testing.T) {
	p := &bt.Parallel{
		Children: []bt.Node{fixed(bt.Failure), fixed(bt.Failure)},
		Policy:   bt.RequireOne,
	}
	if p.Tick(newCtx()) != bt.Failure {
		t.Fatal("expected Failure")
	}
}

// --- Inverter ---

func TestInverter_Success(t *testing.T) {
	inv := &bt.Inverter{Child: fixed(bt.Success)}
	if inv.Tick(newCtx()) != bt.Failure {
		t.Fatal("expected Failure (inverted Success)")
	}
}

func TestInverter_Failure(t *testing.T) {
	inv := &bt.Inverter{Child: fixed(bt.Failure)}
	if inv.Tick(newCtx()) != bt.Success {
		t.Fatal("expected Success (inverted Failure)")
	}
}

func TestInverter_Running(t *testing.T) {
	inv := &bt.Inverter{Child: fixed(bt.Running)}
	if inv.Tick(newCtx()) != bt.Running {
		t.Fatal("expected Running (unchanged)")
	}
}

// --- 嵌套组合 ---

func TestNested_SelectorInSequence(t *testing.T) {
	// Sequence( Selector(Fail, Success), Success ) → Success
	tree := &bt.Sequence{
		Children: []bt.Node{
			&bt.Selector{Children: []bt.Node{fixed(bt.Failure), fixed(bt.Success)}},
			fixed(bt.Success),
		},
	}
	if tree.Tick(newCtx()) != bt.Success {
		t.Fatal("expected Success for nested Selector-in-Sequence")
	}
}

func TestNested_InverterInSelector(t *testing.T) {
	// Selector( Inverter(Success), Success ) → first child Failure, second Success → Success
	tree := &bt.Selector{
		Children: []bt.Node{
			&bt.Inverter{Child: fixed(bt.Success)},
			fixed(bt.Success),
		},
	}
	if tree.Tick(newCtx()) != bt.Success {
		t.Fatal("expected Success")
	}
}
