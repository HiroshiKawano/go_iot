package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
)

// authedDashboardRepo はログイン可能なユーザー (id=7) を持つ fakeAuthRepo を返す。
// 各テストは devices/readings/alerts/各 err を上書きしてダッシュボードの入力を構成する。
func authedDashboardRepo() *fakeAuthRepo {
	u := userWithPassword(7, "テスト農場主", "user@example.com", "password123")
	return &fakeAuthRepo{
		byEmail: map[string]repository.User{"user@example.com": u},
		byID:    map[int64]repository.User{7: u},
	}
}

// getDashboard はログイン→セッション cookie 付きで GET /dashboard した結果を返す。
func getDashboard(t *testing.T, repo *fakeAuthRepo) *httptest.ResponseRecorder {
	t.Helper()
	app := newAuthApp(repo)
	login := postForm(app, "/login", url.Values{"email": {"user@example.com"}, "password": {"password123"}})
	if login.Code != http.StatusSeeOther {
		t.Fatalf("前提のログインに失敗: status=%d body=%s", login.Code, login.Body.String())
	}
	return getWithCookies(app, "/dashboard", login.Result().Cookies())
}

// validReading は計測ありデバイス用の最新計測を返す (温度28.50 / 湿度65.30)。
func validReading(deviceID int64) repository.SensorReading {
	return repository.SensorReading{
		DeviceID:    deviceID,
		Temperature: pgconv.Numeric2(28.50),
		Humidity:    pgconv.Numeric2(65.30),
	}
}

// validComm は通信実績ありの最終通信時刻 (固定・相対時間は非アサート)。
func validComm() pgtype.Timestamptz {
	return pgconv.Timestamptz(time.Date(2026, 6, 7, 11, 0, 0, 0, time.UTC))
}

// --- 5.1 成功系 ---

func TestDashboard_認証済みでデバイスとアラートを描画(t *testing.T) {
	repo := authedDashboardRepo()
	repo.devices = []repository.Device{
		{ID: 1, UserID: 7, Name: "ハウスA温湿度計", Location: strPtr("ビニールハウスA"), IsActive: true, LastCommunicatedAt: validComm()},
	}
	repo.readings = map[int64]repository.SensorReading{1: validReading(1)}
	repo.alerts = []repository.ListUnnotifiedAlertHistoriesWithDeviceRow{
		{ID: 100, DeviceID: 1, DeviceName: "ハウスA温湿度計", Metric: "temperature", Operator: ">", Threshold: pgconv.Numeric2(35.00), ActualValue: pgconv.Numeric2(38.50)},
	}

	w := getDashboard(t, repo)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		"ハウスA温湿度計", // デバイス名
		"● 稼働中",    // 稼働状態
		"28.50℃",   // 温度
		"65.30%",   // 湿度
		"ハウスA温湿度計: 温度が35℃を超えました（38.50℃）", // アラート文言
		`id="device-grid"`,
		`id="unhandled-alert-banner"`,
		`id="device-card-1"`,
		`href="/devices/1"`,      // 詳細遷移
		`href="/devices/create"`, // デバイス登録遷移
	} {
		if !strings.Contains(body, want) {
			t.Errorf("ダッシュボードHTMLに %q が含まれていない", want)
		}
	}
}

// --- 5.2 データ欠損・0件・取得失敗 ---

func TestDashboard_デバイス0件は空メッセージで正常表示(t *testing.T) {
	repo := authedDashboardRepo() // devices/alerts なし

	w := getDashboard(t, repo)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `id="device-grid"`) {
		t.Error("device-grid ラッパーが 0 件でも常設されていない")
	}
	if !strings.Contains(body, "登録されたデバイスはありません。上の「デバイス登録」ボタンから追加してください。") {
		t.Error("デバイス 0 件の空メッセージが表示されていない")
	}
}

func TestDashboard_アラート0件は空メッセージで正常表示(t *testing.T) {
	repo := authedDashboardRepo()
	repo.devices = []repository.Device{{ID: 1, Name: "D", IsActive: true, LastCommunicatedAt: validComm()}}
	repo.readings = map[int64]repository.SensorReading{1: validReading(1)}
	// alerts なし

	w := getDashboard(t, repo)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "未対応のアラートはありません。") {
		t.Error("アラート 0 件の空メッセージが表示されていない")
	}
}

func TestDashboard_計測未受信は温湿度がダッシュ表記(t *testing.T) {
	repo := authedDashboardRepo()
	repo.devices = []repository.Device{{ID: 1, Name: "計測待ち", IsActive: true, LastCommunicatedAt: validComm()}}
	// readings 未設定 → mock は pgx.ErrNoRows を返す = 未受信 (正常)

	w := getDashboard(t, repo)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (未受信は正常系)", w.Code)
	}
	// 値セル限定でアサート (aria-label="メニュー" 等の「ー」誤検出を避ける)
	if !strings.Contains(w.Body.String(), `<div class="value">ー</div>`) {
		t.Error("計測未受信で温湿度欄が「ー」になっていない")
	}
}

func TestDashboard_通信実績なしは該当文言(t *testing.T) {
	repo := authedDashboardRepo()
	repo.devices = []repository.Device{{ID: 1, Name: "未通信", IsActive: true, LastCommunicatedAt: pgtype.Timestamptz{}}} // Valid=false
	repo.readings = map[int64]repository.SensorReading{1: validReading(1)}

	w := getDashboard(t, repo)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "通信実績なし") {
		t.Error("通信実績なしの文言が表示されていない")
	}
}

func TestDashboard_デバイス一覧取得失敗は500(t *testing.T) {
	repo := authedDashboardRepo()
	repo.devicesErr = errInjected

	w := getDashboard(t, repo)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500", w.Code)
	}
}

func TestDashboard_アラート一覧取得失敗は500(t *testing.T) {
	repo := authedDashboardRepo()
	repo.devices = []repository.Device{{ID: 1, Name: "D", IsActive: true, LastCommunicatedAt: validComm()}}
	repo.readings = map[int64]repository.SensorReading{1: validReading(1)}
	repo.alertsErr = errInjected

	w := getDashboard(t, repo)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500", w.Code)
	}
}

func TestDashboard_最新計測の想定外エラーは500(t *testing.T) {
	repo := authedDashboardRepo()
	repo.devices = []repository.Device{{ID: 1, Name: "D", IsActive: true, LastCommunicatedAt: validComm()}}
	repo.readingErrs = map[int64]error{1: errInjected} // ErrNoRows 以外のエラー

	w := getDashboard(t, repo)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500 (ErrNoRows 以外の計測エラー)", w.Code)
	}
}

// R1.3: 本人 ID が各クエリへ伝播し、他ユーザーのデータを引かないことを保証する。
// ログインユーザー (id=7) が3クエリへ渡る uid / per-device の deviceID を fake が記録して検証する。
func TestDashboard_本人IDを各クエリへ伝播する(t *testing.T) {
	repo := authedDashboardRepo() // uid=7 でログイン
	repo.devices = []repository.Device{{ID: 42, Name: "D", IsActive: true, LastCommunicatedAt: validComm()}}
	repo.readings = map[int64]repository.SensorReading{42: validReading(42)}

	w := getDashboard(t, repo)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	if repo.gotDevicesUserID != 7 {
		t.Errorf("ListDevicesByUser へ uid=%d を渡した, want 7 (本人スコープ)", repo.gotDevicesUserID)
	}
	if repo.gotAlertsUserID != 7 {
		t.Errorf("ListUnnotifiedAlertHistoriesWithDevice へ uid=%d を渡した, want 7 (本人スコープ)", repo.gotAlertsUserID)
	}
	if len(repo.gotReadingDeviceIDs) != 1 || repo.gotReadingDeviceIDs[0] != 42 {
		t.Errorf("GetLatestSensorReading へ deviceID=%v を渡した, want [42]", repo.gotReadingDeviceIDs)
	}
}

// R6.3: 初期描画のみ。自動ポーリング/部分更新の HTMX 属性を一切出力しない。
func TestDashboard_自動ポーリング属性を持たない(t *testing.T) {
	repo := authedDashboardRepo()
	repo.devices = []repository.Device{{ID: 1, Name: "D", IsActive: true, LastCommunicatedAt: validComm()}}
	repo.readings = map[int64]repository.SensorReading{1: validReading(1)}
	repo.alerts = []repository.ListUnnotifiedAlertHistoriesWithDeviceRow{
		{ID: 1, DeviceID: 1, DeviceName: "D", Metric: "temperature", Operator: ">", Threshold: pgconv.Numeric2(35.00), ActualValue: pgconv.Numeric2(38.50)},
	}

	body := getDashboard(t, repo).Body.String()
	for _, banned := range []string{"hx-trigger", "hx-get", "hx-post", "hx-swap", "hx-target"} {
		if strings.Contains(body, banned) {
			t.Errorf("dashboard に %q が含まれている (初期描画のみ・自動更新/OOB なしのはず)", banned)
		}
	}
}

// R6.1: 取得失敗の 500 本文は利用者向けの定文で、内部エラー詳細を漏らさない。
func TestDashboard_取得失敗の500本文は機密を漏らさない(t *testing.T) {
	repo := authedDashboardRepo()
	repo.devicesErr = errInjected // 内部エラー "injected db error"

	w := getDashboard(t, repo)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "エラーが発生しました") {
		t.Errorf("500 本文が利用者向け定文でない: %q", body)
	}
	if strings.Contains(body, "injected db error") {
		t.Error("500 本文に内部エラー詳細が漏洩している")
	}
}
