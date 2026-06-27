package handler

import (
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/domain"
	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// readings_report_test.go は集計帳票 (日次/時間別) の純粋バケット集計ロジック
// (buildReadingsReport / readingsDailyRows / readingsHourlyRows) を DB 非依存で検証する
// (テストガイダンス集 §4・device_show_vpd_test.go のバケット作法に倣う)。
// 期待値の VPD/統計は internal/chart で検算済み (33%/67%・σ5.00 等)。

// --- 2.1 作物別適正帯ラベルと滞在率 (R6) ---

// 同一 JST 日・同一時間帯に置いた 3 点。(20,70)=VPD0.70 両帯内、(20,85)=VPD0.35 既定帯のみ、
// (25,50)=VPD1.58 両帯外。これにより作物=ゴーヤ(0.4-1.2)は 33%、未設定既定(0.3-1.5)は 67% になる。
func reportCropDiffRows() []repository.SensorReading {
	return []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 20, 70),  // 12時 JST・両帯内
		sensorRow(1, time.Date(2026, 4, 20, 3, 5, 0, 0, time.UTC), 20, 85),  // 12時 JST・既定帯のみ
		sensorRow(1, time.Date(2026, 4, 20, 3, 10, 0, 0, time.UTC), 25, 50), // 12時 JST・両帯外
	}
}

func TestBuildReadingsReport_作物別適正帯と滞在率が切り替わる(t *testing.T) {
	rows := reportCropDiffRows()

	t.Run("ゴーヤは0.40〜1.20kPaで滞在率33%", func(t *testing.T) {
		rep := buildReadingsReport(rows, domain.CropGoya)
		if !rep.HasData {
			t.Fatal("HasData = false, want true")
		}
		if rep.CropLabel != "ゴーヤ" {
			t.Errorf("CropLabel = %q, want ゴーヤ", rep.CropLabel)
		}
		if rep.RangeLabel != "0.40〜1.20 kPa" {
			t.Errorf("RangeLabel = %q, want 0.40〜1.20 kPa", rep.RangeLabel)
		}
		if len(rep.Daily) != 1 {
			t.Fatalf("Daily 行数 = %d, want 1", len(rep.Daily))
		}
		if rep.Daily[0].InRange != "33%" {
			t.Errorf("Daily[0].InRange = %q, want 33%%", rep.Daily[0].InRange)
		}
		if len(rep.Hourly) != 1 || rep.Hourly[0].InRange != "33%" {
			t.Errorf("Hourly 滞在率 = %+v, want 1行33%%", rep.Hourly)
		}
	})

	t.Run("作物未設定は既定0.30〜1.50kPaで滞在率67%", func(t *testing.T) {
		rep := buildReadingsReport(rows, domain.Crop(""))
		if rep.CropLabel != "既定" {
			t.Errorf("CropLabel = %q, want 既定", rep.CropLabel)
		}
		if rep.RangeLabel != "0.30〜1.50 kPa" {
			t.Errorf("RangeLabel = %q, want 0.30〜1.50 kPa", rep.RangeLabel)
		}
		if rep.Daily[0].InRange != "67%" {
			t.Errorf("Daily[0].InRange = %q, want 67%%", rep.Daily[0].InRange)
		}
	})
}

// --- 2.1 空期間は空帳票 (R4.3) ---

func TestBuildReadingsReport_空期間は空帳票で数値を捏造しない(t *testing.T) {
	rep := buildReadingsReport(nil, domain.CropGoya)
	if rep.HasData {
		t.Error("空 rows で HasData = true")
	}
	if len(rep.Daily) != 0 || len(rep.Hourly) != 0 {
		t.Errorf("空 rows で帳票が非空: Daily=%+v Hourly=%+v", rep.Daily, rep.Hourly)
	}
	// ラベルは空でも保持する (帳票ヘッダ表示用)。
	if rep.CropLabel != "ゴーヤ" || rep.RangeLabel != "0.40〜1.20 kPa" {
		t.Errorf("空 rows でもラベルは保持: Crop=%q Range=%q", rep.CropLabel, rep.RangeLabel)
	}
}

// --- 2.1 日次バケットの統計列整形 (R4.1/4.2) ---

func TestReadingsDailyRows_温湿度の統計列を整形する(t *testing.T) {
	// 同一 JST 日 (2026-04-20) に temps[20,30]/hums[60,80]。chart で検算済み。
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 20, 60),
		sensorRow(1, time.Date(2026, 4, 20, 3, 5, 0, 0, time.UTC), 30, 80),
	}
	got := readingsDailyRows(rows, 0.4, 1.2)
	if len(got) != 1 {
		t.Fatalf("Daily 行数 = %d, want 1", len(got))
	}
	r := got[0]
	checks := []struct{ name, got, want string }{
		{"Bucket", r.Bucket, "2026-04-20"},
		{"TempAvg", r.TempAvg, "25.00"},
		{"TempMax", r.TempMax, "30.00"},
		{"TempMin", r.TempMin, "20.00"},
		{"TempDiurnal", r.TempDiurnal, "10.00"},
		{"TempSigma", r.TempSigma, "5.00"},
		{"TempCV", r.TempCV, "0.20"},
		{"HumAvg", r.HumAvg, "70.00"},
		{"HumMax", r.HumMax, "80.00"},
		{"HumMin", r.HumMin, "60.00"},
		{"HumDiurnal", r.HumDiurnal, "20.00"},
		{"HumSigma", r.HumSigma, "10.00"},
		{"HumCV", r.HumCV, "0.14"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// --- 2.1 JST 日跨ぎで別バケット (R5.2 相当・暦日境界) ---

func TestReadingsDailyRows_JST日跨ぎで別の暦日に分かれる(t *testing.T) {
	// 14:00 UTC = 23:00 JST (04-19)、15:00 UTC = 00:00 JST (04-20)。別の JST 暦日へ。
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 19, 14, 0, 0, 0, time.UTC), 25, 60),
		sensorRow(1, time.Date(2026, 4, 19, 15, 0, 0, 0, time.UTC), 26, 61),
	}
	got := readingsDailyRows(rows, 0.4, 1.2)
	if len(got) != 2 {
		t.Fatalf("Daily 行数 = %d, want 2 (04-19/04-20): %+v", len(got), got)
	}
	if got[0].Bucket != "2026-04-19" || got[1].Bucket != "2026-04-20" {
		t.Errorf("日付昇順 = %q,%q, want 2026-04-19,2026-04-20", got[0].Bucket, got[1].Bucket)
	}
}

// --- 2.1 欠測日は「—」行で補完 (R4.4) ---

func TestReadingsDailyRows_欠測日は全項目ダッシュの行になる(t *testing.T) {
	// 04-18 と 04-20 に計測、間の 04-19 は欠測。最古〜最新を連続で埋める。
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 18, 3, 0, 0, 0, time.UTC), 25, 60),
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 26, 61),
	}
	got := readingsDailyRows(rows, 0.4, 1.2)
	if len(got) != 3 {
		t.Fatalf("Daily 行数 = %d, want 3 (04-18/19/20): %+v", len(got), got)
	}
	miss := got[1]
	if miss.Bucket != "2026-04-19" {
		t.Errorf("欠測日 Bucket = %q, want 2026-04-19", miss.Bucket)
	}
	for _, c := range []struct{ name, got string }{
		{"TempAvg", miss.TempAvg}, {"TempSigma", miss.TempSigma}, {"TempCV", miss.TempCV},
		{"HumAvg", miss.HumAvg}, {"InRange", miss.InRange},
	} {
		if c.got != "—" {
			t.Errorf("欠測日 %s = %q, want —", c.name, c.got)
		}
	}
}

// --- 2.1 時間別バケット (JST 時刻・昇順・データのある時間帯のみ・R5.1/5.2/5.3) ---

func TestReadingsHourlyRows_JST時間帯で昇順データのある時間帯のみ(t *testing.T) {
	// 21:00 UTC = 06:00 JST、03:00 UTC = 12:00 JST。他の時間帯は出さない。
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 25, 60),   // 12時
		sensorRow(1, time.Date(2026, 4, 19, 21, 0, 0, 0, time.UTC), 22, 70),  // 06時
	}
	got := readingsHourlyRows(rows, 0.4, 1.2)
	if len(got) != 2 {
		t.Fatalf("Hourly 行数 = %d, want 2 (06時/12時): %+v", len(got), got)
	}
	if got[0].Bucket != "06時" || got[1].Bucket != "12時" {
		t.Errorf("時刻昇順 = %q,%q, want 06時,12時", got[0].Bucket, got[1].Bucket)
	}
}

// --- 2.1 単点バケットの σ/CV (既存 dailyRow 作法に整合) ---

// 単点は母標準偏差 σ=0.00 (既存 device_show 日次表と同一規約)、CV は μ≈0 のとき未定義「—」。
// (0,0) の単点で温湿度とも平均 0 → CV「—」、σ は「0.00」。
func TestReadingsDailyRows_単点で平均ゼロはCVがダッシュ(t *testing.T) {
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 0, 0),
	}
	got := readingsDailyRows(rows, 0.4, 1.2)
	if len(got) != 1 {
		t.Fatalf("Daily 行数 = %d, want 1", len(got))
	}
	r := got[0]
	if r.TempCV != "—" || r.HumCV != "—" {
		t.Errorf("平均0の CV = temp%q/hum%q, want —/—", r.TempCV, r.HumCV)
	}
	if r.TempSigma != "0.00" {
		t.Errorf("単点 σ = %q, want 0.00 (母σ=0・既存作法)", r.TempSigma)
	}
}
