package handler

import (
	"math"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
)

// seasonal_trend_rollup_test.go は統計分析ページのロールアップ集約・平年比（handler 境界の集約ロジック）を
// DB 非依存に検証する（タスク 4.1/4.2）。入力は ListDailySensorAggregatesJST の戻り値（Querier 出力形）。

func feq(a, b float64) bool { return math.Abs(a-b) <= 1e-9 }

// jstDailyRow は ListDailySensorAggregatesJST の1行（JST 暦日 + 温湿度の日次 avg/max/min/件数）を作る。
// avg は本番 SQL で NUMERIC キャスト（pgtype.Numeric）、max/min はキャストなし interface{}（float64 を渡す）。
func jstDailyRow(y int, m time.Month, d int, avgT, maxT, minT, avgH, maxH, minH float64, samples int64) repository.ListDailySensorAggregatesJSTRow {
	return repository.ListDailySensorAggregatesJSTRow{
		ReadingDate:    pgtype.Date{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC), Valid: true},
		AvgTemperature: pgconv.Numeric2(avgT),
		MaxTemperature: maxT,
		MinTemperature: minT,
		AvgHumidity:    pgconv.Numeric2(avgH),
		MaxHumidity:    maxH,
		MinHumidity:    minH,
		SampleCount:    samples,
	}
}

// 2024-01（2日）・2024-02（空＝行なし）・2024-03（1日）の日次行。月初/月末/空月を含む。
func rollupSampleRows() []repository.ListDailySensorAggregatesJSTRow {
	return []repository.ListDailySensorAggregatesJSTRow{
		jstDailyRow(2024, time.January, 10, 10, 15, 5, 60, 70, 50, 100), // ΔT温=10, ΔT湿=20
		jstDailyRow(2024, time.January, 20, 12, 18, 8, 64, 72, 56, 120), // ΔT温=10, ΔT湿=16
		jstDailyRow(2024, time.March, 5, 20, 26, 14, 70, 80, 62, 110),   // ΔT温=12, ΔT湿=18
	}
}

// ---- 4.1 月次ロールアップ（ΔT=日次ΔT平均・空月スキップ）---------------------------

func TestRollupDailyJST_Monthly(t *testing.T) {
	buckets := rollupDailyJST(rollupSampleRows(), granularityMonthly)

	// 空月(2024-02)はスキップ＝2バケットのみ（0補完しない）。
	if len(buckets) != 2 {
		t.Fatalf("バケット数 = %d, want 2（空月スキップ）", len(buckets))
	}
	if buckets[0].Key != "2024-01" || buckets[1].Key != "2024-03" {
		t.Fatalf("バケットキー = [%q, %q], want [2024-01, 2024-03]", buckets[0].Key, buckets[1].Key)
	}

	jan := buckets[0]
	// 温度: avg=mean(10,12)=11, max=max(15,18)=18, min=min(5,8)=5, ΔT=mean(10,10)=10, σ=StdDev(10,12)=1
	if !feq(jan.Temp.Avg, 11) || !feq(jan.Temp.Max, 18) || !feq(jan.Temp.Min, 5) {
		t.Errorf("1月 温度 avg/max/min = %v/%v/%v, want 11/18/5", jan.Temp.Avg, jan.Temp.Max, jan.Temp.Min)
	}
	if !feq(jan.Temp.DiurnalRange, 10) {
		t.Errorf("1月 温度 ΔT(=日次ΔTの平均) = %v, want 10", jan.Temp.DiurnalRange)
	}
	if !feq(jan.Temp.StdDev, 1) {
		t.Errorf("1月 温度 σ = %v, want 1", jan.Temp.StdDev)
	}
	if !jan.Temp.HasCV || !feq(jan.Temp.CV, 1.0/11.0) {
		t.Errorf("1月 温度 CV = %v(has=%v), want %v", jan.Temp.CV, jan.Temp.HasCV, 1.0/11.0)
	}
	if jan.Samples != 220 {
		t.Errorf("1月 サンプル数 = %d, want 220（日次件数の合計）", jan.Samples)
	}
	// 湿度: avg=mean(60,64)=62, max=72, min=50, ΔT=mean(20,16)=18, σ=StdDev(60,64)=2
	if !feq(jan.Humidity.Avg, 62) || !feq(jan.Humidity.Max, 72) || !feq(jan.Humidity.Min, 50) {
		t.Errorf("1月 湿度 avg/max/min = %v/%v/%v, want 62/72/50", jan.Humidity.Avg, jan.Humidity.Max, jan.Humidity.Min)
	}
	if !feq(jan.Humidity.DiurnalRange, 18) {
		t.Errorf("1月 湿度 ΔT = %v, want 18", jan.Humidity.DiurnalRange)
	}
	if !feq(jan.Humidity.StdDev, 2) {
		t.Errorf("1月 湿度 σ = %v, want 2", jan.Humidity.StdDev)
	}

	mar := buckets[1]
	// 単一日の月: avg=20, ΔT=12, σ=0（1点）。CV=0/20=0 だが HasCV=true（mean 非0）。
	if !feq(mar.Temp.Avg, 20) || !feq(mar.Temp.DiurnalRange, 12) || !feq(mar.Temp.StdDev, 0) {
		t.Errorf("3月 温度 avg/ΔT/σ = %v/%v/%v, want 20/12/0", mar.Temp.Avg, mar.Temp.DiurnalRange, mar.Temp.StdDev)
	}
	if mar.Month != 3 || mar.Year != 2024 {
		t.Errorf("3月 Year/Month = %d/%d, want 2024/3", mar.Year, mar.Month)
	}
	if mar.Samples != 110 {
		t.Errorf("3月 サンプル数 = %d, want 110", mar.Samples)
	}
}

// ---- 4.1 年次ロールアップ ------------------------------------------------------

func TestRollupDailyJST_Yearly(t *testing.T) {
	buckets := rollupDailyJST(rollupSampleRows(), granularityYearly)
	if len(buckets) != 1 {
		t.Fatalf("バケット数 = %d, want 1（2024年）", len(buckets))
	}
	y := buckets[0]
	if y.Key != "2024" {
		t.Errorf("キー = %q, want 2024", y.Key)
	}
	// 温度: avg=mean(10,12,20)=14, max=26, min=5, ΔT=mean(10,10,12)=32/3
	if !feq(y.Temp.Avg, 14) || !feq(y.Temp.Max, 26) || !feq(y.Temp.Min, 5) {
		t.Errorf("年次 温度 avg/max/min = %v/%v/%v, want 14/26/5", y.Temp.Avg, y.Temp.Max, y.Temp.Min)
	}
	if !feq(y.Temp.DiurnalRange, 32.0/3.0) {
		t.Errorf("年次 温度 ΔT = %v, want %v", y.Temp.DiurnalRange, 32.0/3.0)
	}
	if y.Samples != 330 {
		t.Errorf("年次 サンプル数 = %d, want 330", y.Samples)
	}
}

// ---- 4.1 空入力・縮退 ----------------------------------------------------------

func TestRollupDailyJST_Empty(t *testing.T) {
	if got := rollupDailyJST(nil, granularityMonthly); len(got) != 0 {
		t.Errorf("空入力のバケット数 = %d, want 0", len(got))
	}
}

// ---- 4.1 月次平均系列＋月キーの抽出（trend 検定への受け渡し）---------------------

func TestBucketSeries(t *testing.T) {
	buckets := rollupDailyJST(rollupSampleRows(), granularityMonthly)
	vals, months := bucketSeries(buckets, metricTemperature)
	if len(vals) != 2 || !feq(vals[0], 11) || !feq(vals[1], 20) {
		t.Errorf("温度平均系列 = %v, want [11 20]", vals)
	}
	if len(months) != 2 || months[0] != 1 || months[1] != 3 {
		t.Errorf("月キー = %v, want [1 3]", months)
	}
	valsH, _ := bucketSeries(buckets, metricHumidity)
	if len(valsH) != 2 || !feq(valsH[0], 62) {
		t.Errorf("湿度平均系列 = %v, want [62 ...]", valsH)
	}
}

// ---- 4.2 平年比（暦月平均・年数≥3で生成）---------------------------------------

func TestBuildClimatology_Available(t *testing.T) {
	// 3年分（2024/2025/2026）の1月データ → 暦月平均が算出可能（年≥3）。
	rows := []repository.ListDailySensorAggregatesJSTRow{
		jstDailyRow(2024, time.January, 10, 10, 15, 5, 60, 70, 50, 100),
		jstDailyRow(2025, time.January, 10, 12, 17, 7, 62, 72, 52, 100),
		jstDailyRow(2026, time.January, 10, 14, 19, 9, 64, 74, 54, 100),
	}
	buckets := rollupDailyJST(rows, granularityMonthly)
	clim := buildClimatology(buckets, metricTemperature)

	if !clim.Available {
		t.Fatalf("年≥3 で平年比が利用可能であるべき: %+v", clim)
	}
	// 1月の暦月平均 = mean(10,12,14)=12。全バケットが1月ゆえ Values は全て12。
	if len(clim.Values) != 3 {
		t.Fatalf("平年比系列長 = %d, want 3（バケット整列）", len(clim.Values))
	}
	for i, v := range clim.Values {
		if !feq(v, 12) {
			t.Errorf("Values[%d] = %v, want 12（1月の暦月平均）", i, v)
		}
	}
	// 提示時も不確実性注記を付す（要件 7.3）。
	if clim.Note == "" {
		t.Error("平年比提示時に不確実性注記が空")
	}
}

func TestBuildClimatology_InsufficientYears(t *testing.T) {
	// 2年分のみ（2024/2025）→ 平年比は非表示＋注記（要件 7.2）。
	rows := []repository.ListDailySensorAggregatesJSTRow{
		jstDailyRow(2024, time.January, 10, 10, 15, 5, 60, 70, 50, 100),
		jstDailyRow(2025, time.January, 10, 12, 17, 7, 62, 72, 52, 100),
	}
	buckets := rollupDailyJST(rows, granularityMonthly)
	clim := buildClimatology(buckets, metricTemperature)

	if clim.Available {
		t.Errorf("年<3 で平年比は非表示であるべき: %+v", clim)
	}
	if len(clim.Values) != 0 {
		t.Errorf("非表示時に Values が非空: %v", clim.Values)
	}
	if clim.Note == "" {
		t.Error("非表示時に「基準期間依存・不確実」注記が空")
	}
}
