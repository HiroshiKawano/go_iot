// device_show_gdd.go はデバイス詳細画面の GDD（積算温度・収穫予測）パネルの組立を担う。
// 既存温湿度データ（日次最高/最低気温）と作物別 GDD モデル（domain.Crop.GDDModel）から
// 日次 GDD・累積・残り積算・到達日外挿・生育ステージを読み取り時に算出し、GDD パネル View を組む
// （研究用・保存しない）。VPD/露点パネルと決定的に異なり period を引数に取らず、定植日→現在の全期間を走る
// （Show ページからのみ呼ばれ、期間フラグメント Chart/buildChartArea からは呼ばない・要件 6.1/6.2）。
// 純粋な算出は internal/chart へ委譲し、ここは時刻バケット（定植日 JST 00:00 起点化・経過日数換算）と
// 表示整形の handler 境界に集中する（device_show_vpd.go/device_show_dewpoint.go と同作法）。
package handler

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/chart"
	"github.com/HiroshiKawano/go_iot/internal/domain"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/view/component"
)

const (
	// gddLineColor は累積 GDD 曲線・予測到達マークの基準色。mocks/html/style.css の --color-gdd（暖色）と対応し、
	// 温度橙 (#e8590c)/湿度青 (#1971c2)/VPD 緑 (#0ca678)/露点青 (#4263eb) と区別する（積算温度＝生育が早い側）。
	gddLineColor = "#e03131"

	// 予測可否で出し分ける注記（要件 3.4／4.3）。
	gddForecastNote   = "予測収穫日は線形外挿による目安です（季節変動は織り込みません）。"
	gddNoForecastNote = "予測収穫日は算出できません（生育が進んでいない・データ不足・または既に到達済み）。"

	// 前提欠落・未開始・データ未到着の導線注記（縮退・要件 6.3/6.4/2.6）。
	gddGuidanceNote   = "作物と定植日を設定すると GDD（積算温度・収穫予測）を表示します。"
	gddNotStartedNote = "定植日が未来日です。定植日以降に GDD の積算を開始します。"
	gddNoDataNote     = "定植日以降の計測データがまだありません。"
)

// buildGDDPanel は定植日→現在の日次気温＋作物 GDDModel から GDD パネル View を組む（要件 1/3/4/6）。
// period を引数に取らない（定植日→現在で固定・Show からのみ呼ばれる・6.1/6.2）。now は未来日判定・本日基準。
// 前提欠落（具体 GDD モデルなし／定植日 NULL）・未来日・空データ・予測不能はいずれも error を返さず
// 縮退 View（Guidance か "—" 注記）にする。DB 想定外のみ error（→ 500）。
func (h *DeviceHandler) buildGDDPanel(ctx context.Context, device repository.Device, now time.Time) (component.GDDPanelView, error) {
	crop := deviceCrop(device)
	model := crop.GDDModel()
	view := component.GDDPanelView{
		Color:     gddLineColor,
		CropLabel: gddCropLabel(crop),
	}

	// 前提未設定: 作物が未設定（NULL/不正）または定植日 NULL → 汎用の設定導線注記（要件 6.3）。
	// 「作物と定植日を設定すると…」＝まだ設定していない人向けの案内。
	if !crop.Valid() || !device.PlantingDate.Valid {
		view.Guidance = gddGuidanceNote
		return view, nil
	}

	// 前提は設定済みだが、その作物に GDD 具体モデル（非空 Stages＝収穫目標あり）が無い → 未対応作物の専用注記。
	// ここで汎用の「設定してください」を出すと、設定済みユーザーが『設定したのに出ない』と誤解する（本修正の要点）。
	// 対応作物（米・ゴーヤ・インゲン・ウリ・いも・葉野菜 等）を明示し、未対応であることを正確に伝える（要件 5.2/5.4）。
	if len(model.Stages) == 0 {
		view.Guidance = gddUnsupportedCropNote(crop)
		return view, nil
	}

	plantDay := dateOnlyJST(device.PlantingDate.Time)
	today := dateOnlyJST(now)
	// 未来定植日: 経過日数を負にせず未開始を提示する（要件 2.6）。
	if plantDay.After(today) {
		view.Guidance = gddNotStartedNote
		return view, nil
	}

	// 定植日 JST 00:00 起点で日次集計を再利用取得（GDD 専用の新規 SELECT は起こさない）。
	since := pgconv.Timestamptz(plantDay)
	rows, err := h.Repo.ListDailySensorAggregates(ctx, repository.ListDailySensorAggregatesParams{
		DeviceID:   device.ID,
		RecordedAt: since,
	})
	if err != nil {
		return component.GDDPanelView{}, err
	}
	if len(rows) == 0 {
		view.Guidance = gddNoDataNote
		return view, nil
	}

	// present 日ごとに日次最高/最低気温と経過日数（ReadingDate − 定植日）を算出（時刻換算は handler 境界）。
	tMax := make([]float64, len(rows))
	tMin := make([]float64, len(rows))
	elapsed := make([]float64, len(rows))
	for i, r := range rows {
		tMax[i] = aggregateToFloat(r.MaxTemperature)
		tMin[i] = aggregateToFloat(r.MinTemperature)
		elapsed[i] = daysBetween(plantDay, dateOnlyJST(r.ReadingDate.Time))
	}

	// 純粋層で日次 GDD→累積→残り積算→到達日外挿→生育ステージ（[]float64/スカラのみ渡す）。
	daily := chart.DailyGDD(tMax, tMin, model.Tbase)
	cum := chart.CumulativeGDD(daily)
	stageGDDs := stageThresholds(model.Stages)
	target := stageGDDs[len(stageGDDs)-1] // 最終段=収穫目標
	remaining := chart.RemainingGDD(cum, target)
	fDay, hasForecast := chart.ForecastDaysToTarget(elapsed, cum, target)
	stageIdx := chart.GrowthStageIndex(cum[len(cum)-1], stageGDDs)

	optionJSON, err := chart.GDDChartOptionJSON(chart.GDDChartSpec{
		ElapsedDays: elapsed,
		Cumulative:  cum,
		Color:       gddLineColor,
		TargetGDD:   target,
		ForecastDay: fDay,
		HasForecast: hasForecast,
	})
	if err != nil {
		return component.GDDPanelView{}, err
	}

	view.OptionJSON = optionJSON
	view.Note = gddForecastNote
	if !hasForecast {
		view.Note = gddNoForecastNote // 予測不能の理由注記（要件 4.3）
	}
	view.Card = component.GDDCardView{
		Cumulative:   formatGDD(cum[len(cum)-1]),
		Remaining:    formatGDD(remaining),
		ForecastDate: gddForecastDate(plantDay, fDay, hasForecast),
		Stage:        stageName(model.Stages, stageIdx),
		ElapsedDays:  formatElapsedDays(elapsed[len(elapsed)-1]),
	}
	view.Stages = gddStageRows(model.Stages, stageIdx)
	return view, nil
}

// gddCropLabel は作物の日本語ラベルを返す（未設定/不正は "既定"）。VPD パネルの CropLabel と同方針。
func gddCropLabel(c domain.Crop) string {
	if c.Valid() {
		return c.Label()
	}
	return "既定"
}

// gddUnsupportedCropNote は「作物・定植日は設定済みだが、その作物の GDD 具体モデルが未対応」のときの注記。
// 汎用の「設定してください」ではなく、当該作物が未対応であることと現在の対応作物を明示して、
// 設定済みユーザーの誤解（『設定したのに出ない』）を避ける（要件 5.2/5.4・6.3）。
func gddUnsupportedCropNote(c domain.Crop) string {
	return fmt.Sprintf(
		"「%s」の生育ステージ（GDD）モデルはまだ用意されていません。現在 GDD（積算温度・収穫予測）に対応している作物は %s です。",
		c.Label(), strings.Join(gddSupportedCropLabels(), "・"),
	)
}

// gddSupportedCropLabels は GDD 具体モデル（非空 Stages）を持つ作物の日本語ラベルを表示順で返す。
// domain.GDDModel を単一の真実源にし、対応作物を増やせば注記も自動追随する（文言のハードコード回避）。
func gddSupportedCropLabels() []string {
	var out []string
	for _, c := range domain.AllCrops() {
		if c.HasGDDModel() {
			out = append(out, c.Label())
		}
	}
	return out
}

// dateOnlyJST は時点 t を JST 暦日の 0:00（JST）へ切り捨てて返す（定植日起点・経過日数換算の基準）。
// pgtype.Date は時刻成分を持たない UTC 0:00 で渡ってくるが、暦日 (y/m/d) のみ使うため JST 0:00 に再構成する。
func dateOnlyJST(t time.Time) time.Time {
	lt := t.In(jst)
	return time.Date(lt.Year(), lt.Month(), lt.Day(), 0, 0, 0, 0, jst)
}

// daysBetween は同一 TZ の暦日 a→b の経過日数を返す（b−a・整数日）。
func daysBetween(a, b time.Time) float64 {
	return math.Round(b.Sub(a).Hours() / 24)
}

// stageThresholds は生育ステージ列を昇順 GDD しきい値の []float64 に写す（純粋層へ渡す形）。
func stageThresholds(stages []domain.GrowthStage) []float64 {
	out := make([]float64, len(stages))
	for i, s := range stages {
		out[i] = s.GDD
	}
	return out
}

// formatGDD は積算温度を整数 ℃·日 "N ℃·日" に整形する（GDD は粗い目安ゆえ小数不要）。
func formatGDD(v float64) string {
	return strconv.FormatFloat(math.Round(v), 'f', 0, 64) + " ℃·日"
}

// formatElapsedDays は経過日数を "N 日" に整形する。
func formatElapsedDays(d float64) string {
	return strconv.FormatFloat(math.Round(d), 'f', 0, 64) + " 日"
}

// gddForecastDate は予測収穫日を "YYYY-MM-DD" で返す（予測不能は "—"・捏造回避・要件 4.3）。
func gddForecastDate(plantDay time.Time, fDay float64, hasForecast bool) string {
	if !hasForecast {
		return statEmptyMark
	}
	return plantDay.AddDate(0, 0, int(math.Round(fDay))).Format("2006-01-02")
}

// stageName は現在ステージ index の段名を返す（未到達 -1 は "—"）。
func stageName(stages []domain.GrowthStage, idx int) string {
	if idx < 0 || idx >= len(stages) {
		return statEmptyMark
	}
	return stages[idx].Name
}

// gddStageRows は生育ステージ⇔GDD 対応表の行を組む（現在段に Current マーク）。
func gddStageRows(stages []domain.GrowthStage, currentIdx int) []component.GrowthStageRow {
	out := make([]component.GrowthStageRow, len(stages))
	for i, s := range stages {
		out[i] = component.GrowthStageRow{
			Name:    s.Name,
			GDD:     formatGDD(s.GDD),
			Current: i == currentIdx,
		}
	}
	return out
}
