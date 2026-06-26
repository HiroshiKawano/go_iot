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

// 配信 CSS にグラフコンテナ (ECharts マウント先 #temperature-chart/#humidity-chart) の
// 明示 px 高さ規則が存在すること (R9.1)。ECharts は高さ 0 のコンテナに描画できないため、
// 器クラス .chart-wrapper 直下の div へ明示高さを与える (id/data-* はスタイル非使用方針)。
// CSS 正本は mocks/html/style.css で、make sync-css が public へ複製する → 配信物で検証する。
func TestMountStaticChartContainerHasExplicitHeight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	MountStatic(r)

	req := httptest.NewRequest(http.MethodGet, "/static/css/style.css", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET style.css = %d, want 200", w.Code)
	}
	css := w.Body.String()

	const selector = ".chart-wrapper > div"
	idx := strings.Index(css, selector)
	if idx < 0 {
		t.Fatalf("CSS に %q の規則が無い (ECharts コンテナが 0 高さになる・R9.1)", selector)
	}
	// 規則ブロック { ... } 内に px 単位の明示高さがあること。
	block := css[idx:]
	if end := strings.Index(block, "}"); end >= 0 {
		block = block[:end]
	}
	if !strings.Contains(block, "height") || !strings.Contains(block, "px") {
		t.Errorf("%q に明示 px 高さが無い: %q", selector, block)
	}
}

// JSURL は ECharts スクリプトのバージョンクエリ付き URL を返すこと。
func TestJSURL(t *testing.T) {
	got := JSURL()
	const wantPrefix = "/static/js/echarts.min.js?v="
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("JSURL() = %q, want prefix %q", got, wantPrefix)
	}
	if got == wantPrefix {
		t.Fatalf("JSURL() にバージョン値が含まれていない: %q", got)
	}
}

// MountStatic は go:embed した public 配下の echarts.min.js を /static で 200 配信すること。
func TestMountStaticServesJS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	MountStatic(r)

	req := httptest.NewRequest(http.MethodGet, "/static/js/echarts.min.js", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /static/js/echarts.min.js = %d, want 200", w.Code)
	}
	if w.Body.Len() == 0 {
		t.Fatal("echarts.min.js のレスポンスボディが空")
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("Content-Type = %q, want javascript を含む", ct)
	}
}

// バージョンクエリが付いた JSURL でもファイルが配信されること (クエリは無視される)。
func TestMountStaticServesVersionedJSURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	MountStatic(r)

	req := httptest.NewRequest(http.MethodGet, JSURL(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", JSURL(), w.Code)
	}
}
