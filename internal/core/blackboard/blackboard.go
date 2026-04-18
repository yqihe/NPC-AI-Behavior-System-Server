package blackboard

import (
	"fmt"
	"reflect"
	"sync"
)

// --- 全局 Key 注册表 ---

// keyInfo 记录一个已注册 Key 的元信息
type keyInfo struct {
	name     string
	typeName string
}

var (
	registryMu sync.RWMutex
	registry   = make(map[string]keyInfo)
)

// register 注册一个 Key，重复注册同名 Key 会 panic
func register(name string, typeName string) {
	registryMu.Lock()
	defer registryMu.Unlock()

	if existing, ok := registry[name]; ok {
		panic(fmt.Sprintf("blackboard: duplicate key registration: %q (existing type: %s, new type: %s)",
			name, existing.typeName, typeName))
	}
	registry[name] = keyInfo{name: name, typeName: typeName}
}

// RegisterDynamic 动态注册 Key，用于 ADMIN NPC fields 等运行时来源。
// 与 register 不同：同名重复幂等（首次注册生效，后续忽略），不 panic。
func RegisterDynamic(name, typeName string) {
	registryMu.Lock()
	defer registryMu.Unlock()

	if _, ok := registry[name]; ok {
		return
	}
	registry[name] = keyInfo{name: name, typeName: typeName}
}

// SetDynamic 写入动态 field 值，未注册时自动通过 RegisterDynamic 注册。
// 类型名从 val 反射推断。nil val 注册为 "nil"。
func SetDynamic(bb *Blackboard, name string, val any) {
	typeName := "nil"
	if t := reflect.TypeOf(val); t != nil {
		typeName = t.String()
	}
	RegisterDynamic(name, typeName)

	bb.mu.Lock()
	defer bb.mu.Unlock()
	bb.data[name] = val
}

// IsRegistered 检查 Key 名称是否已注册（供配置加载校验用）
func IsRegistered(name string) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	_, ok := registry[name]
	return ok
}

// ValidateKeyName 校验 Key 名称是否已注册，未注册返回 error
func ValidateKeyName(name string) error {
	if !IsRegistered(name) {
		return fmt.Errorf("blackboard: unknown key %q, not in registry", name)
	}
	return nil
}

// RegisteredKeys 返回所有已注册 Key 的名称列表（用于调试/文档）
func RegisteredKeys() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	keys := make([]string, 0, len(registry))
	for k := range registry {
		keys = append(keys, k)
	}
	return keys
}

// --- BBKey[T] 泛型 Key ---

// BBKey 是强类型的 Blackboard Key，编译期绑定值类型 T
type BBKey[T any] struct {
	name string
}

// NewKey 创建并注册一个 BBKey。所有 Key 必须通过此函数创建。
func NewKey[T any](name string) BBKey[T] {
	var zero T
	typeName := reflect.TypeOf(&zero).Elem().String()
	register(name, typeName)
	return BBKey[T]{name: name}
}

// Name 返回 Key 的字符串名称
func (k BBKey[T]) Name() string {
	return k.name
}

// --- Blackboard ---

// Blackboard 是 NPC 的数据共享中心，所有读写通过 BBKey[T] 保证类型安全
type Blackboard struct {
	mu   sync.RWMutex
	data map[string]any
}

// New 创建一个空的 Blackboard
func New() *Blackboard {
	return &Blackboard{
		data: make(map[string]any),
	}
}

// Get 从 Blackboard 读取值，类型由 BBKey[T] 编译期确定
func Get[T any](bb *Blackboard, key BBKey[T]) (T, bool) {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	val, ok := bb.data[key.name]
	if !ok {
		var zero T
		return zero, false
	}
	return val.(T), true
}

// Set 向 Blackboard 写入值，类型由 BBKey[T] 编译期确定
func Set[T any](bb *Blackboard, key BBKey[T], val T) {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	bb.data[key.name] = val
}

// Has 检查 Blackboard 中是否存在某个 Key 的值
func Has[T any](bb *Blackboard, key BBKey[T]) bool {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	_, ok := bb.data[key.name]
	return ok
}

// Delete 从 Blackboard 删除一个 Key 的值
func Delete[T any](bb *Blackboard, key BBKey[T]) {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	delete(bb.data, key.name)
}

// GetRaw 通过字符串名称读取值（供规则匹配器使用）
// 调用前应通过 ValidateKeyName 校验 Key 是否注册
func (bb *Blackboard) GetRaw(name string) (any, bool) {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	val, ok := bb.data[name]
	return val, ok
}

// Dump 导出全部 key-value 副本（供调试查询使用）
func (bb *Blackboard) Dump() map[string]any {
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	result := make(map[string]any, len(bb.data))
	for k, v := range bb.data {
		result[k] = v
	}
	return result
}

// SetRaw 通过字符串名称写入值（供 BT 叶子节点使用）
// 未注册的 Key 写入时 panic（R3）
func (bb *Blackboard) SetRaw(name string, val any) {
	if !IsRegistered(name) {
		panic(fmt.Sprintf("blackboard: SetRaw with unregistered key %q", name))
	}

	bb.mu.Lock()
	defer bb.mu.Unlock()

	bb.data[name] = val
}
