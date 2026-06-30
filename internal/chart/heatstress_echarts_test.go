package chart

import (
	"encoding/json"
	"strings"
	"testing"
)

// 高温ストレス描画層（heatmap/calendar/visualMap/line/bar の option JSON 構築）の単体テスト。
// テストガイダンス集に沿い、go-echarts ネイティブ型→直列化経路で
// camelCase キー（visualMap/calendar/coordinateSystem）が残ること・暑熱=暖色の向き・
// 空データで空 option（"{}"）・HTML 安全（</script> 不混入）を strings.Contains/JSON 検証する。

// assertValidJSON は option 文字列が妥当な JSON であることを確認する。
func assertValidJSON(t *testing.T, label, s string) {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("%s: option が妥当な JSON でない: %v\n%s", label, err, s)
	}
}

// assertHTMLSafe は < > & が \uXXXX 化され生の </script> が混入しないことを確認する（§10-E）。
func assertHTMLSafe(t *testing.T, label, s string) {
	t.Helper()
	if strings.Contains(s, "</script>") || strings.Contains(s, "<script") {
		t.Errorf("%s: HTML 安全でない（生の script タグ混入）: %s", label, s)
	}
}

// sampleHeatSpec は系列が満たされた代表 spec を返す（暖色は --color-heat=#d6336c 想定）。
func sampleHeatSpec() HeatStressChartSpec {
	return HeatStressChartSpec{
		THIHourDay: []HeatCell{
			{Day: 0, Hour: 14, Value: 80.2},
			{Day: 1, Hour: 3, Value: 72.5},
		},
		THIDayLabels: []string{"06/20", "06/21"},
		THIMin:       65,
		THIMax:       90,

		CalendarRange: []string{"2025-07-01", "2026-06-30"},
		CalendarCells: []DateValue{
			{Date: "2026-06-20", Value: 26.3},
			{Date: "2026-06-21", Value: 25.1},
		},
		NightMin: 20,
		NightMax: 30,

		DayLabels:  []string{"06/20", "06/21"},
		NightTemps: []float64{26.3, 25.1},
		DeltaT:     []float64{8.2, 7.5},

		AHLabels: []string{"14:00", "14:05"},
		AH:       []float64{18.2, 18.4},

		HasTrend:     true,
		YearLabels:   []string{"2024", "2025", "2026"},
		YearlyCounts: []float64{40, 48, 55},
		SenLine:      []float64{41, 47.5, 54},

		Color: "#d6336c",
	}
}

// ---- 3.1 THI 時間帯×日ヒートマップ ----------------------------------------

func TestTHIHourDayHeatmapOptionJSON(t *testing.T) {
	got, err := THIHourDayHeatmapOptionJSON(sampleHeatSpec())
	if err != nil {
		t.Fatalf("予期しない error: %v", err)
	}
	assertValidJSON(t, "THI heatmap", got)
	assertHTMLSafe(t, "THI heatmap", got)

	for _, key := range []string{`"heatmap"`, `"visualMap"`, `"inRange"`, `"color"`} {
		if !strings.Contains(got, key) {
			t.Errorf("THI heatmap option に %s が含まれない: %s", key, got)
		}
	}
	// 暑熱=暖色の向き: 低端（薄黄 #fff5b1）が高端（--color-heat #d6336c）より色配列で前に来る。
	lo := strings.Index(got, "#fff5b1")
	hi := strings.Index(got, "#d6336c")
	if lo < 0 || hi < 0 || lo >= hi {
		t.Errorf("visualMap inRange.color が暑熱=暖色の単調並び(低→高)でない: lo=%d hi=%d\n%s", lo, hi, got)
	}
}

func TestTHIHourDayHeatmapOptionJSON_Empty(t *testing.T) {
	spec := sampleHeatSpec()
	spec.THIHourDay = nil
	got, err := THIHourDayHeatmapOptionJSON(spec)
	if err != nil {
		t.Fatalf("予期しない error: %v", err)
	}
	if got != "{}" {
		t.Errorf("空データ時は空 option \"{}\" を返すべき: got %s", got)
	}
}

// ---- 3.2 熱帯夜カレンダーヒートマップ --------------------------------------

func TestTropicalNightCalendarOptionJSON(t *testing.T) {
	got, err := TropicalNightCalendarOptionJSON(sampleHeatSpec())
	if err != nil {
		t.Fatalf("予期しない error: %v", err)
	}
	assertValidJSON(t, "calendar", got)
	assertHTMLSafe(t, "calendar", got)

	// calendar/visualMap/coordinateSystem が camelCase で残ること（Q1 実証）。
	for _, key := range []string{`"calendar"`, `"visualMap"`, `"coordinateSystem"`, `"heatmap"`} {
		if !strings.Contains(got, key) {
			t.Errorf("calendar option に %s（camelCase）が含まれない: %s", key, got)
		}
	}
	// calendar 直近1年レンジが反映される。
	if !strings.Contains(got, "2025-07-01") || !strings.Contains(got, "2026-06-30") {
		t.Errorf("calendar option に CalendarRange が反映されない: %s", got)
	}
	// calendar 座標系ゆえ直交 xAxis を持たない（hasXYAxis=false）。
	if strings.Contains(got, `"xAxis"`) {
		t.Errorf("calendar option に直交 xAxis が混入している（calendar 座標系のはず）: %s", got)
	}
	// 暑熱=暖色の向き。
	if lo, hi := strings.Index(got, "#fff5b1"), strings.Index(got, "#d6336c"); lo < 0 || hi < 0 || lo >= hi {
		t.Errorf("calendar visualMap が暑熱=暖色の並びでない: %s", got)
	}
}

func TestTropicalNightCalendarOptionJSON_Empty(t *testing.T) {
	spec := sampleHeatSpec()
	spec.CalendarCells = nil
	got, err := TropicalNightCalendarOptionJSON(spec)
	if err != nil {
		t.Fatalf("予期しない error: %v", err)
	}
	if got != "{}" {
		t.Errorf("空データ時は空 option \"{}\" を返すべき: got %s", got)
	}
}

// ---- 3.3 夜温推移・日較差ΔT / 絶対湿度 AH（line） --------------------------

func TestNightTempDeltaLineOptionJSON(t *testing.T) {
	got, err := NightTempDeltaLineOptionJSON(sampleHeatSpec())
	if err != nil {
		t.Fatalf("予期しない error: %v", err)
	}
	assertValidJSON(t, "night/delta line", got)
	assertHTMLSafe(t, "night/delta line", got)

	if !strings.Contains(got, `"line"`) {
		t.Errorf("夜温/ΔT option に line series が含まれない: %s", got)
	}
	// 夜温推移・日較差ΔT の2系列名が出る。
	for _, name := range []string{"夜温", "日較差"} {
		if !strings.Contains(got, name) {
			t.Errorf("夜温/ΔT option に系列名 %q が含まれない: %s", name, got)
		}
	}
}

func TestNightTempDeltaLineOptionJSON_Empty(t *testing.T) {
	spec := sampleHeatSpec()
	spec.NightTemps = nil
	spec.DeltaT = nil
	got, err := NightTempDeltaLineOptionJSON(spec)
	if err != nil {
		t.Fatalf("予期しない error: %v", err)
	}
	if got != "{}" {
		t.Errorf("空データ時は空 option \"{}\" を返すべき: got %s", got)
	}
}

func TestAHLineOptionJSON(t *testing.T) {
	got, err := AHLineOptionJSON(sampleHeatSpec())
	if err != nil {
		t.Fatalf("予期しない error: %v", err)
	}
	assertValidJSON(t, "AH line", got)
	assertHTMLSafe(t, "AH line", got)
	if !strings.Contains(got, `"line"`) {
		t.Errorf("AH option に line series が含まれない: %s", got)
	}
}

func TestAHLineOptionJSON_Empty(t *testing.T) {
	spec := sampleHeatSpec()
	spec.AH = nil
	got, err := AHLineOptionJSON(spec)
	if err != nil {
		t.Fatalf("予期しない error: %v", err)
	}
	if got != "{}" {
		t.Errorf("空データ時は空 option \"{}\" を返すべき: got %s", got)
	}
}

// ---- 3.4 熱帯夜年間日数トレンド（bar + Sen markLine） ----------------------

func TestTropicalNightTrendOptionJSON(t *testing.T) {
	got, err := TropicalNightTrendOptionJSON(sampleHeatSpec())
	if err != nil {
		t.Fatalf("予期しない error: %v", err)
	}
	assertValidJSON(t, "trend", got)
	assertHTMLSafe(t, "trend", got)

	if !strings.Contains(got, `"bar"`) {
		t.Errorf("トレンド option に bar series が含まれない: %s", got)
	}
	if !strings.Contains(got, `"markLine"`) {
		t.Errorf("トレンド option に Sen 傾き markLine が含まれない: %s", got)
	}
	// 年系列が反映される。
	if !strings.Contains(got, "2024") || !strings.Contains(got, "2026") {
		t.Errorf("トレンド option に年ラベルが反映されない: %s", got)
	}
}

func TestTropicalNightTrendOptionJSON_NoTrend(t *testing.T) {
	spec := sampleHeatSpec()
	spec.HasTrend = false
	got, err := TropicalNightTrendOptionJSON(spec)
	if err != nil {
		t.Fatalf("予期しない error: %v", err)
	}
	if got != "{}" {
		t.Errorf("トレンド無し時は空 option \"{}\" を返すべき: got %s", got)
	}
}

// HasTrend だが Sen 線が2点未満のときは棒のみ（markLine を描かない・防御的縮退）。
func TestTropicalNightTrendOptionJSON_ShortSenLineBarOnly(t *testing.T) {
	spec := sampleHeatSpec()
	spec.SenLine = nil // Sen 線を引くだけの年数がない（棒のみ）
	got, err := TropicalNightTrendOptionJSON(spec)
	if err != nil {
		t.Fatalf("予期しない error: %v", err)
	}
	if !strings.Contains(got, `"bar"`) {
		t.Errorf("棒は残るべき: %s", got)
	}
	if strings.Contains(got, `"markLine"`) {
		t.Errorf("Sen 線が2点未満のとき markLine を出すべきでない: %s", got)
	}
}

// 基準色未設定でも visualMap は妥当な暖色スケール（最濃側 #d6336c）へフォールバックする。
func TestHeatColorScale_DefaultsWhenEmpty(t *testing.T) {
	colors := heatColorScale("")
	if len(colors) != 4 {
		t.Fatalf("heatColorScale 段数 = %d, want 4", len(colors))
	}
	if colors[0] != "#fff5b1" || colors[len(colors)-1] != "#d6336c" {
		t.Errorf("暖色スケールの端色が想定外: %v", colors)
	}
}

// ---- HTML 安全（外部入力由来の < > & のエスケープ） ------------------------

func TestHeatStressOptionJSON_EscapesHTML(t *testing.T) {
	spec := sampleHeatSpec()
	// 万一ラベルに </script> 類が混じっても \uXXXX 化されること。
	spec.DayLabels = []string{"</script>", "x"}
	got, err := NightTempDeltaLineOptionJSON(spec)
	if err != nil {
		t.Fatalf("予期しない error: %v", err)
	}
	assertHTMLSafe(t, "escape", got)
	// 生の < が残らず、< へエスケープされている。
	if strings.Contains(got, "<") {
		t.Errorf("< が生のまま残っている（エスケープされるべき）: %s", got)
	}
	if !strings.Contains(got, "\\u003c") {
		t.Errorf("< が \\u003c へエスケープされていない: %s", got)
	}
}
