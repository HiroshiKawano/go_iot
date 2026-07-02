package chart

import (
	"encoding/json"
	"testing"
)

// echarts_neutral_test.go は、クライアント側 ECharts テーマ patch（buildChromePatch・App.templ）が
// クローム色を安全にマージできる前提として、サーバ側 option JSON がクローム色
// （xAxis/yAxis の axisLabel.color・legend/visualMap の textStyle.color・backgroundColor）を
// 一切出力しない（中立）ことを構造アサートする（テストガイダンス §56.1 定石・要件 6.1 の
// patch 前提境界ガード）。データ意味色（series の lineStyle.color・visualMap の
// inRange.color・markArea の itemStyle.color 等）はガード対象外（データ意味色は両テーマ
// 据置が要件であり、サーバ側が引き続き所有する）。
//
// 代表4チャート（line／乖離率2軸／heatmap／calendar）:
//   - line: ChartOptionJSON（Deviation 未設定＝単一 yAxis）
//   - 乖離率2軸: ChartOptionJSON（Deviation 設定＝ ExtendYAxis で yAxis 2本・echarts.go:89）
//   - heatmap: THIHourDayHeatmapOptionJSON
//   - calendar: TropicalNightCalendarOptionJSON

func neutralTestLineSpec(withDeviation bool) ChartSpec {
	spec := ChartSpec{
		Labels: []string{"10:00", "10:05", "10:10"},
		Unit:   "℃",
		Color:  "#fd7e14",
		Raw:    []float64{25.1, 25.3, 25.0},
	}
	if withDeviation {
		v := 1.5
		spec.Deviation = []*float64{&v, &v, &v}
	}
	return spec
}

func neutralTestHeatStressSpec() HeatStressChartSpec {
	return HeatStressChartSpec{
		THIHourDay:    []HeatCell{{Hour: 0, Day: 0, Value: 70}, {Hour: 1, Day: 0, Value: 75}},
		THIDayLabels:  []string{"07/01"},
		THIMin:        60,
		THIMax:        90,
		Color:         "#d6336c",
		CalendarRange: []string{"2026-07-01", "2026-07-02"},
		CalendarCells: []DateValue{{Date: "2026-07-01", Value: 26.5}},
		NightMin:      20,
		NightMax:      30,
	}
}

// containsKeyAnywhere は JSON をデコードした構造 (map[string]any/[]any の木) を再帰的に走査し、
// 指定キーがどこかに存在するかを構造的に判定する (文字列 grep ではなく木構造の探索・§56.1)。
func containsKeyAnywhere(v any, key string) bool {
	switch t := v.(type) {
	case map[string]any:
		if _, ok := t[key]; ok {
			return true
		}
		for _, vv := range t {
			if containsKeyAnywhere(vv, key) {
				return true
			}
		}
	case []any:
		for _, vv := range t {
			if containsKeyAnywhere(vv, key) {
				return true
			}
		}
	}
	return false
}

// asObjectSlice は option の xAxis/yAxis/visualMap のように「単一オブジェクトまたは配列」の
// どちらでも来うるフィールドを []map[string]any へ正規化する。
func asObjectSlice(v any) []map[string]any {
	switch t := v.(type) {
	case []any:
		out := make([]map[string]any, 0, len(t))
		for _, e := range t {
			if m, ok := e.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]any:
		return []map[string]any{t}
	}
	return nil
}

func assertNoAxisLabelColor(t *testing.T, option map[string]any, caseName string) {
	t.Helper()
	for _, axisKey := range []string{"xAxis", "yAxis"} {
		for _, axis := range asObjectSlice(option[axisKey]) {
			label, ok := axis["axisLabel"].(map[string]any)
			if !ok {
				continue
			}
			if _, hasColor := label["color"]; hasColor {
				t.Errorf("[%s] %s.axisLabel.color がサーバ option に出現している (patch前提のクローム中立に反する)", caseName, axisKey)
			}
		}
	}
}

func assertNoLegendTextStyleColor(t *testing.T, option map[string]any, caseName string) {
	t.Helper()
	legend, ok := option["legend"].(map[string]any)
	if !ok {
		return
	}
	textStyle, ok := legend["textStyle"].(map[string]any)
	if !ok {
		return
	}
	if _, hasColor := textStyle["color"]; hasColor {
		t.Errorf("[%s] legend.textStyle.color がサーバ option に出現している", caseName)
	}
}

func assertNoVisualMapTextStyleColor(t *testing.T, option map[string]any, caseName string) {
	t.Helper()
	for _, vm := range asObjectSlice(option["visualMap"]) {
		textStyle, ok := vm["textStyle"].(map[string]any)
		if !ok {
			continue
		}
		if _, hasColor := textStyle["color"]; hasColor {
			t.Errorf("[%s] visualMap.textStyle.color がサーバ option に出現している", caseName)
		}
	}
}

func assertNoBackgroundColor(t *testing.T, option map[string]any, caseName string) {
	t.Helper()
	if containsKeyAnywhere(option, "backgroundColor") {
		t.Errorf("[%s] backgroundColor がサーバ option のどこかに出現している (patch前提のクローム中立に反する)", caseName)
	}
}

func TestEChartsOptionsAreChromeNeutral(t *testing.T) {
	lineOpt, err := ChartOptionJSON(neutralTestLineSpec(false))
	if err != nil {
		t.Fatalf("line option 構築に失敗: %v", err)
	}
	deviationOpt, err := ChartOptionJSON(neutralTestLineSpec(true))
	if err != nil {
		t.Fatalf("乖離率2軸 option 構築に失敗: %v", err)
	}
	heatSpec := neutralTestHeatStressSpec()
	heatmapOpt, err := THIHourDayHeatmapOptionJSON(heatSpec)
	if err != nil {
		t.Fatalf("heatmap option 構築に失敗: %v", err)
	}
	calendarOpt, err := TropicalNightCalendarOptionJSON(heatSpec)
	if err != nil {
		t.Fatalf("calendar option 構築に失敗: %v", err)
	}

	cases := map[string]string{
		"line(乖離率なし)": lineOpt,
		"乖離率2軸":       deviationOpt,
		"heatmap":     heatmapOpt,
		"calendar":    calendarOpt,
	}

	for name, raw := range cases {
		name, raw := name, raw
		t.Run(name, func(t *testing.T) {
			var option map[string]any
			if err := json.Unmarshal([]byte(raw), &option); err != nil {
				t.Fatalf("option JSON のパースに失敗: %v\n%s", err, raw)
			}
			assertNoAxisLabelColor(t, option, name)
			assertNoLegendTextStyleColor(t, option, name)
			assertNoVisualMapTextStyleColor(t, option, name)
			assertNoBackgroundColor(t, option, name)
		})
	}
}

// 乖離率2軸は yAxis が2本 (echarts.go:89 ExtendYAxis) であることを前提条件として固定する
// (buildChromePatch の Array.isArray 吸収ロジックがこの形状に依存するため・design.md 参照)。
func TestEChartsDeviationOptionHasTwoYAxes(t *testing.T) {
	raw, err := ChartOptionJSON(neutralTestLineSpec(true))
	if err != nil {
		t.Fatalf("option 構築に失敗: %v", err)
	}
	var option map[string]any
	if err := json.Unmarshal([]byte(raw), &option); err != nil {
		t.Fatalf("option JSON のパースに失敗: %v", err)
	}
	yAxis, ok := option["yAxis"].([]any)
	if !ok {
		t.Fatalf("yAxis が配列でない: %T", option["yAxis"])
	}
	if len(yAxis) != 2 {
		t.Errorf("乖離率2軸の yAxis 本数 = %d, want 2", len(yAxis))
	}
}
