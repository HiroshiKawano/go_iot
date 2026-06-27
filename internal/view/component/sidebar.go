package component

// NavPage は認証後サイドバーの「現在表示中の画面」を表す識別子。
// ゼロ値 "" はどのメニュー項目も active にしない (デバイス登録/編集など対応項目なし)。
type NavPage string

const (
	NavDashboard    NavPage = "dashboard"
	NavDeviceShow   NavPage = "device-show"
	NavReadings     NavPage = "readings"
	NavAlertRules   NavPage = "alert-rules"
	NavAlertHistory NavPage = "alert-history"
)

// SidebarNav はサイドバー描画に必要なナビ文脈。
// Current=active 判定、DeviceID>0 でデバイス文脈リンク (詳細/履歴) を表示する。
// 単一フィールド Current ゆえ、同時に active となる項目は構造的に1つ以下に保たれる。
type SidebarNav struct {
	Current  NavPage // 現在ページ。"" はどれも非 active
	DeviceID int64   // >0 で文脈リンク表示・リンク先 ID。0 は文脈なし
}
