package main

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/migrate"
)

// 正常系: DB オープン直後にマイグレーションが適用され、後続 (listener 取得) へ到達できる。
func TestOpenAndMigrate_AppliesMigrations(t *testing.T) {
	dir := t.TempDir()
	dsn := "file:" + filepath.ToSlash(filepath.Join(dir, "app.db"))

	pool, err := openAndMigrate(context.Background(), dsn, migrate.Up)
	if err != nil {
		t.Fatalf("openAndMigrate() エラー: %v", err)
	}
	defer pool.Close()

	// マイグレーション適用済み = users テーブルが存在する
	var name string
	if err := pool.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='users'`,
	).Scan(&name); err != nil {
		t.Errorf("マイグレーションが適用されていない (users テーブル無し): %v", err)
	}
}

// 異常系: DB オープン失敗時はエラーを返し、マイグレーションを実行しない (順序保証)。
func TestOpenAndMigrate_ErrorOnDBOpenFailure(t *testing.T) {
	// 存在しない DB ファイルを読み取り専用で開く → オープン (ping) が失敗する
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "nonexistent.db")) + "?mode=ro"
	migrateCalled := false

	pool, err := openAndMigrate(context.Background(), dsn, func(*sql.DB) error {
		migrateCalled = true
		return nil
	})
	if err == nil {
		if pool != nil {
			_ = pool.Close()
		}
		t.Fatal("DB オープン失敗時は openAndMigrate() がエラーを返すべき")
	}
	if migrateCalled {
		t.Error("DB オープン失敗時はマイグレーションを実行すべきでない")
	}
}

// 異常系: マイグレーション失敗 (未疎通 DB 相当) で起動シーケンスが中断する (R4.4 fail fast)。
func TestOpenAndMigrate_FailFastOnMigrationError(t *testing.T) {
	dir := t.TempDir()
	dsn := "file:" + filepath.ToSlash(filepath.Join(dir, "app.db"))
	wantErr := errors.New("マイグレーション失敗")

	pool, err := openAndMigrate(context.Background(), dsn, func(*sql.DB) error {
		return wantErr
	})
	if err == nil {
		t.Fatal("マイグレーション失敗時は openAndMigrate() がエラーを返すべき (fail fast)")
	}
	if pool != nil {
		t.Error("マイグレーション失敗時は pool を返さない (閉じる) べき")
	}
}
