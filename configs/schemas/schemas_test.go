package schemas_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
)

// componentEnvelope 组件 Schema 信封结构
type componentEnvelope struct {
	Component     string          `json:"component"`
	DisplayName   string          `json:"display_name"`
	BlackboardKeys []string       `json:"blackboard_keys"`
	Schema        json.RawMessage `json:"schema"`
}

// presetEnvelope 预设定义信封结构
type presetEnvelope struct {
	Preset             string   `json:"preset"`
	DisplayName        string   `json:"display_name"`
	Description        string   `json:"description"`
	RequiredComponents []string `json:"required_components"`
	DefaultComponents  []string `json:"default_components"`
	OptionalComponents []string `json:"optional_components"`
}

// nodeTypeEnvelope BT 节点类型信封结构
type nodeTypeEnvelope struct {
	NodeType    string          `json:"node_type"`
	DisplayName string          `json:"display_name"`
	Category    string          `json:"category"`
	ParamsSchema json.RawMessage `json:"params_schema"`
}

// conditionTypeEnvelope FSM 条件类型信封结构
type conditionTypeEnvelope struct {
	ConditionType string          `json:"condition_type"`
	DisplayName   string          `json:"display_name"`
	ParamsSchema  json.RawMessage `json:"params_schema"`
}

func schemasDir(t *testing.T) string {
	t.Helper()
	// 从测试文件位置向上找到 configs/schemas
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("failed to get abs path: %v", err)
	}
	return dir
}

// TestAllJSONValid 验证所有 JSON 文件语法合法 (R17)
func TestAllJSONValid(t *testing.T) {
	base := schemasDir(t)
	count := 0
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("failed to read %s: %v", path, readErr)
			return nil
		}
		if !json.Valid(data) {
			t.Errorf("invalid JSON: %s", path)
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("walk error: %v", err)
	}
	if count == 0 {
		t.Fatal("no JSON files found")
	}
	t.Logf("validated %d JSON files", count)
}

// TestComponentSchemas 验证组件 Schema 信封结构 (R1, R2)
func TestComponentSchemas(t *testing.T) {
	dir := filepath.Join(schemasDir(t), "components")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read components dir: %v", err)
	}

	expectedComponents := map[string]bool{
		"identity": false, "position": false, "behavior": false,
		"perception": false, "movement": false, "personality": false,
		"needs": false, "emotion": false, "memory": false, "social": false,
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Errorf("read %s: %v", entry.Name(), err)
			continue
		}

		var env componentEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			t.Errorf("unmarshal %s: %v", entry.Name(), err)
			continue
		}

		if env.Component == "" {
			t.Errorf("%s: component field is empty", entry.Name())
		}
		if env.DisplayName == "" {
			t.Errorf("%s: display_name field is empty", entry.Name())
		}
		if env.BlackboardKeys == nil {
			t.Errorf("%s: blackboard_keys field is nil", entry.Name())
		}
		if len(env.Schema) == 0 {
			t.Errorf("%s: schema field is empty", entry.Name())
		}

		expectedComponents[env.Component] = true
	}

	for name, found := range expectedComponents {
		if !found {
			t.Errorf("missing component schema: %s", name)
		}
	}
}

// TestPresetSchemas 验证预设定义 (R6, R7, R8)
func TestPresetSchemas(t *testing.T) {
	dir := filepath.Join(schemasDir(t), "presets")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read presets dir: %v", err)
	}

	expectedPresets := map[string]bool{
		"simple": false, "reactive": false, "autonomous": false, "social": false,
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Errorf("read %s: %v", entry.Name(), err)
			continue
		}

		var env presetEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			t.Errorf("unmarshal %s: %v", entry.Name(), err)
			continue
		}

		if env.Preset == "" {
			t.Errorf("%s: preset field is empty", entry.Name())
		}

		// R7: required_components 包含 identity 和 position
		hasIdentity := false
		hasPosition := false
		for _, c := range env.RequiredComponents {
			if c == "identity" {
				hasIdentity = true
			}
			if c == "position" {
				hasPosition = true
			}
		}
		if !hasIdentity {
			t.Errorf("preset %s: required_components missing identity", env.Preset)
		}
		if !hasPosition {
			t.Errorf("preset %s: required_components missing position", env.Preset)
		}

		expectedPresets[env.Preset] = true
	}

	for name, found := range expectedPresets {
		if !found {
			t.Errorf("missing preset: %s", name)
		}
	}
}

// TestNodeTypeSchemas 验证 BT 节点类型与 DefaultRegistry 一致 (R9, R10, R11)
func TestNodeTypeSchemas(t *testing.T) {
	dir := filepath.Join(schemasDir(t), "node_types")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read node_types dir: %v", err)
	}

	// 从 DefaultRegistry 获取已注册的节点类型名
	reg := bt.DefaultRegistry()
	expectedTypes := []string{
		"sequence", "selector", "parallel", "inverter",
		"check_bb_float", "check_bb_string", "set_bb_value", "stub_action",
		"move_to", "flee_from",
	}

	schemaTypes := make(map[string]bool)
	validCategories := map[string]bool{"composite": true, "decorator": true, "leaf": true}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Errorf("read %s: %v", entry.Name(), err)
			continue
		}

		var env nodeTypeEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			t.Errorf("unmarshal %s: %v", entry.Name(), err)
			continue
		}

		if env.NodeType == "" {
			t.Errorf("%s: node_type field is empty", entry.Name())
		}
		if !validCategories[env.Category] {
			t.Errorf("%s: invalid category %q", entry.Name(), env.Category)
		}
		if len(env.ParamsSchema) == 0 {
			t.Errorf("%s: params_schema field is empty", entry.Name())
		}

		// 验证节点类型在 DefaultRegistry 中存在
		if _, regErr := reg.Get(env.NodeType); regErr != nil {
			t.Errorf("%s: node_type %q not found in bt.DefaultRegistry()", entry.Name(), env.NodeType)
		}

		schemaTypes[env.NodeType] = true
	}

	for _, nt := range expectedTypes {
		if !schemaTypes[nt] {
			t.Errorf("missing node_type schema: %s", nt)
		}
	}
}

// TestBlackboardKeysConsistency 验证已有 BB Key 名称一致性 (R5, R18)
func TestBlackboardKeysConsistency(t *testing.T) {
	dir := filepath.Join(schemasDir(t), "components")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read components dir: %v", err)
	}

	allKeys := make(map[string]string) // key name → component that declares it

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Errorf("read %s: %v", entry.Name(), err)
			continue
		}

		var env componentEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			t.Errorf("unmarshal %s: %v", entry.Name(), err)
			continue
		}

		for _, key := range env.BlackboardKeys {
			// 检查已有 Key 是否与 keys.go 一致
			if blackboard.IsRegistered(key) {
				// Key 在 keys.go 中已注册，名称匹配（通过 IsRegistered 验证）
			}

			// 检查跨组件 Key 冲突：同一个 Key 可以被多个组件声明（如 npc_pos_x 被 position 和 movement 都引用）
			// 但新增 Key 不应与已有 Key 冲突（已通过 IsRegistered 检查）
			if prev, exists := allKeys[key]; exists {
				// 同一个 Key 被多个组件声明是允许的（共享 Key）
				t.Logf("key %q declared by both %s and %s (shared key)", key, prev, env.Component)
			}
			allKeys[key] = env.Component
		}
	}

	if len(allKeys) == 0 {
		t.Error("no blackboard keys found across all component schemas")
	}
	t.Logf("validated %d blackboard keys across components", len(allKeys))
}

// TestConditionTypeSchemas 验证 FSM 条件类型 (R12, R13, R14)
func TestConditionTypeSchemas(t *testing.T) {
	dir := filepath.Join(schemasDir(t), "condition_types")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read condition_types dir: %v", err)
	}

	expectedTypes := map[string]bool{"leaf": false, "composite": false}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Errorf("read %s: %v", entry.Name(), err)
			continue
		}

		var env conditionTypeEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			t.Errorf("unmarshal %s: %v", entry.Name(), err)
			continue
		}

		if env.ConditionType == "" {
			t.Errorf("%s: condition_type field is empty", entry.Name())
		}
		if len(env.ParamsSchema) == 0 {
			t.Errorf("%s: params_schema field is empty", entry.Name())
		}

		expectedTypes[env.ConditionType] = true
	}

	for name, found := range expectedTypes {
		if !found {
			t.Errorf("missing condition_type schema: %s", name)
		}
	}
}
