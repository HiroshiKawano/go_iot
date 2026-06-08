package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/gin-gonic/gin"

	"github.com/HiroshiKawano/go_iot/internal/view/component"
)

// --- 詳細画面テスト用ヘルパ ---

// showDeviceRepo は所有者(uid=7)とデバイス1(最終通信あり)を備えた fake を返す。
func showDeviceRepo() *fakeDeviceRepo {
	repo := deviceOwnerRepo() // users{7: テスト農場主}
	repo.devices = map[int64]repository.Device{
		1: {
			ID: 1, UserID: 7, Name: "ハウスA温湿度計",
			MacAddress: "AA:BB:CC:DD:EE:01", Location: strPtr("ビニールハウスA"),
			IsActive: true,
			// 05:30 UTC = 14:30 JST。表示は JST 変換されるため期待値は 14:30:00 になる。
			LastCommunicatedAt: timePtr(time.Date(2026, 4, 20, 5, 30, 0, 0, time.UTC)),
		},
	}
	return repo
}

// sensorRow は固定時刻・固定値の計測行を作る (決定的テスト用)。
func sensorRow(deviceID int64, t time.Time, temp, hum float64) repository.SensorReading {
	return repository.SensorReading{
		DeviceID:    deviceID,
		RecordedAt:  t,
		Temperature: temp,
		Humidity:    hum,
	}
}

// dailyAggRow は日次集計1行を作る。SQLite 移行後は集計列が CAST(... AS REAL) で float64、
// ReadingDate は date(recorded_at,'+9 hours') の JST 補正済み "YYYY-MM-DD" 文字列として生成される。
// テストでは date の暦日をそのまま日付文字列に整形して渡す (消費側は二重補正しない)。
func dailyAggRow(date time.Time, tMax, tMin, hMax, hMin float64) repository.ListDailySensorAggregatesRow {
	return repository.ListDailySensorAggregatesRow{
		ReadingDate:    date.Format("2006-01-02"),
		MaxTemperature: tMax,
		MinTemperature: tMin,
		MaxHumidity:    hMax,
		MinHumidity:    hMin,
		SampleCount:    10,
	}
}

// newShowRouterWithUser は認証済み(uid)で詳細系3ルートを配線したルータを返す。
func newShowRouterWithUser(h *DeviceHandler, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	withUser := func(c *gin.Context) { auth.SetUserID(c, userID); c.Next() }
	r.GET("/devices/:device", withUser, h.Show)
	r.GET("/devices/:device/chart", withUser, h.Chart)
	r.DELETE("/devices/:device", withUser, h.Delete)
	return r
}

// requestWithHeaders は任意メソッド・ヘッダでリクエストする (HX-Request 検証用)。
func requestWithHeaders(r http.Handler, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// activeButtonHas は label を含む <button> が active クラスを持つか返す。
func activeButtonHas(html, label string) bool {
	for _, p := range strings.Split(html, "<button")[1:] {
		end := strings.Index(p, "</button>")
		if end < 0 {
			continue
		}
		if seg := p[:end]; strings.Contains(seg, label) {
			return strings.Contains(seg, "active")
		}
	}
	return false
}

// --- 4.1 フラグメント描画ヘルパ ---

func TestRenderComponent_フラグメントを200でHTML描画しレイアウトを含まない(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	renderComponent(c, component.DeviceChartArea(component.DeviceChartAreaView{
		DeviceID: 1, Period: "24h",
		TemperatureSVG: "<svg></svg>", HumiditySVG: "<svg></svg>",
	}))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "period-selector") {
		t.Errorf("フラグメント内容が描画されていない:\n%s", body)
	}
	// フラグメントなのでフルページのレイアウト (<html>/サイドバー) を含まない
	if strings.Contains(body, "<html") {
		t.Errorf("フラグメントに <html> が含まれている (レイアウトが付与されている):\n%s", body)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type=%q, want text/html", ct)
	}
}

// --- 4.2 デバイス詳細表示 (GET /devices/:device) ---

func TestShow_200で情報と既定24hアクティブと最新計測(t *testing.T) {
	repo := showDeviceRepo()
	// 入力は UTC、表示は JST 変換される (05:xx UTC = 14:xx JST)。
	repo.latestReadings = []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 5, 30, 0, 0, time.UTC), 28.50, 65.30),
		sensorRow(1, time.Date(2026, 4, 20, 5, 25, 0, 0, time.UTC), 28.30, 65.50),
	}
	repo.recentReadings = []repository.SensorReading{ // 24h グラフ生データ
		sensorRow(1, time.Date(2026, 4, 20, 5, 0, 0, 0, time.UTC), 27.00, 60.00),
		sensorRow(1, time.Date(2026, 4, 20, 5, 30, 0, 0, time.UTC), 29.00, 66.00),
	}
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		"テスト農場主", // App ヘッダーのユーザー名
		"<h1>デバイス詳細: ハウスA温湿度計</h1>", // 見出し (デバイス名)
		"AA:BB:CC:DD:EE:01",          // MAC
		"2026-04-20 14:30:00",        // 最終通信 JST 絶対表記 (05:30 UTC→14:30 JST)
		`id="device-chart-area"`,     // グラフ領域ラッパー
		"<svg",                       // サーバー生成 SVG
		`id="latest-readings-table"`, // 最新計測テーブル
		"2026-04-20 14:30",           // テーブルの計測日時 (分まで・JST)
		"28.50",                      // 最新計測の温度値
		"65.30",                      // 最新計測の湿度値
	} {
		if !strings.Contains(body, want) {
			t.Errorf("詳細ページに %q が含まれていない", want)
		}
	}
	// 既定 24h がアクティブ
	if !activeButtonHas(body, "24時間") {
		t.Errorf("既定 24時間 がアクティブでない:\n%s", body)
	}
	if strings.Count(body, "period-btn active") != 1 {
		t.Errorf(`"period-btn active" が 1 個でない: %d`, strings.Count(body, "period-btn active"))
	}
}

func TestShow_最終通信と計測日時はJSTに変換して表示_日跨ぎ(t *testing.T) {
	repo := showDeviceRepo()
	// 2026-04-20 20:00:00 UTC = 2026-04-21 05:00:00 JST (日付も跨ぐ)
	repo.devices[1] = repository.Device{
		ID: 1, UserID: 7, Name: "ハウスA温湿度計", MacAddress: "AA:BB:CC:DD:EE:01",
		Location: strPtr("ビニールハウスA"), IsActive: true,
		LastCommunicatedAt: timePtr(time.Date(2026, 4, 20, 20, 0, 0, 0, time.UTC)),
	}
	repo.latestReadings = []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 20, 0, 0, 0, time.UTC), 28.50, 65.30),
	}
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	// 最終通信は JST の翌日 05:00:00 で表示される (UTC のままなら 20:00 になり誤り)
	if !strings.Contains(body, "2026-04-21 05:00:00") {
		t.Errorf("最終通信が JST 変換されていない (期待 2026-04-21 05:00:00):\n%s", body)
	}
	if strings.Contains(body, "2026-04-20 20:00:00") {
		t.Errorf("最終通信が UTC のまま表示されている (JST 未変換):\n%s", body)
	}
	// テーブルの計測日時も JST (翌日 05:00)
	if !strings.Contains(body, "2026-04-21 05:00") {
		t.Errorf("計測日時が JST 変換されていない (期待 2026-04-21 05:00):\n%s", body)
	}
}

func TestShow_未通信と未設定のフォールバック表示(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.devices = map[int64]repository.Device{
		3: {ID: 3, UserID: 7, Name: "新規デバイス", MacAddress: "AA:BB:CC:DD:EE:03", IsActive: false},
		// Location=nil（未設定）・LastCommunicatedAt は Valid=false（未通信）
	}
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/3")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "未通信") {
		t.Error("最終通信なしで「未通信」が表示されていない (R2.5)")
	}
	if !strings.Contains(body, "未設定") {
		t.Error("場所未登録で「未設定」が表示されていない (R2.6)")
	}
	// 停止中の状態記号
	if !strings.Contains(body, "○ 停止中") {
		t.Error("停止中デバイスで「○ 停止中」が表示されていない (R2.3)")
	}
}

func TestShow_period7dで日次集計と7dアクティブ(t *testing.T) {
	repo := showDeviceRepo()
	repo.dailyAggs = []repository.ListDailySensorAggregatesRow{
		dailyAggRow(time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC), 30.0, 18.0, 70.0, 40.0),
		dailyAggRow(time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC), 31.0, 19.0, 72.0, 42.0),
	}
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1?period=7d")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !activeButtonHas(body, "7日間") {
		t.Errorf("7日間 がアクティブでない:\n%s", body)
	}
	// 日次2系列 → 凡例 (最高/最低) と日付ラベルが出る
	for _, want := range []string{"最高", "最低", "04-18", "04-19", "31.0", "18.0"} {
		if !strings.Contains(body, want) {
			t.Errorf("7d グラフに %q が含まれていない", want)
		}
	}
}

func TestShow_計測0件で空グラフとテーブル空メッセージ(t *testing.T) {
	repo := showDeviceRepo() // latest/recent ともに空
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "計測データはまだありません。") {
		t.Error("テーブル空メッセージがない")
	}
	if !strings.Contains(body, "データはまだありません") {
		t.Error("空グラフメッセージがない")
	}
}

func TestShow_非数値IDは400(t *testing.T) {
	repo := showDeviceRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/abc")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400 (非数値ID・R8.1)", w.Code)
	}
}

func TestShow_不正periodは400(t *testing.T) {
	repo := showDeviceRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1?period=99h")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400 (period 不正・R8.2)", w.Code)
	}
}

func TestShow_他ユーザー所有は404で列挙防止(t *testing.T) {
	repo := showDeviceRepo()
	repo.devices = map[int64]repository.Device{
		2: {ID: 2, UserID: 999, Name: "他人のデバイス", MacAddress: "AA:BB:CC:DD:EE:02", IsActive: true},
	}
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/2")
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404 (非所有も存在秘匿・R7.2)", w.Code)
	}
}

func TestShow_不在は404(t *testing.T) {
	repo := showDeviceRepo() // device 999 は未登録
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/999")
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404 (不在)", w.Code)
	}
}

func TestShow_ユーザー取得失敗は500(t *testing.T) {
	repo := showDeviceRepo()
	repo.userErr = errInjected // 認可は GetDevice 経由で通過、レイアウト用 GetUser で失敗
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500 (ユーザー取得失敗)", w.Code)
	}
}

func TestShow_認可のDBエラーは500(t *testing.T) {
	repo := showDeviceRepo()
	repo.getErr = errInjected // GetDevice が ErrNoRows 以外
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500", w.Code)
	}
}

func TestShow_最新計測取得のDBエラーは500(t *testing.T) {
	repo := showDeviceRepo()
	repo.latestErr = errInjected
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500 (最新計測取得失敗)", w.Code)
	}
}

func TestShow_グラフデータ取得のDBエラーは500(t *testing.T) {
	repo := showDeviceRepo()
	repo.recentErr = errInjected // 24h グラフ生データ取得失敗
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500 (グラフデータ取得失敗)", w.Code)
	}
}

// --- 4.3 期間切替フラグメント (GET /devices/:device/chart) ---

func hxGet(r http.Handler, path string) *httptest.ResponseRecorder {
	return requestWithHeaders(r, http.MethodGet, path, map[string]string{"HX-Request": "true"})
}

func TestChart_HXリクエストでグラフ領域フラグメントのみ返す(t *testing.T) {
	repo := showDeviceRepo()
	repo.dailyAggs = []repository.ListDailySensorAggregatesRow{
		dailyAggRow(time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC), 30.0, 18.0, 70.0, 40.0),
		dailyAggRow(time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC), 31.0, 19.0, 72.0, 42.0),
	}
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := hxGet(r, "/devices/1/chart?period=7d")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()

	// フラグメント: フルページのレイアウト (html/サイドバー) を含まない
	if strings.Contains(body, "<html") {
		t.Errorf("フラグメントに <html> が含まれている:\n%s", body)
	}
	if strings.Contains(body, "site-header") || strings.Contains(body, `id="main-content"`) {
		t.Errorf("フラグメントにレイアウト要素 (ヘッダー/メイン) が含まれている:\n%s", body)
	}
	// 要求 period のボタンが active
	if !activeButtonHas(body, "7日間") {
		t.Errorf("7日間 がアクティブでない:\n%s", body)
	}
	// 温度/湿度の2 SVG
	if got := strings.Count(body, "<svg"); got != 2 {
		t.Errorf("<svg> の数 = %d, want 2 (温度/湿度)", got)
	}
	// 最新計測テーブルは期間非連動なので返さない
	if strings.Contains(body, "latest-readings-table") {
		t.Errorf("グラフフラグメントに latest-readings-table が含まれている:\n%s", body)
	}
}

func TestChart_period不正は400(t *testing.T) {
	repo := showDeviceRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := hxGet(r, "/devices/1/chart?period=bad")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400 (period 不正・R8.2)", w.Code)
	}
}

func TestChart_period未指定は400(t *testing.T) {
	repo := showDeviceRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := hxGet(r, "/devices/1/chart")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400 (period 必須)", w.Code)
	}
}

func TestChart_非数値IDは400(t *testing.T) {
	repo := showDeviceRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := hxGet(r, "/devices/abc/chart?period=24h")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400 (非数値ID)", w.Code)
	}
}

func TestChart_他ユーザー所有は404(t *testing.T) {
	repo := showDeviceRepo()
	repo.devices = map[int64]repository.Device{
		2: {ID: 2, UserID: 999, Name: "他人", MacAddress: "AA:BB:CC:DD:EE:02", IsActive: true},
	}
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := hxGet(r, "/devices/2/chart?period=24h")
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404 (非所有・列挙防止)", w.Code)
	}
}

func TestChart_24hは生データで取得(t *testing.T) {
	repo := showDeviceRepo()
	// 入力は UTC、X 軸ラベルは JST 変換される (04:00 UTC→13:00 JST, 05:00 UTC→14:00 JST)。
	repo.recentReadings = []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 4, 0, 0, 0, time.UTC), 27.00, 60.00),
		sensorRow(1, time.Date(2026, 4, 20, 5, 0, 0, 0, time.UTC), 29.00, 66.00),
	}
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := hxGet(r, "/devices/1/chart?period=24h")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	if !activeButtonHas(body, "24時間") {
		t.Errorf("24時間 がアクティブでない:\n%s", body)
	}
	if !strings.Contains(body, "13:00") || !strings.Contains(body, "14:00") {
		t.Errorf("24h 生データの時刻ラベル(JST)が無い:\n%s", body)
	}
}

func TestChart_グラフデータ取得のDBエラーは500(t *testing.T) {
	repo := showDeviceRepo()
	repo.dailyErr = errInjected
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := hxGet(r, "/devices/1/chart?period=30d")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500", w.Code)
	}
}

// --- 4.4 削除 (DELETE /devices/:device) ---

func TestDelete_HTMXは200とHX_Redirectで論理削除(t *testing.T) {
	repo := showDeviceRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := requestWithHeaders(r, http.MethodDelete, "/devices/1", map[string]string{"HX-Request": "true"})

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (HTMX)", w.Code)
	}
	if loc := w.Header().Get("HX-Redirect"); loc != "/dashboard" {
		t.Errorf("HX-Redirect=%q, want /dashboard", loc)
	}
	if !repo.softDeleted {
		t.Error("SoftDeleteDevice が呼ばれていない")
	}
	if repo.softDeleteID != 1 {
		t.Errorf("論理削除対象 id=%d, want 1", repo.softDeleteID)
	}
}

func TestDelete_非HTMXは303でダッシュボードへ(t *testing.T) {
	repo := showDeviceRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	// HX-Request ヘッダ無し = フォーム (_method=delete) 経路相当
	w := requestWithHeaders(r, http.MethodDelete, "/devices/1", nil)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303 (非HTMX)", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/dashboard" {
		t.Errorf("Location=%q, want /dashboard", loc)
	}
	if !repo.softDeleted {
		t.Error("SoftDeleteDevice が呼ばれていない")
	}
}

func TestDelete_非数値IDは400で削除しない(t *testing.T) {
	repo := showDeviceRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := requestWithHeaders(r, http.MethodDelete, "/devices/abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", w.Code)
	}
	if repo.softDeleted {
		t.Error("非数値IDで SoftDeleteDevice を呼んではいけない")
	}
}

func TestDelete_他ユーザー所有は403で削除しない(t *testing.T) {
	repo := showDeviceRepo()
	repo.devices = map[int64]repository.Device{
		2: {ID: 2, UserID: 999, Name: "他人", MacAddress: "AA:BB:CC:DD:EE:02", IsActive: true},
	}
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := requestWithHeaders(r, http.MethodDelete, "/devices/2", map[string]string{"HX-Request": "true"})
	if w.Code != http.StatusForbidden {
		t.Errorf("status=%d, want 403 (非所有・BOLA)", w.Code)
	}
	if repo.softDeleted {
		t.Error("非所有で SoftDeleteDevice を呼んではいけない (BOLA)")
	}
}

func TestDelete_不在は404(t *testing.T) {
	repo := showDeviceRepo() // device 999 未登録
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := requestWithHeaders(r, http.MethodDelete, "/devices/999", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404 (不在)", w.Code)
	}
	if repo.softDeleted {
		t.Error("不在で SoftDeleteDevice を呼んではいけない")
	}
}

func TestDelete_論理削除のDBエラーは500(t *testing.T) {
	repo := showDeviceRepo()
	repo.softDeleteErr = errInjected
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := requestWithHeaders(r, http.MethodDelete, "/devices/1", map[string]string{"HX-Request": "true"})
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500", w.Code)
	}
}

// --- 日次/区間集計 max/min の明示型化 (silent 平坦化封じ・R5.3) ---

// TestAggregateToFloat_期待型float64は受理 は、SQLite 移行後に集計列が CAST(... AS REAL) で
// float64 生成されることを前提に、float64 をそのまま受理して error を返さないことを検証する。
func TestAggregateToFloat_期待型float64は受理(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{"正の値", 28.50, 28.50},
		{"別の正値", 30.0, 30.0},
		{"ゼロ値", 0, 0},
		{"負の値", -5.5, -5.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := aggregateToFloat(tt.in)
			if err != nil {
				t.Fatalf("aggregateToFloat(%v) で予期しないエラー: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("aggregateToFloat(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestAggregateToFloat_未知型は0へ平坦化せず明示エラー は、想定外の型を渡したとき
// silent に 0 へフォールバックせず error を返すことを検証する (R5.3 の最重要回帰防止)。
// 旧実装は nil/文字列を 0 に落としてグラフを黙って平坦化していた。
func TestAggregateToFloat_未知型は0へ平坦化せず明示エラー(t *testing.T) {
	unknowns := []struct {
		name string
		in   interface{}
	}{
		{"nil (NULL 集計)", nil},
		{"文字列", "30.0"},
		{"整数", int64(30)},
		{"bool", true},
	}
	for _, tt := range unknowns {
		t.Run(tt.name, func(t *testing.T) {
			got, err := aggregateToFloat(tt.in)
			if err == nil {
				t.Fatalf("aggregateToFloat(%v): error を期待したが nil (silent 平坦化してはならない)", tt.in)
			}
			if got != 0 {
				t.Errorf("エラー時の戻り値 = %v, want 0", got)
			}
		})
	}
}
