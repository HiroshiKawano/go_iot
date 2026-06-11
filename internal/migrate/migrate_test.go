package migrate

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	infradb "github.com/HiroshiKawano/go_iot/internal/infra/db"
)

// newTestDB は一時ファイル DB を疎通済みで開く (modernc・CGO 不要)。
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dsn := "file:" + filepath.ToSlash(filepath.Join(dir, "app.db"))
	db, err := infradb.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("テスト DB オープン: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// 初回適用で全 7 テーブルが作成される (4.1)。
func TestUp_CreatesAllTables(t *testing.T) {
	db := newTestDB(t)

	if err := Up(db); err != nil {
		t.Fatalf("Up() エラー: %v", err)
	}

	wantTables := []string{
		"users", "devices", "device_tokens", "sensor_readings",
		"alert_rules", "alert_histories", "sessions",
	}
	for _, name := range wantTables {
		if !tableExists(t, db, name) {
			t.Errorf("テーブル %q が作成されていない", name)
		}
	}
}

// 2 回目の適用は no-op で既存データを破壊しない (4.3)。
func TestUp_IdempotentNoOp(t *testing.T) {
	db := newTestDB(t)

	if err := Up(db); err != nil {
		t.Fatalf("1 回目 Up() エラー: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO users (name, email, password_hash) VALUES (?, ?, ?)`,
		"テスト太郎", "test@example.com", "hash",
	); err != nil {
		t.Fatalf("データ投入: %v", err)
	}

	if err := Up(db); err != nil {
		t.Fatalf("2 回目 Up() エラー (no-op であるべき): %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		t.Fatalf("件数取得: %v", err)
	}
	if count != 1 {
		t.Errorf("2 回目 Up() で既存データが変化した: users 件数 = %d, want 1", count)
	}
}

// 未疎通 (閉じた) DB への適用はエラーを返す (4.4 の失敗源)。
func TestUp_ErrorOnUnreachableDB(t *testing.T) {
	db := newTestDB(t)
	_ = db.Close() // 閉じる = 未疎通相当

	if err := Up(db); err == nil {
		t.Error("閉じた(未疎通)DB に対し Up() はエラーを返すべき")
	}
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var got string
	err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name,
	).Scan(&got)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("テーブル存在確認(%s): %v", name, err)
	}
	return got == name
}
