package page

import (
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/view/component"
	"github.com/HiroshiKawano/go_iot/internal/view/layout"
)

// baseAlertHistoryView は device_id=2 を選択中・期間指定済みのフルページ View。
func baseAlertHistoryView() AlertHistoryView {
	return AlertHistoryView{
		Layout:   layout.AppLayoutData{Title: "アラート履歴", UserName: "テストユーザー", CSRFToken: "tk", CSSURL: "/x.css"},
		Devices:  []component.DeviceOption{{ID: 1, Name: "ハウスA温湿度計", Selected: false}, {ID: 2, Name: "ハウスB温湿度計", Selected: true}},
		DeviceID: "2",
		From:     "2026-04-13",
		To:       "2026-04-20",
		List: component.AlertHistoryListView{
			Rows: []component.AlertHistoryRow{
				{TriggeredAt: "2026-04-20 14:30", DeviceName: "ハウスA温湿度計", MetricLabel: "温度", Condition: "> 35.00℃", ActualValue: "38.50℃", Notified: "済"},
			},
			HasData:       true,
			HasPagination: false,
			Errors:        map[string]string{},
		},
	}
}

// TestAlertHistory_フィルタフォームと結果領域を統合描画 は、フルページが App レイアウトを
// 継承し、HTMX 部分更新フィルタフォーム（device/from/to）・デバイス選択肢・結果領域
// フラグメントを含むことを検証する (R2.4/2.5, R5.1/5.2, R8.2)。
func TestAlertHistory_フィルタフォームと結果領域を統合描画(t *testing.T) {
	html := render(t, AlertHistory(baseAlertHistoryView()))

	// App レイアウト継承（ヘッダーのユーザー名・csrf meta）。
	assertContains(t, html, "テストユーザー")
	assertContains(t, html, "csrf-token")

	// 見出し。
	assertContains(t, html, "アラート履歴")

	// フィルタフォーム（GET を HTMX 部分更新へ昇華）。
	assertContains(t, html, "filter-form")
	assertContains(t, html, `method="get"`)
	assertContains(t, html, `hx-get="/alerts/history"`)
	assertContains(t, html, `hx-target="#alert-history-list"`)
	assertContains(t, html, `hx-swap="innerHTML"`)
	assertContains(t, html, `hx-push-url="true"`)

	// デバイス select（Tom Select・全デバイス + 本人デバイス）。
	assertContains(t, html, "js-tom-select")
	assertContains(t, html, `name="device_id"`)
	assertContains(t, html, "全デバイス")
	assertContains(t, html, "ハウスA温湿度計")
	assertContains(t, html, "ハウスB温湿度計")
	// 選択中 device_id=2 の option に selected が付く。
	assertContains(t, html, `value="2" selected`)

	// from/to の echo 値。
	assertContains(t, html, `value="2026-04-13"`)
	assertContains(t, html, `value="2026-04-20"`)

	// 結果領域フラグメントを内包。
	assertContains(t, html, `id="alert-history-list"`)
	assertContains(t, html, "data-table")
}

// TestAlertHistory_全デバイス選択時は先頭optionにselected は、device_id 未選択 ("") のとき
// 「全デバイス」option に selected が付き、各デバイス option には付かないことを検証する (R5.1)。
func TestAlertHistory_全デバイス選択時は先頭optionにselected(t *testing.T) {
	v := baseAlertHistoryView()
	v.DeviceID = ""
	v.Devices = []component.DeviceOption{{ID: 1, Name: "ハウスA温湿度計", Selected: false}, {ID: 2, Name: "ハウスB温湿度計", Selected: false}}
	html := render(t, AlertHistory(v))

	// 全デバイス option（value="")が selected。
	assertContains(t, html, `value="" selected`)
	// どのデバイス option も selected でない。
	if strings.Contains(html, `value="1" selected`) || strings.Contains(html, `value="2" selected`) {
		t.Errorf("全デバイス選択中なのにデバイス option に selected が付いている:\n%s", html)
	}
}

// TestAlertHistory_フィルタフォームは結果領域の外 は、入力状態保持のためフィルタフォームが
// 部分更新ターゲット (#alert-history-list) の外側（前方）に配置されることを検証する。
func TestAlertHistory_フィルタフォームは結果領域の外(t *testing.T) {
	html := render(t, AlertHistory(baseAlertHistoryView()))

	formIdx := strings.Index(html, "filter-form")
	listIdx := strings.Index(html, `id="alert-history-list"`)
	if formIdx < 0 || listIdx < 0 {
		t.Fatalf("filter-form(%d) または alert-history-list(%d) が見つからない", formIdx, listIdx)
	}
	if formIdx >= listIdx {
		t.Errorf("フィルタフォーム(%d)が結果領域(%d)より後にある（外側=前に配置すべき）", formIdx, listIdx)
	}
}

// TestAlertHistory_未指定時はecho値が空 は、初期表示（フィルタ未指定）で from/to 入力の
// value が空文字で描画され、前回検索値が残らないことを検証する。
func TestAlertHistory_未指定時はecho値が空(t *testing.T) {
	v := baseAlertHistoryView()
	v.From = ""
	v.To = ""
	html := render(t, AlertHistory(v))

	assertContains(t, html, `name="from" value=""`)
	assertContains(t, html, `name="to" value=""`)
	for _, stale := range []string{`value="2026-04-13"`, `value="2026-04-20"`} {
		if strings.Contains(html, stale) {
			t.Errorf("未指定なのに前回の echo 値 %s が残存している", stale)
		}
	}
}
