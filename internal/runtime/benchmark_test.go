package runtime_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/decision"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
)

// BenchmarkTick_100NPCs 验证 R14：100 NPC 单 Tick < 10ms
func BenchmarkTick_100NPCs(b *testing.B) {
	src := config.NewJSONSource(benchConfigsDir(b))
	btReg := bt.DefaultRegistry()
	evtTypes := benchLoadEvtTypes(b, src)

	bus := event.NewBus()
	reg := npc.NewRegistry()
	center := decision.NewCenter(10.0)

	// 创建 100 个 NPC
	for i := 0; i < 100; i++ {
		pos := event.Vec3{X: float64(i * 10), Z: float64(i * 10)}
		inst := benchCreateCivilian(b, fmt.Sprintf("npc_%d", i), pos, src, btReg)
		reg.Add(inst)
	}

	// 发布一些事件
	explosionCfg := evtTypes["explosion"]
	for i := 0; i < 5; i++ {
		evt := event.NewEvent(explosionCfg, event.Vec3{X: float64(i * 100), Z: float64(i * 100)}, fmt.Sprintf("bomb_%d", i), 80)
		bus.Publish(evt)
	}

	scheduler := runtime.NewScheduler(bus, reg, center, evtTypes, 100*time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.Tick(0.1)
	}
}

// TestTick_100NPCs_Under10ms 显式验证 R14 的延迟要求
func TestTick_100NPCs_Under10ms(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	evtTypes := loadEvtTypes(t, src)

	bus := event.NewBus()
	reg := npc.NewRegistry()
	center := decision.NewCenter(10.0)

	for i := 0; i < 100; i++ {
		pos := event.Vec3{X: float64(i * 10), Z: float64(i * 10)}
		inst := createCivilian(t, fmt.Sprintf("npc_%d", i), pos, src, btReg)
		reg.Add(inst)
	}

	explosionCfg := evtTypes["explosion"]
	for i := 0; i < 5; i++ {
		evt := event.NewEvent(explosionCfg, event.Vec3{X: float64(i * 100), Z: float64(i * 100)}, fmt.Sprintf("bomb_%d", i), 80)
		bus.Publish(evt)
	}

	scheduler := runtime.NewScheduler(bus, reg, center, evtTypes, 100*time.Millisecond)

	// 跑 10 次 Tick，取最大延迟
	var maxDuration time.Duration
	for i := 0; i < 10; i++ {
		start := time.Now()
		scheduler.Tick(0.1)
		d := time.Since(start)
		if d > maxDuration {
			maxDuration = d
		}
	}

	if maxDuration > 10*time.Millisecond {
		t.Errorf("single Tick exceeded 10ms: max=%v", maxDuration)
	} else {
		t.Logf("100 NPC Tick max duration: %v", maxDuration)
	}
}

// --- benchmark helpers ---

func benchConfigsDir(b *testing.B) string {
	b.Helper()
	return configsDirPath()
}

func configsDirPath() string {
	// internal/runtime/ → 项目根目录
	return "../../configs"
}

func benchLoadEvtTypes(b *testing.B, src config.Source) map[string]*event.EventTypeConfig {
	b.Helper()
	rawConfigs, err := src.LoadAllEventConfigs()
	if err != nil {
		b.Fatal(err)
	}
	evtTypes := make(map[string]*event.EventTypeConfig)
	for _, data := range rawConfigs {
		var cfg event.EventTypeConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			b.Fatal(err)
		}
		evtTypes[cfg.Name] = &cfg
	}
	return evtTypes
}

func benchCreateCivilian(b *testing.B, id string, pos event.Vec3, src config.Source, btReg *bt.Registry) *npc.Instance {
	b.Helper()
	rawCfg, err := src.LoadNPCTypeConfig("civilian")
	if err != nil {
		b.Fatal(err)
	}
	typeCfg, err := npc.ParseNPCTypeConfig(rawCfg)
	if err != nil {
		b.Fatal(err)
	}
	inst, err := npc.NewInstance(id, pos, typeCfg, src, btReg)
	if err != nil {
		b.Fatal(err)
	}
	return inst
}
