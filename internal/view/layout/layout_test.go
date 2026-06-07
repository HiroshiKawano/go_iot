package layout

import (
	"bytes"
	"context"
	"strings"
	"testing"

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

func TestApp_共通要素を描画(t *testing.T) {
	data := AppLayoutData{
		Title:     "ダッシュボード - 農業IoTシステム",
		UserName:  "テストユーザー",
		CSRFToken: "tok-xyz",
		CSSURL:    "/static/css/style.css?v=dev",
	}
	html := render(t, App(data))

	assertContains(t, html, `id="main-content"`)           // HTMX 差し替えターゲット
	assertContains(t, html, `name="csrf-token"`)           // meta tag
	assertContains(t, html, "tok-xyz")                     // csrf トークン値
	assertContains(t, html, `id="flash-message"`)          // 共通通知領域
	assertContains(t, html, "/static/css/style.css?v=dev") // CSSURL
	assertContains(t, html, "テストユーザー")                     // SiteHeader 経由
	assertContains(t, html, "htmx:configRequest")          // CSRF ヘッダ自動付与
	assertContains(t, html, `x-data="{ navOpen: false }"`)
}

func TestGuest_カードでchildrenを描画(t *testing.T) {
	var buf bytes.Buffer
	ctx := templ.WithChildren(context.Background(), templ.Raw("<p>子要素</p>"))
	if err := Guest("ログイン - 農業IoTシステム", "/static/css/style.css?v=dev").Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()

	assertContains(t, html, "guest-layout")
	assertContains(t, html, "guest-card")
	assertContains(t, html, "/static/css/style.css?v=dev")
	assertContains(t, html, "<title>ログイン - 農業IoTシステム</title>")
	assertContains(t, html, "子要素")
}
