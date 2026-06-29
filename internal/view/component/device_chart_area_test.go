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
		DeviceID:              12,
		Period:                "7d",
		HasData:               true,
		TemperatureOptionJSON: `{"series":[{"type":"line"}]}`,
		HumidityOptionJSON:    `{"series":[{"type":"line"}]}`,
		TemperatureUnit:       "℃",
		HumidityUnit:          "%",
		TemperatureColor:      "#e8590c",
		HumidityColor:         "#1971c2",
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

	// ECharts コンテナ id（初期化対象マーカー data-echarts + 単位/色の data-*）
	assertContains(t, html, `id="temperature-chart"`)
	assertContains(t, html, `id="humidity-chart"`)
	assertContains(t, html, "data-echarts")
	assertContains(t, html, `data-unit="℃"`)
	assertContains(t, html, `data-unit="%"`)
	assertContains(t, html, `data-color="#e8590c"`)
	assertContains(t, html, `data-color="#1971c2"`)

	// option JSON は <script type="application/json"> で安全供給（@templ.Raw で生の JSON）
	assertContains(t, html, `<script type="application/json" id="temperature-chart-option">`)
	assertContains(t, html, `<script type="application/json" id="humidity-chart-option">`)
	assertContains(t, html, `{"series":[{"type":"line"}]}`)

	// 最新計測テーブルは差し替え対象外（フラグメントに含めない）
	if strings.Contains(html, "latest-readings-table") {
		t.Errorf("グラフ領域フラグメントに latest-readings-table が含まれている:\n%s", html)
	}
}

// dewpointSampleAreaView は露点パネルを内包した DeviceChartAreaView（描画テスト用の代表値）。
func dewpointSampleAreaView() DeviceChartAreaView {
	return DeviceChartAreaView{
		DeviceID: 1, Period: "3d", HasData: true,
		TemperatureOptionJSON: "{}", HumidityOptionJSON: "{}",
		TemperatureUnit: "℃", HumidityUnit: "%",
		TemperatureColor: "#e8590c", HumidityColor: "#1971c2",
		Dewpoint: DewpointPanelView{
			OptionJSON: `{"series":[{"markArea":{}}]}`,
			DewColor:   "#4263eb",
			Note:       "葉面温度センサ不在のため気温で近似。結露帯は露点スプレッド（気温−露点）≦ しきい値の代理判定。",
			Card: DewpointCardView{
				CurrentDewpoint: "13.9℃", CurrentSpread: "1.5℃",
				TodayWetHours: "3.5 時間", RecentCondensation: "結露中",
			},
			Daily: []DewpointDailyRow{
				{Date: "2026-04-20", WetHours: "3.5 時間", DiseaseScore: "40%"},
			},
			Events: []HighHumidityEventRow{
				{Start: "04/20 03:00", End: "04/20 05:30", Duration: "2.5 時間", MinSpread: "0.3℃"},
			},
		},
	}
}

func TestDeviceChartArea_露点パネルを描画する(t *testing.T) {
	html := render(t, DeviceChartArea(dewpointSampleAreaView()))
	for _, want := range []string{
		`id="dewpoint-chart"`,        // 露点チャートマウント先
		`data-color="#4263eb"`,       // 露点基準色（寒色）
		`id="dewpoint-chart-option"`, // 露点 option script
		"現在露点", "13.9℃",              // 露点カード
		"現在スプレッド", "1.5℃",
		"結露中",               // 直近の結露帯（湿り側）
		"2026-04-20", "40%", // 葉面湿潤日次表（病害スコア下地列）
		"04/20 03:00", "04/20 05:30", "0.3℃", // 高湿度イベント表
		"近似", // 近似注記
	} {
		if !strings.Contains(html, want) {
			t.Errorf("露点パネル描画に %q が含まれていない", want)
		}
	}
	// 露点チャートは ℃ 単位（data-unit="℃" が温度チャートと露点チャートの2箇所に出る）。
	if got := strings.Count(html, `data-unit="℃"`); got < 2 {
		t.Errorf(`data-unit="℃" の数 = %d, want ≥2（温度＋露点）`, got)
	}
}

// 研究用スコープ: 露点パネルは農家向け平易表示（信号色の病害警告・圃場共有 URL）を含めない（要件 8.3）。
func TestDeviceChartArea_露点パネルは農家向け表示を含めない(t *testing.T) {
	html := render(t, DeviceChartArea(dewpointSampleAreaView()))
	for _, ng := range []string{"圃場共有", "病害警告", "シェア", "危険", "安全"} {
		if strings.Contains(html, ng) {
			t.Errorf("研究用スコープ外の農家向け表示 %q が混入している", ng)
		}
	}
}

// HasData=true でも Dewpoint 未設定（OptionJSON 空）のときは露点パネルを描画しない（既存 fixture 無回帰のガード）。
func TestDeviceChartArea_露点未設定では描画しない(t *testing.T) {
	v := DeviceChartAreaView{
		DeviceID: 1, Period: "24h", HasData: true,
		TemperatureOptionJSON: "{}", HumidityOptionJSON: "{}",
		TemperatureUnit: "℃", HumidityUnit: "%",
		TemperatureColor: "#e8590c", HumidityColor: "#1971c2",
		// Dewpoint は zero 値（OptionJSON 空）。
	}
	html := render(t, DeviceChartArea(v))
	if strings.Contains(html, `id="dewpoint-chart"`) {
		t.Errorf("Dewpoint 未設定なのに露点パネルが描画されている")
	}
}

// HasData=false のときはグラフ scaffold を出さず「データはまだありません」ブロックのみ。
func TestDeviceChartArea_データ無しは空メッセージのみ(t *testing.T) {
	html := render(t, DeviceChartArea(DeviceChartAreaView{DeviceID: 5, Period: "24h", HasData: false}))

	// 期間セレクタは常に出る
	assertContains(t, html, "period-selector")
	assertContains(t, html, "データはまだありません")

	// option script / data-echarts コンテナは出さない
	if strings.Contains(html, `type="application/json"`) {
		t.Errorf("データ無しなのに option script が出ている:\n%s", html)
	}
	if strings.Contains(html, "data-echarts") {
		t.Errorf("データ無しなのに data-echarts コンテナが出ている:\n%s", html)
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
