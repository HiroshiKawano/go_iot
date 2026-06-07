package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// --- GET 表示 / 認証済みリダイレクト ---

func TestLoginGet_未認証はフォーム表示(t *testing.T) {
	app := newAuthApp(&fakeAuthRepo{})
	w := getWithCookies(app, "/login", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `action="/login"`) {
		t.Error("ログインフォームが表示されていない")
	}
}

func TestRegisterGet_未認証はフォーム表示(t *testing.T) {
	app := newAuthApp(&fakeAuthRepo{})
	w := getWithCookies(app, "/register", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `action="/register"`) {
		t.Error("登録フォームが表示されていない")
	}
}

func TestLoginGet_認証済はdashboardへ(t *testing.T) {
	u := userWithPassword(7, "山田太郎", "user@example.com", "password123")
	repo := &fakeAuthRepo{
		byEmail: map[string]repository.User{"user@example.com": u},
		byID:    map[int64]repository.User{7: u},
	}
	app := newAuthApp(repo)
	login := postForm(app, "/login", url.Values{"email": {"user@example.com"}, "password": {"password123"}})

	w := getWithCookies(app, "/login", login.Result().Cookies())
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/dashboard" {
		t.Errorf("認証済 /login = %d Location=%q, want 302 /dashboard", w.Code, w.Header().Get("Location"))
	}
}

func TestRegisterGet_認証済はdashboardへ(t *testing.T) {
	u := userWithPassword(7, "山田太郎", "user@example.com", "password123")
	repo := &fakeAuthRepo{
		byEmail: map[string]repository.User{"user@example.com": u},
		byID:    map[int64]repository.User{7: u},
	}
	app := newAuthApp(repo)
	login := postForm(app, "/login", url.Values{"email": {"user@example.com"}, "password": {"password123"}})

	w := getWithCookies(app, "/register", login.Result().Cookies())
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/dashboard" {
		t.Errorf("認証済 /register = %d Location=%q, want 302 /dashboard", w.Code, w.Header().Get("Location"))
	}
}

// --- DB 障害は 500 ---

func TestLoginPost_DB障害は500(t *testing.T) {
	repo := &fakeAuthRepo{getByEmailErr: errors.New("db down")}
	app := newAuthApp(repo)
	w := postForm(app, "/login", url.Values{"email": {"user@example.com"}, "password": {"password123"}})
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestRegisterPost_重複確認のDB障害は500(t *testing.T) {
	repo := &fakeAuthRepo{getByEmailErr: errors.New("db down")}
	app := newAuthApp(repo)
	w := postForm(app, "/register", url.Values{
		"name":                  {"太郎"},
		"email":                 {"a@example.com"},
		"password":              {"password123"},
		"password_confirmation": {"password123"},
	})
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestRegisterPost_CreateUser障害は500(t *testing.T) {
	repo := &fakeAuthRepo{byEmail: map[string]repository.User{}, createErr: errors.New("insert failed")}
	app := newAuthApp(repo)
	w := postForm(app, "/register", url.Values{
		"name":                  {"太郎"},
		"email":                 {"a@example.com"},
		"password":              {"password123"},
		"password_confirmation": {"password123"},
	})
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestDashboard_GetUser失敗は500(t *testing.T) {
	// login は GetUserByEmail のみ使用。byID を空にして Dashboard の GetUser を失敗させる。
	u := userWithPassword(7, "山田太郎", "user@example.com", "password123")
	repo := &fakeAuthRepo{byEmail: map[string]repository.User{"user@example.com": u}}
	app := newAuthApp(repo)
	login := postForm(app, "/login", url.Values{"email": {"user@example.com"}, "password": {"password123"}})

	w := getWithCookies(app, "/dashboard", login.Result().Cookies())
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}
