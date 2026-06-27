// Package component は再利用可能な templ 表示部品 (ヘッダー・サイドバー・カード等) を提供する。
// カード/バナー部品が描画する表示用 DTO もここで定義する。これらは整形済み primitive
// のみを持ち、repository 型や pgtype を持ち込まない (view 純粋性)。page パッケージは
// component を import して部品を描画するため、共有 DTO を component 側に置くことで
// page ↔ component の循環 import を避ける。
package component

import "fmt"

// DashboardDevice はデバイスカード1枚 (DeviceCard) の表示データ。
// 温度・湿度・最終通信はすべて整形済み文字列で保持する
// (未受信は TempText/HumidityText が "ー"、通信実績なしは LastCommText が "通信実績なし")。
type DashboardDevice struct {
	ID           int64
	Name         string
	Location     string // 所在地の認識名 (構造化 locality の Locality.Label())。未設定は "" (モックは「場所: 」を表示)
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
// Location は所在地の認識名 (構造化 locality の Locality.Label())・未設定は "未設定"、
// LastCommText は最終通信を "YYYY-MM-DD HH:MM:SS"、一度も通信が無い場合は "未通信" を
// handler 側で整形して渡す。
type DeviceInfoView struct {
	Name         string // デバイス名
	MacAddress   string // MAC アドレス
	Location     string // 所在地の認識名 (構造化 locality の Locality.Label()。未設定は "未設定")
	StatusActive bool   // true=● 稼働中 / false=○ 停止中
	LastCommText string // "2026-04-20 14:30:00" or "未通信"
	Crop         string // 栽培作物の日本語ラベル (domain.Crop.Label()。VPD 適正帯の根拠・センサー毎。未設定/不正は "未設定")
	EditURL      string // 編集画面 URL "/devices/{id}/edit" (S4 提供)
}

// DeviceChartAreaView はグラフ領域フラグメント (DeviceChartArea) の表示データ。
// Period は active 判定用の現在期間 ("24h"/"3d"/"7d"/"30d")、DeviceID は期間ボタンの
// hx-get/hx-push-url URL 用。
//
// グラフは Apache ECharts (クライアント描画) へ移行済み: 温度/湿度それぞれの option JSON
// (internal/chart が go-echarts で構築・HTML 安全) を <script type="application/json"> で供給し、
// Unit/Color は ECharts コンテナの data-unit/data-color へ渡す (endLabel formatter・線色用)。
// HasData=false (計測 0 件) のときは option を構築せず空メッセージのみ描画する。
//
// 数値カード (TemperatureCard/HumidityCard) は常時表示 (HasData=false でも各値 "—"・R1.4)。
// 日次集計表 (TemperatureDaily/HumidityDaily) は ShowDaily=true (複数日かつデータ有り) のときのみ描画する (R5.3)。
type DeviceChartAreaView struct {
	DeviceID              int64
	Period                string
	HasData               bool
	TemperatureOptionJSON string // <script type="application/json"> 埋込用 HTML 安全 JSON
	HumidityOptionJSON    string
	TemperatureUnit       string // "℃" (data-unit へ)
	HumidityUnit          string // "%"
	TemperatureColor      string // "#e8590c" (data-color へ)
	HumidityColor         string // "#1971c2"

	// 数値カード (現在値・最高・最低・日較差／温度・湿度別)。常時表示・空は各値 "—"。
	TemperatureCard StatCardView
	HumidityCard    StatCardView

	// 日次集計表 (複数日のみ)。ShowDaily=false (24h or 空) のとき非表示。
	ShowDaily        bool
	TemperatureDaily []DailyStatRow
	HumidityDaily    []DailyStatRow

	// VPD (飽差) 適正帯ダッシュボードのパネル。HasData=true のときのみ handler が組む
	// (温湿度データから読み取り時算出)。HasData=false では空の VPDPanelView (templ 側で
	// 温湿度同様 if HasData 内に描画するため非表示)。温湿度フィールドとは独立 (無回帰)。
	VPD VPDPanelView
}

// VPDPanelView は VPD 適正帯ダッシュボードのパネル (DeviceChartArea 内・研究用) の表示データ。
// すべて整形済み primitive で保持し、pgtype/repository 型を持ち込まない (view 純粋性)。
// OptionJSON は VPD line + 適正帯3ゾーン markArea + VPD移動平均を内包した HTML 安全 JSON
// (internal/chart.VPDChartOptionJSON が構築)。Color は VPD 線の基準色 (data-color へ)。
// CropLabel は作物名 (未設定は "既定")、Lower/UpperLabel は適正帯の上下限表示文字列、
// Card は VPD 数値カード、InRangeRatio は滞在率バーの幅 (0..1)、Hourly は時間帯別逸脱行。
type VPDPanelView struct {
	OptionJSON   string        // <script type="application/json"> 埋込用 HTML 安全 JSON (markArea 内包)
	Color        string        // VPD 線の基準色 "#0ca678" (data-color へ)
	CropLabel    string        // 作物名 "ゴーヤ" or "既定"
	LowerLabel   string        // 適正帯下限 "0.40 kPa"
	UpperLabel   string        // 適正帯上限 "1.20 kPa"
	Card         VPDCardView   // VPD 数値カード (現在/期間平均/滞在率/最大逸脱)
	InRangeRatio float64       // 適正帯滞在率 0..1 (滞在率バーの動的幅用)
	Hourly       []VPDHourlyRow // 時間帯別逸脱 (JST 時刻バケット昇順・データのある時間帯のみ)
}

// VPDCardView は VPD 数値カード1枚分の表示データ (整形済み・単位付き文字列 or "—")。
// 現在VPD=最新点、期間平均=系列平均、滞在率=適正帯在帯割合(%)、最大逸脱=適正帯から最も外れた量と方向。
type VPDCardView struct {
	CurrentVPD   string // 例 "0.90 kPa" / "—"
	AverageVPD   string // 例 "1.05 kPa" / "—"
	TimeInRange  string // 例 "72%" / "—"
	MaxDeviation string // 例 "+0.40 kPa（乾き）" / "—"（上限超=乾きすぎ "+", 下限割れ=湿りすぎ "-"）
}

// VPDHourlyRow は時間帯別 VPD 逸脱表1行分の表示データ (整形済み)。
// Hour は JST 時刻バケット "06:00"、AvgVPD はバケット平均VPD (素の数値・単位は列見出し側)、
// InRangePercent は在帯率 "40%"、Direction は主な逸脱方向 "乾き"/"湿り"/"—"。
type VPDHourlyRow struct {
	Hour           string // "06:00"
	AvgVPD         string // "0.35" (単位なし・列見出しに kPa)
	InRangePercent string // "40%"
	Direction      string // "乾き" / "湿り" / "—"
}

// StatCardView は数値カード1メトリック分の表示データ (整形済み・単位付き文字列 or "—")。
// 現在値=最新計測点、最高/最低=期間内、日較差=最高−最低 (R1.1)。
type StatCardView struct {
	Current string // 例 "28.50℃" / "—"
	Max     string // 例 "35.20℃" / "—"
	Min     string // 例 "18.50℃" / "—"
	Diurnal string // 日較差 例 "16.70℃" / "—"
}

// DailyStatRow は日次集計表1行分の表示データ (整形済み・欠測は "—")。
// 単位は列見出し側に付くためセルは素の数値文字列 (CV は無次元のため単位なし)。
type DailyStatRow struct {
	Date    string // "2026-04-20"
	Avg     string // 平均
	Max     string // 最高
	Min     string // 最低
	Diurnal string // 日較差 (最高−最低)
	Sigma   string // 標準偏差σ
	CV      string // 変動係数 (σ/μ・無次元)・未定義は "—"
}

// vpdBarWidthStyle は適正帯滞在率 (0..1) を滞在率バー塗りの style 文字列 "width:NN%" に整形する。
// 整数パーセントへ丸める (バー幅は視覚表現ゆえ小数不要)。0..1 外は CSS 側で 0..100% にクランプされる。
func vpdBarWidthStyle(ratio float64) string {
	return fmt.Sprintf("width:%.0f%%", ratio*100)
}

// optionScript は ECharts option JSON を <script type="application/json"> でクライアントへ
// 安全供給するためのスクリプトタグ文字列を返す (§10-E)。templ は <script> 要素内の式
// (@templ.Raw 等) を解釈せずリテラル出力してしまうため、スクリプトタグごと組み立てて
// templ 側で @templ.Raw に渡す。jsonStr は internal/chart が encoding/json で HTML 安全化
// 済み (< > & は \uXXXX) のため </script> は出現しえず、Raw 出力でも XSS にならない。
// id は静的な定数 ("temperature-chart-option" 等) のみを渡す前提。
func optionScript(id, jsonStr string) string {
	return `<script type="application/json" id="` + id + `">` + jsonStr + `</script>`
}

// chartPeriod は期間切替ボタン1個の定義 (Value=クエリ値, Label=表示文言)。
type chartPeriod struct {
	Value string
	Label string
}

// chartPeriods は期間切替の選択肢 (24h/3d/7d/30d の 4 つ, R3.1)。時系列順に並べる。
var chartPeriods = []chartPeriod{
	{Value: "24h", Label: "24時間"},
	{Value: "3d", Label: "3日間"},
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

// DeviceReadingsListView はセンサーデータ履歴のフィルタ結果領域 fragment
// (DeviceReadingsList, id=device-readings-list) の表示データ。
// 集計6項目・履歴行・データ有無・簡易ページャ・形式エラーマップを束ねる。
// HTMX 部分更新でこの DTO のみを差し替えるため、集計・一覧・ページャを内包する。
//
// Report/CSVURL は CSV エクスポート/集計帳票フェーズ (sensor-data-export) の追加分。
// Report は日次/時間別の集計帳票 View、CSVURL は適用済み from/to/items を反映した CSV
// ダウンロードリンク。ともに handler が結線する (空 Report は HasData=false で非表示、空
// CSVURL はボタン非描画) ため既存の集計/一覧/ページャ経路は無回帰 (追加のみ)。
type DeviceReadingsListView struct {
	Summary    SummaryView         // 整形済み集計6項目 (期間全体・ページ非依存)
	Rows       []ReadingHistoryRow // 履歴一覧 (新しい順・最大20件)
	HasData    bool                // len(Rows) > 0。false でテーブル非表示+空状態メッセージ
	Pagination PaginationView      // 簡易ページャ (前へ / N・M / 次へ)
	Errors     map[string]string   // 日付形式エラー (field → 日本語メッセージ。空なら非表示)
	Report     ReadingsReportView  // 日次/時間別の集計帳票 (空帳票は HasData=false で非表示)
	CSVURL     string              // CSV ダウンロードリンク (適用済み from/to/items 反映。空ならボタン非描画)
}

// ReadingsReportRow は集計帳票1バケット分 (日次=1日/時間別=1時間帯) の表示データ
// (整形済み primitive・欠測や未定義は "—")。温度・湿度それぞれ 平均/最高/最低/日較差/σ/CV を
// 持ち、末尾に適正帯滞在率 InRange を置く。単位は列見出し側に付くためセルは素の数値文字列
// (CV は無次元のため単位なし)。将来 P6 で露点/結露時間列をこの末尾へ非破壊追加できる構造に留める。
type ReadingsReportRow struct {
	Bucket      string // 日次 "2026-04-20" / 時間別 "06時"
	TempAvg     string // 温度 平均
	TempMax     string // 温度 最高
	TempMin     string // 温度 最低
	TempDiurnal string // 温度 日較差 (最高−最低)
	TempSigma   string // 温度 標準偏差σ
	TempCV      string // 温度 変動係数 (σ/μ・無次元)・未定義は "—"
	HumAvg      string // 湿度 平均
	HumMax      string // 湿度 最高
	HumMin      string // 湿度 最低
	HumDiurnal  string // 湿度 日較差
	HumSigma    string // 湿度 標準偏差σ
	HumCV       string // 湿度 変動係数・未定義は "—"
	InRange     string // 適正帯滞在率 "72%" / 欠測は "—"
}

// ReadingsReportView は集計帳票 (日次/時間別) の表示データ。
// CropLabel は適正帯の根拠作物 ("ゴーヤ"/"既定")、RangeLabel は適正帯 ("0.40〜1.20 kPa")、
// Daily は日付昇順・Hourly は時刻昇順の帳票行。HasData=false (空期間) のとき帳票表を描画しない
// (数値を捏造しない)。CropLabel/RangeLabel は空期間でもヘッダ表示用に保持する。
type ReadingsReportView struct {
	CropLabel  string              // 適正帯の作物 "ゴーヤ" / "既定"
	RangeLabel string              // 適正帯 "0.40〜1.20 kPa"
	Daily      []ReadingsReportRow // 日次バケット (日付昇順・欠測日は "—" 行)
	Hourly     []ReadingsReportRow // 時間別バケット (時刻昇順・データのある時間帯のみ)
	HasData    bool                // 計測あり。false で帳票表非表示
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

// SelectOption は検索可能 select (Tom Select) の選択肢 1 件 (文字列値キー版)。
// 既存 DeviceOption は ID(int64) キーで地域 (文字列値) に不適合のため別 DTO とする。
// 地域 select 等で handler が Selected 込みの選択肢を組み、templ は domain を直接 range せず
// この DTO を描画する (選択値復元の一貫性のため)。
type SelectOption struct {
	Value    string // option の value 属性 (例 "佐敷町")
	Label    string // 表示文言 (例 "佐敷（南城市）")
	Selected bool   // 現在選択中なら true (selected 属性で復元)
}

// DeviceFormView は登録/編集で共有するデバイスフォーム (DeviceForm) の描画パラメータ。
// 認証後レイアウト (layout.AppLayoutData) は持たない —— layout が component を import する
// ため逆向きの import は循環になる。レイアウトは page 側のラッパ (page.DeviceFormView) が担い、
// ここはフォーム本体の描画に必要な値だけを保持する。
// IsActive は radio の選択状態復元用に "1"(稼働中)/"0"(停止中) の文字列で持つ。
// 設置場所は単一の検索可能 select で沖縄の地域 (domain.Locality・53) から1つ選ぶ
// (旧来の location 自由入力を置換)。Locality は復元用の選択値、Localities は handler が
// domain.AllLocalities() から組んだ Selected 込みの選択肢。
type DeviceFormView struct {
	CSRFToken  string            // hidden gorilla.csrf.Token 用
	Action     string            // 送信先 "/devices"(登録) / "/devices/{id}"(編集)
	IsEdit     bool              // true で hidden _method=put を出し、ボタンを「更新」にする
	CancelURL  string            // キャンセル先 "/dashboard"(登録) / "/devices/{id}"(編集)
	Name       string            // 入力値復元
	MacAddress string            // 入力値復元
	Locality   string            // 設置場所の選択値 (domain.Locality の値・未設定は "")
	Localities []SelectOption    // 地域 select の選択肢 (Selected 込み)
	Crop       string            // 栽培作物の選択値 (domain.Crop の値・未設定は ""=既定しきい値)
	Crops      []SelectOption    // 作物 select の選択肢 (Selected 込み・locality と同型)
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
