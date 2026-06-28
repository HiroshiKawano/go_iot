// readings.go はセンサーデータ履歴画面 (GET /devices/:device/readings) の
// ドメイン整形ロジック (DB 非依存の純関数) を担う。HTTP 境界の ReadingsHandler /
// 消費 interface ReadingsRepo は後続タスクで本ファイルに追加する。
// ここでは日付→区間境界の写像・ページ正規化/総ページ算出・通信遅延整形・
// 集計フォーマット・ページャ URL 生成に集中する (業務ロジックは持たない)。
// jst・formatActual・aggregateToFloat は同 package handler の既存ヘルパを流用する。
package handler

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/authz"
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
	"github.com/jackc/pgx/v5/pgtype"
)

// readingsPageTitle はフルページ <title> (モック readings.html の <title> に一致)。
const readingsPageTitle = "センサーデータ履歴 - 農業IoTシステム"

// ReadingsRepo は ReadingsHandler が必要とする最小 DB ポート (DIP・consumer 最小 interface)。
// *repository.Queries / repository.Querier がこれを満たす。テストでは手書きモックへ差し替える。
// GetDevice は authz.DeviceGetter も満たす (所有者認可で流用)。
type ReadingsRepo interface {
	GetUser(ctx context.Context, id int64) (repository.User, error)
	GetDevice(ctx context.Context, id int64) (repository.Device, error)
	ListSensorReadingsPaginated(ctx context.Context, arg repository.ListSensorReadingsPaginatedParams) ([]repository.SensorReading, error)
	GetSensorReadingsSummary(ctx context.Context, arg repository.GetSensorReadingsSummaryParams) (repository.GetSensorReadingsSummaryRow, error)
	CountSensorReadingsInRange(ctx context.Context, arg repository.CountSensorReadingsInRangeParams) (int64, error)
	// ListSensorReadingsInRange は期間内の全行 (BETWEEN・ASC・LIMIT なし) を取得する
	// (CSV エクスポート・集計帳票の共通入力)。CSV 経路 (Export) と画面帳票 (Index・4.1) が共有する。
	ListSensorReadingsInRange(ctx context.Context, arg repository.ListSensorReadingsInRangeParams) ([]repository.SensorReading, error)
}

// ReadingsHandler はセンサーデータ履歴画面の HTTP 境界を担う (GET /devices/:device/readings)。
// device_show.go と同じ package handler の既存ヘルパ (renderPage/renderComponent/renderError/
// renderDeviceReadError/jst/formatActual/aggregateToFloat) を流用する。業務ロジックは持たない。
type ReadingsHandler struct {
	Repo ReadingsRepo
}

// Index はセンサーデータ履歴画面を描画する (RequireAuth 前提)。
// 非数値 ID→400、所有者認可 (不在/非所有→404 列挙防止・日付検証より先)、ユーザー取得 (失敗→500)、
// 日付 from/to を区間境界へ写像する。形式エラー時は検索/集計を呼ばず 200+空一覧+インラインエラー、
// 正常時は件数→ページクランプ→一覧+集計を同一区間で取得する。HX-Request 有はフラグメント、
// 無はフルページを 200 で返す。DB 想定外は 500。
func (h *ReadingsHandler) Index(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)

	id, err := strconv.ParseInt(c.Param("device"), 10, 64)
	if err != nil {
		renderError(c, http.StatusBadRequest) // 非数値 ID
		return
	}

	device, err := authz.RequireDeviceOwner(ctx, h.Repo, id, uid)
	if err != nil {
		renderDeviceReadError(c, err) // 不在/非所有とも 404 (列挙防止)・日付検証より先
		return
	}

	user, err := h.Repo.GetUser(ctx, uid)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	from := c.Query("from")
	to := c.Query("to")
	items := c.QueryArray("items")
	fromTS, toTS, errs := parseDateBounds(from, to)

	var list component.DeviceReadingsListView
	if len(errs) > 0 {
		// 形式エラー: 区間が確定できないため検索/集計/全行クエリは呼ばず、空一覧+インラインエラーで 200。
		// 帳票・CSV リンクも出さない (有効な区間が無いため)。
		list = component.DeviceReadingsListView{
			Summary:    buildSummary(repository.GetSensorReadingsSummaryRow{}),
			HasData:    false,
			Pagination: buildReadingsPagination(id, from, to, 1, 1),
			Errors:     errs,
			Quality:    buildQualityMetricsView(nil), // 形式不正は品質メタを算出せず空状態 ("—")
		}
	} else {
		list, err = h.fetchResults(ctx, device, from, to, fromTS, toTS, c.Query("page"), items)
		if err != nil {
			renderError(c, http.StatusInternalServerError)
			return
		}
	}

	if c.GetHeader("HX-Request") != "" {
		renderComponent(c, component.DeviceReadingsList(list)) // 部分更新: フラグメントのみ
		return
	}
	v := page.ReadingsView{
		Layout: layout.AppLayoutData{
			Title:     readingsPageTitle,
			UserName:  user.Name,
			CSRFToken: csrf.Token(c.Request),
			CSSURL:    view.CSSURL(),
			// 現在ページ=センサーデータ履歴・選択中デバイス=所有者認可後に確定済みの id
			// (新規データアクセスなし・R1.5)。詳細↔履歴の相互往復導線を供給する。
			Nav: component.SidebarNav{Current: component.NavReadings, DeviceID: id},
		},
		DeviceID:   id,
		DeviceName: device.Name,
		From:       from,
		To:         to,
		Items:      effectiveMetricItems(items), // 項目フィルタ checkbox の checked echo (4.2・未選択は両方)
		List:       list,
	}
	renderPage(c, http.StatusOK, page.ReadingsPage(v))
}

// fetchResults は区間確定後に件数→ページクランプ→一覧+集計+全行を取得し結果領域 View を組み立てる。
// 件数・一覧・集計・全行 (帳票/CSV) はすべて同一 (fromTS,toTS) 境界を共有する (R7 連動・ページ非依存)。
// device は帳票の作物別適正帯解決と CSV リンクの device ID 用、items は CSV リンクの出力項目用。
func (h *ReadingsHandler) fetchResults(ctx context.Context, device repository.Device, from, to string, fromTS, toTS time.Time, pageQuery string, items []string) (component.DeviceReadingsListView, error) {
	id := device.ID
	fromParam := pgconv.Timestamptz(fromTS)
	toParam := pgconv.Timestamptz(toTS)

	total, err := h.Repo.CountSensorReadingsInRange(ctx, repository.CountSensorReadingsInRangeParams{
		DeviceID: id, RecordedAt: fromParam, RecordedAt_2: toParam,
	})
	if err != nil {
		return component.DeviceReadingsListView{}, err
	}

	totalPages := totalPagesOf(total)
	pageNo := parsePage(pageQuery)
	if pageNo > totalPages {
		pageNo = totalPages // 過大ページは最終ページへクランプ
	}
	offset := int32((pageNo - 1) * pageSize)

	rows, err := h.Repo.ListSensorReadingsPaginated(ctx, repository.ListSensorReadingsPaginatedParams{
		DeviceID: id, RecordedAt: fromParam, RecordedAt_2: toParam,
		Limit: pageSize, Offset: offset,
	})
	if err != nil {
		return component.DeviceReadingsListView{}, err
	}

	summary, err := h.Repo.GetSensorReadingsSummary(ctx, repository.GetSensorReadingsSummaryParams{
		DeviceID: id, RecordedAt: fromParam, RecordedAt_2: toParam,
	})
	if err != nil {
		return component.DeviceReadingsListView{}, err
	}

	// 集計帳票・CSV の共通入力 = 同一区間の全行 (BETWEEN・ASC・LIMIT なし)。一覧/集計と同一境界ゆえ
	// CSV・帳票・一覧/集計ボックスが整合する (R7)。CSV リンクは適用済み from/to/items を反映する。
	allRows, err := h.Repo.ListSensorReadingsInRange(ctx, repository.ListSensorReadingsInRangeParams{
		DeviceID: id, RecordedAt: fromParam, RecordedAt_2: toParam,
	})
	if err != nil {
		return component.DeviceReadingsListView{}, err
	}

	// 品質メタ (data-quality-meta): 行フラグは全行 ASC 文脈で算出し ID で表示行へ引き当て、
	// 期間メトリクス/総合バッジは同一 BETWEEN 区間 (一覧/集計/CSV と整合) から組む。
	flagsByID := rowFlagsByID(allRows)
	historyRows := buildReadingHistoryRows(rows, flagsByID)
	return component.DeviceReadingsListView{
		Summary:    buildSummary(summary),
		Rows:       historyRows,
		HasData:    len(historyRows) > 0,
		Pagination: buildReadingsPagination(id, from, to, pageNo, totalPages),
		Errors:     map[string]string{},
		Report:     buildReadingsReport(allRows, deviceCrop(device)),
		CSVURL:     csvDownloadURL(id, from, to, items),
		Quality:    buildQualityMetricsView(allRows),
	}, nil
}

// buildReadingHistoryRows は計測行を履歴一覧 View 行へ写像する
// (日時=分まで JST・温湿度=小数2桁・通信遅延=整数秒)。
// flagsByID は reading ID → 品質フラグ集合の写像 (異常行のみ収録)。正常行は引き当て無し=空 (要件 1.3)。
func buildReadingHistoryRows(rows []repository.SensorReading, flagsByID map[int64][]domain.QualityFlag) []component.ReadingHistoryRow {
	out := make([]component.ReadingHistoryRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, component.ReadingHistoryRow{
			RecordedAt:   timefmt.DateTimeMinuteJP(pgconv.TimestamptzToTime(r.RecordedAt).In(jst)),
			Temp:         formatActual(r.Temperature),
			Humidity:     formatActual(r.Humidity),
			Delay:        formatDelay(r.RecordedAt, r.CreatedAt),
			QualityFlags: flagsByID[r.ID],
		})
	}
	return out
}

// buildReadingsPagination は現在/総ページと前後リンク (from/to 保持) を組み立てる。
func buildReadingsPagination(deviceID int64, from, to string, current, last int) component.PaginationView {
	return component.PaginationView{
		Current: current,
		Last:    last,
		HasPrev: current > 1,
		HasNext: current < last,
		PrevURL: readingsURL(deviceID, from, to, current-1),
		NextURL: readingsURL(deviceID, from, to, current+1),
	}
}

// pageSize は履歴一覧の1ページあたり件数 (R4.1)。
const pageSize = 20

// summaryEmptyMark は集計対象0件のときに各項目へ表示するプレースホルダ (0.00 誤表示の回避)。
const summaryEmptyMark = "—"

// distantPast / distantFuture は「日付未指定＝全期間」を BETWEEN クエリ (from/to 必須) で
// 成立させるためのセンチネル境界。now() 非依存の固定値としテストを決定的にする。
// IoT 稼働開始は 2026 のため 1970〜9999 で全データを包含する (design Decision)。
var (
	distantPast   = time.Date(1970, 1, 1, 0, 0, 0, 0, jst)
	distantFuture = time.Date(9999, 12, 31, 23, 59, 59, 999999999, jst)
)

// parseDateBounds は from/to (YYYY-MM-DD・任意) を BETWEEN 用の検索区間へ写す。
// 未指定: from→遠過去センチネル / to→遠未来センチネル (全期間検索が成立)。
// to 指定時は当日を含めるため end-of-day (23:59:59.999999999) まで上限を拡張する。
// 日付は JST 暦日として解釈する。形式不正は errs に日本語メッセージを積み、
// その区間値は使わせない (呼び出し側が len(errs)>0 で検索をスキップする)。
func parseDateBounds(from, to string) (fromTS, toTS time.Time, errs map[string]string) {
	errs = map[string]string{}
	fromTS = distantPast
	toTS = distantFuture

	if from != "" {
		if d, err := time.ParseInLocation("2006-01-02", from, jst); err != nil {
			errs["from"] = "開始日は YYYY-MM-DD 形式で入力してください"
		} else {
			fromTS = d // 当日始端 00:00:00 JST
		}
	}
	if to != "" {
		if d, err := time.ParseInLocation("2006-01-02", to, jst); err != nil {
			errs["to"] = "終了日は YYYY-MM-DD 形式で入力してください"
		} else {
			// BETWEEN は両端含むため、当日 23:59:59.999999999 まで含めて取りこぼしを防ぐ。
			toTS = d.Add(24*time.Hour - time.Nanosecond)
		}
	}
	return fromTS, toTS, errs
}

// parsePage は page 文字列を 1 以上の int へ正規化する。
// 未指定・1未満・数値として解釈不可は 1 とする (R4.4)。前後空白は許容する。
func parsePage(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// totalPagesOf は総件数から総ページ数を返す (1ページ pageSize 件・0件でも1ページ・R4.2)。
// 加算を伴う ceil ((total+pageSize-1)/pageSize) は total が int64 上限近傍でラップし
// 負のページ数を生むため、除算を先行させオーバーフローを構造的に排除する
// (現実の件数では到達しないが、全入力で正の値を保証する防御的実装)。
func totalPagesOf(total int64) int {
	if total <= 0 {
		return 1
	}
	pages := total / pageSize
	if total%pageSize != 0 {
		pages++
	}
	return int(pages)
}

// formatDelay は計測時刻 (recordedAt) とサーバ受信時刻 (createdAt) の差を
// 四捨五入した整数秒「N秒」へ整形する (R5.1/5.2)。
// クロックずれによる負値は「0秒」にクランプする。
func formatDelay(recordedAt, createdAt pgtype.Timestamptz) string {
	diff := pgconv.TimestamptzToTime(createdAt).Sub(pgconv.TimestamptzToTime(recordedAt))
	secs := diff.Seconds()
	if secs < 0 {
		secs = 0
	}
	return fmt.Sprintf("%d秒", int(math.Round(secs)))
}

// buildSummary は集計行を表示用 SummaryView (整形済み6項目) へ写す。
// sample_count==0 (該当データ0件) のときは全項目を「—」にし 0.00 の誤表示を避ける (R3.1/3.2)。
// 平均は numeric (formatActual)、最高/最低は sqlc が interface{} で生成するため
// aggregateToFloat 経由で安全に float 化する。
func buildSummary(row repository.GetSensorReadingsSummaryRow) component.SummaryView {
	if row.SampleCount == 0 {
		return component.SummaryView{
			AvgTemp: summaryEmptyMark, MaxTemp: summaryEmptyMark, MinTemp: summaryEmptyMark,
			AvgHum: summaryEmptyMark, MaxHum: summaryEmptyMark, MinHum: summaryEmptyMark,
		}
	}
	return component.SummaryView{
		AvgTemp: formatActual(row.AvgTemperature) + "℃",
		MaxTemp: formatAggregate(row.MaxTemperature) + "℃",
		MinTemp: formatAggregate(row.MinTemperature) + "℃",
		AvgHum:  formatActual(row.AvgHumidity) + "%",
		MaxHum:  formatAggregate(row.MaxHumidity) + "%",
		MinHum:  formatAggregate(row.MinHumidity) + "%",
	}
}

// formatAggregate は集計の最高/最低 (interface{}) を小数2桁の数値文字列へ整形する (単位は呼び出し側)。
func formatAggregate(v interface{}) string {
	return fmt.Sprintf("%.2f", aggregateToFloat(v))
}

// readingsURL は現在の開始日・終了日を保持したまま page を差し替えた相対 URL を返す (ページャ用・R8.2)。
// from/to は未指定 ("") のとき省略し、page は常に付与する。
func readingsURL(deviceID int64, from, to string, page int) string {
	q := url.Values{}
	if from != "" {
		q.Set("from", from)
	}
	if to != "" {
		q.Set("to", to)
	}
	q.Set("page", strconv.Itoa(page))
	return fmt.Sprintf("/devices/%d/readings?%s", deviceID, q.Encode())
}
