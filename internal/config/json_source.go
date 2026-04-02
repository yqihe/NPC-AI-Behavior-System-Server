package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
)

// JSONSource 从 configs/ 目录加载 JSON 配置文件
type JSONSource struct {
	basePath string
}

// NewJSONSource 创建 JSON 配置源
func NewJSONSource(basePath string) *JSONSource {
	return &JSONSource{basePath: basePath}
}

// LoadFSMConfig 加载 FSM 配置：configs/fsm/<npcType>.json
func (s *JSONSource) LoadFSMConfig(npcType string) (*fsm.FSMConfig, error) {
	path := filepath.Join(s.basePath, "fsm", npcType+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: load FSM %q: %w", npcType, err)
	}

	var cfg fsm.FSMConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse FSM %q: %w", npcType, err)
	}

	return &cfg, nil
}

// LoadBTTree 加载 BT 树配置：configs/bt_trees/<treeName>.json
func (s *JSONSource) LoadBTTree(treeName string) ([]byte, error) {
	path := filepath.Join(s.basePath, "bt_trees", treeName+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: load BT tree %q: %w", treeName, err)
	}

	// 验证是合法 JSON
	if !json.Valid(data) {
		return nil, fmt.Errorf("config: BT tree %q is not valid JSON", treeName)
	}

	return data, nil
}
