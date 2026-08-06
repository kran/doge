// Package doge is a type-indexed service locator — not a dependency
// injection container, and it does not pretend to be one.
//
// 它解决的问题很窄: 小型项目里 "为了最底层用 *sql.DB 而层层传参" 的装配税。
// 它的代价也明确: 依赖从函数签名移进函数体, 依赖图的可见性由三条纪律维持,
// 其中前两条由本包机器强制, 第三条交给 lint:
//
//  1. Get 只属于装配期。组件在构造函数里 Get 依赖并收进字段;
//     运行路径上取用的是字段, 不是容器。装配完成后调用 Seal(),
//     此后 Get 一律 panic — 违规在第一次测试时就炸, 而不是三个月后。
//  2. Seal 同时物化所有 Provide 的惰性构造并冻结容器 — 缺组件、
//     构造失败、循环依赖全部在 Seal 时暴露 (fail fast), 不留到首个请求。
//  3. 约定只有 cmd/ 与各包的构造函数文件允许 import doge
//     (用 depguard/forbidigo 一条规则在 CI 强制) — 这样构造函数
//     就是该组件的依赖清单, 可见性与手动传参相差无几。
//
// 并发约定: 装配期 (Seal 之前的 Set/Provide/Get) 默认单 goroutine —
// 这是 main 函数的自然形态; Seal 之后 TryGet 并发安全。跨 goroutine
// 并发触发同一惰性构造会被检测并 panic (而非误报为循环依赖)。
//
// 失败即终止: 构造函数 panic 视为装配失败, 容器进入 broken 状态并拒绝
// 一切后续操作 — 装配失败的正确响应是进程终止, 不是恢复。
//
// 典型 main:
//
//	doge.Set(db)
//	doge.Provide(NewUserService)   // 依赖顺序无关: Get 时按需构造
//	doge.Provide(NewOrderService)
//	doge.Seal()                    // 物化 + 冻结, 装配期结束
//	router.Run(8080)
//
// NOTE: 多容器 (default container + New) 需要泛型方法 (Go 1.27) 才能做成
// Container 方法; 届时重构为 Container 类型 + 包级默认容器, 调用方式不变。
package doge

import (
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type state uint8

const (
	stateReady    state = iota // val 可用
	stateLazy                  // provider 未执行
	stateBuilding              // provider 执行中 (同 goroutine 再入 = 环; 跨 goroutine = 并发误用)
)

type entry struct {
	state    state
	val      any
	provider func() any
}

var (
	mu     sync.RWMutex
	sealed bool
	// broken: 某个构造函数 panic 后置位。此后一切操作 panic —
	// 半装配的容器状态未定义, 明确拒绝服务优于静默带病运行。
	broken bool
	// comps: 类型 → key → 组件。类型直接作 map key (底层 *rtype 指针,
	// 类型同一性 = 指针同一性): 零编码、零冲突、匿名类型自动处理。
	comps = map[reflect.Type]map[string]*entry{}
	// resolving: 构建中的解析链, 用于环检测与报错信息。
	resolving []string
	// resolvingG: 当前构建链所属 goroutine (0 = 无构建中)。
	// stateBuilding 撞见时据此区分: 同 goroutine = 真循环依赖,
	// 跨 goroutine = 并发触发惰性构造 (装配期单 goroutine 约定被违反)。
	resolvingG int64
)

// Set registers a ready component under type T and an optional key.
// Panics on duplicate registration (use Replace to override) or after Seal.
func Set[T any](comp T, key ...string) {
	register(typeOf[T](), oneKey(key), &entry{state: stateReady, val: comp}, false)
}

// Provide registers a lazy constructor for T. fn runs at most once — on the
// first Get, or at Seal (whichever comes first) — and the result is memoized.
//
// Provide 让注册顺序与依赖顺序解耦: 构造函数里 Get 的依赖若尚未物化,
// 会被递归按需构造。循环依赖在解析时 panic 并给出完整依赖链。
//
// 守则:
//   - Provide 与 Seal 成对使用: 惰性构造按装配期单 goroutine 设计,
//     不 Seal 就进入并发阶段, 两个 goroutine 并发触发同一构造会 panic
//     (且已构造一半的实例可能泄漏)。Seal 把全部构造做完, 运行期纯读。
//   - 不要在构造函数内 Set/Provide 注册组件 — 组件清单应在 main 里
//     一眼看全, 藏在构造函数里的注册破坏依赖可见性 (Seal 的重扫在
//     技术上容忍它, 但这属于"能跑不等于该用")。
//   - fn 一旦 panic 视为装配失败, 容器进入 broken 状态拒绝一切后续
//     操作 — 进程应当终止, 不要 recover 后继续使用容器。
func Provide[T any](fn func() T, key ...string) {
	if fn == nil {
		panic("doge: Provide with nil constructor: " + displayName(typeOf[T](), oneKey(key)))
	}
	register(typeOf[T](), oneKey(key),
		&entry{state: stateLazy, provider: func() any { return fn() }}, false)
}

// Replace registers or overrides a component. For tests: swap in a mock
// without Reset-ing the whole container. Panics after Seal.
func Replace[T any](comp T, key ...string) {
	register(typeOf[T](), oneKey(key), &entry{state: stateReady, val: comp}, true)
}

// Get retrieves a component, constructing it first if it was Provide-d.
// Panics if not found — a missing component is a wiring bug.
//
// Get 只属于装配期: Seal 之后调用一律 panic。运行期确有动态查找需求时
// 用 TryGet 显式表达。
func Get[T any](key ...string) T {
	t, k := typeOf[T](), oneKey(key)

	mu.RLock()
	s := sealed
	mu.RUnlock()
	if s {
		panic("doge: Get after Seal — capture dependencies at construction time" +
			" (use TryGet for intentional runtime lookup): " + displayName(t, k))
	}

	v, ok := resolve(t, k)
	if !ok {
		panic(notFoundMsg(t, k))
	}
	return v.(T)
}

// TryGet attempts to retrieve a component. Returns (zero, false) if absent.
// Allowed at any time; after Seal it is the sole (and explicit) runtime
// lookup path, backed by a fully materialized, immutable container.
func TryGet[T any](key ...string) (T, bool) {
	v, ok := resolve(typeOf[T](), oneKey(key))
	if !ok {
		var zero T
		return zero, false
	}
	return v.(T), true
}

// Seal ends the assembly phase: it force-resolves every pending Provide
// (surfacing missing deps, constructor panics and cycles right now), then
// freezes the container — subsequent Set/Provide/Replace/Get panic, TryGet
// keeps working. Idempotent.
func Seal() {
	for {
		mu.Lock()
		if broken {
			mu.Unlock()
			panicBroken()
		}
		if sealed {
			mu.Unlock()
			return
		}
		t, k, pending := findPending()
		if !pending {
			sealed = true
			mu.Unlock()
			return
		}
		mu.Unlock()
		resolve(t, k) // 物化; 其构造函数可能连带解析其他组件, 故循环重扫
	}
}

// findPending 找一个未物化的 lazy 条目 (调用方持有 mu)。
func findPending() (reflect.Type, string, bool) {
	for t, m := range comps {
		for k, e := range m {
			if e.state == stateLazy {
				return t, k, true
			}
		}
	}
	return nil, "", false
}

// Reset clears all components and un-seals / un-breaks the container.
// For tests only. 与 t.Parallel 天然冲突 (全局容器的固有代价) —
// 并行测试请勿共享容器状态。
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	comps = map[reflect.Type]map[string]*entry{}
	resolving = nil
	resolvingG = 0
	sealed = false
	broken = false
}

// ── internals ────────────────────────────────────────────

func typeOf[T any]() reflect.Type { return reflect.TypeOf((*T)(nil)).Elem() }

// oneKey 归一化可选 key: 最多一个。多 key 曾被 join 成 "a/b", 与字面
// "a/b" 歧义且无用例, 现直接拒绝。
func oneKey(keys []string) string {
	switch len(keys) {
	case 0:
		return ""
	case 1:
		return keys[0]
	}
	panic("doge: at most one key allowed, got: " + strings.Join(keys, ", "))
}

// panicBroken 报 broken。锁契约: 调用方必须已释放 mu —
// broken 检查一律"锁内读、锁外炸", 在临界区内 panic 会永久持锁
func panicBroken() {
	panic("doge: container broken by an earlier constructor panic — restart the process")
}

func register(t reflect.Type, key string, e *entry, override bool) {
	mu.Lock()
	defer mu.Unlock()
	if broken {
		panicBroken()
	}
	if sealed {
		panic("doge: register after Seal: " + displayName(t, key))
	}
	m := comps[t]
	if m == nil {
		m = map[string]*entry{}
		comps[t] = m
	}
	if _, ok := m[key]; ok && !override {
		panic("doge: component already exists: " + displayName(t, key) +
			" (use Replace to override)")
	}
	m[key] = e
}

// resolve 取值, lazy 条目按需构造 (memoized)。
// 锁纪律: provider 执行期间不持锁 — 构造函数会递归 Get 依赖,
// 持锁会自锁死。stateBuilding 撞见时按 goroutine 区分两种病:
// 同 goroutine 再入 = 循环依赖 (附完整链); 跨 goroutine = 并发触发
// 惰性构造 (违反装配期单 goroutine 约定, 提示调用 Seal)。
// provider panic 时置 broken 后原样重抛 — 不回滚、不修剪解析链,
// broken 状态使残留的中间状态不可达。
func resolve(t reflect.Type, key string) (any, bool) {
	mu.Lock()
	if broken {
		mu.Unlock()
		panicBroken()
	}
	e := comps[t][key] // nil map 索引安全
	if e == nil {
		mu.Unlock()
		return nil, false
	}
	switch e.state {
	case stateReady:
		v := e.val
		mu.Unlock()
		return v, true
	case stateBuilding:
		if goid() == resolvingG {
			chain := strings.Join(append(append([]string{}, resolving...),
				displayName(t, key)), " → ")
			mu.Unlock()
			panic("doge: provider cycle: " + chain)
		}
		mu.Unlock()
		panic("doge: concurrent lazy construction of " + displayName(t, key) +
			" — resolve all Provide'd components before going concurrent (call Seal first)")
	}

	// stateLazy → 构建
	e.state = stateBuilding
	if len(resolving) == 0 {
		resolvingG = goid()
	}
	resolving = append(resolving, displayName(t, key))
	provider := e.provider
	mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			mu.Lock()
			broken = true // 装配失败: 冻结为拒绝服务, 见 checkBroken
			mu.Unlock()
			panic(r) // 原样重抛, 不吞用户 panic
		}
	}()
	v := provider() // 可能递归 resolve 依赖

	mu.Lock()
	e.val, e.state, e.provider = v, stateReady, nil
	resolving = resolving[:len(resolving)-1]
	if len(resolving) == 0 {
		resolvingG = 0
	}
	mu.Unlock()
	return v, true
}

// goid 当前 goroutine id — 仅用于 stateBuilding 冷路径的病因区分,
// 不参与任何热路径。runtime.Stack 首行 "goroutine N [running]:" 的
// 解析是各版本稳定的惯用法; Go 无公开 API 属有意设计, 这里的用途
// (区分报错文案) 不构成对 goroutine 身份的逻辑依赖。
func goid() int64 {
	var buf [32]byte
	n := runtime.Stack(buf[:], false)
	s := strings.TrimPrefix(string(buf[:n]), "goroutine ")
	if i := strings.IndexByte(s, ' '); i > 0 {
		if id, err := strconv.ParseInt(s[:i], 10, 64); err == nil {
			return id
		}
	}
	return -1 // 解析失败: 保守返回不等于任何 resolvingG 的值 → 报并发而非环
}

// notFoundMsg 未找到时的报错: 附带该类型已注册的 key 列表,
// 便于区分 "漏注册" 和 "key 拼错" 两种最常见的布线错误。
func notFoundMsg(t reflect.Type, key string) string {
	msg := "doge: component not found: " + displayName(t, key)
	mu.RLock()
	if m := comps[t]; len(m) > 0 {
		ks := make([]string, 0, len(m))
		for k := range m {
			if k == "" {
				k = `(no key)`
			}
			ks = append(ks, k)
		}
		sort.Strings(ks)
		msg += " — registered keys for this type: " + strings.Join(ks, ", ")
	}
	mu.RUnlock()
	return msg
}

// displayName renders a readable name for error messages (never used as a key).
func displayName(t reflect.Type, key string) string {
	if key != "" {
		return t.String() + "@" + key
	}
	return t.String()
}
