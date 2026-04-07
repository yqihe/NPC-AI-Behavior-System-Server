package zone

import (
	"log/slog"
	"sync"
)

// ZoneManager 管理所有区域
type ZoneManager struct {
	mu    sync.RWMutex
	zones map[string]*Zone
}

// NewZoneManager 创建空的区域管理器
func NewZoneManager() *ZoneManager {
	return &ZoneManager{
		zones: make(map[string]*Zone),
	}
}

// AddZone 注册一个区域
func (zm *ZoneManager) AddZone(z *Zone) {
	zm.mu.Lock()
	defer zm.mu.Unlock()
	zm.zones[z.ID] = z
	slog.Debug("zone.added", "zone_id", z.ID, "name", z.Name, "active", z.Active)
}

// GetZone 按 ID 查询区域
func (zm *ZoneManager) GetZone(id string) (*Zone, bool) {
	zm.mu.RLock()
	defer zm.mu.RUnlock()
	z, ok := zm.zones[id]
	return z, ok
}

// IsActive 检查区域是否活跃。空 zoneID 或未注册的区域返回 true（向后兼容）。
func (zm *ZoneManager) IsActive(zoneID string) bool {
	if zoneID == "" {
		return true
	}
	zm.mu.RLock()
	defer zm.mu.RUnlock()
	z, ok := zm.zones[zoneID]
	if !ok {
		return true
	}
	return z.Active
}

// Sleep 将区域标记为休眠
func (zm *ZoneManager) Sleep(zoneID string) {
	zm.mu.Lock()
	defer zm.mu.Unlock()
	if z, ok := zm.zones[zoneID]; ok {
		z.Active = false
		slog.Info("zone.sleep", "zone_id", zoneID)
	}
}

// Wake 将区域标记为活跃
func (zm *ZoneManager) Wake(zoneID string) {
	zm.mu.Lock()
	defer zm.mu.Unlock()
	if z, ok := zm.zones[zoneID]; ok {
		z.Active = true
		slog.Info("zone.wake", "zone_id", zoneID)
	}
}

// AllZones 返回所有区域
func (zm *ZoneManager) AllZones() []*Zone {
	zm.mu.RLock()
	defer zm.mu.RUnlock()
	result := make([]*Zone, 0, len(zm.zones))
	for _, z := range zm.zones {
		result = append(result, z)
	}
	return result
}

// Count 返回区域总数
func (zm *ZoneManager) Count() int {
	zm.mu.RLock()
	defer zm.mu.RUnlock()
	return len(zm.zones)
}
