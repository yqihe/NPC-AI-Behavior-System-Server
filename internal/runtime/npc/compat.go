package npc

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// ParseNPCTemplate 自动检测配置格式并解析为 TemplateConfig
// 新格式：含 "components" 字段
// 旧格式（v2）：含 "type_name" 字段，自动转换
func ParseNPCTemplate(data []byte) (*TemplateConfig, error) {
	var probe struct {
		Components json.RawMessage `json:"components"`
		TypeName   string          `json:"type_name"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("npc: parse template probe: %w", err)
	}

	if len(probe.Components) > 0 && probe.Components[0] == '{' {
		return parseNewFormat(data)
	}
	if probe.TypeName != "" {
		slog.Debug("npc.compat.v2_format_detected", "type_name", probe.TypeName)
		return convertV2Format(data)
	}
	return nil, fmt.Errorf("npc: unrecognized config format (no 'components' or 'type_name' field)")
}

// parseNewFormat 解析组件化格式
func parseNewFormat(data []byte) (*TemplateConfig, error) {
	var tmpl TemplateConfig
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("npc: parse new format: %w", err)
	}
	if tmpl.Name == "" {
		return nil, fmt.Errorf("npc: template name is required")
	}
	if len(tmpl.Components) == 0 {
		return nil, fmt.Errorf("npc: template must have at least one component")
	}
	return &tmpl, nil
}

// convertV2Format 将 v2 NPCTypeConfig 转为组件化 TemplateConfig
func convertV2Format(data []byte) (*TemplateConfig, error) {
	var old NPCTypeConfig
	if err := json.Unmarshal(data, &old); err != nil {
		return nil, fmt.Errorf("npc: parse v2 format: %w", err)
	}

	components := make(map[string]json.RawMessage)

	// identity: 用 type_name 作为 name 和 model_id
	identityJSON, _ := json.Marshal(map[string]any{
		"name":     old.TypeName,
		"model_id": old.TypeName,
		"tags":     []string{},
	})
	components["identity"] = identityJSON

	// position: 零值（spawn 时由 pos 参数覆盖）
	positionJSON, _ := json.Marshal(map[string]any{
		"x": 0, "y": 0, "z": 0, "orientation": 0, "zone_id": "",
	})
	components["position"] = positionJSON

	// behavior
	behaviorJSON, _ := json.Marshal(map[string]any{
		"fsm_ref": old.FSMRef,
		"bt_refs": old.BTRefs,
	})
	components["behavior"] = behaviorJSON

	// perception
	perceptionJSON, _ := json.Marshal(map[string]any{
		"visual_range":       old.Perception.VisualRange,
		"auditory_range":     old.Perception.AuditoryRange,
		"attention_capacity": 5,
	})
	components["perception"] = perceptionJSON

	slog.Debug("npc.compat.v2_converted",
		"type_name", old.TypeName,
		"components", len(components),
	)

	return &TemplateConfig{
		Name:       old.TypeName,
		Preset:     "reactive",
		Components: components,
	}, nil
}
