package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/config"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
)

// stubQuerier は repository.Querier を埋め込む (全メソッド nil)。
// 配線テストで実際に呼ばれないメソッドはダミーで足り、DB を必要としない。
type stubQuerier struct {
	repository.Querier
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{AppEnv: "development", SessionSecret: "0123456789abcdef0123456789abcdef"}
	sm := scs.New()
	okPing := func(context.Context) error { return nil }
	return newHTTPHandler(cfg, sm, stubQuerier{}, okPing)
}

func get(app http.Handler, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestNewHTTPHandler_GETログインは200(t *testing.T) {
	w := get(newTestHandler(t), "/login")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /login = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `action="/login"`) {
		t.Error("ログインフォームが描画されていない")
	}
}

func TestNewHTTPHandler_未認証dashboardは302(t *testing.T) {
	w := get(newTestHandler(t), "/dashboard")
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/login" {
		t.Errorf("GET /dashboard = %d Location=%q, want 302 /login", w.Code, w.Header().Get("Location"))
	}
}

func TestNewHTTPHandler_静的CSSは200(t *testing.T) {
	w := get(newTestHandler(t), "/static/css/style.css")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /static/css/style.css = %d, want 200", w.Code)
	}
}

func TestNewHTTPHandler_ヘルスチェックは200(t *testing.T) {
	w := get(newTestHandler(t), "/health")
	if w.Code != http.StatusOK {
		t.Errorf("GET /health = %d, want 200", w.Code)
	}
}

func TestNewHTTPHandler_デバイスAPIは認証なしで401かつCSRF対象外(t *testing.T) {
	app := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/sensor-data", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	// CSRF(403) ではなく DeviceAuth(401) になること = /api が CSRF 対象外である証左
	if w.Code != http.StatusUnauthorized {
		t.Errorf("POST /api/sensor-data (no bearer) = %d, want 401", w.Code)
	}
}

func TestNewHTTPHandler_WebのPOSTはCSRFトークン無しで403(t *testing.T) {
	app := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=a@b.com&password=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("POST /login (no csrf) = %d, want 403", w.Code)
	}
}
