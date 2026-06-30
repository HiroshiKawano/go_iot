// device_show_heatstress.go はデバイス詳細画面の高温ストレス（THI・熱帯夜・絶対湿度AH・日較差ΔT）
// パネルの組立を担う。既存の JST 日次集約（ListDailySensorAggregatesJST）と直近生行
// （ListRecentSensorReadings）から、読み取り時に THI 時間帯×日ヒートマップ・熱帯夜カレンダー・
// 夜温/ΔT 推移・AH 系列・熱帯夜年間日数トレンドを算出し、高温ストレスパネル View を組む
// （研究用・保存しない・新規クエリ/マイグレーションなし）。
//
// VPD/露点パネルと決定的に異なり period を引数に取らず、熱帯夜 calendar は年スケールゆえ
// 期間セレクタ非連動（Show ページからのみ呼ばれ、期間フラグメント Chart からは呼ばない・GDD と同型）。
// 純粋な算出は internal/chart へ委譲し、ここは時刻バケット（JST 暦日/夜間=日最低/hour-of-day/年）と
// 表示整形の handler 境界に集中する（device_show_gdd.go/device_show_vpd.go と同作法）。
package handler

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/chart"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/view/component"
)

const (
	// heatLineColor は高温ストレス各系列・visualMap 最濃側の基準色。mocks/html/style.css の
	// --color-heat（暖色＝暑い＝危険側）と対応し、温度橙/GDD赤(#e03131)と判別する。
	heatLineColor = "#d6336c"

	// tropicalNightThresholdC は熱帯夜の夜温（JST 日最低気温）閾値。作物非依存の既定定数（D2/D6/D8・要件 8.1）。
	// 作物別が必要と判明した将来フェーズで domain.Crop.HeatStressModel() 拡張点へ移す（DB 列は増やさない）。
	tropicalNightThresholdC = 25.0

	// heatMaxTrendYears は年間日数トレンド用の JST 日次集約の取得年数（最大蓄積年数の上限）。
	heatMaxTrendYears = 11
	// heatRawWindowDays は THI 時間帯×日ヒートマップ・AH line・現在カード用の生行取得日数。
	heatRawWindowDays = 14

	// 計測ゼロ時の導線注記（縮退・要件 9.2/9.4）。
	heatNoDataNote = "計測データがまだありません。温湿度の計測が蓄積すると高温ストレス（THI・熱帯夜・絶対湿度・日較差）を表示します。"
	// 検出力の留保注記（要件 6.2/6.3・多年でも単年でも掲示）。
	heatTrendNote = "熱帯夜年間日数の傾向（Sen 傾き）は参考値です。統計的に非有意であってもトレンドが無いことを意味しません（評価には複数年の蓄積が必要です）。"
)

// buildHeatStressPanel は JST 日次集約＋直近生行から高温ストレスパネル View を組む（要件 1〜9）。
// period を引数に取らない（直近1年・全蓄積年で固定・Show からのみ呼ばれる・9.3）。now は窓・年判定の基準。
// device は Show 経路で RequireDeviceOwner 検証済み（11.1/11.2 は既存認可に委譲）。
// 計測ゼロは error を返さず HasData=false＋Guidance へ縮退する。DB 想定外のみ error（→ 500）。
func (h *DeviceHandler) buildHeatStressPanel(ctx context.Context, device repository.Device, now time.Time) (component.HeatStressPanelView, error) {
	view := component.HeatStressPanelView{Color: heatLineColor, Card: emptyHeatStressCard()}

	// データアクセス（新規クエリなし）: 全蓄積年の JST 日次集約＋直近窓の生行（5.1）。
	dailyRows, err := h.Repo.ListDailySensorAggregatesJST(ctx, repository.ListDailySensorAggregatesJSTParams{
		DeviceID:   device.ID,
		RecordedAt: pgconv.Timestamptz(now.AddDate(-heatMaxTrendYears, 0, 0)),
	})
	if err != nil {
		return component.HeatStressPanelView{}, err
	}
	rawRows, err := h.Repo.ListRecentSensorReadings(ctx, repository.ListRecentSensorReadingsParams{
		DeviceID:   device.ID,
		RecordedAt: pgconv.Timestamptz(now.AddDate(0, 0, -heatRawWindowDays)),
	})
	if err != nil {
		return component.HeatStressPanelView{}, err
	}

	// 両方空 → 空パネル＋Guidance（error なし・レイアウト非破壊・5.1/9.2）。
	if len(dailyRows) == 0 && len(rawRows) == 0 {
		view.Guidance = heatNoDataNote
		return view, nil
	}
	view.HasData = true

	spec := chart.HeatStressChartSpec{Color: heatLineColor}
	heatDailySeries(&spec, &view, dailyRows, now) // 5.2 熱帯夜 calendar・夜温/ΔT・連続日数
	heatYearlyTrend(&spec, &view, dailyRows)      // 5.4 年間日数トレンド
	heatRawSeries(&spec, &view, rawRows)          // 5.3 THI 時間帯×日・AH・現在カード

	return heatBuildOptions(spec, view)
}

// heatBuildOptions は spec から各 option JSON を構築し View へ詰める（HTML 安全 JSON・空は "{}"）。
func heatBuildOptions(spec chart.HeatStressChartSpec, view component.HeatStressPanelView) (component.HeatStressPanelView, error) {
	var err error
	if view.THIHeatmapJSON, err = chart.THIHourDayHeatmapOptionJSON(spec); err != nil {
		return component.HeatStressPanelView{}, err
	}
	if view.CalendarJSON, err = chart.TropicalNightCalendarOptionJSON(spec); err != nil {
		return component.HeatStressPanelView{}, err
	}
	if view.NightDeltaJSON, err = chart.NightTempDeltaLineOptionJSON(spec); err != nil {
		return component.HeatStressPanelView{}, err
	}
	if view.AHJSON, err = chart.AHLineOptionJSON(spec); err != nil {
		return component.HeatStressPanelView{}, err
	}
	if view.TrendJSON, err = chart.TropicalNightTrendOptionJSON(spec); err != nil {
		return component.HeatStressPanelView{}, err
	}
	return view, nil
}

// heatDailySeries は直近1年の JST 日次から熱帯夜 calendar・夜温/ΔT 系列・連続日数を組む（5.2）。
// 夜温＝JST 暦日の日最低気温（D2）。欠測日（行なし）は暦日連続列で NaN とし熱帯夜0と誤計上せず run を切る（2.5）。
func heatDailySeries(spec *chart.HeatStressChartSpec, view *component.HeatStressPanelView, dailyRows []repository.ListDailySensorAggregatesJSTRow, now time.Time) {
	end := dateOnlyJST(now)
	start := end.AddDate(-1, 0, 0) // 直近1年（D8・period 非連動）

	type minMax struct{ min, max float64 }
	byDay := make(map[time.Time]minMax)
	for _, r := range dailyRows {
		d := dateOnlyJST(r.ReadingDate.Time)
		if d.Before(start) || d.After(end) {
			continue
		}
		byDay[d] = minMax{min: aggregateToFloat(r.MinTemperature), max: aggregateToFloat(r.MaxTemperature)}
	}

	var calCells []chart.DateValue
	var dayLabels []string
	var nightTemps, deltaT, nightByCalDay []float64
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		mm, ok := byDay[d]
		if !ok {
			nightByCalDay = append(nightByCalDay, math.NaN()) // 欠測=判定不能（run を切る）
			continue
		}
		calCells = append(calCells, chart.DateValue{Date: d.Format("2006-01-02"), Value: mm.min})
		dayLabels = append(dayLabels, d.Format("01/02"))
		nightTemps = append(nightTemps, mm.min)
		deltaT = append(deltaT, round2(mm.max-mm.min))
		nightByCalDay = append(nightByCalDay, mm.min)
	}

	if len(calCells) == 0 {
		view.TropicalLongest = statEmptyMark
		view.TropicalCurrent = statEmptyMark
		return
	}
	spec.CalendarRange = []string{start.Format("2006-01-02"), end.Format("2006-01-02")}
	spec.CalendarCells = calCells
	spec.NightMin, spec.NightMax = floorCeilRange(nightTemps)
	spec.DayLabels = dayLabels
	spec.NightTemps = nightTemps
	spec.DeltaT = deltaT

	longest, current := chart.RunStats(chart.TropicalNightMask(nightByCalDay, tropicalNightThresholdC))
	view.TropicalLongest = formatDays(longest)
	view.TropicalCurrent = formatDays(current)
	view.Card.LatestNightTemp = formatHeatTemp(nightTemps[len(nightTemps)-1]) // 直近夜温=最新present日の日最低
}

// heatYearlyTrend は全蓄積年の JST 日次から年ごとの熱帯夜日数を計数し、2年以上のとき Sen 傾きを
// 既存 trend.go の再利用で算出する（新規統計を実装しない・要件 6）。数年では Sen 傾き＋符号に留め
// 有意性は断定しない。年系列が1点以下は HasTrend=false（6.4）。
func heatYearlyTrend(spec *chart.HeatStressChartSpec, view *component.HeatStressPanelView, dailyRows []repository.ListDailySensorAggregatesJSTRow) {
	view.TrendNote = heatTrendNote

	counts := make(map[int]int)
	for _, r := range dailyRows {
		y := dateOnlyJST(r.ReadingDate.Time).Year()
		if _, seen := counts[y]; !seen {
			counts[y] = 0 // 計測のある年は0でも存在させる（熱帯夜0年のタイ対応・6.4）
		}
		if aggregateToFloat(r.MinTemperature) >= tropicalNightThresholdC {
			counts[y]++
		}
	}
	years := make([]int, 0, len(counts))
	for y := range counts {
		years = append(years, y)
	}
	sort.Ints(years)
	if len(years) < 2 {
		view.HasTrend = false // 複数年の蓄積が必要（6.4）。SenSlopeSign は emptyHeatStressCard の "—" のまま
		return
	}

	yearLabels := make([]string, len(years))
	yearlyCounts := make([]float64, len(years))
	for i, y := range years {
		yearLabels[i] = strconv.Itoa(y)
		yearlyCounts[i] = float64(counts[y])
	}
	sen := chart.SensSlope(yearlyCounts)

	view.HasTrend = true
	spec.HasTrend = true
	spec.YearLabels = yearLabels
	spec.YearlyCounts = yearlyCounts
	spec.SenLine = chartRound2(senLineValues(yearlyCounts, sen))
	view.Card.SenSlopeSign = formatSenSlopeSign(sen.Slope)
}

// heatRawSeries は直近生行から THI 時間帯×日ヒートマップ・AH line・現在カード（現在THI/現在AH）を組む（5.3）。
func heatRawSeries(spec *chart.HeatStressChartSpec, view *component.HeatStressPanelView, rawRows []repository.SensorReading) {
	if len(rawRows) == 0 {
		return // 現在THI/AH は emptyHeatStressCard の "—" のまま
	}
	temps := make([]float64, len(rawRows))
	hums := make([]float64, len(rawRows))
	ahLabels := make([]string, len(rawRows))
	for i, r := range rawRows {
		temps[i] = pgconv.NumericToFloat(r.Temperature)
		hums[i] = pgconv.NumericToFloat(r.Humidity)
		ahLabels[i] = pgconv.TimestamptzToTime(r.RecordedAt).In(jst).Format("01/02 15:04")
	}
	spec.AHLabels = ahLabels
	spec.AH = chartRound2(chart.AbsoluteHumiditySeries(temps, hums))

	cells, dayLabels := thiHourDayCells(rawRows, temps, hums)
	spec.THIHourDay = cells
	spec.THIDayLabels = dayLabels
	spec.THIMin, spec.THIMax = thiCellRange(cells)

	last := len(rawRows) - 1
	view.Card.CurrentTHI = formatTHI(chart.THI(temps[last], hums[last]))
	view.Card.CurrentAH = formatAH(chart.AbsoluteHumidity(temps[last], hums[last]))
}

// thiHourDayCells は生行を JST の（暦日×時間帯 0-23）でバケットし平均 THI のセル列と日ラベルを返す。
// セルは（日 index, 時刻, 平均THI）。日昇順・時刻昇順で決定的に並べる（map 由来の非決定を排除）。
func thiHourDayCells(rawRows []repository.SensorReading, temps, hums []float64) ([]chart.HeatCell, []string) {
	type dh struct {
		day  time.Time
		hour int
	}
	buckets := make(map[dh][]float64)
	daySet := make(map[time.Time]bool)
	for i, r := range rawRows {
		t := pgconv.TimestamptzToTime(r.RecordedAt).In(jst)
		day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, jst)
		key := dh{day: day, hour: t.Hour()}
		buckets[key] = append(buckets[key], chart.THI(temps[i], hums[i]))
		daySet[day] = true
	}
	days := make([]time.Time, 0, len(daySet))
	for d := range daySet {
		days = append(days, d)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })
	dayIndex := make(map[time.Time]int, len(days))
	dayLabels := make([]string, len(days))
	for i, d := range days {
		dayIndex[d] = i
		dayLabels[i] = d.Format("01/02")
	}

	cells := make([]chart.HeatCell, 0, len(buckets))
	for k, vals := range buckets {
		cells = append(cells, chart.HeatCell{Hour: k.hour, Day: dayIndex[k.day], Value: round2(chart.Mean(vals))})
	}
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].Day != cells[j].Day {
			return cells[i].Day < cells[j].Day
		}
		return cells[i].Hour < cells[j].Hour
	})
	return cells, dayLabels
}

// senLineValues は Sen 直線 y = Intercept + Slope·i（i=0..n-1）の値列を返す（markLine 両端用）。
func senLineValues(counts []float64, sen chart.SenResult) []float64 {
	out := make([]float64, len(counts))
	for i := range counts {
		out[i] = sen.Intercept + sen.Slope*float64(i)
	}
	return out
}

// emptyHeatStressCard はデータ未到着時の高温ストレス数値カード（全項目 "—"）を返す。
func emptyHeatStressCard() component.HeatStressCardView {
	return component.HeatStressCardView{
		CurrentTHI: statEmptyMark, CurrentAH: statEmptyMark,
		LatestNightTemp: statEmptyMark, SenSlopeSign: statEmptyMark,
	}
}

// floorCeilRange は visualMap レンジ用に [floor(min), ceil(max)] を返す。空入力は (0,0)。
func floorCeilRange(vals []float64) (float64, float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	mn, mx := chart.MinMax(vals)
	return math.Floor(mn), math.Ceil(mx)
}

// thiCellRange は THI ヒートマップセル値の visualMap レンジ [floor(min), ceil(max)] を返す。
func thiCellRange(cells []chart.HeatCell) (float64, float64) {
	if len(cells) == 0 {
		return 0, 0
	}
	mn, mx := cells[0].Value, cells[0].Value
	for _, c := range cells[1:] {
		if c.Value < mn {
			mn = c.Value
		}
		if c.Value > mx {
			mx = c.Value
		}
	}
	return math.Floor(mn), math.Ceil(mx)
}

// formatSenSlopeSign は Sen 傾き＋符号を参考値として整形する（要件 6.1/6.2）。
// 増加/減少/横ばいを符号付きで示し、断定を避けて「参考」と明記する。
func formatSenSlopeSign(slope float64) string {
	abs := math.Abs(slope)
	switch {
	case abs < 0.05:
		return "±0.0 日/年（横ばい・参考）"
	case slope > 0:
		return fmt.Sprintf("+%.1f 日/年（増加傾向・参考）", abs)
	default:
		return fmt.Sprintf("-%.1f 日/年（減少傾向・参考）", abs)
	}
}

// formatDays は連続日数を "N 日" に整形する。
func formatDays(n int) string {
	return strconv.Itoa(n) + " 日"
}

// formatTHI は THI を小数1桁に整形する（無次元・単位なし）。
func formatTHI(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// formatAH は絶対湿度を "X.X g/m³" に整形する。
func formatAH(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64) + " g/m³"
}

// formatHeatTemp は気温（夜温）を "X.X℃" に整形する。
func formatHeatTemp(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64) + "℃"
}

// round2 は計算系列の表示用2桁丸め（スカラ）。検定は生値・丸めは表示専用（feedback_echarts_round_computed_series）。
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
