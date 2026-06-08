package main

import (
	"net/http"
	"strings"
	"testing"
)

// --- 5.1 履歴画面ルーティング配線 (sensor-readings-history) ---

// TestIntegration_未認証の履歴表示GETは302 は、認証必須ルートへ未認証アクセスすると
// /login へ 302 リダイレクトされることを検証する (R8.1 / RequireAuth)。
func TestIntegration_未認証の履歴表示GETは302(t *testing.T) {
	app := showApp()
	w := get(app, "/devices/1/readings")
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/login" {
		t.Errorf("未認証 GET /devices/1/readings = %d Location=%q, want 302 /login",
			w.Code, w.Header().Get("Location"))
	}
}

// TestIntegration_認証済みで履歴初期表示が200 は、認証済みで履歴画面が 200 で描画され、
// 見出し(デバイス名)・結果領域フラグメント・フィルタフォームを含むことを検証する (R1.1)。
func TestIntegration_認証済みで履歴初期表示が200(t *testing.T) {
	app := showApp()
	cookies := loginCookies(t, app)

	w := getWithCookies(app, "/devices/1/readings", cookies)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /devices/1/readings = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		"センサーデータ履歴: ハウスA温湿度計",       // 見出し (デバイス名)
		`id="device-readings-list"`, // 結果領域フラグメント
		"フィルター",                     // フィルタフォーム
		"テストユーザー",                   // App レイアウト (ログインユーザー名)
	} {
		if !strings.Contains(body, want) {
			t.Errorf("履歴ページに %q が含まれていない", want)
		}
	}
}

// TestIntegration_もっと見る導線が履歴画面へ到達する は、デバイス詳細の「もっと見る」リンク先
// (/devices/{id}/readings) が登録され 200 を返すこと（導線が機能すること）を検証する。
func TestIntegration_もっと見る導線が履歴画面へ到達する(t *testing.T) {
	app := showApp()
	cookies := loginCookies(t, app)

	// 詳細画面に「もっと見る」リンク (履歴画面 URL) がある。
	sw := getWithCookies(app, "/devices/1", cookies)
	if !strings.Contains(sw.Body.String(), `href="/devices/1/readings"`) {
		t.Fatalf("詳細画面に履歴への導線 href=/devices/1/readings が無い")
	}
	// その URL が 200 を返す（導線が機能）。
	rw := getWithCookies(app, "/devices/1/readings", cookies)
	if rw.Code != http.StatusOK {
		t.Errorf("もっと見る先 /devices/1/readings = %d, want 200", rw.Code)
	}
}

// TestIntegration_readingsは既存device子経路と競合しない は、:device 配下の各経路
// (/devices/1, /edit, /chart, /readings) と静的 /devices/create が各々正しく解決することを検証する。
func TestIntegration_readingsは既存device子経路と競合しない(t *testing.T) {
	app := showApp()
	cookies := loginCookies(t, app)

	cases := []struct {
		path string
		want string
	}{
		{"/devices/create", "<h1>デバイス登録</h1>"},
		{"/devices/1", "デバイス詳細: ハウスA温湿度計"},
		{"/devices/1/edit", "デバイス編集: ハウスA温湿度計"},
		{"/devices/1/chart?period=24h", "period-selector"},
		{"/devices/1/readings", "センサーデータ履歴: ハウスA温湿度計"},
	}
	for _, tc := range cases {
		w := getWithCookies(app, tc.path, cookies)
		if w.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200 (経路解決ミス?)", tc.path, w.Code)
			continue
		}
		if !strings.Contains(w.Body.String(), tc.want) {
			t.Errorf("GET %s に %q が含まれていない (誤ったハンドラへ解決?)", tc.path, tc.want)
		}
	}
}
