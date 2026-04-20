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
	regions      map[string][]byte // region_id → raw JSON（config body，含 region_id 冗余字段）
}

// NewHTTPSource 从 ADMIN 平台全量拉取配置到内存
func NewHTTPSource(ctx context.Context, baseURL string) (*HTTPSource, error) {
	s := &HTTPSource{
		npcTypes:     make(map[string][]byte),
		npcTemplates: make(map[string][]byte),
		fsmConfigs:   make(map[string][]byte),
		btTrees:      make(map[string][]byte),
		eventTypes:   make(map[string][]byte),
		regions:      make(map[string][]byte),
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

	// regions 端点单独处理：空 items[] 合法（与 JSONSource 目录不存在行为一致），
	// 且 500+业务码 47011 需提取 details[] 悬空引用 fail-fast（见 memory: Admin regions 端点契约）
	regionsPath := "/api/configs/regions"
	if err := fetchRegionsEndpoint(ctx, client, baseURL+regionsPath, s.regions); err != nil {
		return nil, err
	}
	slog.Info("config.http.loaded", "endpoint", regionsPath, "count", len(s.regions))

	return s, nil
}

// regionsErrorBody regions 端点业务错误响应（500 + code 47011）
type regionsErrorBody struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Details []regionsDangling `json:"details"`
}

// regionsDangling regions 端点 details[] 条目。
// ADMIN 侧复用 NpcExportDanglingRef 类型未新建独立类型，字段名保留 npc_name —
// 但 regions 端点语境下它实际承载 region_id。打日志时重命名以免运维看岔。
type regionsDangling struct {
	RegionID string `json:"npc_name"`
	RefType  string `json:"ref_type"`
	RefValue string `json:"ref_value"`
	Reason   string `json:"reason"`
}

// fetchRegionsEndpoint regions 专用拉取：空 items[] 合法；500+47011 解码悬空 details[] 后 fail-fast。
func fetchRegionsEndpoint(ctx context.Context, client *http.Client, url string, target map[string][]byte) error {
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("config: http read %s: %w", url, err)
	}

	if resp.StatusCode == http.StatusInternalServerError {
		var errBody regionsErrorBody
		if jerr := json.Unmarshal(body, &errBody); jerr == nil && errBody.Code == 47011 {
			for _, d := range errBody.Details {
				slog.Error("config.http.regions.dangling",
					"region_id", d.RegionID,
					"ref_type", d.RefType,
					"ref_value", d.RefValue,
					"reason", d.Reason,
				)
			}
			return fmt.Errorf("config: regions export dangling refs (code=47011, count=%d): %s",
				len(errBody.Details), errBody.Message)
		}
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("config: http request %s: status %d body=%s", url, resp.StatusCode, string(body))
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
	return nil
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
	data, ok := s.regions[regionID]
	if !ok {
		return nil, fmt.Errorf("config: region %q not found", regionID)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("config: region %q is not valid JSON", regionID)
	}
	return data, nil
}

func (s *HTTPSource) LoadAllRegionConfigs() (map[string][]byte, error) {
	result := make(map[string][]byte, len(s.regions))
	for name, data := range s.regions {
		result[name] = data
	}
	return result, nil
}
