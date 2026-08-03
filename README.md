# doge

Type-safe DI container for Go 1.18+. Global registry, generic API, no code generation.

One file, ~100 lines, zero dependencies.

## Install

```
go get codeberg.org/kran/doge
```

## API

```go
func Set[T any](comp T, keys ...string)        // Register. Panics if already registered.
func Get[T any](keys ...string) T              // Retrieve. Panics if not found.
func TryGet[T any](keys ...string) (T, bool)   // Retrieve. Returns (zero, false) if not found.
func Reset()                                    // Clear all. For testing.
```

## Usage

```go
// Register
db := connectDB()
doge.Set[*sql.DB](db)

// Retrieve
db := doge.Get[*sql.DB]()

// Safe retrieve
db, ok := doge.TryGet[*sql.DB]()
```

### Interfaces

```go
type UserRepo interface { Find(id int64) (*User, error) }

doge.Set[UserRepo](&PostgresUserRepo{db: db})

repo := doge.Get[UserRepo]()
```

### Keyed instances

Multiple instances of the same type, distinguished by key:

```go
doge.Set[*sql.DB](primaryDB, "primary")
doge.Set[*sql.DB](replicaDB, "replica")

primary := doge.Get[*sql.DB]("primary")
replica := doge.Get[*sql.DB]("replica")
```

### Testing

```go
func TestSomething(t *testing.T) {
    doge.Reset()
    doge.Set[UserRepo](&MockUserRepo{})
    // ...
}
```

## Behavior

- `Set` the same type (and key) twice: panic.
- `Get` an unregistered type: panic.
- `TryGet` an unregistered type: returns zero value and `false`.
- All operations are goroutine-safe (`sync.RWMutex`).
- **The type itself is the registry key** (`reflect.Type` — underlying `*rtype`
  pointer, so type identity = pointer identity): no string encoding, no
  cross-package collisions, no naming rules. `*T`, `T`, `[]T` are distinct
  slots. Anonymous types (struct/interface/function literals) follow the
  same rules as named types — type identity makes same-shaped literals the
  same key, so a single instance needs no key:

```go
doge.Set[struct{ X int }](v)            // single instance: no key needed
x := doge.Get[struct{ X int }]()        // works — same shape = same type

doge.Set[struct{ X int }](v1, "a")      // multiple instances: key disambiguates
doge.Set[struct{ X int }](v2, "b")
```

- Optional keys are joined with `/` (`Set(x, "a", "b")` ≡ `Set(x, "a/b")`).

## What it does not do

- Scopes or lifecycles
- Auto-wiring or constructor injection
- Lazy initialization
- Dependency graphs or cycle detection
- Multiple containers (planned: `New()` + package-level default container —
  requires generic methods, Go 1.27)

## Roadmap

- **Go 1.27 (generic methods)**: refactor to `Container` type with methods
  `Set/Get/TryGet/Reset`, plus a package-level default container — call sites
  stay exactly as they are today.
