package database

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/lib/pq"
	"github.com/tursodatabase/libsql-client-go/libsql"
)

// IsTurso identifies the SQL dialect of a connection, including pools opened by Open.
func IsTurso(db *sql.DB) bool {
	if db == nil {
		return false
	}
	switch db.Driver().(type) {
	case tursoDriver, *libsql.Driver, libsql.Driver:
		return true
	}
	return false
}

// SQL selects an explicit query variant. Portable statements require no variant.
func SQL(db *sql.DB, postgres, sqlite string) string {
	if IsTurso(db) {
		return sqlite
	}
	return postgres
}

type ArrayValue interface {
	driver.Valuer
	sql.Scanner
}

// Array preserves PostgreSQL arrays and represents SQLite arrays as JSON.
// Passing a pointer also provides a scanner for stored array columns.
func Array(db *sql.DB, value any) ArrayValue {
	if IsTurso(db) {
		return &jsonArray{value: value}
	}
	return pq.Array(value)
}

type jsonArray struct{ value any }

func (a *jsonArray) Value() (driver.Value, error) {
	v := reflect.ValueOf(a.value)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if !v.IsValid() || (v.Kind() == reflect.Slice && v.IsNil()) {
		return nil, nil
	}
	b, err := json.Marshal(v.Interface())
	return string(b), err
}
func (a *jsonArray) Scan(src any) error {
	v := reflect.ValueOf(a.value)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Slice {
		return errors.New("array scan requires a pointer to a slice")
	}
	if src == nil {
		v.Elem().Set(reflect.Zero(v.Elem().Type()))
		return nil
	}
	var b []byte
	switch x := src.(type) {
	case string:
		b = []byte(x)
	case []byte:
		b = x
	default:
		return fmt.Errorf("cannot scan %T as JSON array", src)
	}
	// encoding/json silently replaces NULL primitive elements with zero values.
	switch v.Elem().Type().Elem().Kind() {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		var elements []json.RawMessage
		if err := json.Unmarshal(b, &elements); err != nil {
			return err
		}
		for i, element := range elements {
			if string(element) == "null" {
				return fmt.Errorf("cannot scan NULL array element %d into %s", i, v.Elem().Type().Elem())
			}
		}
	}
	return json.Unmarshal(b, a.value)
}

func IsUniqueViolation(err error) bool {
	var pg *pq.Error
	if errors.As(err, &pg) {
		return pg.Code == "23505"
	}
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "SQLITE_CONSTRAINT_UNIQUE") || strings.Contains(s, "SQLITE_CONSTRAINT_PRIMARYKEY") || strings.Contains(s, "UNIQUE constraint failed:")
}

// IsUniqueViolationFor matches one logical constraint across both providers.
// SQLite reports the constrained column list rather than PostgreSQL's name.
func IsUniqueViolationFor(err error, postgresConstraint, sqliteColumns string) bool {
	if !IsUniqueViolation(err) {
		return false
	}
	var pg *pq.Error
	if errors.As(err, &pg) {
		return pg.Constraint == postgresConstraint
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed: "+sqliteColumns) ||
		strings.Contains(err.Error(), "index '"+postgresConstraint+"'")
}
