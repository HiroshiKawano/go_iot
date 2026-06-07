package page

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/view/component"
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

func TestDashboardPage_0件でも見出しと登録リンクとグリッドを描画(t *testing.T) {
	v := DashboardView{
		Layout: layout.AppLayoutData{
			Title:     "ダッシュボード - 農業IoTシステム",
			UserName:  "テストユーザー",
			CSRFToken: "tk",
			CSSURL:    "/static/css/style.css?v=dev",
		},
		// Devices/Alerts ともに 0 件
	}
	html := render(t, DashboardPage(v))

	// App レイアウト継承
	assertContains(t, html, "テストユーザー") // ヘッダー経由
	assertContains(t, html, `id="main-content"`)
	// ページ見出し + 登録リンク (h1 限定でサイドバーの「🏠 ダッシュボード」誤検出を避ける)
	assertContains(t, html, "<h1>ダッシュボード</h1>")
	assertContains(t, html, "+ デバイス登録")
	assertContains(t, html, `href="/devices/create"`)
	assertContains(t, html, "デバイス一覧")
	assertContains(t, html, "u-mbe-4")
	// device-grid は 0 件でも常設 (R03)
	assertContains(t, html, `id="device-grid"`)
	// 0 件メッセージ (モック全文)
	assertContains(t, html, "登録されたデバイスはありません。上の「デバイス登録」ボタンから追加してください。")
	// アラート 0 件メッセージ
	assertContains(t, html, `id="unhandled-alert-banner"`)
	assertContains(t, html, "未対応のアラートはありません。")
}

func TestDashboardPage_デバイスとアラートをカード描画(t *testing.T) {
	v := DashboardView{
		Layout: layout.AppLayoutData{Title: "ダッシュボード", UserName: "テストユーザー", CSSURL: "/x.css"},
		Devices: []component.DashboardDevice{
			{
				ID:           1,
				Name:         "ハウスA温湿度計",
				Location:     "ビニールハウスA",
				IsActive:     true,
				TempText:     "28.50℃",
				HumidityText: "65.30%",
				LastCommText: "2分前",
			},
		},
		Alerts: []component.DashboardAlert{
			{Message: "ハウスA温湿度計: 温度が35℃を超えました（38.50℃）"},
		},
	}
	html := render(t, DashboardPage(v))

	assertContains(t, html, `id="device-grid"`)
	assertContains(t, html, `id="device-card-1"`)
	assertContains(t, html, "ハウスA温湿度計")
	assertContains(t, html, "28.50℃")
	assertContains(t, html, `href="/devices/1"`)
	assertContains(t, html, "ハウスA温湿度計: 温度が35℃を超えました（38.50℃）")
	if strings.Contains(html, "登録されたデバイスはありません") {
		t.Error("デバイスありで 0 件メッセージが描画されている")
	}
}
