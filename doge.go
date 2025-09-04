package doge

import (
	"reflect"
	"sync"
)

var locker sync.RWMutex
var comps = map[string]any{}

func Bind(comp any, keys ...string) {
	t := reflect.TypeOf(comp)
	if t.Kind() != reflect.Ptr {
		panic("component must be a pointer")
	}

	locker.Lock()
	defer locker.Unlock()

	name := compName(t.Elem().String(), keys)
	if _, ok := comps[name]; ok {
		panic("component already exists: " + name)
	}
	comps[name] = comp
}

func Get[T any](keys ...string) *T {
	locker.RLock()
	defer locker.RUnlock()

	t := reflect.TypeOf((*T)(nil)).Elem().String()
	name := compName(t, keys)
	if comp, ok := comps[name]; ok {
		return comp.(*T)
	} else {
		panic("component not found: " + name)
	}
}

func compName(prefix string, arr []string) string {
	if len(arr) == 0 {
		return prefix + "@default"
	} else {
		return prefix + "@" + arr[0]
	}
}
