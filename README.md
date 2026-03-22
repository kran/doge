# doge

Type-safe DI container for Go 1.18+. Global registry, generic API, no code generation.

One file, 77 lines, zero dependencies.

## Install

```
go get codeberg.org/kran/doge
```

## API

```go
func Provide[T any](comp T, keys ...string)          // Register. Panics if already registered.
func Resolve[T any](keys ...string) T                 // Retrieve. Panics if not found.
func TryResolve[T any](keys ...string) (T, bool)      // Retrieve. Returns (zero, false) if not found.
func Reset()                                           // Clear all. For testing.
```

## Usage

```go
// Register
db := connectDB()
doge.Provide[*sql.DB](db)

// Retrieve
db := doge.Resolve[*sql.DB]()

// Safe retrieve
db, ok := doge.TryResolve[*sql.DB]()
```

### Interfaces

```go
type UserRepo interface { Find(id int64) (*User, error) }

doge.Provide[UserRepo](&PostgresUserRepo{db: db})

repo := doge.Resolve[UserRepo]()
```

### Keyed instances

Multiple instances of the same type, distinguished by key:

```go
doge.Provide[*sql.DB](primaryDB, "primary")
doge.Provide[*sql.DB](replicaDB, "replica")

primary := doge.Resolve[*sql.DB]("primary")
replica := doge.Resolve[*sql.DB]("replica")
```

### Testing

```go
func TestSomething(t *testing.T) {
    doge.Reset()
    doge.Provide[UserRepo](&MockUserRepo{})
    // ...
}
```

## Behavior

- `Provide` the same type (and key) twice: panic.
- `Resolve` an unregistered type: panic.
- `TryResolve` an unregistered type: returns zero value and `false`.
- All operations are goroutine-safe (`sync.RWMutex`).
- Component names are derived from `reflect.Type`: package path + type name. Pointer types are unwrapped.

## What it does not do

- Scopes or lifecycles
- Auto-wiring or constructor injection
- Lazy initialization
- Dependency graphs or cycle detection
