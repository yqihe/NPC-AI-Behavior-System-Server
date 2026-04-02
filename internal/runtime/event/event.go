package event

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
)

// Vec3 三维坐标
type Vec3 struct {
	X, Y, Z float64
}

// Distance 计算两点之间的 XZ 平面距离（忽略 Y）
func Distance(a, b Vec3) float64 {
	dx := a.X - b.X
	dz := a.Z - b.Z
	return math.Sqrt(dx*dx + dz*dz)
}

// --- 事件类型配置 ---

// EventTypeConfig 事件类型的配置定义（从 JSON 加载）
type EventTypeConfig struct {
	Name            string  `json:"name"`             // "explosion", "gunshot", "shout"
	DefaultSeverity float64 `json:"default_severity"` // 默认严重度
	DefaultTTL      float64 `json:"default_ttl"`      // 默认 TTL（秒）
	PerceptionMode  string  `json:"perception_mode"`  // "visual" | "auditory" | "global"
	Range           float64 `json:"range"`            // 事件传播范围（米）
}

// --- 事件实例 ---

var nextEventID atomic.Int64

// Event 一个运行时事件实例
type Event struct {
	ID       string  // 唯一 ID
	Type     string  // 事件类型名，对应 EventTypeConfig.Name
	Position Vec3    // 事件发生位置
	Severity float64 // 实际 severity
	TTL      float64 // 剩余生存时间（秒），每 Tick 递减
	SourceID string  // 事件来源实体 ID
}

// NewEvent 从事件类型配置创建事件实例，可覆盖 severity
func NewEvent(typeCfg *EventTypeConfig, pos Vec3, sourceID string, severityOverride float64) *Event {
	severity := typeCfg.DefaultSeverity
	if severityOverride > 0 {
		severity = severityOverride
	}
	id := fmt.Sprintf("evt_%d", nextEventID.Add(1))
	return &Event{
		ID:       id,
		Type:     typeCfg.Name,
		Position: pos,
		Severity: severity,
		TTL:      typeCfg.DefaultTTL,
		SourceID: sourceID,
	}
}

// --- 事件总线 ---

// Bus 事件总线，管理活跃事件的生命周期
type Bus struct {
	mu     sync.RWMutex
	active []*Event
}

// NewBus 创建空的事件总线
func NewBus() *Bus {
	return &Bus{
		active: make([]*Event, 0),
	}
}

// Publish 发布一个事件到总线
func (b *Bus) Publish(evt *Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.active = append(b.active, evt)
}

// Tick 衰减所有事件的 TTL，移除过期事件
func (b *Bus) Tick(dt float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	alive := b.active[:0] // 复用底层数组
	for _, evt := range b.active {
		evt.TTL -= dt
		if evt.TTL > 0 {
			alive = append(alive, evt)
		}
	}
	b.active = alive
}

// Active 返回当前所有活跃事件的快照（只读）
func (b *Bus) Active() []*Event {
	b.mu.RLock()
	defer b.mu.RUnlock()

	snapshot := make([]*Event, len(b.active))
	copy(snapshot, b.active)
	return snapshot
}

// ActiveCount 返回当前活跃事件数量
func (b *Bus) ActiveCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.active)
}
