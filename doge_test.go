package doge

// doge_test.go — 契约钉板。每条测试对应一条包注释里承诺的行为。
// 全局容器 + Reset 的组合与 t.Parallel 冲突, 本文件所有测试串行。

import (
	"strings"
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
	Provide(func() *svcB { return &svcB{} })
	Provide(func() *svcC { Get[*svcB](); return &svcC{} })
	// 制造环: A → C → A
	Provide(func() *svcA { Get[*svcC](); return &svcA{} })
	Replace(&svcC{}) // 清掉上面的 C, 重建带环版本
	Reset()

	Provide(func() *svcA { Get[*svcC](); return &svcA{} })
	Provide(func() *svcC { Get[*svcA](); return &svcC{} })
	mustPanic(t, "cycle", func() { Get[*svcA]() })
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
