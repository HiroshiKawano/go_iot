// Package page は各画面のフルページ templ コンポーネントを提供する。
// ハンドラはここで定義する View 構造体を組み立てて各ページを描画する
// (binding 構造体は handler 側に置き、view → handler の循環依存を避ける)。
package page

import (
	"github.com/HiroshiKawano/go_iot/internal/view/component"
	"github.com/HiroshiKawano/go_iot/internal/view/layout"
)

// DashboardView はダッシュボード全体の描画データ (表示用 primitive のみ)。
// pgtype やリポジトリ型は持ち込まず、整形は handler 側で完結させる (view 純粋性)。
// デバイス/アラートの1件分 DTO は component が所有する (page ↔ component の循環回避)。
type DashboardView struct {
	Layout  layout.AppLayoutData // Title/UserName/CSRFToken/CSSURL/Flash
	Devices []component.DashboardDevice
	Alerts  []component.DashboardAlert
}

// DeviceShowView はデバイス詳細フルページ (DeviceShowPage) の描画データ。
// 認証後レイアウト Layout に、情報パネル・グラフ領域・最新計測テーブルの各 component DTO を束ねる。
// 整形済み primitive のみを保持し pgtype/repository 型は持ち込まない (view 純粋性)。
// DeviceID は削除 (hx-delete) URL、DeleteName は削除確認モーダルに表示するデバイス名。
// ページ見出しのデバイス名は Info.Name を流用する。
type DeviceShowView struct {
	Layout     layout.AppLayoutData
	DeviceID   int64
	Info       component.DeviceInfoView
	ChartArea  component.DeviceChartAreaView
	Latest     component.LatestReadingsView
	DeleteName string
}

// ReadingsView はセンサーデータ履歴フルページ (ReadingsPage) の描画データ。
// 認証後レイアウト Layout に、見出し用デバイス名・フィルタフォームの echo 値 (From/To)・
// 結果領域 component DTO (List) を束ねる。整形済み primitive のみを保持し
// pgtype/repository 型は持ち込まない (view 純粋性)。
// From/To は未指定時 "" (入力欄の value 復元用)、List は HTMX 部分更新で差し替える結果領域。
type ReadingsView struct {
	Layout     layout.AppLayoutData             // Title/UserName/CSRFToken/CSSURL
	DeviceID   int64                            // フィルタフォーム・ページャの URL 構築用
	DeviceName string                           // page-header 見出し「センサーデータ履歴: {DeviceName}」用
	From       string                           // フィルタフォーム echo (未指定は "")
	To         string                           // フィルタフォーム echo (未指定は "")
	List       component.DeviceReadingsListView // フィルタ結果領域 (集計+一覧+ページャ)
}

// LoginView はログイン画面の描画に必要なデータ。
// Email は再描画時の入力値再表示用。Errors は field 名 → 日本語メッセージ
// ("form" キーはフォーム全体に対する汎用エラー = 認証失敗の共通メッセージ)。
type LoginView struct {
	CSSURL    string
	CSRFToken string
	Email     string
	Errors    map[string]string
}

// RegisterView はユーザー登録画面の描画に必要なデータ。
type RegisterView struct {
	CSSURL    string
	CSRFToken string
	Name      string
	Email     string
	Errors    map[string]string
}

// AlertRulesPageView はアラートルール管理フルページ (AlertRules) の描画データ。
// 認証後レイアウト Layout に、デバイス選択肢 Devices (所有デバイスのみ) と
// 選択中デバイスのセクション (フォーム + 一覧) を束ねる。HasDevice==false は所有デバイス 0 件で、
// デバイス選択・セクションの代わりに案内文を表示する。Section はフォーム部品が所有する
// component 側 DTO (layout を内包しない。layout → component の import 方向を保ち循環を避ける)。
type AlertRulesPageView struct {
	Layout    layout.AppLayoutData           // App レイアウト (Title/UserName/CSRFToken/CSSURL/Flash)
	Devices   []component.DeviceOption       // デバイス選択肢 (所有デバイスのみ)
	HasDevice bool                           // false=所有デバイス 0 件 → 案内表示
	Section   component.AlertRuleSectionView // 選択中デバイスのフォーム + 一覧
}

// DeviceFormView はデバイス登録/編集ページ (DeviceCreatePage/DeviceEditPage) の描画データ。
// 登録/編集で単一の View を共有する (画面差分は Form.IsEdit/Action/CancelURL と DeviceName のみ)。
// 認証後レイアウト用の Layout と、編集見出し「デバイス編集: {DeviceName}」用の DeviceName、
// 共有フォーム本体へ渡す Form を束ねる。Form はフォーム部品が所有する component 側 DTO で、
// レイアウトを内包しない (layout → component の import 方向を保ち循環を避ける)。
type DeviceFormView struct {
	Layout     layout.AppLayoutData     // App レイアウト (Title/UserName/CSRFToken/CSSURL)
	DeviceName string                   // 編集見出し用 (登録時は未使用)
	Form       component.DeviceFormView // 共有フォーム描画パラメータ
}
