package page

import (
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/view/component"
	"github.com/HiroshiKawano/go_iot/internal/view/layout"
)

// seasonal_trend_test.go は統計分析フルページ（page.SeasonalTrend）を Render→strings.Contains で検証する
// （タスク 5.1）。App レイアウト・対象/期間セレクタ（swap 非対象）・#trend-section を確認する。

func TestSeasonalTrendPage_セレクタとセクションを描画(t *testing.T) {
	v := SeasonalTrendPageView{
		Layout:      layout.AppLayoutData{Title: "統計分析 - 農業IoTシステム", CSSURL: "/static/css/style.css?v=dev"},
		Devices:     []component.DeviceOption{{ID: 1, Name: "ハウスA温湿度計", Selected: true}, {ID: 2, Name: "ハウスB温湿度計"}},
		Granularity: "monthly",
		Section: component.TrendSectionView{
			HasData:         true,
			TempRows:        []component.RollupRow{{Bucket: "2024-04", Avg: "24.80"}},
			TrendOptionJSON: `{"series":[]}`,
			TrendColor:      "#7048e8",
			ChartUnit:       "℃",
		},
	}
	html := render(t, SeasonalTrend(v))

	assertContains(t, html, "<html")         // フルページ（App レイアウト）
	assertContains(t, html, "統計分析")          // 見出し
	assertContains(t, html, "js-tom-select") // デバイス選択は Tom Select
	// デバイス/期間切替は #trend-section を部分差し替え（セレクタは swap 対象外）。
	assertContains(t, html, `hx-get="/analysis/trend"`)
	assertContains(t, html, `hx-target="#trend-section"`)
	assertContains(t, html, `hx-trigger="change"`)
	assertContains(t, html, `id="trend-section"`)
	// 集計期間（粒度）セレクタ。
	assertContains(t, html, `name="granularity"`)
	assertContains(t, html, `name="device_id"`)
	// デバイス選択肢の復元。
	assertContains(t, html, "ハウスA温湿度計")
	assertContains(t, html, `value="1" selected`)
}

func TestSeasonalTrendPage_未選択は空セクション案内(t *testing.T) {
	v := SeasonalTrendPageView{
		Layout:      layout.AppLayoutData{Title: "統計分析 - 農業IoTシステム", CSSURL: "/static/css/style.css?v=dev"},
		Devices:     []component.DeviceOption{{ID: 1, Name: "ハウスA温湿度計"}},
		Granularity: "monthly",
		Section: component.TrendSectionView{
			HasData:      false,
			EmptyMessage: "対象のデバイスを選択してください。",
		},
	}
	html := render(t, SeasonalTrend(v))

	assertContains(t, html, "<html")
	assertContains(t, html, "対象のデバイスを選択してください。")
}
