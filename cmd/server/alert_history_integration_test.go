package main

import (
	"net/http"
	"strings"
	"testing"
)

// --- 4.1 アラート履歴ルーティング配線 (alert-history) ---

// TestIntegration_未認証のアラート履歴GETは302 は、認証必須ルートへ未認証アクセスすると
// /login へ 302 リダイレクトされることを検証する (R8.2 / RequireAuth)。
func TestIntegration_未認証のアラート履歴GETは302(t *testing.T) {
	app := showApp()
	w := get(app, "/alerts/history")
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/login" {
		t.Errorf("未認証 GET /alerts/history = %d Location=%q, want 302 /login",
			w.Code, w.Header().Get("Location"))
	}
}

// TestIntegration_認証済みでアラート履歴初期表示が200 は、認証済みで履歴画面が 200 で
// フルページ描画され、見出し h1・結果領域フラグメント・フィルタフォーム・デバイス選択肢・
// 0件空状態を含むことを検証する (R8.2 配線・R1.1)。
func TestIntegration_認証済みでアラート履歴初期表示が200(t *testing.T) {
	app := showApp()
	cookies := loginCookies(t, app)

	w := getWithCookies(app, "/alerts/history", cookies)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /alerts/history = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		"<h1>アラート履歴</h1>",         // ページ見出し (サイドバーのリンク文言と区別)
		`id="alert-history-list"`, // 結果領域フラグメント
		"フィルター",                   // フィルタフォーム
		"全デバイス",                   // デバイス選択肢の先頭 option
		"ハウスA温湿度計",                // 本人デバイスが選択肢に出る (ListDevicesByUser)
		"テストユーザー",                 // App レイアウト (ログインユーザー名)
		"指定期間のアラート履歴はありません。", // 0件空状態 (fake は空データ)
	} {
		if !strings.Contains(body, want) {
			t.Errorf("アラート履歴ページに %q が含まれていない", want)
		}
	}
}

// TestIntegration_アラート履歴のサイドバー導線 は、共通サイドバーに /alerts/history を指す
// アラート履歴リンクが配置されていることを検証する (導線・タスク4.1)。
func TestIntegration_アラート履歴のサイドバー導線(t *testing.T) {
	app := showApp()
	cookies := loginCookies(t, app)

	w := getWithCookies(app, "/alerts/history", cookies)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /alerts/history = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `href="/alerts/history"`) {
		t.Error("サイドバーに /alerts/history への導線が無い")
	}
	if !strings.Contains(body, "アラート履歴") {
		t.Error("サイドバーのアラート履歴リンク文言が無い")
	}
}
