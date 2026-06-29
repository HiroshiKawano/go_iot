package component

import (
	"strings"
	"testing"
)

// seasonal_trend_view_test.go は統計分析ページの component（TrendSection/TrendBadge）と
// 左サイドメニュー「統計分析」項目を Render→strings.Contains で検証する（タスク 5.1/5.2）。

// baselineTrendSectionView はデータあり・全要素を持つ TrendSectionView（決定的テスト用）。
func baselineTrendSectionView() TrendSectionView {
	return TrendSectionView{
		HasData:   true,
		PowerNote: "非有意はトレンドが無いことを意味しません（非有意 ≠ トレンド無し）。",
		Badges: []TrendBadgeView{
			{Metric: "温度", Stage: "一次判定（多重比較未補正）", Direction: "up", Verdict: "有意な上昇", Slope: "+0.03 ℃/年", PValue: "0.012"},
			{Metric: "温度", Stage: "補正済み（Hamed-Rao）", Direction: "flat", Verdict: "非有意", Slope: "+0.03 ℃/年", PValue: "0.087"},
			{Metric: "湿度", Stage: "一次判定（多重比較未補正）", Direction: "down", Verdict: "有意な下降", Slope: "−0.15 %/年", PValue: "0.030"},
		},
		TempRows: []RollupRow{
			{Bucket: "2024-04", Avg: "24.80", Max: "33.10", Min: "17.40", DiurnalRange: "11.20", StdDev: "3.85", CV: "0.16", Samples: "4320"},
		},
		HumidityRows: []RollupRow{
			{Bucket: "2024-04", Avg: "68.20", Max: "94.00", Min: "42.10", DiurnalRange: "51.90", StdDev: "12.40", CV: "0.18", Samples: "4320"},
		},
		TrendOptionJSON:   `{"series":[{"type":"line"}]}`,
		DiurnalOptionJSON: `{"series":[{"type":"line","name":"diurnal"}]}`,
		TrendColor:        "#7048e8",
		ChartUnit:         "℃",
		ClimatologyNote:   "平年比は自社データの暦月平均に基づきます。基準期間に依存し不確実性を伴うため参考値です。",
	}
}

// ---- 5.1 TrendBadge（信号色バッジ）-------------------------------------------

func TestTrendBadge_信号色pillと判定文言(t *testing.T) {
	cases := []struct {
		dir, verdict, wantClass string
	}{
		{"up", "有意な上昇", "badge-trend-up"},
		{"down", "有意な下降", "badge-trend-down"},
		{"flat", "非有意", "badge-trend-flat"},
	}
	for _, tc := range cases {
		html := render(t, TrendBadge(TrendBadgeView{Direction: tc.dir, Verdict: tc.verdict}))
		assertContains(t, html, "badge")
		assertContains(t, html, tc.wantClass)
		assertContains(t, html, tc.verdict)
	}
}

// ---- 5.1 TrendSection（部分更新単位）-----------------------------------------

func TestTrendSection_データありで全要素を描画(t *testing.T) {
	html := render(t, TrendSection(baselineTrendSectionView()))

	// HX 部分更新ターゲットの器。
	assertContains(t, html, `id="trend-section"`)

	// 検出力留保注記（非有意≠トレンド無し）。
	assertContains(t, html, "非有意 ≠ トレンド無し")

	// 判定バッジ: 信号色 pill ＋ 区分ラベル ＋ Sen 傾き ＋ p 値。
	for _, v := range []string{"badge-trend-up", "badge-trend-flat", "badge-trend-down",
		"有意な上昇", "非有意", "有意な下降",
		"一次判定（多重比較未補正）", "補正済み（Hamed-Rao）",
		"+0.03 ℃/年", "−0.15 %/年", "0.012", "0.087", "0.030"} {
		assertContains(t, html, v)
	}

	// チャートコンテナ（data-echarts ＋ 兄弟 option script）。
	assertContains(t, html, `id="trend-chart"`)
	assertContains(t, html, "data-echarts")
	assertContains(t, html, `data-color="#7048e8"`)
	assertContains(t, html, `<script type="application/json" id="trend-chart-option">`)
	assertContains(t, html, `{"series":[{"type":"line"}]}`)
	assertContains(t, html, `id="diurnal-chart"`)
	assertContains(t, html, `<script type="application/json" id="diurnal-chart-option">`)

	// サマリ表（温度・湿度の2表・各バケット行）。
	assertContains(t, html, "ロールアップ統計サマリ（温度）")
	assertContains(t, html, "ロールアップ統計サマリ（湿度）")
	for _, v := range []string{"2024-04", "24.80", "11.20", "0.16", "4320", "68.20", "51.90"} {
		assertContains(t, html, v)
	}

	// 平年比注記。
	assertContains(t, html, "平年比は自社データの暦月平均")
}

func TestTrendSection_データ無は案内のみでチャート器を出さない(t *testing.T) {
	v := TrendSectionView{
		HasData:      false,
		EmptyMessage: "対象のデバイスを選択してください。",
	}
	html := render(t, TrendSection(v))

	assertContains(t, html, `id="trend-section"`)
	assertContains(t, html, "対象のデバイスを選択してください。")
	// データ無ではチャート器・判定表を描かない（縮退）。
	if strings.Contains(html, `id="trend-chart"`) {
		t.Errorf("データ無で #trend-chart が描画されている（縮退すべき）:\n%s", html)
	}
}

// 平年比が空のときは注記行を出さない（年数不足は handler が別注記を PowerNote/EmptyMessage で処理）。
func TestTrendSection_平年比注記が空なら出さない(t *testing.T) {
	v := baselineTrendSectionView()
	v.ClimatologyNote = ""
	html := render(t, TrendSection(v))
	if strings.Contains(html, "平年比は自社データの暦月平均") {
		t.Error("ClimatologyNote 空なのに平年比注記が描画されている")
	}
}

// ---- 5.2 左サイドメニュー「統計分析」項目 -------------------------------------

func TestSidebar_統計分析項目はトップ階層で常時表示(t *testing.T) {
	// どの文脈でも統計分析リンクは常時表示（ダッシュボード等と同列）。
	html := render(t, Sidebar(SidebarNav{Current: NavDashboard}))
	assertContains(t, html, `href="/analysis/trend"`)
	assertContains(t, html, "統計分析")
	// ダッシュボード文脈では統計分析は非 active。
	assertNotContains(t, html, `href="/analysis/trend" class="active"`)
}

func TestSidebar_統計分析文脈で統計分析のみactive(t *testing.T) {
	html := render(t, Sidebar(SidebarNav{Current: NavAnalysisTrend}))
	assertContains(t, html, `href="/analysis/trend" class="active"`)
	assertActiveCount(t, html, 1) // 同時 active ≤1
	// デバイス文脈リンクは出さない（統計分析はデバイス非依存のトップ階層）。
	assertNotContains(t, html, "/devices/")
}
