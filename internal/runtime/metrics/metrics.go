package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Metrics 运行时性能指标
type Metrics struct {
	mu                sync.RWMutex
	TickCount         uint64
	TickDurationLast  float64        // 上一次 Tick 耗时（秒）
	ActiveNPCCount    int
	ZoneActiveCounts  map[string]int // zone_id → 活跃 NPC 数
	ZoneSleepingCount int            // 休眠区域的 NPC 总数
}

// New 创建 Metrics 实例
func New() *Metrics {
	return &Metrics{
		ZoneActiveCounts: make(map[string]int),
	}
}

// RecordTick 更新 Tick 指标
func (m *Metrics) RecordTick(duration float64, activeCount int, zoneCounts map[string]int, sleepingCount int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TickCount++
	m.TickDurationLast = duration
	m.ActiveNPCCount = activeCount
	m.ZoneActiveCounts = zoneCounts
	m.ZoneSleepingCount = sleepingCount
}

// PrometheusText 输出 Prometheus text 格式指标
func (m *Metrics) PrometheusText() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString("# HELP npc_tick_total Total number of ticks\n")
	sb.WriteString("# TYPE npc_tick_total counter\n")
	sb.WriteString(fmt.Sprintf("npc_tick_total %d\n", m.TickCount))

	sb.WriteString("# HELP npc_tick_duration_seconds Last tick duration in seconds\n")
	sb.WriteString("# TYPE npc_tick_duration_seconds gauge\n")
	sb.WriteString(fmt.Sprintf("npc_tick_duration_seconds %f\n", m.TickDurationLast))

	sb.WriteString("# HELP npc_active_count Active NPC count by zone\n")
	sb.WriteString("# TYPE npc_active_count gauge\n")
	if len(m.ZoneActiveCounts) == 0 {
		sb.WriteString(fmt.Sprintf("npc_active_count %d\n", m.ActiveNPCCount))
	} else {
		// 按 zone_id 排序保证输出稳定
		zones := make([]string, 0, len(m.ZoneActiveCounts))
		for z := range m.ZoneActiveCounts {
			zones = append(zones, z)
		}
		sort.Strings(zones)
		for _, z := range zones {
			label := z
			if label == "" {
				label = "global"
			}
			sb.WriteString(fmt.Sprintf("npc_active_count{zone=\"%s\"} %d\n", label, m.ZoneActiveCounts[z]))
		}
	}

	sb.WriteString("# HELP npc_sleeping_count Total sleeping NPCs\n")
	sb.WriteString("# TYPE npc_sleeping_count gauge\n")
	sb.WriteString(fmt.Sprintf("npc_sleeping_count %d\n", m.ZoneSleepingCount))

	return sb.String()
}
