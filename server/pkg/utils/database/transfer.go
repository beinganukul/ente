//go:build !js

package database

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
)

// Snapshots contain private account information. Stop every Museum instance
// before exporting/importing; database snapshots cannot stop external writers.
type transferColumn struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Identity bool   `json:"identity,omitempty"`
}
type transferTable struct {
	Name      string           `json:"name"`
	Columns   []transferColumn `json:"columns"`
	Parents   []string         `json:"parents,omitempty"`
	HighWater int64            `json:"highWater,string,omitempty"`
}
type transferCell struct {
	Kind  string `json:"kind,omitempty"`
	Value string `json:"value,omitempty"`
	Null  bool   `json:"null,omitempty"`
}
type transferRecord struct {
	Format  string            `json:"format,omitempty"`
	Version int64             `json:"version,omitempty"`
	Tables  []transferTable   `json:"tables,omitempty"`
	Table   string            `json:"table,omitempty"`
	Row     []transferCell    `json:"row,omitempty"`
	Counts  map[string]int64  `json:"counts,omitempty"`
	Digests map[string]string `json:"digests,omitempty"`
}
type transferDigest struct {
	count int64
	sum   [32]byte
}

func (d *transferDigest) add(row []transferCell) error {
	data, err := json.Marshal(row)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	carry := 0
	for i := len(hash) - 1; i >= 0; i-- {
		carry += int(d.sum[i]) + int(hash[i])
		d.sum[i] = byte(carry)
		carry >>= 8
	}
	d.count++
	return nil
}
func (d transferDigest) value() string { return base64.StdEncoding.EncodeToString(d.sum[:]) }
func transferQuote(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

type transferQuery interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func transferSchema(ctx context.Context, q transferQuery, provider Provider) ([]transferTable, int64, error) {
	var version int64
	var dirty bool
	if err := q.QueryRowContext(ctx, "SELECT version,dirty FROM schema_migrations").Scan(&version, &dirty); err != nil {
		return nil, 0, err
	}
	if dirty {
		return nil, 0, errors.New("cannot transfer a dirty schema")
	}
	var names []string
	query := `SELECT table_name FROM information_schema.tables WHERE table_schema=current_schema() AND table_type='BASE TABLE' AND table_name NOT IN ('schema_migrations','museum_migration_lock') ORDER BY table_name`
	if provider == Turso {
		query = `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN ('schema_migrations','museum_migration_lock') ORDER BY name`
	}
	rows, err := q.QueryContext(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return nil, 0, err
		}
		names = append(names, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, 0, err
	}
	var tables []transferTable
	for _, name := range names {
		t := transferTable{Name: name}
		if provider == Postgres {
			rows, err = q.QueryContext(ctx, `SELECT column_name,data_type,is_identity,COALESCE(column_default,'') FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=$1 AND is_generated='NEVER' ORDER BY ordinal_position`, name)
			if err != nil {
				return nil, 0, err
			}
			for rows.Next() {
				var c transferColumn
				var typ, id, def string
				if err = rows.Scan(&c.Name, &typ, &id, &def); err != nil {
					break
				}
				c.Kind = "text"
				c.Identity = id == "YES" || strings.HasPrefix(def, "nextval(")
				switch typ {
				case "bytea":
					c.Kind = "binary"
				case "boolean":
					c.Kind = "bool"
				case "ARRAY":
					c.Kind = "array"
				case "json", "jsonb":
					c.Kind = "json"
				case "bigint", "integer", "smallint":
					c.Kind = "int"
				case "real", "double precision":
					c.Kind = "float"
				case "timestamp without time zone":
					c.Kind = "timestamp"
				case "numeric", "timestamp with time zone":
					err = fmt.Errorf("unsupported lossless transfer type %s.%s: %s", name, c.Name, typ)
				}
				if err != nil {
					break
				}
				t.Columns = append(t.Columns, c)
			}
			if err == nil {
				err = rows.Err()
			}
			rows.Close()
			if err != nil {
				return nil, 0, err
			}
			rows, err = q.QueryContext(ctx, `SELECT DISTINCT ccu.table_name FROM information_schema.table_constraints tc JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name=tc.constraint_name AND ccu.constraint_schema=tc.constraint_schema WHERE tc.constraint_type='FOREIGN KEY' AND tc.table_schema=current_schema() AND tc.table_name=$1`, name)
		} else {
			var ddl string
			if err = q.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&ddl); err != nil {
				return nil, 0, err
			}
			rows, err = q.QueryContext(ctx, "PRAGMA table_xinfo("+transferQuote(name)+")")
			if err != nil {
				return nil, 0, err
			}
			for rows.Next() {
				var c transferColumn
				var cid, notnull, pk, hidden int
				var typ string
				var def any
				if err = rows.Scan(&cid, &c.Name, &typ, &notnull, &def, &pk, &hidden); err != nil {
					break
				}
				if hidden != 0 {
					continue
				}
				c.Kind = "text"
				line := ""
				for _, candidate := range strings.Split(ddl, "\n") {
					if strings.HasPrefix(strings.TrimSpace(candidate), transferQuote(c.Name)+" ") {
						line = candidate
						break
					}
				}
				switch {
				case strings.Contains(line, "DEFAULT (CURRENT_TIMESTAMP)"):
					c.Kind = "timestamp"
				case strings.Contains(line, "IN (0, 1)"):
					c.Kind = "bool"
				case strings.Contains(line, "= 'array'"):
					c.Kind = "array"
				case strings.Contains(line, "json_valid"):
					c.Kind = "json"
				case strings.EqualFold(typ, "BLOB"):
					c.Kind = "binary"
				case strings.Contains(strings.ToUpper(typ), "INT"):
					c.Kind = "int"
				}
				c.Identity = pk > 0 && strings.EqualFold(typ, "INTEGER") && strings.Contains(line, "AUTOINCREMENT")
				t.Columns = append(t.Columns, c)
			}
			if err == nil {
				err = rows.Err()
			}
			rows.Close()
			if err != nil {
				return nil, 0, err
			}
			rows, err = q.QueryContext(ctx, "PRAGMA foreign_key_list("+transferQuote(name)+")")
		}
		if err != nil {
			return nil, 0, err
		}
		for rows.Next() {
			var parent string
			if provider == Postgres {
				err = rows.Scan(&parent)
			} else {
				var id, seq int
				var from, to, update, del, match string
				err = rows.Scan(&id, &seq, &parent, &from, &to, &update, &del, &match)
			}
			if err != nil {
				break
			}
			if parent != name {
				t.Parents = append(t.Parents, parent)
			}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return nil, 0, err
		}
		// These insertion triggers enforce scope beyond declared foreign keys.
		if name == "reactions" {
			t.Parents = append(t.Parents, "comments")
		}
		for _, c := range t.Columns {
			if !c.Identity {
				continue
			}
			if provider == Postgres {
				var seq string
				if err = q.QueryRowContext(ctx, "SELECT pg_get_serial_sequence($1,$2)", name, c.Name).Scan(&seq); err != nil {
					return nil, 0, err
				}
				var called bool
				if err = q.QueryRowContext(ctx, "SELECT last_value,is_called FROM "+seq).Scan(&t.HighWater, &called); err != nil {
					return nil, 0, err
				}
				if !called {
					t.HighWater--
				}
			} else {
				err = q.QueryRowContext(ctx, "SELECT seq FROM sqlite_sequence WHERE name=?", name).Scan(&t.HighWater)
				if errors.Is(err, sql.ErrNoRows) {
					err = nil
				}
				if err != nil {
					return nil, 0, err
				}
			}
		}
		tables = append(tables, t)
	}
	// Reject dependency cycles; never disable constraints to force an import.
	ordered := make([]transferTable, 0, len(tables))
	done := map[string]bool{}
	for len(ordered) < len(tables) {
		progress := false
		for _, t := range tables {
			if done[t.Name] {
				continue
			}
			ready := true
			for _, p := range t.Parents {
				if !done[p] {
					ready = false
				}
			}
			if ready {
				ordered = append(ordered, t)
				done[t.Name] = true
				progress = true
			}
		}
		if !progress {
			return nil, 0, errors.New("schema contains an unsupported foreign-key dependency cycle")
		}
	}
	return ordered, version, nil
}

func transferCells(values []any, columns []transferColumn, provider Provider) ([]transferCell, error) {
	cells := make([]transferCell, len(values))
	for i, value := range values {
		cell := transferCell{Kind: columns[i].Kind}
		if value == nil {
			cells[i] = transferCell{Null: true}
			continue
		}
		var text string
		switch v := value.(type) {
		case []byte:
			text = string(v)
		case string:
			text = v
		case int64:
			text = strconv.FormatInt(v, 10)
		case float64:
			text = strconv.FormatFloat(v, 'g', -1, 64)
		case bool:
			text = strconv.FormatBool(v)
		case time.Time:
			text = v.Format("2006-01-02 15:04:05.999999")
		default:
			return nil, fmt.Errorf("unsupported transfer value %T", value)
		}
		switch cell.Kind {
		case "timestamp":
			parsed, err := time.Parse("2006-01-02 15:04:05.999999", text)
			if err != nil {
				return nil, fmt.Errorf("unsupported timestamp %s: %w", columns[i].Name, err)
			}
			text = parsed.Format("2006-01-02 15:04:05.999999")
		case "binary":
			bytes, ok := value.([]byte)
			if !ok {
				return nil, fmt.Errorf("expected binary column %s", columns[i].Name)
			}
			text = base64.StdEncoding.EncodeToString(bytes)
		case "bool":
			if text == "1" {
				text = "true"
			}
			if text == "0" {
				text = "false"
			}
			if text != "true" && text != "false" {
				return nil, errors.New("invalid boolean")
			}
		case "array":
			if provider == Postgres {
				var array []sql.NullString
				if err := pq.Array(&array).Scan(value); err != nil {
					return nil, err
				}
				items := make([]any, len(array))
				for j, v := range array {
					if v.Valid {
						items[j] = v.String
					}
				}
				encoded, err := json.Marshal(items)
				if err != nil {
					return nil, err
				}
				text = string(encoded)
			}
			fallthrough
		case "json":
			var decoded any
			decoder := json.NewDecoder(strings.NewReader(text))
			decoder.UseNumber()
			if err := decoder.Decode(&decoded); err != nil {
				return nil, err
			}
			encoded, err := json.Marshal(decoded)
			if err != nil {
				return nil, err
			}
			text = string(encoded)
		}
		cell.Value = text
		cells[i] = cell
	}
	return cells, nil
}

func transferRows(ctx context.Context, q transferQuery, t transferTable, offset int64) (*sql.Rows, error) {
	cols := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		cols[i] = transferQuote(c.Name)
	}
	query := "SELECT " + strings.Join(cols, ",") + " FROM " + transferQuote(t.Name) + " ORDER BY " + strings.Join(cols, ",")
	if t.Name == "comments" {
		// A scope-validation trigger requires each parent to be present first.
		qualified := make([]string, len(cols))
		for i, c := range cols {
			qualified[i] = "c." + c
		}
		query = `WITH RECURSIVE tree(id,depth) AS (SELECT id,0 FROM comments WHERE parent_comment_id IS NULL UNION ALL SELECT c.id,t.depth+1 FROM comments c JOIN tree t ON c.parent_comment_id=t.id) SELECT ` + strings.Join(qualified, ",") + ` FROM comments c JOIN tree ON tree.id=c.id ORDER BY tree.depth,c.id`
	}
	// Hrana drivers buffer each response; bound it to a page of rows.
	return q.QueryContext(ctx, query+" LIMIT 256 OFFSET "+strconv.FormatInt(offset, 10))
}

func transferWalk(ctx context.Context, q transferQuery, t transferTable, provider Provider, visit func([]transferCell) error) error {
	for offset := int64(0); ; offset += 256 {
		rows, err := transferRows(ctx, q, t, offset)
		if err != nil {
			return err
		}
		count := 0
		for rows.Next() {
			values := make([]any, len(t.Columns))
			pointers := make([]any, len(values))
			for i := range values {
				pointers[i] = &values[i]
			}
			if err = rows.Scan(pointers...); err != nil {
				break
			}
			cells, cellErr := transferCells(values, t.Columns, provider)
			if cellErr != nil {
				err = cellErr
				break
			}
			if err = visit(cells); err != nil {
				break
			}
			count++
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return err
		}
		if count < 256 {
			return nil
		}
	}
}

// ExportSnapshot creates a new private file and streams a consistent snapshot.
// Existing files are never overwritten. A failed export lacks its final footer.
func ExportSnapshot(ctx context.Context, cfg Config, path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	provider, err := cfg.EffectiveProvider()
	if err != nil {
		return err
	}
	db, err := Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	options := &sql.TxOptions{}
	if provider == Postgres {
		options.Isolation = sql.LevelRepeatableRead
		options.ReadOnly = true
	}
	tx, err := db.BeginTx(ctx, options)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tables, version, err := transferSchema(ctx, tx, provider)
	if err != nil {
		return err
	}
	buffer := bufio.NewWriter(file)
	encoder := json.NewEncoder(buffer)
	if err = encoder.Encode(transferRecord{Format: "ente-database-v1", Version: version, Tables: tables}); err != nil {
		return err
	}
	counts := map[string]int64{}
	digests := map[string]string{}
	for _, t := range tables {
		digest := transferDigest{}
		err = transferWalk(ctx, tx, t, provider, func(cells []transferCell) error {
			if err := digest.add(cells); err != nil {
				return err
			}
			return encoder.Encode(transferRecord{Table: t.Name, Row: cells})
		})
		if err != nil {
			return fmt.Errorf("export %s: %w", t.Name, err)
		}
		var expected int64
		if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+transferQuote(t.Name)).Scan(&expected); err != nil {
			return err
		}
		if expected != digest.count {
			return fmt.Errorf("%s contains unreachable parent rows; export refused", t.Name)
		}
		counts[t.Name] = digest.count
		digests[t.Name] = digest.value()
	}
	if err = encoder.Encode(transferRecord{Counts: counts, Digests: digests}); err != nil {
		return err
	}
	if err = buffer.Flush(); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	return tx.Commit()
}

func transferArgs(row []transferCell, columns []transferColumn, provider Provider) ([]any, error) {
	if len(row) != len(columns) {
		return nil, errors.New("snapshot column count mismatch")
	}
	args := make([]any, len(row))
	for i, c := range row {
		if c.Null {
			continue
		}
		if c.Kind != columns[i].Kind {
			return nil, fmt.Errorf("snapshot type mismatch for %s", columns[i].Name)
		}
		switch c.Kind {
		case "binary":
			value, err := base64.StdEncoding.DecodeString(c.Value)
			if err != nil {
				return nil, err
			}
			args[i] = value
		case "int":
			value, err := strconv.ParseInt(c.Value, 10, 64)
			if err != nil {
				return nil, err
			}
			args[i] = value
		case "float":
			value, err := strconv.ParseFloat(c.Value, 64)
			if err != nil {
				return nil, err
			}
			args[i] = value
		case "bool":
			value, err := strconv.ParseBool(c.Value)
			if err != nil {
				return nil, err
			}
			if provider == Turso {
				args[i] = int64(0)
				if value {
					args[i] = int64(1)
				}
			} else {
				args[i] = value
			}
		case "array":
			if provider == Postgres {
				var values []*string
				if err := json.Unmarshal([]byte(c.Value), &values); err != nil {
					return nil, err
				}
				array := make([]sql.NullString, len(values))
				for j, value := range values {
					if value != nil {
						array[j] = sql.NullString{String: *value, Valid: true}
					}
				}
				args[i] = pq.Array(array)
			} else {
				args[i] = c.Value
			}
		default:
			args[i] = c.Value
		}
	}
	return args, nil
}

// ImportSnapshot only writes to an empty, clean, already-migrated target.
// All rows and data checksums are verified before committing. Constraints and
// application triggers stay enabled; an unsupported dataset is rolled back.
func ImportSnapshot(ctx context.Context, cfg Config, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return errors.New("snapshot must be a regular private file with mode 0600")
	}
	decoder := json.NewDecoder(bufio.NewReader(file))
	var header transferRecord
	if err = decoder.Decode(&header); err != nil {
		return err
	}
	if header.Format != "ente-database-v1" || len(header.Tables) == 0 {
		return errors.New("unsupported or empty snapshot header")
	}
	provider, err := cfg.EffectiveProvider()
	if err != nil {
		return err
	}
	db, err := Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tables, version, err := transferSchema(ctx, tx, provider)
	if err != nil {
		return err
	}
	if version != header.Version {
		return fmt.Errorf("schema versions differ: source %d, target %d", header.Version, version)
	}
	if len(tables) != len(header.Tables) {
		sourceNames, targetNames := map[string]bool{}, map[string]bool{}
		for _, table := range header.Tables {
			sourceNames[table.Name] = true
		}
		for _, table := range tables {
			targetNames[table.Name] = true
		}
		var sourceOnly, targetOnly []string
		for _, table := range header.Tables {
			if !targetNames[table.Name] {
				sourceOnly = append(sourceOnly, table.Name)
			}
		}
		for _, table := range tables {
			if !sourceNames[table.Name] {
				targetOnly = append(targetOnly, table.Name)
			}
		}
		return fmt.Errorf("snapshot table set differs from target: source-only %v, target-only %v", sourceOnly, targetOnly)
	}
	byName := map[string]transferTable{}
	for _, t := range tables {
		byName[t.Name] = t
		if provider == Postgres {
			// An accidental writer cannot invalidate the empty-target check or
			// insert unverified rows between our final checksum and commit.
			if _, err = tx.ExecContext(ctx, "LOCK TABLE "+transferQuote(t.Name)+" IN ACCESS EXCLUSIVE MODE"); err != nil {
				return err
			}
		}
		var count int64
		if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+transferQuote(t.Name)).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("target table %s is not empty", t.Name)
		}
	}
	for _, source := range header.Tables {
		target, ok := byName[source.Name]
		if !ok || len(target.Columns) != len(source.Columns) {
			return fmt.Errorf("schema differs for table %s", source.Name)
		}
		for i, c := range source.Columns {
			if c.Name != target.Columns[i].Name || c.Kind != target.Columns[i].Kind {
				return fmt.Errorf("schema differs for %s.%s", source.Name, c.Name)
			}
		}
	}
	var footer transferRecord
	seen := map[string]*transferDigest{}
	for {
		var record transferRecord
		if err = decoder.Decode(&record); err != nil {
			return fmt.Errorf("snapshot is truncated: %w", err)
		}
		if record.Counts != nil {
			footer = record
			break
		}
		t, ok := byName[record.Table]
		if !ok {
			return errors.New("snapshot references unknown table")
		}
		args, err := transferArgs(record.Row, t.Columns, provider)
		if err != nil {
			return err
		}
		cols := make([]string, len(t.Columns))
		params := make([]string, len(t.Columns))
		for i, c := range t.Columns {
			cols[i] = transferQuote(c.Name)
			params[i] = "$" + strconv.Itoa(i+1)
		}
		override := ""
		if provider == Postgres {
			override = " OVERRIDING SYSTEM VALUE"
		}
		query := "INSERT INTO " + transferQuote(t.Name) + " (" + strings.Join(cols, ",") + ")" + override + " VALUES (" + strings.Join(params, ",") + ")"
		if _, err = tx.ExecContext(ctx, query, args...); err != nil {
			return fmt.Errorf("import %s (no data committed): %w", t.Name, err)
		}
		if seen[t.Name] == nil {
			seen[t.Name] = &transferDigest{}
		}
		if err = seen[t.Name].add(record.Row); err != nil {
			return err
		}
	}
	var extra transferRecord
	if err = decoder.Decode(&extra); err != io.EOF {
		return errors.New("snapshot has data after its footer")
	}
	if len(footer.Counts) != len(tables) || len(footer.Digests) != len(tables) {
		return errors.New("snapshot footer table set mismatch")
	}
	for _, t := range tables {
		digest := seen[t.Name]
		if digest == nil {
			digest = &transferDigest{}
		}
		if footer.Counts[t.Name] != digest.count || footer.Digests[t.Name] != digest.value() {
			return fmt.Errorf("snapshot checksum failed for %s", t.Name)
		}
		actual := transferDigest{}
		err = transferWalk(ctx, tx, t, provider, actual.add)
		if err != nil {
			return err
		}
		if actual.count != digest.count || actual.value() != digest.value() {
			return fmt.Errorf("target triggers changed %s data; import rolled back", t.Name)
		}
	}
	if provider == Turso {
		rows, err := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
		if err != nil {
			return err
		}
		bad := rows.Next()
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if bad {
			return errors.New("target foreign-key validation failed")
		}
	}
	for _, source := range header.Tables {
		target := byName[source.Name]
		for _, c := range target.Columns {
			if !c.Identity {
				continue
			}
			high := max(source.HighWater, target.HighWater)
			var maximum sql.NullInt64
			if err = tx.QueryRowContext(ctx, "SELECT MAX("+transferQuote(c.Name)+") FROM "+transferQuote(target.Name)).Scan(&maximum); err != nil {
				return err
			}
			if maximum.Valid {
				high = max(high, maximum.Int64)
			}
			if provider == Postgres {
				if _, err = tx.ExecContext(ctx, "SELECT setval(pg_get_serial_sequence($1,$2),$3,$4)", target.Name, c.Name, max(high, 1), high > 0); err != nil {
					return err
				}
			} else {
				if _, err = tx.ExecContext(ctx, "DELETE FROM sqlite_sequence WHERE name=?", target.Name); err != nil {
					return err
				}
				if _, err = tx.ExecContext(ctx, "INSERT INTO sqlite_sequence(name,seq) VALUES (?,?)", target.Name, high); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}
