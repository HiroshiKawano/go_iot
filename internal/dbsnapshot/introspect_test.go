package dbsnapshot

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// freshMigratedDB は空インメモリ SQLite に goose(sqlite3)で全マイグレーションを適用した *sql.DB を返す。
func freshMigratedDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, thisFile, _, _ := runtime.Caller(0)
	migDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "db", "migrations") // internal/dbsnapshot → リポジトリルート
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("goose.SetDialect: %v", err)
	}
	goose.SetLogger(goose.NopLogger())
	if err := goose.Up(db, migDir); err != nil {
		t.Fatalf("goose.Up: %v", err)
	}
	return db
}

func tableByName(s *Schema, name string) *Table {
	for i := range s.Tables {
		if s.Tables[i].Name == name {
			return &s.Tables[i]
		}
	}
	return nil
}

func colByName(t *Table, name string) *Column {
	for i := range t.Columns {
		if t.Columns[i].Name == name {
			return &t.Columns[i]
		}
	}
	return nil
}

// TestIntrospect_SQLiteスキーマを内省する は、dbsnapshot を pg_catalog→sqlite_master+PRAGMA へ移植した
// Introspect が、実マイグレーション適用済み SQLite から全テーブル・カラム・PK・索引・CHECK を取得できることを検証する(R9.4)。
func TestIntrospect_SQLiteスキーマを内省する(t *testing.T) {
	db := freshMigratedDB(t)
	schema, err := Introspect(context.Background(), db)
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}

	// goose_db_version を除く全7テーブルが昇順で取得される
	want := []string{
		"alert_histories", "alert_rules", "device_tokens", "devices",
		"sensor_readings", "sessions", "users",
	}
	if len(schema.Tables) != len(want) {
		t.Fatalf("テーブル数 = %d, want %d (%v)", len(schema.Tables), len(want), tableNames(schema))
	}
	for i, w := range want {
		if schema.Tables[i].Name != w {
			t.Errorf("Tables[%d] = %q, want %q (昇順/除外が不正)", i, schema.Tables[i].Name, w)
		}
	}
	if tableByName(schema, "goose_db_version") != nil {
		t.Error("goose_db_version が内省結果に混入している (除外漏れ)")
	}

	// users.id は PK・NOT NULL、email カラムが存在
	users := tableByName(schema, "users")
	if id := colByName(users, "id"); id == nil || !id.IsPK || id.Nullable {
		t.Errorf("users.id = %+v, want PK かつ NOT NULL", id)
	}
	if colByName(users, "email") == nil {
		t.Error("users.email カラムが取得できていない")
	}

	// sensor_readings: temperature は REAL、deleted_at は NULL 許容
	sr := tableByName(schema, "sensor_readings")
	if temp := colByName(sr, "temperature"); temp == nil || temp.Type != "REAL" {
		t.Errorf("sensor_readings.temperature = %+v, want type REAL", temp)
	}
	if del := colByName(sr, "deleted_at"); del == nil || !del.Nullable {
		t.Errorf("sensor_readings.deleted_at = %+v, want NULL 許容", del)
	}

	// sensor_readings の CHECK 制約(温度/湿度の範囲)が取得される
	if !hasCheck(sr, "sensor_readings_temperature_range", "BETWEEN") {
		t.Errorf("sensor_readings の温度 CHECK が取得できていない: %+v", sr.Checks)
	}

	// alert_rules の enum CHECK(metric/operator)が取得される(許容値が読める)
	ar := tableByName(schema, "alert_rules")
	if !hasCheck(ar, "alert_rules_metric_valid", "temperature") {
		t.Errorf("alert_rules の metric CHECK が取得できていない: %+v", ar.Checks)
	}
	if !hasCheck(ar, "alert_rules_operator_valid", "'>='") {
		t.Errorf("alert_rules の operator CHECK が取得できていない: %+v", ar.Checks)
	}

	// devices: 部分 UNIQUE 索引が IsUnique=true・WHERE 句付きで取得される
	dev := tableByName(schema, "devices")
	idx := indexByName(dev, "devices_mac_address_unique_active")
	if idx == nil || !idx.IsUnique {
		t.Errorf("devices の部分 UNIQUE 索引が取得できていない: %+v", dev.Indexes)
	}

	// レンダリングが破綻しないこと(スモーク)
	md := RenderMarkdown(schema)
	if !strings.Contains(md, "## sensor_readings") || !strings.Contains(md, "CHECK") {
		t.Error("RenderMarkdown 出力にテーブル/CHECK が含まれない")
	}
}

func tableNames(s *Schema) []string {
	out := make([]string, len(s.Tables))
	for i, t := range s.Tables {
		out[i] = t.Name
	}
	return out
}

func hasCheck(t *Table, name, mustContain string) bool {
	for _, ck := range t.Checks {
		if ck.Name == name && strings.Contains(ck.Expr, mustContain) {
			return true
		}
	}
	return false
}

func indexByName(t *Table, name string) *Index {
	for i := range t.Indexes {
		if t.Indexes[i].Name == name {
			return &t.Indexes[i]
		}
	}
	return nil
}
