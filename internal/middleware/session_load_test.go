package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
)

// sessionLoadRouter は LoadAndSave + SessionLoad を適用し、
// /login でセッションに user_id を保存、/whoami で auth.UserID(c) を返す。
func sessionLoadRouter(sm *scs.SessionManager) http.Handler {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SessionLoad(sm))
	r.POST("/login", func(c *gin.Context) {
		_ = auth.Login(c.Request.Context(), sm, 42, false)
		c.Status(http.StatusOK)
	})
	r.GET("/whoami", func(c *gin.Context) {
		c.String(http.StatusOK, "%d", auth.UserID(c))
	})
	return sm.LoadAndSave(r)
}

func TestSessionLoad_セッションのuser_idをコンテキストへ橋渡し(t *testing.T) {
	sm := scs.New()
	handler := sessionLoadRouter(sm)

	// 1. login でセッション確立 → cookie 取得
	lw := httptest.NewRecorder()
	handler.ServeHTTP(lw, httptest.NewRequest(http.MethodPost, "/login", nil))
	cookies := lw.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login 後にセッション cookie が発行されていない")
	}

	// 2. cookie 付き whoami → SessionLoad が user_id を橋渡しする
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Body.String(); got != "42" {
		t.Errorf("auth.UserID(c) = %q, want 42 (橋渡し失敗)", got)
	}
}

func TestSessionLoad_未ログインは0のまま(t *testing.T) {
	sm := scs.New()
	handler := sessionLoadRouter(sm)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/whoami", nil))

	if got := w.Body.String(); got != "0" {
		t.Errorf("未ログイン時 auth.UserID(c) = %q, want 0", got)
	}
}
