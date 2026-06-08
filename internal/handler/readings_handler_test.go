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

// fakeReadingsRepo は ReadingsRepo の手書きモック (DB 非依存)。
// GetUser/GetDevice は map で引き未登録は pgx.ErrNoRows。Count/List/Summary は
// 戻り値・エラー注入と、引数 captor (last*) ・呼び出し記録 (*Called) に対応する。
type fakeReadingsRepo struct {
	users   map[int64]repository.User
	userErr error

	devices map[int64]repository.Device
	getErr  error

	countVal    int64
	countErr    error
	countCalled bool
	lastCount   repository.CountSensorReadingsInRangeParams

	listRows   []repository.SensorReading
	listErr    error
	listCalled bool
	lastList   repository.ListSensorReadingsPaginatedParams

	summaryRow    repository.GetSensorReadingsSummaryRow
	summaryErr    error
	summaryCalled bool
	lastSummary   repository.GetSensorReadingsSummaryParams
}

func (f *fakeReadingsRepo) GetUser(_ context.Context, id int64) (repository.User, error) {
	if f.userErr != nil {
		return repository.User{}, f.userErr
	}
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return repository.User{}, pgx.ErrNoRows
}

func (f *fakeReadingsRepo) GetDevice(_ context.Context, id int64) (repository.Device, error) {
	if f.getErr != nil {
		return repository.Device{}, f.getErr
	}
	if d, ok := f.devices[id]; ok {
		return d, nil
	}
	return repository.Device{}, pgx.ErrNoRows
}

func (f *fakeReadingsRepo) CountSensorReadingsInRange(_ context.Context, arg repository.CountSensorReadingsInRangeParams) (int64, error) {
	f.countCalled = true
	f.lastCount = arg
	return f.countVal, f.countErr
}

func (f *fakeReadingsRepo) ListSensorReadingsPaginated(_ context.Context, arg repository.ListSensorReadingsPaginatedParams) ([]repository.SensorReading, error) {
	f.listCalled = true
	f.lastList = arg
	return f.listRows, f.listErr
}

func (f *fakeReadingsRepo) GetSensorReadingsSummary(_ context.Context, arg repository.GetSensorReadingsSummaryParams) (repository.GetSensorReadingsSummaryRow, error) {
	f.summaryCalled = true
	f.lastSummary = arg
	return f.summaryRow, f.summaryErr
}

// コンパイル時に ReadingsRepo を満たすことを保証する。
var _ ReadingsRepo = (*fakeReadingsRepo)(nil)

// ownerReadingsRepo は所有者(uid=7)とデバイス1(uid=7 所有)を備えた fake を返す。
// 既定は 0 件 (countVal=0・SampleCount=0)。各テストが必要に応じ上書きする。
func ownerReadingsRepo() *fakeReadingsRepo {
	return &fakeReadingsRepo{
		users: map[int64]repository.User{7: {ID: 7, Name: "テスト農場主"}},
		devices: map[int64]repository.Device{
			1: {ID: 1, UserID: 7, Name: "ハウスA温湿度計", MacAddress: "AA:BB:CC:DD:EE:01", IsActive: true},
		},
		summaryRow: repository.GetSensorReadingsSummaryRow{SampleCount: 0},
	}
}

// historyRow は計測時刻・サーバ受信時刻・温湿度を指定した計測行を作る (通信遅延検証用)。
func historyRow(recordedAt, createdAt time.Time, temp, hum float64) repository.SensorReading {
	return repository.SensorReading{
		DeviceID:    1,
		RecordedAt:  pgconv.Timestamptz(recordedAt),
		CreatedAt:   pgconv.Timestamptz(createdAt),
		Temperature: pgconv.Numeric2(temp),
		Humidity:    pgconv.Numeric2(hum),
	}
}

// newReadingsRouterWithUser は auth.SetUserID 済み (認証済み) で履歴ルートを配線したルータを返す。
func newReadingsRouterWithUser(h *ReadingsHandler, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	withUser := func(c *gin.Context) { auth.SetUserID(c, userID); c.Next() }
	r.GET("/devices/:device/readings", withUser, h.Index)
	return r
}

func assertHistoryBodyHas(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("応答に %q が含まれていない:\n%s", w, body)
		}
	}
}

// fullSummaryRow はデータ有の集計行 (SampleCount>0) を作る。
func fullSummaryRow() repository.GetSensorReadingsSummaryRow {
	return repository.GetSensorReadingsSummaryRow{
		AvgTemperature: pgconv.Numeric2(28.30), MaxTemperature: pgconv.Numeric2(35.20), MinTemperature: pgconv.Numeric2(18.50),
		AvgHumidity: pgconv.Numeric2(62.50), MaxHumidity: pgconv.Numeric2(85.00), MinHumidity: pgconv.Numeric2(30.20),
		SampleCount: 5,
	}
}

// --- 4.1 初期表示・所有者認可 ---

// TestReadingsIndex_初期表示は200でフルページに集計一覧デバイス名 は、日付未指定の初期表示で
// 全期間センチネル境界・limit20・offset0 でクエリが呼ばれ、フルページに集計・一覧・結果領域・
// デバイス名を含むことを検証する (R1.1, R1.5, R3.3, R6.1)。
func TestReadingsIndex_初期表示は200でフルページに集計一覧デバイス名(t *testing.T) {
	repo := ownerReadingsRepo()
	repo.countVal = 5
	repo.summaryRow = fullSummaryRow()
	repo.listRows = []repository.SensorReading{
		historyRow(
			time.Date(2026, 4, 20, 5, 30, 0, 0, time.UTC),
			time.Date(2026, 4, 20, 5, 30, 2, 0, time.UTC), // +2秒 → 通信遅延 "2秒"
			28.50, 65.30,
		),
	}
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1/readings")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	assertHistoryBodyHas(t, body,
		"テスト農場主", // App ヘッダーのユーザー名
		"センサーデータ履歴: ハウスA温湿度計",              // 見出し (デバイス名)
		`id="device-readings-list"`,        // 結果領域
		"summary-grid", "28.30℃", "62.50%", // 集計
		"data-table", "2026-04-20 14:30", "28.50", "65.30", "2秒", // 一覧 (JST 変換・通信遅延)
		"<html", // フルページ (レイアウト付与)
	)

	// 日付未指定 → 全期間センチネル境界・limit20・offset0 がモックへ渡る。
	if !repo.listCalled {
		t.Fatal("ListSensorReadingsPaginated が呼ばれていない")
	}
	if y := repo.lastList.RecordedAt.Time.Year(); y != 1970 {
		t.Errorf("from センチネル year=%d, want 1970", y)
	}
	if y := repo.lastList.RecordedAt_2.Time.Year(); y != 9999 {
		t.Errorf("to センチネル year=%d, want 9999", y)
	}
	if repo.lastList.Limit != 20 {
		t.Errorf("limit=%d, want 20", repo.lastList.Limit)
	}
	if repo.lastList.Offset != 0 {
		t.Errorf("offset=%d, want 0", repo.lastList.Offset)
	}
}

func TestReadingsIndex_非数値IDは400(t *testing.T) {
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: ownerReadingsRepo()}, 7)
	w := getPath(r, "/devices/abc/readings")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", w.Code)
	}
}

func TestReadingsIndex_不在デバイスは404で列挙防止(t *testing.T) {
	repo := ownerReadingsRepo()
	delete(repo.devices, 1) // 不在 → GetDevice が ErrNoRows
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)
	w := getPath(r, "/devices/1/readings")
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404 (不在は秘匿)", w.Code)
	}
}

func TestReadingsIndex_非所有デバイスは404で列挙防止(t *testing.T) {
	repo := ownerReadingsRepo()
	repo.devices[1] = repository.Device{ID: 1, UserID: 999, Name: "他人のデバイス"} // 別所有者
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)
	w := getPath(r, "/devices/1/readings")
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404 (非所有も不在と同一応答で秘匿)", w.Code)
	}
}

func TestReadingsIndex_ユーザー取得DBエラーは500(t *testing.T) {
	repo := ownerReadingsRepo()
	repo.userErr = errors.New("db down")
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)
	w := getPath(r, "/devices/1/readings")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500", w.Code)
	}
}

// --- 4.2 期間フィルタ検索・ページ送り ---

// TestReadingsIndex_HTMX期間検索はフラグメントのみでend_of_day は、HX-Request 時に
// レイアウトを含まないフラグメントのみを返し、終了日が end-of-day まで含めて検索に渡ることを検証する
// (R2.2, R8.1)。
func TestReadingsIndex_HTMX期間検索はフラグメントのみでend_of_day(t *testing.T) {
	repo := ownerReadingsRepo()
	repo.countVal = 5
	repo.summaryRow = fullSummaryRow()
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)

	w := requestWithHeaders(r, http.MethodGet, "/devices/1/readings?from=2026-04-13&to=2026-04-20",
		map[string]string{"HX-Request": "true"})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	assertHistoryBodyHas(t, body, `id="device-readings-list"`)
	if strings.Contains(body, "<html") {
		t.Errorf("HTMX 応答にレイアウト(<html)が含まれている:\n%s", body)
	}

	// from=当日始端、to=end-of-day がモックへ渡る (同一区間を List/Summary/Count が共有)。
	fromT := repo.lastList.RecordedAt.Time
	toT := repo.lastList.RecordedAt_2.Time
	if !fromT.Equal(time.Date(2026, 4, 13, 0, 0, 0, 0, jst)) {
		t.Errorf("from=%v, want 2026-04-13 始端 JST", fromT)
	}
	reading2359 := time.Date(2026, 4, 20, 23, 59, 0, 0, jst)
	nextDay := time.Date(2026, 4, 21, 0, 0, 0, 0, jst)
	if toT.Before(reading2359) || !toT.Before(nextDay) {
		t.Errorf("to=%v, want end-of-day (2026-04-20 23:59 含む・翌日未満)", toT)
	}
	// 集計も同一区間を使う (ページ非依存・フィルタ連動)。
	if !repo.lastSummary.RecordedAt.Time.Equal(fromT) || !repo.lastSummary.RecordedAt_2.Time.Equal(toT) {
		t.Errorf("Summary 区間=(%v,%v) が List 区間=(%v,%v) と不一致", repo.lastSummary.RecordedAt.Time, repo.lastSummary.RecordedAt_2.Time, fromT, toT)
	}
	// 件数も同一区間を共有する (総ページ数算出が同条件・design 不変条件「List/Summary/Count は同一区間」)。
	if !repo.lastCount.RecordedAt.Time.Equal(fromT) || !repo.lastCount.RecordedAt_2.Time.Equal(toT) {
		t.Errorf("Count 区間=(%v,%v) が List 区間=(%v,%v) と不一致", repo.lastCount.RecordedAt.Time, repo.lastCount.RecordedAt_2.Time, fromT, toT)
	}
	// 3クエリとも同一デバイスにスコープされる (BOLA・device 限定)。
	if repo.lastList.DeviceID != 1 || repo.lastSummary.DeviceID != 1 || repo.lastCount.DeviceID != 1 {
		t.Errorf("DeviceID list=%d summary=%d count=%d, want すべて 1", repo.lastList.DeviceID, repo.lastSummary.DeviceID, repo.lastCount.DeviceID)
	}
}

// TestReadingsIndex_開始日が終了日より後は0件扱い は、from>to（意味的に空）でも形式は妥当なため
// 通常検索を実行し0件として空状態メッセージを返すこと (R2.4) を検証する（形式エラー経路ではない）。
func TestReadingsIndex_開始日が終了日より後は0件扱い(t *testing.T) {
	repo := ownerReadingsRepo() // countVal=0・SampleCount=0
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)
	w := getPath(r, "/devices/1/readings?from=2026-04-20&to=2026-04-01")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	assertHistoryBodyHas(t, body, "指定期間の計測データはありません。")
	if strings.Contains(body, "error-message") {
		t.Errorf("from>to は形式エラーではない（error-message を出さない）:\n%s", body)
	}
	// 形式エラーの未実行経路ではなく、通常検索（0件）経路を通る。
	if !repo.countCalled || !repo.listCalled {
		t.Errorf("from>to で通常検索が実行されていない: count=%v list=%v", repo.countCalled, repo.listCalled)
	}
	// 反転区間（from=2026-04-20 始端）がそのまま渡る（BETWEEN が自然に空を返す）。
	if !repo.lastList.RecordedAt.Time.Equal(time.Date(2026, 4, 20, 0, 0, 0, 0, jst)) {
		t.Errorf("from=%v, want 2026-04-20 始端", repo.lastList.RecordedAt.Time)
	}
}

func TestReadingsIndex_ページ送りでoffsetが進む(t *testing.T) {
	repo := ownerReadingsRepo()
	repo.countVal = 50 // 3ページ
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)
	getPath(r, "/devices/1/readings?page=2")
	if repo.lastList.Offset != 20 {
		t.Errorf("page=2 の offset=%d, want 20", repo.lastList.Offset)
	}
}

func TestReadingsIndex_page正規化で0やabcはoffset0(t *testing.T) {
	for _, p := range []string{"0", "-1", "abc", ""} {
		repo := ownerReadingsRepo()
		repo.countVal = 50
		r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)
		getPath(r, "/devices/1/readings?page="+p)
		if repo.lastList.Offset != 0 {
			t.Errorf("page=%q の offset=%d, want 0", p, repo.lastList.Offset)
		}
	}
}

func TestReadingsIndex_過大ページは最終ページにクランプ(t *testing.T) {
	repo := ownerReadingsRepo()
	repo.countVal = 50 // 3ページ → 最終ページ offset=40
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)
	getPath(r, "/devices/1/readings?page=999")
	if repo.lastList.Offset != 40 {
		t.Errorf("過大 page の offset=%d, want 40 (3ページ目)", repo.lastList.Offset)
	}
}

// --- 4.3 日付形式エラー・0件・障害時のエラー写像 ---

// TestReadingsIndex_日付形式エラーは200インラインで検索クエリ未実行 は、形式不正時に
// 同一画面 200 + インラインエラー + 空一覧を返し、検索・集計クエリを実行しないことを検証する
// (R6.2, R6.3, R7.1)。
func TestReadingsIndex_日付形式エラーは200インラインで検索クエリ未実行(t *testing.T) {
	repo := ownerReadingsRepo()
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)
	w := getPath(r, "/devices/1/readings?from=2026/04/20")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (エラーページへ遷移しない)", w.Code)
	}
	body := w.Body.String()
	assertHistoryBodyHas(t, body, "error-message", "指定期間の計測データはありません。")
	// 区間が確定できないため Count/List/Summary は呼ばない。
	if repo.countCalled || repo.listCalled || repo.summaryCalled {
		t.Errorf("形式エラー時にクエリが呼ばれた: count=%v list=%v summary=%v",
			repo.countCalled, repo.listCalled, repo.summaryCalled)
	}
}

func TestReadingsIndex_0件は空状態メッセージと集計ダッシュ(t *testing.T) {
	repo := ownerReadingsRepo() // countVal=0・SampleCount=0
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)
	w := getPath(r, "/devices/1/readings")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	assertHistoryBodyHas(t, body, "指定期間の計測データはありません。", "—")
	if strings.Contains(body, "data-table") {
		t.Errorf("0件なのにデータ一覧テーブルが描画されている:\n%s", body)
	}
}

func TestReadingsIndex_DBエラーは500(t *testing.T) {
	t.Run("Count エラー", func(t *testing.T) {
		repo := ownerReadingsRepo()
		repo.countErr = errors.New("x")
		r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)
		if w := getPath(r, "/devices/1/readings"); w.Code != http.StatusInternalServerError {
			t.Errorf("Count エラー status=%d, want 500", w.Code)
		}
	})
	t.Run("List エラー", func(t *testing.T) {
		repo := ownerReadingsRepo()
		repo.countVal = 5
		repo.listErr = errors.New("x")
		r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)
		if w := getPath(r, "/devices/1/readings"); w.Code != http.StatusInternalServerError {
			t.Errorf("List エラー status=%d, want 500", w.Code)
		}
	})
	t.Run("Summary エラー", func(t *testing.T) {
		repo := ownerReadingsRepo()
		repo.countVal = 5
		repo.summaryErr = errors.New("x")
		r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)
		if w := getPath(r, "/devices/1/readings"); w.Code != http.StatusInternalServerError {
			t.Errorf("Summary エラー status=%d, want 500", w.Code)
		}
	})
}
