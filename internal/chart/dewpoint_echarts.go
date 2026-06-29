package chart

import (
	"encoding/json"
	"fmt"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

// 露点グラフの凡例名（凡例トグル・legend.selected のキー）。
const (
	seriesNameDewpoint = "露点"
	seriesNameDewTemp  = "気温"
)

// dewTempOverlayColor は気温重ね線（series[1]）の色。温度=暖色を踏襲し、
// 「気温が露点（寒色）に接近するほど結露しやすい」関係を一目で見せる（device_show.go の tempLineColor と同値）。
// グラフ内部描画ゆえモック反映対象外（feedback_mock_graph_rendering_exception）。
const dewTempOverlayColor = "#e8590c"

// condensationBandColor は結露帯 markArea の塗り色（寒色＝湿り側・半透明・物理規約）。
// 結露・葉面湿潤・高湿度は VPD の「湿り側＝寒色」と一貫した向きで表現する（要件 2.4・
// project_vpd_physics_convention）。露点線の基準色 --color-dewpoint と同じ寒色系の青。
// グラフ内部描画ゆえモック反映対象外（feedback_mock_graph_rendering_exception）。
const condensationBandColor = "rgba(66,99,235,0.12)"

// DewpointChartOptionJSON は露点 Td 折れ線＋気温 T 重ね線＋結露帯 markArea の
// Apache ECharts option を構築し、<script type="application/json"> 埋込用の HTML 安全な JSON を返す。
//
// 系列構成（気温が露点に接近する=結露しやすさを見せる）:
//   - series[0]: 露点 Td 線（主役・基準色 DewColor＝寒色＝湿り側）。結露帯 markArea の付与先。
//   - series[1]: 気温 T 重ね線（温度色・細線・symbol 非表示）。
//
// 結露帯 markArea（時間区間ハイライト＝xAxis 範囲）:
//   - Condensation の各 Run（spread ≤ しきい値の連続区間・湿り側）を寒色帯でハイライトする。
//   - go-echarts の MarkAreaData は JSON タグ非準拠（大文字 D-1）ゆえ使用せず、P5 injectGapMarkArea と
//     同型の ECharts 準拠の小文字 `xAxis` キー markArea を自前構築し series[0] へ注入する。
//   - 空区間（結露帯なし）では markArea を付与しない（series[0] そのまま）。
//
// y 軸は Scale:true の auto 範囲（気温と露点は同℃レンジゆえ VPD の YMax 固定は不要）。
//
// 不変条件: 返り値は encoding/json（SetEscapeHTML=true 既定）でシリアライズ済み。
// 外部入力（時刻ラベル）由来の < > & は \uXXXX 化され `</script>` は混入しない（§10-E）。
// 温湿度/VPD option には影響しない（別関数・無回帰）。
func DewpointChartOptionJSON(spec DewpointChartSpec) (string, error) {
	line := charts.NewLine()
	line.SetGlobalOptions(
		// 十字ホバー + 値/時刻ツールチップ（温湿度/VPD と同経路で connect 連動可能）。
		charts.WithTooltipOpts(opts.Tooltip{
			Trigger:     "axis",
			AxisPointer: &opts.AxisPointer{Type: "cross"},
		}),
		charts.WithXAxisOpts(opts.XAxis{Type: "category"}),
		// y 軸は Scale=true でデータ範囲に追従（0 を強制しない＝気温/露点の接近を見せる・氷点下も可）。
		charts.WithYAxisOpts(opts.YAxis{Type: "value", Scale: opts.Bool(true)}),
		// 露点 Td・気温の2本を凡例で区別（両方とも主役ゆえ既定表示）。
		charts.WithLegendOpts(opts.Legend{
			Show: opts.Bool(true),
			Data: []string{seriesNameDewpoint, seriesNameDewTemp},
		}),
	)
	line.SetXAxis(spec.Labels)

	// series[0]: 露点 Td 線（主役・寒色＝湿り側）。結露帯 markArea の付与先。
	// itemStyle も同色にして凡例マーカー/シンボルを線色（寒色）に揃える（既定パレットへのフォールバック回避）。
	line.AddSeries(seriesNameDewpoint, lineData(spec.Dewpoint),
		charts.WithLineStyleOpts(opts.LineStyle{Color: spec.DewColor}),
		charts.WithItemStyleOpts(opts.ItemStyle{Color: spec.DewColor}),
	)
	// series[1]: 気温 T 重ね線（温度色・細線・symbol 非表示＝主役の露点を邪魔しない）。
	// itemStyle も暖色にして凡例マーカーを線色（橙）に揃える（既定パレットの緑になる不一致を防ぐ）。
	line.AddSeries(seriesNameDewTemp, lineData(spec.Temperature),
		charts.WithLineStyleOpts(opts.LineStyle{Color: dewTempOverlayColor, Width: 1}),
		charts.WithItemStyleOpts(opts.ItemStyle{Color: dewTempOverlayColor}),
		charts.WithLineChartOpts(opts.LineChart{ShowSymbol: opts.Bool(false)}),
	)

	// Validate() で xAxis データを option へ反映してから option マップを取り出す。
	line.Validate()

	base, err := json.Marshal(line.JSON())
	if err != nil {
		return "", fmt.Errorf("chart: 露点 option の JSON 化に失敗: %w", err)
	}
	var optionMap map[string]any
	if err := json.Unmarshal(base, &optionMap); err != nil {
		return "", fmt.Errorf("chart: 露点 option の再構築に失敗: %w", err)
	}
	if err := injectCondensationMarkArea(optionMap, spec.Condensation); err != nil {
		return "", err
	}

	// 再シリアライズ（SetEscapeHTML=true 既定）で < > & を \uXXXX 化し HTML 安全化する。
	out, err := json.Marshal(optionMap)
	if err != nil {
		return "", fmt.Errorf("chart: 露点 option の HTML 安全化に失敗: %w", err)
	}
	return string(out), nil
}

// injectCondensationMarkArea は series[0] へ結露帯（各 Run の xAxis 範囲）の寒色 markArea を注入する。
// P5 injectGapMarkArea と同型の小文字 `xAxis` キーで構築する（go-echarts 大文字キー不具合 D-1 回避）。
// 空区間のときは無注入で原 option のまま（結露帯なし＝series[0] そのまま）。
// P5 の gapZone（灰色固定）は改変せず、結露帯専用の寒色ゾーンを本ファイルに起こす（無回帰優先）。
func injectCondensationMarkArea(optionMap map[string]any, runs []Run) error {
	if len(runs) == 0 {
		return nil
	}
	series, ok := optionMap["series"].([]any)
	if !ok || len(series) == 0 {
		return fmt.Errorf("chart: 露点 option に series[0] が無い")
	}
	head, ok := series[0].(map[string]any)
	if !ok {
		return fmt.Errorf("chart: 露点 option の series[0] が想定形でない")
	}
	zones := make([]any, 0, len(runs))
	for _, r := range runs {
		zones = append(zones, condensationZone(r.StartIdx, r.EndIdx))
	}
	head["markArea"] = map[string]any{
		"silent": true,
		"data":   zones,
	}
	return nil
}

// condensationZone は結露帯区間 [startIdx, endIdx] の xAxis 範囲 markArea ゾーン（寒色帯1本）を構築する。
// 始点に itemStyle.color（寒色＝湿り側の塗り色）を持たせ、両端は ECharts 準拠の小文字 xAxis キーで指定する。
func condensationZone(startIdx, endIdx int) []any {
	return []any{
		map[string]any{"xAxis": startIdx, "itemStyle": map[string]any{"color": condensationBandColor}},
		map[string]any{"xAxis": endIdx},
	}
}
