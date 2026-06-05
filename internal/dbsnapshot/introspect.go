package dbsnapshot

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Querier は内省に必要な最小の DB アクセス機能 (pgxpool.Pool が満たす)。
// テスト時にこの interface を差し替えてモックできる。
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Introspect は public スキーマの全テーブル (goose 管理表を除く) を内省して
// Schema を構築する。テーブル名昇順で安定した出力になる。
func Introspect(ctx context.Context, db Querier) (*Schema, error) {
	tables, err := listTables(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}

	schema := &Schema{Tables: make([]Table, 0, len(tables))}
	for _, t := range tables {
		cols, err := listColumns(ctx, db, t.Name)
		if err != nil {
			return nil, fmt.Errorf("columns(%s): %w", t.Name, err)
		}
		t.Columns = cols

		if err := markPrimaryKeys(ctx, db, &t); err != nil {
			return nil, fmt.Errorf("pk(%s): %w", t.Name, err)
		}

		t.Indexes, err = listIndexes(ctx, db, t.Name)
		if err != nil {
			return nil, fmt.Errorf("indexes(%s): %w", t.Name, err)
		}

		t.Checks, err = listChecks(ctx, db, t.Name)
		if err != nil {
			return nil, fmt.Errorf("checks(%s): %w", t.Name, err)
		}

		schema.Tables = append(schema.Tables, t)
	}
	return schema, nil
}

func listTables(ctx context.Context, db Querier) ([]Table, error) {
	const q = `
		SELECT c.relname, COALESCE(obj_description(c.oid), '')
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public'
		  AND c.relkind = 'r'
		  AND c.relname <> 'goose_db_version'
		ORDER BY c.relname`

	rows, err := db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var t Table
		if err := rows.Scan(&t.Name, &t.Comment); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func listColumns(ctx context.Context, db Querier, table string) ([]Column, error) {
	const q = `
		SELECT a.attname,
		       format_type(a.atttypid, a.atttypmod),
		       a.attnotnull,
		       COALESCE(pg_get_expr(ad.adbin, ad.adrelid), ''),
		       COALESCE(col_description(a.attrelid, a.attnum), '')
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
		WHERE n.nspname = 'public' AND c.relname = $1
		  AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum`

	rows, err := db.Query(ctx, q, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []Column
	for rows.Next() {
		var c Column
		var notNull bool
		if err := rows.Scan(&c.Name, &c.Type, &notNull, &c.Default, &c.Comment); err != nil {
			return nil, err
		}
		c.Nullable = !notNull
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func markPrimaryKeys(ctx context.Context, db Querier, t *Table) error {
	const q = `
		SELECT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		JOIN pg_class c ON c.oid = i.indrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE i.indisprimary AND n.nspname = 'public' AND c.relname = $1`

	rows, err := db.Query(ctx, q, t.Name)
	if err != nil {
		return err
	}
	defer rows.Close()

	pk := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		pk[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range t.Columns {
		if pk[t.Columns[i].Name] {
			t.Columns[i].IsPK = true
		}
	}
	return nil
}

func listIndexes(ctx context.Context, db Querier, table string) ([]Index, error) {
	const q = `
		SELECT indexname, indexdef
		FROM pg_indexes
		WHERE schemaname = 'public' AND tablename = $1
		ORDER BY indexname`

	rows, err := db.Query(ctx, q, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var idxs []Index
	for rows.Next() {
		var idx Index
		if err := rows.Scan(&idx.Name, &idx.Def); err != nil {
			return nil, err
		}
		// 主キー索引 (<table>_pkey) は列の PK マーカーで表現済みのため除外。
		if strings.HasSuffix(idx.Name, "_pkey") {
			continue
		}
		idx.IsUnique = strings.Contains(idx.Def, "UNIQUE INDEX")
		idxs = append(idxs, idx)
	}
	return idxs, rows.Err()
}

func listChecks(ctx context.Context, db Querier, table string) ([]Check, error) {
	const q = `
		SELECT con.conname, pg_get_constraintdef(con.oid)
		FROM pg_constraint con
		JOIN pg_class c ON c.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE con.contype = 'c' AND n.nspname = 'public' AND c.relname = $1
		ORDER BY con.conname`

	rows, err := db.Query(ctx, q, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []Check
	for rows.Next() {
		var ck Check
		if err := rows.Scan(&ck.Name, &ck.Expr); err != nil {
			return nil, err
		}
		checks = append(checks, ck)
	}
	return checks, rows.Err()
}
