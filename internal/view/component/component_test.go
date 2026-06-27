package component

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func assertContains(t *testing.T, html, substr string) {
	t.Helper()
	if !strings.Contains(html, substr) {
		t.Errorf("出力に %q が含まれていない:\n%s", substr, html)
	}
}

// assertActiveCount はサイドバー HTML 内の active リンク数を検証する (R2.7 同時 active ≤1)。
func assertActiveCount(t *testing.T, html string, want int) {
	t.Helper()
	if got := strings.Count(html, `class="active"`); got != want {
		t.Errorf(`class="active" の数 = %d, want %d:\n%s`, got, want, html)
	}
}

func TestSiteHeader_ユーザー名とログアウトフォームを描画(t *testing.T) {
	html := render(t, SiteHeader("テストユーザー", "tok-123"))
	assertContains(t, html, "site-header")
	assertContains(t, html, "テストユーザー")
	assertContains(t, html, `action="/logout"`)
	assertContains(t, html, `method="post"`)
	assertContains(t, html, `name="gorilla.csrf.Token"`)
	assertContains(t, html, "tok-123")
	assertContains(t, html, "nav-toggle")
}

func TestSidebar_常時表示3項目とドロワー属性を描画(t *testing.T) {
	// 常時表示項目 (ダッシュボード/アラートルール/アラート履歴) の遷移先と
	// モバイルドロワー開閉属性は、いかなるナビ文脈でも不変 (R4.1/4.2/4.4)。
	html := render(t, Sidebar(SidebarNav{Current: NavDashboard}))
	assertContains(t, html, `href="/dashboard"`)
	assertContains(t, html, `href="/alerts/rules"`)
	assertContains(t, html, `href="/alerts/history"`)
	assertContains(t, html, "ダッシュボード")
	// Alpine の開閉バインディングはサーバ HTML へ属性として出力される
	// (§4.11: 後付け状態クラスでなく、それを駆動する :class バインディングの存在を検証)。
	assertContains(t, html, `:class="{ 'is-open': navOpen }"`)
}

func TestSidebar_ダッシュボード文脈はダッシュボードのみactive(t *testing.T) {
	html := render(t, Sidebar(SidebarNav{Current: NavDashboard}))
	assertContains(t, html, `href="/dashboard" class="active"`) // 現在ページに連動 (R2.1)
	assertActiveCount(t, html, 1)                                // 同時 active ≤1 (R2.7)
	// デバイス文脈を持たない画面では文脈リンクを出さない (R1.3)
	assertNotContains(t, html, "📟 デバイス詳細")
	assertNotContains(t, html, "📈 センサーデータ履歴")
	assertNotContains(t, html, "/devices/")
}

func TestSidebar_アラートルール文脈はアラートルールのみactive(t *testing.T) {
	html := render(t, Sidebar(SidebarNav{Current: NavAlertRules}))
	assertContains(t, html, `href="/alerts/rules" class="active"`) // R2.4
	assertActiveCount(t, html, 1)
	assertNotContains(t, html, "/devices/") // 文脈リンクなし (R1.3)
}

func TestSidebar_アラート履歴文脈はアラート履歴のみactive(t *testing.T) {
	html := render(t, Sidebar(SidebarNav{Current: NavAlertHistory}))
	assertContains(t, html, `href="/alerts/history" class="active"`) // R2.5
	assertActiveCount(t, html, 1)
	assertNotContains(t, html, "/devices/")
}

func TestSidebar_ゼロ値はactiveも文脈リンクも持たない(t *testing.T) {
	// デバイス登録/編集相当: 対応メニュー項目なし=どれも active にしない (R2.6)、
	// 文脈リンクも出さない (R1.3)。ただし常時3項目は描画する (R4.1)。
	html := render(t, Sidebar(SidebarNav{}))
	assertContains(t, html, `href="/dashboard"`)
	assertContains(t, html, `href="/alerts/rules"`)
	assertContains(t, html, `href="/alerts/history"`)
	assertActiveCount(t, html, 0) // active 0件 (ダッシュボード固定 active を再現しない)
	assertNotContains(t, html, "📟 デバイス詳細")
	assertNotContains(t, html, "📈 センサーデータ履歴")
	assertNotContains(t, html, "/devices/")
}

func TestSidebar_デバイス詳細文脈で文脈2リンクと詳細active(t *testing.T) {
	html := render(t, Sidebar(SidebarNav{Current: NavDeviceShow, DeviceID: 42}))
	// 選択中デバイス id を指す文脈2リンクが存在 (R1.1/1.4・同一 id R3.3)
	assertContains(t, html, `href="/devices/42"`)
	assertContains(t, html, `href="/devices/42/readings"`)
	assertContains(t, html, "📟 デバイス詳細")
	assertContains(t, html, "📈 センサーデータ履歴")
	// 📟 デバイス詳細が active、📈 は非 active で存在 (R2.2)
	assertContains(t, html, `href="/devices/42" class="active"`)
	assertNotContains(t, html, `href="/devices/42/readings" class="active"`)
	// ダッシュボードは非 active (active 固定の解消)
	assertNotContains(t, html, `href="/dashboard" class="active"`)
	assertActiveCount(t, html, 1)
}

func TestSidebar_センサーデータ履歴文脈で履歴active(t *testing.T) {
	html := render(t, Sidebar(SidebarNav{Current: NavReadings, DeviceID: 42}))
	assertContains(t, html, `href="/devices/42"`)
	assertContains(t, html, `href="/devices/42/readings"`)
	// 📈 センサーデータ履歴が active、📟 詳細は非 active で存在 (R2.3)
	assertContains(t, html, `href="/devices/42/readings" class="active"`)
	assertNotContains(t, html, `href="/devices/42" class="active"`)
	assertActiveCount(t, html, 1)
}

func TestFlashMessage_メッセージ未指定でも領域を描画(t *testing.T) {
	html := render(t, FlashMessage(""))
	assertContains(t, html, `id="flash-message"`)
}

func TestFlashMessage_メッセージを表示(t *testing.T) {
	html := render(t, FlashMessage("保存しました"))
	assertContains(t, html, `id="flash-message"`)
	assertContains(t, html, "保存しました")
}

func TestDeviceCard_稼働中デバイスの全要素を描画(t *testing.T) {
	d := DashboardDevice{
		ID:           7,
		Name:         "ハウスA温湿度計",
		Location:     "佐敷（南城市）", // 所在地は構造化 locality の認識名 (handler が整形済)
		IsActive:     true,
		TempText:     "28.50℃",
		HumidityText: "65.30%",
		LastCommText: "2分前",
	}
	html := render(t, DeviceCard(d))

	assertContains(t, html, `id="device-card-7"`) // 個別カード id (将来 OOB ターゲット)
	assertContains(t, html, "device-card")
	assertContains(t, html, "ハウスA温湿度計") // 名前
	assertContains(t, html, "場所:")
	assertContains(t, html, "佐敷（南城市）") // 所在地を認識名で表示 (R6.2)
	assertContains(t, html, "status-active")
	assertContains(t, html, "● 稼働中")
	assertContains(t, html, "28.50℃") // 温度表示値
	assertContains(t, html, "65.30%") // 湿度表示値
	assertContains(t, html, "最終通信:")
	assertContains(t, html, "2分前")
	assertContains(t, html, `href="/devices/7"`) // 詳細遷移先
	assertContains(t, html, "詳細を見る")
}

func TestDeviceCard_停止中はstatus_inactiveと停止中表記(t *testing.T) {
	d := DashboardDevice{
		ID:           8,
		Name:         "停止デバイス",
		Location:     "",
		IsActive:     false,
		TempText:     "ー",
		HumidityText: "ー",
		LastCommText: "通信実績なし",
	}
	html := render(t, DeviceCard(d))

	assertContains(t, html, `id="device-card-8"`)
	assertContains(t, html, "status-inactive")
	assertContains(t, html, "○ 停止中")
	assertContains(t, html, "通信実績なし")
	if strings.Contains(html, "● 稼働中") {
		t.Error("停止中デバイスに「● 稼働中」が描画されている")
	}
}

func TestUnhandledAlertBanner_件数ありで見出しと各メッセージ(t *testing.T) {
	alerts := []DashboardAlert{
		{Message: "ハウスA温湿度計: 温度が35℃を超えました（38.50℃）"},
		{Message: "ハウスB温湿度計: 湿度が30%を下回りました（25.00%）"},
	}
	html := render(t, UnhandledAlertBanner(alerts))

	assertContains(t, html, `id="unhandled-alert-banner"`) // OOB ターゲット用 id
	assertContains(t, html, "alert-banner")
	assertContains(t, html, "⚠ 未対応アラート")
	assertContains(t, html, "ハウスA温湿度計: 温度が35℃を超えました（38.50℃）")
	assertContains(t, html, "ハウスB温湿度計: 湿度が30%を下回りました（25.00%）")
	if strings.Contains(html, "未対応のアラートはありません。") {
		t.Error("件数ありで空メッセージが描画されている")
	}
}

func TestUnhandledAlertBanner_0件で空メッセージ(t *testing.T) {
	html := render(t, UnhandledAlertBanner(nil))

	assertContains(t, html, `id="unhandled-alert-banner"`) // ラッパーは0件でも残す
	assertContains(t, html, "未対応のアラートはありません。")
	if strings.Contains(html, "⚠ 未対応アラート") {
		t.Error("0件で未対応アラート見出しが描画されている")
	}
}
