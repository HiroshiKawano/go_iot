package repository

import (
	"encoding/json"
	"testing"
	"time"
)

// Test_生成型_SQLite移行後の想定マッピングを型レベルで固定する は、Task 1.2 の
// codegen ゲートで実機確定した sqlc 生成型を回帰固定する。
//
// 将来 migrations/queries/sqlc.yaml の変更で再生成した際に、生成型が想定マッピング
// (design のデータ型マッピング表)から外れると本テストがコンパイルエラーになり検知できる。
// design「Revalidation Triggers: 生成型の形が確定 → pgconv ヘルパ形・handler 整形に影響」
// に対する型レベルのガードレール。
func Test_生成型_SQLite移行後の想定マッピングを型レベルで固定する(t *testing.T) {
	// --- NUMERIC(5,2) NOT NULL → REAL → float64 (オーナー決定 REAL) ---
	var _ float64 = SensorReading{}.Temperature
	var _ float64 = SensorReading{}.Humidity
	var _ float64 = AlertRule{}.Threshold
	var _ float64 = AlertHistory{}.ActualValue

	// --- BIGSERIAL PK / BIGINT → INTEGER → int64 ---
	var _ int64 = SensorReading{}.ID
	var _ int64 = Device{}.UserID
	var _ int64 = AlertHistory{}.AlertRuleID

	// --- TIMESTAMPTZ NOT NULL → DATETIME(datetime affinity) → time.Time ---
	var _ time.Time = SensorReading{}.RecordedAt
	var _ time.Time = SensorReading{}.CreatedAt
	var _ time.Time = AlertHistory{}.TriggeredAt

	// --- TIMESTAMPTZ NULL → DATETIME → *time.Time (emit_pointers_for_null_types) ---
	var _ *time.Time = SensorReading{}.DeletedAt
	var _ *time.Time = Device{}.LastCommunicatedAt
	var _ *time.Time = User{}.EmailVerifiedAt
	var _ *time.Time = DeviceToken{}.ExpiresAt
	var _ *time.Time = DeviceToken{}.LastUsedAt

	// --- VARCHAR NULL → *string ---
	var _ *string = Device{}.Location

	// --- JSONB → json affinity → json.RawMessage ---
	var _ json.RawMessage = DeviceToken{}.Abilities

	// --- sessions: BYTEA→BLOB→[]byte / expiry TIMESTAMPTZ→REAL→float64 (sqlite3store 要求) ---
	var _ []byte = Session{}.Data
	var _ float64 = Session{}.Expiry

	// --- 集計: CAST(AVG/MAX/MIN(...) AS REAL) → float64 (silent 平坦化防止の明示型確定 R5.1/5.3) ---
	var _ float64 = GetSensorReadingsSummaryRow{}.AvgTemperature
	var _ float64 = GetSensorReadingsSummaryRow{}.MaxHumidity
	var _ float64 = ListDailySensorAggregatesRow{}.MinTemperature

	// --- COUNT(*) → int64 ---
	var _ int64 = GetSensorReadingsSummaryRow{}.SampleCount

	// --- date(recorded_at,'+9 hours') を CAST(... AS TEXT) → string (JST 日次バケット R5.2) ---
	// CAST AS TEXT を付けないと interface{} 生成になるため string 確定が必須。
	var _ string = ListDailySensorAggregatesRow{}.ReadingDate

	// --- narg('device_id') の CAST(... AS INTEGER) → *int64 (nullable フィルタ) ---
	var _ *int64 = ListAlertHistoriesPaginatedParams{}.DeviceID
	var _ *int64 = CountAlertHistoriesInRangeParams{}.DeviceID

	// --- BETWEEN を col>=? AND col<=? に展開し from/to が Params へ取り込まれること ---
	// (sqlc SQLite engine の BETWEEN bind parameter 欠落バグの回避結果を固定)
	var _ time.Time = GetSensorReadingsSummaryParams{}.RecordedAt
	var _ time.Time = GetSensorReadingsSummaryParams{}.RecordedAt_2
	var _ time.Time = CountSensorReadingsInRangeParams{}.RecordedAt_2
	var _ time.Time = ListSensorReadingsPaginatedParams{}.RecordedAt_2
	var _ time.Time = ListAlertHistoriesPaginatedParams{}.FromAt
	var _ time.Time = ListAlertHistoriesPaginatedParams{}.ToAt

	// --- LIMIT/OFFSET は SQLite で int64 (PostgreSQL 版の int32 から変化・Task4 で追従) ---
	var _ int64 = ListSensorReadingsPaginatedParams{}.Limit
	var _ int64 = ListSensorReadingsPaginatedParams{}.Offset
	var _ int64 = ListAlertHistoriesPaginatedParams{}.LimitN
	var _ int64 = ListAlertHistoriesPaginatedParams{}.OffsetN
}
