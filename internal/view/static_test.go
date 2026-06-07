package view

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// CSSURL はキャッシュバスティング用のバージョンクエリ付き URL を返すこと。
func TestCSSURL(t *testing.T) {
	got := CSSURL()
	const wantPrefix = "/static/css/style.css?v="
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("CSSURL() = %q, want prefix %q", got, wantPrefix)
	}
	if got == wantPrefix {
		t.Fatalf("CSSURL() にバージョン値が含まれていない: %q", got)
	}
}

// MountStatic は go:embed した public 配下を /static で配信し、CSS を 200 で返すこと。
func TestMountStaticServesCSS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	MountStatic(r)

	req := httptest.NewRequest(http.MethodGet, "/static/css/style.css", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /static/css/style.css = %d, want 200", w.Code)
	}
	if w.Body.Len() == 0 {
		t.Fatal("CSS のレスポンスボディが空")
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "css") {
		t.Errorf("Content-Type = %q, want css を含む", ct)
	}
}

// バージョンクエリが付いた URL でもファイルが配信されること (クエリは無視される)。
func TestMountStaticServesVersionedURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	MountStatic(r)

	req := httptest.NewRequest(http.MethodGet, CSSURL(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", CSSURL(), w.Code)
	}
}
