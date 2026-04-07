package perception

import (
	"log/slog"
	"math"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
)

// PerceptionConfig NPC 类型的感知参数（从 NPC 类型配置加载）
type PerceptionConfig struct {
	VisualRange   float64 `json:"visual_range"`   // 视觉感知距离（米）
	AuditoryRange float64 `json:"auditory_range"` // 听觉感知距离（米）
}

// PerceiveResult 感知结果，携带事件及其感知强度
type PerceiveResult struct {
	Event    *event.Event
	Strength float64 // 0.0~severity，距离衰减后的感知强度
}

// CalcStrength 计算 NPC 对事件的感知强度
// 返回 0 表示不可感知，>0 表示感知强度
// 公式：strength = severity × max(0, 1 - distance / min(npc_range, event_range))
func CalcStrength(npcPos event.Vec3, cfg *PerceptionConfig, evt *event.Event, evtTypeCfg *event.EventTypeConfig) float64 {
	switch evtTypeCfg.PerceptionMode {
	case "global":
		return evt.Severity
	case "visual":
		return calcRangeStrength(npcPos, evt, cfg.VisualRange, evtTypeCfg.Range)
	case "auditory":
		return calcRangeStrength(npcPos, evt, cfg.AuditoryRange, evtTypeCfg.Range)
	default:
		slog.Warn("perception.unknown_mode", "mode", evtTypeCfg.PerceptionMode, "event_type", evtTypeCfg.Name)
		return 0
	}
}

// calcRangeStrength 计算基于距离的感知强度
// dist <= maxRange 返回正值（边界包含，与 v2 一致）
// dist > maxRange 返回 0
func calcRangeStrength(npcPos event.Vec3, evt *event.Event, npcRange, evtRange float64) float64 {
	maxRange := math.Min(npcRange, evtRange)
	if maxRange <= 0 {
		return 0
	}
	dist := event.Distance(npcPos, evt.Position)
	if dist > maxRange {
		return 0
	}
	factor := math.Max(0, 1-dist/maxRange)
	if factor == 0 {
		// dist == maxRange 边界点，给极小正值保持 CanPerceive 兼容
		factor = 1e-6
	}
	return evt.Severity * factor
}

// ShouldFilterByZone 判断是否应因区域不同而过滤事件
// global 事件无视区域；任一 zone_id 为空不过滤（向后兼容）
func ShouldFilterByZone(npcZoneID, evtZoneID, perceptionMode string) bool {
	if perceptionMode == "global" {
		return false
	}
	if npcZoneID == "" || evtZoneID == "" {
		return false
	}
	return npcZoneID != evtZoneID
}

// CanPerceive 判断 NPC 是否能感知到事件（v2 兼容，二值判断）
func CanPerceive(npcPos event.Vec3, cfg *PerceptionConfig, evt *event.Event, evtTypeCfg *event.EventTypeConfig) bool {
	return CalcStrength(npcPos, cfg, evt, evtTypeCfg) > 0
}
