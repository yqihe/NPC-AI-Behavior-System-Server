package rule_test

import (
	"encoding/json"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/rule"
)

func setupBB() *blackboard.Blackboard {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 75.0)
	blackboard.Set(bb, blackboard.KeyThreatSource, "player_1")
	blackboard.Set(bb, blackboard.KeyThreatExpireAt, int64(9999))
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(5000))
	blackboard.Set(bb, blackboard.KeyLastEventType, "E01")
	return bb
}

func mustParseCondition(t *testing.T, jsonStr string) rule.Condition {
	t.Helper()
	var c rule.Condition
	if err := json.Unmarshal([]byte(jsonStr), &c); err != nil {
		t.Fatalf("failed to parse condition JSON: %v", err)
	}
	return c
}

// --- 基础操作符测试 ---

func TestEvaluate_Equals(t *testing.T) {
	bb := setupBB()
	c := mustParseCondition(t, `{"key": "threat_level", "op": "==", "value": 75.0}`)
	if !c.Evaluate(bb) {
		t.Fatal("expected true for threat_level == 75.0")
	}

	c2 := mustParseCondition(t, `{"key": "threat_level", "op": "==", "value": 50.0}`)
	if c2.Evaluate(bb) {
		t.Fatal("expected false for threat_level == 50.0")
	}
}

func TestEvaluate_NotEquals(t *testing.T) {
	bb := setupBB()
	c := mustParseCondition(t, `{"key": "threat_level", "op": "!=", "value": 50.0}`)
	if !c.Evaluate(bb) {
		t.Fatal("expected true for threat_level != 50.0")
	}
}

func TestEvaluate_GreaterThan(t *testing.T) {
	bb := setupBB()
	c := mustParseCondition(t, `{"key": "threat_level", "op": ">", "value": 50.0}`)
	if !c.Evaluate(bb) {
		t.Fatal("expected true for threat_level > 50.0")
	}

	c2 := mustParseCondition(t, `{"key": "threat_level", "op": ">", "value": 75.0}`)
	if c2.Evaluate(bb) {
		t.Fatal("expected false for threat_level > 75.0")
	}
}

func TestEvaluate_GreaterEqual(t *testing.T) {
	bb := setupBB()
	c := mustParseCondition(t, `{"key": "threat_level", "op": ">=", "value": 75.0}`)
	if !c.Evaluate(bb) {
		t.Fatal("expected true for threat_level >= 75.0")
	}
}

func TestEvaluate_LessThan(t *testing.T) {
	bb := setupBB()
	c := mustParseCondition(t, `{"key": "threat_level", "op": "<", "value": 100.0}`)
	if !c.Evaluate(bb) {
		t.Fatal("expected true for threat_level < 100.0")
	}
}

func TestEvaluate_LessEqual(t *testing.T) {
	bb := setupBB()
	c := mustParseCondition(t, `{"key": "threat_level", "op": "<=", "value": 75.0}`)
	if !c.Evaluate(bb) {
		t.Fatal("expected true for threat_level <= 75.0")
	}
}

func TestEvaluate_StringEquals(t *testing.T) {
	bb := setupBB()
	c := mustParseCondition(t, `{"key": "last_event_type", "op": "==", "value": "E01"}`)
	if !c.Evaluate(bb) {
		t.Fatal("expected true for last_event_type == E01")
	}

	c2 := mustParseCondition(t, `{"key": "last_event_type", "op": "==", "value": "E99"}`)
	if c2.Evaluate(bb) {
		t.Fatal("expected false for last_event_type == E99")
	}
}

func TestEvaluate_StringNotEquals(t *testing.T) {
	bb := setupBB()
	c := mustParseCondition(t, `{"key": "last_event_type", "op": "!=", "value": ""}`)
	if !c.Evaluate(bb) {
		t.Fatal("expected true for last_event_type != empty string")
	}
}

func TestEvaluate_In(t *testing.T) {
	bb := setupBB()
	c := mustParseCondition(t, `{"key": "last_event_type", "op": "in", "value": ["E01","E02","E03"]}`)
	if !c.Evaluate(bb) {
		t.Fatal("expected true for last_event_type in [E01,E02,E03]")
	}

	c2 := mustParseCondition(t, `{"key": "last_event_type", "op": "in", "value": ["E04","E05"]}`)
	if c2.Evaluate(bb) {
		t.Fatal("expected false for last_event_type in [E04,E05]")
	}
}

func TestEvaluate_InNumeric(t *testing.T) {
	bb := setupBB()
	c := mustParseCondition(t, `{"key": "threat_level", "op": "in", "value": [25.0, 50.0, 75.0]}`)
	if !c.Evaluate(bb) {
		t.Fatal("expected true for threat_level in [25, 50, 75]")
	}
}

// --- RefKey 测试 ---

func TestEvaluate_RefKey(t *testing.T) {
	bb := setupBB()
	// threat_expire_at(9999) > current_time(5000) → true
	c := mustParseCondition(t, `{"key": "threat_expire_at", "op": ">", "ref_key": "current_time"}`)
	if !c.Evaluate(bb) {
		t.Fatal("expected true for threat_expire_at > current_time")
	}

	// current_time(5000) > threat_expire_at(9999) → false
	c2 := mustParseCondition(t, `{"key": "current_time", "op": ">", "ref_key": "threat_expire_at"}`)
	if c2.Evaluate(bb) {
		t.Fatal("expected false for current_time > threat_expire_at")
	}
}

// --- AND/OR 组合测试 ---

func TestEvaluate_And(t *testing.T) {
	bb := setupBB()
	c := mustParseCondition(t, `{
		"and": [
			{"key": "threat_level", "op": ">=", "value": 50},
			{"key": "threat_expire_at", "op": ">", "ref_key": "current_time"}
		]
	}`)
	if !c.Evaluate(bb) {
		t.Fatal("expected true for AND(threat >= 50, expire > now)")
	}
}

func TestEvaluate_AndPartialFail(t *testing.T) {
	bb := setupBB()
	c := mustParseCondition(t, `{
		"and": [
			{"key": "threat_level", "op": ">=", "value": 50},
			{"key": "threat_level", "op": "<", "value": 10}
		]
	}`)
	if c.Evaluate(bb) {
		t.Fatal("expected false for AND(>= 50, < 10)")
	}
}

func TestEvaluate_Or(t *testing.T) {
	bb := setupBB()
	c := mustParseCondition(t, `{
		"or": [
			{"key": "threat_level", "op": ">=", "value": 90},
			{"key": "last_event_type", "op": "==", "value": "E01"}
		]
	}`)
	if !c.Evaluate(bb) {
		t.Fatal("expected true for OR(>= 90, == E01)")
	}
}

func TestEvaluate_OrAllFail(t *testing.T) {
	bb := setupBB()
	c := mustParseCondition(t, `{
		"or": [
			{"key": "threat_level", "op": ">=", "value": 90},
			{"key": "last_event_type", "op": "==", "value": "E99"}
		]
	}`)
	if c.Evaluate(bb) {
		t.Fatal("expected false for OR(>= 90, == E99)")
	}
}

func TestEvaluate_NestedAndOr(t *testing.T) {
	bb := setupBB()
	// (threat >= 50 AND (event == E01 OR event == E02))
	c := mustParseCondition(t, `{
		"and": [
			{"key": "threat_level", "op": ">=", "value": 50},
			{
				"or": [
					{"key": "last_event_type", "op": "==", "value": "E01"},
					{"key": "last_event_type", "op": "==", "value": "E02"}
				]
			}
		]
	}`)
	if !c.Evaluate(bb) {
		t.Fatal("expected true for nested AND+OR")
	}
}

// --- 边界条件 ---

func TestEvaluate_EmptyCondition(t *testing.T) {
	bb := setupBB()
	c := rule.Condition{}
	if !c.Evaluate(bb) {
		t.Fatal("expected empty condition to return true")
	}
}

func TestEvaluate_MissingBBKey(t *testing.T) {
	bb := blackboard.New() // 空 BB
	c := mustParseCondition(t, `{"key": "threat_level", "op": ">=", "value": 50}`)
	if c.Evaluate(bb) {
		t.Fatal("expected false when BB key is missing")
	}
}

func TestEvaluate_MissingRefKey(t *testing.T) {
	bb := setupBB()
	blackboard.Delete(bb, blackboard.KeyCurrentTime)
	c := mustParseCondition(t, `{"key": "threat_expire_at", "op": ">", "ref_key": "current_time"}`)
	if c.Evaluate(bb) {
		t.Fatal("expected false when ref_key value is missing from BB")
	}
}

// --- Validate 测试 ---

func TestValidate_ValidCondition(t *testing.T) {
	c := mustParseCondition(t, `{
		"and": [
			{"key": "threat_level", "op": ">=", "value": 50},
			{"key": "threat_expire_at", "op": ">", "ref_key": "current_time"}
		]
	}`)
	if err := c.Validate(); err != nil {
		t.Fatalf("expected valid condition, got: %v", err)
	}
}

func TestValidate_UnknownKey(t *testing.T) {
	c := mustParseCondition(t, `{"key": "nonexistent_key_xyz", "op": "==", "value": 1}`)
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestValidate_UnknownRefKey(t *testing.T) {
	c := mustParseCondition(t, `{"key": "threat_level", "op": ">", "ref_key": "nonexistent_ref_xyz"}`)
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for unknown ref_key")
	}
}

func TestValidate_UnknownOp(t *testing.T) {
	c := mustParseCondition(t, `{"key": "threat_level", "op": "~=", "value": 50}`)
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for unknown operator")
	}
}

func TestValidate_EmptyCondition(t *testing.T) {
	c := rule.Condition{}
	if err := c.Validate(); err != nil {
		t.Fatalf("expected empty condition to be valid, got: %v", err)
	}
}
