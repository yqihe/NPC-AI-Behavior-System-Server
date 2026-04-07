package zone

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/social"
)

// Position 刷怪点坐标
type Position struct {
	X float64 `json:"x"`
	Z float64 `json:"z"`
}

// SpawnEntry 刷怪表条目
type SpawnEntry struct {
	TemplateRef    string     `json:"template_ref"`
	Count          int        `json:"count"`
	SpawnPoints    []Position `json:"spawn_points"`
	WanderRadius   float64    `json:"wander_radius"`
	RespawnSeconds float64    `json:"respawn_seconds"`
}

// Zone 一个游戏区域
type Zone struct {
	ID         string       `json:"region_id"`
	Name       string       `json:"name"`
	RegionType string       `json:"region_type"`
	SpawnTable []SpawnEntry `json:"spawn_table"`
	Active     bool         `json:"-"` // true=活跃, false=休眠
	npcs       []string     // 该区域的 NPC ID 列表
}

// Spawn 从 spawn_table 批量创建 NPC
func (z *Zone) Spawn(compReg *component.Registry, src config.Source, btReg *bt.Registry, npcReg *npc.Registry, gm *social.GroupManager) error {
	for _, entry := range z.SpawnTable {
		if len(entry.SpawnPoints) == 0 {
			slog.Warn("zone.spawn.no_spawn_points", "zone", z.ID, "template", entry.TemplateRef)
			continue
		}
		for i := 0; i < entry.Count; i++ {
			sp := entry.SpawnPoints[i%len(entry.SpawnPoints)]
			pos := event.Vec3{X: sp.X, Z: sp.Z}

			// 加载模板（先新格式，降级旧格式）
			raw, err := src.LoadNPCTemplate(entry.TemplateRef)
			if err != nil {
				raw, err = src.LoadNPCTypeConfig(entry.TemplateRef)
				if err != nil {
					return fmt.Errorf("zone %s: load template %q: %w", z.ID, entry.TemplateRef, err)
				}
			}
			tmpl, err := npc.ParseNPCTemplate(raw)
			if err != nil {
				return fmt.Errorf("zone %s: parse template %q: %w", z.ID, entry.TemplateRef, err)
			}

			// 注入 zone_id 到 position 组件
			if posRaw, ok := tmpl.Components["position"]; ok {
				var posData map[string]any
				if err := json.Unmarshal(posRaw, &posData); err == nil {
					posData["zone_id"] = z.ID
					if newRaw, err := json.Marshal(posData); err == nil {
						tmpl.Components["position"] = newRaw
					}
				}
			}

			id := fmt.Sprintf("%s_%s_%d", z.ID, entry.TemplateRef, i)
			inst, err := npc.NewInstanceFromTemplate(id, pos, tmpl, compReg, src, btReg)
			if err != nil {
				return fmt.Errorf("zone %s: create NPC %q: %w", z.ID, id, err)
			}

			npcReg.Add(inst)
			if gm != nil {
				gm.Register(inst)
			}
			z.npcs = append(z.npcs, id)
		}
	}
	slog.Info("zone.spawned", "zone", z.ID, "npc_count", len(z.npcs))
	return nil
}

// NPCIDs 返回该区域的 NPC ID 列表
func (z *Zone) NPCIDs() []string {
	return z.npcs
}
