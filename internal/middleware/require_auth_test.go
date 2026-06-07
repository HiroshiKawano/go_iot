package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/gin-gonic/gin"
)

func TestRequireAuth_未認証は302でloginへ(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/dash", RequireAuth(), func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/dash", nil))

	if w.Code != http.StatusFound {
		t.Fatalf("未認証アクセス: got %d, want 302", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}
}

func TestRequireAuth_認証済は通過(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	setAuthed := func(c *gin.Context) { auth.SetUserID(c, 42); c.Next() }
	r.GET("/dash", setAuthed, RequireAuth(), func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/dash", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("認証済アクセス: got %d, want 200", w.Code)
	}
}
