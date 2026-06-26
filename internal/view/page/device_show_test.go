package page

import (
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/view/component"
	"github.com/HiroshiKawano/go_iot/internal/view/layout"
)

func baseDeviceShowView() DeviceShowView {
	return DeviceShowView{
		Layout:   layout.AppLayoutData{Title: "デバイス詳細", UserName: "テストユーザー", CSRFToken: "tk", CSSURL: "/x.css"},
		DeviceID: 1,
		Info: component.DeviceInfoView{
			Name:         "ハウスA温湿度計",
			MacAddress:   "AA:BB:CC:DD:EE:01",
			Location:     "ビニールハウスA",
			StatusActive: true,
			LastCommText: "2026-04-20 14:30:00",
			EditURL:      "/devices/1/edit",
		},
		ChartArea: component.DeviceChartAreaView{
			DeviceID:              1,
			Period:                "24h",
			HasData:               true,
			TemperatureOptionJSON: `{"series":[{"type":"line"}]}`,
			HumidityOptionJSON:    `{"series":[{"type":"line"}]}`,
			TemperatureUnit:       "℃",
			HumidityUnit:          "%",
			TemperatureColor:      "#e8590c",
			HumidityColor:         "#1971c2",
		},
		Latest: component.LatestReadingsView{
			DeviceID: 1,
			Rows:     []component.ReadingRow{{RecordedAt: "2026-04-20 14:30", Temp: "28.50", Humidity: "65.30"}},
		},
		DeleteName: "ハウスA温湿度計",
	}
}

func TestDeviceShowPage_見出しと3部品を統合描画(t *testing.T) {
	html := render(t, DeviceShowPage(baseDeviceShowView()))

	// App レイアウト継承（ヘッダーのユーザー名・メイン枠・csrf meta）
	assertContains(t, html, "テストユーザー")
	assertContains(t, html, `id="main-content"`)
	assertContains(t, html, `name="csrf-token"`)

	// ページ見出しにデバイス名（R1.4）
	assertContains(t, html, "<h1>デバイス詳細: ハウスA温湿度計</h1>")

	// 情報パネル（3.1）
	assertContains(t, html, "device-info")
	assertContains(t, html, "AA:BB:CC:DD:EE:01")

	// グラフ領域: ラッパー #device-chart-area は DeviceShow が提供し、内側に DeviceChartArea
	assertContains(t, html, `id="device-chart-area"`)
	assertContains(t, html, `id="temperature-chart"`)
	assertContains(t, html, `id="humidity-chart"`)
	assertContains(t, html, `<script type="application/json" id="temperature-chart-option">`)
	assertContains(t, html, "24時間")

	// 最新計測テーブル（3.2）
	assertContains(t, html, `id="latest-readings-table"`)
	assertContains(t, html, "2026-04-20 14:30")
	assertContains(t, html, "28.50")
}

// innerOfDeviceChartArea は <div id="device-chart-area"> の中身を div の対応を取って抽出する。
// 期間切替は innerHTML swap でこの範囲を丸ごと差し替えるため、範囲内に何があるかが重要。
func innerOfDeviceChartArea(t *testing.T, html string) string {
	t.Helper()
	const openTag = `<div id="device-chart-area">`
	open := strings.Index(html, openTag)
	if open < 0 {
		t.Fatal("device-chart-area の開始 div が見つからない")
	}
	rest := html[open+len(openTag):]
	depth, i := 1, 0
	for i < len(rest) {
		o := strings.Index(rest[i:], "<div")
		c := strings.Index(rest[i:], "</div>")
		if c < 0 {
			t.Fatal("device-chart-area の閉じ div が見つからない")
		}
		if o >= 0 && o < c {
			depth++
			i += o + len("<div")
			continue
		}
		depth--
		if depth == 0 {
			return rest[:i+c]
		}
		i += c + len("</div>")
	}
	t.Fatal("device-chart-area に対応する閉じ div が見つからない")
	return ""
}

// R3.4/R5.4: 最新計測テーブルは期間切替に連動しない。期間切替は #device-chart-area の
// innerHTML を差し替えるため、テーブルがこの範囲内にあると切替時に消えてしまう。
// テーブルが swap 範囲の「外」にあること（＝据え置かれること）を構造で固定する。
func TestDeviceShowPage_テーブルはグラフ領域swap範囲外にある(t *testing.T) {
	html := render(t, DeviceShowPage(baseDeviceShowView()))
	inner := innerOfDeviceChartArea(t, html)

	if strings.Contains(inner, "latest-readings-table") {
		t.Errorf("latest-readings-table が device-chart-area の swap 範囲内にある (期間切替で消える):\n%s", inner)
	}
	// サニティ: グラフ自体は swap 範囲内にある
	if !strings.Contains(inner, `id="temperature-chart"`) || !strings.Contains(inner, `id="humidity-chart"`) {
		t.Errorf("グラフが device-chart-area 範囲内に無い:\n%s", inner)
	}
	// テーブルはページ全体には存在する (範囲外に据え置き)
	if !strings.Contains(html, `id="latest-readings-table"`) {
		t.Error("ページに latest-readings-table が無い")
	}
}

func TestDeviceShowPage_削除モーダルが同一xdataスコープ内(t *testing.T) {
	html := render(t, DeviceShowPage(baseDeviceShowView()))

	// 削除確認モーダル: 対象デバイス名 + 注意書き（R6.2）
	assertContains(t, html, "modal-overlay")
	assertContains(t, html, `x-show="deleteModalOpen"`)
	assertContains(t, html, "ハウスA温湿度計")
	assertContains(t, html, "※この操作は取り消せません。計測データも削除されます。")

	// キャンセル（削除しない）/ 確認（hx-delete）ボタン
	assertContains(t, html, `@click="deleteModalOpen = false"`)
	assertContains(t, html, "キャンセル")
	assertContains(t, html, `hx-delete="/devices/1"`)
	assertContains(t, html, "削除する")

	// 削除を開くボタン（情報パネル）とモーダルが同一 x-data スコープ内（§62 スコープ外回避）
	assertContains(t, html, `x-data="{ deleteModalOpen: false }"`)
	xdata := strings.Index(html, `x-data="{ deleteModalOpen`)
	openBtn := strings.Index(html, `@click="deleteModalOpen = true"`)
	modal := strings.Index(html, `x-show="deleteModalOpen"`)
	if xdata < 0 || openBtn < 0 || modal < 0 {
		t.Fatalf("必須要素が欠落 (xdata=%d open=%d modal=%d)", xdata, openBtn, modal)
	}
	if !(xdata < openBtn && xdata < modal) {
		t.Errorf("削除ボタン/モーダルが x-data 宣言より前にある (xdata=%d open=%d modal=%d)", xdata, openBtn, modal)
	}
}
