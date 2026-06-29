// device_show.go はデバイス詳細画面 (GET /devices/:device)・期間切替フラグメント
// (GET /devices/:device/chart)・論理削除 (DELETE /devices/:device) の HTTP 境界を担う。
// device.go の DeviceHandler を共有し (S4 と同 struct・別ファイル)、リクエスト解釈・
// 所有者認可写像・行→表示 primitive 写像・ECharts option 構築 (internal/chart) の呼出・templ 描画に集中する。
// 業務ロジックは持たない (service 層なし)。
package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/authz"
	"github.com/HiroshiKawano/go_iot/internal/chart"
	"github.com/HiroshiKawano/go_iot/internal/domain"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/timefmt"
	"github.com/HiroshiKawano/go_iot/internal/view"
	"github.com/HiroshiKawano/go_iot/internal/view/component"
	"github.com/HiroshiKawano/go_iot/internal/view/layout"
	"github.com/HiroshiKawano/go_iot/internal/view/page"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// defaultPeriod は ?period 未指定時の既定期間。
const defaultPeriod = "24h"

// deviceShowTitleSuffix はフルページ <title> の接尾辞。
const deviceShowTitleSuffix = " - 農業IoTシステム"

// グラフの単位と系列色 (ECharts コンテナの data-unit/data-color、option の lineStyle.color)。
// 温度=暖色 / 湿度=寒色。モック (mocks/html/style.css) の配色を踏襲する (R2.4)。
const (
	tempChartUnit     = "℃"
	humidityChartUnit = "%"
	tempLineColor     = "#e8590c"
	humidityLineColor = "#1971c2"
)

// jst は表示用の日本標準時 (UTC+9)。timestamptz は時点 (instant) のため、農場運営者の
// ローカル時刻で見せるには表示直前に JST へ変換する (R2.4/R5.3 の日本向け絶対表記)。
// DST が無いため FixedZone で十分 (tzdata 非依存・time.LoadLocation のエラーを避ける)。
// 注意: 日次集計の日付バケット (DATE(recorded_at)) は DB セッション TZ 依存で本変換の対象外
// (design Open Questions「接続 TZ=Asia/Tokyo を前提」= Out of Boundary。monthDayLabel 参照)。
var jst = time.FixedZone("JST", 9*60*60)

// chartQuery は期間切替フラグメント (Chart) の period クエリバインド。
// 24h/3d/7d/30d 以外は binding で弾き 400 にする (R8.2)。
type chartQuery struct {
	Period string `form:"period" binding:"required,oneof=24h 3d 7d 30d"`
}

// isValidPeriod は period が許容値 (24h/3d/7d/30d) か判定する (Show の任意 ?period 検証用)。
func isValidPeriod(p string) bool {
	return p == "24h" || p == "3d" || p == "7d" || p == "30d"
}

// periodSince は period から取得開始時刻を返す (now 基準)。
func periodSince(period string, now time.Time) time.Time {
	switch period {
	case "3d":
		return now.AddDate(0, 0, -3)
	case "7d":
		return now.AddDate(0, 0, -7)
	case "30d":
		return now.AddDate(0, 0, -30)
	default: // "24h"
		return now.Add(-24 * time.Hour)
	}
}

// rawLabelFor は生データ折れ線の X ラベル整形関数を period から選ぶ。
// 24h は時刻のみ "HH:MM"、複数日 (3d/7d/30d) は日跨ぎで時刻が重複するため日付付き "M/D HH:MM"。
func rawLabelFor(period string) func(pgtype.Timestamptz) string {
	if period == defaultPeriod {
		return hourMinuteLabel
	}
	return dayTimeLabel
}

// Show はデバイス詳細フルページを描画する (GET /devices/:device・RequireAuth 前提)。
// 非数値 ID→400、任意 ?period(既定24h・不正→400)、所有者認可(不在/非所有→404 列挙防止)、
// デバイス情報＋最新10件＋期間別グラフを取得して 1 ページを 200 で返す。DB 想定外は 500。
func (h *DeviceHandler) Show(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)
	now := time.Now()

	id, err := strconv.ParseInt(c.Param("device"), 10, 64)
	if err != nil {
		renderError(c, http.StatusBadRequest) // R8.1 非数値 ID
		return
	}

	period := c.Query("period")
	if period == "" {
		period = defaultPeriod
	} else if !isValidPeriod(period) {
		renderError(c, http.StatusBadRequest) // R8.2 不正 period
		return
	}

	device, err := authz.RequireDeviceOwner(ctx, h.Repo, id, uid)
	if err != nil {
		renderDeviceReadError(c, err) // 不在/非所有とも 404 (R7.2 列挙防止)
		return
	}

	user, err := h.Repo.GetUser(ctx, uid)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	latest, err := h.Repo.ListLatestSensorReadings(ctx, id)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	chartArea, err := h.buildChartArea(ctx, device, period, now)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	v := page.DeviceShowView{
		Layout: layout.AppLayoutData{
			Title:     device.Name + deviceShowTitleSuffix,
			UserName:  user.Name,
			CSRFToken: csrf.Token(c.Request),
			CSSURL:    view.CSSURL(),
			// 現在ページ=デバイス詳細・選択中デバイス=所有者認可後に確定済みの id
			// (新規データアクセスなし・文脈リンクは現在表示中デバイスのみを指す・R1.5)。
			Nav: component.SidebarNav{Current: component.NavDeviceShow, DeviceID: id},
		},
		DeviceID:   id,
		Info:       buildDeviceInfoView(device),
		ChartArea:  chartArea,
		Latest:     buildLatestReadingsView(id, latest),
		DeleteName: device.Name,
	}
	renderPage(c, http.StatusOK, page.DeviceShowPage(v))
}

// Chart は期間切替のグラフ領域フラグメントのみを返す (GET /devices/:device/chart・RequireAuth 前提)。
// 非数値 ID→400、period バリデーション (required,oneof=24h 3d 7d 30d・不正→400)、所有者認可
// (不在/非所有→404 列挙防止) を行い、期間別 SVG を再生成してグラフ領域 component を 200 で返す。
// 最新計測テーブルは期間に連動しないため返さない (R3.4/5.4)。アクティブ期間はサーバー側で往復する。
func (h *DeviceHandler) Chart(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)
	now := time.Now()

	id, err := strconv.ParseInt(c.Param("device"), 10, 64)
	if err != nil {
		renderError(c, http.StatusBadRequest) // R8.1 非数値 ID
		return
	}

	var q chartQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		renderError(c, http.StatusBadRequest) // R8.2 period 不正/未指定
		return
	}

	device, err := authz.RequireDeviceOwner(ctx, h.Repo, id, uid)
	if err != nil {
		renderDeviceReadError(c, err) // 不在/非所有とも 404
		return
	}

	chartArea, err := h.buildChartArea(ctx, device, q.Period, now)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	renderComponent(c, component.DeviceChartArea(chartArea))
}

// Delete はデバイスを論理削除する (DELETE /devices/:device・RequireAuth + CSRF 前提)。
// 非数値 ID→400、所有者認可 (不在→404・非所有→403。閲覧系と異なり mutation は BOLA 403)、
// 論理削除を実行後、HX-Request 有なら HX-Redirect ヘッダ＋200、非 HTMX (フォーム
// _method=delete) なら 303 でダッシュボードへ遷移させる (§9)。DB 想定外は 500。
func (h *DeviceHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)

	id, err := strconv.ParseInt(c.Param("device"), 10, 64)
	if err != nil {
		renderError(c, http.StatusBadRequest) // R8.1 非数値 ID
		return
	}

	if _, err := authz.RequireDeviceOwner(ctx, h.Repo, id, uid); err != nil {
		renderDeviceOwnerError(c, err) // 不在→404 / 非所有→403 (BOLA)
		return
	}

	if err := h.Repo.SoftDeleteDevice(ctx, id); err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	if c.GetHeader("HX-Request") != "" {
		c.Header("HX-Redirect", "/dashboard") // HTMX はヘッダで遷移指示 (200)
		c.Status(http.StatusOK)
		return
	}
	c.Redirect(http.StatusSeeOther, "/dashboard") // 非 HTMX フォームは 303
}

// 統計オーバーレイの固定パラメータ。
//   - bandSigmaK: 正常帯 SMA±kσ の倍率 (k=2 ≈ 95%・design 確定値)。
//   - statEpsilon: ゼロ除算ガードのしきい値 (乖離率の |SMA|<eps / CV の |μ|<eps を未定義化)。
const (
	bandSigmaK  = 2.0
	statEpsilon = 1e-9
)

// statEmptyMark は数値カード・日次集計表でデータ未到着/欠測/未定義を示すプレースホルダ。
const statEmptyMark = "—"

// smaWindowFor は period 別の SMA 窓幅 (点数) を返す (約5分間隔=12点/時 前提・design 決定表)。
// 24h=12(約1時間)/3d=36(約3時間)/7d=72(約6時間)/30d=288(約1日)。点数窓のため計算層に時刻を持ち込まない。
func smaWindowFor(period string) int {
	switch period {
	case "3d":
		return 36
	case "7d":
		return 72
	case "30d":
		return 288
	default: // "24h"
		return 12
	}
}

// buildChartArea は period に応じてグラフデータを取得し、温度/湿度の ECharts option JSON
// (生実測線＋統計オーバーレイ)・数値カード・日次集計表を構築したグラフ領域 View を返す。
// 統計 (SMA/σ/正常帯/乖離率・カード・日次) はすべて取得済み生行から読み取り時に算出する
// (追加クエリ・スキーマ変更なし・R8)。計測 0 件のときは option を構築せず HasData=false
// (空メッセージのみ・templ 側で分岐) とし、カードは "—" を出す (R1.4)。
// 描画/対話 (十字ホバー・最高最低・現在値・温湿度連動) はクライアントの ECharts が担う (S5)。
func (h *DeviceHandler) buildChartArea(ctx context.Context, device repository.Device, period string, now time.Time) (component.DeviceChartAreaView, error) {
	deviceID := device.ID
	rows, err := h.Repo.ListRecentSensorReadings(ctx, repository.ListRecentSensorReadingsParams{
		DeviceID:   deviceID,
		RecordedAt: pgconv.Timestamptz(periodSince(period, now)),
	})
	if err != nil {
		return component.DeviceChartAreaView{}, err
	}

	// 計測 0 件: option を構築せず空メッセージのみ。カードは "—" でレイアウトを保つ (R1.4)。
	// VPD パネルも組まない (HasData=false で templ 側非表示・R4.5/5.4/6.3)。
	if len(rows) == 0 {
		return component.DeviceChartAreaView{
			DeviceID:        deviceID,
			Period:          period,
			HasData:         false,
			TemperatureCard: emptyStatCard(),
			HumidityCard:    emptyStatCard(),
			ShowDaily:       false,
		}, nil
	}

	// 生行 → 温度/湿度の float 列 ＋ X ラベル列 (pgconv 境界変換・計算層は float64 のみ)。
	label := rawLabelFor(period)
	labels := make([]string, len(rows))
	temps := make([]float64, len(rows))
	hums := make([]float64, len(rows))
	for i, r := range rows {
		labels[i] = label(r.RecordedAt)
		temps[i] = pgconv.NumericToFloat(r.Temperature)
		hums[i] = pgconv.NumericToFloat(r.Humidity)
	}

	window := smaWindowFor(period)
	tempSpec := overlaySpec(labels, temps, tempChartUnit, tempLineColor, window)
	humSpec := overlaySpec(labels, hums, humidityChartUnit, humidityLineColor, window)

	// 欠測ギャップ可視化 (data-quality-meta): 間隔中央値から欠測区間を検出し拡張グリッド化する。
	// MissingStats の round(間隔/中央値)-1 が「中央値の約1.5倍超で欠測」を符号化する (gapFactor≒1.5)。
	// device-show は開区間窓の視覚表現のみで率の数値は出さない (readings の BETWEEN 側に集約・C1)。
	// 欠測なし (2点未満含む) は従来描画 (RawNullable/GapBands 未設定=後方互換)。
	hasGap := false
	if _, _, gaps, ok := chart.MissingStats(intervalSeconds(rows)); ok && len(gaps) > 0 {
		slotsAfter := make([]int, len(rows))
		for _, g := range gaps {
			slotsAfter[g.StartIdx] = g.MissingSlots
		}
		tempSpec = applyGapGrid(tempSpec, slotsAfter)
		humSpec = applyGapGrid(humSpec, slotsAfter)
		hasGap = true
	}

	tempOpt, err := chart.ChartOptionJSON(tempSpec)
	if err != nil {
		return component.DeviceChartAreaView{}, err
	}
	humOpt, err := chart.ChartOptionJSON(humSpec)
	if err != nil {
		return component.DeviceChartAreaView{}, err
	}

	// 日次集計表は複数日 (3d/7d/30d) のみ (24h はカードで把握・R5.3)。
	showDaily := period != defaultPeriod
	var tempDaily, humDaily []component.DailyStatRow
	if showDaily {
		tempDaily = dailyStatRows(rows, func(r repository.SensorReading) float64 { return pgconv.NumericToFloat(r.Temperature) })
		humDaily = dailyStatRows(rows, func(r repository.SensorReading) float64 { return pgconv.NumericToFloat(r.Humidity) })
	}

	// VPD (飽差) 適正帯ダッシュボードを温湿度データから読み取り時に組む (デバイスの作物で適正帯を解決)。
	// 温湿度 option/カード/日次表とは独立 (別 option・別 DTO=無回帰・R8)。JSON 化失敗は温湿度同様 500。
	crop := deviceCrop(device)
	vpdPanel, err := buildVPDPanel(labels, temps, hums, rows, crop, period)
	if err != nil {
		return component.DeviceChartAreaView{}, err
	}

	// 露点・病害リスク蓄積解析パネルを温湿度データから読み取り時に組む (デバイスの作物で病害しきい値を解決)。
	// VPD パネルの直後に組み込む。温湿度/VPD option/カード/日次表とは独立 (別 option・別 DTO=無回帰・R7)。
	dewpointPanel, err := buildDewpointPanel(labels, temps, hums, rows, crop, period, now)
	if err != nil {
		return component.DeviceChartAreaView{}, err
	}

	return component.DeviceChartAreaView{
		DeviceID:              deviceID,
		Period:                period,
		HasData:               true,
		TemperatureOptionJSON: tempOpt,
		HumidityOptionJSON:    humOpt,
		TemperatureUnit:       tempChartUnit,
		HumidityUnit:          humidityChartUnit,
		TemperatureColor:      tempLineColor,
		HumidityColor:         humidityLineColor,
		TemperatureCard:       statCard(temps, tempChartUnit),
		HumidityCard:          statCard(hums, humidityChartUnit),
		ShowDaily:             showDaily,
		TemperatureDaily:      tempDaily,
		HumidityDaily:         humDaily,
		VPD:                   vpdPanel,
		Dewpoint:              dewpointPanel,
		HasGap:                hasGap,
	}, nil
}

// applyGapGrid は欠測スロット数列 (slotsAfter[i] = 点 i の後に挿入する nil スロット数) に従い
// ChartSpec を拡張グリッド化する。Labels と全系列を同一インデックス空間へ揃え、生実測線は
// RawNullable で分断 (欠測スロット=nil・補間しない)、連続欠測区間は GapBands で markArea
// ハイライトする。オーバーレイ (SMA/正常帯) は既定 off ゆえ直前値で carry-forward して整列を保ち、
// 乖離率 (nil 許容) は欠測スロットを nil で分断する。元 spec は破壊せず新 spec を返す。
func applyGapGrid(spec chart.ChartSpec, slotsAfter []int) chart.ChartSpec {
	n := len(spec.Labels)
	hasSMA := len(spec.SMA) == n
	hasBand := len(spec.BandLower) == n && len(spec.BandWidth) == n
	hasDev := len(spec.Deviation) == n

	extLabels := make([]string, 0, n)
	rawNullable := make([]*float64, 0, n)
	var extSMA, extLower, extWidth []float64
	var extDev []*float64
	var bands []chart.GapBand

	ext := 0 // 拡張グリッド上の現在インデックス
	for i := 0; i < n; i++ {
		startExt := ext
		// 実点をそのまま積む。
		extLabels = append(extLabels, spec.Labels[i])
		v := spec.Raw[i]
		rawNullable = append(rawNullable, &v)
		if hasSMA {
			extSMA = append(extSMA, spec.SMA[i])
		}
		if hasBand {
			extLower = append(extLower, spec.BandLower[i])
			extWidth = append(extWidth, spec.BandWidth[i])
		}
		if hasDev {
			extDev = append(extDev, spec.Deviation[i])
		}
		ext++

		// 点 i の後の欠測スロットを nil/空/carry-forward で挿入する (最終点の後は挿入しない)。
		slots := 0
		if i < len(slotsAfter) && i < n-1 {
			slots = slotsAfter[i]
		}
		if slots <= 0 {
			continue
		}
		for s := 0; s < slots; s++ {
			extLabels = append(extLabels, "")      // 欠測スロットのラベルは空 (markArea が区間を示す)
			rawNullable = append(rawNullable, nil) // 欠測=nil (線分断・補間しない)
			if hasSMA {
				extSMA = append(extSMA, spec.SMA[i]) // 既定 off ゆえ直前値で carry-forward (整列維持)
			}
			if hasBand {
				extLower = append(extLower, spec.BandLower[i])
				extWidth = append(extWidth, spec.BandWidth[i])
			}
			if hasDev {
				extDev = append(extDev, nil) // 乖離率は nil 許容ゆえ分断
			}
		}
		ext += slots
		// 連続欠測区間 = 点 i (startExt) 〜 点 i+1 (ext) の xAxis 範囲。
		bands = append(bands, chart.GapBand{StartIdx: startExt, EndIdx: ext})
	}

	out := spec // 値コピー (元 spec を破壊しない)
	out.Labels = extLabels
	out.RawNullable = rawNullable
	if hasSMA {
		out.SMA = extSMA
	}
	if hasBand {
		out.BandLower = extLower
		out.BandWidth = extWidth
	}
	if hasDev {
		out.Deviation = extDev
	}
	out.GapBands = bands
	return out
}

// deviceCrop はデバイスの作物列 (*string・NULL 許容) を domain.Crop へ写す。
// 未設定 (NULL)・不正値は空 Crop を返し、VPDRange()/CropLabel が既定 (0.3-1.5・"既定") にフォールバックする。
func deviceCrop(d repository.Device) domain.Crop {
	if d.Crop == nil {
		return ""
	}
	return domain.Crop(*d.Crop)
}

// overlaySpec は生実測列から統計オーバーレイ (SMA/正常帯/乖離率) を算出した ChartSpec を組む。
// SMA・移動σは窓幅 window (立ち上がりは expanding window)、正常帯は SMA±kσ、乖離率は (実測−SMA)/SMA%。
// すべて読み取り時計算 (生行から派生・保存しない・R8.2)。
func overlaySpec(labels []string, values []float64, unit, color string, window int) chart.ChartSpec {
	sma := chart.SMA(values, window)
	sigma := chart.MovingStdDev(values, window)
	lower, width := chart.Band(sma, sigma, bandSigmaK)
	deviation := chart.Deviation(values, sma, statEpsilon)
	return chart.ChartSpec{
		Labels:    labels,
		Unit:      unit,
		Color:     color,
		Raw:       values,
		SMA:       sma,
		BandLower: lower,
		BandWidth: width,
		Deviation: deviation,
	}
}

// statCard は期間内の数値カード (現在値=最新点・最高/最低=期間内・日較差=最高−最低) を整形する (R1.1)。
// 空入力は全項目 "—" (呼び出し側は非空を渡すが防御的)。
func statCard(values []float64, unit string) component.StatCardView {
	if len(values) == 0 {
		return emptyStatCard()
	}
	min, max := chart.MinMax(values)
	return component.StatCardView{
		Current: formatStat(values[len(values)-1], unit),
		Max:     formatStat(max, unit),
		Min:     formatStat(min, unit),
		Diurnal: formatStat(chart.DiurnalRange(values), unit),
	}
}

// emptyStatCard はデータ未到着時の数値カード (全項目 "—") を返す (R1.4)。
func emptyStatCard() component.StatCardView {
	return component.StatCardView{
		Current: statEmptyMark, Max: statEmptyMark, Min: statEmptyMark, Diurnal: statEmptyMark,
	}
}

// dailyStatRows は生行を JST 暦日でバケット化し、各日の集計 (平均/最高/最低/日較差/σ/CV) を
// 日付昇順で返す (R5.1)。データのある最古日〜最新日の間で計測の無い日は欠測行 ("—") として
// 補完し、表の体裁を保つ (R5.4)。pick は行から対象指標 (温度 or 湿度) の float を取り出す。
func dailyStatRows(rows []repository.SensorReading, pick func(repository.SensorReading) float64) []component.DailyStatRow {
	if len(rows) == 0 {
		return nil
	}
	// JST 暦日キー (時刻切り捨て) でグルーピング。
	buckets := make(map[time.Time][]float64)
	var minDay, maxDay time.Time
	for i, r := range rows {
		day := jstDay(r.RecordedAt)
		buckets[day] = append(buckets[day], pick(r))
		if i == 0 || day.Before(minDay) {
			minDay = day
		}
		if i == 0 || day.After(maxDay) {
			maxDay = day
		}
	}
	// 最古日〜最新日を1日ずつ進め、欠測日は "—" で埋める (JST は DST 無しのため AddDate で安全)。
	var out []component.DailyStatRow
	for d := minDay; !d.After(maxDay); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		vals, ok := buckets[d]
		if !ok {
			out = append(out, emptyDailyRow(dateStr))
			continue
		}
		out = append(out, dailyRow(dateStr, vals))
	}
	return out
}

// dailyRow は1日分の値列から日次集計行を整形する (単位は列見出し側・セルは素の数値)。
func dailyRow(date string, values []float64) component.DailyStatRow {
	min, max := chart.MinMax(values)
	cv, ok := chart.CV(values, statEpsilon)
	cvStr := statEmptyMark
	if ok {
		cvStr = formatStat(cv, "")
	}
	return component.DailyStatRow{
		Date:    date,
		Avg:     formatStat(chart.Mean(values), ""),
		Max:     formatStat(max, ""),
		Min:     formatStat(min, ""),
		Diurnal: formatStat(chart.DiurnalRange(values), ""),
		Sigma:   formatStat(chart.StdDev(values), ""),
		CV:      cvStr,
	}
}

// emptyDailyRow は欠測日の行 (日付以外すべて "—") を返す (R5.4)。
func emptyDailyRow(date string) component.DailyStatRow {
	return component.DailyStatRow{
		Date: date, Avg: statEmptyMark, Max: statEmptyMark, Min: statEmptyMark,
		Diurnal: statEmptyMark, Sigma: statEmptyMark, CV: statEmptyMark,
	}
}

// jstDay は計測時刻 (instant) を JST に変換し、その暦日の 0:00 JST に切り捨てて返す
// (日次バケットのキー。表示ラベルの JST と整合させる)。
func jstDay(ts pgtype.Timestamptz) time.Time {
	t := pgconv.TimestamptzToTime(ts).In(jst)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, jst)
}

// formatStat は統計値を小数2桁の文字列へ整形する (unit が非空なら単位を付す)。
func formatStat(v float64, unit string) string {
	return strconv.FormatFloat(v, 'f', 2, 64) + unit
}

// buildDeviceInfoView はデバイス行を情報パネル View へ写像する。
// 所在地は構造化 locality を認識名で表示し未設定は "未設定"、最終通信は絶対表記
// ("YYYY-MM-DD HH:MM:SS") / 未通信は "未通信"。
func buildDeviceInfoView(d repository.Device) component.DeviceInfoView {
	return component.DeviceInfoView{
		Name:         d.Name,
		MacAddress:   d.MacAddress,
		Location:     deviceLocalityOrUnset(d),
		StatusActive: d.IsActive,
		LastCommText: lastCommAbsText(d),
		Crop:         deviceCropLabelOrUnset(d),
		EditURL:      "/devices/" + strconv.FormatInt(d.ID, 10) + "/edit",
	}
}

// deviceCropLabelOrUnset はデバイスの作物を日本語ラベルで返し、未設定 (NULL)・不正値は "未設定"
// とする (情報パネル表示用。場所と同じフォールバック表記で「センサー毎に作物を持つ」ことを可視化する)。
// VPD パネルの "既定" 表記 (vpdCropLabel) とは別: あちらは適正帯しきい値の文脈、こちらは設定有無の表示。
func deviceCropLabelOrUnset(d repository.Device) string {
	if c := deviceCrop(d); c.Valid() {
		return c.Label()
	}
	return "未設定"
}

// buildLatestReadingsView は最新計測行をテーブル View へ写像する (日時=分まで・温湿度=小数2桁)。
func buildLatestReadingsView(deviceID int64, rows []repository.SensorReading) component.LatestReadingsView {
	out := make([]component.ReadingRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, component.ReadingRow{
			RecordedAt: timefmt.DateTimeMinuteJP(pgconv.TimestamptzToTime(r.RecordedAt).In(jst)),
			Temp:       formatActual(r.Temperature),
			Humidity:   formatActual(r.Humidity),
		})
	}
	return component.LatestReadingsView{DeviceID: deviceID, Rows: out}
}

// deviceLocalityOrUnset は構造化所在地を認識名 (Locality.Label()) で返し、未設定・未知値は
// "未設定" とする (R6.1/R6.3・情報パネルの従来表記)。認識名整形は deviceLocalityLabel に委譲。
func deviceLocalityOrUnset(d repository.Device) string {
	if label := deviceLocalityLabel(d); label != "" {
		return label
	}
	return "未設定"
}

// lastCommAbsText は最終通信を JST 絶対表記で返す。未通信 (未記録) は "未通信" (R2.5)。
func lastCommAbsText(d repository.Device) string {
	if !d.LastCommunicatedAt.Valid {
		return "未通信"
	}
	return timefmt.DateTimeJP(pgconv.TimestamptzToTime(d.LastCommunicatedAt).In(jst))
}

// hourMinuteLabel は計測時刻 (instant) を JST に変換し 24h グラフの X ラベル "HH:MM" に整形する。
func hourMinuteLabel(ts pgtype.Timestamptz) string {
	return pgconv.TimestamptzToTime(ts).In(jst).Format("15:04")
}

// dayTimeLabel は計測時刻 (instant) を JST に変換し 3d/7d グラフの X ラベル "M/D HH:MM" に整形する。
// 24h の "HH:MM" と異なり日跨ぎで時刻が重複するため、日付を併記して区別できるようにする。
func dayTimeLabel(ts pgtype.Timestamptz) string {
	return pgconv.TimestamptzToTime(ts).In(jst).Format("1/2 15:04")
}

// aggregateToFloat は日次集計の MAX/MIN を float64 へ安全変換する。
// これらは SQL で NUMERIC への明示キャストが無い (MAX(temperature) 等) ため sqlc が any(interface{})
// として生成する。本番の pgx/v5 は numeric を pgtype.Numeric として渡すのでそれを優先し、
// float64・nil(NULL 集計)・想定外型は 0 にフォールバックする (防御的)。
func aggregateToFloat(v interface{}) float64 {
	switch n := v.(type) {
	case pgtype.Numeric:
		return pgconv.NumericToFloat(n)
	case float64:
		return n
	default:
		return 0
	}
}

// renderDeviceReadError は閲覧系 (Show/Chart) の認可エラーを HTTP ステータスへ写す。
// R7.2 列挙防止: 不在 (ErrNoRows) も非所有 (ErrNotOwner) も同じ 404 とし存在を秘匿する。
// 想定外 (DB エラー等) は 500。
func renderDeviceReadError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, pgx.ErrNoRows), errors.Is(err, authz.ErrNotOwner):
		renderError(c, http.StatusNotFound)
	default:
		renderError(c, http.StatusInternalServerError)
	}
}
