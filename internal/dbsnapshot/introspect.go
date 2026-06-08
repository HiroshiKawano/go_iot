package dbsnapshot

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Querier は内省に必要な最小の DB アクセス機能 (*sql.DB が満たす)。
// テスト時にこの interface を差し替えてモックできる。
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Introspect は SQLite の全ユーザーテーブル (goose 管理表・sqlite 内部表を除く) を
// sqlite_master + PRAGMA で内省して Schema を構築する。テーブル名昇順で安定した出力になる。
//
// PostgreSQL の pg_catalog からの最小移植版 (R9.4)。SQLite にはテーブル/カラムコメントが
// 無いため Comment は常に空 (機能縮退・許容)。
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

// listTables は sqlite_master からユーザーテーブルを昇順で取得する。
// goose 管理表と sqlite 内部表 (sqlite_*) は除外する。
func listTables(ctx context.Context, db Querier) ([]Table, error) {
	const q = `
		SELECT name
		FROM sqlite_master
		WHERE type = 'table'
		  AND name <> 'goose_db_version'
		  AND name NOT LIKE 'sqlite_%'
		ORDER BY name`

	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var t Table
		if err := rows.Scan(&t.Name); err != nil {
			return nil, err
		}
		// SQLite はテーブルコメントを持たない (Comment は空のまま)。
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

// listColumns は PRAGMA table_info をテーブル値関数 (pragma_table_info) 経由で取得する。
// PK は同 PRAGMA の pk 列から判定し、PostgreSQL 版の markPrimaryKeys を兼ねる。
func listColumns(ctx context.Context, db Querier, table string) ([]Column, error) {
	const q = `
		SELECT name, type, "notnull", pk, IFNULL(dflt_value, '')
		FROM pragma_table_info(?)
		ORDER BY cid`

	rows, err := db.QueryContext(ctx, q, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []Column
	for rows.Next() {
		var (
			c              Column
			notNull, pkPos int
		)
		if err := rows.Scan(&c.Name, &c.Type, &notNull, &pkPos, &c.Default); err != nil {
			return nil, err
		}
		c.IsPK = pkPos > 0
		// notnull=0 でも PK 構成列は NULL 不可 (rowid PK 含む)。
		c.Nullable = notNull == 0 && !c.IsPK
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// listIndexes は sqlite_master から明示的に作成された索引 (CREATE [UNIQUE] INDEX) を取得する。
// UNIQUE/PK 制約由来の自動索引 (sqlite_autoindex_*) は sql が NULL のため自然に除外される。
func listIndexes(ctx context.Context, db Querier, table string) ([]Index, error) {
	const q = `
		SELECT name, sql
		FROM sqlite_master
		WHERE type = 'index' AND tbl_name = ? AND sql IS NOT NULL
		ORDER BY name`

	rows, err := db.QueryContext(ctx, q, table)
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
		idx.IsUnique = strings.Contains(strings.ToUpper(idx.Def), "UNIQUE")
		idxs = append(idxs, idx)
	}
	return idxs, rows.Err()
}

// listChecks はテーブルの CREATE 文 (sqlite_master.sql) から CHECK 制約を抽出する。
// SQLite には CHECK 専用のカタログ/PRAGMA が無いため DDL テキストをパースする。
// enum 的な許容値 (metric/operator の IN リスト) を保全することが目的。
func listChecks(ctx context.Context, db Querier, table string) ([]Check, error) {
	const q = `SELECT IFNULL(sql, '') FROM sqlite_master WHERE type = 'table' AND name = ?`

	rows, err := db.QueryContext(ctx, q, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var createSQL string
	if rows.Next() {
		if err := rows.Scan(&createSQL); err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return parseCheckConstraints(createSQL), nil
}

// parseCheckConstraints は CREATE TABLE 文から `CONSTRAINT <name> CHECK (<expr>)` を抽出する。
// 本プロジェクトの CHECK は全て名前付き制約として宣言されている。UNIQUE/PRIMARY KEY 等の
// 別種 CONSTRAINT はスキップする。文字列リテラル内の括弧は深さ計算から除外する。
//
// 前提と限界 (本プロジェクトの DDL では成立): 制約名は引用なし識別子であること
// (`CONSTRAINT "spaced name"` のような引用識別子は名前境界の判定が崩れ脱落しうる)、
// および文字列リテラル/コメント内に `CONSTRAINT ... CHECK` キーワード列が現れないこと
// (トークナイザではなくキーワード探索のため)。完全な SQL パースは過剰なため採用しない。
func parseCheckConstraints(createSQL string) []Check {
	var checks []Check
	s := createSQL
	upper := strings.ToUpper(s) // ASCII DDL 前提 (バイト長は ToUpper で不変)

	for {
		rel := strings.Index(upper, "CONSTRAINT")
		if rel < 0 {
			break
		}
		// "CONSTRAINT" の次のトークン (制約名) を読む。
		after := s[rel+len("CONSTRAINT"):]
		trimmed := strings.TrimLeft(after, " \t\r\n")
		lead := len(after) - len(trimmed)
		nameEnd := strings.IndexAny(trimmed, " \t\r\n(")
		if nameEnd < 0 {
			break
		}
		name := trimmed[:nameEnd]
		rest := strings.TrimLeft(trimmed[nameEnd:], " \t\r\n")

		if !strings.HasPrefix(strings.ToUpper(rest), "CHECK") {
			// CHECK 以外の制約 (UNIQUE/PRIMARY KEY/FOREIGN KEY 等) は対象外。次の CONSTRAINT へ。
			advance := rel + len("CONSTRAINT") + lead + nameEnd
			s = s[advance:]
			upper = upper[advance:]
			continue
		}

		exprSrc := strings.TrimLeft(rest[len("CHECK"):], " \t\r\n")
		expr, consumed, ok := extractBalancedParens(exprSrc)
		if !ok {
			advance := rel + len("CONSTRAINT") + lead + nameEnd
			s = s[advance:]
			upper = upper[advance:]
			continue
		}
		checks = append(checks, Check{Name: name, Expr: "CHECK (" + strings.TrimSpace(expr) + ")"})

		// 抽出済み CHECK 式の末尾以降へ前進する。
		consumedTotal := len(s) - len(exprSrc) + consumed
		s = s[consumedTotal:]
		upper = upper[consumedTotal:]
	}
	return checks
}

// extractBalancedParens は先頭が '(' の文字列から対応する ')' までの中身 (外側括弧を除く) と
// 消費バイト数を返す。単一引用符の文字列リテラル内の括弧は深さ計算から除外する。
func extractBalancedParens(s string) (inner string, consumed int, ok bool) {
	if len(s) == 0 || s[0] != '(' {
		return "", 0, false
	}
	depth := 0
	inStr := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inStr {
			if ch == '\'' {
				inStr = false
			}
			continue
		}
		switch ch {
		case '\'':
			inStr = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[1:i], i + 1, true
			}
		}
	}
	return "", 0, false
}
