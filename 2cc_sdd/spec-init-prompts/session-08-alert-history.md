# セッション8 spec-init プロンプト: アラート履歴

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: alert-history
> 実行例: /kiro-spec-init "（本文を貼り付け）"
> 前提セッション: S1（基盤）、S2（履歴データを生成。seed でも代替可）
> 設計フェーズで参照: 画面設計書(静的).md §8（524-580）・§ページネーション件数（606-614）、HTMX実装ガイド(動的).md §3.7（1319-1328）・§4 alert-history（1565-1584）・§10 ページネーション（1973-2023）・§16 Tom Select、DB設計書.md alert_histories 表定義（482-511）・sqlc リレーション方針（514-527）、mocks/html/alert-history.html

--- spec-init 本文 ここから ---

## 機能概要

農場運営者は、発火したアラートの履歴を時系列で確認し、デバイス・期間でフィルタ検索して対応状況を把握する必要がある。
現状、アラート履歴テーブルは完全に実装されているが、Web UI（画面表示・フィルタ・ページネーション）は全く未着手である。
本セッションで、`/alerts/history` ページを templ + HTMX で実装し、デバイス・開始日・終了日によるフィルタ検索と 20件/ページのページネーション機能を提供するものとする。

## 背景・現状

**実装現状サマリより:**
- **実装済み**: DB（alert_histories テーブル）、sqlc クエリ 5件（`ListAlertHistoriesPaginated` / `CountAlertHistoriesInRange` / `ListUnnotifiedAlertHistoriesWithDevice` / `MarkAlertHistoryNotified` / `CreateAlertHistory`）、アラート履歴の非正規化保持（metric/operator/threshold/actual_value は発火時点の値をそのまま格納）。
- **未実装**: Web UI 層すべて（templ コンポーネント・ハンドラ・HTMX 属性）、scs Session 認証（S1で整備済みと仮定）。
- **前提**: S1（基盤、SessionLoad・MethodOverride ミドルウェア、共通 templ レイアウト）が完成していることを前提。S2 または seed で履歴データが存在する状態。

**DB設計書より:**
- `alert_histories` → `alert_rules` → `devices` の3テーブル JOIN で非正規化された発火時点の値（metric/operator/threshold）と実測値（actual_value）、デバイス名、通知状態（is_notified）を取得。
- `ListAlertHistoriesPaginated` は `sqlc.narg('device_id')` で NULL 指定により「全デバイス」をサポート（生成コード上は `*int64`）。

## このセッションのスコープ（実装対象）

**ページ・ハンドラ構成:**
- **GET /alerts/history** → `AlertHistoryHandler.List(c *gin.Context)`
  - 返却: フルページ時は `AlertHistory.templ`（共通レイアウト + ページ全体）
  - 返却: HTMX 時（HX-Request）は `AlertHistoryList.templ`（tbody + pagination Fragment）
  - ページングは常に1始まり、未指定時は 1 にデフォルト

**フォーム・パラメータ:**
- フィルターフォーム（`method="GET"`、R17 準拠）:
  - `device_id`: select、name、値は `int64` または空文字（全デバイス）、Tom Select 適用
  - `from`: input type=date、name、value は "YYYY-MM-DD"、省略可
  - `to`: input type=date、name、value は "YYYY-MM-DD"、省略可
  - `page`: hidden/クエリパラメータ、value は `int64` 1以上、未指定/不正値は 1 に正規化
- バリデーション: 初期表示（パラメータなし）ではバリデーション スキップ（TL14）し全件最新順（`triggered_at DESC`）で表示。from/to が指定された場合のみ範囲チェック（from ≤ to）。

**HTMX 操作仕様（§4 alert-history より）:**
- **検索（フォーム submit）**: `hx-get="/alerts/history?device_id=...&from=...&to=..."` + `hx-target="#alert-history-list"` + `hx-swap="innerHTML"` + `hx-push-url="true"` → `AlertHistoryList.templ` 返却
- **ページネーション（リンク click）**: `<a href="?device_id=...&from=...&to=...&page={N}">` に `hx-boost="true"` 適用 → 同上
- **OOB 更新**: `AlertHistoryList` の同時レスポンスで `AlertHistoryPagination.templ` も `hx-swap-oob="true"` で返却（`#alert-history-pagination`）
- ページ送りは `hx-boost` による HTMX 化（§10 より）

**表示内容・列:**
- 一覧各行: 発火日時（`triggered_at`、YYYY-MM-DD HH:MM）/ デバイス名（`devices.name`、JOIN で取得）/ 指標（`metric.Label()`、温度/湿度）/ ルール条件（`operator.Label() + "%.2f" + metric.Unit()` 例: ">35.00℃"）/ 実測値（`"%.2f" + metric.Unit()` 例: "38.50℃"）/ 通知状態（`is_notified` が true なら "済"、false なら "未"）
- 0件時: `.empty-message` を表示（R12）
- thead 固定（`alert-history-list` は tbody のみ差し替え）

**ID 属性（templ 付与）:**
- `alert-history-list`: tbody の id、HTMX ターゲット
- `alert-history-pagination`: ページネーション領域の id、OOB Swap ターゲット
- 任意: `alert-history-filter`: フォーム全体の id（通常は固定でよい）

**使用 sqlc クエリ:**
- `ListAlertHistoriesPaginated(ctx, device_id *int64, from/to time.Time, limit int32, offset int64)` → 履歴行取得、3テーブル JOIN で `devices.name` 含める
  - ハンドラ実装ポイント: `device_id` クエリが空文字（全デバイス）の場合は strconv.ParseInt を行わず `nil`（*int64 の nil）を渡す＝全デバイス対象（sqlc.narg）。非空のときのみ ParseInt して `*int64` を渡す。
- `CountAlertHistoriesInRange(ctx, device_id *int64, from/to time.Time)` → ページング用の総件数取得
- `ListDevicesByUser(ctx, user_id int64)` → デバイスセレクト選択肢（全デバイス + 各デバイス名）

**新規作成ファイル・テンプレート:**
- `internal/handler/alert_handler.go`: ハンドラ新規作成（`AlertHistoryHandler.List`）
- `internal/view/page/AlertHistory.templ`: フルページ（h1 + フィルター + 一覧 + ページネーション）
- `internal/view/component/AlertHistoryList.templ`: Fragment 返却用（tbody + ページネーション、フォーム・thead は含めない）
- `internal/view/component/AlertHistoryPagination.templ`: ページネーション部品（`hx-swap-oob="true"` を設定した最上位要素）
- `internal/view/component/DeviceSelectOption.templ`: デバイス選択肢部品（再利用性のため S3 ほか複数画面で同一テンプレート使用）

## スコープ外（このセッションでやらないこと）

- アラート履歴の通知状態の「済」マーク操作（`MarkAlertHistoryNotified`）。別セッション（ダッシュボード or アラート関連セッション）で実装される想定。
- アラートルール管理（S7）。本セッションは履歴表示のみ。
- アラート判定ロジック実行（S2）。履歴表示のみ。

## 技術制約・準拠事項

**準拠ドキュメント:**
1. **画面設計書(静的).md § 8（524-580）**: ページレイアウト・フォーム項目・ダミーデータ形式
2. **HTMX実装ガイド(動的).md § 3.7（1319-1328）**: id 属性・HTMX 操作仕様の正確な定義
3. **HTMX実装ガイド(動的).md § 4 alert-history（1565-1584）**: 検索・ページネーション・バリデーション・Tom Select の詳細
4. **HTMX実装ガイド(動的).md § 10（1973-2023）**: ページネーション LIMIT/OFFSET・総ページ数計算・ハンドラパターン
5. **HTMX実装ガイド(動的).md § 16 Tom Select**: デバイス select への Tom Select 適用・初期化方式
6. **DB設計書.md § alert_histories（482-511）**: テーブル定義・非正規化保持・CHECK 制約
7. **DB設計書.md § sqlc リレーション方針（514-527）**: `ListAlertHistoriesPaginated` / `CountAlertHistoriesInRange` の要件
8. **templ実装仕様書（other/）**: templ コンポーネント分割・Fragment 返却方式

**実装言語・フレームワーク:**
- Go 1.26 + Gin v1.12 + templ v0.3 + HTMX + 素のモダンCSS（CSSフレームワーク不使用。トークンは自前 :root 定義 mocks/html/style.css、カスケードは @layer reset, base, components, utilities。詳細は .kiro/steering/tech.md「CSS方針」参照）
- Handler: `*gin.Context` → `c.Query` でパラメータ取得、`c.GetHeader("HX-Request")` で分岐、`.Render()` で templ 返却
- バリデーション: 初期表示ではスキップ（TL14）、from/to が指定されたら `from ≤ to` を検証
- エラーハンドリング: DB エラー → 500、バリデーション失敗 → 422 + Fragment 返却（form 内 `errors` map で templ へ明示的に引数渡し、$errors 共有バッグなし）

**HTMX 実装:**
- フィルターフォーム: `hx-get` / `hx-target="#alert-history-list"` / `hx-swap="innerHTML"` / `hx-push-url="true"` / method="GET" + CSRF 無し（GET）
- ページネーションリンク: `hx-boost="true"`（§R19）
- OOB: メイン応答（AlertHistoryList）+ OOB 応答（AlertHistoryPagination）を `.Render()` で同時に返却

**テスト・品質要件:**
- TDD: テスト駆動で ハンドラ → templ コンポーネント を開発。80% 以上カバレッジ。
- 初期表示（デバイス: 全、期間: 空）で全件最新順・デフォルト1ページ目を表示。
- デバイス指定 → 該当アラート行のみ、期間指定で範囲外を除外。
- ページネーション: 前ページ/現在ページ/次ページリンク・ページ番号の正確な計算。

**日本語コメント・ラベル:** すべて日本語。コード識別子（関数・変数・id 名）は英語。

## 受け入れ基準（概略）

1. **初期表示**: GET /alerts/history（パラメータなし）で全件最新順・1ページ目（20件）が templ フルページとして表示される
2. **フィルター検索（HTMX）**: フォーム submit で `?device_id=2&from=2026-04-13&to=2026-04-20` クエリを生成、HTMX リクエスト時に tbody が AlertHistoryList Fragment で差し替わる
3. **ページネーション（HTMX）**: 「2」「3」等のリンク click で同じ検索条件を保持したまま `?...&page=N` に遷移、tbody + ページネーション領域が同時更新される
4. **デバイス選択肢**: ListDevicesByUser で取得した全デバイスが select option として表示、Tom Select で検索可能
5. **0件表示**: データ 0 件時に `.empty-message`（「指定期間のアラート履歴はありません」等）が表示される
6. **ルール条件・実測値**: metric/operator/threshold/actual_value を `>35.00℃` / `38.50℃` のように正確にフォーマット
7. **通知状態**: is_notified=true で「済」、false で「未」と表示
8. **テスト**: Handler + templ Fragment 返却・バリデーション・ページ計算を TDD で網羅、80% 以上カバレッジ

## 未確定事項・要確認（あれば）

1. **CSRF ミドルウェア**: フィルターフォームは method="GET" のため CSRF トークン不要。確認（S1 で既に整備されている前提）。
2. **scs Session**: S1 で SessionLoad ミドルウェアが整備済みと仮定。未実装の場合は本セッション開始時点で詳細仕様確認が必要。
3. **MethodOverride**: PUT/PATCH/DELETE の hidden `_method` オーバーライド対応は本セッション外だが、S1 で実装済みと仮定。
4. **Tom Select ライフサイクル**: Fragment swap 時に Tom Select インスタンスの破棄→再初期化が必要（§16 参照）。グローバルハンドラで自動対応されているか確認。
5. **テナント分離**: sqlc クエリ `ListAlertHistoriesPaginated` は `d.user_id` フィルタで自動的にテナント分離される。ハンドラで Session から user_id 取得して渡すこと。

--- spec-init 本文 ここまで ---
