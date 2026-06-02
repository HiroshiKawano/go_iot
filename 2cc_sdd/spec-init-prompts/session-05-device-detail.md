# セッション5 spec-init プロンプト: デバイス詳細（情報・SVGグラフ・期間切替・削除・最新計測）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: device-detail
> 命名注記: feature-name は業務名 device-detail。ルート/templ は REST 観点で device-show を用いる（GET /devices/{device}、internal/view/page/DeviceShow.templ）。両者は同一画面を指す。
> 実行例: /kiro-spec-init "（本文を貼り付け）"
> 前提セッション: S1（基盤）、S4（登録・編集。編集リンク先 /devices/{device}/edit を使用）
> 設計フェーズで参照: 画面設計書(静的).md 行267-346・606-614、HTMX実装ガイド(動的).md §3.3（1266-1278）・§4 device-show（1445-1469）・§10-D フルフラグメント swap（2132-2171）・§11 削除確認（2351-2371）・§24 Alpine.jsモーダル（3603-3720）、DB設計書.md のdevices/sensor_readings、システム構成図.md §バックエンド サーバサイドSVGグラフ自作、mocks/html/device-show.html

--- spec-init 本文 ここから ---

## 機能概要

農場運営者はデバイスから送信される温湿度計測データを時系列で可視化し、デバイスの稼働状態を監視する必要がある。現状、DB層の全クエリとセンサーAPI受信は完成しているが、ブラウザ向けのデバイス詳細表示画面が全面未着手である。本セッションでは、デバイス情報パネル、期間切替によるグラフ差し替え（HTMX中核）、最新計測データテーブル、削除機能を実装し、グラフ領域はサーバーサイドで SVG を自作生成する templ コンポーネントとして完成させる。

## 背景・現状

バックエンド（DB・sqlc全34クエリ・デバイスBearer認証・POST /api/sensor-data・CLI）はほぼ完成している。設計が想定する Web UI 層（scs Session未実装・templ画面未着手・HTMX属性0件・アラート判定接続）は全面未着手。internal/service, internal/middleware, internal/view は空。本セッションはS1基盤（セッション認証・ルーティング・エラーハンドリング）とS4デバイス登録編集の後に実行し、/devices/{device} エンドポイントを通じてtempl + HTMX + Alpine.js の統合を初めて具体化する。

## このセッションのスコープ（実装対象）

### ルーティングとハンドラ

- **GET /devices/{device}** → `DeviceHandler.Show` → `internal/view/page/DeviceShow.templ`（フルページ初期表示。デフォルト期間24h）
- **GET /devices/{device}/chart?period={24h|7d|30d}** → `DeviceHandler.Chart` → `internal/view/component/DeviceChartArea.templ`（HTMX フラグメント。温度グラフ + 湿度グラフ + 期間セレクタを一括差し替え）
- **DELETE /devices/{device}** → `DeviceHandler.Delete` → 論理削除（`SoftDeleteDevice` sqlc クエリ）→ HX-Redirect /dashboard またはフルページリダイレクト

### デバイス情報パネル

- デバイス名・MAC・場所・稼働状態（ステータス記号●/○）・最終通信（YYYY-MM-DD HH:MM:SS形式）を表示
- [編集]リンク（`/devices/{device}/edit`、S4が提供）と[削除]ボタンを配置
- sqlcクエリ: `GetDevice`（デバイス基本情報・最終通信日時取得）

### 期間切替HTMX機構（本セッション技術的中心）

- 期間ボタン: 24時間 / 7日間 / 30日間（R22-2 ネイティブselect相当。Tom Select不要。3値のみ）
- 各ボタンに `hx-get="/devices/{device}/chart?period=24h"` + `hx-target="#device-chart-area"` + `hx-swap="innerHTML"` を付与
- レスポンスは`DeviceChartArea.templ`（温度グラフ`#temperature-chart` + 湿度グラフ`#humidity-chart` + 期間ボタン群を内包）
- アクティブ期間ボタンの active状態（`.active` 等）をサーバー側で判定し、レスポンスに含める（フルフラグメント swap で選択状態を往復、§10-D）
- Handler内でクエリパラメータ `period` をバリデーション（required, oneof=24h 7d 30d）

### SVGグラフ生成（サーバーサイド）

- 24時間グラフ: `sensor_readings` 生データ（温度・湿度）を取得し、横軸時間・縦軸℃/% の SVG を生成（システム構成図.md参照）
- 7日/30日グラフ: `ListDailySensorAggregates` sqlcクエリで日次集計（GROUP BY DATE(recorded_at)、MAX/MIN集計）を取得
- 集計結果の MAX/MIN は nullable な pgtype.Numeric のため pgconv.NumericToFloat() で安全に変換（実装現状サマリ §5.2 の関数名）
- SVG 出力は templ の `@templ.Raw(svgString)` で埋め込み（HTML エスケープ回避）
- SVG 自作生成の技術仕様（線色・フォント・軸ラベル等）は other/templ実装仕様書.md を参照。詳細レイアウトはこのセッション内で実装確定

### 最新計測データテーブル

- 固定10件（ページネーションなし）。グラフの期間切替に連動しない（常に最新10件）
- id: `latest-readings-table`（tbody。期間切替時は差し替え対象に含めない）
- 列: 計測日時（YYYY-MM-DD HH:MM形式）/ 温度（小数2桁℃）/ 湿度（小数2桁%）
- sqlcクエリ: `ListRecentSensorReadings`（device_id で LIMIT 10 、 ORDER BY recorded_at DESC）

### 削除機能（Alpine.js モーダル）

- [削除]ボタンクリック → Alpine.js x-show で確認モーダル（id: `delete-device-modal`）を表示
- モーダル内容: デバイス名・注意書き「※この操作は取り消せません。計測データも削除されます。」
- 確認ボタン: `hx-delete="/devices/{device}"` + hx-target 省略（本体へリダイレクト）
- 削除成功後: `c.Header("HX-Redirect", "/dashboard")` を返す（一覧へ戻す）
- 非HTMX（フルページリダイレクト）の場合: `c.Redirect(http.StatusSeeOther, "/dashboard")`
- sqlcクエリ: `SoftDeleteDevice`（論理削除、deleted_at = NOW()）
- §24 推奨パターンを採用（fetch + Alpine.js + htmx.trigger も検討可）

### 新規作成templ / ミドルウェア / ファイル

- `internal/view/page/DeviceShow.templ` — フルページ（共通レイアウト App.templ を使用）
- `internal/view/component/DeviceChartArea.templ` — グラフ領域フラグメント（期間ボタン + 温度グラフ + 湿度グラフ）
- `internal/view/component/DeviceInfoPanel.templ` — デバイス情報パネル（名前・MAC・場所・状態・最終通信・[編集][削除]ボタン）
- `internal/view/component/LatestReadingsTable.templ` — 最新計測テーブル（10件固定）
- `internal/handler/device_handler.go` — Show / Chart / Delete メソッド追加（S4 Create/Edit は別途実装）
- `internal/service/device_service.go` — グラフSVG生成ロジック（サーバーサイド）。温度・湿度ごとの線描画・スケーリング関数を集約

### バリデーションとエラーハンドリング

- `/devices/{device}` の device パラメータは ParseInt で int64 に変換。不正な場合 400 Bad Request
- `/devices/{device}/chart?period=...` の period クエリ: Handler 内で `c.ShouldBindQuery` + `binding:"required,oneof=24h 7d 30d"`
- グラフデータが 0 件の場合、空のグラフ SVG（「データはまだありません」のテキスト付き）を返す
- HTMX リクエスト判定: `c.GetHeader("HX-Request") != ""` で分岐（フルページ vs フラグメント）

### CSRF / HX-Redirect 対応

- DELETE リクエストに CSRF 保護を適用（共通レイアウト App.templ の meta + htmx:configRequest で X-CSRF-Token ヘッダを自動付与）
- HX-Redirect: `c.Header("HX-Redirect", "/dashboard")` を DELETE レスポンスで返す（HTMX リクエスト時）

## スコープ外（このセッションでやらないこと）

- デバイス登録/編集フォーム（S4）
- センサーデータ履歴全件取得・ページネーション（S6 /devices/{device}/readings）
- アラート判定エンジン・履歴記録（別セッション）
- セッション認証ミドルウェア（S1 で実装済み想定）
- 自動更新ポーリング（ダッシュボード dashboard は「基本はフルページ表示、自動更新は MVP では実装しない」方針）
- Tom Select 等のセレクタ（期間選択はネイティブボタン＆HTMX）

## 技術制約・準拠事項

- **Gin** ルーティング・バリデーション・ハンドラ署名（`c *gin.Context` のコンテキスト処理）
- **templ** v0.3（`.templ` 関数構文、`@templ.Raw()` で SVG 埋め込み、レイアウト継承）
- **sqlc** v1.30（`GetDevice` / `ListRecentSensorReadings` / `ListDailySensorAggregates` / `SoftDeleteDevice` クエリ呼び出し）
- **scs** セッション（S1で実装済み。認証後のユーザーID を `c.Request.Context()` から取得）
- **HTMX** 属性: hx-get / hx-target / hx-swap（innerHTML）を period ボタンに付与。HX-Request ヘッダ判定
- **Alpine.js** x-data / x-show / @click でモーダル開閉（v3.x CDN）
- **CSS** 素のモダンCSS（CSSフレームワーク不使用）。自前トークン（mocks/html/style.css の :root: --space-*, --fs-*, --radius, --shadow-* 等）と @layer reset, base, components, utilities; を使用。グラフ領域・情報パネル・テーブル・モーダルの class（grid / flex / form-help / btn / status-active 等）は mocks/html/device-show.html / style.css に準拠。コンポーネント固有CSSは @layer components へ追記し、templ の css スコープスタイル式は使わない（生成 style が unlayered になりカスケードが崩れるため）。id はHTMX差し替え専用とし、スタイリングには使わない（CSS方針の正は .kiro/steering/tech.md「CSS方針」を参照）
- 日本語コメント・エラーメッセージ。コード識別子は英語
- TDD: 80% 以上のカバレッジ。Handler テスト（Get / Post / Delete）、Service テスト（グラフSVG生成の入出力検証）
- イミュータブル方針: sqlc 生成構造体は読取専用。Service で値を組み立てて Handler に渡す
- HTMX実装ガイド(動的).md §3.3（id一覧）・§4 device-show（1445-1469）・§10-D（フルフラグメント）・§11（削除確認）・§24（Alpine.jsモーダル）を準拠
- DB設計書.md devices テーブル（deleted_at 論理削除）・sensor_readings テーブル（GROUP BY 集計）・pgtype.Numeric 型安全処理

## 受け入れ基準（概略）

1. **フルページ初期表示（GET /devices/{device}?）** が正常に動作。デバイス名・MAC・最終通信・デフォルト24hグラフ・最新計測10件が表示される
2. **期間切替HTMX** が正常に動作。24h / 7d / 30d ボタンのいずれかをクリックすると、グラフが差し替わり、アクティブボタンの状態がサーバーから往復される
3. **SVG グラフ生成** が正常に動作。24h グラフは温度・湿度のデータ点を線でつないだ線グラフ。7d / 30d は日次集計値（MAX/MIN）を表示。軸ラベル・凡例を含む
4. **最新計測テーブル** が固定10件で表示される。期間ボタン操作でグラフは変わるが、テーブルは「常に最新10件」で更新されない
5. **削除機能** が正常に動作。[削除]ボタン → モーダル表示 → 確認 → DELETE リクエスト発火 → 削除成功後 /dashboard へリダイレクト
6. **バリデーション・エラーハンドリング** が正常。不正な device ID / period パラメータに対して 400 / 422 を返す
7. **テストカバレッジ 80% 以上** のハンドラテスト（Show / Chart / Delete）、Service テスト（グラフSVG生成）を完備

## 未確定事項・要確認（あれば）

- **SVG グラフ生成ライブラリ**: 標準 Go 実装（strings.Builder で XML 直描画）か golang.org/x/image 等の軽量ライブラリを使うか？ — 本プロジェクトは軽量・自作方針を選択（他/templ実装仕様書.md を参照）
- **CSRF ライブラリ**: Gin 公式ミドルウェア（github.com/gin-contrib/sessions）か別ライブラリか？ — S1で確定済み想定
- **Method Override ミドルウェア**: HTML フォーム DELETE 対応の _method ハンドリング方式 — S1で確定済み想定（または fetch 使用）
- **グラフ色・フォント仕様**: SVG内の線色（温度・湿度の区別）、フォントサイズ、軸ラベルフォーマット — other/templ実装仕様書.md で確認。なければこのセッション設計フェーズで確定

--- spec-init 本文 ここまで ---