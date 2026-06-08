package page

import (
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/view/component"
	"github.com/HiroshiKawano/go_iot/internal/view/layout"
)

// --- 3.5 page.AlertRules ---

func TestAlertRulesPage_デバイス選択とセクションを描画(t *testing.T) {
	v := AlertRulesPageView{
		Layout:    layout.AppLayoutData{Title: "アラートルール管理 - 農業IoTシステム", CSSURL: "/static/css/style.css?v=dev"},
		Devices:   []component.DeviceOption{{ID: 1, Name: "ハウスA温湿度計", Selected: true}, {ID: 2, Name: "ハウスB温湿度計"}},
		HasDevice: true,
		Section: component.AlertRuleSectionView{
			DeviceID: 1,
			Form:     component.AlertRuleFormView{DeviceID: 1},
		},
	}
	html := render(t, AlertRules(v))

	assertContains(t, html, "<html")         // フルページ (App レイアウト)
	assertContains(t, html, "アラートルール管理")     // 見出し
	assertContains(t, html, "js-tom-select") // デバイス選択は Tom Select 適用
	assertContains(t, html, `hx-get="/alerts/rules"`)
	assertContains(t, html, `hx-target="#alert-rule-section"`)
	assertContains(t, html, `hx-trigger="change"`) // デバイス切替トリガ
	assertContains(t, html, `id="alert-rule-section"`)
	assertContains(t, html, `id="alert-rule-form"`) // 空の追加フォーム
	assertContains(t, html, "ハウスA温湿度計")
	assertContains(t, html, "ハウスB温湿度計")
	assertContains(t, html, `value="1" selected`) // 選択中デバイス(ID=1)の復元
}

func TestAlertRulesPage_所有デバイス0件で案内表示(t *testing.T) {
	v := AlertRulesPageView{
		Layout:    layout.AppLayoutData{Title: "アラートルール管理 - 農業IoTシステム", CSSURL: "/static/css/style.css?v=dev"},
		HasDevice: false,
	}
	html := render(t, AlertRules(v))

	assertContains(t, html, "<html")
	assertContains(t, html, "アラートルール管理")
	assertContains(t, html, "デバイスがありません") // 設定可能なデバイスがない旨の案内
}
