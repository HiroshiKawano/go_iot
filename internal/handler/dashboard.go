// dashboard.go はダッシュボード画面 (GET /dashboard) のデータ整形ロジックを担う。
// 認証フロー (auth.go) から分離し、リポジトリ行を表示用 primitive へ写像する
// unexported helper 群と、それらを束ねる buildDashboardView / Dashboard を持つ。
// 表示語彙は domain (Metric/ComparisonOperator)、pgtype 変換は pgconv、
// 相対時間は timefmt に委譲する (view へ pgtype を持ち込まない)。
package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/auth"
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
)

// readingPlaceholder は計測未受信時に温度・湿度欄へ入れる代替表記。
const readingPlaceholder = "ー"

// dashboardAlertLimit は未対応アラートバナーの表示上限 (新しい順)。
// 超過時は最新 N 件のみ表示する (明示的キャップ)。
const dashboardAlertLimit int64 = 50

// Dashboard は認証後のトップ画面を表示する (RequireAuth 適用前提)。
// 本人のユーザー名・デバイス一覧・未通知アラートを取得し、デバイス別に最新計測を
// 取得して表示用 View-model へ整形し、フルページを描画する。
// エラー振り分け: GetLatestSensorReading の sql.ErrNoRows のみ未受信 (正常・温湿度「ー」)、
// それ以外の取得エラー (ユーザー・デバイス一覧・アラート一覧・想定外の計測エラー) は 500。
func (h *AuthHandler) Dashboard(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)

	user, err := h.Repo.GetUser(ctx, uid)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	devices, err := h.Repo.ListDevicesByUser(ctx, uid)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	alerts, err := h.Repo.ListUnnotifiedAlertHistoriesWithDevice(ctx, repository.ListUnnotifiedAlertHistoriesWithDeviceParams{
		UserID: uid,
		Limit:  dashboardAlertLimit,
	})
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	// 各デバイスの最新計測を取得 (N+1。小規模前提で許容)。
	// 未受信 (ErrNoRows) は nil として整形側で「ー」に写像し、想定外エラーのみ 500。
	readings := make(map[int64]*repository.SensorReading, len(devices))
	for _, d := range devices {
		r, err := h.Repo.GetLatestSensorReading(ctx, d.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				readings[d.ID] = nil // 未受信
				continue
			}
			renderError(c, http.StatusInternalServerError)
			return
		}
		reading := r
		readings[d.ID] = &reading
	}

	dv := buildDashboardView(layout.AppLayoutData{
		Title:     "ダッシュボード - 農業IoTシステム",
		UserName:  user.Name,
		CSRFToken: csrf.Token(c.Request),
		CSSURL:    view.CSSURL(),
	}, devices, readings, alerts, time.Now())

	renderPage(c, http.StatusOK, page.DashboardPage(dv))
}

// buildDashboardView は repo 行を表示用 View-model へ写像する。
// デバイスは buildDashboardDevice、アラートは composeAlertMessage で整形する。
// readings は deviceID→最新計測 (未受信は nil)。now は相対時間整形の基準。
func buildDashboardView(
	layoutData layout.AppLayoutData,
	devices []repository.Device,
	readings map[int64]*repository.SensorReading,
	alerts []repository.ListUnnotifiedAlertHistoriesWithDeviceRow,
	now time.Time,
) page.DashboardView {
	deviceViews := make([]component.DashboardDevice, 0, len(devices))
	for _, d := range devices {
		deviceViews = append(deviceViews, buildDashboardDevice(d, readings[d.ID], now))
	}
	alertViews := make([]component.DashboardAlert, 0, len(alerts))
	for _, a := range alerts {
		alertViews = append(alertViews, component.DashboardAlert{Message: composeAlertMessage(a)})
	}
	return page.DashboardView{
		Layout:  layoutData,
		Devices: deviceViews,
		Alerts:  alertViews,
	}
}

// buildDashboardDevice はデバイス行＋最新計測(無い場合は nil)＋基準時刻から
// 表示用デバイスデータ (page.DashboardDevice) を生成する。
// 温湿度は小数2桁＋単位、未受信(reading==nil)は「ー」、
// 最終通信は通信実績なしなら「通信実績なし」、ありなら相対時間で表す。
func buildDashboardDevice(d repository.Device, reading *repository.SensorReading, now time.Time) component.DashboardDevice {
	tempText, humidityText := readingPlaceholder, readingPlaceholder
	if reading != nil {
		tempText = formatReadingText(reading.Temperature, domain.MetricTemperature.Unit())
		humidityText = formatReadingText(reading.Humidity, domain.MetricHumidity.Unit())
	}
	return component.DashboardDevice{
		ID:           d.ID,
		Name:         d.Name,
		Location:     deviceLocation(d),
		IsActive:     d.IsActive,
		TempText:     tempText,
		HumidityText: humidityText,
		LastCommText: lastCommText(d, now),
	}
}

// formatReadingText は計測値を「小数2桁＋単位」で整形する (例: 28.50℃)。
func formatReadingText(n float64, unit string) string {
	return pgconv.Format2(n) + unit
}

// lastCommText は最終通信時刻の表示文字列を返す。
// 通信実績なし(LastCommunicatedAt が未設定)なら「通信実績なし」、
// あれば now を基準とした日本語相対時間を返す。
func lastCommText(d repository.Device, now time.Time) string {
	if d.LastCommunicatedAt == nil {
		return "通信実績なし"
	}
	return timefmt.RelativeJP(*d.LastCommunicatedAt, now)
}

// deviceLocation は設置場所 (*string) を表示用文字列へ変換する (未設定 nil は "")。
func deviceLocation(d repository.Device) string {
	if d.Location == nil {
		return ""
	}
	return *d.Location
}

// composeAlertMessage は未対応アラート1件の表示文言を合成する。
// 書式: 「{デバイス名}: {指標}が{閾値}{単位}{動詞}（{実測値}{単位}）」
// 例: 「ハウスA温湿度計: 温度が35℃を超えました（38.50℃）」
func composeAlertMessage(row repository.ListUnnotifiedAlertHistoriesWithDeviceRow) string {
	m := domain.Metric(row.Metric)
	unit := m.Unit()
	return fmt.Sprintf("%s: %sが%s%s%s（%s%s）",
		row.DeviceName,
		m.Label(),
		formatThreshold(row.Threshold),
		unit,
		exceedanceVerb(domain.ComparisonOperator(row.Operator)),
		formatActual(row.ActualValue),
		unit,
	)
}

// exceedanceVerb は比較演算子を超過/下回りの動詞へ写像する。
// `>` `>=` → 「を超えました」、`<` `<=` → 「を下回りました」。
func exceedanceVerb(op domain.ComparisonOperator) string {
	switch op {
	case domain.OpLessThan, domain.OpLessThanOrEqual:
		return "を下回りました"
	default:
		// OpGreaterThan / OpGreaterThanOrEqual および想定外値は超過扱い
		return "を超えました"
	}
}

// formatThreshold は閾値を末尾ゼロ除去で整形する (35.00→35 / 35.50→35.5)。
func formatThreshold(n float64) string {
	return strconv.FormatFloat(n, 'f', -1, 64)
}

// formatActual は実測値を小数2桁固定で整形する (38.5→38.50)。
func formatActual(n float64) string {
	return pgconv.Format2(n)
}
