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

func TestCSRF_トークン無しのPOSTは419(t *testing.T) {
	r := newCSRFRouter()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/submit", nil))
	// CSRF 失敗は BOLA 認可拒否 (403) と区別するため 419 を返す (StatusCSRFExpired)
	if w.Code != StatusCSRFExpired {
		t.Fatalf("トークン無し POST: got %d, want %d", w.Code, StatusCSRFExpired)
	}
}

func TestCSRF_HTMX失敗は本文なし419_非HTMXは誘導本文あり419(t *testing.T) {
	r := newCSRFRouter()

	// HTMX (HX-Request 有): 本文なし 419 (フロントの 419 分岐がトースト表示)
	hw := httptest.NewRecorder()
	hreq := httptest.NewRequest(http.MethodPost, "/submit", nil)
	hreq.Header.Set("HX-Request", "true")
	r.ServeHTTP(hw, hreq)
	if hw.Code != StatusCSRFExpired {
		t.Fatalf("HTMX CSRF 失敗: got %d, want %d", hw.Code, StatusCSRFExpired)
	}
	if hw.Body.Len() != 0 {
		t.Errorf("HTMX は本文なし 419 のはず: body=%q", hw.Body.String())
	}

	// 非 HTMX フルページ送信: ログイン誘導の本文あり 419
	fw := httptest.NewRecorder()
	r.ServeHTTP(fw, httptest.NewRequest(http.MethodPost, "/submit", nil))
	if fw.Code != StatusCSRFExpired {
		t.Fatalf("非HTMX CSRF 失敗: got %d, want %d", fw.Code, StatusCSRFExpired)
	}
	if fw.Body.Len() == 0 {
		t.Error("非 HTMX は誘導本文を含む 419 のはず")
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
