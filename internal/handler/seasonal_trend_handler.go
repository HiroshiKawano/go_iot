// seasonal_trend_handler.go は統計分析ページ（長期トレンド・季節サマリ／GET /analysis/trend）の
// HTTP 境界を担う。認可は internal/authz（閲覧系＝非所有/不在は 404・列挙防止）へ、ロールアップ集約は
// seasonal_trend_rollup.go へ、検定計算・描画 option は internal/chart の純粋層へ委譲し、ここは
// リクエスト解釈・JST 取得境界・一次/厳密判定の組立・検出力留保の判断・HX 分岐・DTO 詰めに集中する。
// 永続化は repository.Querier を満たす最小 interface SeasonalTrendRepo 経由で受ける（service 層なし）。
package handler

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/authz"
	"github.com/HiroshiKawano/go_iot/internal/chart"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/view"
	"github.com/HiroshiKawano/go_iot/internal/view/component"
	"github.com/HiroshiKawano/go_iot/internal/view/layout"
	"github.com/HiroshiKawano/go_iot/internal/view/page"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
)

const seasonalTrendTitle = "統計分析（長期トレンド・季節サマリ） - 農業IoTシステム"

const (
	// trendChartColor は主役線・Sen 線・CI 帯の基準色（mocks/html/style.css の --color-trend・紫）。
	trendChartColor = "#7048e8"

	// 有意水準・検出力留保しきい値（design D6）。N_eff<10 または span<3年 は検定を断定しない。
	trendAlpha     = 0.05
	trendNEffMin   = 10
	trendSpanYears = 3

	// ブロックブートストラップ（seed 固定で決定的・R5.5）。
	trendBootstrapB    = 1000
	trendBootstrapSeed = 42

	// JST 日次集計の取得下限（広めに取り全履歴を拾う。集計後 1 行/日ゆえ行数は実用域）。
	trendLookbackYears = 10

	// 判定の区分ラベル（一次/厳密の段階表示・要件 6.3）。
	trendStagePrimary = "一次判定（多重比較未補正）"
	trendStageStrict  = "補正済み（Hamed-Rao）"

	// 縮退・留保の案内文。
	trendMsgSelectDevice = "対象のデバイスを選択してください。選択した対象の長期トレンド・季節サマリを表示します。"
	trendMsgNoData       = "選択した対象に計測データがありません。期間や対象を変えてお試しください。"
	trendPowerNote       = "非有意はトレンドが無いことを意味しません（非有意 ≠ トレンド無し）。蓄積年数が少ない間は検定の検出力が低いため、Sen の傾き・符号・記述統計を主たる判断材料にしてください。"
)

// SeasonalTrendRepo は SeasonalTrendHandler が必要とする最小 DB ポート（DIP・consumer 最小 interface）。
// repository.Querier が満たす。GetDevice を含むため authz の DeviceGetter も満たす（所有者認可で流用）。
type SeasonalTrendRepo interface {
	GetUser(ctx context.Context, id int64) (repository.User, error)
	ListDevicesByUser(ctx context.Context, userID int64) ([]repository.Device, error)
	GetDevice(ctx context.Context, id int64) (repository.Device, error)
	ListDailySensorAggregatesJST(ctx context.Context, arg repository.ListDailySensorAggregatesJSTParams) ([]repository.ListDailySensorAggregatesJSTRow, error)
}

// SeasonalTrendHandler は統計分析ページ（GET /analysis/trend）を提供する。
type SeasonalTrendHandler struct {
	Repo SeasonalTrendRepo
}

// TrendQuery は統計分析ページの入力（device_id・集計粒度）。granularity は monthly/yearly のみ（既定 monthly）。
type TrendQuery struct {
	DeviceID    int64  `form:"device_id"`
	Granularity string `form:"granularity" binding:"omitempty,oneof=monthly yearly"`
}

// Show は統計分析ページを描画する（GET /analysis/trend・RequireAuth 前提）。
// device_id 未指定＝未選択（空セクション＋案内）。指定時は所有検証（非所有/不在→404・列挙防止）後、
// JST ロールアップ→一次/厳密判定を同期で組み立てる。HX-Request はセクション部分、通常はフルページを 200 で返す。
func (h *SeasonalTrendHandler) Show(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)

	var q TrendQuery
	_ = c.ShouldBindQuery(&q) // 不正な granularity は既定 monthly へフォールバック（断定しない）
	gran := granularityMonthly
	if q.Granularity == granularityYearly {
		gran = granularityYearly
	}

	devices, err := h.Repo.ListDevicesByUser(ctx, uid)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	user, err := h.Repo.GetUser(ctx, uid)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	// 未選択: 検定を断定せず空セクション＋案内（要件 1.4）。
	if q.DeviceID <= 0 {
		h.render(c, devices, 0, gran, user.Name,
			component.TrendSectionView{HasData: false, EmptyMessage: trendMsgSelectDevice})
		return
	}

	// 所有者認可（閲覧系＝非所有/不在は 404・列挙防止・要件 9.1/9.2）。
	device, err := authz.RequireDeviceOwner(ctx, h.Repo, q.DeviceID, uid)
	if err != nil {
		renderDeviceReadError(c, err)
		return
	}

	// JST 日次集計を取得（取得下限を広めに取り全履歴を拾う）。
	since := pgconv.Timestamptz(dateOnlyJST(time.Now()).AddDate(-trendLookbackYears, 0, 0))
	rows, err := h.Repo.ListDailySensorAggregatesJST(ctx, repository.ListDailySensorAggregatesJSTParams{
		DeviceID:   device.ID,
		RecordedAt: since,
	})
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	section, err := buildTrendSection(rows, gran)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	h.render(c, devices, device.ID, gran, user.Name, section)
}

// render は HX-Request 時は TrendSection を部分返却、通常はフルページを描画する（alert_rule と同型）。
func (h *SeasonalTrendHandler) render(c *gin.Context, devices []repository.Device, selectedID int64, gran, userName string, section component.TrendSectionView) {
	if c.GetHeader("HX-Request") != "" {
		renderPage(c, http.StatusOK, component.TrendSection(section))
		return
	}
	renderPage(c, http.StatusOK, page.SeasonalTrend(page.SeasonalTrendPageView{
		Layout: layout.AppLayoutData{
			Title:     seasonalTrendTitle,
			UserName:  userName,
			CSRFToken: csrf.Token(c.Request),
			CSSURL:    view.CSSURL(),
			Nav:       component.SidebarNav{Current: component.NavAnalysisTrend},
		},
		Devices:     toDeviceOptions(devices, selectedID),
		Granularity: gran,
		Section:     section,
	}))
}

// buildTrendSection は JST 日次集計から #trend-section の表示データを組み立てる。
// ロールアップ→一次（Seasonal MK/Sen/N_eff）・厳密（Hamed-Rao＋多重比較 BH）判定・検出力留保・
// 平年比・チャート option を同期で算出する（design D2: 月次 N 小ゆえ同期で足りる）。
func buildTrendSection(rows []repository.ListDailySensorAggregatesJSTRow, gran string) (component.TrendSectionView, error) {
	if len(rows) == 0 {
		return component.TrendSectionView{HasData: false, EmptyMessage: trendMsgNoData}, nil
	}
	buckets := rollupDailyJST(rows, gran)
	span := spanYears(buckets)
	stepsPerYear := 12.0
	if gran == granularityYearly {
		stepsPerYear = 1.0
	}

	// 指標ごとの一次/厳密判定（純粋層 chart へ委譲）。
	temp := judgeMetric(buckets, metricTemperature, "温度", "℃", span, stepsPerYear)
	hum := judgeMetric(buckets, metricHumidity, "湿度", "%", span, stepsPerYear)

	// 多重比較補正（厳密 p 値配列に BH）。reject[i]=true で補正後も棄却。
	reject := chart.BenjaminiHochberg([]float64{temp.strictMK.PValue, hum.strictMK.PValue}, trendAlpha)

	badges := []component.TrendBadgeView{
		temp.primaryBadge(),
		temp.strictBadge(reject[0]),
		hum.primaryBadge(),
		hum.strictBadge(reject[1]),
	}

	// 平年比（温度・暦月平均・年≥3 のみ）。
	clim := buildClimatology(buckets, metricTemperature)

	// 温度トレンドチャート＋日較差ΔT チャート。
	trendOpt, err := buildTrendChartJSON(buckets, temp, clim.Values)
	if err != nil {
		return component.TrendSectionView{}, err
	}
	diurnalOpt, err := buildDiurnalChartJSON(rows)
	if err != nil {
		return component.TrendSectionView{}, err
	}

	section := component.TrendSectionView{
		HasData:           true,
		Badges:            badges,
		TempRows:          toRollupRows(buckets, metricTemperature),
		HumidityRows:      toRollupRows(buckets, metricHumidity),
		TrendOptionJSON:   trendOpt,
		DiurnalOptionJSON: diurnalOpt,
		TrendColor:        trendChartColor,
		ChartUnit:         "℃",
		ClimatologyNote:   clim.Note,
	}
	// 検出力不足（いずれかの指標）は留保注記を出す（非有意≠トレンド無し・要件 6.1/6.2）。
	if temp.caution || hum.caution {
		section.PowerNote = trendPowerNote
	}
	return section, nil
}

// metricTrend は1指標の一次/厳密判定の途中結果。
type metricTrend struct {
	label, unit  string
	values       []float64
	sen          chart.SenResult
	slopePerYear float64
	caution      bool           // N_eff<10 または span<3年
	primaryMK    chart.MKResult // Seasonal MK（一次）
	strictMK     chart.MKResult // Hamed-Rao 補正 MK（厳密）
}

// judgeMetric は1指標の一次（Seasonal MK/Sen/N_eff）・厳密（Hamed-Rao）判定を算出する。
func judgeMetric(buckets []trendBucket, metric, label, unit string, span int, stepsPerYear float64) metricTrend {
	values, months := bucketSeries(buckets, metric)
	sen := chart.SensSlope(values)
	r1 := chart.Lag1Autocorr(values)
	nEff := chart.EffectiveSampleSize(len(values), r1)
	return metricTrend{
		label:        label,
		unit:         unit,
		values:       values,
		sen:          sen,
		slopePerYear: sen.Slope * stepsPerYear, // 月次は ×12 で「単位/年」へ換算
		caution:      nEff < trendNEffMin || span < trendSpanYears,
		primaryMK:    chart.SeasonalMannKendall(values, months),
		strictMK:     chart.HamedRaoModifiedMK(values),
	}
}

// primaryBadge は一次判定（多重比較未補正）のバッジを返す。
func (m metricTrend) primaryBadge() component.TrendBadgeView {
	dir, verdict := classifyTrend(m.primaryMK, m.caution)
	return component.TrendBadgeView{
		Metric:    m.label,
		Stage:     trendStagePrimary,
		Direction: dir,
		Verdict:   verdict,
		Slope:     formatSlopePerYear(m.slopePerYear, m.unit),
		PValue:    formatTrendP(m.primaryMK.PValue),
	}
}

// strictBadge は厳密判定（Hamed-Rao＋多重比較補正）のバッジを返す。rejected は BH 補正後の棄却可否。
func (m metricTrend) strictBadge(rejected bool) component.TrendBadgeView {
	dir, verdict := classifyStrict(m.strictMK, rejected, m.caution)
	return component.TrendBadgeView{
		Metric:    m.label,
		Stage:     trendStageStrict,
		Direction: dir,
		Verdict:   verdict,
		Slope:     formatSlopePerYear(m.slopePerYear, m.unit),
		PValue:    formatTrendP(m.strictMK.PValue),
	}
}

// classifyTrend は一次判定の信号方向と文言を返す。検出力不足は断定せず「判定保留」（flat=グレー）。
func classifyTrend(mk chart.MKResult, caution bool) (direction, verdict string) {
	if caution {
		return "flat", "判定保留"
	}
	switch {
	case mk.PValue < trendAlpha && mk.Z > 0:
		return "up", "有意な上昇"
	case mk.PValue < trendAlpha && mk.Z < 0:
		return "down", "有意な下降"
	default:
		return "flat", "非有意"
	}
}

// classifyStrict は厳密判定の信号方向と文言を返す（有意性は BH 補正後の rejected で判定）。
func classifyStrict(mk chart.MKResult, rejected, caution bool) (direction, verdict string) {
	if caution {
		return "flat", "判定保留"
	}
	switch {
	case rejected && mk.Z > 0:
		return "up", "有意な上昇"
	case rejected && mk.Z < 0:
		return "down", "有意な下降"
	default:
		return "flat", "非有意"
	}
}

// buildTrendChartJSON は温度ロールアップの長期トレンドチャート option を構築する。
// Sen 線（intercept+slope*i）・ブートストラップ CI 帯（傾き CI を切片起点で扇状に）・平年比・有意区間を載せる。
func buildTrendChartJSON(buckets []trendBucket, m metricTrend, climValues []float64) (string, error) {
	n := len(m.values)
	senLine := make([]float64, n)
	for i := range senLine {
		senLine[i] = m.sen.Intercept + m.sen.Slope*float64(i)
	}
	lo, hi := chart.BlockBootstrapSenCI(m.values, 0, trendBootstrapB, trendAlpha, trendBootstrapSeed)
	ciLower := make([]float64, n)
	ciUpper := make([]float64, n)
	for i := range ciLower {
		ciLower[i] = m.sen.Intercept + lo*float64(i)
		ciUpper[i] = m.sen.Intercept + hi*float64(i)
	}
	// 有意区間: 一次判定が有意（検出力充足）のときのみ全区間を強調する。
	var significant []chart.Run
	if !m.caution && m.primaryMK.PValue < trendAlpha && n >= 1 {
		significant = []chart.Run{{StartIdx: 0, EndIdx: n - 1}}
	}
	// 表示系列は2桁に丸める（endLabel/markPoint が長い小数を出さない・サマリ表の丸めと整合）。
	// 検定（MK/Sen）は judgeMetric で生値に対し済みゆえ、ここは表示専用の丸めで安全。
	return chart.TrendChartOptionJSON(chart.TrendChartSpec{
		Labels:      bucketKeys(buckets),
		Color:       trendChartColor,
		Unit:        "℃",
		RollupAvg:   chartRound2(m.values),
		SenLine:     chartRound2(senLine),
		CILower:     chartRound2(ciLower),
		CIUpper:     chartRound2(ciUpper),
		Climatology: chartRound2(climValues),
		Significant: significant,
	})
}

// buildDiurnalChartJSON は日次 ΔT（最高−最低）の推移チャート option を構築する（要件 2.4）。
func buildDiurnalChartJSON(rows []repository.ListDailySensorAggregatesJSTRow) (string, error) {
	labels := make([]string, len(rows))
	deltaT := make([]float64, len(rows))
	for i, r := range rows {
		labels[i] = r.ReadingDate.Time.Format("2006-01-02")
		deltaT[i] = aggregateToFloat(r.MaxTemperature) - aggregateToFloat(r.MinTemperature)
	}
	return chart.DiurnalRangeChartOptionJSON(labels, deltaT, trendChartColor, "℃")
}

// toRollupRows はバケット統計を表示用 RollupRow（整形済み文字列）へ写す。
func toRollupRows(buckets []trendBucket, metric string) []component.RollupRow {
	out := make([]component.RollupRow, 0, len(buckets))
	for _, b := range buckets {
		s := metricStat(b, metric)
		out = append(out, component.RollupRow{
			Bucket:       b.Key,
			Avg:          fmtTrend2(s.Avg),
			Max:          fmtTrend2(s.Max),
			Min:          fmtTrend2(s.Min),
			DiurnalRange: fmtTrend2(s.DiurnalRange),
			StdDev:       fmtTrend2(s.StdDev),
			CV:           cvString(s),
			Samples:      strconv.Itoa(b.Samples),
		})
	}
	return out
}

// spanYears はロールアップが跨る暦年数（distinct Year）を返す（検出力留保の span 判定用）。
func spanYears(buckets []trendBucket) int {
	years := make(map[int]bool)
	for _, b := range buckets {
		years[b.Year] = true
	}
	return len(years)
}

// bucketKeys はバケットキー列（"2024-01" 等）を返す（チャート X 軸ラベル）。
func bucketKeys(buckets []trendBucket) []string {
	keys := make([]string, len(buckets))
	for i, b := range buckets {
		keys[i] = b.Key
	}
	return keys
}

// formatSlopePerYear は Sen の傾きを符号付き "単位/年" に整形する（"+0.03 ℃/年" / "-0.15 %/年"）。
func formatSlopePerYear(v float64, unit string) string {
	return fmt.Sprintf("%+.2f %s/年", v, unit)
}

// formatTrendP は p 値を "0.012" に整形する（極小は "<0.001"）。
func formatTrendP(p float64) string {
	if p < 0.001 {
		return "<0.001"
	}
	return strconv.FormatFloat(p, 'f', 3, 64)
}

// fmtTrend2 は小数第2位までに整形する（サマリ表セル）。
func fmtTrend2(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// chartRound2 は表示用チャート系列を2桁に丸めた新スライスで返す（入力非破壊）。
// ロールアップ平均は割り切れない長小数になり得るため、endLabel/markPoint のクラッタを防ぐ。
// nil/空は nil を返す。
func chartRound2(vs []float64) []float64 {
	if len(vs) == 0 {
		return nil
	}
	out := make([]float64, len(vs))
	for i, v := range vs {
		out[i] = math.Round(v*100) / 100
	}
	return out
}

// cvString は変動係数を整形する（未定義は "—"）。
func cvString(s rollupStat) string {
	if !s.HasCV {
		return statEmptyMark
	}
	return fmtTrend2(s.CV)
}
