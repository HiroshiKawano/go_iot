package chart

import (
	"encoding/json"
	"fmt"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

// オーバーレイ系列の凡例名（凡例トグルと legend.selected のキーになる）。
const (
	seriesNameSMA       = "移動平均"
	seriesNameBand      = "正常帯"
	seriesNameBandLower = "帯下限" // 凡例には出さない透明ベース線
	seriesNameDeviation = "乖離率(%)"
	bandStackGroup      = "band" // 帯下限＋帯幅の積み上げグループ名
)

// ChartOptionJSON は複数系列の折れ線について Apache ECharts の option を構築し、
// <script type="application/json"> 埋込用の HTML 安全な JSON 文字列を返す。
//
// 系列構成（クラッタ回避のため主役は生実測線・オーバーレイは凡例で既定オフ）:
//   - series[0]: 生実測線（Unit・基準色・markPoint max/min）。client の endLabel/sampling 対象を温存（R7.3/7.5）。
//   - 移動平均: SMA 指定時のみ。細線・symbol 非表示・凡例「移動平均」selected:false（R2）。
//   - 正常帯: BandLower/BandWidth 指定時のみ。帯下限（透明線・stack）＋帯幅（半透明 area・stack）の2系列。
//     帯下限は凡例に出さず常時透明描画、帯幅のみ凡例「正常帯」selected:false でトグル（R3）。
//   - 乖離率: Deviation 指定時のみ。第2 y軸（右・%）へ点線・symbol 非表示・凡例「乖離率(%)」selected:false（R4）。
//
// endLabel（右端現在値）と sampling("lttb") は本関数では構築せず、クライアント側の
// 初期化スクリプト（EChartsInitializer）が series[0] へ付与する（R7.3・client 無変更）。
//
// 不変条件: 返り値は encoding/json（SetEscapeHTML=true 既定）でシリアライズ済み。
// 外部入力（時刻ラベル）由来の < > & は \uXXXX 化され、生タグ/`</script>` は混入しない（§10-E）。
func ChartOptionJSON(spec ChartSpec) (string, error) {
	hasSMA := len(spec.SMA) > 0
	hasBand := len(spec.BandLower) > 0 && len(spec.BandWidth) > 0
	hasDeviation := len(spec.Deviation) > 0
	hasOverlay := hasSMA || hasBand || hasDeviation

	line := charts.NewLine()

	global := []charts.GlobalOpts{
		// 十字ホバー + 値/時刻ツールチップ。
		charts.WithTooltipOpts(opts.Tooltip{
			Trigger:     "axis",
			AxisPointer: &opts.AxisPointer{Type: "cross"},
		}),
		// X 軸はカテゴリ（ラベル列）、Y 軸は Scale=true でデータ範囲に追従（0 を強制しない）。
		charts.WithXAxisOpts(opts.XAxis{Type: "category"}),
		charts.WithYAxisOpts(opts.YAxis{Type: "value", Scale: opts.Bool(true)}),
	}

	// オーバーレイがあるときのみ凡例を出す（生線のみではクラッタ回避で凡例なし）。
	// 凡例 data には「生実測 + 出すオーバーレイ」を載せ、オーバーレイは selected:false で既定オフ。
	// 帯下限は凡例 data に載せず（項目を出さず）常時透明描画する。
	if hasOverlay {
		legendData := []string{spec.Unit}
		selected := map[string]bool{}
		if hasSMA {
			legendData = append(legendData, seriesNameSMA)
			selected[seriesNameSMA] = false
		}
		if hasBand {
			legendData = append(legendData, seriesNameBand)
			selected[seriesNameBand] = false
		}
		if hasDeviation {
			legendData = append(legendData, seriesNameDeviation)
			selected[seriesNameDeviation] = false
		}
		global = append(global, charts.WithLegendOpts(opts.Legend{
			Show:     opts.Bool(true),
			Data:     legendData,
			Selected: selected,
		}))
	}

	line.SetGlobalOptions(global...)

	// 乖離率用の第2 y軸（右・%）。SetGlobalOptions が YAxisList[0] を設定した後に append する。
	if hasDeviation {
		line.ExtendYAxis(opts.YAxis{Type: "value", Scale: opts.Bool(true), Position: "right", Name: "%"})
	}

	line.SetXAxis(spec.Labels)

	// series[0]: 生実測線（主役）。markPoint max/min・基準色。
	line.AddSeries(spec.Unit, lineData(spec.Raw),
		charts.WithLineStyleOpts(opts.LineStyle{Color: spec.Color}),
		charts.WithMarkPointNameTypeItemOpts(
			opts.MarkPointNameTypeItem{Type: "max"}, // 最高
			opts.MarkPointNameTypeItem{Type: "min"}, // 最低
		),
	)

	// 移動平均（SMA）: 細線・symbol 非表示。EMA/WMA は作らない（SMA 1本のみ）。
	if hasSMA {
		line.AddSeries(seriesNameSMA, lineData(spec.SMA),
			charts.WithLineStyleOpts(opts.LineStyle{Color: spec.Color, Width: 1}),
			charts.WithLineChartOpts(opts.LineChart{ShowSymbol: opts.Bool(false)}),
		)
	}

	// 正常帯: 帯下限（透明線・stack）→ 帯幅（半透明 area・stack）。帯幅のみ凡例トグル対象。
	// Opacity は明示ポインタ（opts.Float）で渡し、omitempty による省略（透明指定の消失）を防ぐ。
	if hasBand {
		line.AddSeries(seriesNameBandLower, lineData(spec.BandLower),
			charts.WithLineStyleOpts(opts.LineStyle{Opacity: opts.Float(0)}),
			charts.WithLineChartOpts(opts.LineChart{Stack: bandStackGroup, ShowSymbol: opts.Bool(false)}),
		)
		line.AddSeries(seriesNameBand, lineData(spec.BandWidth),
			charts.WithLineStyleOpts(opts.LineStyle{Opacity: opts.Float(0)}),
			charts.WithAreaStyleOpts(opts.AreaStyle{Color: spec.Color, Opacity: opts.Float(0.15)}),
			charts.WithLineChartOpts(opts.LineChart{Stack: bandStackGroup, ShowSymbol: opts.Bool(false)}),
		)
	}

	// 乖離率: 第2 y軸（YAxisIndex=1）・点線・symbol 非表示。nil 要素は欠落点。
	if hasDeviation {
		line.AddSeries(seriesNameDeviation, deviationData(spec.Deviation),
			charts.WithLineStyleOpts(opts.LineStyle{Color: spec.Color, Type: "dotted"}),
			charts.WithLineChartOpts(opts.LineChart{YAxisIndex: 1, ShowSymbol: opts.Bool(false)}),
		)
	}

	// Validate() で xAxis データを option へ反映してから option マップを取り出す。
	line.Validate()

	// option マップを encoding/json で再シリアライズして HTML 安全化する（< > & を \uXXXX 化）。
	bs, err := json.Marshal(line.JSON())
	if err != nil {
		return "", fmt.Errorf("chart: ECharts option の JSON 化に失敗: %w", err)
	}
	return string(bs), nil
}

// lineData は []float64 を opts.LineData 列へ変換する。
func lineData(values []float64) []opts.LineData {
	data := make([]opts.LineData, len(values))
	for i, v := range values {
		data[i] = opts.LineData{Value: v}
	}
	return data
}

// deviationData は乖離率（[]*float64）を opts.LineData 列へ変換する。
// nil（未定義）は Value 未設定（omitempty で値なし＝ECharts は欠落点として描画しない）。
func deviationData(values []*float64) []opts.LineData {
	data := make([]opts.LineData, len(values))
	for i, p := range values {
		if p != nil {
			data[i] = opts.LineData{Value: *p}
		}
	}
	return data
}
