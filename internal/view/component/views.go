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
// Period は active 判定用の現在期間 ("24h"/"3d"/"7d"/"30d")、View は表示形式 ("line"=折れ線 /
// "candle"=30分足ローソク足)。温度/湿度 SVG は internal/chart が生成済みの文字列 (templ.Raw で
// 埋め込む)。DeviceID は期間ボタン・形式トグルの hx-get/hx-push-url URL 用。
// View 未設定 ("") は既定の折れ線扱い (chartViewActive 参照)。
type DeviceChartAreaView struct {
	DeviceID       int64
	Period         string
	View           string
	CandleUnit     string // ローソク足の足ラベル ("15分足"/"30分足"/"1時間足"/"4時間足"、注記用。line 時は空)
	TemperatureSVG string
	HumiditySVG    string
}

// chartPeriod は期間切替ボタン1個の定義 (Value=クエリ値, Label=表示文言)。
type chartPeriod struct {
	Value string
	Label string
}

// chartPeriods は期間切替の選択肢 (24h/2d/3d/7d/30d)。時系列順に並べる。
// 2d (2日間=直近48時間) は30分足ローソク足が見やすい範囲として追加した。
var chartPeriods = []chartPeriod{
	{Value: "24h", Label: "24時間"},
	{Value: "2d", Label: "2日間"},
	{Value: "3d", Label: "3日間"},
	{Value: "7d", Label: "7日間"},
	{Value: "30d", Label: "30日間"},
}

// chartViewMode は表示形式トグル1個の定義 (Value=クエリ値, Label=表示文言)。
type chartViewMode struct {
	Value string
	Label string
}

// chartViewModes は表示形式の選択肢 (折れ線 / ローソク足)。
var chartViewModes = []chartViewMode{
	{Value: "line", Label: "ライン"},
	{Value: "candle", Label: "ローソク足"},
}

// chartViewActive はトグルボタン (mode) が現在の表示形式 (current) で active かを返す。
// "candle" は current=="candle" のときのみ active。"line" は既定のため current が "" / "line"
// (= candle 以外) のとき active とする (View 未設定は折れ線扱い)。
func chartViewActive(mode, current string) bool {
	if mode == "candle" {
		return current == "candle"
	}
	return current != "candle"
}

// chartViewOrDefault は View 未設定 ("") を既定の "line" に正規化する (期間ボタンの
// hx-get/hx-push-url 構築用)。期間ボタンは現在の表示形式を保持して往復するため、
// URL に view を必ず含める (ローソク足表示中に期間を変えてもローソク足のまま)。
func chartViewOrDefault(current string) string {
	if current == "" {
		return "line"
	}
	return current
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

// DeviceReadingsListView はセンサーデータ履歴のフィルタ結果領域 fragment
// (DeviceReadingsList, id=device-readings-list) の表示データ。
// 集計6項目・履歴行・データ有無・簡易ページャ・形式エラーマップを束ねる。
// HTMX 部分更新でこの DTO のみを差し替えるため、集計・一覧・ページャを内包する。
type DeviceReadingsListView struct {
	Summary    SummaryView         // 整形済み集計6項目 (期間全体・ページ非依存)
	Rows       []ReadingHistoryRow // 履歴一覧 (新しい順・最大20件)
	HasData    bool                // len(Rows) > 0。false でテーブル非表示+空状態メッセージ
	Pagination PaginationView      // 簡易ページャ (前へ / N・M / 次へ)
	Errors     map[string]string   // 日付形式エラー (field → 日本語メッセージ。空なら非表示)
}

// SummaryView は集計情報 (.summary-grid) の表示データ (整形済み)。
// 平均/最高/最低 × 温度/湿度 の6項目を、小数第2位+単位 (℃/%) 付き文字列で保持する。
// 該当データ0件 (sample_count==0) のときは全項目を "—" にして 0.00 の誤表示を避ける。
type SummaryView struct {
	AvgTemp, MaxTemp, MinTemp string // 例 "28.30℃" / "—"
	AvgHum, MaxHum, MinHum    string // 例 "65.30%" / "—"
}

// ReadingHistoryRow は履歴一覧テーブル1行分の表示データ (整形済み)。
// 既存の ReadingRow (3列) とは別型で、通信遅延 Delay を第4列に加える。
// RecordedAt は "YYYY-MM-DD HH:MM"、Temp/Humidity は小数2桁の数値文字列 (単位は列見出し側)、
// Delay は計測時刻とサーバ受信時刻の差を四捨五入した整数秒 ("N秒"、負値は "0秒")。
type ReadingHistoryRow struct {
	RecordedAt string // "2026-04-20 14:30"
	Temp       string // "28.50"
	Humidity   string // "65.30"
	Delay      string // "2秒"
}

// PaginationView は簡易ページャ (Pagination) の表示データ。
// 現在/総ページ番号と前後ページの有無、前後ページへの相対 URL を保持する。
// 番号ウィンドウは持たず「前へ / N・M ページ / 次へ」のみ (design Decision)。
type PaginationView struct {
	Current, Last    int    // 現在ページ / 総ページ数 (ともに 1 以上)
	HasPrev, HasNext bool   // 前へ/次へリンクの表示可否
	PrevURL, NextURL string // from/to を保持し page を差し替えた相対 URL
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

// PageLink は番号付きページネーションのページ番号リンク1個 (AlertHistoryPagination 用)。
// Num は表示するページ番号、URL は検索条件を保持し page のみ差し替えた相対 URL、
// Current は現在ページ (true なら <a> ではなく .current の span で描画) を表す。
type PageLink struct {
	Num     int    // ページ番号
	URL     string // そのページへの相対 URL (現在ページは未使用)
	Current bool   // 現在ページ判定
}

// AlertHistoryPaginationView は番号付きページャ (AlertHistoryPagination) の表示データ。
// 既存の簡易 PaginationView (前/次のみ) と異なり、ページ番号リンク配列 Pages を持つ
// (mocks/html/alert-history.html の番号 1,2,3 + 前/次 を写経・design Decision「番号付き新規部品」)。
// 前へ/次への有無と相対 URL を保持し、リンクは handler 生成の信頼 URL (templ.SafeURL で埋める)。
type AlertHistoryPaginationView struct {
	HasPrev, HasNext bool       // 前へ/次へリンクの表示可否 (端ページで false→.disabled)
	PrevURL, NextURL string     // 検索条件を保持し page を差し替えた相対 URL
	Pages            []PageLink // ページ番号リンク (現状 1..Last を列挙・IoT 小規模)
}

// AlertHistoryRow はアラート履歴一覧テーブル1行分の表示データ (整形済み primitive のみ)。
// すべて発火時点の非正規化値を handler が整形して詰める (R4.7)。pgtype/repository 型は持ち込まない。
type AlertHistoryRow struct {
	TriggeredAt string // 発火日時 "YYYY-MM-DD HH:MM" (JST)
	DeviceName  string // デバイス名
	MetricLabel string // 指標ラベル "温度"/"湿度"
	Condition   string // ルール条件 "> 35.00℃" (演算子記号 + 閾値2桁 + 単位)
	ActualValue string // 実測値 "38.50℃" (数値2桁 + 単位)
	Notified    string // 通知状態 "済"/"未"
}

// AlertHistoryListView はアラート履歴のフィルタ結果領域 fragment
// (AlertHistoryList, id=alert-history-list) の表示データ。
// HTMX 部分更新でこの DTO のみを差し替えるため、一覧・空状態・ページャ・エラーを内包する
// (OOB 不使用・readings 踏襲・design Decision 1)。
type AlertHistoryListView struct {
	Rows          []AlertHistoryRow          // 履歴一覧 (発火日時の新しい順・最大20件)
	HasData       bool                       // len(Rows) > 0。false でテーブル非表示+空状態メッセージ
	HasPagination bool                       // 0件・1ページのみは false (R6.2 ページャ非表示)
	Pagination    AlertHistoryPaginationView // 番号付きページャ
	Errors        map[string]string          // from/to のインラインエラー (空なら非表示)
}
