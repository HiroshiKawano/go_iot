// device_show_dewpoint.go はデバイス詳細画面の露点・病害リスク蓄積解析パネルの組立を担う。
// 温湿度データから読み取り時に露点 Td・スプレッド・結露帯・葉面湿潤時間・高湿度イベント・病害スコア下地を
// 算出し、作物別の病害モデルしきい値 (domain.Crop.DiseaseModel) を適用した露点パネル View を組む
// (研究用・保存しない)。純粋な算出は internal/chart へ委譲し、ここは時刻バケット (JST 暦日・イベント時刻化) と
// 表示整形の handler 境界に集中する (device_show_vpd.go と同作法)。
package handler

import (
	"math"
	"strconv"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/chart"
	"github.com/HiroshiKawano/go_iot/internal/domain"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/view/component"
)

const (
	// dewpointLineColor は露点 Td 線の基準色。mocks/html/style.css の --color-dewpoint (寒色＝湿り側) と対応し、
	// 温度橙 (#e8590c)/湿度青 (#1971c2)/VPD 緑 (#0ca678) と区別する。結露帯 markArea も同じ寒色系。
	dewpointLineColor = "#4263eb"

	// dewpointApproxNote は葉面温度を気温で近似した代理判定である旨の注記 (要件 2.3)。
	// 葉面温度センサ不在のため気温で近似し、結露帯は露点スプレッド ≦ しきい値の代理で判定する。
	dewpointApproxNote = "葉面温度センサ不在のため気温で近似。結露帯は露点スプレッド（気温−露点）≦ しきい値の代理判定で、厳密な「葉面温度 ≦ 露点」判定ではない。"

	// 直近の結露帯有無のラベル (湿り側＝結露中・物理規約)。
	dewCondensationPresent = "結露中"
	dewCondensationAbsent  = "なし"
)

// buildDewpointPanel は整形済みの温湿度列＋生行＋作物から露点パネル View を組む (要件 2/3/4/5)。
// labels/temps/hums は buildChartArea が整形済み (温湿度グラフと共通の時刻ラベル列)。
// rows は時刻換算 (JST 暦日積算・イベント時刻化) 用の生行・crop は病害しきい値解決用・period は点数換算用・now は本日判定用。
// crop.DiseaseModel() でしきい値 (未設定→既定) を解決し、露点系列→スプレッド→結露帯→option を構築する。
// 時刻を要する日次/イベント時刻換算は handler 境界で行い純粋層に時刻を持ち込まない (dailyStatRows と同作法)。
// 計測 0 件 (空系列) のときは Card "—"・Daily/Events 空・OptionJSON 空のパネルを返す (呼出側は HasData で非表示)。
func buildDewpointPanel(labels []string, temps, hums []float64, rows []repository.SensorReading, crop domain.Crop, period string, now time.Time) (component.DewpointPanelView, error) {
	dm := crop.DiseaseModel()
	view := component.DewpointPanelView{
		DewColor: dewpointLineColor,
		Note:     dewpointApproxNote,
	}

	dews := chart.DewPointSeries(temps, hums)
	if len(dews) == 0 {
		view.Card = emptyDewpointCard()
		return view, nil
	}

	spread := chart.DewPointSpread(temps, dews)
	cond := chart.CondensationRuns(spread, dm.CondensationMaxSpread)
	wet := chart.WetnessMask(hums, dm.WetnessRHThreshold)

	optionJSON, err := chart.DewpointChartOptionJSON(chart.DewpointChartSpec{
		Labels:       labels,
		DewColor:     dewpointLineColor,
		Dewpoint:     dews,
		Temperature:  temps,
		Condensation: cond,
	})
	if err != nil {
		return component.DewpointPanelView{}, err
	}
	view.OptionJSON = optionJSON

	// 葉面湿潤時間＋病害スコア下地の JST 暦日積算 (時刻換算は handler 境界)。
	view.Daily = dewpointDailyRows(rows, wet, temps, dm)

	// 高湿度継続イベント一覧 (各 Run を recorded_at で時刻化・該当なしは空)。
	view.Events = highHumidityEventRows(rows, hums, spread, dm)

	view.Card = component.DewpointCardView{
		CurrentDewpoint:    formatDewpointStat(dews[len(dews)-1]),     // 現在=最新点 Td
		CurrentSpread:      formatDewpointStat(spread[len(spread)-1]), // 現在スプレッド T−Td
		TodayWetHours:      todayWetHours(view.Daily, now),            // 本日の葉面湿潤時間
		RecentCondensation: recentCondensationLabel(cond, len(dews)),  // 直近の結露帯有無 (湿り側)
	}
	return view, nil
}

// maxWetGap は葉面湿潤時間積算で連続する湿潤点間隔の上限。欠測/停波で間隔が異常に長いとき
// この上限でキャップし、葉面湿潤時間の水増しを防ぐ (公称間隔 約5分 の数倍目安=1時間・design 確定)。
const maxWetGap = time.Hour

// dewpointDailyRows は生行＋葉面湿潤マスク＋気温から、JST 暦日ごとの葉面湿潤時間と病害スコア下地を
// 日付昇順で返す (要件 3.1/5.1)。葉面湿潤時間は「連続する湿潤点 (wet[i] かつ wet[i+1]) の recorded_at 間隔」を
// 暦日へ積算し、異常に長い間隔は maxWetGap でキャップする (欠測の水増し防止)。間隔は実 recorded_at 差ゆえ
// 時刻換算は handler 境界で行う (純粋層に time を持ち込まない)。病害スコアは各暦日の点で DiseaseScore を算出する。
// データのある最古日〜最新日を1日ずつ進め、計測の無い日は "—" 行で補完する (dailyStatRows と同作法)。
func dewpointDailyRows(rows []repository.SensorReading, wet []bool, temps []float64, dm domain.DiseaseModel) []component.DewpointDailyRow {
	n := len(rows)
	if len(wet) < n {
		n = len(wet)
	}
	if len(temps) < n {
		n = len(temps)
	}
	if n == 0 {
		return nil
	}

	// JST 暦日キーで気温・湿潤フラグをグルーピングし、最古日/最新日も控える。
	wetSeconds := make(map[time.Time]float64)
	dayTemps := make(map[time.Time][]float64)
	dayWet := make(map[time.Time][]bool)
	var minDay, maxDay time.Time
	for i := 0; i < n; i++ {
		day := jstDay(rows[i].RecordedAt)
		dayTemps[day] = append(dayTemps[day], temps[i])
		dayWet[day] = append(dayWet[day], wet[i])
		if i == 0 || day.Before(minDay) {
			minDay = day
		}
		if i == 0 || day.After(maxDay) {
			maxDay = day
		}
	}

	// 連続する湿潤点間の間隔を葉面湿潤時間として暦日 (区間始点の暦日) へ積算する (maxWetGap でキャップ)。
	for i := 0; i+1 < n; i++ {
		if !wet[i] || !wet[i+1] {
			continue
		}
		ti := pgconv.TimestamptzToTime(rows[i].RecordedAt)
		tj := pgconv.TimestamptzToTime(rows[i+1].RecordedAt)
		dt := tj.Sub(ti)
		if dt < 0 {
			dt = 0 // 負の時間を生じない (防御的・行は通常昇順)
		}
		if dt > maxWetGap {
			dt = maxWetGap // 欠測の水増しを防ぐ
		}
		wetSeconds[jstDay(rows[i].RecordedAt)] += dt.Seconds()
	}

	var out []component.DewpointDailyRow
	for d := minDay; !d.After(maxDay); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		ts, ok := dayTemps[d]
		if !ok {
			// 計測の無い日は葉面湿潤・病害スコアとも未定義 "—" (体裁を保つ)。
			out = append(out, component.DewpointDailyRow{Date: dateStr, WetHours: statEmptyMark, DiseaseScore: statEmptyMark})
			continue
		}
		score := chart.DiseaseScore(ts, dayWet[d], dm.DiseaseTempLow, dm.DiseaseTempHigh)
		out = append(out, component.DewpointDailyRow{
			Date:         dateStr,
			WetHours:     formatWetHours(wetSeconds[d]),
			DiseaseScore: formatPercent(score),
		})
	}
	return out
}

// samplingPointsPerHour は公称サンプリング点数/時 (約5分間隔送信=12点/時)。高湿度イベントの
// 最小継続 [時間] を連続点数 minRun へ換算するのに使う。サンプリング間隔は period に依存しない (常に約5分)。
const samplingPointsPerHour = 12

// minRunFromHours は高湿度イベントの最小継続 [時間] を連続点数 minRun へ換算する (公称12点/時)。
// 0 以下や微小値は 1 にクランプし、HighHumidityRuns の事前条件 (minRun ≥ 1) を満たす。
func minRunFromHours(hours float64) int {
	n := int(math.Round(hours * samplingPointsPerHour))
	if n < 1 {
		n = 1
	}
	return n
}

// highHumidityEventRows は HighHumidityRuns の各区間を recorded_at で時刻化したイベント行を時刻昇順で返す
// (要件 4.1/4.2)。RH ≥ しきい値が最小継続 (minRunFromHours) 以上続いた区間のみ抽出し、単発は除外する。
// 各イベントは開始/終了/継続時間/区間内最小スプレッドを保持する。該当区間が無ければ空 (要件 4.4)。
func highHumidityEventRows(rows []repository.SensorReading, hums, spread []float64, dm domain.DiseaseModel) []component.HighHumidityEventRow {
	minRun := minRunFromHours(dm.HighHumidityMinHours)
	events := chart.HighHumidityRuns(hums, dm.WetnessRHThreshold, minRun)
	var out []component.HighHumidityEventRow
	for _, e := range events {
		if e.StartIdx >= len(rows) || e.EndIdx >= len(rows) {
			continue // 防御的 (events は hums と同じ index 空間ゆえ通常到達しない)
		}
		startT := pgconv.TimestamptzToTime(rows[e.StartIdx].RecordedAt).In(jst)
		endT := pgconv.TimestamptzToTime(rows[e.EndIdx].RecordedAt).In(jst)
		out = append(out, component.HighHumidityEventRow{
			Start:     startT.Format("01/02 15:04"),
			End:       endT.Format("01/02 15:04"),
			Duration:  formatWetHours(endT.Sub(startT).Seconds()),
			MinSpread: formatDewpointStat(minSpreadInRun(spread, e)),
		})
	}
	return out
}

// minSpreadInRun は区間 [StartIdx, EndIdx] 内の最小スプレッド (最も結露しやすい=湿り側の代表値) を返す。
func minSpreadInRun(spread []float64, r chart.Run) float64 {
	if r.StartIdx >= len(spread) {
		return 0
	}
	min := spread[r.StartIdx]
	for i := r.StartIdx + 1; i <= r.EndIdx && i < len(spread); i++ {
		if spread[i] < min {
			min = spread[i]
		}
	}
	return min
}

// todayWetHours は本日 (now の JST 暦日) の葉面湿潤時間をカード用に返す。本日のデータが無ければ "—"。
func todayWetHours(daily []component.DewpointDailyRow, now time.Time) string {
	today := jstDayOf(now).Format("2006-01-02")
	for _, r := range daily {
		if r.Date == today {
			return r.WetHours
		}
	}
	return statEmptyMark
}

// jstDayOf は時刻 t を JST に変換し、その暦日の 0:00 JST に切り捨てて返す (jstDay の time.Time 版)。
func jstDayOf(t time.Time) time.Time {
	lt := t.In(jst)
	return time.Date(lt.Year(), lt.Month(), lt.Day(), 0, 0, 0, 0, jst)
}

// formatWetHours は葉面湿潤の積算秒を小数1桁の時間 "N.N 時間" に整形する。
func formatWetHours(seconds float64) string {
	return strconv.FormatFloat(seconds/3600, 'f', 1, 64) + " 時間"
}

// recentCondensationLabel は最新点が結露帯 (CondensationRuns のいずれかの区間) に含まれるかを判定し、
// 含まれれば「結露中」(湿り側)、含まれなければ「なし」を返す (要件 2.2・物理規約の向き)。
func recentCondensationLabel(runs []chart.Run, n int) string {
	last := n - 1
	for _, r := range runs {
		if last >= r.StartIdx && last <= r.EndIdx {
			return dewCondensationPresent
		}
	}
	return dewCondensationAbsent
}

// formatDewpointStat は露点・スプレッドを小数1桁＋単位 "X.X℃" に整形する (露点カード・イベント用)。
func formatDewpointStat(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64) + "℃"
}

// emptyDewpointCard はデータ未到着時の露点数値カード (全項目 "—") を返す。
func emptyDewpointCard() component.DewpointCardView {
	return component.DewpointCardView{
		CurrentDewpoint: statEmptyMark, CurrentSpread: statEmptyMark,
		TodayWetHours: statEmptyMark, RecentCondensation: statEmptyMark,
	}
}
