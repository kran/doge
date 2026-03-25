# doge

Type-safe DI container for Go 1.18+. Global registry, generic API, no code generation.

One file, 75 lines, zero dependencies.

## Install

```
go get codeberg.org/kran/doge
```

## API

```go
func Set[T any](comp T, scopes ...string)        // Register. Panics if already registered.
func Get[T any](scopes ...string) T              // Retrieve. Panics if not found.
func TryGet[T any](keys ...string) (T, bool)     // Retrieve. Returns (zero, false) if not found.
func Reset()                                      // Clear all. For testing.
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
- Component names are derived from `reflect.Type`: package path + type name. 

## What it does not do

- Scopes or lifecycles
- Auto-wiring or constructor injection
- Lazy initialization
- Dependency graphs or cycle detection
