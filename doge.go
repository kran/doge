package doge

import (
	"reflect"
	"strings"
	"sync"
)

var (
	locker sync.RWMutex
	comps  = map[string]any{}
)

// Provide registers a component by its type T and optional key.
func Provide[T any](comp T, scopes ...string) {
	t := reflect.TypeOf((*T)(nil)).Elem()

	locker.Lock()
	defer locker.Unlock()

	name := compName(t, scopes)
	if _, ok := comps[name]; ok {
		panic("doge: component already exists: " + name)
	}
	comps[name] = comp
}

// Resolve retrieves a component by type T and optional key. Panics if not found.
func Resolve[T any](scopes ...string) T {
	locker.RLock()
	defer locker.RUnlock()

	t := reflect.TypeOf((*T)(nil)).Elem()
	name := compName(t, scopes)

	if comp, ok := comps[name]; ok {
		return comp.(T)
	}
	panic("doge: component not found: " + name)
}

// TryResolve attempts to retrieve a component. Returns zero value and false if not found.
func TryResolve[T any](keys ...string) (T, bool) {
	locker.RLock()
	defer locker.RUnlock()

	t := reflect.TypeOf((*T)(nil)).Elem()
	name := compName(t, keys)

	if comp, ok := comps[name]; ok {
		return comp.(T), true
	}
	var zero T
	return zero, false
}

// Reset clears all registered components. Primarily for testing.
func Reset() {
	locker.Lock()
	defer locker.Unlock()
	comps = map[string]any{}
}

func compName(t reflect.Type, arr []string) string {
	prefix := t.PkgPath() + "/" + t.Name()
	if prefix == "/" {
		prefix = t.String()
	}

	if len(arr) == 0 {
		return prefix
	}
	return prefix + "@" + strings.Join(arr, "/")
}
