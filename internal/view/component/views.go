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

// DeviceInfoView はデバイス詳細の情報パネル (DeviceInfoPanel) の表示データ。
// すべて整形済み primitive で保持し、pgtype/repository 型を持ち込まない (view 純粋性)。
// Location 未設定は "未設定"、LastCommText は最終通信を "YYYY-MM-DD HH:MM:SS"、
// 一度も通信が無い場合は "未通信" を handler 側で整形して渡す。
type DeviceInfoView struct {
	Name         string // デバイス名
	MacAddress   string // MAC アドレス
	Location     string // 設置場所 (未設定は "未設定")
	StatusActive bool   // true=● 稼働中 / false=○ 停止中
	LastCommText string // "2026-04-20 14:30:00" or "未通信"
	EditURL      string // 編集画面 URL "/devices/{id}/edit" (S4 提供)
}

// DeviceChartAreaView はグラフ領域フラグメント (DeviceChartArea) の表示データ。
// Period は active 判定用の現在期間 ("24h"/"7d"/"30d")、温度/湿度 SVG は internal/chart が
// 生成済みの文字列 (templ.Raw で埋め込む)。DeviceID は期間ボタンの hx-get/hx-push-url URL 用。
type DeviceChartAreaView struct {
	DeviceID       int64
	Period         string
	TemperatureSVG string
	HumiditySVG    string
}

// chartPeriod は期間切替ボタン1個の定義 (Value=クエリ値, Label=表示文言)。
type chartPeriod struct {
	Value string
	Label string
}

// chartPeriods は期間切替の選択肢 (24h/7d/30d の 3 つのみ, R3.1)。
var chartPeriods = []chartPeriod{
	{Value: "24h", Label: "24時間"},
	{Value: "7d", Label: "7日間"},
	{Value: "30d", Label: "30日間"},
}

// ReadingRow は最新計測テーブル1行分の表示データ (整形済み)。
// RecordedAt は "YYYY-MM-DD HH:MM"、Temp/Humidity は小数2桁の数値文字列 (単位は列見出し側)。
type ReadingRow struct {
	RecordedAt string // "2026-04-20 14:30"
	Temp       string // "28.50"
	Humidity   string // "65.30"
}

// LatestReadingsView は最新計測データテーブル (LatestReadingsTable) の表示データ。
// Rows は handler が新しい順・最大10件に整形済み (期間切替に非連動)。
// DeviceID は「もっと見る」導線 (計測履歴一覧 /devices/{id}/readings, S6) の URL 構築用。
type LatestReadingsView struct {
	DeviceID int64
	Rows     []ReadingRow
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
