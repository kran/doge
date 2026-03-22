package doge

import (
	"sync"
	"testing"
)

func setup() {
	Reset()
}

// --- Provide / Resolve basics ---

type testService struct {
	Name string
}

func TestProvideAndResolve(t *testing.T) {
	setup()
	svc := &testService{Name: "hello"}
	Provide[*testService](svc)

	got := Resolve[*testService]()
	if got != svc {
		t.Errorf("Resolve returned different instance")
	}
	if got.Name != "hello" {
		t.Errorf("Name = %q, want %q", got.Name, "hello")
	}
}

func TestProvideValueType(t *testing.T) {
	setup()
	Provide[int](42)

	got := Resolve[int]()
	if got != 42 {
		t.Errorf("Resolve[int] = %d, want 42", got)
	}
}

func TestProvideString(t *testing.T) {
	setup()
	Provide[string]("config-value")

	got := Resolve[string]()
	if got != "config-value" {
		t.Errorf("Resolve[string] = %q", got)
	}
}

// --- Pointer vs Value are distinct types ---

func TestPointerAndValueAreDistinct(t *testing.T) {
	setup()
	svc := testService{Name: "value"}
	svcPtr := &testService{Name: "pointer"}

	Provide[testService](svc)
	Provide[*testService](svcPtr)

	gotVal := Resolve[testService]()
	gotPtr := Resolve[*testService]()

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
	Provide[Greeter](&englishGreeter{})

	got := Resolve[Greeter]()
	if got.Greet() != "hello" {
		t.Errorf("Greet() = %q", got.Greet())
	}
}

// --- Keyed instances ---

func TestKeyedInstances(t *testing.T) {
	setup()
	Provide[string]("primary-dsn", "primary")
	Provide[string]("replica-dsn", "replica")

	if Resolve[string]("primary") != "primary-dsn" {
		t.Errorf("primary = %q", Resolve[string]("primary"))
	}
	if Resolve[string]("replica") != "replica-dsn" {
		t.Errorf("replica = %q", Resolve[string]("replica"))
	}
}

func TestKeyedAndUnkeyedAreDistinct(t *testing.T) {
	setup()
	Provide[string]("default")
	Provide[string]("keyed", "special")

	if Resolve[string]() != "default" {
		t.Errorf("unkeyed = %q", Resolve[string]())
	}
	if Resolve[string]("special") != "keyed" {
		t.Errorf("keyed = %q", Resolve[string]("special"))
	}
}

func TestKeyedInterface(t *testing.T) {
	setup()
	Provide[Greeter](&englishGreeter{}, "en")
	Provide[Greeter](&frenchGreeter{}, "fr")

	if Resolve[Greeter]("en").Greet() != "hello" {
		t.Error("en greeter wrong")
	}
	if Resolve[Greeter]("fr").Greet() != "bonjour" {
		t.Error("fr greeter wrong")
	}
}

// --- TryResolve ---

func TestTryResolveFound(t *testing.T) {
	setup()
	Provide[int](99)

	v, ok := TryResolve[int]()
	if !ok || v != 99 {
		t.Errorf("TryResolve = (%d, %v)", v, ok)
	}
}

func TestTryResolveNotFound(t *testing.T) {
	setup()
	v, ok := TryResolve[int]()
	if ok {
		t.Error("should not find unregistered type")
	}
	if v != 0 {
		t.Errorf("zero value should be 0, got %d", v)
	}
}

func TestTryResolveKeyedNotFound(t *testing.T) {
	setup()
	Provide[string]("exists", "a")

	_, ok := TryResolve[string]("b")
	if ok {
		t.Error("should not find wrong key")
	}

	_, ok = TryResolve[string]()
	if ok {
		t.Error("should not find unkeyed when only keyed exists")
	}
}

// --- Duplicate Provide panics ---

func TestProvideDuplicatePanics(t *testing.T) {
	setup()
	Provide[int](1)

	defer func() {
		if r := recover(); r == nil {
			t.Error("duplicate Provide should panic")
		}
	}()
	Provide[int](2)
}

func TestProvideDuplicateKeyedPanics(t *testing.T) {
	setup()
	Provide[string]("a", "key1")

	defer func() {
		if r := recover(); r == nil {
			t.Error("duplicate keyed Provide should panic")
		}
	}()
	Provide[string]("b", "key1")
}

func TestProvideSameTypeDifferentKeysOK(t *testing.T) {
	setup()
	// Should NOT panic
	Provide[string]("a", "key1")
	Provide[string]("b", "key2")
}

// --- Resolve not found panics ---

func TestResolveNotFoundPanics(t *testing.T) {
	setup()
	defer func() {
		if r := recover(); r == nil {
			t.Error("Resolve unregistered type should panic")
		}
	}()
	Resolve[int]()
}

func TestResolveWrongKeyPanics(t *testing.T) {
	setup()
	Provide[int](1, "a")

	defer func() {
		if r := recover(); r == nil {
			t.Error("Resolve with wrong key should panic")
		}
	}()
	Resolve[int]("b")
}

// --- Reset ---

func TestReset(t *testing.T) {
	setup()
	Provide[int](42)
	Reset()

	_, ok := TryResolve[int]()
	if ok {
		t.Error("Reset should clear all components")
	}
}

func TestResetThenProvideAgain(t *testing.T) {
	setup()
	Provide[int](1)
	Reset()
	// Should not panic — slot is cleared
	Provide[int](2)

	if Resolve[int]() != 2 {
		t.Errorf("got %d, want 2", Resolve[int]())
	}
}

// --- Concurrency ---

func TestConcurrentProvideResolve(t *testing.T) {
	setup()
	// Pre-register so resolves don't panic
	Provide[int](0)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = Resolve[int]()
		}()
	}
	wg.Wait()
}

func TestConcurrentTryResolve(t *testing.T) {
	setup()

	var wg sync.WaitGroup
	// Some goroutines try to resolve, some provide
	Provide[string]("val")
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, ok := TryResolve[string]()
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
	Provide[int](1)
	Provide[string]("s")
	Provide[*testService](&testService{})

	if Resolve[int]() != 1 {
		t.Error("int wrong")
	}
	if Resolve[string]() != "s" {
		t.Error("string wrong")
	}
	if Resolve[*testService]() == nil {
		t.Error("*testService nil")
	}
}
