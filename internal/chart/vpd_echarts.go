package chart

import (
	"encoding/json"
	"fmt"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

// VPD グラフの凡例名（凡例トグル・legend.selected のキー）。
const (
	seriesNameVPD    = "VPD"
	seriesNameVPDSMA = "VPD移動平均"
)

// 適正帯3ゾーンの塗り色（markArea・半透明）。VPD は乾燥度指標ゆえ
// 乾きすぎ=高VPD側=暖色／適正=中立緑／湿りすぎ=低VPD側=寒色。
// グラフ内部描画ゆえモック反映対象外（feedback_mock_graph_rendering_exception）。
const (
	vpdZoneDryColor     = "rgba(230,126,34,0.10)" // 乾きすぎ [upper, YMax]（高VPD=乾燥）
	vpdZoneOptimalColor = "rgba(64,160,90,0.12)"  // 適正 [lower, upper]（中立色）
	vpdZoneWetColor     = "rgba(52,120,200,0.10)" // 湿りすぎ [0, lower]（低VPD=多湿）
)

// VPDChartOptionJSON は VPD 折れ線＋適正帯3ゾーン markArea＋VPD移動平均(SMA)の
// Apache ECharts option を構築し、<script type="application/json"> 埋込用の HTML 安全な JSON を返す。
//
// 系列構成（クラッタ回避のため主役は VPD 実測線・移動平均は凡例で既定オフ）:
//   - series[0]: VPD 実測線（基準色・markPoint max/min）。client の endLabel/sampling 対象を温存。
//   - VPD移動平均: SMA 指定時のみ。細線・symbol 非表示・凡例「VPD移動平均」selected:false（要件 7）。
//
// 適正帯3ゾーン markArea（VPD は乾燥度指標＝低VPD が多湿・高VPD が乾燥）:
//   - [0, lower]     湿りすぎ（低VPD=多湿）
//   - [lower, upper] 適正（中立色）
//   - [upper, YMax]  乾きすぎ（高VPD=乾燥）
//
// go-echarts の MarkAreaData.YAxis は JSON タグが非準拠（"YAxis"・D-1）ゆえ使用せず、
// ECharts 準拠の小文字 `yAxis` キーを持つ markArea を自前構築し option マップの series[0] へ注入する。
//
// y 軸は min:0・max:YMax 固定で3ゾーンを常時可視にする（YMax は handler が算出）。
//
// 不変条件: 返り値は encoding/json（SetEscapeHTML=true 既定）でシリアライズ済み。
// 外部入力（時刻ラベル）由来の < > & は \uXXXX 化され `</script>` は混入しない（§10-E）。
func VPDChartOptionJSON(spec VPDChartSpec) (string, error) {
	hasSMA := len(spec.SMA) > 0

	line := charts.NewLine()

	global := []charts.GlobalOpts{
		// 十字ホバー + 値/時刻ツールチップ（温湿度と同経路で connect 連動可能）。
		charts.WithTooltipOpts(opts.Tooltip{
			Trigger:     "axis",
			AxisPointer: &opts.AxisPointer{Type: "cross"},
		}),
		charts.WithXAxisOpts(opts.XAxis{Type: "category"}),
		// 3ゾーン（特に高VPD側の乾きすぎ）を常に見せるため min:0・max:YMax 固定（温湿度の Scale:true とは別方針）。
		charts.WithYAxisOpts(opts.YAxis{Type: "value", Min: 0, Max: spec.YMax}),
	}

	// 移動平均を出すときのみ凡例（VPD 実測線を主役・移動平均は既定オフ）。
	if hasSMA {
		global = append(global, charts.WithLegendOpts(opts.Legend{
			Show:     opts.Bool(true),
			Data:     []string{seriesNameVPD, seriesNameVPDSMA},
			Selected: map[string]bool{seriesNameVPDSMA: false},
		}))
	}

	line.SetGlobalOptions(global...)
	line.SetXAxis(spec.Labels)

	// series[0]: VPD 実測線（主役）。基準色・markPoint max/min。
	line.AddSeries(seriesNameVPD, lineData(spec.VPD),
		charts.WithLineStyleOpts(opts.LineStyle{Color: spec.Color}),
		charts.WithMarkPointNameTypeItemOpts(
			opts.MarkPointNameTypeItem{Type: "max"},
			opts.MarkPointNameTypeItem{Type: "min"},
		),
	)

	// VPD移動平均（SMA）: 細線・symbol 非表示・既定オフ。EMA/WMA は作らない（SMA 1本のみ）。
	if hasSMA {
		line.AddSeries(seriesNameVPDSMA, lineData(spec.SMA),
			charts.WithLineStyleOpts(opts.LineStyle{Color: spec.Color, Width: 1}),
			charts.WithLineChartOpts(opts.LineChart{ShowSymbol: opts.Bool(false)}),
		)
	}

	// Validate() で xAxis データを option へ反映してから option マップを取り出す。
	line.Validate()

	// option マップを一旦 JSON 化 → 汎用マップへ戻し、series[0] に適正帯3ゾーン markArea を注入する。
	base, err := json.Marshal(line.JSON())
	if err != nil {
		return "", fmt.Errorf("chart: VPD option の JSON 化に失敗: %w", err)
	}
	var optionMap map[string]any
	if err := json.Unmarshal(base, &optionMap); err != nil {
		return "", fmt.Errorf("chart: VPD option の再構築に失敗: %w", err)
	}
	if err := injectVPDMarkArea(optionMap, spec.Lower, spec.Upper, spec.YMax); err != nil {
		return "", err
	}

	// 再シリアライズ（SetEscapeHTML=true 既定）で < > & を \uXXXX 化し HTML 安全化する。
	out, err := json.Marshal(optionMap)
	if err != nil {
		return "", fmt.Errorf("chart: VPD option の HTML 安全化に失敗: %w", err)
	}
	return string(out), nil
}

// injectVPDMarkArea は series[0] へ適正帯3ゾーンの markArea を正しい小文字 yAxis キーで注入する。
func injectVPDMarkArea(optionMap map[string]any, lower, upper, ymax float64) error {
	series, ok := optionMap["series"].([]any)
	if !ok || len(series) == 0 {
		return fmt.Errorf("chart: VPD option に series[0] が無い")
	}
	head, ok := series[0].(map[string]any)
	if !ok {
		return fmt.Errorf("chart: VPD option の series[0] が想定形でない")
	}
	head["markArea"] = map[string]any{
		"silent": true,
		"data": []any{
			vpdZone(0, lower, vpdZoneWetColor),     // [0,lower] 低VPD=湿りすぎ（寒色）
			vpdZone(lower, upper, vpdZoneOptimalColor),
			vpdZone(upper, ymax, vpdZoneDryColor),  // [upper,YMax] 高VPD=乾きすぎ（暖色）
		},
	}
	return nil
}

// vpdZone は markArea の1ゾーン（[lo,hi] の水平帯）を構築する。
// 始点に itemStyle.color（塗り色）を持たせ、両端は ECharts 準拠の小文字 yAxis キーで指定する。
func vpdZone(lo, hi float64, color string) []any {
	return []any{
		map[string]any{"yAxis": lo, "itemStyle": map[string]any{"color": color}},
		map[string]any{"yAxis": hi},
	}
}
