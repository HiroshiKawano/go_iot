package chart

import (
	"encoding/json"
	"fmt"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

// LineOptionJSON は単一系列の折れ線について Apache ECharts の option を構築し、
// <script type="application/json"> 埋込用の HTML 安全な JSON 文字列を返す。
//
// 構築範囲（go-echarts が型安全に表現できる範囲に限定）:
//   - xAxis: category 軸 = series[0] の Label 列
//   - series: 1 本の line（実測値 Y 列）+ markPoint（最高 max / 最低 min）+ lineStyle.color
//   - tooltip: trigger="axis" + axisPointer type="cross"
//
// endLabel（右端現在値）と sampling("lttb") は本関数では構築しない。go-echarts opts に無く、
// クライアント側の初期化スクリプト（EChartsInitializer）が setOption 前に付与する。
//
// 事前条件: series の点数 > 0（空は呼び出し側で HasData=false 分岐するため本関数は呼ばれない）。
// 不変条件: 返り値は encoding/json（SetEscapeHTML=true 既定）でシリアライズ済み。
// 外部入力（時刻ラベル）由来の < > & は \uXXXX 化され、生タグ/`</script>` は混入しない（§10-E）。
func LineOptionJSON(series []Series, unit, color string) (string, error) {
	line := charts.NewLine()
	line.SetGlobalOptions(
		// 十字ホバー + 値/時刻ツールチップ（R3.1, 3.2）。
		charts.WithTooltipOpts(opts.Tooltip{
			Trigger:     "axis",
			AxisPointer: &opts.AxisPointer{Type: "cross"},
		}),
		// X 軸はカテゴリ（ラベル列）、Y 軸は数値。
		// Scale=true で 0 を強制せずデータ範囲に追従する相対表示にする（旧 SVG の paddedScale 相当）。
		// 0 始まり固定だと温度 0〜35 / 湿度 0〜100 の帯に値が偏り折れ線が平たく見えるため。
		charts.WithXAxisOpts(opts.XAxis{Type: "category"}),
		charts.WithYAxisOpts(opts.YAxis{Type: "value", Scale: opts.Bool(true)}),
	)

	// 単一系列前提: 先頭系列から X ラベルと Y 実測値を取り出す。
	var labels []string
	var data []opts.LineData
	if len(series) > 0 {
		points := series[0].Points
		labels = make([]string, len(points))
		data = make([]opts.LineData, len(points))
		for i, p := range points {
			labels[i] = p.Label
			data[i] = opts.LineData{Value: p.Y}
		}
	}

	line.SetXAxis(labels).
		AddSeries(unit, data,
			charts.WithLineStyleOpts(opts.LineStyle{Color: color}),
			charts.WithMarkPointNameTypeItemOpts(
				opts.MarkPointNameTypeItem{Type: "max"}, // 最高
				opts.MarkPointNameTypeItem{Type: "min"}, // 最低
			),
		)

	// Validate() で xAxis データを option へ反映してから option マップを取り出す。
	line.Validate()

	// go-echarts の RenderSnippet().Option / JSONNotEscaped は HTML-unescape 済みのため、
	// option マップを encoding/json で再シリアライズして HTML 安全化する（< > & を \uXXXX 化）。
	bs, err := json.Marshal(line.JSON())
	if err != nil {
		return "", fmt.Errorf("chart: ECharts option の JSON 化に失敗: %w", err)
	}
	return string(bs), nil
}
