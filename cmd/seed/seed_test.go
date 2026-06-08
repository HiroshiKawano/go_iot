package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	infradb "github.com/HiroshiKawano/go_iot/internal/infra/db"
	"github.com/pressly/goose/v3"
)

// migratedSeedDB は WAL/busy_timeout 付き実 SQLite ファイル(本番同条件の NewPool)へ
// goose(sqlite3)で全マイグレーションを適用した *sql.DB を返す。
// seed は NewPool(SetMaxOpenConns(4)) を使うため :memory: では接続ごと別 DB になり不可 → ファイルを使う。
func migratedSeedDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "seed_test.sqlite")
	db, err := infradb.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, thisFile, _, _ := runtime.Caller(0)
	migDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "db", "migrations") // cmd/seed → リポジトリルート
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("goose.SetDialect: %v", err)
	}
	goose.SetLogger(goose.NopLogger())
	if err := goose.Up(db, migDir); err != nil {
		t.Fatalf("goose.Up: %v", err)
	}
	return db
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// wantCounts は seed 投入後に各テーブルが持つべき行数。
// users 1 / devices 2 / sensor_readings 288*2 / alert_rules 2*2 / alert_histories 3。
var wantCounts = map[string]int{
	"users":           1,
	"devices":         2,
	"sensor_readings": 576,
	"alert_rules":     4,
	"alert_histories": 3,
}

// TestSeedAll_SQLiteへ投入でき冪等にIDが振り直される は、seed の SQLite 対応(8.x 観測可能完了)を実機固定する。
// (1) seedAll が全テーブルへ期待件数を投入できること、
// (2) 再実行(冪等)しても件数が変わらず、truncateAll(DELETE)で id が 1 から振り直される
//     (PostgreSQL の TRUNCATE ... RESTART IDENTITY 等価。AUTOINCREMENT 不使用ゆえ sqlite_sequence 不要)
// を検証する。
func TestSeedAll_SQLiteへ投入でき冪等にIDが振り直される(t *testing.T) {
	db := migratedSeedDB(t)
	ctx := context.Background()

	// 1 回目
	if err := seedAll(ctx, db); err != nil {
		t.Fatalf("seedAll(1回目): %v", err)
	}
	for table, want := range wantCounts {
		if got := countRows(t, db, table); got != want {
			t.Errorf("初回 %s 件数 = %d, want %d", table, got, want)
		}
	}
	if id := firstUserID(t, db); id != 1 {
		t.Errorf("初回 user.id = %d, want 1", id)
	}

	// 2 回目(冪等 + RESTART IDENTITY 相当): DELETE で空化→再投入。件数は不変、id は再び 1 から。
	if err := seedAll(ctx, db); err != nil {
		t.Fatalf("seedAll(2回目): %v", err)
	}
	for table, want := range wantCounts {
		if got := countRows(t, db, table); got != want {
			t.Errorf("再投入後 %s 件数 = %d, want %d (冪等でない)", table, got, want)
		}
	}
	if id := firstUserID(t, db); id != 1 {
		t.Errorf("再投入後 user.id = %d, want 1 (DELETE で id が振り直されていない)", id)
	}
}

func firstUserID(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRowContext(context.Background(), "SELECT MIN(id) FROM users").Scan(&id); err != nil {
		t.Fatalf("select user id: %v", err)
	}
	return id
}
