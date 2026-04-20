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
// Role 为自由 string，group_manager 只对 "leader"/"follower" 触发队形逻辑，其他取值表示"在 group 中但无队形行为"
func SocialFactory(raw json.RawMessage) (Component, error) {
	var c SocialComponent
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("social: %w", err)
	}
	return &c, nil
}
