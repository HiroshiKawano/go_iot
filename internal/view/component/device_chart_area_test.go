package component

import (
	"strings"
	"testing"
)

// buttonFor は label を内容に持つ <button> タグのセグメント（属性部分）を返す。
// 各期間ボタンの active 付与を個別検証するために使う。
func buttonFor(html, label string) string {
	for _, p := range strings.Split(html, "<button")[1:] {
		end := strings.Index(p, "</button>")
		if end < 0 {
			continue
		}
		if seg := p[:end]; strings.Contains(seg, label) {
			return seg
		}
	}
	return ""
}

func TestDeviceChartArea_期間7dでactiveとidとHTMX属性(t *testing.T) {
	v := DeviceChartAreaView{
		DeviceID:       12,
		Period:         "7d",
		TemperatureSVG: `<svg id="temp-svg"></svg>`,
		HumiditySVG:    `<svg id="humid-svg"></svg>`,
	}
	html := render(t, DeviceChartArea(v))

	// 期間ボタンは <button type="button">（<a> ではない）と 4 ラベル
	assertContains(t, html, `<button type="button"`)
	for _, label := range []string{"24時間", "3日間", "7日間", "30日間"} {
		assertContains(t, html, label)
	}

	// active は 7日間 のみ（24時間/3日間/30日間 には付かない）
	if seg := buttonFor(html, "7日間"); !strings.Contains(seg, "active") {
		t.Errorf("7日間ボタンに active が付いていない: %q", seg)
	}
	for _, other := range []string{"24時間", "3日間", "30日間"} {
		if seg := buttonFor(html, other); strings.Contains(seg, "active") {
			t.Errorf("%sボタンに active が付いている: %q", other, seg)
		}
	}
	if got := strings.Count(html, "period-btn active"); got != 1 {
		t.Errorf(`"period-btn active" の数 = %d, want 1`+"\n%s", got, html)
	}

	// HTMX 属性: フラグメント取得 + #device-chart-area を innerHTML swap + フルページ URL を push
	assertContains(t, html, `hx-get="/devices/12/chart?period=7d"`)
	assertContains(t, html, `hx-target="#device-chart-area"`)
	assertContains(t, html, `hx-swap="innerHTML"`)
	assertContains(t, html, `hx-push-url="/devices/12?period=7d"`)

	// グラフ領域 id（HTMX 専用 id）と SVG の @templ.Raw 埋め込み（エスケープされない生 SVG）
	assertContains(t, html, `id="temperature-chart"`)
	assertContains(t, html, `id="humidity-chart"`)
	assertContains(t, html, `<svg id="temp-svg"></svg>`)
	assertContains(t, html, `<svg id="humid-svg"></svg>`)

	// 最新計測テーブルは差し替え対象外（フラグメントに含めない）
	if strings.Contains(html, "latest-readings-table") {
		t.Errorf("グラフ領域フラグメントに latest-readings-table が含まれている:\n%s", html)
	}
}

func TestDeviceChartArea_デフォルト24hがactive(t *testing.T) {
	html := render(t, DeviceChartArea(DeviceChartAreaView{DeviceID: 3, Period: "24h"}))

	if seg := buttonFor(html, "24時間"); !strings.Contains(seg, "active") {
		t.Errorf("24時間ボタンに active が付いていない: %q", seg)
	}
	if got := strings.Count(html, "period-btn active"); got != 1 {
		t.Errorf(`"period-btn active" の数 = %d, want 1`, got)
	}
	assertContains(t, html, `hx-get="/devices/3/chart?period=24h"`)
}

func TestDeviceChartArea_表示形式トグルとデフォルトはラインactive(t *testing.T) {
	// View 未設定 (既定の折れ線) でも表示形式トグルが描画され、ライン側が active になる。
	html := render(t, DeviceChartArea(DeviceChartAreaView{DeviceID: 5, Period: "24h"}))

	assertContains(t, html, "chart-type-selector")
	assertContains(t, html, "ライン")
	assertContains(t, html, "ローソク足")

	// 既定 (View 未設定=折れ線) は ライン が active・ローソク足 は非 active
	if seg := buttonFor(html, "ライン"); !strings.Contains(seg, "active") {
		t.Errorf("ラインボタンに active が付いていない: %q", seg)
	}
	if seg := buttonFor(html, "ローソク足"); strings.Contains(seg, "active") {
		t.Errorf("ローソク足ボタンに active が付いている: %q", seg)
	}
	// ローソク足トグルは view=candle を取得する HTMX 属性を持つ (templ は属性内の & を &amp; にエスケープ)
	assertContains(t, html, `hx-get="/devices/5/chart?period=24h&amp;view=candle"`)
	// 折れ線なので注記 (30分足…) は出ない
	if strings.Contains(html, "30分足") {
		t.Errorf("折れ線表示なのにローソク足の注記が含まれる:\n%s", html)
	}
}

func TestDeviceChartArea_ローソク足でcandle_activeかつ期間は非activeかつ注記(t *testing.T) {
	html := render(t, DeviceChartArea(DeviceChartAreaView{DeviceID: 9, Period: "24h", View: "candle"}))

	// ローソク足が active・ライン は非 active
	if seg := buttonFor(html, "ローソク足"); !strings.Contains(seg, "active") {
		t.Errorf("ローソク足ボタンに active が付いていない: %q", seg)
	}
	if seg := buttonFor(html, "ライン"); strings.Contains(seg, "active") {
		t.Errorf("ラインボタンに active が付いている: %q", seg)
	}
	// ローソク足は期間に連動しないため、期間ボタンは1つも active にしない
	if got := strings.Count(html, "period-btn active"); got != 0 {
		t.Errorf(`"period-btn active" の数 = %d, want 0`+"\n%s", got, html)
	}
	// 期間セレクタ自体は残す (折れ線へ戻る導線)
	assertContains(t, html, "period-selector")
	// 30分足・直近48時間の注記を表示
	assertContains(t, html, "30分足")
}

func TestDeviceChartArea_期間3dでactiveとHTMX属性(t *testing.T) {
	html := render(t, DeviceChartArea(DeviceChartAreaView{DeviceID: 8, Period: "3d"}))

	// active は 3日間 のみ
	if seg := buttonFor(html, "3日間"); !strings.Contains(seg, "active") {
		t.Errorf("3日間ボタンに active が付いていない: %q", seg)
	}
	for _, other := range []string{"24時間", "7日間", "30日間"} {
		if seg := buttonFor(html, other); strings.Contains(seg, "active") {
			t.Errorf("%sボタンに active が付いている: %q", other, seg)
		}
	}
	if got := strings.Count(html, "period-btn active"); got != 1 {
		t.Errorf(`"period-btn active" の数 = %d, want 1`+"\n%s", got, html)
	}

	// HTMX 属性は 3d クエリでフラグメント取得・フルページ URL を push
	assertContains(t, html, `hx-get="/devices/8/chart?period=3d"`)
	assertContains(t, html, `hx-push-url="/devices/8?period=3d"`)
}
