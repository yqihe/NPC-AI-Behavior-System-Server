package npc

import (
	"sync"
)

// Registry NPC 注册表，管理所有活跃 NPC 实例
type Registry struct {
	mu   sync.RWMutex
	npcs map[string]*Instance
}

// NewRegistry 创建空的 NPC 注册表
func NewRegistry() *Registry {
	return &Registry{
		npcs: make(map[string]*Instance),
	}
}

// Add 添加 NPC 实例到注册表
func (r *Registry) Add(inst *Instance) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.npcs[inst.ID] = inst
}

// Remove 从注册表移除 NPC
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.npcs, id)
}

// Get 按 ID 查找 NPC
func (r *Registry) Get(id string) (*Instance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inst, ok := r.npcs[id]
	return inst, ok
}

// Count 返回注册表中的 NPC 数量
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.npcs)
}

// ForEach 遍历所有 NPC 实例，执行回调
// 注意：回调中不要调用 Registry 的写方法（Add/Remove），否则死锁
func (r *Registry) ForEach(fn func(*Instance)) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, inst := range r.npcs {
		fn(inst)
	}
}
