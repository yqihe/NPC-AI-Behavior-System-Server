package component

import (
	"encoding/json"
	"fmt"
)

// SocialComponent 社交组件
type SocialComponent struct {
	GroupID      string `json:"group_id"`
	Faction      string `json:"faction"`
	Role         string `json:"role"`
	FollowTarget string `json:"follow_target"`
}

func (c *SocialComponent) Name() string { return "social" }

// SocialFactory 从 JSON 创建 SocialComponent
func SocialFactory(raw json.RawMessage) (Component, error) {
	var c SocialComponent
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("social: %w", err)
	}
	if c.Role != "" {
		validRoles := map[string]bool{"leader": true, "follower": true}
		if !validRoles[c.Role] {
			return nil, fmt.Errorf("social: unknown role %q", c.Role)
		}
	}
	return &c, nil
}
