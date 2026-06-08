package component

import (
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/domain"
)

// assertNotContains は html に substr が含まれないことを検証する (render/assertContains は component_test.go)。
func assertNotContains(t *testing.T, html, substr string) {
	t.Helper()
	if strings.Contains(html, substr) {
		t.Errorf("出力に %q が含まれてはならない:\n%s", substr, html)
	}
}

// --- 3.1 AlertRuleRow ---

func TestAlertRuleRow_温度ルールを描画(t *testing.T) {
	v := AlertRuleRowView{
		ID: 1, Metric: domain.MetricTemperature, Operator: domain.OpGreaterThan,
		Threshold: 35.00, IsEnabled: true,
	}
	html := render(t, AlertRuleRow(v))

	assertContains(t, html, `id="alert-rule-row-1"`)
	assertContains(t, html, "温度")
	assertContains(t, html, "より大きい")
	assertContains(t, html, "35.00℃")
	// 有効切替トリガ (当該行のみ outerHTML 差し替え)
	assertContains(t, html, "hx-patch")
	assertContains(t, html, "/alerts/rules/1/toggle")
	assertContains(t, html, `hx-swap="outerHTML"`)
	assertContains(t, html, "checked") // IsEnabled=true
	// 編集読込 (フォーム差し替え) と削除 (確認付き)
	assertContains(t, html, "/alerts/rules/1/edit")
	assertContains(t, html, "hx-delete")
	assertContains(t, html, "hx-confirm")
	assertContains(t, html, "削除しますか")
}

func TestAlertRuleRow_湿度ルールと無効状態を描画(t *testing.T) {
	v := AlertRuleRowView{
		ID: 2, Metric: domain.MetricHumidity, Operator: domain.OpLessThan,
		Threshold: 30.00, IsEnabled: false,
	}
	html := render(t, AlertRuleRow(v))

	assertContains(t, html, `id="alert-rule-row-2"`)
	assertContains(t, html, "湿度")
	assertContains(t, html, "より小さい")
	assertContains(t, html, "30.00%")
	assertNotContains(t, html, "checked") // IsEnabled=false は checked を出さない
}

// --- 3.2 AlertRuleList ---

func TestAlertRuleList_0件で空状態メッセージ(t *testing.T) {
	html := render(t, AlertRuleList(nil))

	assertContains(t, html, `id="alert-rule-list"`)
	assertContains(t, html, "このデバイスにはアラートルールが設定されていません")
	assertNotContains(t, html, "data-table") // 表は描画しない
}

func TestAlertRuleList_N件で行を描画(t *testing.T) {
	rules := []AlertRuleRowView{
		{ID: 1, Metric: domain.MetricTemperature, Operator: domain.OpGreaterThan, Threshold: 35, IsEnabled: true},
		{ID: 2, Metric: domain.MetricHumidity, Operator: domain.OpLessThan, Threshold: 30, IsEnabled: true},
	}
	html := render(t, AlertRuleList(rules))

	assertContains(t, html, `id="alert-rule-list"`)
	assertContains(t, html, "data-table")
	assertContains(t, html, `id="alert-rule-row-1"`)
	assertContains(t, html, `id="alert-rule-row-2"`)
	if n := strings.Count(html, `id="alert-rule-row-`); n != 2 {
		t.Errorf("行数: got %d, want 2", n)
	}
	assertNotContains(t, html, "設定されていません") // 空状態は出さない
}

// --- 3.3 AlertRuleForm ---

func TestAlertRuleForm_追加モード(t *testing.T) {
	v := AlertRuleFormView{DeviceID: 10}
	html := render(t, AlertRuleForm(v))

	assertContains(t, html, `id="alert-rule-form"`)
	assertContains(t, html, `hx-post="/alerts/rules"`)
	assertContains(t, html, `hx-target="#alert-rule-section"`)
	assertContains(t, html, "ルールを追加")
	// hidden device_id
	assertContains(t, html, `name="device_id"`)
	assertContains(t, html, `value="10"`)
	// 指標/条件の選択肢
	assertContains(t, html, "温度")
	assertContains(t, html, "湿度")
	assertContains(t, html, "より大きい")
	assertContains(t, html, "以下")
	assertNotContains(t, html, "hx-put") // 追加モードでは更新送信属性を出さない
}

func TestAlertRuleForm_編集モードで既存値復元と更新送信(t *testing.T) {
	v := AlertRuleFormView{
		DeviceID: 10, EditingRuleID: 5,
		Metric: "temperature", Operator: ">", Threshold: "35.00",
	}
	html := render(t, AlertRuleForm(v))

	assertContains(t, html, `hx-put="/alerts/rules/5"`)
	assertContains(t, html, "更新")
	assertContains(t, html, `value="35.00"`)                // threshold 復元
	assertContains(t, html, `value="temperature" selected`) // metric の選択状態復元
	assertContains(t, html, `&gt;" selected`)               // operator(>) の選択状態復元 (value は &gt; にエスケープ)
	assertContains(t, html, "キャンセル")                        // 編集モードのキャンセル導線
	assertNotContains(t, html, `hx-post="/alerts/rules"`)
}

func TestAlertRuleForm_項目別エラーと入力復元(t *testing.T) {
	v := AlertRuleFormView{
		DeviceID: 10, Metric: "humidity", Threshold: "abc",
		Errors: map[string]string{
			"metric":    "指標を選択してください",
			"threshold": "数値を入力してください",
		},
	}
	html := render(t, AlertRuleForm(v))

	assertContains(t, html, "error-message")
	assertContains(t, html, "指標を選択してください")
	assertContains(t, html, "数値を入力してください")
	assertContains(t, html, `value="abc"`) // 不正入力でも値を復元
}

// --- 3.4 AlertRuleSection ---

func TestAlertRuleSection_フォームと一覧を内包(t *testing.T) {
	v := AlertRuleSectionView{
		DeviceID: 10,
		Form:     AlertRuleFormView{DeviceID: 10},
		Rules: []AlertRuleRowView{
			{ID: 1, Metric: domain.MetricTemperature, Operator: domain.OpGreaterThan, Threshold: 35, IsEnabled: true},
		},
	}
	html := render(t, AlertRuleSection(v))

	assertContains(t, html, `id="alert-rule-section"`)
	assertContains(t, html, `id="alert-rule-form"`) // 追加フォーム
	assertContains(t, html, `id="alert-rule-list"`) // 一覧
	assertContains(t, html, `id="alert-rule-row-1"`)
}
