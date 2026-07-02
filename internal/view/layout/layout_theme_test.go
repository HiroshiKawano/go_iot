package layout

import (
	"os"
	"strings"
	"testing"
)

// layout_theme_test.go は FOUC 防止スクリプト (ThemeInit) の配置順序と、
// テーマ切替トグル (ThemeToggle) の設置を App/Guest 両レイアウトで検証する。

// App の <head> は ThemeInit の script を <link rel="stylesheet"> より前に出力すること
// (FOUC なし・R1.3)。SiteHeader 経由の .user-menu 内にトグルが存在すること (R2.1/1.5)。
func TestApp_ThemeInitがlink前_ThemeToggleがuser_menu内(t *testing.T) {
	data := AppLayoutData{
		Title:     "ダッシュボード - 農業IoTシステム",
		UserName:  "テストユーザー",
		CSRFToken: "tok-theme",
		CSSURL:    "/static/css/style.css?v=dev",
	}
	html := render(t, App(data))

	linkIdx := strings.Index(html, `<link rel="stylesheet"`)
	themeInitIdx := strings.Index(html, "localStorage.getItem('theme')")
	if linkIdx < 0 {
		t.Fatalf("<link rel=\"stylesheet\"> が出力に無い")
	}
	if themeInitIdx < 0 {
		t.Fatalf("ThemeInit の script が出力に無い")
	}
	if themeInitIdx >= linkIdx {
		t.Errorf("ThemeInit が <link rel=\"stylesheet\"> より後に出現している (FOUC が起きる): themeInitIdx=%d, linkIdx=%d", themeInitIdx, linkIdx)
	}

	assertContains(t, html, `class="theme-toggle"`)
	userMenuIdx := strings.Index(html, `class="user-menu"`)
	toggleIdx := strings.Index(html, `class="theme-toggle"`)
	if userMenuIdx < 0 {
		t.Fatalf(".user-menu が出力に無い")
	}
	if toggleIdx < userMenuIdx {
		t.Errorf("theme-toggle が user-menu より前に出現している (user-menu 内配置ではない)")
	}
}

// Guest の <head> は ThemeInit の script を <link rel="stylesheet"> より前に出力すること
// (FOUC なし・R1.3)。.guest-theme-toggle ラッパ内にトグルが存在すること (R2.1/1.5)。
func TestGuest_ThemeInitがlink前_ThemeToggleがguest_theme_toggle内(t *testing.T) {
	html := render(t, Guest("ログイン - 農業IoTシステム", "/static/css/style.css?v=dev"))

	linkIdx := strings.Index(html, `<link rel="stylesheet"`)
	themeInitIdx := strings.Index(html, "localStorage.getItem('theme')")
	if linkIdx < 0 {
		t.Fatalf("<link rel=\"stylesheet\"> が出力に無い")
	}
	if themeInitIdx < 0 {
		t.Fatalf("ThemeInit の script が出力に無い")
	}
	if themeInitIdx >= linkIdx {
		t.Errorf("ThemeInit が <link rel=\"stylesheet\"> より後に出現している (FOUC が起きる): themeInitIdx=%d, linkIdx=%d", themeInitIdx, linkIdx)
	}

	assertContains(t, html, `class="guest-theme-toggle"`)
	assertContains(t, html, `class="theme-toggle"`)
	wrapperIdx := strings.Index(html, `class="guest-theme-toggle"`)
	toggleIdx := strings.Index(html, `class="theme-toggle"`)
	if toggleIdx < wrapperIdx {
		t.Errorf("theme-toggle が guest-theme-toggle より前に出現している (ラッパ内配置ではない)")
	}
}

// EChartsInitializer に isDarkTheme/buildChromePatch ヘルパと、themechange リスナ・
// matchMedia change リスナが存在すること (1.2, 5.1-5.3)。dispose→init方式ではなく
// setOption マージであること (状態保持・5.2)、既存 tooltip.formatter 等の関数プロパティに
// 触れないこと (既存機能無回帰・7.2) を断片文字列で固定する。
func TestApp_EChartsThemePatchのヘルパとリスナが存在する(t *testing.T) {
	data := AppLayoutData{
		Title:     "デバイス詳細 - 農業IoTシステム",
		UserName:  "テストユーザー",
		CSRFToken: "tok-patch",
		CSSURL:    "/static/css/style.css?v=dev",
	}
	html := render(t, App(data))

	for _, marker := range []string{
		"function isDarkTheme",
		"document.documentElement.dataset.theme",
		"matchMedia('(prefers-color-scheme: dark)')",
		"function buildChromePatch",
		"Array.isArray",
		"document.addEventListener('themechange'",
	} {
		assertContains(t, html, marker)
	}

	// 適用①: initContainer 内で inst.setOption(option) の直後に buildChromePatch をマージする。
	setOptionIdx := strings.Index(html, "inst.setOption(option);")
	if setOptionIdx < 0 {
		t.Fatalf("inst.setOption(option); が出力に無い")
	}
	afterSetOption := html[setOptionIdx:]
	patchCallIdx := strings.Index(afterSetOption, "buildChromePatch(option, isDarkTheme())")
	if patchCallIdx < 0 {
		t.Fatalf("initContainer 内に buildChromePatch(option, isDarkTheme()) の適用が無い")
	}
	// 既存の `return inst;` より前 (同一 initContainer 内) にあること。
	returnIdx := strings.Index(afterSetOption, "return inst;")
	if returnIdx >= 0 && patchCallIdx > returnIdx {
		t.Errorf("buildChromePatch の適用が initContainer の return より後にある")
	}

	// 適用②: themechange で全 [data-echarts] インスタンスへ setOption マージ (dispose→init はしない)。
	assertContains(t, html, "buildChromePatch(inst.getOption(), isDarkTheme())")

	// matchMedia change リスナは data-theme 未設定時のみ themechange を dispatch する。
	assertContains(t, html, "!document.documentElement.dataset.theme")
	assertContains(t, html, "new CustomEvent('themechange')")

	// 既存の tooltip.formatter 等の関数プロパティ加工ロジックは無変更のまま残っている (回帰なし)。
	assertContains(t, html, "seriesName === '帯下限'")
	assertContains(t, html, "toFixed(2)")
}

// App.templ・static.go の ECharts バージョンコメントは実バンドル (5.4.4) を指すこと。
// コメントは templ レンダリング結果には出力されないため (Go コメントはコンパイル時に消える)、
// ソースファイルを直接読んで検証する (css_theme_guard_test.go と同様の定石)。
func TestEChartsバージョンコメントが実バンドルと一致(t *testing.T) {
	for _, path := range []string{"App.templ", "../static.go"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s の読込に失敗: %v", path, err)
		}
		src := string(b)
		if strings.Contains(src, "ECharts 5.4.3") {
			t.Errorf("%s の ECharts バージョンコメントが旧バージョン(5.4.3)のまま", path)
		}
		if !strings.Contains(src, "ECharts 5.4.4") {
			t.Errorf("%s に ECharts 5.4.4 のバージョンコメントが無い", path)
		}
	}
}
