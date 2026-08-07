package doge

// doge_test.go — 契约钉板。每条测试对应一条包注释里承诺的行为。
// 全局容器 + Reset 的组合与 t.Parallel 冲突, 本文件所有测试串行。

import (
	"strings"
	"sync"
	"testing"
)

type svcA struct{ n int }
type svcB struct{ a *svcA }
type svcC struct{ b *svcB }

func mustPanic(t *testing.T, substr string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("want panic containing %q, got none", substr)
		}
		if s, _ := r.(string); !strings.Contains(s, substr) {
			t.Fatalf("want panic containing %q, got: %v", substr, r)
		}
	}()
	fn()
}

func TestSetGet(t *testing.T) {
	defer Reset()
	Set(&svcA{n: 1})
	Set(&svcA{n: 2}, "second")

	if Get[*svcA]().n != 1 {
		t.Fatal("keyless get")
	}
	if Get[*svcA]("second").n != 2 {
		t.Fatal("keyed get")
	}
	if _, ok := TryGet[*svcB](); ok {
		t.Fatal("TryGet on absent must be false")
	}
}

func TestDuplicateAndReplace(t *testing.T) {
	defer Reset()
	Set(&svcA{n: 1})
	mustPanic(t, "already exists", func() { Set(&svcA{n: 2}) })

	Replace(&svcA{n: 9}) // 覆盖
	if Get[*svcA]().n != 9 {
		t.Fatal("replace did not override")
	}
	Replace(&svcB{}) // 也可作首次注册 (测试先 mock 后真实的场景)
	if _, ok := TryGet[*svcB](); !ok {
		t.Fatal("replace as first registration")
	}
}

func TestAtMostOneKey(t *testing.T) {
	defer Reset()
	mustPanic(t, "at most one key", func() { Set(&svcA{}, "a", "b") })
}

func TestNotFoundListsKeys(t *testing.T) {
	defer Reset()
	Set(&svcA{}, "east")
	Set(&svcA{}, "west")
	mustPanic(t, "east", func() { Get[*svcA]("eastt") }) // 报错附带已注册 key
}

func TestProvideLazyMemoizedOrderIndependent(t *testing.T) {
	defer Reset()
	calls := 0
	// 注册顺序与依赖顺序相反: B 先注册, 依赖后注册的 A
	Provide(func() *svcB { return &svcB{a: Get[*svcA]()} })
	Provide(func() *svcA { calls++; return &svcA{n: 7} })

	b := Get[*svcB]() // 触发递归构造 A
	if b.a.n != 7 {
		t.Fatal("lazy dependency not resolved")
	}
	Get[*svcA]()
	Get[*svcB]()
	if calls != 1 {
		t.Fatalf("provider must run once, ran %d", calls)
	}
}

func TestProvideCyclePanicsWithChain(t *testing.T) {
	defer Reset()
	// A → C → A: 同 goroutine 再入 stateBuilding = 真环, 报 cycle 附完整链
	Provide(func() *svcA { Get[*svcC](); return &svcA{} })
	Provide(func() *svcC { Get[*svcA](); return &svcC{} })
	mustPanic(t, "cycle", func() { Get[*svcA]() })
}

func TestConcurrentBuildDetectedNotMisreportedAsCycle(t *testing.T) {
	defer Reset()
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})

	Provide(func() *svcA { close(entered); <-release; return &svcA{} })

	go func() {
		defer close(done)
		Get[*svcA]() // goroutine 1: 进入构造并阻塞
	}()
	<-entered

	// goroutine 0 (main): 撞见 stateBuilding, 不同 goid → 报并发而非环
	mustPanic(t, "concurrent", func() { Get[*svcA]() })

	close(release)
	<-done // 等构造完成再 Reset, 避免后台 goroutine 触碰已清空的解析链
}

func TestBrokenFuse(t *testing.T) {
	defer Reset()
	Provide(func() *svcA { panic("boom") })

	mustPanic(t, "boom", func() { Get[*svcA]() }) // 用户 panic 原样重抛
	// 此后容器进入 broken: 一切操作明确拒绝, 而非未定义行为
	mustPanic(t, "broken", func() { Get[*svcA]() })
	mustPanic(t, "broken", func() { TryGet[*svcA]() })
	mustPanic(t, "broken", func() { Set(&svcB{}) })
	mustPanic(t, "broken", Seal)

	Reset() // Reset 清除 broken (测试场景)
	Set(&svcA{n: 1})
	if Get[*svcA]().n != 1 {
		t.Fatal("reset must clear broken state")
	}
}

func TestSealForcesResolutionAndFreezes(t *testing.T) {
	defer Reset()
	calls := 0
	Provide(func() *svcA { calls++; return &svcA{n: 5} }) // 从未被 Get

	Seal()
	if calls != 1 {
		t.Fatal("Seal must force-resolve pending providers")
	}
	// Seal 后: TryGet 可用 (读已物化值, 不再触发构造)
	if a, ok := TryGet[*svcA](); !ok || a.n != 5 || calls != 1 {
		t.Fatal("TryGet after Seal")
	}
	// Seal 后: Get / 注册 一律 panic
	mustPanic(t, "Get after Seal", func() { Get[*svcA]() })
	mustPanic(t, "register after Seal", func() { Set(&svcB{}) })
	mustPanic(t, "register after Seal", func() { Replace(&svcA{}) })

	Seal() // 幂等
}

func TestSealSurfacesMissingDependency(t *testing.T) {
	defer Reset()
	Provide(func() *svcB { return &svcB{a: Get[*svcA]()} }) // A 从未注册
	mustPanic(t, "not found", Seal)                         // fail fast 于 Seal, 而非首个请求
}

func TestResetUnseals(t *testing.T) {
	Set(&svcA{})
	Seal()
	Reset()
	Set(&svcA{n: 3}) // Reset 后可重新装配
	if Get[*svcA]().n != 3 {
		t.Fatal("reset must unseal")
	}
	Reset()
}

// ── 状态机穷举补充 ──────────────────────────────────────

// Seal 后 TryGet 并发安全: 注释声称的行为, 用 race 证明。
func TestSealConcurrentTryGet(t *testing.T) {
	defer Reset()
	Set(&svcA{n: 1})
	Provide(func() *svcB { return &svcB{} })
	Seal()

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if a, ok := TryGet[*svcA](); !ok || a.n != 1 {
				t.Errorf("TryGet after Seal: %v %v", a, ok)
			}
			if _, ok := TryGet[*svcB](); !ok {
				t.Error("TryGet lazy materialized by Seal")
			}
		}()
	}
	wg.Wait()
}

// 嵌套依赖链: C→B→A 三层 (现有 svcC→svcB→svcA), resolving 链进出正确,
// 每组件只构造一次。
func TestNestedDependencyChain(t *testing.T) {
	defer Reset()
	order := []string{}
	Provide(func() *svcC { order = append(order, "C"); return &svcC{b: Get[*svcB]()} })
	Provide(func() *svcB { order = append(order, "B"); return &svcB{a: Get[*svcA]()} })
	Provide(func() *svcA { order = append(order, "A"); return &svcA{n: 7} })

	c := Get[*svcC]() // 触发整链: A → B → C
	if c == nil || c.b == nil || c.b.a == nil || c.b.a.n != 7 {
		t.Fatal("chain not wired")
	}
	// provider 的 append 在函数体开头: 开始序 = 外层先开始 = [C, B, A]
	if len(order) != 3 || order[0] != "C" || order[1] != "B" || order[2] != "A" {
		t.Fatalf("construction order: %v", order)
	}
	// 再次 Get: 全物化, 不再构造
	Get[*svcC]()
	if len(order) != 3 {
		t.Fatalf("providers must run once: %v", order)
	}
}

// Seal 循环重扫: 构造函数里注册新组件 (技术容忍路径)。
func TestSealRescanOnRegister(t *testing.T) {
	defer Reset()
	Provide(func() *svcA { Set(&svcC{}); return &svcA{n: 1} }) // 构造时注册 C
	Seal()                                                     // 重扫应物化 C
	if _, ok := TryGet[*svcC](); !ok {
		t.Fatal("Seal should rescan and materialize constructor-registered C")
	}
	if _, ok := TryGet[*svcA](); !ok {
		t.Fatal("A should be materialized")
	}
}

// nil provider: panic 带类型名。
func TestProvideNilPanics(t *testing.T) {
	defer Reset()
	mustPanic(t, "nil constructor", func() {
		Provide[*svcA](nil)
	})
}
