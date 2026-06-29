package chart

import (
	"encoding/json"
	"fmt"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

// トレンドグラフの凡例名・stack グループ名（凡例トグル・legend.selected のキー）。
const (
	seriesNameTrendBand       = "範囲帯"   // min/max 帯（凡例トグル対象）
	seriesNameTrendBandLower  = "範囲帯下限" // 凡例に出さない透明ベース線
	seriesNameTrendCI         = "信頼区間"  // ブートストラップ CI 帯（凡例トグル対象）
	seriesNameTrendCILower    = "CI下限"  // 凡例に出さない透明ベース線
	seriesNameClimatology     = "平年比"
	trendBandStackGroup       = "trendband"             // min/max 帯の積み上げグループ
	trendCIStackGroup         = "trendci"               // CI 帯の積み上げグループ
	trendSignificantBandColor = "rgba(112,72,232,0.10)" // 有意区間 markArea の塗り色（--color-trend 系・薄紫）
)

// TrendChartOptionJSON は統計分析ページの長期トレンドグラフ option を構築する。
// ロールアップ平均を主役に、Sen トレンド線（markLine・2端点 coord）・有意区間（markArea・xAxis 範囲）・
// ブートストラップ CI 帯（積み上げ area）・min/max 帯（積み上げ area）・平年比（独立線）を重ね、
// 長期閲覧用 dataZoom（inside/slider）を含めて <script type="application/json"> 埋込用の
// HTML 安全な JSON を返す（要件 3.1, 5.1, 5.3, 6.4, 7.1）。
//
// go-echarts の MarkLineData/MarkAreaData は JSON タグが ECharts 非準拠（大文字キー）ゆえ使用せず、
// gdd_echarts.go/vpd_echarts.go/gap_echarts.go と同型に option マップへ ECharts 準拠の小文字
// `coord`/`xAxis` キーを自前注入する。
//
// 不変条件: 返り値は encoding/json（SetEscapeHTML=true 既定）でシリアライズ済みで `</script>` は混入しない（§10-E）。
func TrendChartOptionJSON(spec TrendChartSpec) (string, error) {
	hasBand := len(spec.BandLower) > 0 && len(spec.BandUpper) > 0
	hasCI := len(spec.CILower) > 0 && len(spec.CIUpper) > 0
	hasClim := len(spec.Climatology) > 0
	hasOverlay := hasBand || hasCI || hasClim

	line := charts.NewLine()

	global := []charts.GlobalOpts{
		charts.WithTooltipOpts(opts.Tooltip{
			Trigger:     "axis",
			AxisPointer: &opts.AxisPointer{Type: "cross"},
		}),
		// X 軸はカテゴリ（月次/年次ラベル）、Y 軸は Scale=true でデータ範囲に追従。
		charts.WithXAxisOpts(opts.XAxis{Type: "category"}),
		charts.WithYAxisOpts(opts.YAxis{Type: "value", Scale: opts.Bool(true)}),
		// 長期（多年・多月）の拡大・スクロール手段（要件 6.4）。
		charts.WithDataZoomOpts(
			opts.DataZoom{Type: "inside", Start: 0, End: 100},
			opts.DataZoom{Type: "slider", Start: 0, End: 100},
		),
	}

	// オーバーレイがあるときのみ凡例を出し、オーバーレイは selected:false で既定オフ（クラッタ回避）。
	if hasOverlay {
		legendData := []string{spec.Unit}
		selected := map[string]bool{}
		if hasBand {
			legendData = append(legendData, seriesNameTrendBand)
			selected[seriesNameTrendBand] = false
		}
		if hasCI {
			legendData = append(legendData, seriesNameTrendCI)
			selected[seriesNameTrendCI] = false
		}
		if hasClim {
			legendData = append(legendData, seriesNameClimatology)
			selected[seriesNameClimatology] = false
		}
		global = append(global, charts.WithLegendOpts(opts.Legend{
			Show:     opts.Bool(true),
			Data:     legendData,
			Selected: selected,
		}))
	}

	line.SetGlobalOptions(global...)
	line.SetXAxis(spec.Labels)

	// series[0]: ロールアップ平均（主役）。markPoint max/min・基準色。Sen 線/有意区間は後段で注入。
	line.AddSeries(spec.Unit, lineData(spec.RollupAvg),
		charts.WithLineStyleOpts(opts.LineStyle{Color: spec.Color}),
		charts.WithMarkPointNameTypeItemOpts(
			opts.MarkPointNameTypeItem{Type: "max"},
			opts.MarkPointNameTypeItem{Type: "min"},
		),
	)

	// min/max 帯: 帯下限（透明線・stack）→ 帯幅（半透明 area・stack）。帯幅のみ凡例トグル対象。
	if hasBand {
		lower, width := stackBand(spec.BandLower, spec.BandUpper)
		line.AddSeries(seriesNameTrendBandLower, lineData(lower),
			charts.WithLineStyleOpts(opts.LineStyle{Opacity: opts.Float(0)}),
			charts.WithLineChartOpts(opts.LineChart{Stack: trendBandStackGroup, ShowSymbol: opts.Bool(false)}),
		)
		line.AddSeries(seriesNameTrendBand, lineData(width),
			charts.WithLineStyleOpts(opts.LineStyle{Opacity: opts.Float(0)}),
			charts.WithAreaStyleOpts(opts.AreaStyle{Color: spec.Color, Opacity: opts.Float(0.10)}),
			charts.WithLineChartOpts(opts.LineChart{Stack: trendBandStackGroup, ShowSymbol: opts.Bool(false)}),
		)
	}

	// ブートストラップ CI 帯: CI 下限（透明線・stack）→ CI 幅（半透明 area・stack）。
	if hasCI {
		lower, width := stackBand(spec.CILower, spec.CIUpper)
		line.AddSeries(seriesNameTrendCILower, lineData(lower),
			charts.WithLineStyleOpts(opts.LineStyle{Opacity: opts.Float(0)}),
			charts.WithLineChartOpts(opts.LineChart{Stack: trendCIStackGroup, ShowSymbol: opts.Bool(false)}),
		)
		line.AddSeries(seriesNameTrendCI, lineData(width),
			charts.WithLineStyleOpts(opts.LineStyle{Opacity: opts.Float(0)}),
			charts.WithAreaStyleOpts(opts.AreaStyle{Color: spec.Color, Opacity: opts.Float(0.18)}),
			charts.WithLineChartOpts(opts.LineChart{Stack: trendCIStackGroup, ShowSymbol: opts.Bool(false)}),
		)
	}

	// 平年比: 独立線（破線・symbol 非表示）。年数不足時は出さない（要件 7.1〜7.3）。
	if hasClim {
		line.AddSeries(seriesNameClimatology, lineData(spec.Climatology),
			charts.WithLineStyleOpts(opts.LineStyle{Color: spec.Color, Type: "dashed", Width: 1}),
			charts.WithLineChartOpts(opts.LineChart{ShowSymbol: opts.Bool(false)}),
		)
	}

	line.Validate()

	base, err := json.Marshal(line.JSON())
	if err != nil {
		return "", fmt.Errorf("chart: トレンド option の JSON 化に失敗: %w", err)
	}
	var optionMap map[string]any
	if err := json.Unmarshal(base, &optionMap); err != nil {
		return "", fmt.Errorf("chart: トレンド option の再構築に失敗: %w", err)
	}
	if err := injectTrendMarks(optionMap, spec); err != nil {
		return "", err
	}

	// 再シリアライズ（SetEscapeHTML=true 既定）で < > & を \uXXXX 化し HTML 安全化する。
	out, err := json.Marshal(optionMap)
	if err != nil {
		return "", fmt.Errorf("chart: トレンド option の HTML 安全化に失敗: %w", err)
	}
	return string(out), nil
}

// injectTrendMarks は series[0] へ Sen トレンド線（markLine）と有意区間（markArea）を
// ECharts 準拠の小文字キーで注入する。
//   - markLine: Sen 線を2端点の coord で表す（[0, SenLine先頭]→[lastIdx, SenLine末尾]）。
//     カテゴリ x 軸ゆえ端点 x はインデックス（0 と len(RollupAvg)-1）。
//   - markArea: 有意区間（Run）ごとに xAxis 範囲ゾーン（薄紫）を出す。
//
// SenLine 空なら markLine を出さず、Significant 空なら markArea を出さない（縮退）。
func injectTrendMarks(optionMap map[string]any, spec TrendChartSpec) error {
	series, ok := optionMap["series"].([]any)
	if !ok || len(series) == 0 {
		return fmt.Errorf("chart: トレンド option に series[0] が無い")
	}
	head, ok := series[0].(map[string]any)
	if !ok {
		return fmt.Errorf("chart: トレンド option の series[0] が想定形でない")
	}

	// Sen トレンド線（2端点 coord 線）。x はカテゴリインデックス。
	if len(spec.SenLine) >= 2 {
		lastIdx := len(spec.RollupAvg) - 1
		if lastIdx < 1 {
			lastIdx = len(spec.SenLine) - 1
		}
		yStart := spec.SenLine[0]
		yEnd := spec.SenLine[len(spec.SenLine)-1]
		head["markLine"] = map[string]any{
			"silent": true,
			"symbol": []any{"none", "none"},
			"data": []any{
				[]any{
					map[string]any{"coord": []any{0, yStart}, "name": "Senトレンド"},
					map[string]any{"coord": []any{lastIdx, yEnd}},
				},
			},
		}
	}

	// 有意区間 markArea（xAxis 範囲ゾーン）。
	if len(spec.Significant) > 0 {
		zones := make([]any, 0, len(spec.Significant))
		for _, r := range spec.Significant {
			zones = append(zones, []any{
				map[string]any{"xAxis": r.StartIdx, "itemStyle": map[string]any{"color": trendSignificantBandColor}},
				map[string]any{"xAxis": r.EndIdx},
			})
		}
		head["markArea"] = map[string]any{
			"silent": true,
			"data":   zones,
		}
	}
	return nil
}

// DiurnalRangeChartOptionJSON は日較差ΔT 推移（日次 ΔT＝最高−最低 の時系列）の line option を返す。
// 主役1系列の素朴な折れ線に長期閲覧用 dataZoom を付ける（要件 2.4・長期の日次点を拡大可能に）。
// markPoint max/min で最大・最小の日較差を示す。HTML 安全化は ChartOptionJSON と同方針。
func DiurnalRangeChartOptionJSON(labels []string, deltaT []float64, color, unit string) (string, error) {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTooltipOpts(opts.Tooltip{
			Trigger:     "axis",
			AxisPointer: &opts.AxisPointer{Type: "cross"},
		}),
		charts.WithXAxisOpts(opts.XAxis{Type: "category"}),
		charts.WithYAxisOpts(opts.YAxis{Type: "value", Scale: opts.Bool(true)}),
		charts.WithDataZoomOpts(
			opts.DataZoom{Type: "inside", Start: 0, End: 100},
			opts.DataZoom{Type: "slider", Start: 0, End: 100},
		),
	)
	line.SetXAxis(labels)
	line.AddSeries(unit, lineData(deltaT),
		charts.WithLineStyleOpts(opts.LineStyle{Color: color}),
		charts.WithMarkPointNameTypeItemOpts(
			opts.MarkPointNameTypeItem{Type: "max"},
			opts.MarkPointNameTypeItem{Type: "min"},
		),
	)
	line.Validate()

	bs, err := json.Marshal(line.JSON())
	if err != nil {
		return "", fmt.Errorf("chart: 日較差 option の JSON 化に失敗: %w", err)
	}
	return string(bs), nil
}

// stackBand は下限/上限の2系列から「下限（透明ベース）」と「帯幅=上限-下限」を返す（積み上げ area 用）。
// 長さ不一致は短い方に合わせて防御的に扱う（純粋層と同方針）。
func stackBand(lower, upper []float64) (base, width []float64) {
	n := len(lower)
	if len(upper) < n {
		n = len(upper)
	}
	base = make([]float64, n)
	width = make([]float64, n)
	for i := 0; i < n; i++ {
		base[i] = lower[i]
		width[i] = upper[i] - lower[i]
	}
	return base, width
}
