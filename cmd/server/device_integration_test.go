package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/config"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// fakeDeviceQuerier はデバイス登録/編集フローに必要なクエリを in-memory 実装する
// (login 用の users を含む)。埋め込み Querier (nil) のため未実装メソッド呼び出しは panic。
type fakeDeviceQuerier struct {
	repository.Querier
	byEmail map[string]repository.User
	byID    map[int64]repository.User
	devices map[int64]repository.Device
	byMac   map[string]repository.Device
	created repository.Device
	updated repository.Device
}

func (f fakeDeviceQuerier) GetUserByEmail(_ context.Context, email string) (repository.User, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return repository.User{}, pgx.ErrNoRows
}

func (f fakeDeviceQuerier) GetUser(_ context.Context, id int64) (repository.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return repository.User{}, pgx.ErrNoRows
}

func (f fakeDeviceQuerier) GetDevice(_ context.Context, id int64) (repository.Device, error) {
	if d, ok := f.devices[id]; ok {
		return d, nil
	}
	return repository.Device{}, pgx.ErrNoRows
}

func (f fakeDeviceQuerier) GetDeviceByMacAddress(_ context.Context, mac string) (repository.Device, error) {
	if d, ok := f.byMac[mac]; ok {
		return d, nil
	}
	return repository.Device{}, pgx.ErrNoRows
}

func (f fakeDeviceQuerier) CreateDevice(_ context.Context, _ repository.CreateDeviceParams) (repository.Device, error) {
	return f.created, nil
}

func (f fakeDeviceQuerier) UpdateDevice(_ context.Context, _ repository.UpdateDeviceParams) (repository.Device, error) {
	return f.updated, nil
}

// deviceApp は user(id=1) と任意のデバイスを備えた合成ハンドラを返す。
func deviceApp(devices map[int64]repository.Device, created, updated repository.Device) http.Handler {
	gin.SetMode(gin.TestMode)
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	u := repository.User{ID: 1, Name: "テストユーザー", Email: "test@example.com", PasswordHash: string(hash)}
	q := fakeDeviceQuerier{
		byEmail: map[string]repository.User{"test@example.com": u},
		byID:    map[int64]repository.User{1: u},
		devices: devices,
		byMac:   map[string]repository.Device{},
		created: created,
		updated: updated,
	}
	cfg := &config.Config{AppEnv: "development", SessionSecret: "0123456789abcdef0123456789abcdef"}
	return newHTTPHandler(cfg, scs.New(), q, func(context.Context) error { return nil })
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

func postFormWithCookies(app http.Handler, path string, form url.Values, cookies []*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	return w
}

// loginCookies はログインを通し、以後のリクエストで使う cookie 群 (csrf + session) を返す。
func loginCookies(t *testing.T, app http.Handler) []*http.Cookie {
	t.Helper()
	lw := get(app, "/login")
	token := extractCSRFToken(lw.Body.String())
	if token == "" {
		t.Fatal("ログインページから CSRF トークンを取得できない")
	}
	getCookies := lw.Result().Cookies()
	form := url.Values{
		"email":              {"test@example.com"},
		"password":           {"password"},
		"gorilla.csrf.Token": {token},
	}
	pw := postFormWithCookies(app, "/login", form, getCookies)
	if pw.Code != http.StatusSeeOther {
		t.Fatalf("前提のログイン失敗: status=%d body=%s", pw.Code, pw.Body.String())
	}
	return mergeCookies(getCookies, pw.Result().Cookies())
}

// --- 7.1 未認証は 302 /login (静的経路とパラメータ経路の共存も兼ねる) ---

func TestIntegration_未認証のデバイスフォームGETは302(t *testing.T) {
	app := deviceApp(nil, repository.Device{}, repository.Device{})

	for _, path := range []string{"/devices/create", "/devices/1/edit"} {
		w := get(app, path)
		if w.Code != http.StatusFound || w.Header().Get("Location") != "/login" {
			t.Errorf("未認証 GET %s = %d Location=%q, want 302 /login", path, w.Code, w.Header().Get("Location"))
		}
	}
}

func TestIntegration_未認証のPOST_devicesは302(t *testing.T) {
	app := deviceApp(nil, repository.Device{}, repository.Device{})

	// 未認証でも CSRF トークンは /login GET から取得できる (csrf cookie 由来・セッション非依存)。
	lw := get(app, "/login")
	token := extractCSRFToken(lw.Body.String())
	form := url.Values{
		"name":               {"温室"},
		"mac_address":        {"aa:bb:cc:dd:ee:ff"},
		"is_active":          {"1"},
		"gorilla.csrf.Token": {token},
	}
	w := postFormWithCookies(app, "/devices", form, lw.Result().Cookies())
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/login" {
		t.Errorf("未認証 POST /devices = %d Location=%q, want 302 /login (RequireAuth)", w.Code, w.Header().Get("Location"))
	}
}

// --- CSRF 往復 (認証済み) ---

func TestIntegration_認証済みCSRF往復で登録が303(t *testing.T) {
	app := deviceApp(nil, repository.Device{ID: 10, UserID: 1}, repository.Device{})
	cookies := loginCookies(t, app)

	// GET /devices/create → 200 + フォーム内 CSRF トークン取得
	cw := getWithCookies(app, "/devices/create", cookies)
	if cw.Code != http.StatusOK {
		t.Fatalf("GET /devices/create = %d, want 200", cw.Code)
	}
	token := extractCSRFToken(cw.Body.String())
	if token == "" {
		t.Fatal("登録フォームから CSRF トークンを取得できない")
	}
	cookies = mergeCookies(cookies, cw.Result().Cookies())

	form := url.Values{
		"name":               {"温室センサー"},
		"mac_address":        {"aa:bb:cc:dd:ee:ff"},
		"location":           {""},
		"is_active":          {"1"},
		"gorilla.csrf.Token": {token},
	}
	pw := postFormWithCookies(app, "/devices", form, cookies)
	if pw.Code != http.StatusSeeOther || pw.Header().Get("Location") != "/devices/10" {
		t.Fatalf("CSRF 往復の POST /devices = %d Location=%q, want 303 /devices/10 (body=%s)", pw.Code, pw.Header().Get("Location"), pw.Body.String())
	}
}

func TestIntegration_認証済みでもCSRFトークン欠落のPOSTは403(t *testing.T) {
	app := deviceApp(nil, repository.Device{ID: 10, UserID: 1}, repository.Device{})
	cookies := loginCookies(t, app)

	form := url.Values{
		"name":        {"温室センサー"},
		"mac_address": {"aa:bb:cc:dd:ee:ff"},
		"is_active":   {"1"},
		// gorilla.csrf.Token なし
	}
	w := postFormWithCookies(app, "/devices", form, cookies)
	if w.Code != http.StatusForbidden {
		t.Errorf("トークン欠落 POST /devices = %d, want 403", w.Code)
	}
}

// --- _method オーバーライドで PUT ルートへ解決 ---

func TestIntegration_methodオーバーライドでPUTに解決され更新303(t *testing.T) {
	devices := map[int64]repository.Device{
		1: {ID: 1, UserID: 1, Name: "既存デバイス", MacAddress: "AA:BB:CC:DD:EE:01", IsActive: true},
	}
	app := deviceApp(devices, repository.Device{}, repository.Device{ID: 1, UserID: 1})
	cookies := loginCookies(t, app)

	// GET /devices/1/edit → 200 + トークン取得
	ew := getWithCookies(app, "/devices/1/edit", cookies)
	if ew.Code != http.StatusOK {
		t.Fatalf("GET /devices/1/edit = %d, want 200 (body=%s)", ew.Code, ew.Body.String())
	}
	token := extractCSRFToken(ew.Body.String())
	cookies = mergeCookies(cookies, ew.Result().Cookies())

	// POST /devices/1 + hidden _method=put → MethodOverride で PUT /devices/:device に解決
	form := url.Values{
		"_method":            {"put"},
		"name":               {"更新後デバイス"},
		"mac_address":        {"aa:bb:cc:dd:ee:99"},
		"location":           {""},
		"is_active":          {"0"},
		"gorilla.csrf.Token": {token},
	}
	pw := postFormWithCookies(app, "/devices/1", form, cookies)
	if pw.Code != http.StatusSeeOther || pw.Header().Get("Location") != "/devices/1" {
		t.Fatalf("_method=put の POST /devices/1 = %d Location=%q, want 303 /devices/1 (body=%s)", pw.Code, pw.Header().Get("Location"), pw.Body.String())
	}
}
