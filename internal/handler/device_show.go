// device_show.go はデバイス詳細画面 (GET /devices/:device)・期間切替フラグメント
// (GET /devices/:device/chart)・論理削除 (DELETE /devices/:device) の HTTP 境界を担う。
// device.go の DeviceHandler を共有し (S4 と同 struct・別ファイル)、リクエスト解釈・
// 所有者認可写像・行→表示 primitive 写像・SVG 生成 (internal/chart) の呼出・templ 描画に集中する。
// 業務ロジックは持たない (service 層なし)。
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/authz"
	"github.com/HiroshiKawano/go_iot/internal/chart"
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

// usesRawSeries は生データの折れ線 (実測値) で描く期間か判定する (24h/3d/7d)。
// 30d のみ 1 か月ぶんの生データが過密になるため日次 max/min 集計に集約する。
func usesRawSeries(period string) bool {
	return period != "30d"
}

// rawLabelFor は生データ折れ線の X ラベル整形関数を period から選ぶ。
// 24h は時刻のみ "HH:MM"、複数日 (3d/7d) は日跨ぎで時刻が重複するため日付付き "M/D HH:MM"。
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

	chartArea, err := h.buildChartArea(ctx, id, period, now)
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

	if _, err := authz.RequireDeviceOwner(ctx, h.Repo, id, uid); err != nil {
		renderDeviceReadError(c, err) // 不在/非所有とも 404
		return
	}

	chartArea, err := h.buildChartArea(ctx, id, q.Period, now)
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

// buildChartArea は period に応じてグラフデータを取得し、温度/湿度 SVG を生成した
// グラフ領域 View を返す。24h/3d/7d=生データ1系列 (実測値の折れ線)、30d=日次 max/min の2系列
// (1 か月は生データだと過密なため日次集計に集約する、という設計閾値)。
// X ラベルは 24h が "HH:MM"、3d/7d は日跨ぎのため "M/D HH:MM"、30d は日付 "MM-DD"。
func (h *DeviceHandler) buildChartArea(ctx context.Context, deviceID int64, period string, now time.Time) (component.DeviceChartAreaView, error) {
	var tempSeries, humSeries []chart.Series
	tempHover, humHover := "[]", "[]" // 既定 (日次集計=30d はホバー無効)

	if usesRawSeries(period) {
		rows, err := h.Repo.ListRecentSensorReadings(ctx, repository.ListRecentSensorReadingsParams{
			DeviceID:   deviceID,
			RecordedAt: pgconv.Timestamptz(periodSince(period, now)),
		})
		if err != nil {
			return component.DeviceChartAreaView{}, err
		}
		tempSeries, humSeries = rawSeries(rows, rawLabelFor(period))
		// 生データ折れ線のみホバー (十字ポインター+値/時刻) 用の点列を持たせる。
		tempHover = hoverJSON(chart.LineChartHoverPoints(tempSeries))
		humHover = hoverJSON(chart.LineChartHoverPoints(humSeries))
	} else {
		rows, err := h.Repo.ListDailySensorAggregates(ctx, repository.ListDailySensorAggregatesParams{
			DeviceID:   deviceID,
			RecordedAt: pgconv.Timestamptz(periodSince(period, now)),
		})
		if err != nil {
			return component.DeviceChartAreaView{}, err
		}
		tempSeries, humSeries = dailySeries(rows)
	}

	return component.DeviceChartAreaView{
		DeviceID:             deviceID,
		Period:               period,
		TemperatureSVG:       chart.LineChartSVG("温度", "℃", tempSeries),
		HumiditySVG:          chart.LineChartSVG("湿度", "%", humSeries),
		TemperatureHoverJSON: tempHover,
		HumidityHoverJSON:    humHover,
	}, nil
}

// hoverJSON は折れ線ホバーの点列を JSON 文字列へ整形する (templ の x-data へ埋め込む)。
// 点が無い場合は "[]" を返す (Alpine 側でホバー無効)。
func hoverJSON(pts []chart.HoverPoint) string {
	if len(pts) == 0 {
		return "[]"
	}
	b, err := json.Marshal(pts)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// rawSeries は生データ行を温度/湿度それぞれ1系列 (折れ線) へ写像する。
// X ラベルは label で整形する (24h は "HH:MM"、複数日は "M/D HH:MM")。
// 0 件なら空 Points の系列を返す (空グラフ分岐は chart 側)。
func rawSeries(rows []repository.SensorReading, label func(pgtype.Timestamptz) string) (temp, hum []chart.Series) {
	tempPts := make([]chart.Point, 0, len(rows))
	humPts := make([]chart.Point, 0, len(rows))
	for _, r := range rows {
		l := label(r.RecordedAt)
		tempPts = append(tempPts, chart.Point{Label: l, Y: pgconv.NumericToFloat(r.Temperature)})
		humPts = append(humPts, chart.Point{Label: l, Y: pgconv.NumericToFloat(r.Humidity)})
	}
	return []chart.Series{{Points: tempPts}}, []chart.Series{{Points: humPts}}
}

// dailySeries は日次集計行を温度/湿度それぞれ「最高(実線)・最低(破線)」の2系列へ写像する。
// X ラベルは日付の "MM-DD"。max/min は sqlc が interface{} 型のため aggregateToFloat で安全変換する。
func dailySeries(rows []repository.ListDailySensorAggregatesRow) (temp, hum []chart.Series) {
	tMax := make([]chart.Point, 0, len(rows))
	tMin := make([]chart.Point, 0, len(rows))
	hMax := make([]chart.Point, 0, len(rows))
	hMin := make([]chart.Point, 0, len(rows))
	for _, r := range rows {
		label := monthDayLabel(r.ReadingDate)
		tMax = append(tMax, chart.Point{Label: label, Y: aggregateToFloat(r.MaxTemperature)})
		tMin = append(tMin, chart.Point{Label: label, Y: aggregateToFloat(r.MinTemperature)})
		hMax = append(hMax, chart.Point{Label: label, Y: aggregateToFloat(r.MaxHumidity)})
		hMin = append(hMin, chart.Point{Label: label, Y: aggregateToFloat(r.MinHumidity)})
	}
	temp = []chart.Series{
		{Name: "最高", Points: tMax},
		{Name: "最低", Dashed: true, Points: tMin},
	}
	hum = []chart.Series{
		{Name: "最高", Points: hMax},
		{Name: "最低", Dashed: true, Points: hMin},
	}
	return temp, hum
}

// buildDeviceInfoView はデバイス行を情報パネル View へ写像する。
// 場所未設定は "未設定"、最終通信は絶対表記 ("YYYY-MM-DD HH:MM:SS") / 未通信は "未通信"。
func buildDeviceInfoView(d repository.Device) component.DeviceInfoView {
	return component.DeviceInfoView{
		Name:         d.Name,
		MacAddress:   d.MacAddress,
		Location:     deviceLocationOrDefault(d),
		StatusActive: d.IsActive,
		LastCommText: lastCommAbsText(d),
		EditURL:      "/devices/" + strconv.FormatInt(d.ID, 10) + "/edit",
	}
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

// deviceLocationOrDefault は設置場所を返し、未設定 (nil/空) は "未設定" とする (R2.6)。
func deviceLocationOrDefault(d repository.Device) string {
	if loc := deviceLocation(d); loc != "" {
		return loc
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

// monthDayLabel は集計日を 3d/7d/30d グラフの X ラベル "MM-DD" に整形する。
// ReadingDate は DB の DATE(recorded_at) バケット (時点ではなく日付値) であり、その日境界は
// DB セッション TZ 依存 (design Out of Boundary / Open Question)。ここでは TZ 変換せず日付を
// そのまま表示する (Date の .In() はかえって境界をずらすため適用しない)。
func monthDayLabel(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("01-02")
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
