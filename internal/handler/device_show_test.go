package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
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
			LastCommunicatedAt: pgconv.Timestamptz(time.Date(2026, 4, 20, 5, 30, 0, 0, time.UTC)),
		},
	}
	return repo
}

// sensorRow は固定時刻・固定値の計測行を作る (決定的テスト用)。
func sensorRow(deviceID int64, t time.Time, temp, hum float64) repository.SensorReading {
	return repository.SensorReading{
		DeviceID:    deviceID,
		RecordedAt:  pgconv.Timestamptz(t),
		Temperature: pgconv.Numeric2(temp),
		Humidity:    pgconv.Numeric2(hum),
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
		DeviceID: 1, Period: "24h", HasData: true,
		TemperatureOptionJSON: "{}", HumidityOptionJSON: "{}",
		TemperatureUnit: "℃", HumidityUnit: "%",
		TemperatureColor: "#e8590c", HumidityColor: "#1971c2",
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
		`id="device-chart-area"`,            // グラフ領域ラッパー
		`id="temperature-chart"`,            // 温度 ECharts コンテナ
		`id="humidity-chart"`,               // 湿度 ECharts コンテナ
		`id="temperature-chart-option"`,     // 温度 option script
		`id="humidity-chart-option"`,        // 湿度 option script
		"data-echarts",                      // ECharts 初期化対象マーカー
		`id="latest-readings-table"`,        // 最新計測テーブル
		"2026-04-20 14:30",                  // テーブルの計測日時 (分まで・JST)
		"28.50",                             // 最新計測の温度値
		"65.30",                             // 最新計測の湿度値
	} {
		if !strings.Contains(body, want) {
			t.Errorf("詳細ページに %q が含まれていない", want)
		}
	}
	// コンテナ id は DOM 内で一意 (温/湿それぞれ 1 個。option script の id とは別物)
	if got := strings.Count(body, `id="temperature-chart"`); got != 1 {
		t.Errorf(`id="temperature-chart" が %d 個 (want 1・一意)`, got)
	}
	if got := strings.Count(body, `id="humidity-chart"`); got != 1 {
		t.Errorf(`id="humidity-chart" が %d 個 (want 1・一意)`, got)
	}
	// option script は温/湿の 2 本
	if got := strings.Count(body, `type="application/json"`); got != 2 {
		t.Errorf("option script の数 = %d, want 2 (温度/湿度)", got)
	}
	// 旧 SVG 描画は撤去済み (グラフは ECharts コンテナへ移行)
	if strings.Contains(body, "<polyline") {
		t.Errorf("旧 SVG 折れ線 (<polyline) が残存している:\n%s", body)
	}
	// 既定 24h がアクティブ
	if !activeButtonHas(body, "24時間") {
		t.Errorf("既定 24時間 がアクティブでない:\n%s", body)
	}
	if strings.Count(body, "period-btn active") != 1 {
		t.Errorf(`"period-btn active" が 1 個でない: %d`, strings.Count(body, "period-btn active"))
	}
}

// TestShow_ECharts配線がフルページに揃う は、デバイス詳細フルページに ECharts 移行の配線一式
// (self-host アセット読込 + グローバル init/dispose/connect + コンテナ + option script) が
// 揃うことをエンドツーエンドで検証する回帰ガード (R5.3 self-host・R6.1/6.2 init/dispose/connect)。
func TestShow_ECharts配線がフルページに揃う(t *testing.T) {
	repo := showDeviceRepo()
	repo.recentReadings = []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 4, 0, 0, 0, time.UTC), 27.00, 60.00),
		sensorRow(1, time.Date(2026, 4, 20, 5, 0, 0, 0, time.UTC), 29.00, 66.00),
	}
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()

	for _, want := range []string{
		"/static/js/echarts.min.js", // self-host アセットを <head> で読込 (R5)
		"echarts.init",              // 描画インスタンス生成
		"getInstanceByDom",          // 既存検出 → dispose (再描画・リーク防止 R6)
		"echarts.connect",           // 温湿度連動 (R3.3)
		"htmx:beforeSwap",           // swap 前破棄 (R6.2)
		"htmx:afterSwap",            // swap 後再初期化
		`id="temperature-chart"`,    // 温度コンテナ
		`id="humidity-chart"`,       // 湿度コンテナ
		`id="temperature-chart-option"`, // 温度 option script
		`id="humidity-chart-option"`,    // 湿度 option script
	} {
		if !strings.Contains(body, want) {
			t.Errorf("ECharts 配線 %q がフルページに無い", want)
		}
	}
}

func TestShow_最終通信と計測日時はJSTに変換して表示_日跨ぎ(t *testing.T) {
	repo := showDeviceRepo()
	// 2026-04-20 20:00:00 UTC = 2026-04-21 05:00:00 JST (日付も跨ぐ)
	repo.devices[1] = repository.Device{
		ID: 1, UserID: 7, Name: "ハウスA温湿度計", MacAddress: "AA:BB:CC:DD:EE:01",
		Location: strPtr("ビニールハウスA"), IsActive: true,
		LastCommunicatedAt: pgconv.Timestamptz(time.Date(2026, 4, 20, 20, 0, 0, 0, time.UTC)),
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

func TestShow_情報パネルに認識名の所在地を表示(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.devices = map[int64]repository.Device{
		1: {
			ID: 1, UserID: 7, Name: "ハウスA温湿度計", MacAddress: "AA:BB:CC:DD:EE:01",
			Location: strPtr("旧自由入力ハウスA"), // 移行元として残置・表示しない
			Locality: strPtr("佐敷町"),          // 表示は構造化 locality を認識名で
			IsActive: true,
		},
	}
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	// 所在地は認識名 (合併=「旧町村（現市町村）」) で表示する (R6.1)
	if !strings.Contains(body, "佐敷（南城市）") {
		t.Errorf("情報パネルに認識名の所在地「佐敷（南城市）」が表示されていない:\n%s", body)
	}
	// 自由入力 location は表示に使わない (locality へ切替済)
	if strings.Contains(body, "旧自由入力ハウスA") {
		t.Errorf("自由入力 location が情報パネルに表示されている (locality へ切替のはず):\n%s", body)
	}
}

func TestShow_period7dは生データ折れ線と7dアクティブ(t *testing.T) {
	repo := showDeviceRepo()
	// 7d は 24h と同じ生データ折れ線。複数日のため X ラベルは "M/D HH:MM" (日付併記)。
	repo.recentReadings = []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 18, 5, 0, 0, 0, time.UTC), 27.00, 60.00), // 14:00 JST
		sensorRow(1, time.Date(2026, 4, 19, 5, 0, 0, 0, time.UTC), 29.00, 66.00),
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
	// 生データ1系列 → 日付付き時刻ラベル "M/D HH:MM" が option JSON の xAxis に出る。
	if !strings.Contains(body, "4/18 14:00") || !strings.Contains(body, "4/19 14:00") {
		t.Errorf("7d 生データの日付時刻ラベル(JST)が無い:\n%s", body)
	}
}

func TestShow_period3dは生データ折れ線と3dアクティブ(t *testing.T) {
	repo := showDeviceRepo()
	repo.recentReadings = []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 18, 5, 0, 0, 0, time.UTC), 27.00, 60.00), // 14:00 JST
		sensorRow(1, time.Date(2026, 4, 19, 5, 0, 0, 0, time.UTC), 29.00, 66.00),
	}
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1?period=3d")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !activeButtonHas(body, "3日間") {
		t.Errorf("3日間 がアクティブでない:\n%s", body)
	}
	// 3d は 24h と同じ生データ折れ線。日付付き時刻ラベルが option JSON の xAxis に出る。
	if !strings.Contains(body, "4/18 14:00") || !strings.Contains(body, "4/19 14:00") {
		t.Errorf("3d 生データの日付時刻ラベル(JST)が無い:\n%s", body)
	}
}

// periodSince の純粋関数を期間別に直接検証する。fake repo は取得開始時刻 (params の RecordedAt) を
// 破棄するため、ハンドラ経由テストでは 3d→-3日 の写像ミスを捕捉できない。ここで写像そのものを固定する。
func TestPeriodSince_期間ごとの取得開始時刻(t *testing.T) {
	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		period string
		want   time.Time
	}{
		{"24h", now.Add(-24 * time.Hour)},
		{"3d", now.AddDate(0, 0, -3)},
		{"7d", now.AddDate(0, 0, -7)},
		{"30d", now.AddDate(0, 0, -30)},
		{"", now.Add(-24 * time.Hour)},    // 既定 (空) は 24h
		{"bad", now.Add(-24 * time.Hour)}, // 不正値も既定 24h にフォールバック
	}
	for _, c := range cases {
		if got := periodSince(c.period, now); !got.Equal(c.want) {
			t.Errorf("periodSince(%q) = %v, want %v", c.period, got, c.want)
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
	// 空データでは option script を出さない (ECharts 初期化対象がない)。
	// 注: App レイアウトのグローバル初期化スクリプトに [data-echarts] セレクタ文字列が
	// 常駐するため、コンテナ有無は ECharts マウント div そのものの有無で判定する。
	if strings.Contains(body, `type="application/json"`) {
		t.Errorf("空データなのに option script が出力されている:\n%s", body)
	}
	if strings.Contains(body, `<div id="temperature-chart"`) || strings.Contains(body, `<div id="humidity-chart"`) {
		t.Errorf("空データなのに ECharts コンテナ div が出力されている:\n%s", body)
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
	repo.recentReadings = []repository.SensorReading{ // 7d は生データ折れ線経路
		sensorRow(1, time.Date(2026, 4, 18, 5, 0, 0, 0, time.UTC), 27.00, 60.00),
		sensorRow(1, time.Date(2026, 4, 19, 5, 0, 0, 0, time.UTC), 29.00, 66.00),
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
	// 温度/湿度の option script は 2 本だけ (フラグメントはレイアウト非包含のため他に json script なし)
	if got := strings.Count(body, `type="application/json"`); got != 2 {
		t.Errorf("option script の数 = %d, want 2 (温度/湿度)", got)
	}
	for _, want := range []string{`id="temperature-chart-option"`, `id="humidity-chart-option"`, "data-echarts"} {
		if !strings.Contains(body, want) {
			t.Errorf("グラフフラグメントに %q が含まれていない:\n%s", want, body)
		}
	}
	// 情報パネル・最新計測テーブルは期間非連動なので返さない
	if strings.Contains(body, "latest-readings-table") {
		t.Errorf("グラフフラグメントに latest-readings-table が含まれている:\n%s", body)
	}
	if strings.Contains(body, "device-info") {
		t.Errorf("グラフフラグメントに情報パネル(device-info)が含まれている:\n%s", body)
	}
	// echarts.min.js は <head> で1回だけ読込む。期間切替フラグメントには出さない=再 DL させない (R5.3)
	if strings.Contains(body, "echarts.min.js") {
		t.Errorf("期間切替フラグメントに echarts.min.js が含まれている (再 DL させてしまう・R5.3):\n%s", body)
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

func TestChart_3dは生データで取得(t *testing.T) {
	repo := showDeviceRepo()
	repo.recentReadings = []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 18, 5, 0, 0, 0, time.UTC), 27.00, 60.00), // 14:00 JST
		sensorRow(1, time.Date(2026, 4, 19, 5, 0, 0, 0, time.UTC), 29.00, 66.00),
	}
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	// oneof バインディングが 3d を受理し (400 にならず)、24h と同じ生データ折れ線経路を通る
	w := hxGet(r, "/devices/1/chart?period=3d")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (3d は許容値・R8.2)", w.Code)
	}
	body := w.Body.String()
	if !activeButtonHas(body, "3日間") {
		t.Errorf("3日間 がアクティブでない:\n%s", body)
	}
	// 生データ折れ線 → 日付付き時刻ラベルが option JSON の xAxis に出る。
	if !strings.Contains(body, "4/18 14:00") {
		t.Errorf("3d 生データの日付時刻ラベルが無い:\n%s", body)
	}
}

func TestChart_30dは生データ折れ線で取得(t *testing.T) {
	repo := showDeviceRepo()
	// 30d も 24h/3d/7d と同じ生データ折れ線 (単一系列)。日付付き時刻ラベル + Y軸「最高/最低」見出し。
	repo.recentReadings = []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 18, 5, 0, 0, 0, time.UTC), 27.00, 60.00), // 14:00 JST
		sensorRow(1, time.Date(2026, 5, 18, 5, 0, 0, 0, time.UTC), 29.00, 66.00),
	}
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := hxGet(r, "/devices/1/chart?period=30d")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	if !activeButtonHas(body, "30日間") {
		t.Errorf("30日間 がアクティブでない:\n%s", body)
	}
	// 生データ単一系列 → 日付付き時刻ラベルが option JSON の xAxis に出る。
	// (最高/最低は ECharts の markPoint(type max/min) でクライアント描画されるため
	//  サーバー JSON には日本語見出しは含まれない)
	if !strings.Contains(body, "4/18 14:00") {
		t.Errorf("30d 生データの日付時刻ラベルが無い:\n%s", body)
	}
	// markPoint(最高/最低) は option に含まれる
	for _, want := range []string{`"type":"max"`, `"type":"min"`} {
		if !strings.Contains(body, want) {
			t.Errorf("30d option に markPoint %q が無い:\n%s", want, body)
		}
	}
}

func TestChart_グラフデータ取得のDBエラーは500(t *testing.T) {
	repo := showDeviceRepo()
	repo.recentErr = errInjected // 全期間が生データ取得 (ListRecentSensorReadings) になったため
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

// --- 日次集計 max/min の interface{} 安全変換 ---

func TestAggregateToFloat_型ごとの安全変換(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want float64
	}{
		{"pgtype.Numeric (本番 pgx の uncast numeric)", pgconv.Numeric2(28.50), 28.50},
		{"float64 (防御的)", float64(30.0), 30.0},
		{"nil (NULL 集計)", nil, 0},
		{"想定外の型", "30.0", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := aggregateToFloat(tt.in); got != tt.want {
				t.Errorf("aggregateToFloat(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
