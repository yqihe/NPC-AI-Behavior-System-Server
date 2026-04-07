package metrics_test

import (
	"strings"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/metrics"
)

func TestMetrics_RecordAndPrometheus(t *testing.T) {
	m := metrics.New()

	zoneCounts := map[string]int{"meadow": 10, "forest": 5}
	m.RecordTick(0.032, 15, zoneCounts, 20)

	text := m.PrometheusText()

	if !strings.Contains(text, "npc_tick_total 1") {
		t.Error("should contain tick_total 1")
	}
	if !strings.Contains(text, "npc_tick_duration_seconds 0.032") {
		t.Error("should contain tick_duration")
	}
	if !strings.Contains(text, `npc_active_count{zone="meadow"} 10`) {
		t.Error("should contain meadow count")
	}
	if !strings.Contains(text, `npc_active_count{zone="forest"} 5`) {
		t.Error("should contain forest count")
	}
	if !strings.Contains(text, "npc_sleeping_count 20") {
		t.Error("should contain sleeping count")
	}
}

func TestMetrics_MultipleRecords(t *testing.T) {
	m := metrics.New()
	m.RecordTick(0.01, 5, nil, 0)
	m.RecordTick(0.02, 10, nil, 0)

	text := m.PrometheusText()
	if !strings.Contains(text, "npc_tick_total 2") {
		t.Error("should have tick_total 2 after two records")
	}
}
