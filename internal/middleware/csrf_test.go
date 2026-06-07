package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
)

func TestCSRFAuthKey_常に32バイト(t *testing.T) {
	for _, s := range []string{"", "short", strings.Repeat("x", 64)} {
		if got := len(csrfAuthKey(s)); got != 32 {
			t.Errorf("csrfAuthKey(%q) の長さ = %d, want 32", s, got)
		}
	}
	if !bytes.Equal(csrfAuthKey("abc"), csrfAuthKey("abc")) {
		t.Error("同一 secret から異なる鍵が導出された (決定的でない)")
	}
	if bytes.Equal(csrfAuthKey("a"), csrfAuthKey("b")) {
		t.Error("異なる secret から同一鍵が導出された")
	}
}

func newCSRFRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{AppEnv: "development", SessionSecret: "test-secret-for-csrf-key-derivation"}
	r := gin.New()
	grp := r.Group("/", CSRF(cfg))
	grp.GET("/form", func(c *gin.Context) { c.String(http.StatusOK, csrf.Token(c.Request)) })
	grp.POST("/submit", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func TestCSRF_トークン無しのPOSTは403(t *testing.T) {
	r := newCSRFRouter()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/submit", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("トークン無し POST: got %d, want 403", w.Code)
	}
}

func TestCSRF_GETでトークン取得しPOSTが通過(t *testing.T) {
	r := newCSRFRouter()

	// 1. GET でトークンと cookie を取得 (GET は安全メソッドなので通過)
	gw := httptest.NewRecorder()
	r.ServeHTTP(gw, httptest.NewRequest(http.MethodGet, "/form", nil))
	if gw.Code != http.StatusOK {
		t.Fatalf("GET /form: got %d", gw.Code)
	}
	token := gw.Body.String()
	cookies := gw.Result().Cookies()
	if token == "" || len(cookies) == 0 {
		t.Fatalf("トークン/cookie が取得できない: token=%q cookies=%d", token, len(cookies))
	}

	// 2. トークン (ヘッダ) + cookie 付きで POST → 通過
	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	req.Header.Set("X-CSRF-Token", token)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("トークン付き POST: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
}
