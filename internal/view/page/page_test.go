package page

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/view/layout"
	"github.com/a-h/templ"
)

func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func assertContains(t *testing.T, html, substr string) {
	t.Helper()
	if !strings.Contains(html, substr) {
		t.Errorf("出力に %q が含まれていない:\n%s", substr, html)
	}
}

func TestLoginPage_フォームとエラーと入力値再表示(t *testing.T) {
	v := LoginView{
		CSSURL:    "/static/css/style.css?v=dev",
		CSRFToken: "tk",
		Email:     "user@example.com",
		Errors:    map[string]string{"email": "メールアドレス形式で入力してください"},
	}
	html := render(t, LoginPage(v))

	assertContains(t, html, "guest-layout")
	assertContains(t, html, `action="/login"`)
	assertContains(t, html, `name="email"`)
	assertContains(t, html, `value="user@example.com"`) // 入力値再表示
	assertContains(t, html, `name="password"`)
	assertContains(t, html, `name="remember"`)
	assertContains(t, html, "メールアドレス形式で入力してください") // フィールドエラー
	assertContains(t, html, `name="gorilla.csrf.Token"`)
	assertContains(t, html, "tk")
}

func TestLoginPage_フォームレベルエラー(t *testing.T) {
	v := LoginView{
		CSRFToken: "tk",
		Errors:    map[string]string{"form": "メールアドレスまたはパスワードが間違っています"},
	}
	html := render(t, LoginPage(v))
	assertContains(t, html, "メールアドレスまたはパスワードが間違っています")
}

func TestRegisterPage_4項目とヘルプとエラー(t *testing.T) {
	v := RegisterView{
		CSSURL:    "/static/css/style.css?v=dev",
		CSRFToken: "tk",
		Name:      "山田太郎",
		Email:     "user@example.com",
		Errors:    map[string]string{"password": "8文字以上で入力してください"},
	}
	html := render(t, RegisterPage(v))

	assertContains(t, html, `action="/register"`)
	assertContains(t, html, `name="name"`)
	assertContains(t, html, `value="山田太郎"`)
	assertContains(t, html, `name="email"`)
	assertContains(t, html, `name="password"`)
	assertContains(t, html, `name="password_confirmation"`)
	assertContains(t, html, "8文字以上")          // form-help
	assertContains(t, html, "8文字以上で入力してください") // エラー
	assertContains(t, html, `name="gorilla.csrf.Token"`)
}

func TestDashboardPage_ユーザー名とプレースホルダ(t *testing.T) {
	data := layout.AppLayoutData{
		Title:     "ダッシュボード - 農業IoTシステム",
		UserName:  "テストユーザー",
		CSRFToken: "tk",
		CSSURL:    "/static/css/style.css?v=dev",
	}
	html := render(t, DashboardPage(data))

	assertContains(t, html, "テストユーザー") // App ヘッダー経由
	assertContains(t, html, `id="main-content"`)
	assertContains(t, html, "ダッシュボード")
	assertContains(t, html, "デバイス一覧")
	assertContains(t, html, "empty-message") // 空状態プレースホルダ
}
