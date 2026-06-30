package chart

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

// 高温ストレス描画層。HeatStressChartSpec から THI 時間帯×日 heatmap・熱帯夜 calendar・
// 夜温/ΔT line・AH line・年間日数トレンド bar の各 option JSON を構築する
// （派生指標パネルの第4弾・VPD/露点/GDD と同型の *_echarts.go）。
//
// heatmap/calendar/visualMap は go-echarts v2.7.2 ネイティブ型（charts.HeatMap・opts.Calendar・
// opts.VisualMap）で組む。base.go の MarshalJSON 経路が calendar/visualMap を camelCase 出力するため
// （research §2・Decision D1/Q1）、本リポジトリ初導入でも小文字キー自前注入は不要。
// トレンドの Sen 傾き markLine だけは go-echarts の MarkLineData が非準拠キーを吐くため
// GDD/VPD と同型に小文字キー自前注入で補う。
//
// 不変条件:
//   - 返り値は encoding/json（SetEscapeHTML=true 既定）でシリアライズ済みで `</script>` は混入しない（§10-E）。
//   - visualMap の InRange.Color は低→高で薄黄→橙→赤→最濃側（--color-heat）の単調暖色＝暑熱が濃い暖色（要件 7.1）。
//   - 当該系列の主データが空のとき空 option `"{}"` を返し破綻しない（要件 3.5/4.4/5.4/6.4）。

// 高温ストレス各系列の凡例名（系列名）。
const (
	seriesNameTHIHeat      = "THI"
	seriesNameTropicalCal  = "夜温"      // 熱帯夜 calendar（夜温の濃淡）
	seriesNameNightTemp    = "夜温推移"   // 夜温推移 line
	seriesNameDeltaT       = "日較差ΔT"  // 日較差ΔT line
	seriesNameAH           = "絶対湿度AH" // AH line
	seriesNameTropicalYear = "熱帯夜日数"  // 年間日数 bar
	seriesNameSenTrend     = "Sen傾き"   // Sen 傾き markLine
)

// emptyOption は当該系列の主データが空のときに返す空 option（クライアントの setOption({}) は no-op）。
const emptyOption = "{}"

// heatColorScale は visualMap の連続暖色スケール（低→高＝薄黄→橙→赤→最濃側 highColor）を返す。
// 暑熱＝暖色の物理規約（project_vpd_physics_convention）。highColor は --color-heat（最も暑い＝危険側）。
// モック凡例 .heat-scale-bar のグラデ（#fff5b1→#ffa94d→#f03e3e→--color-heat）と同並び。
func heatColorScale(highColor string) []string {
	if highColor == "" {
		highColor = "#d6336c"
	}
	return []string{"#fff5b1", "#ffa94d", "#f03e3e", highColor}
}

// heatVisualMap は暑熱=暖色の連続 visualMap（水平・中央・ドラッグハンドル付き）を返す。
func heatVisualMap(min, max float64, highColor string) opts.VisualMap {
	return opts.VisualMap{
		Type:       "continuous",
		Calculable: opts.Bool(true),
		Min:        float32(min),
		Max:        float32(max),
		InRange:    &opts.VisualMapInRange{Color: heatColorScale(highColor)},
		Orient:     "horizontal",
		Left:       "center",
	}
}

// marshalOption は go-echarts チャートの option マップを HTML 安全な JSON 文字列にする
// （encoding/json 既定の SetEscapeHTML=true で < > & を \uXXXX 化）。
func marshalOption(label string, chart interface{ JSON() map[string]any }) (string, error) {
	out, err := json.Marshal(chart.JSON())
	if err != nil {
		return "", fmt.Errorf("chart: %s option の JSON 化に失敗: %w", label, err)
	}
	return string(out), nil
}

// hourCategoryLabels は THI heatmap の x 軸（時刻 0..23）カテゴリラベルを返す。
func hourCategoryLabels() []string {
	labels := make([]string, 24)
	for h := 0; h < 24; h++ {
		labels[h] = strconv.Itoa(h)
	}
	return labels
}

// thiHeatData は HeatCell 列を heatmap データ点 [Hour, Day, Value]（x,y,value）へ変換する。
func thiHeatData(cells []HeatCell) []opts.HeatMapData {
	data := make([]opts.HeatMapData, len(cells))
	for i, c := range cells {
		data[i] = opts.HeatMapData{Value: []any{c.Hour, c.Day, c.Value}}
	}
	return data
}

// calendarHeatData は DateValue 列を calendar heatmap データ点 ["YYYY-MM-DD", Value] へ変換する。
func calendarHeatData(cells []DateValue) []opts.HeatMapData {
	data := make([]opts.HeatMapData, len(cells))
	for i, c := range cells {
		data[i] = opts.HeatMapData{Value: []any{c.Date, c.Value}}
	}
	return data
}

// barData は値列を bar データ点へ変換する。
func barData(values []float64) []opts.BarData {
	data := make([]opts.BarData, len(values))
	for i, v := range values {
		data[i] = opts.BarData{Value: v}
	}
	return data
}

// THIHourDayHeatmapOptionJSON は THI 時間帯（x=0..23）×日（y）のヒートマップ option を構築する
// （要件 4.1, 4.2, 4.4, 7.1）。連続暖色 visualMap（高 THI ほど濃い暖色）。空データは "{}"。
func THIHourDayHeatmapOptionJSON(spec HeatStressChartSpec) (string, error) {
	if len(spec.THIHourDay) == 0 {
		return emptyOption, nil
	}
	hm := charts.NewHeatMap()
	hm.SetGlobalOptions(
		charts.WithTooltipOpts(opts.Tooltip{Trigger: "item"}),
		charts.WithXAxisOpts(opts.XAxis{Type: "category"}),
		charts.WithYAxisOpts(opts.YAxis{Type: "category", Data: spec.THIDayLabels}),
		charts.WithVisualMapOpts(heatVisualMap(spec.THIMin, spec.THIMax, spec.Color)),
	)
	hm.SetXAxis(hourCategoryLabels())
	hm.AddSeries(seriesNameTHIHeat, thiHeatData(spec.THIHourDay))
	hm.Validate()
	return marshalOption("THI heatmap", hm)
}

// TropicalNightCalendarOptionJSON は熱帯夜カレンダー（暦日×月・直近1年）の option を構築する
// （要件 3.1, 3.2, 3.5, 7.1）。calendar 座標系 + heatmap + 連続暖色 visualMap（夜温が高いほど濃い暖色）。
// AddCalendar が hasXYAxis=false に切替えるため直交 xAxis/yAxis は出ない。空データは "{}"。
func TropicalNightCalendarOptionJSON(spec HeatStressChartSpec) (string, error) {
	if len(spec.CalendarCells) == 0 {
		return emptyOption, nil
	}
	hm := charts.NewHeatMap()
	hm.SetGlobalOptions(
		charts.WithTooltipOpts(opts.Tooltip{Trigger: "item"}),
		charts.WithVisualMapOpts(heatVisualMap(spec.NightMin, spec.NightMax, spec.Color)),
	)
	hm.AddCalendar(&opts.Calendar{
		Range:  spec.CalendarRange,
		Orient: "horizontal",
	})
	hm.AddSeries(seriesNameTropicalCal, calendarHeatData(spec.CalendarCells),
		charts.WithCoordinateSystem("calendar"),
		charts.WithCalendarIndex(0),
	)
	hm.Validate()
	return marshalOption("calendar", hm)
}

// NightTempDeltaLineOptionJSON は夜温推移＋日較差ΔT の2 line option を構築する
// （要件 3.3, 5.2, 5.3）。両系列とも空なら "{}"。
func NightTempDeltaLineOptionJSON(spec HeatStressChartSpec) (string, error) {
	if len(spec.NightTemps) == 0 && len(spec.DeltaT) == 0 {
		return emptyOption, nil
	}
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTooltipOpts(opts.Tooltip{
			Trigger:     "axis",
			AxisPointer: &opts.AxisPointer{Type: "cross"},
		}),
		charts.WithXAxisOpts(opts.XAxis{Type: "category"}),
		charts.WithYAxisOpts(opts.YAxis{Type: "value", Scale: opts.Bool(true)}),
		charts.WithLegendOpts(opts.Legend{
			Show: opts.Bool(true),
			Data: []string{seriesNameNightTemp, seriesNameDeltaT},
		}),
	)
	line.SetXAxis(spec.DayLabels)
	line.AddSeries(seriesNameNightTemp, lineData(spec.NightTemps),
		charts.WithLineStyleOpts(opts.LineStyle{Color: spec.Color}),
	)
	line.AddSeries(seriesNameDeltaT, lineData(spec.DeltaT),
		charts.WithLineStyleOpts(opts.LineStyle{Color: spec.Color, Width: 1}),
		charts.WithLineChartOpts(opts.LineChart{ShowSymbol: opts.Bool(false)}),
	)
	line.Validate()
	return marshalOption("night/delta line", line)
}

// AHLineOptionJSON は絶対湿度 AH（除湿負荷）の line option を構築する（要件 5.1, 5.4）。空データは "{}"。
func AHLineOptionJSON(spec HeatStressChartSpec) (string, error) {
	if len(spec.AH) == 0 {
		return emptyOption, nil
	}
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTooltipOpts(opts.Tooltip{
			Trigger:     "axis",
			AxisPointer: &opts.AxisPointer{Type: "cross"},
		}),
		charts.WithXAxisOpts(opts.XAxis{Type: "category"}),
		charts.WithYAxisOpts(opts.YAxis{Type: "value", Scale: opts.Bool(true)}),
	)
	line.SetXAxis(spec.AHLabels)
	line.AddSeries(seriesNameAH, lineData(spec.AH),
		charts.WithLineStyleOpts(opts.LineStyle{Color: spec.Color}),
	)
	line.Validate()
	return marshalOption("AH line", line)
}

// TropicalNightTrendOptionJSON は熱帯夜年間日数の棒 + Sen 傾き markLine の option を構築する
// （要件 6.1）。HasTrend=false（年系列1点以下）のときは描かず "{}" を返す。
// Sen 傾き線は go-echarts の MarkLineData が非準拠キーを吐くため、GDD と同型に小文字キーで自前注入する。
func TropicalNightTrendOptionJSON(spec HeatStressChartSpec) (string, error) {
	if !spec.HasTrend || len(spec.YearlyCounts) == 0 {
		return emptyOption, nil
	}
	bar := charts.NewBar()
	bar.SetGlobalOptions(
		charts.WithTooltipOpts(opts.Tooltip{
			Trigger:     "axis",
			AxisPointer: &opts.AxisPointer{Type: "shadow"},
		}),
		charts.WithXAxisOpts(opts.XAxis{Type: "category"}),
		charts.WithYAxisOpts(opts.YAxis{Type: "value", Name: "日/年"}),
	)
	bar.SetXAxis(spec.YearLabels)
	bar.AddSeries(seriesNameTropicalYear, barData(spec.YearlyCounts),
		charts.WithItemStyleOpts(opts.ItemStyle{Color: spec.Color}),
	)
	bar.Validate()

	base, err := json.Marshal(bar.JSON())
	if err != nil {
		return "", fmt.Errorf("chart: trend option の JSON 化に失敗: %w", err)
	}
	var optionMap map[string]any
	if err := json.Unmarshal(base, &optionMap); err != nil {
		return "", fmt.Errorf("chart: trend option の再構築に失敗: %w", err)
	}
	if err := injectSenTrendMarkLine(optionMap, spec); err != nil {
		return "", err
	}
	out, err := json.Marshal(optionMap)
	if err != nil {
		return "", fmt.Errorf("chart: trend option の HTML 安全化に失敗: %w", err)
	}
	return string(out), nil
}

// injectSenTrendMarkLine は series[0] へ Sen 傾き線（始点→終点の2点 markLine）を
// ECharts 準拠の小文字 coord キーで注入する。SenLine が2点未満なら markLine を出さない（棒のみ）。
func injectSenTrendMarkLine(optionMap map[string]any, spec HeatStressChartSpec) error {
	if len(spec.SenLine) < 2 || len(spec.YearLabels) < 2 {
		return nil // 棒のみ（Sen 線を引くだけの年数がない）
	}
	series, ok := optionMap["series"].([]any)
	if !ok || len(series) == 0 {
		return fmt.Errorf("chart: trend option に series[0] が無い")
	}
	head, ok := series[0].(map[string]any)
	if !ok {
		return fmt.Errorf("chart: trend option の series[0] が想定形でない")
	}
	last := len(spec.SenLine) - 1
	lastLabel := len(spec.YearLabels) - 1
	head["markLine"] = map[string]any{
		"silent": true,
		"symbol": []any{"none", "none"},
		"data": []any{
			[]any{
				map[string]any{"coord": []any{spec.YearLabels[0], spec.SenLine[0]}, "name": seriesNameSenTrend},
				map[string]any{"coord": []any{spec.YearLabels[lastLabel], spec.SenLine[last]}},
			},
		},
	}
	return nil
}
