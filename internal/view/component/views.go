// Package component は再利用可能な templ 表示部品 (ヘッダー・サイドバー・カード等) を提供する。
// カード/バナー部品が描画する表示用 DTO もここで定義する。これらは整形済み primitive
// のみを持ち、repository 型や pgtype を持ち込まない (view 純粋性)。page パッケージは
// component を import して部品を描画するため、共有 DTO を component 側に置くことで
// page ↔ component の循環 import を避ける。
package component

// DashboardDevice はデバイスカード1枚 (DeviceCard) の表示データ。
// 温度・湿度・最終通信はすべて整形済み文字列で保持する
// (未受信は TempText/HumidityText が "ー"、通信実績なしは LastCommText が "通信実績なし")。
type DashboardDevice struct {
	ID           int64
	Name         string
	Location     string // 未設定は "" (モックは「場所: 」を表示)
	IsActive     bool
	TempText     string // "28.50℃" or "ー"
	HumidityText string // "65.30%" or "ー"
	LastCommText string // "2分前" or "通信実績なし"
}

// DashboardAlert は未対応アラート1件 (UnhandledAlertBanner の1行) の表示データ。
// Message は handler の composeAlertMessage が合成した自然文を保持する。
type DashboardAlert struct {
	Message string // 例: "ハウスA温湿度計: 温度が35℃を超えました（38.50℃）"
}

// DeviceFormView は登録/編集で共有するデバイスフォーム (DeviceForm) の描画パラメータ。
// 認証後レイアウト (layout.AppLayoutData) は持たない —— layout が component を import する
// ため逆向きの import は循環になる。レイアウトは page 側のラッパ (page.DeviceFormView) が担い、
// ここはフォーム本体の描画に必要な値だけを保持する。
// IsActive は radio の選択状態復元用に "1"(稼働中)/"0"(停止中) の文字列で持つ。
type DeviceFormView struct {
	CSRFToken  string            // hidden gorilla.csrf.Token 用
	Action     string            // 送信先 "/devices"(登録) / "/devices/{id}"(編集)
	IsEdit     bool              // true で hidden _method=put を出し、ボタンを「更新」にする
	CancelURL  string            // キャンセル先 "/dashboard"(登録) / "/devices/{id}"(編集)
	Name       string            // 入力値復元
	MacAddress string            // 入力値復元
	Location   string            // 入力値復元
	IsActive   string            // "1"/"0" の radio checked 復元用
	Errors     map[string]string // field → 日本語メッセージ
}
