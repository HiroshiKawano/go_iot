package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// fakeDeviceRepo は DeviceRepo の手書きモック (DB 非依存)。
// GetDevice/GetDeviceByMacAddress は map で引き、未登録は pgx.ErrNoRows を返す。
// Create/Update は呼び出し記録 (Called/last*) と戻り値・エラー注入に対応する。
type fakeDeviceRepo struct {
	users   map[int64]repository.User
	userErr error

	devices map[int64]repository.Device // GetDevice 用 (id → device)
	getErr  error

	byMac  map[string]repository.Device // GetDeviceByMacAddress 用
	macErr error

	createResult repository.Device
	createErr    error
	createCalled bool
	lastCreate   repository.CreateDeviceParams

	updateResult repository.Device
	updateErr    error
	updateCalled bool
	lastUpdate   repository.UpdateDeviceParams

	// --- デバイス詳細画面 (device-detail) 用 ---
	latestReadings []repository.SensorReading // ListLatestSensorReadings 戻り値
	latestErr      error
	recentReadings []repository.SensorReading // ListRecentSensorReadings 戻り値 (24h 生データ)
	recentErr      error
	dailyAggs      []repository.ListDailySensorAggregatesRow // ListDailySensorAggregates 戻り値
	dailyErr       error
	softDeleteErr  error
	softDeleteID   int64 // 最後に論理削除を要求された id
	softDeleted    bool  // SoftDeleteDevice 呼び出し記録
}

func (f *fakeDeviceRepo) ListLatestSensorReadings(_ context.Context, _ int64) ([]repository.SensorReading, error) {
	return f.latestReadings, f.latestErr
}

func (f *fakeDeviceRepo) ListRecentSensorReadings(_ context.Context, _ repository.ListRecentSensorReadingsParams) ([]repository.SensorReading, error) {
	return f.recentReadings, f.recentErr
}

func (f *fakeDeviceRepo) ListDailySensorAggregates(_ context.Context, _ repository.ListDailySensorAggregatesParams) ([]repository.ListDailySensorAggregatesRow, error) {
	return f.dailyAggs, f.dailyErr
}

func (f *fakeDeviceRepo) SoftDeleteDevice(_ context.Context, id int64) error {
	f.softDeleted = true
	f.softDeleteID = id
	return f.softDeleteErr
}

func (f *fakeDeviceRepo) GetUser(_ context.Context, id int64) (repository.User, error) {
	if f.userErr != nil {
		return repository.User{}, f.userErr
	}
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return repository.User{}, pgx.ErrNoRows
}

func (f *fakeDeviceRepo) GetDevice(_ context.Context, id int64) (repository.Device, error) {
	if f.getErr != nil {
		return repository.Device{}, f.getErr
	}
	if d, ok := f.devices[id]; ok {
		return d, nil
	}
	return repository.Device{}, pgx.ErrNoRows
}

func (f *fakeDeviceRepo) GetDeviceByMacAddress(_ context.Context, mac string) (repository.Device, error) {
	if f.macErr != nil {
		return repository.Device{}, f.macErr
	}
	if d, ok := f.byMac[mac]; ok {
		return d, nil
	}
	return repository.Device{}, pgx.ErrNoRows
}

func (f *fakeDeviceRepo) CreateDevice(_ context.Context, arg repository.CreateDeviceParams) (repository.Device, error) {
	f.createCalled = true
	f.lastCreate = arg
	if f.createErr != nil {
		return repository.Device{}, f.createErr
	}
	return f.createResult, nil
}

func (f *fakeDeviceRepo) UpdateDevice(_ context.Context, arg repository.UpdateDeviceParams) (repository.Device, error) {
	f.updateCalled = true
	f.lastUpdate = arg
	if f.updateErr != nil {
		return repository.Device{}, f.updateErr
	}
	return f.updateResult, nil
}

// コンパイル時に DeviceRepo を満たすことを保証する。
var _ DeviceRepo = (*fakeDeviceRepo)(nil)

// deviceOwner は所有者 (uid) を持つ標準テストユーザーを備えた fake を返す。
func deviceOwnerRepo() *fakeDeviceRepo {
	return &fakeDeviceRepo{
		users: map[int64]repository.User{7: {ID: 7, Name: "テスト農場主"}},
	}
}

// newDeviceRouterWithUser は auth.SetUserID 済み (認証済み) のルータを返す。
// RequireAuth/CSRF の実機構は通さず、ハンドラの認可・描画分岐だけを単体検証する (§6.1)。
func newDeviceRouterWithUser(h *DeviceHandler, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	withUser := func(c *gin.Context) { auth.SetUserID(c, userID); c.Next() }
	r.GET("/devices/create", withUser, h.ShowCreateForm)
	r.POST("/devices", withUser, h.Create)
	r.GET("/devices/:device/edit", withUser, h.ShowEditForm)
	r.PUT("/devices/:device", withUser, h.Update)
	return r
}

func getPath(r http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// formRequest は method (POST/PUT) で form-urlencoded を送る。
func formRequest(r http.Handler, method, path string, vals url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// validDeviceVals は登録/更新で通過する有効なフォーム値を返す。
func validDeviceVals() url.Values {
	return url.Values{
		"name":        {"温室センサー"},
		"mac_address": {"aa:bb:cc:dd:ee:ff"}, // 小文字 → 正規化で大文字化される
		"locality":    {"国頭村"},               // 沖縄の地域 (domain.Locality・未合併=市町村名)
		"is_active":   {"1"},
	}
}

// --- device-context-nav 2.3: 登録/編集はナビ文脈に入れない ---

// assertSidebarNoNavContext は登録/編集相当のサイドバーが、デバイス文脈リンクを描画せず
// いずれのメニュー項目も active でない (R1.3/2.6) ことを検証する。編集は URL に device id を
// 持つが、確定済みのユーザー判断として文脈に入れない。
func assertSidebarNoNavContext(t *testing.T, body string) {
	t.Helper()
	if strings.Contains(body, "📟 デバイス詳細") || strings.Contains(body, "📈 センサーデータ履歴") {
		t.Errorf("文脈リンク (デバイス詳細/センサーデータ履歴) が描画されている (R1.3 違反):\n%s", body)
	}
	for _, item := range []string{"🏠 ダッシュボード", "🔔 アラートルール", "🕐 アラート履歴"} {
		if strings.Contains(body, `class="active">`+item) {
			t.Errorf("メニュー項目 %q が active になっている (登録/編集は非 active・R2.6)", item)
		}
	}
}

// TestShowCreateForm_サイドバーはactiveも文脈リンクも持たない は、デバイス登録画面の
// サイドバーがゼロ値ナビ文脈で描画され、active マークも文脈リンクも持たないことを固定する (R1.3/2.6)。
func TestShowCreateForm_サイドバーはactiveも文脈リンクも持たない(t *testing.T) {
	repo := deviceOwnerRepo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/create")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	assertSidebarNoNavContext(t, w.Body.String())
}

// TestShowEditForm_URLにIDがあってもサイドバーは文脈に入れない は、編集画面が URL に device id を
// 持っていてもサイドバーの文脈リンクを出さず active も付けないことを回帰防止として固定する
// (R1.3 boundary/2.6・確定済みのユーザー判断)。
func TestShowEditForm_URLにIDがあってもサイドバーは文脈に入れない(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.devices = map[int64]repository.Device{
		1: {ID: 1, UserID: 7, Name: "ハウスA温湿度計", MacAddress: "AA:BB:CC:DD:EE:01", IsActive: true},
	}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1/edit")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	assertSidebarNoNavContext(t, w.Body.String())
}

// --- 3.1 デバイス登録フォーム表示 ---

func TestShowCreateForm_空フォームと稼働中初期とCSRF(t *testing.T) {
	repo := deviceOwnerRepo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/create")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		"テスト農場主",                                         // App レイアウトのユーザー名
		"<h1>デバイス登録</h1>",                                // 見出し
		`id="device-form"`,                               // 共有フォーム
		`action="/devices"`,                              // 送信先 (POST)
		`name="gorilla.csrf.Token"`,                      // CSRF 隠しフィールド
		`name="name" value=""`,                           // 空フォーム (デバイス名未入力)
		`value="1" checked`,                              // ステータス初期=稼働中
		`href="/dashboard"`,                              // キャンセル導線
		`<select name="locality" class="js-tom-select">`, // 設置場所=単一の検索可能 地域 select
		`<option value="">選択してください</option>`,             // 先頭の空 option (任意項目)
		`<option value="具志川市">具志川（うるま市）</option>`, // 同名区別 (現市町村併記)
		`<option value="具志川村">具志川（久米島町）</option>`, // 同名区別 (別自治体)
		`<option value="国頭村">国頭村</option>`,        // 未合併は市町村名そのもの
	} {
		if !strings.Contains(body, want) {
			t.Errorf("登録フォームHTMLに %q が含まれていない", want)
		}
	}
	// 登録フォームは method override 隠しフィールドを持たない
	if strings.Contains(body, `name="_method"`) {
		t.Error("登録フォームに _method 隠しフィールドが描画されている")
	}
	// 旧 location 自由入力は廃止されている
	if strings.Contains(body, `name="location"`) {
		t.Error("廃止された location 自由入力が残っている")
	}
}

func TestShowCreateForm_ユーザー取得失敗は500(t *testing.T) {
	repo := &fakeDeviceRepo{userErr: errInjected}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/create")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500", w.Code)
	}
}

// --- 3.2 デバイス編集フォーム表示と所有者認可 ---

func TestShowEditForm_所有者一致で既存値復元(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.devices = map[int64]repository.Device{
		1: {ID: 1, UserID: 7, Name: "ハウスA温湿度計", MacAddress: "AA:BB:CC:DD:EE:01", Location: strPtr("ビニールハウスA"), Locality: strPtr("佐敷町"), IsActive: false},
	}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1/edit")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		"<h1>デバイス編集: ハウスA温湿度計</h1>", // デバイス名込み見出し
		`action="/devices/1"`, // 送信先
		`name="_method"`,      // PUT 用隠しフィールド
		`value="put"`,
		`value="ハウスA温湿度計"`,          // 既存値復元 (name)
		`value="AA:BB:CC:DD:EE:01"`, // 既存値復元 (mac)
		`<option value="佐敷町" selected>佐敷（南城市）</option>`, // 既存地域の選択復元 (認識名表示)
		`value="0" checked`, // 停止中 (IsActive=false) の選択復元
		`href="/devices/1"`, // キャンセル導線
	} {
		if !strings.Contains(body, want) {
			t.Errorf("編集フォームHTMLに %q が含まれていない", want)
		}
	}
	// 旧 location 自由入力は廃止され、地域 select に置き換わっている
	if strings.Contains(body, `name="location"`) {
		t.Error("廃止された location 自由入力が残っている")
	}
	if !strings.Contains(body, `name="locality"`) {
		t.Error("地域 select (name=locality) が描画されていない")
	}
}

func TestShowEditForm_存在しないIDは404(t *testing.T) {
	repo := deviceOwnerRepo() // devices 未登録 → GetDevice は ErrNoRows
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/999/edit")
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404 (不在/論理削除)", w.Code)
	}
}

func TestShowEditForm_他ユーザー所有は403(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.devices = map[int64]repository.Device{
		2: {ID: 2, UserID: 999, Name: "他人のデバイス", MacAddress: "AA:BB:CC:DD:EE:02", IsActive: true},
	}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/2/edit")
	if w.Code != http.StatusForbidden {
		t.Errorf("status=%d, want 403 (非所有・BOLA防止)", w.Code)
	}
}

func TestShowEditForm_非数値IDは404(t *testing.T) {
	repo := deviceOwnerRepo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/abc/edit")
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404 (非数値ID)", w.Code)
	}
}

func TestShowEditForm_認可成功後のユーザー取得失敗は500(t *testing.T) {
	repo := &fakeDeviceRepo{
		devices: map[int64]repository.Device{1: {ID: 1, UserID: 7, Name: "D", MacAddress: "AA:BB:CC:DD:EE:01", IsActive: true}},
		userErr: errInjected, // users 取得で内部エラー
	}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1/edit")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500", w.Code)
	}
}

func TestShowEditForm_所有者確認のDBエラーは500(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.getErr = errInjected // GetDevice が ErrNoRows 以外の DB エラー
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1/edit")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500 (ErrNoRows 以外の DB エラー)", w.Code)
	}
}

func TestShowEditForm_稼働中デバイスはvalue1がchecked(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.devices = map[int64]repository.Device{
		3: {ID: 3, UserID: 7, Name: "稼働中デバイス", MacAddress: "AA:BB:CC:DD:EE:03", IsActive: true},
	}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/3/edit")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `value="1" checked`) {
		t.Error("稼働中(IsActive=true)で value=1 が checked になっていない")
	}
	if strings.Contains(body, `value="0" checked`) {
		t.Error("稼働中なのに value=0 が checked になっている")
	}
}

// --- 4.1 デバイス登録の実行 (POST /devices) ---

func TestCreate_正常時は所有者uidと正規化MACとnil_locationで303(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.createResult = repository.Device{ID: 10, UserID: 7}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("is_active", "0")
	w := formRequest(r, http.MethodPost, "/devices", vals)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303 (body=%s)", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "/devices/10" {
		t.Errorf("Location=%q, want /devices/10", loc)
	}
	if !repo.createCalled {
		t.Fatal("CreateDevice が呼ばれていない")
	}
	if repo.lastCreate.UserID != 7 {
		t.Errorf("所有者 UserID=%d, want 7 (session 由来)", repo.lastCreate.UserID)
	}
	if repo.lastCreate.MacAddress != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("MacAddress=%q, want 大文字正規化 AA:BB:CC:DD:EE:FF", repo.lastCreate.MacAddress)
	}
	if repo.lastCreate.Location != nil {
		t.Errorf("Location=%v, want nil (新規デバイスは旧 location を持たない)", repo.lastCreate.Location)
	}
	if repo.lastCreate.IsActive != false {
		t.Errorf("IsActive=%v, want false (is_active=0)", repo.lastCreate.IsActive)
	}
}

func TestCreate_地域は非nilのlocalityで保存(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.createResult = repository.Device{ID: 11, UserID: 7}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := formRequest(r, http.MethodPost, "/devices", validDeviceVals()) // locality=国頭村
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303", w.Code)
	}
	if repo.lastCreate.Locality == nil || *repo.lastCreate.Locality != "国頭村" {
		t.Errorf("Locality=%v, want &\"国頭村\"", repo.lastCreate.Locality)
	}
	if repo.lastCreate.Location != nil {
		t.Errorf("Location=%v, want nil (新規は旧 location 不使用)", repo.lastCreate.Location)
	}
}

func TestCreate_デバイス名未入力は200で作成せずエラー(t *testing.T) {
	repo := deviceOwnerRepo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("name", "")
	w := formRequest(r, http.MethodPost, "/devices", vals)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (再描画)", w.Code)
	}
	if repo.createCalled {
		t.Error("検証失敗時に CreateDevice を呼んではいけない")
	}
	body := w.Body.String()
	if !strings.Contains(body, "デバイス名を入力してください") {
		t.Error("name 必須エラーが表示されていない")
	}
	// 入力値復元 (MAC は元の入力をそのまま復元)
	if !strings.Contains(body, `value="aa:bb:cc:dd:ee:ff"`) {
		t.Error("入力値 (mac) が復元されていない")
	}
}

func TestCreate_デバイス名255超は200でエラー(t *testing.T) {
	repo := deviceOwnerRepo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("name", strings.Repeat("あ", 256))
	w := formRequest(r, http.MethodPost, "/devices", vals)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	if repo.createCalled {
		t.Error("検証失敗時に CreateDevice を呼んではいけない")
	}
	if !strings.Contains(w.Body.String(), "デバイス名は255文字以内で入力してください") {
		t.Error("name 上限超過エラーが表示されていない")
	}
}

func TestCreate_地域未選択はnil_localityで保存(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.createResult = repository.Device{ID: 12, UserID: 7}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("locality", "") // 未選択 (任意項目)
	w := formRequest(r, http.MethodPost, "/devices", vals)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303 (未選択も成功)", w.Code)
	}
	if repo.lastCreate.Locality != nil {
		t.Errorf("Locality=%v, want nil (未選択は未設定)", repo.lastCreate.Locality)
	}
}

func TestCreate_不正な地域は200で作成せずエラー(t *testing.T) {
	repo := deviceOwnerRepo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("locality", "東京都") // 53地域に無い値
	w := formRequest(r, http.MethodPost, "/devices", vals)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (再描画)", w.Code)
	}
	if repo.createCalled {
		t.Error("不正な地域で CreateDevice を呼んではいけない")
	}
	if !strings.Contains(w.Body.String(), localityInvalidMessage) {
		t.Error("地域不正エラーが表示されていない")
	}
}

func TestCreate_ステータス不正は200でエラー(t *testing.T) {
	repo := deviceOwnerRepo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("is_active", "2") // oneof 範囲外
	w := formRequest(r, http.MethodPost, "/devices", vals)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	if repo.createCalled {
		t.Error("検証失敗時に CreateDevice を呼んではいけない")
	}
	if !strings.Contains(w.Body.String(), "ステータスが不正です") {
		t.Error("is_active 不正エラーが表示されていない")
	}
}

func TestCreate_MAC形式不正は200で作成せずエラー(t *testing.T) {
	repo := deviceOwnerRepo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("mac_address", "XX-YY-ZZ") // 形式不正
	w := formRequest(r, http.MethodPost, "/devices", vals)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	if repo.createCalled {
		t.Error("MAC 形式不正で CreateDevice を呼んではいけない")
	}
	if !strings.Contains(w.Body.String(), "MACアドレス") {
		t.Error("mac 形式エラーが表示されていない")
	}
}

func TestCreate_MAC重複は200で作成せずエラー(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.byMac = map[string]repository.Device{
		"AA:BB:CC:DD:EE:FF": {ID: 99, UserID: 7, MacAddress: "AA:BB:CC:DD:EE:FF"},
	}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := formRequest(r, http.MethodPost, "/devices", validDeviceVals())
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	if repo.createCalled {
		t.Error("MAC 重複で CreateDevice を呼んではいけない")
	}
	if !strings.Contains(w.Body.String(), "既に登録されています") {
		t.Error("mac 重複エラーが表示されていない")
	}
}

func TestCreate_大文字小文字違いのMACは重複とみなす(t *testing.T) {
	repo := deviceOwnerRepo()
	// 既存は大文字、入力は小文字 → 正規化で同一視され重複
	repo.byMac = map[string]repository.Device{
		"AA:BB:CC:DD:EE:FF": {ID: 99, UserID: 7, MacAddress: "AA:BB:CC:DD:EE:FF"},
	}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals() // mac_address = aa:bb:cc:dd:ee:ff (小文字)
	w := formRequest(r, http.MethodPost, "/devices", vals)
	if w.Code != http.StatusOK || repo.createCalled {
		t.Errorf("大小文字違いの MAC は重複とみなすべき: status=%d createCalled=%v", w.Code, repo.createCalled)
	}
}

func TestCreate_MAC一意検査のDBエラーは500(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.macErr = errInjected // GetDeviceByMacAddress が ErrNoRows 以外
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := formRequest(r, http.MethodPost, "/devices", validDeviceVals())
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500", w.Code)
	}
}

func TestCreate_作成処理の内部エラーは500(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.createErr = errInjected
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := formRequest(r, http.MethodPost, "/devices", validDeviceVals())
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500", w.Code)
	}
}

// --- 4.2 デバイス更新の実行 (PUT /devices/:device) ---

func ownedDevice1Repo() *fakeDeviceRepo {
	repo := deviceOwnerRepo()
	repo.devices = map[int64]repository.Device{
		1: {ID: 1, UserID: 7, Name: "旧名", MacAddress: "AA:BB:CC:DD:EE:01", Location: strPtr("旧場所"), IsActive: true},
	}
	repo.updateResult = repository.Device{ID: 1, UserID: 7}
	return repo
}

func TestUpdate_正常時は更新して303(t *testing.T) {
	repo := ownedDevice1Repo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("mac_address", "aa:bb:cc:dd:ee:99") // 別MAC(重複なし)へ変更
	w := formRequest(r, http.MethodPut, "/devices/1", vals)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303 (body=%s)", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "/devices/1" {
		t.Errorf("Location=%q, want /devices/1", loc)
	}
	if !repo.updateCalled {
		t.Fatal("UpdateDevice が呼ばれていない")
	}
	if repo.lastUpdate.ID != 1 {
		t.Errorf("更新対象 ID=%d, want 1", repo.lastUpdate.ID)
	}
	if repo.lastUpdate.MacAddress != "AA:BB:CC:DD:EE:99" {
		t.Errorf("MacAddress=%q, want 正規化 AA:BB:CC:DD:EE:99", repo.lastUpdate.MacAddress)
	}
	if repo.lastUpdate.Locality == nil || *repo.lastUpdate.Locality != "国頭村" {
		t.Errorf("Locality=%v, want &\"国頭村\" (フォームの地域)", repo.lastUpdate.Locality)
	}
}

// TestUpdate_既存locationを保全しlocalityを更新する は、フォームから廃止された旧 location 列を
// 編集で破壊せず既存値を保全し (非破壊・R7)、所在地は locality として更新する契約を固定する。
func TestUpdate_既存locationを保全しlocalityを更新する(t *testing.T) {
	repo := ownedDevice1Repo() // 既存 device1 の Location は "旧場所"
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("mac_address", "aa:bb:cc:dd:ee:99") // 重複なし
	vals.Set("locality", "佐敷町")                  // 別地域へ更新
	w := formRequest(r, http.MethodPut, "/devices/1", vals)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303 (body=%s)", w.Code, w.Body.String())
	}
	if !repo.updateCalled {
		t.Fatal("UpdateDevice が呼ばれていない")
	}
	if repo.lastUpdate.Location == nil || *repo.lastUpdate.Location != "旧場所" {
		t.Errorf("Location=%v, want 既存値保全 \"旧場所\" (location は編集対象外・非破壊)", repo.lastUpdate.Location)
	}
	if repo.lastUpdate.Locality == nil || *repo.lastUpdate.Locality != "佐敷町" {
		t.Errorf("Locality=%v, want &\"佐敷町\"", repo.lastUpdate.Locality)
	}
}

func TestUpdate_自身のMAC据置は許可して303(t *testing.T) {
	repo := ownedDevice1Repo()
	// 自身の現在 MAC が byMac にヒットするが、existing.ID==id なので重複扱いしない
	repo.byMac = map[string]repository.Device{
		"AA:BB:CC:DD:EE:01": {ID: 1, UserID: 7, MacAddress: "AA:BB:CC:DD:EE:01"},
	}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("mac_address", "AA:BB:CC:DD:EE:01") // 自身の現在値で据置
	w := formRequest(r, http.MethodPut, "/devices/1", vals)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303 (自身MAC据置は許可)", w.Code)
	}
	if !repo.updateCalled {
		t.Error("自身MAC据置で UpdateDevice が呼ばれていない")
	}
}

func TestUpdate_他デバイスとMAC重複は200で更新せずエラー(t *testing.T) {
	repo := ownedDevice1Repo()
	// 別デバイス(ID:2)が同じ MAC を保持 → existing.ID != 1 で重複
	repo.byMac = map[string]repository.Device{
		"AA:BB:CC:DD:EE:99": {ID: 2, UserID: 7, MacAddress: "AA:BB:CC:DD:EE:99"},
	}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("mac_address", "AA:BB:CC:DD:EE:99")
	w := formRequest(r, http.MethodPut, "/devices/1", vals)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	if repo.updateCalled {
		t.Error("MAC 重複で UpdateDevice を呼んではいけない")
	}
	if !strings.Contains(w.Body.String(), "既に登録されています") {
		t.Error("mac 重複エラーが表示されていない")
	}
}

func TestUpdate_不在は404で更新しない(t *testing.T) {
	repo := deviceOwnerRepo() // devices 未登録
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := formRequest(r, http.MethodPut, "/devices/999", validDeviceVals())
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", w.Code)
	}
	if repo.updateCalled {
		t.Error("不在で UpdateDevice を呼んではいけない")
	}
}

func TestUpdate_他ユーザー所有は403で更新しない(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.devices = map[int64]repository.Device{
		2: {ID: 2, UserID: 999, Name: "他人", MacAddress: "AA:BB:CC:DD:EE:02", IsActive: true},
	}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := formRequest(r, http.MethodPut, "/devices/2", validDeviceVals())
	if w.Code != http.StatusForbidden {
		t.Errorf("status=%d, want 403 (BOLA)", w.Code)
	}
	if repo.updateCalled {
		t.Error("非所有で UpdateDevice を呼んではいけない (BOLA)")
	}
}

func TestUpdate_検証失敗は200で更新せず再描画(t *testing.T) {
	repo := ownedDevice1Repo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("name", "")
	w := formRequest(r, http.MethodPut, "/devices/1", vals)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	if repo.updateCalled {
		t.Error("検証失敗で UpdateDevice を呼んではいけない")
	}
	body := w.Body.String()
	if !strings.Contains(body, "デバイス名を入力してください") {
		t.Error("name 必須エラーが表示されていない")
	}
	if !strings.Contains(body, `name="_method"`) {
		t.Error("編集フォーム再描画に _method 隠しフィールドがない")
	}
}

func TestUpdate_MAC形式不正は200で更新しない(t *testing.T) {
	repo := ownedDevice1Repo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("mac_address", "bad-mac")
	w := formRequest(r, http.MethodPut, "/devices/1", vals)

	if w.Code != http.StatusOK || repo.updateCalled {
		t.Errorf("MAC 形式不正: status=%d updateCalled=%v, want 200 & false", w.Code, repo.updateCalled)
	}
}

func TestUpdate_更新処理の内部エラーは500(t *testing.T) {
	repo := ownedDevice1Repo()
	repo.updateErr = errInjected
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("mac_address", "aa:bb:cc:dd:ee:99")
	w := formRequest(r, http.MethodPut, "/devices/1", vals)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500", w.Code)
	}
}

func TestUpdate_非数値IDは404(t *testing.T) {
	repo := deviceOwnerRepo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := formRequest(r, http.MethodPut, "/devices/abc", validDeviceVals())
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404 (非数値ID)", w.Code)
	}
}

func TestUpdate_MAC一意検査のDBエラーは500(t *testing.T) {
	repo := ownedDevice1Repo()
	repo.macErr = errInjected // GetDeviceByMacAddress が ErrNoRows 以外
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("mac_address", "aa:bb:cc:dd:ee:99")
	w := formRequest(r, http.MethodPut, "/devices/1", vals)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500", w.Code)
	}
}

// --- 再描画時のレイアウト用ユーザー取得失敗 (App レイアウトは UserName を要する) ---

func TestCreate_再描画時のユーザー取得失敗は500(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.userErr = errInjected // 再描画時の GetUser が失敗
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("name", "") // 検証失敗 → reRenderCreate へ
	w := formRequest(r, http.MethodPost, "/devices", vals)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500 (再描画時のユーザー取得失敗)", w.Code)
	}
}

func TestUpdate_再描画時のユーザー取得失敗は500(t *testing.T) {
	repo := ownedDevice1Repo()
	repo.userErr = errInjected // 認可は GetDevice 経由で通過、再描画の GetUser で失敗
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("name", "") // 検証失敗 → reRenderEdit へ
	w := formRequest(r, http.MethodPut, "/devices/1", vals)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500 (再描画時のユーザー取得失敗)", w.Code)
	}
}

// --- 6. Validation: エッジケースの通し確認 (ハンドラ→templ 描画の end-to-end) ---

// R5.6: 複数項目で同時に失敗したとき、各項目欄にそれぞれのエラーが同時表示され、
// 有効だった項目の入力値は復元される (R5.5) ことを描画結果で固定する。
func TestValidation_複数項目同時エラーが各項目に同時表示される(t *testing.T) {
	repo := deviceOwnerRepo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := url.Values{
		"name":        {""},    // 必須エラー
		"mac_address": {""},    // 必須エラー
		"locality":    {"国頭村"}, // 有効 → 選択復元される
		"is_active":   {""},    // 必須エラー
	}
	w := formRequest(r, http.MethodPost, "/devices", vals)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (再描画)", w.Code)
	}
	if repo.createCalled {
		t.Error("検証失敗で CreateDevice を呼んではいけない")
	}
	body := w.Body.String()
	for _, msg := range []string{
		"デバイス名を入力してください",
		"MACアドレスを入力してください",
		"ステータスを選択してください",
	} {
		if !strings.Contains(body, msg) {
			t.Errorf("複数項目同時エラーで %q が表示されていない", msg)
		}
	}
	if !strings.Contains(body, `<option value="国頭村" selected>国頭村</option>`) {
		t.Error("有効だった地域の選択が復元されていない (R5.5)")
	}
}

// R5.2: 所在地 (procedural 検証) と他項目が同時に不備のとき、両方のエラーが同時表示される
// (地域検証が early-return せず累積されることを固定する)。
func TestValidation_地域不正と他項目不備が同時表示される(t *testing.T) {
	repo := deviceOwnerRepo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("name", "")            // 必須エラー (binding)
	vals.Set("locality", "存在しない地域") // 地域エラー (procedural)
	w := formRequest(r, http.MethodPost, "/devices", vals)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (再描画)", w.Code)
	}
	if repo.createCalled {
		t.Error("検証失敗で CreateDevice を呼んではいけない")
	}
	body := w.Body.String()
	if !strings.Contains(body, "デバイス名を入力してください") {
		t.Error("name 必須エラーが表示されていない")
	}
	if !strings.Contains(body, localityInvalidMessage) {
		t.Error("地域不正エラーが同時表示されていない (累積されていない)")
	}
}

// R6.3: 大文字小文字違いの MAC (既存 AA:.. と入力 aa:..) を同一とみなし、
// 重複登録を防止して項目別エラーを表示し、入力値 (入力したまま) を復元する。
func TestValidation_大文字小文字違いMACは重複として防止しエラー表示(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.byMac = map[string]repository.Device{
		"AA:BB:CC:DD:EE:FF": {ID: 99, UserID: 7, MacAddress: "AA:BB:CC:DD:EE:FF"}, // 既存は大文字
	}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals() // mac_address = aa:bb:cc:dd:ee:ff (小文字)
	w := formRequest(r, http.MethodPost, "/devices", vals)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	if repo.createCalled {
		t.Error("大小文字違いの MAC 重複で CreateDevice を呼んではいけない (重複登録防止)")
	}
	body := w.Body.String()
	if !strings.Contains(body, "既に登録されています") {
		t.Error("MAC 重複エラーが表示されていない")
	}
	if !strings.Contains(body, `value="aa:bb:cc:dd:ee:ff"`) {
		t.Error("入力した MAC (小文字) が復元されていない")
	}
}
