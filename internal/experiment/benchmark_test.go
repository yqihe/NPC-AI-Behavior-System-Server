//go:build experiment

package experiment_test

import (
	"fmt"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/experiment"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/experiment/modes"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
)

var scaleBehaviors = []int{10, 50, 100, 200}
var scaleNPCs = []int{100, 500, 1000, 5000}

func benchSetup() ([]*event.Event, map[string]*event.EventTypeConfig) {
	evtType := &event.EventTypeConfig{Name: "test", DefaultSeverity: 80, DefaultTTL: 15, PerceptionMode: "auditory", Range: 500}
	evtTypes := map[string]*event.EventTypeConfig{"test": evtType}
	events := []*event.Event{
		event.NewEvent(evtType, event.Vec3{X: 50}, "bomb_0", 80, ""),
		event.NewEvent(evtType, event.Vec3{X: 150}, "bomb_1", 60, ""),
	}
	return events, evtTypes
}

// --- 图 2 + 图 5: 吞吐量 + 内存 ---

func BenchmarkScale_Hybrid(b *testing.B) {
	btReg := bt.DefaultRegistry()
	events, evtTypes := benchSetup()
	for _, nb := range scaleBehaviors {
		cfg := experiment.GenerateScaleConfig(nb)
		for _, nn := range scaleNPCs {
			b.Run(fmt.Sprintf("%dB_%dN", nb, nn), func(b *testing.B) {
				npcs := make([]experiment.ExperimentNPC, nn)
				for i := 0; i < nn; i++ {
					npc, err := modes.NewHybridFromScale(fmt.Sprintf("h_%d", i), cfg, btReg)
					if err != nil {
						b.Fatal(err)
					}
					npcs[i] = npc
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					for _, npc := range npcs {
						blackboard.Set(npc.BB(), blackboard.KeyCurrentTime, int64(i)*100)
						npc.Tick(events, evtTypes, 0.1)
					}
				}
			})
		}
	}
}

func BenchmarkScale_PureFSM(b *testing.B) {
	events, evtTypes := benchSetup()
	for _, nb := range scaleBehaviors {
		cfg := experiment.GenerateScaleConfig(nb)
		for _, nn := range scaleNPCs {
			b.Run(fmt.Sprintf("%dB_%dN", nb, nn), func(b *testing.B) {
				npcs := make([]experiment.ExperimentNPC, nn)
				for i := 0; i < nn; i++ {
					npc, err := modes.NewPureFSMFromScale(fmt.Sprintf("f_%d", i), cfg)
					if err != nil {
						b.Fatal(err)
					}
					npcs[i] = npc
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					for _, npc := range npcs {
						blackboard.Set(npc.BB(), blackboard.KeyCurrentTime, int64(i)*100)
						npc.Tick(events, evtTypes, 0.1)
					}
				}
			})
		}
	}
}

func BenchmarkScale_PureBT(b *testing.B) {
	btReg := bt.DefaultRegistry()
	events, evtTypes := benchSetup()
	for _, nb := range scaleBehaviors {
		cfg := experiment.GenerateScaleConfig(nb)
		for _, nn := range scaleNPCs {
			b.Run(fmt.Sprintf("%dB_%dN", nb, nn), func(b *testing.B) {
				npcs := make([]experiment.ExperimentNPC, nn)
				for i := 0; i < nn; i++ {
					npc, err := modes.NewPureBTFromScale(fmt.Sprintf("b_%d", i), cfg, btReg)
					if err != nil {
						b.Fatal(err)
					}
					npcs[i] = npc
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					for _, npc := range npcs {
						blackboard.Set(npc.BB(), blackboard.KeyCurrentTime, int64(i)*100)
						npc.Tick(events, evtTypes, 0.1)
					}
				}
			})
		}
	}
}
