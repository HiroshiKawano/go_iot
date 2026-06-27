package page

import (
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/view/component"
	"github.com/HiroshiKawano/go_iot/internal/view/layout"
)

func baseReadingsView() ReadingsView {
	return ReadingsView{
		Layout:     layout.AppLayoutData{Title: "センサーデータ履歴", UserName: "テストユーザー", CSRFToken: "tk", CSSURL: "/x.css"},
		DeviceID:   42,
		DeviceName: "ハウスA温湿度計",
		From:       "2026-04-13",
		To:         "2026-04-20",
		List: component.DeviceReadingsListView{
			Summary: component.SummaryView{
				AvgTemp: "28.30℃", MaxTemp: "35.20℃", MinTemp: "18.50℃",
				AvgHum: "62.50%", MaxHum: "85.00%", MinHum: "30.20%",
			},
			Rows: []component.ReadingHistoryRow{
				{RecordedAt: "2026-04-20 14:30", Temp: "28.50", Humidity: "65.30", Delay: "2秒"},
			},
			HasData:    true,
			Pagination: component.PaginationView{Current: 1, Last: 1},
			Errors:     map[string]string{},
		},
	}
}

// TestReadingsPage_見出しフィルタフォーム結果領域を統合描画 は、フルページが
// App レイアウトを継承し、見出し・フィルタフォーム・結果領域フラグメントの双方を
// 含むことを Render 結果で検証する (R1.1, R2.1, R8.2)。
func TestReadingsPage_見出しフィルタフォーム結果領域を統合描画(t *testing.T) {
	html := render(t, ReadingsPage(baseReadingsView()))

	// App レイアウト継承（ヘッダーのユーザー名・csrf meta）。
	assertContains(t, html, "テストユーザー")
	assertContains(t, html, "csrf-token")

	// 見出し（「センサーデータ履歴: デバイス名」）。
	assertContains(t, html, "センサーデータ履歴: ハウスA温湿度計")

	// フィルタフォーム（GET・期間入力・echo 値・検索ボタン・HTMX 部分更新）。
	assertContains(t, html, "filter-form")
	assertContains(t, html, `method="get"`)
	assertContains(t, html, `name="from"`)
	assertContains(t, html, `name="to"`)
	assertContains(t, html, `value="2026-04-13"`) // from の echo
	assertContains(t, html, `value="2026-04-20"`) // to の echo
	assertContains(t, html, "検索")
	// フィルタフォームは結果領域を部分更新する（hx-get + ターゲット + push-url）。
	assertContains(t, html, `hx-target="#device-readings-list"`)
	assertContains(t, html, `hx-push-url="true"`)

	// 結果領域フラグメントを内包（id・集計・データ一覧）。
	assertContains(t, html, `id="device-readings-list"`)
	assertContains(t, html, "summary-grid")
	assertContains(t, html, "28.30℃")
	assertContains(t, html, "data-table")
	assertContains(t, html, "2秒")
}

// TestReadingsPage_項目フィルタの選択をcheckedで再描画 は、フィルタフォームに温度/湿度の
// 項目 checkbox が描画され、適用済み Items に応じて checked を echo すること (4.2) を検証する。
func TestReadingsPage_項目フィルタの選択をcheckedで再描画(t *testing.T) {
	t.Run("温度のみ選択は温度checkedで湿度未checked", func(t *testing.T) {
		v := baseReadingsView()
		v.Items = []string{"temperature"}
		html := render(t, ReadingsPage(v))

		assertContains(t, html, `name="items" value="temperature"`)
		assertContains(t, html, `name="items" value="humidity"`)
		if !strings.Contains(html, `value="temperature" checked`) {
			t.Errorf("温度 checkbox が checked でない:\n%s", html)
		}
		if strings.Contains(html, `value="humidity" checked`) {
			t.Errorf("湿度未選択なのに checked になっている")
		}
	})
	t.Run("両方選択は両方checked", func(t *testing.T) {
		v := baseReadingsView()
		v.Items = []string{"temperature", "humidity"}
		html := render(t, ReadingsPage(v))

		if !strings.Contains(html, `value="temperature" checked`) || !strings.Contains(html, `value="humidity" checked`) {
			t.Errorf("両方選択なのに両方 checked でない:\n%s", html)
		}
	})
}

// TestReadingsPage_EChartsは読込むがコンテナ非在でno_op は、デバイス詳細以外の認証画面でも
// App レイアウト経由で echarts.min.js を self-host 読込する一方 (R5・全認証画面共通)、ECharts
// コンテナ ([data-echarts] / option script) を持たないためグローバル初期化が no-op になり、
// 無回帰であることを検証する (device-show 以外は描画対象ゼロ)。
func TestReadingsPage_EChartsは読込むがコンテナ非在でno_op(t *testing.T) {
	html := render(t, ReadingsPage(baseReadingsView()))

	// App レイアウト経由で echarts.min.js を読み込む (self-host・全認証画面共通)。
	assertContains(t, html, "/static/js/echarts.min.js")

	// グラフコンテナ div / option script は持たない → 初期化対象ゼロで no-op。
	if strings.Contains(html, `<div id="temperature-chart"`) || strings.Contains(html, `<div id="humidity-chart"`) {
		t.Errorf("非デバイス画面に ECharts コンテナ div が混入している:\n%s", html)
	}
	if strings.Contains(html, `type="application/json"`) {
		t.Errorf("非デバイス画面に option script が混入している:\n%s", html)
	}
}

// TestReadingsPage_フィルタフォームは結果領域の外 は、入力状態保持のためフィルタフォームが
// 部分更新ターゲット (#device-readings-list) の外側に配置されることを検証する。
func TestReadingsPage_フィルタフォームは結果領域の外(t *testing.T) {
	html := render(t, ReadingsPage(baseReadingsView()))

	formIdx := strings.Index(html, "filter-form")
	listIdx := strings.Index(html, `id="device-readings-list"`)
	if formIdx < 0 || listIdx < 0 {
		t.Fatalf("filter-form(%d) または device-readings-list(%d) が見つからない", formIdx, listIdx)
	}
	if formIdx >= listIdx {
		t.Errorf("フィルタフォーム(%d)が結果領域(%d)より後にある（外側=前に配置すべき）", formIdx, listIdx)
	}
}

// TestReadingsPage_未指定時はecho値が空 は、初期表示（フィルタ未指定）で from/to 入力の
// value が空文字で描画され、前回検索値が残らないことを検証する (R8.2/8.3 状態再現)。
func TestReadingsPage_未指定時はecho値が空(t *testing.T) {
	v := baseReadingsView()
	v.From = ""
	v.To = ""
	html := render(t, ReadingsPage(v))

	// 入力欄は存在し、value は空（templ は空文字でも value="" を出力する）。
	assertContains(t, html, `name="from" value=""`)
	assertContains(t, html, `name="to" value=""`)

	// 前回の検索値（baseReadingsView の echo 値）が残っていない。
	for _, stale := range []string{`value="2026-04-13"`, `value="2026-04-20"`} {
		if strings.Contains(html, stale) {
			t.Errorf("未指定なのに前回の echo 値 %s が残存している", stale)
		}
	}
}
