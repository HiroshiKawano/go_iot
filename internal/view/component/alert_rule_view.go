package component

import "github.com/HiroshiKawano/go_iot/internal/domain"

// DeviceOption はデバイス選択欄 (検索可能 select) の選択肢 1 件。
// Selected で現在選択中デバイスを復元する。所有デバイスのみを handler が詰める。
type DeviceOption struct {
	ID       int64
	Name     string
	Selected bool
}

// AlertRuleFormView は追加/編集兼用フォーム (AlertRuleForm) の描画パラメータ。
// EditingRuleID==0 で追加モード (送信先 POST /alerts/rules・ボタン「ルールを追加」)、
// >0 で編集モード (送信先 PUT /alerts/rules/{id}・ボタン「更新」+ キャンセル導線) に分岐する。
// Metric/Operator は DB 格納値 (temperature 等 / 記号 >)、Threshold は入力文字列のまま保持して
// バリデーションエラー時に選択状態・入力値を復元する。Errors は項目名→日本語メッセージ。
// 認証後レイアウトは持たない (layout → component の import 方向を保ち循環を避ける)。
type AlertRuleFormView struct {
	DeviceID      int64             // hidden。追加/更新の対象デバイス
	EditingRuleID int64             // 0=追加モード, >0=編集モード
	Metric        string            // 復元用 (temperature/humidity の DB 値)
	Operator      string            // 復元用 (記号 > / < / >= / <=)
	Threshold     string            // 復元用 (入力文字列のまま保持)
	Errors        map[string]string // 項目名(metric/operator/threshold)→日本語メッセージ
}

// AlertRuleRowView はルール一覧 1 行 (AlertRuleRow) の表示データ。
// Metric/Operator は domain 型のまま保持し、templ で表示メソッド (Label()/Unit()) を呼ぶ
// (view → domain の表示メソッドのみ許可。pgtype/repository 型は持ち込まない)。
type AlertRuleRowView struct {
	ID        int64
	Metric    domain.Metric             // Label()/Unit() を templ で使用
	Operator  domain.ComparisonOperator // Label()
	Threshold float64                   // 表示は %.2f + Unit()
	IsEnabled bool
}

// AlertRuleSectionView は #alert-rule-section (追加/編集フォーム + ルール一覧) の表示データ。
// デバイス切替・追加・更新時にこの領域全体を innerHTML 差し替えする単位。
type AlertRuleSectionView struct {
	DeviceID int64
	Form     AlertRuleFormView
	Rules    []AlertRuleRowView
}
