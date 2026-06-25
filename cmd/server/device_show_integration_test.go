package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// showApp はデバイス1(所有者=ログインユーザー id=1)を備えた合成ハンドラを返す。
func showApp() http.Handler {
	devices := map[int64]repository.Device{
		1: {ID: 1, UserID: 1, Name: "ハウスA温湿度計", MacAddress: "AA:BB:CC:DD:EE:01", IsActive: true},
	}
	return deviceApp(devices, repository.Device{}, repository.Device{})
}

// deleteWithCookies は DELETE を任意ヘッダ・cookie 付きで送る (HTMX DELETE 検証用)。
func deleteWithCookies(app http.Handler, path string, headers map[string]string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	return w
}

// authedCSRFToken はログイン後に /devices/create フォームから CSRF トークンを取得し、
// 更新後の cookie 群とともに返す (削除リクエストのトークン送出用)。
func authedCSRFToken(t *testing.T, app http.Handler, cookies []*http.Cookie) (string, []*http.Cookie) {
	t.Helper()
	cw := getWithCookies(app, "/devices/create", cookies)
	if cw.Code != http.StatusOK {
		t.Fatalf("GET /devices/create = %d, want 200", cw.Code)
	}
	token := extractCSRFToken(cw.Body.String())
	if token == "" {
		t.Fatal("CSRF トークンを取得できない")
	}
	return token, mergeCookies(cookies, cw.Result().Cookies())
}

// --- 7.1 未認証は 302 /login (閲覧系 GET 2 ルート) ---

func TestIntegration_未認証の詳細表示と期間切替GETは302(t *testing.T) {
	app := showApp()

	for _, path := range []string{"/devices/1", "/devices/1/chart?period=24h"} {
		w := get(app, path)
		if w.Code != http.StatusFound || w.Header().Get("Location") != "/login" {
			t.Errorf("未認証 GET %s = %d Location=%q, want 302 /login", path, w.Code, w.Header().Get("Location"))
		}
	}
}

// --- 認証済みで詳細表示フルページが 200 ---

func TestIntegration_認証済みで詳細表示が200(t *testing.T) {
	app := showApp()
	cookies := loginCookies(t, app)

	w := getWithCookies(app, "/devices/1", cookies)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /devices/1 = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		"<h1>デバイス詳細: ハウスA温湿度計</h1>",
		`id="device-chart-area"`,
		`id="latest-readings-table"`,
		"24時間", // 期間ボタン
	} {
		if !strings.Contains(body, want) {
			t.Errorf("詳細ページに %q が含まれていない", want)
		}
	}
}

// --- 静的経路 /devices/create とパラメータ経路 /devices/:device の共存 ---

func TestIntegration_createとパラメータ経路が共存解決する(t *testing.T) {
	app := showApp()
	cookies := loginCookies(t, app)

	// 静的 create は登録フォームへ
	cw := getWithCookies(app, "/devices/create", cookies)
	if cw.Code != http.StatusOK || !strings.Contains(cw.Body.String(), "<h1>デバイス登録</h1>") {
		t.Errorf("GET /devices/create = %d, want 200 登録フォーム (静的経路)", cw.Code)
	}
	// パラメータ :device は詳細へ
	dw := getWithCookies(app, "/devices/1", cookies)
	if dw.Code != http.StatusOK || !strings.Contains(dw.Body.String(), "デバイス詳細: ハウスA温湿度計") {
		t.Errorf("GET /devices/1 = %d, want 200 詳細 (パラメータ経路)", dw.Code)
	}
}

// --- 認証済みで期間切替フラグメントが 200 (レイアウト無し) ---

func TestIntegration_認証済みで期間切替フラグメントが200(t *testing.T) {
	app := showApp()
	cookies := loginCookies(t, app)

	req := httptest.NewRequest(http.MethodGet, "/devices/1/chart?period=7d", nil)
	req.Header.Set("HX-Request", "true")
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /devices/1/chart = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "<html") {
		t.Errorf("フラグメントに <html> が含まれている (レイアウトが付与):\n%s", body)
	}
	if strings.Contains(body, "latest-readings-table") {
		t.Error("フラグメントに latest-readings-table が含まれている (期間非連動のはず)")
	}
	if !strings.Contains(body, "7日間") {
		t.Error("期間ボタン 7日間 が含まれていない")
	}
}

// --- 削除: HTMX DELETE と 非HTMX フォーム(_method=delete) が同一ハンドラへ到達 ---

func TestIntegration_HTMX_DELETEは200とHX_Redirect(t *testing.T) {
	app := showApp()
	cookies := loginCookies(t, app)
	token, cookies := authedCSRFToken(t, app, cookies)

	w := deleteWithCookies(app, "/devices/1", map[string]string{
		"X-CSRF-Token": token,
		"HX-Request":   "true",
	}, cookies)

	if w.Code != http.StatusOK {
		t.Fatalf("HTMX DELETE /devices/1 = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("HX-Redirect"); loc != "/dashboard" {
		t.Errorf("HX-Redirect=%q, want /dashboard", loc)
	}
}

func TestIntegration_非HTMXフォームDELETEは303でダッシュボードへ(t *testing.T) {
	app := showApp()
	cookies := loginCookies(t, app)
	token, cookies := authedCSRFToken(t, app, cookies)

	// POST /devices/1 + _method=delete → MethodOverride で DELETE /devices/:device に解決
	form := url.Values{
		"_method":            {"delete"},
		"gorilla.csrf.Token": {token},
	}
	w := postFormWithCookies(app, "/devices/1", form, cookies)

	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/dashboard" {
		t.Fatalf("_method=delete の POST /devices/1 = %d Location=%q, want 303 /dashboard (body=%s)",
			w.Code, w.Header().Get("Location"), w.Body.String())
	}
}

func TestIntegration_CSRFトークン欠落のDELETEは419(t *testing.T) {
	app := showApp()
	cookies := loginCookies(t, app)

	// トークン無しの HTMX DELETE → gorilla/csrf が拒否。CSRF 失敗は BOLA 認可拒否 (403) と
	// 区別するため 419 (middleware.StatusCSRFExpired) を返す。
	w := deleteWithCookies(app, "/devices/1", map[string]string{"HX-Request": "true"}, cookies)
	if w.Code != 419 { // middleware.StatusCSRFExpired
		t.Errorf("トークン欠落 DELETE /devices/1 = %d, want 419", w.Code)
	}
}
