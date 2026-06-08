// alert_history.go はアラート履歴画面 (GET /alerts/history) の HTTP 境界 (AlertHistoryHandler)・
// 消費 interface (AlertHistoryRepo)・ドメイン整形/正規化の純関数を担う。
// 兄弟画面 readings (センサーデータ履歴) と同型で、フィルタ+一覧+ページネーション+HTMX 部分更新を
// 提供する。テナント分離はクエリの d.user_id スコープに委ね、authz.RequireDeviceOwner は経由しない
// (device_id は任意フィルタ・非所有は空表示で列挙防止・design Decision 4)。
// jst・pageSize・parseDateBounds・parsePage・totalPagesOf・formatActual・render ヘルパは
// 同 package handler の既存実装を流用する (新規に作り直さない)。
package handler

import (
	"context"
	"net/http"
	"net/url"
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

// alertHistoryPageTitle はフルページ <title> (モック alert-history.html の <title> に一致)。
const alertHistoryPageTitle = "アラート履歴 - 農業IoTシステム"

// --- 入力正規化・表示整形の純関数 (DB 非依存) ---

// parseDeviceID は device_id クエリを解釈する。
// 空→(nil, true)=全デバイス / 数値→(*int64, true) / 非数値→(nil, false)=不正。
// 非所有・不在 id は数値であれば valid とし、クエリの user_id スコープが空を返す (列挙防止)。
func parseDeviceID(s string) (*int64, bool) {
	if s == "" {
		return nil, true
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, false
	}
	return &id, true
}

// dateRangeError は from・to 両指定かつ from>to のとき範囲エラーメッセージを返す。
// 片方のみ・未指定・from<=to は "" (検証スキップ/許容)。fromTS/toTS は parseDateBounds 由来で、
// to は end-of-day のため from==to は After=false で自然に許容される (R7.1/7.2)。
// 範囲検証は readings 共有の parseDateBounds には足さず本画面ローカルに置く (readings 非回帰・Decision 3)。
func dateRangeError(from, to string, fromTS, toTS time.Time) string {
	if from == "" || to == "" {
		return ""
	}
	if fromTS.After(toTS) {
		return "終了日は開始日以降の日付を指定してください"
	}
	return ""
}

// buildAlertHistoryRows は履歴行を表示用 View 行へ写す
// (発火日時=分まで JST・指標ラベル・条件=演算子+閾値2桁+単位・実測値2桁+単位・通知 済/未)。
// 値はすべて発火時点の非正規化値をそのまま使う (R4.7)。metric は CHECK 制約保証済みのため
// domain.Metric の Label()/Unit() を直接適用する。operator は記号値そのもの (templ が自動エスケープ)。
func buildAlertHistoryRows(rows []repository.ListAlertHistoriesPaginatedRow) []component.AlertHistoryRow {
	out := make([]component.AlertHistoryRow, 0, len(rows))
	for _, r := range rows {
		m := domain.Metric(r.Metric)
		unit := m.Unit()
		out = append(out, component.AlertHistoryRow{
			TriggeredAt: timefmt.DateTimeMinuteJP(pgconv.TimestamptzToTime(r.TriggeredAt).In(jst)),
			DeviceName:  r.DeviceName,
			MetricLabel: m.Label(),
			Condition:   r.Operator + " " + formatActual(r.Threshold) + unit,
			ActualValue: formatActual(r.ActualValue) + unit,
			Notified:    notifiedLabel(r.IsNotified),
		})
	}
	return out
}

// notifiedLabel は通知済みフラグを表示文言 (済/未) へ写す (R4.5/4.6)。
func notifiedLabel(notified bool) string {
	if notified {
		return "済"
	}
	return "未"
}

// buildAlertHistoryDeviceOptions は本人所有デバイスをフィルタ select の選択肢へ写す。
// selectedID (device_id クエリ) と一致するデバイスに Selected を立てる (R5.1)。
func buildAlertHistoryDeviceOptions(devices []repository.Device, selectedID string) []component.DeviceOption {
	out := make([]component.DeviceOption, 0, len(devices))
	for _, d := range devices {
		out = append(out, component.DeviceOption{
			ID:       d.ID,
			Name:     d.Name,
			Selected: strconv.FormatInt(d.ID, 10) == selectedID,
		})
	}
	return out
}

// buildAlertHistoryPagination は番号付きページャ (前へ/番号/次へ・条件保持 URL) を組み立てる。
// ページ番号は現状 1..last を列挙する (IoT 小規模・モック準拠・design Follow-up)。
func buildAlertHistoryPagination(deviceID, from, to string, current, last int) component.AlertHistoryPaginationView {
	pages := make([]component.PageLink, 0, last)
	for i := 1; i <= last; i++ {
		pages = append(pages, component.PageLink{
			Num:     i,
			URL:     alertHistoryURL(deviceID, from, to, i),
			Current: i == current,
		})
	}
	return component.AlertHistoryPaginationView{
		HasPrev: current > 1,
		HasNext: current < last,
		PrevURL: alertHistoryURL(deviceID, from, to, current-1),
		NextURL: alertHistoryURL(deviceID, from, to, current+1),
		Pages:   pages,
	}
}

// alertHistoryURL は device_id/from/to を保持し page のみ差し替えた相対 URL を返す (ページャ用・R3.2)。
// 空の device_id/from/to は省略し、page は常に付与する。
func alertHistoryURL(deviceID, from, to string, page int) string {
	q := url.Values{}
	if deviceID != "" {
		q.Set("device_id", deviceID)
	}
	if from != "" {
		q.Set("from", from)
	}
	if to != "" {
		q.Set("to", to)
	}
	q.Set("page", strconv.Itoa(page))
	return "/alerts/history?" + q.Encode()
}

// --- HTTP 境界 (AlertHistoryHandler) ---

// AlertHistoryRepo は AlertHistoryHandler が必要とする最小 DB ポート (DIP・consumer 最小 interface)。
// *repository.Queries / repository.Querier がこれを満たす。テストでは手書きモックへ差し替える。
// テナント分離は ListAlertHistoriesPaginated / CountAlertHistoriesInRange の d.user_id スコープに
// 集約され、本 interface に所有者チェック専用メソッドは持たない (Decision 4)。
type AlertHistoryRepo interface {
	GetUser(ctx context.Context, id int64) (repository.User, error)
	ListDevicesByUser(ctx context.Context, userID int64) ([]repository.Device, error)
	ListAlertHistoriesPaginated(ctx context.Context, arg repository.ListAlertHistoriesPaginatedParams) ([]repository.ListAlertHistoriesPaginatedRow, error)
	CountAlertHistoriesInRange(ctx context.Context, arg repository.CountAlertHistoriesInRangeParams) (int64, error)
}

// AlertHistoryHandler はアラート履歴画面の HTTP 境界を担う (GET /alerts/history・RequireAuth 前提)。
// 同 package handler の既存ヘルパ (renderPage/renderComponent/renderError/parseDateBounds/parsePage/
// totalPagesOf/formatActual/jst/pageSize) を流用する。業務ロジックは持たない。
type AlertHistoryHandler struct {
	Repo AlertHistoryRepo
}

// Index はアラート履歴画面を描画する (RequireAuth 前提)。
// device_id 非数値→400、device/from/to の形式エラーまたは from>to→200+インライン (検索 skip・Decision 3)、
// 正常時は件数→ページクランプ→一覧を user_id スコープで取得する。HX-Request 有はフラグメント、
// 無はフルページ (ユーザー名+デバイス選択肢を追加取得) を 200 で返す。DB 想定外は 500 (R9.1)。
// テナント分離は必ず auth.UserID(c) を UserID へ渡すことで成立する (BOLA 防止・Decision 4)。
func (h *AlertHistoryHandler) Index(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)

	deviceIDStr := c.Query("device_id")
	deviceID, ok := parseDeviceID(deviceIDStr)
	if !ok {
		renderError(c, http.StatusBadRequest) // device_id 非数値
		return
	}

	from := c.Query("from")
	to := c.Query("to")
	fromTS, toTS, errs := parseDateBounds(from, to)
	if len(errs) == 0 {
		if msg := dateRangeError(from, to, fromTS, toTS); msg != "" {
			errs["to"] = msg // 両指定 from>to は範囲エラー (形式は妥当)
		}
	}

	var list component.AlertHistoryListView
	if len(errs) > 0 {
		// 形式/範囲エラー: 区間が確定できない/矛盾するため件数/一覧クエリは呼ばず、
		// 空一覧+インラインエラーで 200 を返す (Decision 3)。
		list = component.AlertHistoryListView{
			HasData:       false,
			HasPagination: false,
			Errors:        errs,
		}
	} else {
		var err error
		list, err = h.fetchResults(ctx, uid, deviceID, deviceIDStr, from, to, fromTS, toTS, c.Query("page"))
		if err != nil {
			renderError(c, http.StatusInternalServerError)
			return
		}
	}

	if c.GetHeader("HX-Request") != "" {
		renderComponent(c, component.AlertHistoryList(list)) // 部分更新: 結果領域フラグメントのみ
		return
	}

	// フルページ時のみユーザー名とデバイス選択肢を取得する (フィルタフォームは swap 外)。
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

	v := page.AlertHistoryView{
		Layout: layout.AppLayoutData{
			Title:     alertHistoryPageTitle,
			UserName:  user.Name,
			CSRFToken: csrf.Token(c.Request),
			CSSURL:    view.CSSURL(),
		},
		Devices:  buildAlertHistoryDeviceOptions(devices, deviceIDStr),
		DeviceID: deviceIDStr,
		From:     from,
		To:       to,
		List:     list,
	}
	renderPage(c, http.StatusOK, page.AlertHistory(v))
}

// fetchResults は区間確定後に件数→ページクランプ→一覧を取得し結果領域 View を組み立てる。
// 件数・一覧は同一 (deviceID, fromTS, toTS) 境界を共有する (ページ非依存)。
// テナント分離のため uid を必ず UserID へ渡す。1ページに収まる/0件はページャを非表示にする (R3.1/R6.2)。
func (h *AlertHistoryHandler) fetchResults(ctx context.Context, uid int64, deviceID *int64, deviceIDStr, from, to string, fromTS, toTS time.Time, pageQuery string) (component.AlertHistoryListView, error) {
	fromParam := pgconv.Timestamptz(fromTS)
	toParam := pgconv.Timestamptz(toTS)

	total, err := h.Repo.CountAlertHistoriesInRange(ctx, repository.CountAlertHistoriesInRangeParams{
		UserID: uid, DeviceID: deviceID, FromAt: fromParam, ToAt: toParam,
	})
	if err != nil {
		return component.AlertHistoryListView{}, err
	}

	totalPages := totalPagesOf(total)
	pageNo := parsePage(pageQuery)
	if pageNo > totalPages {
		pageNo = totalPages // 過大ページは最終ページへクランプ
	}
	offset := int32((pageNo - 1) * pageSize)

	rows, err := h.Repo.ListAlertHistoriesPaginated(ctx, repository.ListAlertHistoriesPaginatedParams{
		UserID: uid, DeviceID: deviceID, FromAt: fromParam, ToAt: toParam,
		OffsetN: offset, LimitN: pageSize,
	})
	if err != nil {
		return component.AlertHistoryListView{}, err
	}

	historyRows := buildAlertHistoryRows(rows)
	return component.AlertHistoryListView{
		Rows:          historyRows,
		HasData:       len(historyRows) > 0,
		HasPagination: totalPages > 1, // 1ページに収まる/0件はページャ非表示 (R3.1/R6.2)
		Pagination:    buildAlertHistoryPagination(deviceIDStr, from, to, pageNo, totalPages),
		Errors:        map[string]string{},
	}, nil
}
