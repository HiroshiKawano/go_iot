package component

import (
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/domain"
)

// quality_components_test.go は data-quality-meta の View 描画 (履歴一覧の品質メトリクスボックス・
// 総合品質バッジ・行品質フラグ列、デバイス詳細の欠測ギャップ凡例) を Render→strings.Contains で検証する。

// ---- 6.2 品質メトリクスボックス + 総合品質バッジ ----------------------------

func TestDeviceReadingsList_品質メトリクスボックスと総合バッジ(t *testing.T) {
	v := fullListView()
	v.Quality = QualityMetricsView{
		MissingRate: "3.2%",
		IntervalCV:  "0.18",
		DelayAvg:    "5秒",
		DelayMax:    "9秒",
		Level:       domain.QualityLevelCaution,
		HasData:     true,
	}
	html := render(t, DeviceReadingsList(v))

	// データ品質セクションの見出しとメトリクス4項目 (ラベル + 整形済み値)。
	assertContains(t, html, "データ品質")
	for _, label := range []string{"欠測率", "間隔ばらつき", "通信遅延(平均)", "通信遅延(最大)"} {
		assertContains(t, html, label)
	}
	for _, val := range []string{"3.2%", "0.18", "9秒"} {
		assertContains(t, html, val)
	}

	// 総合品質バッジ (domain の信号色クラス + ラベル)。
	assertContains(t, html, "badge-caution")
	assertContains(t, html, "注意")
}

// 品質メタが空 (0件/単一点) のときは各値 "—"・総合バッジ (信号色) を出さない。
func TestDeviceReadingsList_品質メタ空はダッシュとバッジ非表示(t *testing.T) {
	v := fullListView()
	v.Quality = QualityMetricsView{
		MissingRate: "—", IntervalCV: "—", DelayAvg: "—", DelayMax: "—",
		Level: domain.QualityLevelGood, HasData: false,
	}
	html := render(t, DeviceReadingsList(v))

	assertContains(t, html, "データ品質")
	assertContains(t, html, "—")
	// HasData=false では総合バッジ (信号色 variant) を描画しない。
	for _, cls := range []string{"badge-good", "badge-caution", "badge-bad"} {
		if strings.Contains(html, cls) {
			t.Errorf("品質メタ空なのに総合バッジ %q が描画されている:\n%s", cls, html)
		}
	}
}

// ---- 6.2 行品質フラグ列 (異常行のみバッジ・正常行は空) -----------------------

func TestDeviceReadingsList_品質フラグ列に異常行のみバッジ(t *testing.T) {
	v := fullListView()
	v.Quality = QualityMetricsView{HasData: false, MissingRate: "—", IntervalCV: "—", DelayAvg: "—", DelayMax: "—", Level: domain.QualityLevelGood}
	v.Rows = []ReadingHistoryRow{
		{RecordedAt: "2026-04-20 14:30", Temp: "28.50", Humidity: "65.30", Delay: "2秒",
			QualityFlags: []domain.QualityFlag{domain.QualityFlagOutlier}},
		{RecordedAt: "2026-04-20 14:25", Temp: "28.30", Humidity: "65.50", Delay: "1秒"}, // 正常行
	}
	html := render(t, DeviceReadingsList(v))

	// 品質フラグ列の見出し。
	assertContains(t, html, "<th>品質</th>")
	// 異常行はフラグ Label をバッジ表示。
	assertContains(t, html, "外れ値")
	assertContains(t, html, `class="badge"`)
	// 正常行はバッジを足さない: フラグバッジ (class="badge") はちょうど異常行分のみ。
	if got := strings.Count(html, `class="badge"`); got != 1 {
		t.Errorf(`class="badge" の数=%d, want 1 (異常行1のみ・正常行は空)`+"\n%s", got, html)
	}
}

// 複数フラグに該当する行は複数バッジを並べる。
func TestDeviceReadingsList_複数フラグ行は複数バッジ(t *testing.T) {
	v := fullListView()
	v.Quality = QualityMetricsView{HasData: false, MissingRate: "—", IntervalCV: "—", DelayAvg: "—", DelayMax: "—", Level: domain.QualityLevelGood}
	v.Rows = []ReadingHistoryRow{
		{RecordedAt: "2026-04-20 14:30", Temp: "70.00", Humidity: "50.00", Delay: "2秒",
			QualityFlags: []domain.QualityFlag{domain.QualityFlagStuck, domain.QualityFlagPhysical}},
	}
	html := render(t, DeviceReadingsList(v))

	assertContains(t, html, "固着")
	assertContains(t, html, "物理異常")
	if got := strings.Count(html, `class="badge"`); got != 2 {
		t.Errorf(`class="badge" の数=%d, want 2 (固着+物理異常)`, got)
	}
}

// ---- 6.3 欠測ギャップ凡例/注記 (デバイス詳細グラフ領域) -----------------------

func TestDeviceChartArea_欠測ギャップ凡例(t *testing.T) {
	v := DeviceChartAreaView{
		DeviceID: 1, Period: "24h", HasData: true, HasGap: true,
		TemperatureOptionJSON: `{"series":[{"type":"line"}]}`,
		HumidityOptionJSON:    `{"series":[{"type":"line"}]}`,
		TemperatureUnit:       "℃", HumidityUnit: "%",
		TemperatureColor: "#e8590c", HumidityColor: "#1971c2",
	}
	html := render(t, DeviceChartArea(v))
	// 欠測ギャップの凡例/注記 (静的な器)。
	assertContains(t, html, "欠測区間")
}

func TestDeviceChartArea_欠測なしは凡例なし(t *testing.T) {
	v := DeviceChartAreaView{
		DeviceID: 1, Period: "24h", HasData: true, HasGap: false,
		TemperatureOptionJSON: `{"series":[{"type":"line"}]}`,
		HumidityOptionJSON:    `{"series":[{"type":"line"}]}`,
		TemperatureUnit:       "℃", HumidityUnit: "%",
		TemperatureColor: "#e8590c", HumidityColor: "#1971c2",
	}
	html := render(t, DeviceChartArea(v))
	if strings.Contains(html, "欠測区間") {
		t.Errorf("欠測なしなのにギャップ凡例が描画されている:\n%s", html)
	}
}
