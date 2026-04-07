package component_test

import (
	"encoding/json"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
)

func TestDefaultRegistry_AllComponentsRegistered(t *testing.T) {
	reg := component.DefaultRegistry()

	tests := []struct {
		name string
		raw  string
	}{
		{"identity", `{"name":"test","model_id":"m1"}`},
		{"position", `{"x":0,"z":0}`},
		{"behavior", `{"fsm_ref":"test","bt_refs":{"Idle":"test/idle"}}`},
		{"perception", `{"visual_range":100,"auditory_range":200}`},
		{"movement", `{"move_type":"wander","move_speed":3,"wander_radius":50}`},
		{"personality", `{"personality_type":"docile","decision_weights":{"threat":0.5,"needs":0.3,"emotion":0.2}}`},
		{"needs", `{"need_types":[{"name":"hunger","max":100,"decay_rate":5}]}`},
		{"emotion", `{"emotion_states":[{"name":"fear","value":0,"accumulate_rate":10,"decay_rate":5}]}`},
		{"memory", `{"capacity":10,"memory_types":["threat"],"decay_time":60}`},
		{"social", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp, err := reg.Create(tt.name, json.RawMessage(tt.raw))
			if err != nil {
				t.Fatalf("Create(%q) failed: %v", tt.name, err)
			}
			if comp.Name() != tt.name {
				t.Errorf("Name() = %q, want %q", comp.Name(), tt.name)
			}
		})
	}
}
