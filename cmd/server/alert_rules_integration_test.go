package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- 5.1 アラートルール管理ルーティング配線 (alert-rules) ---

// TestIntegration_未認証のアラートルールGETは302 は、認証必須の表示系ルート
// (初期表示・編集読込) へ未認証アクセスすると /login へ 302 されることを検証する
// (要件 10.1 / RequireAuth。GET は CSRF 対象外のため RequireAuth が先に判定する)。
func TestIntegration_未認証のアラートルールGETは302(t *testing.T) {
	app := newTestHandler(t)
	for _, path := range []string{"/alerts/rules", "/alerts/rules/1/edit"} {
		w := get(app, path)
		if w.Code != http.StatusFound || w.Header().Get("Location") != "/login" {
			t.Errorf("未認証 GET %s = %d Location=%q, want 302 /login",
				path, w.Code, w.Header().Get("Location"))
		}
	}
}

// TestIntegration_アラートルールのミューテーションはCSRF欠如で403 は、追加/更新/有効切替/削除の
// 各ミューテーションが CSRF トークン欠如で 403 になることを検証する (要件 10.3 / gorilla/csrf)。
// web グループの CSRF ミドルウェアは RequireAuth より先に評価されるため、未認証でも 403 になる。
// 同時に「ルートが解決し 404 でない」= 6 ルートが配線済みであることの証左でもある。
func TestIntegration_アラートルールのミューテーションはCSRF欠如で403(t *testing.T) {
	app := newTestHandler(t)
	cases := []struct {
		method, path string
	}{
		{http.MethodPost, "/alerts/rules"},
		{http.MethodPut, "/alerts/rules/1"},
		{http.MethodPatch, "/alerts/rules/1/toggle"},
		{http.MethodDelete, "/alerts/rules/1"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s %s (CSRF 欠如) = %d, want 403", tc.method, tc.path, w.Code)
		}
	}
}
