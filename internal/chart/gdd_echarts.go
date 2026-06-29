package chart

import (
	"encoding/json"
	"fmt"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

// GDD 累積曲線の凡例名（主役単独系列）。
const seriesNameGDD = "累積GDD"

// GDDChartOptionJSON は定植日→現在の累積 GDD 曲線の Apache ECharts option を構築し、
// 目標 GDD 水平 markLine・予測到達日 垂直 markLine／予測点 markPoint を自前注入して
// <script type="application/json"> 埋込用の HTML 安全な JSON を返す（要件 3.1〜3.3, 3.6, 3.7）。
//
// 他パネル（温湿度/VPD/露点）との決定的な違い:
//   - x 軸は時刻 category でなく**経過日数の value 軸**。予測到達日（データ範囲外の未来）を
//     markLine の `xAxis: 数値` で表すため value 軸が必須で、series データは [経過日, 累積] 座標ペアで与える。
//   - dataZoom（inside/slider）で長期曲線（数十〜百数十日）を拡大・スクロールできる（要件 3.6）。
//   - 凡例は主役1系列ゆえ最小（クラッタ回避・§2-2）。生育ステージ閾値は markLine 群にせず**表**で見せる。
//
// go-echarts の MarkLineData/MarkPointData は JSON タグが ECharts 非準拠（大文字キー）ゆえ使用せず、
// vpd_echarts.go/gap_echarts.go と同型に option マップへ ECharts 準拠の小文字 `yAxis`/`xAxis`/`coord`
// キーを自前注入する。
//
// 不変条件: 返り値は encoding/json（SetEscapeHTML=true 既定）でシリアライズ済みで `</script>` は混入しない（§10-E）。
func GDDChartOptionJSON(spec GDDChartSpec) (string, error) {
	line := charts.NewLine()

	// 軸 max を明示し、目標 markLine（y=TargetGDD）と予測 markLine/markPoint（x=ForecastDay）を
	// 既定ビューで必ず可視にする。未指定だと dataZoom がデータ域（累積の最終点まで）のみを表示し、
	// データ範囲外の目標線（target > 最終累積）・予測マーク（forecastDay > 最終経過日）が見切れる。
	xMax, yMax := gddAxisMax(spec)

	line.SetGlobalOptions(
		// 十字ホバー（GDD は connect 非参加だが単体ツールチップは有効）。
		charts.WithTooltipOpts(opts.Tooltip{
			Trigger:     "axis",
			AxisPointer: &opts.AxisPointer{Type: "cross"},
		}),
		// x=経過日数の value 軸（min:0=定植日, max=予測到達日 or データ最終日に余白）。予測到達日を markLine xAxis 数値で表すため必須。
		charts.WithXAxisOpts(opts.XAxis{Type: "value", Min: 0, Max: xMax, Name: "経過日数"}),
		// y=累積 GDD の value 軸（min:0, max=収穫目標 GDD に余白）。目標 markLine を常に画面内に収める。
		charts.WithYAxisOpts(opts.YAxis{Type: "value", Min: 0, Max: yMax}),
		// 長期曲線の拡大・スクロール手段（要件 3.6）。
		charts.WithDataZoomOpts(
			opts.DataZoom{Type: "inside", Start: 0, End: 100},
			opts.DataZoom{Type: "slider", Start: 0, End: 100},
		),
	)

	// value 軸ゆえ SetXAxis（category 用データ）は呼ばず、series を [経過日, 累積] 座標ペアで与える。
	line.AddSeries(seriesNameGDD, gddCoordData(spec.ElapsedDays, spec.Cumulative),
		charts.WithLineStyleOpts(opts.LineStyle{Color: spec.Color}),
	)

	// Validate() で軸・系列を option へ反映してから option マップを取り出す。
	line.Validate()

	base, err := json.Marshal(line.JSON())
	if err != nil {
		return "", fmt.Errorf("chart: GDD option の JSON 化に失敗: %w", err)
	}
	var optionMap map[string]any
	if err := json.Unmarshal(base, &optionMap); err != nil {
		return "", fmt.Errorf("chart: GDD option の再構築に失敗: %w", err)
	}
	if err := injectGDDMarks(optionMap, spec); err != nil {
		return "", err
	}

	// 再シリアライズ（SetEscapeHTML=true 既定）で < > & を \uXXXX 化し HTML 安全化する。
	out, err := json.Marshal(optionMap)
	if err != nil {
		return "", fmt.Errorf("chart: GDD option の HTML 安全化に失敗: %w", err)
	}
	return string(out), nil
}

// gddAxisMax は目標線・予測マークを既定ビューに収める x/y 軸上限を返す。
//   - yMax: 収穫目標 GDD と最終累積の大きい方に余白（8%）。目標 markLine を常時可視にする。
//   - xMax: 最終経過日と（予測ありなら）予測到達日の大きい方に余白（5%）。予測 markLine/markPoint を可視にする。
//
// 余白を付けることでマークが軸端に貼り付かず見やすくなる。空入力でも 0 除算なく安全。
func gddAxisMax(spec GDDChartSpec) (xMax, yMax float64) {
	yMax = spec.TargetGDD
	for _, c := range spec.Cumulative {
		if c > yMax {
			yMax = c
		}
	}
	yMax *= 1.08

	for _, x := range spec.ElapsedDays {
		if x > xMax {
			xMax = x
		}
	}
	if spec.HasForecast && spec.ForecastDay > xMax {
		xMax = spec.ForecastDay
	}
	xMax *= 1.05
	return xMax, yMax
}

// gddCoordData は [経過日数, 累積] の座標ペア列を opts.LineData へ変換する（value 軸用）。
// 長さ不一致は短い方に合わせて防御的に扱う（純粋層と同方針・nullableLineData 類型）。
func gddCoordData(xs, ys []float64) []opts.LineData {
	n := len(xs)
	if len(ys) < n {
		n = len(ys)
	}
	data := make([]opts.LineData, n)
	for i := 0; i < n; i++ {
		data[i] = opts.LineData{Value: []float64{xs[i], ys[i]}}
	}
	return data
}

// injectGDDMarks は series[0] へ目標/予測のマークを ECharts 準拠の小文字キーで注入する。
//   - markLine: 目標 GDD 水平線 {yAxis: TargetGDD}（常時）＋ 予測到達日 垂直線 {xAxis: ForecastDay}（HasForecast 時）
//   - markPoint: 予測到達点 {coord: [ForecastDay, TargetGDD]}（HasForecast 時のみ）
//
// HasForecast=false（予測不能・到達済み）では予測の垂直線・点を出さず、目標水平線のみ残す（要件 3.7）。
func injectGDDMarks(optionMap map[string]any, spec GDDChartSpec) error {
	series, ok := optionMap["series"].([]any)
	if !ok || len(series) == 0 {
		return fmt.Errorf("chart: GDD option に series[0] が無い")
	}
	head, ok := series[0].(map[string]any)
	if !ok {
		return fmt.Errorf("chart: GDD option の series[0] が想定形でない")
	}

	// 目標 GDD 水平線（常時）。silent + 端点 symbol なしでクラッタを抑える。
	markLineData := []any{
		map[string]any{"yAxis": spec.TargetGDD, "name": "収穫目標"},
	}
	if spec.HasForecast {
		// 予測到達日 垂直線（value x 軸ゆえ data 範囲外の未来でも描ける）。
		markLineData = append(markLineData, map[string]any{"xAxis": spec.ForecastDay, "name": "予測到達"})
	}
	head["markLine"] = map[string]any{
		"silent": true,
		"symbol": []any{"none", "none"},
		"data":   markLineData,
	}

	if spec.HasForecast {
		// 予測到達点（累積曲線と目標線の交点の見込み）。
		head["markPoint"] = map[string]any{
			"data": []any{
				map[string]any{
					"coord": []any{spec.ForecastDay, spec.TargetGDD},
					"name":  "予測収穫",
				},
			},
		}
	}
	return nil
}
