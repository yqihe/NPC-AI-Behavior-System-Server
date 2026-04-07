package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
)

// safePath 校验路径组件不包含目录穿越
func safePath(name string) error {
	if strings.Contains(name, "..") {
		return fmt.Errorf("config: path traversal rejected: %q", name)
	}
	return nil
}

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
	if err := safePath(npcType); err != nil {
		return nil, err
	}
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
	if err := safePath(treeName); err != nil {
		return nil, err
	}
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

// LoadEventConfig 加载事件类型配置：configs/events/<eventType>.json
func (s *JSONSource) LoadEventConfig(eventType string) ([]byte, error) {
	if err := safePath(eventType); err != nil {
		return nil, err
	}
	path := filepath.Join(s.basePath, "events", eventType+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: load event %q: %w", eventType, err)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("config: event %q is not valid JSON", eventType)
	}
	return data, nil
}

// LoadAllEventConfigs 遍历 configs/events/ 目录，加载所有事件类型配置
func (s *JSONSource) LoadAllEventConfigs() (map[string][]byte, error) {
	dir := filepath.Join(s.basePath, "events")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("config: read events dir: %w", err)
	}

	result := make(map[string][]byte)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-5] // 去掉 .json 后缀
		data, err := s.LoadEventConfig(name)
		if err != nil {
			return nil, err
		}
		result[name] = data
	}
	return result, nil
}

// LoadNPCTemplate 加载组件化 NPC 模板：configs/npc_templates/<name>.json
func (s *JSONSource) LoadNPCTemplate(name string) ([]byte, error) {
	if err := safePath(name); err != nil {
		return nil, err
	}
	path := filepath.Join(s.basePath, "npc_templates", name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: load NPC template %q: %w", name, err)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("config: NPC template %q is not valid JSON", name)
	}
	return data, nil
}

// LoadNPCTypeConfig 加载 NPC 类型配置：configs/npc_types/<npcType>.json（v2 兼容）
func (s *JSONSource) LoadNPCTypeConfig(npcType string) ([]byte, error) {
	if err := safePath(npcType); err != nil {
		return nil, err
	}
	path := filepath.Join(s.basePath, "npc_types", npcType+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: load NPC type %q: %w", npcType, err)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("config: NPC type %q is not valid JSON", npcType)
	}
	return data, nil
}
