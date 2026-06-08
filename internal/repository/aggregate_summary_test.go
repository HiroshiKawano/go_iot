package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// newSensorReadingsDB は sensor_readings テーブルだけを持つインメモリ SQLite を用意する。
// 集計クエリの実機スモーク用(migration の最小サブセット・goose 非依存で軽量)。
func newSensorReadingsDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE sensor_readings (
		id          INTEGER  PRIMARY KEY,
		device_id   INTEGER  NOT NULL,
		temperature REAL     NOT NULL,
		humidity    REAL     NOT NULL,
		recorded_at DATETIME NOT NULL,
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		deleted_at  DATETIME
	)`); err != nil {
		t.Fatalf("CREATE TABLE sensor_readings: %v", err)
	}
	return db
}

// Test_GetSensorReadingsSummary_空集合期間はScan失敗せずErrNoRowsを返す は、移行で混入し得た
// 回帰(対象期間が空集合のとき AVG/MAX/MIN が NULL を返し float64 Scan が失敗して 500 になる)を
// GROUP BY device_id で 0 行=sql.ErrNoRows に確定したことを実機固定する。
// 移行前(pgtype.Numeric の Valid=false)と等価な「データなし」応答を保つための回帰ガード。
func Test_GetSensorReadingsSummary_空集合期間はScan失敗せずErrNoRowsを返す(t *testing.T) {
	q := New(newSensorReadingsDB(t))

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)

	_, err := q.GetSensorReadingsSummary(context.Background(), GetSensorReadingsSummaryParams{
		DeviceID:     999, // 計測が1件も無いデバイス
		RecordedAt:   from,
		RecordedAt_2: to,
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("空集合期間は sql.ErrNoRows を期待(Scan エラーは回帰)。got err=%v", err)
	}
}

// Test_GetSensorReadingsSummary_計測ありは集計をfloat64で返す は、非空時に
// CAST(... AS REAL) で MAX/MIN/AVG が float64 として正しく集計されること、および
// CreateSensorReading の RETURNING * と recorded_at の time.Time 往復が modernc で動くことを実機固定する。
func Test_GetSensorReadingsSummary_計測ありは集計をfloat64で返す(t *testing.T) {
	q := New(newSensorReadingsDB(t))
	ctx := context.Background()

	rec := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	for _, temp := range []float64{20.00, 30.00} {
		if _, err := q.CreateSensorReading(ctx, CreateSensorReadingParams{
			DeviceID: 1, Temperature: temp, Humidity: 50.00, RecordedAt: rec,
		}); err != nil {
			t.Fatalf("CreateSensorReading(RETURNING *): %v", err)
		}
	}

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	row, err := q.GetSensorReadingsSummary(ctx, GetSensorReadingsSummaryParams{
		DeviceID: 1, RecordedAt: from, RecordedAt_2: to,
	})
	if err != nil {
		t.Fatalf("非空集計でエラー: %v", err)
	}
	if row.SampleCount != 2 {
		t.Errorf("SampleCount = %d, want 2", row.SampleCount)
	}
	if row.MaxTemperature != 30.00 || row.MinTemperature != 20.00 {
		t.Errorf("Max/MinTemperature = %v/%v, want 30/20", row.MaxTemperature, row.MinTemperature)
	}
	if row.AvgTemperature != 25.00 {
		t.Errorf("AvgTemperature = %v, want 25", row.AvgTemperature)
	}
}
