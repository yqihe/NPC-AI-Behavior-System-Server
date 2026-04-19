package runtime_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
)

// TestAdminOrphanField_HPTransparentPassthrough R14 验收：
// 锚 ADMIN snapshot §4 guard_basic.fields={hp:100} 用例。
// 断言：
//  1. hp 字段不阻塞 spawn（NewInstanceFromADMIN 不返回 error）
//  2. hp 通过 SetDynamic 写入 BB，GetRaw 可读回 100
//  3. 翻译层对 hp 字段不产生 WARN 日志（R7 透明透传不是静默降级）
//
// FSMRef 使用 civilian（本仓保留，R10）——guard 本尊 FSM 已在 T7 删除；
// 本测试不验证 FSM 具体行为，仅验证 hp 孤儿字段透传路径。
func TestAdminOrphanField_HPTransparentPassthrough(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	// 捕获 WARN+ 级别日志；测试结束后恢复默认 logger
	var logBuf bytes.Buffer
	prevDefault := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(prevDefault)

	tmpl := &npc.ADMINTemplate{
		Name:        "guard_basic",
		TemplateRef: "tpl_guard",
		Fields: map[string]any{
			"hp": 100.0, // 孤儿字段：ADMIN T9 治理遗留（应为 max_hp），服务端透明透传
		},
		Behavior: npc.ADMINBehavior{
			FSMRef: "civilian",
			BTRefs: map[string]string{"Idle": "civilian/idle"},
		},
	}

	inst, err := npc.NewInstanceFromADMIN("guard_1", event.Vec3{}, tmpl, src, btReg, compReg)
	if err != nil {
		t.Fatalf("hp 孤儿字段不应阻塞 spawn: %v", err)
	}

	hp, ok := inst.BB.GetRaw("hp")
	if !ok {
		t.Fatal("hp should be in BB via SetDynamic (R7 透传)")
	}
	if hp != 100.0 {
		t.Errorf("hp = %v, want 100.0", hp)
	}

	if strings.Contains(logBuf.String(), "level=WARN") {
		t.Errorf("unexpected WARN during hp orphan passthrough:\n%s", logBuf.String())
	}
}

// TestAdminOrphanField_MultipleOrphansCoexist 扩展场景：
// 多个孤儿字段共存（hp + loot_table + is_boss），每个都应独立透传不干扰。
func TestAdminOrphanField_MultipleOrphansCoexist(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	tmpl := &npc.ADMINTemplate{
		Name:        "wolf_alpha_like",
		TemplateRef: "reactive_carnivore",
		Fields: map[string]any{
			"hp":         120.0,
			"loot_table": "wolf_alpha_loot",
			"is_boss":    true,
			"max_hp":     800.0,
		},
		Behavior: npc.ADMINBehavior{
			FSMRef: "civilian",
			BTRefs: map[string]string{"Idle": "civilian/idle"},
		},
	}

	inst, err := npc.NewInstanceFromADMIN("alpha_1", event.Vec3{}, tmpl, src, btReg, compReg)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	cases := []struct {
		key  string
		want any
	}{
		{"hp", 120.0},
		{"loot_table", "wolf_alpha_loot"},
		{"is_boss", true},
		{"max_hp", 800.0},
	}
	for _, c := range cases {
		got, ok := inst.BB.GetRaw(c.key)
		if !ok {
			t.Errorf("field %q missing from BB", c.key)
			continue
		}
		if got != c.want {
			t.Errorf("field %q: got %v, want %v", c.key, got, c.want)
		}
	}
}
