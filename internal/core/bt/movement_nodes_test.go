package bt_test

import (
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
)

// --- move_to ---

func buildMoveTo(t *testing.T, body string) bt.Node {
	t.Helper()
	n, err := bt.BuildFromJSON([]byte(body), bt.DefaultRegistry())
	if err != nil {
		t.Fatalf("build move_to: %v", err)
	}
	return n
}

func TestMoveTo_ArrivedWithinThreshold(t *testing.T) {
	n := buildMoveTo(t, `{"type":"move_to","params":{"target_key_x":"move_target_x","target_key_z":"move_target_z","speed":3}}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 10.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 10.0)
	bb.SetRaw("move_target_x", 10.5)
	bb.SetRaw("move_target_z", 10.0)

	if got := n.Tick(&bt.Context{BB: bb, DeltaTime: 0.1}); got != bt.Success {
		t.Fatalf("expected Success when within 1.0 of target, got %v", got)
	}
	state, _ := blackboard.Get(bb, blackboard.KeyMoveState)
	if state != "arrived" {
		t.Fatalf("expected move_state=arrived, got %q", state)
	}
}

func TestMoveTo_RunningStepsTowardTarget(t *testing.T) {
	n := buildMoveTo(t, `{"type":"move_to","params":{"target_key_x":"move_target_x","target_key_z":"move_target_z","speed":10}}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	bb.SetRaw("move_target_x", 100.0)
	bb.SetRaw("move_target_z", 0.0)

	if got := n.Tick(&bt.Context{BB: bb, DeltaTime: 0.1}); got != bt.Running {
		t.Fatalf("expected Running when far from target, got %v", got)
	}
	newX, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	if newX <= 0 || newX >= 100 {
		t.Fatalf("expected 0 < newX < 100, got %v", newX)
	}
	state, _ := blackboard.Get(bb, blackboard.KeyMoveState)
	if state != "moving" {
		t.Fatalf("expected move_state=moving, got %q", state)
	}
}

func TestMoveTo_ZeroDeltaTimeFallsBackTo100ms(t *testing.T) {
	n := buildMoveTo(t, `{"type":"move_to","params":{"target_key_x":"move_target_x","target_key_z":"move_target_z","speed":10}}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	bb.SetRaw("move_target_x", 100.0)
	bb.SetRaw("move_target_z", 0.0)

	if got := n.Tick(&bt.Context{BB: bb, DeltaTime: 0}); got != bt.Running {
		t.Fatalf("expected Running, got %v", got)
	}
	newX, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	if newX != 1.0 {
		t.Fatalf("expected fallback dt=0.1 × speed=10 = 1.0 step, got %v", newX)
	}
}

func TestMoveTo_MissingTargetKeyFailure(t *testing.T) {
	n := buildMoveTo(t, `{"type":"move_to","params":{"target_key_x":"move_target_x","target_key_z":"move_target_z","speed":3}}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)

	if got := n.Tick(&bt.Context{BB: bb, DeltaTime: 0.1}); got != bt.Failure {
		t.Fatalf("expected Failure when target keys missing, got %v", got)
	}
}

func TestMoveTo_NonNumericTargetFailure(t *testing.T) {
	n := buildMoveTo(t, `{"type":"move_to","params":{"target_key_x":"move_target_x","target_key_z":"move_target_z","speed":3}}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	bb.SetRaw("move_target_x", "not-a-number")
	bb.SetRaw("move_target_z", 10.0)

	if got := n.Tick(&bt.Context{BB: bb, DeltaTime: 0.1}); got != bt.Failure {
		t.Fatalf("expected Failure on non-numeric X, got %v", got)
	}

	bb.SetRaw("move_target_x", 10.0)
	bb.SetRaw("move_target_z", "not-a-number")
	if got := n.Tick(&bt.Context{BB: bb, DeltaTime: 0.1}); got != bt.Failure {
		t.Fatalf("expected Failure on non-numeric Z, got %v", got)
	}
}

func TestMoveTo_FactoryErrors(t *testing.T) {
	reg := bt.DefaultRegistry()
	factory, _ := reg.Get("move_to")

	if _, err := factory([]byte(`{not json`)); err == nil {
		t.Fatal("expected error on bad JSON")
	}
	if _, err := factory([]byte(`{"target_key_x":"","target_key_z":""}`)); err == nil {
		t.Fatal("expected error on missing required keys")
	}
	// speed default fallback (speed<=0 → 3.0)
	node, err := factory([]byte(`{"target_key_x":"move_target_x","target_key_z":"move_target_z","speed":0}`))
	if err != nil || node == nil {
		t.Fatalf("expected default speed fallback, got err=%v", err)
	}
}

// --- flee_from ---

func TestFleeFrom_AlreadyFarEnough(t *testing.T) {
	n := buildMoveTo(t, `{"type":"flee_from","params":{"source_key_x":"move_target_x","source_key_z":"move_target_z","distance":50,"speed":5}}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 100.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	bb.SetRaw("move_target_x", 0.0)
	bb.SetRaw("move_target_z", 0.0)

	if got := n.Tick(&bt.Context{BB: bb, DeltaTime: 0.1}); got != bt.Success {
		t.Fatalf("expected Success when already beyond flee distance, got %v", got)
	}
	state, _ := blackboard.Get(bb, blackboard.KeyMoveState)
	if state != "arrived" {
		t.Fatalf("expected move_state=arrived, got %q", state)
	}
}

func TestFleeFrom_RunningMovesAwayFromSource(t *testing.T) {
	n := buildMoveTo(t, `{"type":"flee_from","params":{"source_key_x":"move_target_x","source_key_z":"move_target_z","distance":50,"speed":10}}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 10.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	bb.SetRaw("move_target_x", 0.0)
	bb.SetRaw("move_target_z", 0.0)

	if got := n.Tick(&bt.Context{BB: bb, DeltaTime: 0.1}); got != bt.Running {
		t.Fatalf("expected Running while within flee distance, got %v", got)
	}
	newX, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	if newX <= 10.0 {
		t.Fatalf("expected NPC to move farther from source (>10), got %v", newX)
	}
}

func TestFleeFrom_SourceOverlapDefaultsToXPositive(t *testing.T) {
	n := buildMoveTo(t, `{"type":"flee_from","params":{"source_key_x":"move_target_x","source_key_z":"move_target_z","distance":50,"speed":10}}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	bb.SetRaw("move_target_x", 0.0)
	bb.SetRaw("move_target_z", 0.0)

	if got := n.Tick(&bt.Context{BB: bb, DeltaTime: 0.1}); got != bt.Running {
		t.Fatalf("expected Running, got %v", got)
	}
	newX, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	newZ, _ := blackboard.Get(bb, blackboard.KeyNPCPosZ)
	if newX <= 0 || newZ != 0 {
		t.Fatalf("overlap should flee along +X only, got (%v, %v)", newX, newZ)
	}
}

func TestFleeFrom_ZeroDeltaTimeFallsBackTo100ms(t *testing.T) {
	n := buildMoveTo(t, `{"type":"flee_from","params":{"source_key_x":"move_target_x","source_key_z":"move_target_z","distance":50,"speed":10}}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 1.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	bb.SetRaw("move_target_x", 0.0)
	bb.SetRaw("move_target_z", 0.0)

	if got := n.Tick(&bt.Context{BB: bb, DeltaTime: 0}); got != bt.Running {
		t.Fatalf("expected Running, got %v", got)
	}
	newX, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	// dx=1, norm=1, step=10*0.1=1, newX=1+1=2
	if newX != 2.0 {
		t.Fatalf("expected fallback dt=0.1 step=1, newX=2, got %v", newX)
	}
}

func TestFleeFrom_MissingOrNonNumericFailure(t *testing.T) {
	n := buildMoveTo(t, `{"type":"flee_from","params":{"source_key_x":"move_target_x","source_key_z":"move_target_z","distance":50,"speed":5}}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 10.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)

	if got := n.Tick(&bt.Context{BB: bb, DeltaTime: 0.1}); got != bt.Failure {
		t.Fatalf("expected Failure when source keys missing, got %v", got)
	}

	bb.SetRaw("move_target_x", "nope")
	bb.SetRaw("move_target_z", 0.0)
	if got := n.Tick(&bt.Context{BB: bb, DeltaTime: 0.1}); got != bt.Failure {
		t.Fatalf("expected Failure on non-numeric source X, got %v", got)
	}
	bb.SetRaw("move_target_x", 0.0)
	bb.SetRaw("move_target_z", "nope")
	if got := n.Tick(&bt.Context{BB: bb, DeltaTime: 0.1}); got != bt.Failure {
		t.Fatalf("expected Failure on non-numeric source Z, got %v", got)
	}
}

func TestFleeFrom_FactoryErrors(t *testing.T) {
	reg := bt.DefaultRegistry()
	factory, _ := reg.Get("flee_from")

	if _, err := factory([]byte(`{not json`)); err == nil {
		t.Fatal("expected error on bad JSON")
	}
	if _, err := factory([]byte(`{"source_key_x":""}`)); err == nil {
		t.Fatal("expected error on missing required keys")
	}
	// distance & speed default fallback
	node, err := factory([]byte(`{"source_key_x":"move_target_x","source_key_z":"move_target_z","distance":0,"speed":0}`))
	if err != nil || node == nil {
		t.Fatalf("expected default fallback, got err=%v", err)
	}
}

// --- stub_action ---

func TestStubAction_AllResults(t *testing.T) {
	cases := []struct {
		result string
		want   bt.Status
	}{
		{"success", bt.Success},
		{"", bt.Success},
		{"failure", bt.Failure},
		{"running", bt.Running},
	}
	reg := bt.DefaultRegistry()
	factory, _ := reg.Get("stub_action")
	for _, c := range cases {
		body := []byte(`{"name":"probe","result":"` + c.result + `"}`)
		n, err := factory(body)
		if err != nil {
			t.Fatalf("factory err for %q: %v", c.result, err)
		}
		if got := n.Tick(&bt.Context{BB: blackboard.New()}); got != c.want {
			t.Fatalf("stub result=%q: expected %v, got %v", c.result, c.want, got)
		}
	}
}

func TestStubAction_FactoryErrors(t *testing.T) {
	reg := bt.DefaultRegistry()
	factory, _ := reg.Get("stub_action")

	if _, err := factory([]byte(`{bad`)); err == nil {
		t.Fatal("expected error on bad JSON")
	}
	if _, err := factory([]byte(`{"name":""}`)); err == nil {
		t.Fatal("expected error on empty name")
	}
	if _, err := factory([]byte(`{"name":"probe","result":"unknown"}`)); err == nil {
		t.Fatal("expected error on unknown result")
	}
}

// --- compare helper branches via check_bb_float / check_bb_string ---

func checkFloat(t *testing.T, op string, val float64, stored any, want bt.Status) {
	t.Helper()
	body := []byte(`{"type":"check_bb_float","params":{"key":"threat_level","op":"` + op + `","value":` + floatStr(val) + `}}`)
	n, err := bt.BuildFromJSON(body, bt.DefaultRegistry())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	bb := blackboard.New()
	bb.SetRaw("threat_level", stored)
	if got := n.Tick(&bt.Context{BB: bb}); got != want {
		t.Fatalf("op=%s stored=%v val=%v: expected %v, got %v", op, stored, val, want, got)
	}
}

func floatStr(v float64) string {
	// go test compiles this fine; keeps tests dependency-free
	if v == float64(int64(v)) {
		return itoa(int64(v))
	}
	return "0.0"
}

func itoa(i int64) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func TestCompareFloat64_AllOps(t *testing.T) {
	checkFloat(t, "==", 10, 10.0, bt.Success)
	checkFloat(t, "==", 10, 11.0, bt.Failure)
	checkFloat(t, "!=", 10, 11.0, bt.Success)
	checkFloat(t, "!=", 10, 10.0, bt.Failure)
	checkFloat(t, ">", 10, 11.0, bt.Success)
	checkFloat(t, ">", 10, 9.0, bt.Failure)
	checkFloat(t, ">=", 10, 10.0, bt.Success)
	checkFloat(t, "<", 10, 9.0, bt.Success)
	checkFloat(t, "<=", 10, 10.0, bt.Success)
	checkFloat(t, "<=", 10, 11.0, bt.Failure)

	// unknown op → Failure
	body := []byte(`{"type":"check_bb_float","params":{"key":"threat_level","op":"!!!","value":10}}`)
	n, err := bt.BuildFromJSON(body, bt.DefaultRegistry())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	bb := blackboard.New()
	bb.SetRaw("threat_level", 10.0)
	if got := n.Tick(&bt.Context{BB: bb}); got != bt.Failure {
		t.Fatalf("unknown op should Failure, got %v", got)
	}
}

func TestCompareFloat64_AcceptsIntTypes(t *testing.T) {
	// toFloat64 int64 branch
	checkFloat(t, ">=", 10, int64(15), bt.Success)
	// toFloat64 int branch
	checkFloat(t, ">=", 10, int(15), bt.Success)
	// toFloat64 unsupported type
	checkFloat(t, ">=", 10, "not-a-number", bt.Failure)
}

func TestCompareString_Branches(t *testing.T) {
	build := func(op, val string) bt.Node {
		body := []byte(`{"type":"check_bb_string","params":{"key":"last_event_type","op":"` + op + `","value":"` + val + `"}}`)
		n, err := bt.BuildFromJSON(body, bt.DefaultRegistry())
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		return n
	}

	bb := blackboard.New()
	bb.SetRaw("last_event_type", "E01")
	if build("==", "E01").Tick(&bt.Context{BB: bb}) != bt.Success {
		t.Fatal("== equal should Success")
	}
	if build("==", "E02").Tick(&bt.Context{BB: bb}) != bt.Failure {
		t.Fatal("== different should Failure")
	}
	if build("!=", "E02").Tick(&bt.Context{BB: bb}) != bt.Success {
		t.Fatal("!= different should Success")
	}
	if build("!=", "E01").Tick(&bt.Context{BB: bb}) != bt.Failure {
		t.Fatal("!= equal should Failure")
	}
	if build(">=", "E01").Tick(&bt.Context{BB: bb}) != bt.Failure {
		t.Fatal("unknown op should Failure")
	}

	// non-string stored value → Failure
	bb.SetRaw("last_event_type", 42)
	if build("==", "E01").Tick(&bt.Context{BB: bb}) != bt.Failure {
		t.Fatal("non-string stored should Failure")
	}
}
