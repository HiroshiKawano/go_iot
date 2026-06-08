package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"database/sql"
	"github.com/HiroshiKawano/go_iot/internal/middleware"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// fakeAuthRepo は AuthRepo の最小モック。
type fakeAuthRepo struct {
	byEmail       map[string]repository.User
	byID          map[int64]repository.User
	getByEmailErr error
	createErr     error
	createCalled  bool
	lastCreate    repository.CreateUserParams

	// ダッシュボード表示用 (既定はゼロ値 = 空スライス / 未受信)
	devices     []repository.Device                                    // ListDevicesByUser の戻り
	devicesErr  error                                                  // ListDevicesByUser のエラー注入
	readings    map[int64]repository.SensorReading                     // deviceID → 最新計測 (per-device)
	readingErrs map[int64]error                                        // deviceID → エラー注入 (per-device)
	alerts      []repository.ListUnnotifiedAlertHistoriesWithDeviceRow // 未対応アラート
	alertsErr   error                                                  // ListUnnotified... のエラー注入

	// 本人スコープ伝播検証用に handler から渡された引数を記録する (R1.3)
	gotDevicesUserID    int64   // ListDevicesByUser が受領した userID
	gotAlertsUserID     int64   // ListUnnotifiedAlertHistoriesWithDevice が受領した userID
	gotReadingDeviceIDs []int64 // GetLatestSensorReading が受領した deviceID 列 (呼び出し順)
}

func (f *fakeAuthRepo) GetUserByEmail(_ context.Context, email string) (repository.User, error) {
	if f.getByEmailErr != nil {
		return repository.User{}, f.getByEmailErr
	}
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return repository.User{}, sql.ErrNoRows
}

func (f *fakeAuthRepo) CreateUser(_ context.Context, arg repository.CreateUserParams) (repository.User, error) {
	f.createCalled = true
	f.lastCreate = arg
	if f.createErr != nil {
		return repository.User{}, f.createErr
	}
	u := repository.User{ID: 99, Name: arg.Name, Email: arg.Email, PasswordHash: arg.PasswordHash}
	if f.byID == nil {
		f.byID = map[int64]repository.User{}
	}
	f.byID[99] = u
	return u, nil
}

func (f *fakeAuthRepo) GetUser(_ context.Context, id int64) (repository.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return repository.User{}, sql.ErrNoRows
}

func (f *fakeAuthRepo) ListDevicesByUser(_ context.Context, userID int64) ([]repository.Device, error) {
	f.gotDevicesUserID = userID
	if f.devicesErr != nil {
		return nil, f.devicesErr
	}
	return f.devices, nil
}

// GetLatestSensorReading は per-device の値/エラー注入に対応する。
// エラー注入 > 値注入 の優先順で判定し、いずれも無ければ未受信 (sql.ErrNoRows)。
func (f *fakeAuthRepo) GetLatestSensorReading(_ context.Context, deviceID int64) (repository.SensorReading, error) {
	f.gotReadingDeviceIDs = append(f.gotReadingDeviceIDs, deviceID)
	if err, ok := f.readingErrs[deviceID]; ok {
		return repository.SensorReading{}, err
	}
	if r, ok := f.readings[deviceID]; ok {
		return r, nil
	}
	return repository.SensorReading{}, sql.ErrNoRows
}

func (f *fakeAuthRepo) ListUnnotifiedAlertHistoriesWithDevice(_ context.Context, arg repository.ListUnnotifiedAlertHistoriesWithDeviceParams) ([]repository.ListUnnotifiedAlertHistoriesWithDeviceRow, error) {
	f.gotAlertsUserID = arg.UserID
	if f.alertsErr != nil {
		return nil, f.alertsErr
	}
	return f.alerts, nil
}

// newAuthApp は LoadAndSave + SessionLoad + 認証ルートを備えた http.Handler を返す。
func newAuthApp(repo AuthRepo) http.Handler {
	sm := scs.New()
	h := &AuthHandler{Repo: repo, SM: sm}
	r := gin.New()
	web := r.Group("/", middleware.SessionLoad(sm))
	web.GET("/login", h.LoginGet)
	web.POST("/login", h.LoginPost)
	web.GET("/register", h.RegisterGet)
	web.POST("/register", h.RegisterPost)
	web.POST("/logout", h.Logout)
	web.GET("/dashboard", middleware.RequireAuth(), h.Dashboard)
	web.GET("/", h.Root)
	return sm.LoadAndSave(r)
}

func postForm(app http.Handler, path string, vals url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	return w
}

func getWithCookies(app http.Handler, path string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	return w
}

func userWithPassword(id int64, name, email, password string) repository.User {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return repository.User{ID: id, Name: name, Email: email, PasswordHash: string(hash)}
}

// --- ログイン ---

func TestLoginPost_成功で303とセッション確立(t *testing.T) {
	u := userWithPassword(7, "山田太郎", "user@example.com", "password123")
	repo := &fakeAuthRepo{
		byEmail: map[string]repository.User{"user@example.com": u},
		byID:    map[int64]repository.User{7: u},
	}
	app := newAuthApp(repo)

	w := postForm(app, "/login", url.Values{"email": {"user@example.com"}, "password": {"password123"}})
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body=%s)", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "/dashboard" {
		t.Errorf("Location = %q, want /dashboard", loc)
	}

	// セッション確立を /dashboard 到達 + ユーザー名表示で確認
	w2 := getWithCookies(app, "/dashboard", w.Result().Cookies())
	if w2.Code != http.StatusOK {
		t.Fatalf("/dashboard after login = %d, want 200", w2.Code)
	}
	if !strings.Contains(w2.Body.String(), "山田太郎") {
		t.Error("ダッシュボードにログインユーザー名が表示されていない")
	}
}

func TestLoginPost_パスワード不一致は200で共通エラー(t *testing.T) {
	u := userWithPassword(7, "山田太郎", "user@example.com", "password123")
	repo := &fakeAuthRepo{byEmail: map[string]repository.User{"user@example.com": u}}
	app := newAuthApp(repo)

	w := postForm(app, "/login", url.Values{"email": {"user@example.com"}, "password": {"wrongpass"}})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "メールアドレスまたはパスワードが間違っています") {
		t.Error("共通エラーメッセージが表示されていない")
	}
}

func TestLoginPost_存在しないメールは200で共通エラー(t *testing.T) {
	repo := &fakeAuthRepo{byEmail: map[string]repository.User{}}
	app := newAuthApp(repo)

	w := postForm(app, "/login", url.Values{"email": {"nobody@example.com"}, "password": {"whatever1"}})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "メールアドレスまたはパスワードが間違っています") {
		t.Error("共通エラーメッセージが表示されていない")
	}
}

func TestLoginPost_メール形式不正は200でフィールドエラー(t *testing.T) {
	app := newAuthApp(&fakeAuthRepo{})
	w := postForm(app, "/login", url.Values{"email": {"not-an-email"}, "password": {"x"}})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "メールアドレス形式で入力してください") {
		t.Error("メール形式エラーが表示されていない")
	}
}

// --- 登録 ---

func TestRegisterPost_成功でCreateUserと自動ログイン(t *testing.T) {
	repo := &fakeAuthRepo{byEmail: map[string]repository.User{}}
	app := newAuthApp(repo)

	w := postForm(app, "/register", url.Values{
		"name":                  {"新規太郎"},
		"email":                 {"new@example.com"},
		"password":              {"password123"},
		"password_confirmation": {"password123"},
	})
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body=%s)", w.Code, w.Body.String())
	}
	if w.Header().Get("Location") != "/dashboard" {
		t.Errorf("Location = %q, want /dashboard", w.Header().Get("Location"))
	}
	if !repo.createCalled {
		t.Fatal("CreateUser が呼ばれていない")
	}
	if repo.lastCreate.PasswordHash == "password123" || repo.lastCreate.PasswordHash == "" {
		t.Error("パスワードがハッシュ化されていない")
	}
	// 自動ログイン確認
	w2 := getWithCookies(app, "/dashboard", w.Result().Cookies())
	if w2.Code != http.StatusOK {
		t.Fatalf("/dashboard after register = %d, want 200", w2.Code)
	}
}

func TestRegisterPost_メール重複は200でエラー(t *testing.T) {
	existing := repository.User{ID: 1, Email: "dup@example.com"}
	repo := &fakeAuthRepo{byEmail: map[string]repository.User{"dup@example.com": existing}}
	app := newAuthApp(repo)

	w := postForm(app, "/register", url.Values{
		"name":                  {"重複太郎"},
		"email":                 {"dup@example.com"},
		"password":              {"password123"},
		"password_confirmation": {"password123"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "このメールアドレスは既に登録されています") {
		t.Error("重複エラーが表示されていない")
	}
	if repo.createCalled {
		t.Error("重複時に CreateUser が呼ばれてはいけない")
	}
}

func TestRegisterPost_パスワード8字未満は200でエラー(t *testing.T) {
	app := newAuthApp(&fakeAuthRepo{byEmail: map[string]repository.User{}})
	w := postForm(app, "/register", url.Values{
		"name":                  {"太郎"},
		"email":                 {"a@example.com"},
		"password":              {"short"},
		"password_confirmation": {"short"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "8文字以上で入力してください") {
		t.Error("パスワード長エラーが表示されていない")
	}
}

func TestRegisterPost_確認用不一致は200でエラー(t *testing.T) {
	app := newAuthApp(&fakeAuthRepo{byEmail: map[string]repository.User{}})
	w := postForm(app, "/register", url.Values{
		"name":                  {"太郎"},
		"email":                 {"a@example.com"},
		"password":              {"password123"},
		"password_confirmation": {"different1"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "パスワードが一致しません") {
		t.Error("確認用不一致エラーが表示されていない")
	}
}

// --- ログアウト ---

func TestLogout_セッション破棄後はガードが効く(t *testing.T) {
	u := userWithPassword(7, "山田太郎", "user@example.com", "password123")
	repo := &fakeAuthRepo{
		byEmail: map[string]repository.User{"user@example.com": u},
		byID:    map[int64]repository.User{7: u},
	}
	app := newAuthApp(repo)

	login := postForm(app, "/login", url.Values{"email": {"user@example.com"}, "password": {"password123"}})
	cookies := login.Result().Cookies()

	// ログアウト
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	lw := httptest.NewRecorder()
	app.ServeHTTP(lw, req)
	if lw.Code != http.StatusSeeOther || lw.Header().Get("Location") != "/login" {
		t.Fatalf("logout = %d Location=%q, want 303 /login", lw.Code, lw.Header().Get("Location"))
	}

	// ログアウト後の cookie で /dashboard → 302 /login
	w2 := getWithCookies(app, "/dashboard", lw.Result().Cookies())
	if w2.Code != http.StatusFound || w2.Header().Get("Location") != "/login" {
		t.Errorf("logout 後の /dashboard = %d Location=%q, want 302 /login", w2.Code, w2.Header().Get("Location"))
	}
}

// --- 認証ガード / ルート振り分け ---

func TestDashboard_未認証は302でlogin(t *testing.T) {
	app := newAuthApp(&fakeAuthRepo{})
	w := getWithCookies(app, "/dashboard", nil)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/login" {
		t.Errorf("未認証 /dashboard = %d Location=%q, want 302 /login", w.Code, w.Header().Get("Location"))
	}
}

// --- ダッシュボード用 DB ポート拡張 (fakeAuthRepo の注入セマンティクス) ---

// errInjected はクエリのエラー注入用センチネル。
var errInjected = errors.New("injected db error")

// 拡張後の AuthRepo を fakeAuthRepo が満たすことをコンパイル時に保証する。
var _ AuthRepo = (*fakeAuthRepo)(nil)

func TestFakeAuthRepo_既定は空スライスと未受信(t *testing.T) {
	repo := &fakeAuthRepo{}
	ctx := context.Background()

	devs, err := repo.ListDevicesByUser(ctx, 1)
	if err != nil {
		t.Fatalf("ListDevicesByUser err = %v", err)
	}
	if len(devs) != 0 {
		t.Errorf("既定デバイス len = %d, want 0", len(devs))
	}

	alerts, err := repo.ListUnnotifiedAlertHistoriesWithDevice(ctx,
		repository.ListUnnotifiedAlertHistoriesWithDeviceParams{UserID: 1, Limit: 50})
	if err != nil {
		t.Fatalf("ListUnnotified err = %v", err)
	}
	if len(alerts) != 0 {
		t.Errorf("既定アラート len = %d, want 0", len(alerts))
	}

	// 計測未注入は未受信 (:one → sql.ErrNoRows) として扱う
	if _, err := repo.GetLatestSensorReading(ctx, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("未注入の最新計測 err = %v, want sql.ErrNoRows", err)
	}
}

func TestFakeAuthRepo_最新計測はper_deviceで値とエラーを注入できる(t *testing.T) {
	repo := &fakeAuthRepo{
		devices:     []repository.Device{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}},
		readings:    map[int64]repository.SensorReading{1: {ID: 11, DeviceID: 1}},
		readingErrs: map[int64]error{2: errInjected},
	}
	ctx := context.Background()

	if devs, _ := repo.ListDevicesByUser(ctx, 1); len(devs) != 2 {
		t.Fatalf("デバイス len = %d, want 2", len(devs))
	}

	// device 1: 値あり
	r, err := repo.GetLatestSensorReading(ctx, 1)
	if err != nil {
		t.Fatalf("device1 計測 err = %v", err)
	}
	if r.ID != 11 {
		t.Errorf("device1 計測 ID = %d, want 11", r.ID)
	}

	// device 2: エラー注入
	if _, err := repo.GetLatestSensorReading(ctx, 2); !errors.Is(err, errInjected) {
		t.Errorf("device2 計測 err = %v, want errInjected", err)
	}

	// device 3: 未設定 → 未受信
	if _, err := repo.GetLatestSensorReading(ctx, 3); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("device3 計測 err = %v, want sql.ErrNoRows", err)
	}
}

func TestFakeAuthRepo_各クエリのエラーを注入できる(t *testing.T) {
	ctx := context.Background()

	dRepo := &fakeAuthRepo{devicesErr: errInjected}
	if _, err := dRepo.ListDevicesByUser(ctx, 1); !errors.Is(err, errInjected) {
		t.Errorf("ListDevicesByUser err = %v, want errInjected", err)
	}

	aRepo := &fakeAuthRepo{alertsErr: errInjected}
	if _, err := aRepo.ListUnnotifiedAlertHistoriesWithDevice(ctx,
		repository.ListUnnotifiedAlertHistoriesWithDeviceParams{UserID: 1, Limit: 50}); !errors.Is(err, errInjected) {
		t.Errorf("ListUnnotified err = %v, want errInjected", err)
	}
}

func TestRoot_未認証はlogin認証済はdashboard(t *testing.T) {
	u := userWithPassword(7, "山田太郎", "user@example.com", "password123")
	repo := &fakeAuthRepo{
		byEmail: map[string]repository.User{"user@example.com": u},
		byID:    map[int64]repository.User{7: u},
	}
	app := newAuthApp(repo)

	// 未認証
	w := getWithCookies(app, "/", nil)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/login" {
		t.Errorf("未認証 / = %d Location=%q, want 302 /login", w.Code, w.Header().Get("Location"))
	}

	// 認証済
	login := postForm(app, "/login", url.Values{"email": {"user@example.com"}, "password": {"password123"}})
	w2 := getWithCookies(app, "/", login.Result().Cookies())
	if w2.Code != http.StatusFound || w2.Header().Get("Location") != "/dashboard" {
		t.Errorf("認証済 / = %d Location=%q, want 302 /dashboard", w2.Code, w2.Header().Get("Location"))
	}
}
