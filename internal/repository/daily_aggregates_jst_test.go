package repository

import (
	"context"
	"testing"
	"time"
)

// Test_ListDailySensorAggregates_UTC計測がJST日境界でバケットされる は、移行で導入した
// date(recorded_at, '+9 hours') による JST 日次バケット(R5.2)の回帰ガード。
//
// 検証の肝: 同一 UTC 日(2026-06-01)の計測でも、JST(UTC+9)では 15:00 UTC を境に翌日へ繰り上がる。
//   - 14:59 UTC = 06-01 23:59 JST → バケット "2026-06-01"
//   - 15:00 UTC = 06-02 00:00 JST → バケット "2026-06-02"
//
// 素の date(recorded_at)(UTC基準)なら全件 "2026-06-01" の 1 バケットに潰れるため、
// 2 バケットに分かれること自体が JST 日境界グルーピングの証左になる。
// 併せて MAX/MIN/AVG が正値(R5.1)・非平坦(silent 0 でない・R5.3)であることを固定する。
func Test_ListDailySensorAggregates_UTC計測がJST日境界でバケットされる(t *testing.T) {
	q := New(newSensorReadingsDB(t))
	ctx := context.Background()

	// (recordedAt UTC, temp, humidity) — 06-01 UTC 内だが JST では 06-01 と 06-02 に分かれる
	samples := []struct {
		at   time.Time
		temp float64
		hum  float64
	}{
		{time.Date(2026, 6, 1, 13, 0, 0, 0, time.UTC), 20.00, 50.00},  // 22:00 JST → 06-01
		{time.Date(2026, 6, 1, 14, 59, 0, 0, time.UTC), 30.00, 60.00}, // 23:59 JST → 06-01
		{time.Date(2026, 6, 1, 15, 0, 0, 0, time.UTC), 22.00, 70.00},  // 00:00 JST → 06-02
		{time.Date(2026, 6, 1, 20, 0, 0, 0, time.UTC), 28.00, 80.00},  // 05:00 JST → 06-02
	}
	for _, s := range samples {
		if _, err := q.CreateSensorReading(ctx, CreateSensorReadingParams{
			DeviceID: 1, Temperature: s.temp, Humidity: s.hum, RecordedAt: s.at,
		}); err != nil {
			t.Fatalf("CreateSensorReading(%v): %v", s.at, err)
		}
	}

	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	rows, err := q.ListDailySensorAggregates(ctx, ListDailySensorAggregatesParams{
		DeviceID: 1, RecordedAt: from,
	})
	if err != nil {
		t.Fatalf("ListDailySensorAggregates: %v", err)
	}

	// JST 日境界で 2 バケットに分かれる(UTC 基準なら 1 バケットに潰れる = 回帰)
	if len(rows) != 2 {
		t.Fatalf("バケット数 = %d, want 2 (JST 日境界で分割されていない可能性。rows=%+v)", len(rows), rows)
	}

	// ORDER BY reading_date ASC で [06-01, 06-02] の順
	want := []struct {
		date             string
		maxT, minT, avgT float64
		count            int64
	}{
		{"2026-06-01", 30.00, 20.00, 25.00, 2},
		{"2026-06-02", 28.00, 22.00, 25.00, 2},
	}
	for i, w := range want {
		got := rows[i]
		if got.ReadingDate != w.date {
			t.Errorf("rows[%d].ReadingDate = %q, want %q (JST 日付)", i, got.ReadingDate, w.date)
		}
		if got.MaxTemperature != w.maxT || got.MinTemperature != w.minT {
			t.Errorf("rows[%d] Max/MinTemperature = %v/%v, want %v/%v", i, got.MaxTemperature, got.MinTemperature, w.maxT, w.minT)
		}
		if got.AvgTemperature != w.avgT {
			t.Errorf("rows[%d].AvgTemperature = %v, want %v", i, got.AvgTemperature, w.avgT)
		}
		if got.SampleCount != w.count {
			t.Errorf("rows[%d].SampleCount = %d, want %d", i, got.SampleCount, w.count)
		}
		// R5.1/R5.3: 集計が正値・非平坦(silent 0 に落ちていない)
		if got.MaxHumidity <= 0 || got.MinHumidity <= 0 || got.AvgHumidity <= 0 {
			t.Errorf("rows[%d] 湿度集計が非正(silent 平坦化の疑い): max=%v min=%v avg=%v", i, got.MaxHumidity, got.MinHumidity, got.AvgHumidity)
		}
	}
}

// Test_ListDailySensorAggregates_削除済みは集計から除外される は、deleted_at IS NULL 条件が
// SQLite 方言でも効くこと(論理削除した計測が日次集計に混入しない)を実機固定する。
func Test_ListDailySensorAggregates_削除済みは集計から除外される(t *testing.T) {
	db := newSensorReadingsDB(t)
	q := New(db)
	ctx := context.Background()

	rec := time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC) // 12:00 JST → 06-01
	for _, temp := range []float64{20.00, 40.00} {
		if _, err := q.CreateSensorReading(ctx, CreateSensorReadingParams{
			DeviceID: 1, Temperature: temp, Humidity: 50.00, RecordedAt: rec,
		}); err != nil {
			t.Fatalf("CreateSensorReading: %v", err)
		}
	}
	// 1 件を論理削除 (temp=40 の行)
	if _, err := db.ExecContext(ctx, `UPDATE sensor_readings SET deleted_at = CURRENT_TIMESTAMP WHERE temperature = 40.00`); err != nil {
		t.Fatalf("論理削除 UPDATE: %v", err)
	}

	rows, err := q.ListDailySensorAggregates(ctx, ListDailySensorAggregatesParams{
		DeviceID: 1, RecordedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ListDailySensorAggregates: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("バケット数 = %d, want 1", len(rows))
	}
	// 削除済み(40)が除外され、残り 1 件(20)のみが集計対象
	if rows[0].MaxTemperature != 20.00 || rows[0].SampleCount != 1 {
		t.Errorf("削除済み除外後 Max=%v Count=%d, want 20/1 (deleted_at 条件が効いていない)", rows[0].MaxTemperature, rows[0].SampleCount)
	}
}
