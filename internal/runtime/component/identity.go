package component

import (
	"encoding/json"
	"fmt"
)

// IdentityComponent 身份组件，所有 NPC 必有
type IdentityComponent struct {
	DisplayName string   `json:"name"`
	ModelID     string   `json:"model_id"`
	Tags        []string `json:"tags"`
}

func (c *IdentityComponent) Name() string { return "identity" }

// IdentityFactory 从 JSON 创建 IdentityComponent
func IdentityFactory(raw json.RawMessage) (Component, error) {
	var c IdentityComponent
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("identity: %w", err)
	}
	if c.DisplayName == "" {
		return nil, fmt.Errorf("identity: name is required")
	}
	if c.ModelID == "" {
		return nil, fmt.Errorf("identity: model_id is required")
	}
	if c.Tags == nil {
		c.Tags = make([]string, 0)
	}
	return &c, nil
}
