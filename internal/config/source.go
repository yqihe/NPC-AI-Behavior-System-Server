package config

import (
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
)

// Source 配置数据源接口
type Source interface {
	LoadFSMConfig(npcType string) (*fsm.FSMConfig, error)
	LoadBTTree(treeName string) ([]byte, error)          // 返回原始 JSON，由调用方用 bt.BuildFromJSON 构建
	LoadEventConfig(eventType string) ([]byte, error)    // 返回原始 JSON，调用方 Unmarshal 为 event.EventTypeConfig
	LoadAllEventConfigs() (map[string][]byte, error)     // 返回所有事件类型配置：name → raw JSON
	LoadNPCTypeConfig(npcType string) ([]byte, error)    // 返回原始 JSON，调用方 Unmarshal 为 npc.NPCTypeConfig
}

// 确保 HTTPSource 实现 Source 接口
var _ Source = (*HTTPSource)(nil)
