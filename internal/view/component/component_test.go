package component

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

func TestSiteHeader_ユーザー名とログアウトフォームを描画(t *testing.T) {
	html := render(t, SiteHeader("テストユーザー", "tok-123"))
	assertContains(t, html, "site-header")
	assertContains(t, html, "テストユーザー")
	assertContains(t, html, `action="/logout"`)
	assertContains(t, html, `method="post"`)
	assertContains(t, html, `name="gorilla.csrf.Token"`)
	assertContains(t, html, "tok-123")
	assertContains(t, html, "nav-toggle")
}

func TestSidebar_ナビゲーションリンクを描画(t *testing.T) {
	html := render(t, Sidebar())
	assertContains(t, html, `href="/dashboard"`)
	assertContains(t, html, `href="/alerts/rules"`)
	assertContains(t, html, `href="/alerts/history"`)
	assertContains(t, html, "ダッシュボード")
}

func TestFlashMessage_メッセージ未指定でも領域を描画(t *testing.T) {
	html := render(t, FlashMessage(""))
	assertContains(t, html, `id="flash-message"`)
}

func TestFlashMessage_メッセージを表示(t *testing.T) {
	html := render(t, FlashMessage("保存しました"))
	assertContains(t, html, `id="flash-message"`)
	assertContains(t, html, "保存しました")
}
