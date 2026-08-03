package doge

import (
	"reflect"
	"strings"
	"sync"
)

// NOTE: 多容器 (default container + New) 需要泛型方法 (Go 1.27) 才能做成
// Container 方法; 届时重构为 Container 类型 + 包级默认容器, 调用方式不变。

var (
	locker sync.RWMutex
	// comps: 类型 → key → 组件。类型直接作 map key (底层 *rtype 指针,
	// 类型同一性 = 指针同一性): 零编码、零冲突、匿名类型自动处理。
	comps = map[reflect.Type]map[string]any{}
)

// Set registers a component by its type T and optional key.
// Panics if the type (and key) is already registered.
func Set[T any](comp T, keys ...string) {
	t := reflect.TypeOf((*T)(nil)).Elem()
	key := joinKeys(keys)

	locker.Lock()
	defer locker.Unlock()

	m := comps[t]
	if m == nil {
		m = map[string]any{}
		comps[t] = m
	}
	if _, ok := m[key]; ok {
		panic("doge: component already exists: " + displayName(t, key))
	}
	m[key] = comp
}

// Get retrieves a component by type T and optional key. Panics if not found.
func Get[T any](keys ...string) T {
	t := reflect.TypeOf((*T)(nil)).Elem()
	key := joinKeys(keys)

	locker.RLock()
	defer locker.RUnlock()

	if m, ok := comps[t]; ok {
		if comp, ok := m[key]; ok {
			return comp.(T)
		}
	}
	panic("doge: component not found: " + displayName(t, key))
}

// TryGet attempts to retrieve a component. Returns zero value and false if not found.
func TryGet[T any](keys ...string) (T, bool) {
	t := reflect.TypeOf((*T)(nil)).Elem()
	key := joinKeys(keys)

	locker.RLock()
	defer locker.RUnlock()

	if m, ok := comps[t]; ok {
		if comp, ok := m[key]; ok {
			return comp.(T), true
		}
	}
	var zero T
	return zero, false
}

// Reset clears all registered components. Primarily for testing.
func Reset() {
	locker.Lock()
	defer locker.Unlock()
	comps = map[reflect.Type]map[string]any{}
}

// joinKeys normalizes optional keys into the inner map key.
func joinKeys(keys []string) string {
	return strings.Join(keys, "/")
}

// displayName renders a readable name for error messages (never used as a key).
func displayName(t reflect.Type, key string) string {
	if key != "" {
		return t.String() + "@" + key
	}
	return t.String()
}
