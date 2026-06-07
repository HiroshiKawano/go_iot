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

// fakeUserQuerier は users 関連のみ in-memory で実装し、残りは埋め込み Querier (nil)。
type fakeUserQuerier struct {
	repository.Querier
	byEmail map[string]repository.User
	byID    map[int64]repository.User
}

func (f fakeUserQuerier) GetUserByEmail(_ context.Context, email string) (repository.User, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return repository.User{}, pgx.ErrNoRows
}

func (f fakeUserQuerier) GetUser(_ context.Context, id int64) (repository.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return repository.User{}, pgx.ErrNoRows
}

func extractCSRFToken(html string) string {
	const marker = `name="gorilla.csrf.Token" value="`
	i := strings.Index(html, marker)
	if i < 0 {
		return ""
	}
	rest := html[i+len(marker):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

func mergeCookies(sets ...[]*http.Cookie) []*http.Cookie {
	m := map[string]*http.Cookie{}
	for _, set := range sets {
		for _, ck := range set {
			m[ck.Name] = ck
		}
	}
	out := make([]*http.Cookie, 0, len(m))
	for _, ck := range m {
		out = append(out, ck)
	}
	return out
}

// TestIntegration_CSRF通しのログインフロー は、合成済みハンドラ
// (MethodOverride → LoadAndSave → CSRF → SessionLoad → handler → templ) を
// 1本に通し、CSRF トークン取得 → ログイン → セッションでダッシュボード到達までを検証する。
func TestIntegration_CSRF通しのログインフロー(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	u := repository.User{ID: 1, Name: "テストユーザー", Email: "test@example.com", PasswordHash: string(hash)}
	q := fakeUserQuerier{
		byEmail: map[string]repository.User{"test@example.com": u},
		byID:    map[int64]repository.User{1: u},
	}
	cfg := &config.Config{AppEnv: "development", SessionSecret: "0123456789abcdef0123456789abcdef"}
	sm := scs.New()
	app := newHTTPHandler(cfg, sm, q, func(context.Context) error { return nil })

	// 1. GET /login → CSRF トークンと cookie を取得
	lw := get(app, "/login")
	if lw.Code != http.StatusOK {
		t.Fatalf("GET /login = %d", lw.Code)
	}
	token := extractCSRFToken(lw.Body.String())
	if token == "" {
		t.Fatal("CSRF トークンが hidden input から取得できない")
	}
	getCookies := lw.Result().Cookies()

	// 2. POST /login（トークン + cookie）→ 303 /dashboard + セッション cookie
	form := url.Values{
		"email":              {"test@example.com"},
		"password":           {"password"},
		"gorilla.csrf.Token": {token},
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, ck := range getCookies {
		req.AddCookie(ck)
	}
	pw := httptest.NewRecorder()
	app.ServeHTTP(pw, req)
	if pw.Code != http.StatusSeeOther || pw.Header().Get("Location") != "/dashboard" {
		t.Fatalf("POST /login = %d Location=%q, want 303 /dashboard (body=%s)", pw.Code, pw.Header().Get("Location"), pw.Body.String())
	}

	// 3. GET /dashboard（csrf + session cookie）→ 200 + ユーザー名
	req2 := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	for _, ck := range mergeCookies(getCookies, pw.Result().Cookies()) {
		req2.AddCookie(ck)
	}
	dw := httptest.NewRecorder()
	app.ServeHTTP(dw, req2)
	if dw.Code != http.StatusOK {
		t.Fatalf("GET /dashboard = %d, want 200", dw.Code)
	}
	if !strings.Contains(dw.Body.String(), "テストユーザー") {
		t.Error("ダッシュボードにログインユーザー名が表示されていない")
	}
	if !strings.Contains(dw.Body.String(), `id="main-content"`) {
		t.Error("App レイアウトの #main-content が描画されていない")
	}
}

// TestIntegration_CSRFトークン不正は403 は、正規 cookie でも誤ったトークンなら拒否されることを確認する。
func TestIntegration_CSRFトークン不正は403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{AppEnv: "development", SessionSecret: "0123456789abcdef0123456789abcdef"}
	sm := scs.New()
	app := newHTTPHandler(cfg, sm, stubQuerier{}, func(context.Context) error { return nil })

	lw := get(app, "/login")
	getCookies := lw.Result().Cookies()

	form := url.Values{
		"email":              {"test@example.com"},
		"password":           {"password"},
		"gorilla.csrf.Token": {"invalid-token"},
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, ck := range getCookies {
		req.AddCookie(ck)
	}
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("不正トークン POST /login = %d, want 403", w.Code)
	}
}
