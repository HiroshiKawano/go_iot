package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// seasonal_trend_integration_test.go は統計分析ページのクリティカルパスを end-to-end で固める
// 回帰ガード（タスク 7.1）。デバイス選択→トレンド表示・空月スキップ・チャート描画パイプライン疎通・
// HX 部分更新差し替えを httptest で通し、別型 TrendChartSpec/新クエリ追加が既存に影響しないことを担保する。

// genTrendRowsSkipFeb は startYear から years 年分の月次行を生成するが2月を欠測（行なし）にする。
// 「空月を欠測としてスキップ（0補完しない）」を end-to-end で検証するためのデータ。
func genTrendRowsSkipFeb(startYear, years int) []repository.ListDailySensorAggregatesJSTRow {
	var rows []repository.ListDailySensorAggregatesJSTRow
	i := 0
	for y := 0; y < years; y++ {
		for m := 1; m <= 12; m++ {
			if m == 2 {
				continue // 2月は計測なし（空月）
			}
			t := 15.0 + 0.15*float64(i)
			h := 60.0 + 0.05*float64(i)
			rows = append(rows, jstDailyRow(startYear+y, time.Month(m), 15,
				t, t+5, t-5, h, h+8, h-8, 144))
			i++
		}
	}
	return rows
}

// 空月（2月）を含む期間でも、存在する月のみで集計し欠測を 0 補完しない（要件 2.3・end-to-end）。
func TestSeasonalTrend_空月は欠測スキップでトレンド成立(t *testing.T) {
	h := &SeasonalTrendHandler{Repo: trendRepo(genTrendRowsSkipFeb(2024, 3))}
	w := doTrendGET(t, h, 7, "/analysis/trend?device_id=1&granularity=monthly", false)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()

	// 存在する月（1月/3月）はサマリ表に現れる。
	assertContains(t, body, "2024-01")
	assertContains(t, body, "2024-03")
	// 空月（2024-02）は欠測としてスキップ＝行を生成しない（0 補完しない）。
	if strings.Contains(body, "2024-02") {
		t.Errorf("空月 2024-02 が出力されている（欠測スキップすべき・0補完しない）")
	}
	// 空月があってもトレンド全体は成立（チャート器が描画される）。
	assertContains(t, body, `id="trend-chart"`)
}

// トレンドチャートの描画パイプライン疎通: Sen 線(markLine coord)・dataZoom・日較差 option が
// handler→view→HTML を通って出力される（別型 TrendChartSpec 経路の end-to-end 疎通）。
func TestSeasonalTrend_チャート描画パイプライン疎通(t *testing.T) {
	h := &SeasonalTrendHandler{Repo: trendRepo(genTrendRows(2024, 3))}
	w := doTrendGET(t, h, 7, "/analysis/trend?device_id=1&granularity=monthly", false)
	body := w.Body.String()

	// トレンドチャートの option script（兄弟 <script type="application/json">）。
	assertContains(t, body, `id="trend-chart-option"`)
	// Sen トレンド線は markLine の coord 端点で注入される。
	assertContains(t, body, `"coord"`)
	// 長期閲覧用 dataZoom（inside/slider）。
	assertContains(t, body, "dataZoom")
	// 日較差ΔT チャートの option script。
	assertContains(t, body, `id="diurnal-chart-option"`)
	// option JSON は HTML 安全（生の </script> が漏れない・§10-E）。
	if strings.Contains(body, "</script></script>") {
		t.Errorf("option JSON 由来の生 </script> 連続が漏れている恐れ")
	}
}

// 年次粒度でも end-to-end でトレンドが成立する（granularity 切替の回帰ガード）。
func TestSeasonalTrend_年次粒度でも成立(t *testing.T) {
	h := &SeasonalTrendHandler{Repo: trendRepo(genTrendRows(2020, 5))} // 5年分
	w := doTrendGET(t, h, 7, "/analysis/trend?device_id=1&granularity=yearly", false)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	assertContains(t, body, `id="trend-section"`)
	assertContains(t, body, "2020") // 年次バケットキー
	assertContains(t, body, "2024")
}

// HX 部分更新で device/期間切替時に #trend-section の中身（判定・サマリ）が差し替わる（要件 1.2）。
func TestSeasonalTrend_HX切替で判定とサマリが差し替わる(t *testing.T) {
	h := &SeasonalTrendHandler{Repo: trendRepo(genTrendRows(2024, 3))}
	w := doTrendGET(t, h, 7, "/analysis/trend?device_id=1&granularity=yearly", true)
	body := w.Body.String()
	if strings.Contains(body, "<html") {
		t.Errorf("HX 切替でフルページが返っている（部分返却すべき）")
	}
	assertContains(t, body, `id="trend-section"`)
	assertContains(t, body, "トレンド判定")
	assertContains(t, body, "ロールアップ統計サマリ（湿度）") // 湿度サマリも差し替え対象に含む
}
