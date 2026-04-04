package perception

import (
	"log/slog"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
)

// PerceptionConfig NPC 类型的感知参数（从 NPC 类型配置加载）
type PerceptionConfig struct {
	VisualRange   float64 `json:"visual_range"`   // 视觉感知距离（米）
	AuditoryRange float64 `json:"auditory_range"` // 听觉感知距离（米）
}

// CanPerceive 判断 NPC 是否能感知到事件
// 纯函数，无状态，所有输入来自配置
func CanPerceive(npcPos event.Vec3, cfg *PerceptionConfig, evt *event.Event, evtTypeCfg *event.EventTypeConfig) bool {
	switch evtTypeCfg.PerceptionMode {
	case "global":
		return true
	case "visual":
		dist := event.Distance(npcPos, evt.Position)
		maxRange := min(cfg.VisualRange, evtTypeCfg.Range)
		return dist <= maxRange
	case "auditory":
		dist := event.Distance(npcPos, evt.Position)
		maxRange := min(cfg.AuditoryRange, evtTypeCfg.Range)
		return dist <= maxRange
	default:
		slog.Warn("perception.unknown_mode", "mode", evtTypeCfg.PerceptionMode, "event_type", evtTypeCfg.Name)
		return false
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
