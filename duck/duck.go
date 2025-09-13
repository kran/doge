package duck

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

var (
	MissingPKError = errors.New("missing primary key declaration")
	ArgsNumError   = errors.New("args number not match with query placeholders")
)

const (
	FIELD  = "*FIELD"
	IGNORE = "*IGNORE"
)

type Params map[string]interface{}

type Duck struct {
	driverName  string
	queries     []string
	args        []any
	marks       map[string]int
	db          *sql.DB
	tx          *sql.Conn
	FieldMapper FieldMapFunc
	TableMapper TableMapFunc
	QuoteColumn func(string) string
	QuoteTable  func(string) string
	Marker      func(int) string
	Log         func(string, ...any)
}

func NewFromDB(db *sql.DB, driverName string) *Duck {
	duck := &Duck{
		driverName:  driverName,
		queries:     make([]string, 0),
		args:        make([]any, 0),
		marks:       make(map[string]int),
		db:          db,
		FieldMapper: DefaultFieldMapFunc,
		TableMapper: GetTableName,
		QuoteColumn: quoter,
		QuoteTable:  quoter,
		Marker:      marker,
		Log:         logger,
	}

	if driverName == "mysql" {
		duck.QuoteColumn = mysqlQuoter
		duck.QuoteTable = mysqlQuoter
	}

	if driverName == "postgres" {
		duck.Marker = pgMarker
	}

	return duck
}

func Open(driverName, dsn string) (*Duck, error) {
	sqlDB, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}

	return NewFromDB(sqlDB, driverName), nil
}

func MustOpen(driverName, dsn string) (*Duck, error) {
	db, err := Open(driverName, dsn)
	if err != nil {
		return nil, err
	}
	if err := db.db.Ping(); err != nil {
		_ = db.db.Close()
		return nil, err
	}
	return db, nil
}

func (d *Duck) DB() *sql.DB {
	return d.db
}

func (d *Duck) copy() *Duck {
	duck := &Duck{
		driverName:  d.driverName,
		marks:       make(map[string]int),
		queries:     make([]string, len(d.queries)),
		args:        make([]any, len(d.args)),
		db:          d.db,
		tx:          d.tx,
		FieldMapper: d.FieldMapper,
		TableMapper: d.TableMapper,
		QuoteColumn: d.QuoteColumn,
		QuoteTable:  d.QuoteTable,
		Marker:      d.Marker,
		Log:         d.Log,
	}

	copy(duck.queries, d.queries)
	copy(duck.args, d.args)

	for k, v := range d.marks {
		duck.marks[k] = v
	}

	return duck
}

func (d *Duck) Mark(name string, query string) *Duck {
	duck := d.copy()
	if _, ok := duck.marks[name]; ok {
		duck.queries[duck.marks[name]] = query
	} else {
		duck.marks[name] = len(duck.queries)
		duck.queries = append(duck.queries, query)
	}
	return duck
}

func (d *Duck) Add(query string, args ...any) *Duck {
	duck := d.copy()
	duck.addInner(query, args)
	return duck
}

func (d *Duck) AddIf(cond bool, query string, args ...any) *Duck {
	if cond {
		return d.Add(query, args...)
	}
	return d
}

func (d *Duck) Row(a ...any) error {
	rows, err := d.query()
	if err != nil {
		return err
	}
	return rows.row(a...)
}

func (d *Duck) Column(slice any) error {
	rows, err := d.query()
	if err != nil {
		return err
	}
	return rows.column(slice)
}

func (d *Duck) One(a any) error {
	rows, err := d.query()
	if err != nil {
		return err
	}
	return rows.one(a)
}

func (d *Duck) All(a any) error {
	rows, err := d.query()
	if err != nil {
		return err
	}
	return rows.all(a)
}

func (d *Duck) Select(table string, where string, args ...any) *Duck {
	q := "FROM " + d.QuoteTable(table) + " WHERE " + where
	return d.Add("SELECT").Mark(FIELD, "*").Add(q, args...)
}

func (d *Duck) Delete(table string, where string, args ...any) (int64, error) {
	q := "DELETE FROM " + d.QuoteTable(table) + " WHERE " + where
	return d.Add(q, args...).Update()
}

func (d *Duck) Insert() (int64, error) {
	res, err := d.exec()
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *Duck) Update() (int64, error) {
	res, err := d.exec()
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (d *Duck) InsertModel(model any, attrs ...string) (int64, error) {
	mv := newStructValue(model, d.FieldMapper, d.TableMapper)
	cols := mv.columns(attrs, []string{})

	pkName := ""
	for name, value := range mv.pk() {
		if isAutoInc(value) {
			delete(cols, name)
			pkName = name
			break
		}
	}

	if pkName == "" {
		return d.InsertMap(mv.tableName, cols)
	}

	pkValue, err := d.InsertMap(mv.tableName, cols)
	if err != nil {
		return 0, err
	}

	pkField := indirect(mv.dbNameMap[pkName].getField(mv.value))
	switch pkField.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		pkField.SetUint(uint64(pkValue))
	default:
		pkField.SetInt(pkValue)
	}

	return pkValue, nil
}

func (d *Duck) InsertMap(table string, cols Params) (int64, error) {
	return d.AddInsertMap(table, cols).Insert()
}

func (d *Duck) AddInsertModel(model any, attrs ...string) *Duck {
	mv := newStructValue(model, d.FieldMapper, d.TableMapper)
	cols := mv.columns(attrs, []string{})

	for name, value := range mv.pk() {
		if isAutoInc(value) {
			delete(cols, name)
			break
		}
	}

	return d.AddInsertMap(mv.tableName, cols)
}

func (d *Duck) AddInsertMap(table string, cols Params) *Duck {
	names := make([]string, 0, len(cols))
	for name := range cols {
		names = append(names, name)
	}
	sort.Strings(names)

	params := make([]any, len(names))
	columns := make([]string, len(names))
	marks := make([]string, len(names))
	for idx, name := range names {
		columns[idx] = d.QuoteColumn(d.FieldMapper(name))
		marks[idx] = "?"
		params[idx] = cols[name]
	}

	s := fmt.Sprintf("INTO %v (%v) VALUES (%v)",
		d.QuoteTable(table),
		strings.Join(columns, ", "),
		strings.Join(marks, ", "),
	)

	return d.Add("INSERT").Mark(IGNORE, "").Add(s, params...)
}

func (d *Duck) UpdateModel(model any, attrs ...string) (int64, error) {
	mv := newStructValue(model, d.FieldMapper, d.TableMapper)
	pk := mv.pk()
	if len(pk) == 0 {
		return 0, MissingPKError
	}

	cols := mv.columns(attrs, []string{})
	for name := range pk {
		delete(cols, name)
	}

	idx := 0
	where := ""
	args := make([]any, len(pk))
	for name, value := range pk {
		if where != "" {
			where += " AND "
		}
		where += d.QuoteColumn(name) + "=?"
		args[idx] = value
		idx = idx + 1
	}

	return d.UpdateMap(mv.tableName, cols, where, args...)
}

func (d *Duck) UpdateMap(table string, cols Params, where string, args ...any) (int64, error) {
	names := make([]string, 0, len(cols))
	for name := range cols {
		names = append(names, name)
	}
	sort.Strings(names)

	params := make([]any, len(names))
	marks := make([]string, len(names))
	for idx, name := range names {
		marks[idx] = d.QuoteColumn(d.FieldMapper(name)) + "=?"
		params[idx] = cols[name]
	}

	s := fmt.Sprintf("UPDATE %v SET %v WHERE",
		d.QuoteTable(table),
		strings.Join(marks, ", "),
	)

	return d.Add(s, params...).Add(where, args...).Update()
}

func (d *Duck) exec() (sql.Result, error) {
	query := strings.Join(d.queries, " ")
	if d.Log != nil {
		d.Log(query, d.args...)
	}

	return d.db.Exec(query, d.args...)
}

func (d *Duck) query() (*Rows, error) {
	query := strings.Join(d.queries, " ")

	if d.Log != nil {
		d.Log(query, d.args...)
	}

	rows, err := d.db.Query(query, d.args...)
	if err != nil {
		return nil, err
	}

	return &Rows{
		Rows:         rows,
		fieldMapFunc: d.FieldMapper,
	}, nil
}

func (d *Duck) addInner(query string, args []any) {
	parts := strings.Split(query+" ", "?")
	if len(parts) != len(args)+1 {
		panic(ArgsNumError)
	}
	for i, part := range parts {
		if len(args) <= i {
			d.queries = append(d.queries, part)
			continue
		}

		arg := args[i]

		if arg == nil {
			d.queries = append(d.queries, part+d.Marker(len(d.args)))
			d.args = append(d.args, nil)
			continue
		}

		rv := indirect(reflect.ValueOf(arg))
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			marks := make([]string, rv.Len())
			for idx := 0; idx < rv.Len(); idx++ {
				marks[idx] = d.Marker(len(d.args))
				d.args = append(d.args, rv.Index(idx).Interface())
			}
			d.queries = append(d.queries, part+strings.Join(marks, ","))
		} else {
			d.queries = append(d.queries, part+d.Marker(len(d.args)))
			d.args = append(d.args, arg)
		}
	}
}

func isAutoInc(value interface{}) bool {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Ptr:
		return v.IsNil() || isAutoInc(v.Elem())
	case reflect.Invalid:
		return true
	default:
		return false
	}
}
