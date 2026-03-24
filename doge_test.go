package doge

import (
	"sync"
	"testing"
)

func setup() {
	Reset()
}

// --- Set / Get basics ---

type testService struct {
	Name string
}

func TestProvideAndResolve(t *testing.T) {
	setup()
	svc := &testService{Name: "hello"}
	Set[*testService](svc)

	got := Get[*testService]()
	if got != svc {
		t.Errorf("Get returned different instance")
	}
	if got.Name != "hello" {
		t.Errorf("Name = %q, want %q", got.Name, "hello")
	}
}

func TestProvideValueType(t *testing.T) {
	setup()
	Set[int](42)

	got := Get[int]()
	if got != 42 {
		t.Errorf("Get[int] = %d, want 42", got)
	}
}

func TestProvideString(t *testing.T) {
	setup()
	Set[string]("config-value")

	got := Get[string]()
	if got != "config-value" {
		t.Errorf("Get[string] = %q", got)
	}
}

// --- Pointer vs Value are distinct types ---

func TestPointerAndValueAreDistinct(t *testing.T) {
	setup()
	svc := testService{Name: "value"}
	svcPtr := &testService{Name: "pointer"}

	Set[testService](svc)
	Set[*testService](svcPtr)

	gotVal := Get[testService]()
	gotPtr := Get[*testService]()

	if gotVal.Name != "value" {
		t.Errorf("value type: Name = %q, want %q", gotVal.Name, "value")
	}
	if gotPtr.Name != "pointer" {
		t.Errorf("pointer type: Name = %q, want %q", gotPtr.Name, "pointer")
	}
}

// --- Interface ---

type Greeter interface {
	Greet() string
}

type englishGreeter struct{}

func (g *englishGreeter) Greet() string { return "hello" }

type frenchGreeter struct{}

func (g *frenchGreeter) Greet() string { return "bonjour" }

func TestProvideInterface(t *testing.T) {
	setup()
	Set[Greeter](&englishGreeter{})

	got := Get[Greeter]()
	if got.Greet() != "hello" {
		t.Errorf("Greet() = %q", got.Greet())
	}
}

// --- Keyed instances ---

func TestKeyedInstances(t *testing.T) {
	setup()
	Set[string]("primary-dsn", "primary")
	Set[string]("replica-dsn", "replica")

	if Get[string]("primary") != "primary-dsn" {
		t.Errorf("primary = %q", Get[string]("primary"))
	}
	if Get[string]("replica") != "replica-dsn" {
		t.Errorf("replica = %q", Get[string]("replica"))
	}
}

func TestKeyedAndUnkeyedAreDistinct(t *testing.T) {
	setup()
	Set[string]("default")
	Set[string]("keyed", "special")

	if Get[string]() != "default" {
		t.Errorf("unkeyed = %q", Get[string]())
	}
	if Get[string]("special") != "keyed" {
		t.Errorf("keyed = %q", Get[string]("special"))
	}
}

func TestKeyedInterface(t *testing.T) {
	setup()
	Set[Greeter](&englishGreeter{}, "en")
	Set[Greeter](&frenchGreeter{}, "fr")

	if Get[Greeter]("en").Greet() != "hello" {
		t.Error("en greeter wrong")
	}
	if Get[Greeter]("fr").Greet() != "bonjour" {
		t.Error("fr greeter wrong")
	}
}

// --- TryGet ---

func TestTryResolveFound(t *testing.T) {
	setup()
	Set[int](99)

	v, ok := TryGet[int]()
	if !ok || v != 99 {
		t.Errorf("TryGet = (%d, %v)", v, ok)
	}
}

func TestTryResolveNotFound(t *testing.T) {
	setup()
	v, ok := TryGet[int]()
	if ok {
		t.Error("should not find unregistered type")
	}
	if v != 0 {
		t.Errorf("zero value should be 0, got %d", v)
	}
}

func TestTryResolveKeyedNotFound(t *testing.T) {
	setup()
	Set[string]("exists", "a")

	_, ok := TryGet[string]("b")
	if ok {
		t.Error("should not find wrong key")
	}

	_, ok = TryGet[string]()
	if ok {
		t.Error("should not find unkeyed when only keyed exists")
	}
}

// --- Duplicate Set panics ---

func TestProvideDuplicatePanics(t *testing.T) {
	setup()
	Set[int](1)

	defer func() {
		if r := recover(); r == nil {
			t.Error("duplicate Set should panic")
		}
	}()
	Set[int](2)
}

func TestProvideDuplicateKeyedPanics(t *testing.T) {
	setup()
	Set[string]("a", "key1")

	defer func() {
		if r := recover(); r == nil {
			t.Error("duplicate keyed Set should panic")
		}
	}()
	Set[string]("b", "key1")
}

func TestProvideSameTypeDifferentKeysOK(t *testing.T) {
	setup()
	// Should NOT panic
	Set[string]("a", "key1")
	Set[string]("b", "key2")
}

// --- Get not found panics ---

func TestResolveNotFoundPanics(t *testing.T) {
	setup()
	defer func() {
		if r := recover(); r == nil {
			t.Error("Get unregistered type should panic")
		}
	}()
	Get[int]()
}

func TestResolveWrongKeyPanics(t *testing.T) {
	setup()
	Set[int](1, "a")

	defer func() {
		if r := recover(); r == nil {
			t.Error("Get with wrong key should panic")
		}
	}()
	Get[int]("b")
}

// --- Reset ---

func TestReset(t *testing.T) {
	setup()
	Set[int](42)
	Reset()

	_, ok := TryGet[int]()
	if ok {
		t.Error("Reset should clear all components")
	}
}

func TestResetThenProvideAgain(t *testing.T) {
	setup()
	Set[int](1)
	Reset()
	// Should not panic — slot is cleared
	Set[int](2)

	if Get[int]() != 2 {
		t.Errorf("got %d, want 2", Get[int]())
	}
}

// --- Concurrency ---

func TestConcurrentProvideResolve(t *testing.T) {
	setup()
	// Pre-register so resolves don't panic
	Set[int](0)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = Get[int]()
		}()
	}
	wg.Wait()
}

func TestConcurrentTryResolve(t *testing.T) {
	setup()

	var wg sync.WaitGroup
	// Some goroutines try to resolve, some provide
	Set[string]("val")
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, ok := TryGet[string]()
			if ok && v != "val" {
				t.Errorf("unexpected value: %q", v)
			}
		}()
	}
	wg.Wait()
}

// --- compName ---

func TestCompNameDistinguishesTypes(t *testing.T) {
	// Verify that the internal naming produces distinct keys
	// for different types. We test this indirectly: registering
	// int, string, and *testService should all coexist.
	setup()
	Set[int](1)
	Set[string]("s")
	Set[*testService](&testService{})

	if Get[int]() != 1 {
		t.Error("int wrong")
	}
	if Get[string]() != "s" {
		t.Error("string wrong")
	}
	if Get[*testService]() == nil {
		t.Error("*testService nil")
	}
}
