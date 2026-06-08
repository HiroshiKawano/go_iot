package db_test

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
	// SQLite ドライバ "sqlite" は同パッケージの pool.go が blank import 済み(driver 登録はグローバル)。
)

// migrationsDir は本テストファイルの位置を基準に db/migrations の絶対パスを解決する。
// テスト実行時のカレントディレクトリに依存しないようにするための補助。
func migrationsDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller に失敗しました")
	}
	// internal/infra/db/ から リポジトリルートの db/migrations へ遡る。
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "db", "migrations")
}

// freshMigratedDB は空のインメモリ SQLite に goose(dialect=sqlite3)で全マイグレーションを
// 適用した *sql.DB を返す。SQLite 移行の各検証テストの共通土台。
func freshMigratedDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("goose.SetDialect(sqlite3): %v", err)
	}
	goose.SetLogger(goose.NopLogger())

	if err := goose.Up(database, migrationsDir(t)); err != nil {
		t.Fatalf("goose.Up(SQLite): %v", err)
	}
	return database
}

// TestMigrations_SQLite方言で全7本が空DBへ適用できる は、SQLite 移行の Foundation
// ゲート(タスク1.1)の観測可能完了条件を検証する。
// goose(dialect=sqlite3)で空のインメモリ SQLite へ全マイグレーションを適用し、
// 全7テーブルと全索引が作成されることを sqlite_master 走査で確認する。
//
// PostgreSQL 方言のままだと BIGSERIAL/TIMESTAMPTZ/JSONB/BYTEA/COMMENT ON/正規表現 CHECK
// 等が modernc.org/sqlite で構文・型エラーになるため、書換前は RED となる。
func TestMigrations_SQLite方言で全7本が空DBへ適用できる(t *testing.T) {
	database := freshMigratedDB(t)

	wantTables := []string{
		"users",
		"devices",
		"device_tokens",
		"sensor_readings",
		"alert_rules",
		"alert_histories",
		"sessions",
	}
	for _, tbl := range wantTables {
		if !objectExists(t, database, "table", tbl) {
			t.Errorf("テーブル %q が作成されていません", tbl)
		}
	}

	// 部分INDEX・DESC複合INDEX・UNIQUEなど、現行スキーマの索引がすべて移植されていること。
	wantIndexes := []string{
		"users_email_unique",
		"devices_mac_address_unique_active",
		"devices_user_id_idx",
		"devices_is_active_idx",
		"device_tokens_token_hash_unique",
		"device_tokens_user_id_idx",
		"sensor_readings_device_id_recorded_at_idx",
		"sensor_readings_recorded_at_idx",
		"alert_rules_device_id_is_enabled_idx",
		"alert_histories_alert_rule_id_idx",
		"alert_histories_triggered_at_idx",
		"alert_histories_unnotified_idx",
		"sessions_expiry_idx",
	}
	for _, idx := range wantIndexes {
		if !objectExists(t, database, "index", idx) {
			t.Errorf("索引 %q が作成されていません", idx)
		}
	}
}

// TestMigrations_部分INDEXとDESC索引が維持される は、tasks.md 1.1 の要件
// 「部分INDEX(WHERE deleted_at IS NULL 等)・DESC複合INDEX は維持」を、索引名の存在だけでなく
// DDL 属性(部分条件・DESC・UNIQUE)のレベルで回帰固定する。方言書換で属性を取りこぼすと検知する。
func TestMigrations_部分INDEXとDESC索引が維持される(t *testing.T) {
	database := freshMigratedDB(t)

	cases := []struct {
		index    string
		contains []string
	}{
		{"devices_mac_address_unique_active", []string{"UNIQUE", "WHERE", "deleted_at IS NULL"}},
		{"devices_user_id_idx", []string{"WHERE", "deleted_at IS NULL"}},
		{"devices_is_active_idx", []string{"WHERE", "deleted_at IS NULL"}},
		{"sensor_readings_device_id_recorded_at_idx", []string{"recorded_at DESC", "WHERE", "deleted_at IS NULL"}},
		{"sensor_readings_recorded_at_idx", []string{"recorded_at DESC", "WHERE", "deleted_at IS NULL"}},
		{"alert_rules_device_id_is_enabled_idx", []string{"WHERE", "deleted_at IS NULL"}},
		{"alert_histories_triggered_at_idx", []string{"triggered_at DESC", "WHERE", "deleted_at IS NULL"}},
		{"alert_histories_unnotified_idx", []string{"is_notified = FALSE", "deleted_at IS NULL"}},
	}
	for _, c := range cases {
		var ddl string
		err := database.QueryRow(
			`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?`, c.index,
		).Scan(&ddl)
		if err != nil {
			t.Errorf("索引 %q の DDL 取得に失敗: %v", c.index, err)
			continue
		}
		for _, want := range c.contains {
			if !strings.Contains(ddl, want) {
				t.Errorf("索引 %q が属性 %q を失っています(方言書換での喪失): %s", c.index, want, ddl)
			}
		}
	}
}

// TestMigrations_CHECK制約が維持される は、tasks.md 1.1 の要件「BETWEEN/IN CHECK は維持」を
// 違反 INSERT が拒否されることで実機確認する。方言書換で CHECK を取りこぼすと検知する。
// (devices の正規表現 CHECK のみアプリ層へ委譲=削除済みのため対象外)
func TestMigrations_CHECK制約が維持される(t *testing.T) {
	database := freshMigratedDB(t)

	// sensor_readings.temperature の範囲 CHECK (BETWEEN -40 AND 125)
	if _, err := database.Exec(
		`INSERT INTO sensor_readings (device_id, temperature, humidity, recorded_at)
		 VALUES (1, 200, 50, '2026-06-01 00:00:00')`,
	); err == nil {
		t.Error("temperature=200 は範囲 CHECK 違反だが INSERT が通りました(CHECK 喪失)")
	}

	// alert_rules.operator の IN CHECK ('>', '<', '>=', '<=')
	if _, err := database.Exec(
		`INSERT INTO alert_rules (device_id, metric, operator, threshold)
		 VALUES (1, 'temperature', '!=', 30)`,
	); err == nil {
		t.Error("operator='!=' は IN CHECK 違反だが INSERT が通りました(CHECK 喪失)")
	}

	// alert_rules.metric の IN CHECK ('temperature', 'humidity')
	if _, err := database.Exec(
		`INSERT INTO alert_rules (device_id, metric, operator, threshold)
		 VALUES (1, 'pressure', '>', 30)`,
	); err == nil {
		t.Error("metric='pressure' は IN CHECK 違反だが INSERT が通りました(CHECK 喪失)")
	}
}

// objectExists は sqlite_master に指定 type/name のオブジェクトが存在するかを返す。
func objectExists(t *testing.T, database *sql.DB, objType, name string) bool {
	t.Helper()
	var found string
	err := database.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = ? AND name = ?`,
		objType, name,
	).Scan(&found)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("sqlite_master 走査(%s %s): %v", objType, name, err)
	}
	return found == name
}
