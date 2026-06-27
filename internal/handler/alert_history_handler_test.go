package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// fakeAlertHistoryRepo は AlertHistoryRepo の手書きモック (DB 非依存)。
// GetUser は map で引き未登録は pgx.ErrNoRows。Count/List/Devices は戻り値・エラー注入と、
// 引数 captor (last*)・呼び出し記録 (*Called) に対応する。
type fakeAlertHistoryRepo struct {
	users   map[int64]repository.User
	userErr error

	devices    []repository.Device
	devicesErr error

	countVal    int64
	countErr    error
	countCalled bool
	lastCount   repository.CountAlertHistoriesInRangeParams

	listRows   []repository.ListAlertHistoriesPaginatedRow
	listErr    error
	listCalled bool
	lastList   repository.ListAlertHistoriesPaginatedParams
}

func (f *fakeAlertHistoryRepo) GetUser(_ context.Context, id int64) (repository.User, error) {
	if f.userErr != nil {
		return repository.User{}, f.userErr
	}
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return repository.User{}, pgx.ErrNoRows
}

func (f *fakeAlertHistoryRepo) ListDevicesByUser(_ context.Context, _ int64) ([]repository.Device, error) {
	if f.devicesErr != nil {
		return nil, f.devicesErr
	}
	return f.devices, nil
}

func (f *fakeAlertHistoryRepo) CountAlertHistoriesInRange(_ context.Context, arg repository.CountAlertHistoriesInRangeParams) (int64, error) {
	f.countCalled = true
	f.lastCount = arg
	return f.countVal, f.countErr
}

func (f *fakeAlertHistoryRepo) ListAlertHistoriesPaginated(_ context.Context, arg repository.ListAlertHistoriesPaginatedParams) ([]repository.ListAlertHistoriesPaginatedRow, error) {
	f.listCalled = true
	f.lastList = arg
	return f.listRows, f.listErr
}

// コンパイル時に AlertHistoryRepo を満たすことを保証する。
var _ AlertHistoryRepo = (*fakeAlertHistoryRepo)(nil)

// ownerAlertHistoryRepo は所有者(uid=7)と本人デバイス2件を備えた fake を返す。
// 既定は 0 件 (countVal=0・listRows=nil)。各テストが必要に応じ上書きする。
func ownerAlertHistoryRepo() *fakeAlertHistoryRepo {
	return &fakeAlertHistoryRepo{
		users: map[int64]repository.User{7: {ID: 7, Name: "テスト農場主"}},
		devices: []repository.Device{
			{ID: 1, UserID: 7, Name: "ハウスA温湿度計"},
			{ID: 2, UserID: 7, Name: "ハウスB温湿度計"},
		},
	}
}

// alertHistoryTestRow は発火時刻・指標・演算子・閾値・実測値・通知・デバイス名を指定した履歴行を作る。
func alertHistoryTestRow(triggeredAt time.Time, metric, op string, threshold, actual float64, notified bool, deviceName string) repository.ListAlertHistoriesPaginatedRow {
	return repository.ListAlertHistoriesPaginatedRow{
		Metric:      metric,
		Operator:    op,
		Threshold:   pgconv.Numeric2(threshold),
		ActualValue: pgconv.Numeric2(actual),
		IsNotified:  notified,
		TriggeredAt: pgconv.Timestamptz(triggeredAt),
		DeviceName:  deviceName,
	}
}

// newAlertHistoryRouterWithUser は auth.SetUserID 済み (認証済み) で履歴ルートを配線したルータを返す。
func newAlertHistoryRouterWithUser(h *AlertHistoryHandler, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	withUser := func(c *gin.Context) { auth.SetUserID(c, userID); c.Next() }
	r.GET("/alerts/history", withUser, h.Index)
	return r
}

// --- 3.2 初期表示 ---

// TestAlertHistoryIndex_初期表示は200フルページで全デバイス最新順 は、パラメータ無しで
// フルページを 200 で返し、device_id=nil・全期間センチネル・limit20・offset0・user_id=7 でクエリが
// 呼ばれ、整形済み行・デバイス選択肢を含むことを検証する (R1.1〜1.4, R5.1, R8.1)。
func TestAlertHistoryIndex_初期表示は200フルページで全デバイス最新順(t *testing.T) {
	repo := ownerAlertHistoryRepo()
	repo.countVal = 1
	repo.listRows = []repository.ListAlertHistoriesPaginatedRow{
		alertHistoryTestRow(time.Date(2026, 4, 20, 5, 30, 0, 0, time.UTC), "temperature", ">", 35.00, 38.50, true, "ハウスA温湿度計"),
	}
	r := newAlertHistoryRouterWithUser(&AlertHistoryHandler{Repo: repo}, 7)

	w := getPath(r, "/alerts/history")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	assertHistoryBodyHas(t, body,
		"テスト農場主",                                                                         // App ヘッダーのユーザー名
		"アラート履歴",                                                                         // 見出し
		`id="alert-history-list"`,                                                        // 結果領域
		"data-table", "2026-04-20 14:30", "ハウスA温湿度計", "温度", "&gt; 35.00℃", "38.50℃", "済", // 一覧 (JST 変換・条件エスケープ)
		"<html",             // フルページ (レイアウト付与)
		"全デバイス", "ハウスB温湿度計", // デバイス選択肢
	)

	if !repo.listCalled {
		t.Fatal("ListAlertHistoriesPaginated が呼ばれていない")
	}
	if repo.lastList.DeviceID != nil {
		t.Errorf("DeviceID=%v, want nil (全デバイス)", repo.lastList.DeviceID)
	}
	if repo.lastList.UserID != 7 {
		t.Errorf("UserID=%d, want 7 (テナント分離)", repo.lastList.UserID)
	}
	if repo.lastList.LimitN != 20 {
		t.Errorf("LimitN=%d, want 20", repo.lastList.LimitN)
	}
	if repo.lastList.OffsetN != 0 {
		t.Errorf("OffsetN=%d, want 0", repo.lastList.OffsetN)
	}
	if y := repo.lastList.FromAt.Time.Year(); y != 1970 {
		t.Errorf("from センチネル year=%d, want 1970", y)
	}
	if y := repo.lastList.ToAt.Time.Year(); y != 9999 {
		t.Errorf("to センチネル year=%d, want 9999", y)
	}
}

// TestAlertHistoryIndex_サイドバーはアラート履歴のみactiveで文脈リンクなし は、アラート履歴
// フルページのサイドバーが「🕐 アラート履歴」のみ active で、デバイス文脈リンクを描画しない
// ことを固定する (R1.3/2.5)。
func TestAlertHistoryIndex_サイドバーはアラート履歴のみactiveで文脈リンクなし(t *testing.T) {
	repo := ownerAlertHistoryRepo()
	r := newAlertHistoryRouterWithUser(&AlertHistoryHandler{Repo: repo}, 7)

	w := getPath(r, "/alerts/history")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `class="active">🕐 アラート履歴`) {
		t.Errorf("アラート履歴が active になっていない:\n%s", body)
	}
	if strings.Contains(body, "📟 デバイス詳細") || strings.Contains(body, "📈 センサーデータ履歴") {
		t.Errorf("アラート履歴にデバイス文脈リンクが描画されている (R1.3 違反):\n%s", body)
	}
}

// TestAlertHistoryIndex_フルページで選択中デバイスにselected は、device_id=2 (数値・本人所有) の
// フルページで該当 option に selected が付くことを検証する (R5.1)。
func TestAlertHistoryIndex_フルページで選択中デバイスにselected(t *testing.T) {
	repo := ownerAlertHistoryRepo()
	r := newAlertHistoryRouterWithUser(&AlertHistoryHandler{Repo: repo}, 7)

	w := getPath(r, "/alerts/history?device_id=2")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	assertHistoryBodyHas(t, w.Body.String(), `value="2" selected`)
}

// --- 3.2 HTMX 検索・ページ送り ---

// TestAlertHistoryIndex_HTMX検索はフラグメントのみでデバイス絞り込み は、HX-Request 時に
// レイアウトを含まないフラグメントのみを返し、device_id が *int64 で・期間境界が end-of-day まで・
// user_id=7 でクエリへ渡ることを検証する (R2.1, R2.3, R2.4, R8.1)。
func TestAlertHistoryIndex_HTMX検索はフラグメントのみでデバイス絞り込み(t *testing.T) {
	repo := ownerAlertHistoryRepo()
	repo.countVal = 1
	repo.listRows = []repository.ListAlertHistoriesPaginatedRow{
		alertHistoryTestRow(time.Date(2026, 4, 20, 4, 15, 0, 0, time.UTC), "humidity", "<", 30.00, 25.00, false, "ハウスB温湿度計"),
	}
	r := newAlertHistoryRouterWithUser(&AlertHistoryHandler{Repo: repo}, 7)

	w := requestWithHeaders(r, http.MethodGet, "/alerts/history?device_id=2&from=2026-04-13&to=2026-04-20",
		map[string]string{"HX-Request": "true"})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	assertHistoryBodyHas(t, body, `id="alert-history-list"`, "ハウスB温湿度計", "&lt; 30.00%", "25.00%", "未")
	if strings.Contains(body, "<html") {
		t.Errorf("HTMX 応答にレイアウト(<html)が含まれている:\n%s", body)
	}

	// device_id=2 が *int64 で・user_id=7 で List/Count へ渡る。
	if repo.lastList.DeviceID == nil || *repo.lastList.DeviceID != 2 {
		t.Errorf("List.DeviceID=%v, want *2", repo.lastList.DeviceID)
	}
	if repo.lastCount.DeviceID == nil || *repo.lastCount.DeviceID != 2 {
		t.Errorf("Count.DeviceID=%v, want *2", repo.lastCount.DeviceID)
	}
	if repo.lastList.UserID != 7 || repo.lastCount.UserID != 7 {
		t.Errorf("UserID list=%d count=%d, want ともに 7 (テナント分離)", repo.lastList.UserID, repo.lastCount.UserID)
	}
	// from=当日始端、to=end-of-day。
	if !repo.lastList.FromAt.Time.Equal(time.Date(2026, 4, 13, 0, 0, 0, 0, jst)) {
		t.Errorf("from=%v, want 2026-04-13 始端 JST", repo.lastList.FromAt.Time)
	}
	reading2359 := time.Date(2026, 4, 20, 23, 59, 0, 0, jst)
	nextDay := time.Date(2026, 4, 21, 0, 0, 0, 0, jst)
	if repo.lastList.ToAt.Time.Before(reading2359) || !repo.lastList.ToAt.Time.Before(nextDay) {
		t.Errorf("to=%v, want end-of-day (2026-04-20 23:59 含む・翌日未満)", repo.lastList.ToAt.Time)
	}
	// Count と List は同一区間を共有する (総ページ算出が同条件)。
	if !repo.lastCount.FromAt.Time.Equal(repo.lastList.FromAt.Time) || !repo.lastCount.ToAt.Time.Equal(repo.lastList.ToAt.Time) {
		t.Errorf("Count 区間が List 区間と不一致")
	}
}

// TestAlertHistoryIndex_ページ送りでoffset20と条件保持URL は、page=2 で OFFSET=20 になり、
// ページャに検索条件 (device_id/from/to) を保持した URL が出力されることを検証する (R3.2, R3.4)。
func TestAlertHistoryIndex_ページ送りでoffset20と条件保持URL(t *testing.T) {
	repo := ownerAlertHistoryRepo()
	repo.countVal = 50 // 3ページ → ページャ表示
	repo.listRows = []repository.ListAlertHistoriesPaginatedRow{
		alertHistoryTestRow(time.Date(2026, 4, 20, 5, 30, 0, 0, time.UTC), "temperature", ">", 35.00, 38.50, true, "ハウスA温湿度計"),
	}
	r := newAlertHistoryRouterWithUser(&AlertHistoryHandler{Repo: repo}, 7)

	w := requestWithHeaders(r, http.MethodGet, "/alerts/history?device_id=2&from=2026-04-01&to=2026-04-20&page=2",
		map[string]string{"HX-Request": "true"})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	if repo.lastList.OffsetN != 20 {
		t.Errorf("page=2 の OffsetN=%d, want 20", repo.lastList.OffsetN)
	}
	body := w.Body.String()
	assertHistoryBodyHas(t, body, `class="pagination"`, "device_id=2", "from=2026-04-01", "to=2026-04-20")
}

// TestAlertHistoryIndex_過大ページは最終ページにクランプ は、総ページ超過の page でも最終ページ
// (offset=40) にクランプされることを検証する (R3.3)。
func TestAlertHistoryIndex_過大ページは最終ページにクランプ(t *testing.T) {
	repo := ownerAlertHistoryRepo()
	repo.countVal = 50 // 3ページ → 最終 offset=40
	r := newAlertHistoryRouterWithUser(&AlertHistoryHandler{Repo: repo}, 7)
	getPath(r, "/alerts/history?page=999")
	if repo.lastList.OffsetN != 40 {
		t.Errorf("過大 page の OffsetN=%d, want 40 (3ページ目)", repo.lastList.OffsetN)
	}
}

// --- 3.2 0件・バリデーション・エラー写像 ---

// TestAlertHistoryIndex_0件は空状態でページャ非表示 は、該当0件で空状態メッセージを表示し
// ページャを出力しないことを検証する (R6.1, R6.2)。
func TestAlertHistoryIndex_0件は空状態でページャ非表示(t *testing.T) {
	repo := ownerAlertHistoryRepo() // countVal=0・listRows=nil
	r := newAlertHistoryRouterWithUser(&AlertHistoryHandler{Repo: repo}, 7)

	w := getPath(r, "/alerts/history")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	assertHistoryBodyHas(t, body, "指定期間のアラート履歴はありません。")
	if strings.Contains(body, "data-table") {
		t.Errorf("0件なのにデータ一覧テーブルが描画されている:\n%s", body)
	}
	if strings.Contains(body, `class="pagination"`) {
		t.Errorf("0件なのにページャが描画されている:\n%s", body)
	}
}

// TestAlertHistoryIndex_開始日が終了日より後は200インラインで一覧未実行 は、from>to のとき
// 200+インラインエラーを返し、件数/一覧クエリを呼ばないことを検証する (R7.1, Decision 3)。
func TestAlertHistoryIndex_開始日が終了日より後は200インラインで一覧未実行(t *testing.T) {
	repo := ownerAlertHistoryRepo()
	r := newAlertHistoryRouterWithUser(&AlertHistoryHandler{Repo: repo}, 7)

	w := getPath(r, "/alerts/history?from=2026-04-20&to=2026-04-01")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (エラーページへ遷移しない)", w.Code)
	}
	body := w.Body.String()
	assertHistoryBodyHas(t, body, "error-message", "終了日は開始日以降の日付を指定してください")
	if repo.countCalled || repo.listCalled {
		t.Errorf("from>to で件数/一覧クエリが呼ばれた: count=%v list=%v", repo.countCalled, repo.listCalled)
	}
}

// TestAlertHistoryIndex_日付形式エラーは200インラインで検索未実行 は、形式不正時に
// 200+インラインエラーを返し検索クエリを呼ばないことを検証する (R7.1)。
func TestAlertHistoryIndex_日付形式エラーは200インラインで検索未実行(t *testing.T) {
	repo := ownerAlertHistoryRepo()
	r := newAlertHistoryRouterWithUser(&AlertHistoryHandler{Repo: repo}, 7)

	w := getPath(r, "/alerts/history?from=2026/04/20")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	assertHistoryBodyHas(t, w.Body.String(), "error-message")
	if repo.countCalled || repo.listCalled {
		t.Errorf("形式エラー時にクエリが呼ばれた: count=%v list=%v", repo.countCalled, repo.listCalled)
	}
}

// TestAlertHistoryIndex_テナント分離はUserIDをクエリへ渡す は、非所有 device_id を指定しても
// user_id=7 がクエリへ渡る (クエリスコープで分離) ことを検証する (R8.1, R8.3)。
func TestAlertHistoryIndex_テナント分離はUserIDをクエリへ渡す(t *testing.T) {
	repo := ownerAlertHistoryRepo()
	repo.countVal = 1
	repo.listRows = []repository.ListAlertHistoriesPaginatedRow{
		alertHistoryTestRow(time.Date(2026, 4, 20, 5, 30, 0, 0, time.UTC), "temperature", ">", 35.00, 38.50, true, "ハウスA温湿度計"),
	}
	r := newAlertHistoryRouterWithUser(&AlertHistoryHandler{Repo: repo}, 7)

	getPath(r, "/alerts/history?device_id=999") // 非所有(数値)
	if repo.lastList.UserID != 7 || repo.lastCount.UserID != 7 {
		t.Errorf("UserID list=%d count=%d, want ともに 7", repo.lastList.UserID, repo.lastCount.UserID)
	}
	if repo.lastList.DeviceID == nil || *repo.lastList.DeviceID != 999 {
		t.Errorf("DeviceID=%v, want *999 (クエリスコープが空を返す)", repo.lastList.DeviceID)
	}
}

// TestAlertHistoryIndex_非所有device_idは空表示 は、非所有 device_id (モックが空返却) のとき
// 空状態メッセージを返すことを検証する (R8.3 列挙防止)。
func TestAlertHistoryIndex_非所有device_idは空表示(t *testing.T) {
	repo := ownerAlertHistoryRepo() // countVal=0・listRows=nil (クエリスコープで空)
	r := newAlertHistoryRouterWithUser(&AlertHistoryHandler{Repo: repo}, 7)

	w := requestWithHeaders(r, http.MethodGet, "/alerts/history?device_id=999",
		map[string]string{"HX-Request": "true"})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	assertHistoryBodyHas(t, w.Body.String(), "指定期間のアラート履歴はありません。")
}

// TestAlertHistoryIndex_device_id非数値は400 は、device_id が非数値のとき 400 を返し
// クエリを呼ばないことを検証する。
func TestAlertHistoryIndex_device_id非数値は400(t *testing.T) {
	repo := ownerAlertHistoryRepo()
	r := newAlertHistoryRouterWithUser(&AlertHistoryHandler{Repo: repo}, 7)

	w := getPath(r, "/alerts/history?device_id=abc")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", w.Code)
	}
	if repo.countCalled || repo.listCalled {
		t.Errorf("400 なのにクエリが呼ばれた: count=%v list=%v", repo.countCalled, repo.listCalled)
	}
}

// TestAlertHistoryIndex_DBエラーは500 は、各クエリの error を 500 へ写像することを検証する (R9.1)。
func TestAlertHistoryIndex_DBエラーは500(t *testing.T) {
	t.Run("Count エラー", func(t *testing.T) {
		repo := ownerAlertHistoryRepo()
		repo.countErr = errors.New("db down")
		r := newAlertHistoryRouterWithUser(&AlertHistoryHandler{Repo: repo}, 7)
		if w := getPath(r, "/alerts/history"); w.Code != http.StatusInternalServerError {
			t.Errorf("Count エラー status=%d, want 500", w.Code)
		}
	})
	t.Run("List エラー", func(t *testing.T) {
		repo := ownerAlertHistoryRepo()
		repo.countVal = 5
		repo.listErr = errors.New("db down")
		r := newAlertHistoryRouterWithUser(&AlertHistoryHandler{Repo: repo}, 7)
		if w := getPath(r, "/alerts/history"); w.Code != http.StatusInternalServerError {
			t.Errorf("List エラー status=%d, want 500", w.Code)
		}
	})
	t.Run("GetUser エラー(フルページ)", func(t *testing.T) {
		repo := ownerAlertHistoryRepo()
		repo.userErr = errors.New("db down")
		r := newAlertHistoryRouterWithUser(&AlertHistoryHandler{Repo: repo}, 7)
		if w := getPath(r, "/alerts/history"); w.Code != http.StatusInternalServerError {
			t.Errorf("GetUser エラー status=%d, want 500", w.Code)
		}
	})
	t.Run("ListDevicesByUser エラー(フルページ)", func(t *testing.T) {
		repo := ownerAlertHistoryRepo()
		repo.devicesErr = errors.New("db down")
		r := newAlertHistoryRouterWithUser(&AlertHistoryHandler{Repo: repo}, 7)
		if w := getPath(r, "/alerts/history"); w.Code != http.StatusInternalServerError {
			t.Errorf("ListDevicesByUser エラー status=%d, want 500", w.Code)
		}
	})
}
