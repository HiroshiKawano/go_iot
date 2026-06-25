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
