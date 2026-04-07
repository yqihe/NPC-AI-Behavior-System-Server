package protocol

import "encoding/json"

// --- 消息类型常量 ---

const (
	TypeSpawnNPC      = "spawn_npc"
	TypeRemoveNPC     = "remove_npc"
	TypePublishEvent  = "publish_event"
	TypeQueryNPC      = "query_npc"
	TypeResponse      = "response"
	TypeError         = "error"
	TypeWorldSnapshot = "world_snapshot"
	TypeEnterZone     = "enter_zone"
	TypeLeaveZone     = "leave_zone"
)

// --- 信封 ---

// Message WS 消息信封，所有消息都用此结构包装
type Message struct {
	Type string          `json:"type"`           // 消息类型
	ID   string          `json:"id,omitempty"`   // 请求 ID（客户端生成，响应原样返回）
	Data json.RawMessage `json:"data,omitempty"` // 载荷（延迟解析）
}

// --- 请求载荷 ---

// SpawnNPCRequest 创建 NPC
type SpawnNPCRequest struct {
	NpcID    string  `json:"npc_id"`
	TypeName string  `json:"type_name"`
	X        float64 `json:"x"`
	Z        float64 `json:"z"`
}

// RemoveNPCRequest 移除 NPC
type RemoveNPCRequest struct {
	NpcID string `json:"npc_id"`
}

// PublishEventRequest 发布事件
type PublishEventRequest struct {
	EventType string  `json:"event_type"`
	X         float64 `json:"x"`
	Z         float64 `json:"z"`
	Severity  float64 `json:"severity,omitempty"`  // 可选，0 则用默认值
	SourceID  string  `json:"source_id,omitempty"` // 可选
	ZoneID    string  `json:"zone_id,omitempty"`   // 可选，事件发生的区域 ID
}

// EnterZoneRequest 客户端进入区域
type EnterZoneRequest struct {
	ZoneID string `json:"zone_id"`
}

// QueryNPCRequest 查询 NPC 状态
type QueryNPCRequest struct {
	NpcID string `json:"npc_id"`
}

// --- 响应载荷 ---

// SpawnNPCResponse spawn_npc 成功响应
type SpawnNPCResponse struct {
	NpcID    string `json:"npc_id"`
	TypeName string `json:"type_name"`
}

// RemoveNPCResponse remove_npc 成功响应
type RemoveNPCResponse struct {
	NpcID string `json:"npc_id"`
}

// PublishEventResponse publish_event 成功响应
type PublishEventResponse struct {
	EventID string `json:"event_id"`
}

// QueryNPCResponse query_npc 成功响应
type QueryNPCResponse struct {
	NpcID         string  `json:"npc_id"`
	TypeName      string  `json:"type_name"`
	X             float64 `json:"x"`
	Z             float64 `json:"z"`
	FSMState      string  `json:"fsm_state"`
	CurrentAction string  `json:"current_action"`
	ThreatLevel   float64 `json:"threat_level"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    string `json:"code"`    // 错误码（如 "npc_not_found"）
	Message string `json:"message"` // 人类可读描述
}

// --- 广播载荷 ---

// NPCState 单个 NPC 的状态快照
type NPCState struct {
	NpcID         string  `json:"npc_id"`
	TypeName      string  `json:"type_name"`
	X             float64 `json:"x"`
	Z             float64 `json:"z"`
	FSMState      string  `json:"fsm_state"`
	CurrentAction string  `json:"current_action"`
	ThreatLevel   float64 `json:"threat_level"`
}

// WorldSnapshot 每 Tick 广播的世界状态快照
type WorldSnapshot struct {
	Tick uint64     `json:"tick"`
	NPCs []NPCState `json:"npcs"`
}

// --- 辅助函数 ---

// NewResponse 构建成功响应消息的 JSON 字节
func NewResponse(id string, data any) ([]byte, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Message{
		Type: TypeResponse,
		ID:   id,
		Data: json.RawMessage(dataBytes),
	})
}

// NewError 构建错误响应消息的 JSON 字节
func NewError(id string, code string, msg string) ([]byte, error) {
	dataBytes, err := json.Marshal(ErrorResponse{
		Code:    code,
		Message: msg,
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(Message{
		Type: TypeError,
		ID:   id,
		Data: json.RawMessage(dataBytes),
	})
}
