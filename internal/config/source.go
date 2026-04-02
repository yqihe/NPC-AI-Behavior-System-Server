package config

import (
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
)

// Source 配置数据源接口
type Source interface {
	LoadFSMConfig(npcType string) (*fsm.FSMConfig, error)
	LoadBTTree(treeName string) ([]byte, error) // 返回原始 JSON，由调用方用 bt.BuildFromJSON 构建
}

// 确保 JSONSource 实现 Source 接口
var _ Source = (*JSONSource)(nil)
