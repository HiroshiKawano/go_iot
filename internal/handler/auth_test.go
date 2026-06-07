package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/middleware"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
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
}

func (f *fakeAuthRepo) GetUserByEmail(_ context.Context, email string) (repository.User, error) {
	if f.getByEmailErr != nil {
		return repository.User{}, f.getByEmailErr
	}
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return repository.User{}, pgx.ErrNoRows
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
	return repository.User{}, pgx.ErrNoRows
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
