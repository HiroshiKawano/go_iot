package component

import (
	"reflect"
	"testing"
)

// 高温ストレスパネルの View DTO（HeatStressPanelView/HeatStressCardView）の構造テスト。
// 本タスク（Task 4）は DTO 定義と DeviceChartAreaView への末尾非破壊追加のみで、
// templ 描画（HeatStressPanel）は Task 6。よってここでは「フィールドが揃って構築できること」と
// 「DeviceChartAreaView の末尾に HeatStress が非破壊で追加されたこと（既存フィールド順を変えない）」を
// reflect で固定する（要件 9.2, 10.1・design DTO 契約）。

// baselineHeatStressPanelView は系列・カードが満たされた決定的な HeatStressPanelView。
// 全フィールドを明示構築することで、いずれかのフィールド欠落/改名をコンパイルで捕捉する（RED の核）。
func baselineHeatStressPanelView() HeatStressPanelView {
	return HeatStressPanelView{
		HasData:        true,
		Guidance:       "",
		Color:          "#d6336c",
		THIHeatmapJSON: `{"series":[{"type":"heatmap"}]}`,
		CalendarJSON:   `{"calendar":[{}]}`,
		NightDeltaJSON: `{"series":[{"type":"line"}]}`,
		AHJSON:         `{"series":[{"type":"line"}]}`,
		HasTrend:       true,
		TrendJSON:      `{"series":[{"type":"bar"}]}`,
		TrendNote:      "統計的に非有意であってもトレンドが無いことを意味しません（複数年の蓄積が必要です）。",
		Card: HeatStressCardView{
			CurrentTHI:      "79.5",
			CurrentAH:       "18.2 g/m³",
			LatestNightTemp: "26.3℃",
			SenSlopeSign:    "+0.6 日/年（増加傾向・参考）",
		},
		TropicalLongest: "12 日",
		TropicalCurrent: "5 日",
	}
}

// HeatStressPanelView/HeatStressCardView の全フィールドが構築でき、値が保持される。
func TestHeatStressPanelView_全フィールド構築(t *testing.T) {
	v := baselineHeatStressPanelView()
	if !v.HasData || !v.HasTrend {
		t.Fatalf("HasData/HasTrend が保持されない: %+v", v)
	}
	if v.Color != "#d6336c" {
		t.Errorf("Color = %q, want #d6336c", v.Color)
	}
	if v.Card.CurrentTHI != "79.5" || v.Card.SenSlopeSign == "" {
		t.Errorf("Card が保持されない: %+v", v.Card)
	}
	if v.TropicalLongest != "12 日" || v.TropicalCurrent != "5 日" {
		t.Errorf("連続日数が保持されない: longest=%q current=%q", v.TropicalLongest, v.TropicalCurrent)
	}
}

// DeviceChartAreaView の末尾に HeatStress HeatStressPanelView が追加され、
// 既存フィールド（順序）が一切変わっていないこと（末尾非破壊追加）を固定する。
func TestDeviceChartAreaView_HeatStress末尾非破壊追加(t *testing.T) {
	// 既存フィールドの並び（views.go の現状＝無回帰の基準）。末尾に HeatStress を加える。
	wantOrder := []string{
		"DeviceID", "Period", "HasData",
		"TemperatureOptionJSON", "HumidityOptionJSON",
		"TemperatureUnit", "HumidityUnit", "TemperatureColor", "HumidityColor",
		"TemperatureCard", "HumidityCard",
		"ShowDaily", "TemperatureDaily", "HumidityDaily",
		"VPD", "Dewpoint", "HasGap",
		"HeatStress", // ← 末尾非破壊追加
	}

	typ := reflect.TypeOf(DeviceChartAreaView{})
	if got := typ.NumField(); got != len(wantOrder) {
		t.Fatalf("DeviceChartAreaView のフィールド数 = %d, want %d（既存フィールドの増減＝無回帰違反の疑い）", got, len(wantOrder))
	}
	for i, name := range wantOrder {
		if got := typ.Field(i).Name; got != name {
			t.Errorf("フィールド[%d] = %q, want %q（既存順序の変更＝無回帰違反）", i, got, name)
		}
	}

	// 末尾フィールドが HeatStress であり、型が HeatStressPanelView であること。
	last := typ.Field(typ.NumField() - 1)
	if last.Name != "HeatStress" {
		t.Fatalf("末尾フィールド = %q, want HeatStress", last.Name)
	}
	if last.Type != reflect.TypeOf(HeatStressPanelView{}) {
		t.Errorf("HeatStress の型 = %v, want HeatStressPanelView", last.Type)
	}
}

// DeviceChartAreaView へ HeatStress を詰めて代入できる（handler 配線の前提）。
func TestDeviceChartAreaView_HeatStress代入可能(t *testing.T) {
	area := DeviceChartAreaView{HeatStress: baselineHeatStressPanelView()}
	if !area.HeatStress.HasData {
		t.Errorf("HeatStress が DeviceChartAreaView に保持されない")
	}
}
