package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
)

// 确保 HTTPSource 实现 Source 接口
var _ Source = (*HTTPSource)(nil)

// httpConfigItem ADMIN API 返回的配置条目
type httpConfigItem struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

// httpConfigResponse ADMIN API 统一响应格式
type httpConfigResponse struct {
	Items []httpConfigItem `json:"items"`
}

// HTTPSource 从 ADMIN 平台 HTTP API 加载配置，启动时全量拉取到内存
type HTTPSource struct {
	npcTypes     map[string][]byte // name → raw JSON（v2 兼容）
	npcTemplates map[string][]byte // name → raw JSON（v3 组件化）
	fsmConfigs   map[string][]byte // name → raw JSON
	btTrees      map[string][]byte // name → raw JSON
	eventTypes   map[string][]byte // name → raw JSON
}

// NewHTTPSource 从 ADMIN 平台全量拉取配置到内存
func NewHTTPSource(ctx context.Context, baseURL string) (*HTTPSource, error) {
	s := &HTTPSource{
		npcTypes:     make(map[string][]byte),
		npcTemplates: make(map[string][]byte),
		fsmConfigs:   make(map[string][]byte),
		btTrees:      make(map[string][]byte),
		eventTypes:   make(map[string][]byte),
	}

	endpoints := []struct {
		path     string
		target   map[string][]byte
		optional bool // optional 端点失败时不报错（ADMIN 可能尚未实现）
	}{
		{"/api/configs/event_types", s.eventTypes, false},
		{"/api/configs/fsm_configs", s.fsmConfigs, false},
		{"/api/configs/bt_trees", s.btTrees, false},
		{"/api/configs/npc_templates", s.npcTemplates, false},
	}

	client := &http.Client{}

	for _, ep := range endpoints {
		url := baseURL + ep.path
		if err := fetchEndpoint(ctx, client, url, ep.target); err != nil {
			if ep.optional {
				slog.Warn("config.http.optional_endpoint_skipped", "endpoint", ep.path, "err", err)
				continue
			}
			return nil, err
		}
		slog.Info("config.http.loaded", "endpoint", ep.path, "count", len(ep.target))
	}

	return s, nil
}

// fetchEndpoint 从一个 API endpoint 拉取配置到 map
func fetchEndpoint(ctx context.Context, client *http.Client, url string, target map[string][]byte) error {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("config: http create request %s: %w", url, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("config: http request %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("config: http request %s: status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("config: http read %s: %w", url, err)
	}

	var response httpConfigResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("config: http parse %s: %w", url, err)
	}

	for _, item := range response.Items {
		if item.Name == "" {
			continue
		}
		target[item.Name] = item.Config
	}

	if len(target) == 0 {
		return fmt.Errorf("config: http endpoint %s returned empty items", url)
	}

	return nil
}

// --- Source 接口实现（纯内存读取）---

func (s *HTTPSource) LoadFSMConfig(npcType string) (*fsm.FSMConfig, error) {
	data, ok := s.fsmConfigs[npcType]
	if !ok {
		return nil, fmt.Errorf("config: FSM %q not found via ADMIN API", npcType)
	}
	var cfg fsm.FSMConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse FSM %q: %w", npcType, err)
	}
	return &cfg, nil
}

func (s *HTTPSource) LoadBTTree(treeName string) ([]byte, error) {
	data, ok := s.btTrees[treeName]
	if !ok {
		return nil, fmt.Errorf("config: BT tree %q not found via ADMIN API", treeName)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("config: BT tree %q is not valid JSON", treeName)
	}
	return data, nil
}

func (s *HTTPSource) LoadEventConfig(eventType string) ([]byte, error) {
	data, ok := s.eventTypes[eventType]
	if !ok {
		return nil, fmt.Errorf("config: event %q not found via ADMIN API", eventType)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("config: event %q is not valid JSON", eventType)
	}
	return data, nil
}

func (s *HTTPSource) LoadAllEventConfigs() (map[string][]byte, error) {
	result := make(map[string][]byte, len(s.eventTypes))
	for name, data := range s.eventTypes {
		result[name] = data
	}
	return result, nil
}

func (s *HTTPSource) LoadNPCTemplate(name string) ([]byte, error) {
	data, ok := s.npcTemplates[name]
	if !ok {
		return nil, fmt.Errorf("config: NPC template %q not found via ADMIN API", name)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("config: NPC template %q is not valid JSON", name)
	}
	return data, nil
}

func (s *HTTPSource) LoadNPCTypeConfig(npcType string) ([]byte, error) {
	data, ok := s.npcTypes[npcType]
	if !ok {
		return nil, fmt.Errorf("config: NPC type %q not found via ADMIN API", npcType)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("config: NPC type %q is not valid JSON", npcType)
	}
	return data, nil
}

func (s *HTTPSource) LoadAllNPCTemplates() (map[string][]byte, error) {
	result := make(map[string][]byte, len(s.npcTemplates))
	for name, data := range s.npcTemplates {
		result[name] = data
	}
	return result, nil
}

func (s *HTTPSource) LoadRegionConfig(regionID string) ([]byte, error) {
	return nil, fmt.Errorf("config: region loading via HTTP not yet implemented")
}

func (s *HTTPSource) LoadAllRegionConfigs() (map[string][]byte, error) {
	return make(map[string][]byte), nil
}
