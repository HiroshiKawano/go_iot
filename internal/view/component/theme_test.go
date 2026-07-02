package component

import "testing"

// ThemeInit の出力 script は、保存テーマが 'dark'/'light' のときのみ
// document.documentElement.dataset.theme を設定し、それ以外 (未選択・破損値・
// localStorage 例外) では何もしない (OS の prefers-color-scheme 追従へフォールバック・
// R1.4/2.7)。契約を文字列アサートで固定する。
func TestThemeInit_保存値をenum検証してdataset_themeへ同期(t *testing.T) {
	html := render(t, ThemeInit())

	assertContains(t, html, "<script>")
	assertContains(t, html, "localStorage.getItem('theme')")
	assertContains(t, html, "'dark'")
	assertContains(t, html, "'light'")
	assertContains(t, html, "try")
	assertContains(t, html, "catch")
	assertContains(t, html, "dataset.theme")
}

// ThemeToggle はライト/ダーク2状態の明示切替ボタン。キーボード操作可能なネイティブ
// button・aria-pressed の初期値と Alpine バインド・視覚的に隠したラベル・
// アイコン span 2個を持つ (R2.1/2.6)。サーバへは一切通信しない (fetch/hx-* なし・R2.5)。
func TestThemeToggle_マークアップ契約とサーバ非通信(t *testing.T) {
	html := render(t, ThemeToggle())

	assertContains(t, html, `<button type="button" class="theme-toggle"`)
	assertContains(t, html, `aria-pressed="false"`)
	assertContains(t, html, `:aria-pressed`)
	assertContains(t, html, `class="u-visually-hidden">ダークモード切替`)
	assertContains(t, html, `class="icon-moon"`)
	assertContains(t, html, `class="icon-sun"`)

	assertNotContains(t, html, "fetch(")
	assertNotContains(t, html, "hx-")
}

// クリック時: dark 反転 → localStorage 保存 (try/catch で保存不可でも継続・R2.7) →
// data-theme 更新 → themechange イベント発火 (R2.2/2.3)。x-data 初期値は
// data-theme 属性優先→matchMedia フォールバック (R1.1 と整合)。
func TestThemeToggle_クリック処理と初期状態解決(t *testing.T) {
	html := render(t, ThemeToggle())

	assertContains(t, html, "x-data=")
	assertContains(t, html, "document.documentElement.dataset.theme")
	assertContains(t, html, "matchMedia('(prefers-color-scheme: dark)').matches")

	assertContains(t, html, "@click=")
	assertContains(t, html, "localStorage.setItem('theme'")
	assertContains(t, html, "try")
	assertContains(t, html, "catch (e) {}")
	assertContains(t, html, "new CustomEvent('themechange')")
	assertContains(t, html, "document.dispatchEvent(")
}
