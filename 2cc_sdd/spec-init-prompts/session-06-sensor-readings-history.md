# セッション6 spec-init プロンプト: センサーデータ履歴

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: sensor-readings-history
> 実行例: /kiro-spec-init "（本文を貼り付け）"
> 前提セッション: S1（基盤）のみ。S5（device-detail）の「もっと見る」リンク先として使用されるが、コード上の依存はない（S5 と並行実装も可）
> 設計フェーズで参照: 
> - 画面設計書(静的).md § 6（行406-465）/ § ページネーション件数（行606-614）
> - HTMX実装ガイド(動的).md § 3.5 id一覧（行1292-1301）/ § 4 readings操作仕様（行1508-1526）/ § 10 ページネーション（行1973-）/ § 7 GET検索バリデーション（行1707-）
> - DB設計書.md § sensor_readings テーブル定義（行419-447）/ § sqlc集計クエリ例（行565-581）/ § sqlc ページネーション関数（行291-292）
> - mocks/html/readings.html（完成済みモック）

--- spec-init 本文 ここから ---

## 機能概要

農業IoTシステムにおいて、特定デバイスのセンサーデータを期間指定で検索・閲覧し、その統計（平均/最高/最低）を確認する機能を実装する。
現状は設計書存在・HTMLモック完成の状態。本セッションで、期間フィルタ検索と20件/ページのページネーションを HTMX で動的化し、計測日時・温湿度・通信遅延（recorded_at と created_at の差分）を表示する Web UI を完成させる。

## 背景・現状

- **バックエンド完成状態**: DB 6テーブル・sqlc 全37クエリが実装済み。`ListSensorReadingsPaginated`（LIMIT 20 OFFSET）、`GetSensorReadingsSummary`（平均/最高/最低）、`CountSensorReadingsInRange`（総件数）はすべて用意済み。
- **フロントエンド未着手**: `internal/view/` は空。templ コンポーネント・レイアウト、ミドルウェア（SessionLoad / MethodOverride）は S1 で準備予定。本セッションはそれ以降を想定。
- **モック完成**: `mocks/html/readings.html` は静的 HTMLモック完成。素のモダンCSS（`mocks/html/style.css` の自前トークン + `@layer` カスケード）+ クラスセレクタのみで id 属性ゼロ。HTMX 属性はまだなく、templ 変換時に付与する。スタイルは class のみで行い、id はスタイリングに使わない（HTMX 差し替え専用）。
- **位置づけ**: S5（device-detail）の「詳細情報表示」から「もっと見る」リンクで遷移してくる。初期表示は日付フィルタなし・全期間最新順。

## このセッションのスコープ（実装対象）

### ルーティング・ハンドラ

- **GET /devices/{device}/readings** → ハンドラ `ReadingsHandler.Index` → 初期表示時はフルページ`Readings.templ`、HTMX リクエスト時は Fragment `DeviceReadingsList.templ` を返す
  - パラメータ: `device`（デバイスID、URL パス）
  - クエリ: `from`（開始日、YYYY-MM-DD 形式、任意）/ `to`（終了日、YYYY-MM-DD 形式、任意）/ `page`（ページ番号、1 始まり、任意。未指定/page<1 なら 1 に正規化）
  - HTMX: `hx-get` トリガ + `hx-target="#device-readings-list"` + `hx-swap="innerHTML"` + `hx-push-url="true"`（ブラウザ履歴反映）
  - 使用 sqlc クエリ: `ListSensorReadingsPaginated`（LIMIT 20 OFFSET (page-1)*20）、`GetSensorReadingsSummary`（期間 from〜to の集計）、`CountSensorReadingsInRange`（総件数→総ページ数計算）

### フォーム・入力フィールド

**フィルターフォーム（GET メソッド、R17 準拠）**

| 項目 | name | type | バリデーションタグ | 表示上の制約 |
|------|------|------|-------------|----------|
| 開始日 | from | date | `datetime_format=2006-01-02`（任意、形式検証のみ） | 任意 |
| 終了日 | to | date | `datetime_format=2006-01-02`（任意、形式検証のみ） | 任意 |

**バリデーション方針（GET 検索型、HTMX実装ガイド TL14）**
- 日付未指定（初期表示）: バリデーションをスキップ、全期間最新順で応答（HTTP 200）
- 日付指定あり: 形式チェック（YYYY-MM-DD のみ許可）。形式エラー時は同画面に 200 + インラインエラーメッセージ + 空一覧で応答
- クエリパラメータは Handler で `c.Query("from")` / `c.Query("to")` / `c.Query("page")` で受け取り、string → time.Time は `time.Parse("2006-01-02", ...)` で変換

### UI レイアウト・表示要素

**フィルターセクション** 
- `.filter-form`: method=GET、テーブル定義のフォーム項目を配置
- 開始日・終了日の 2 フィールド + 検索ボタン（type=submit）
- フォーム送信は hx-get により HTMX リクエストに自動昇華（R17 準拠）

**集計情報セクション（§ 3.5 id: `readings-summary` の一部）**
- `.summary-grid` / `.summary-box` で 6 項目を表示: 平均温度 / 最高温度 / 最低温度 / 平均湿度 / 最高湿度 / 最低湿度
- 各値は小数第2位 + 単位（℃ / %）で表示
- フィルタ条件に連動するため、一覧と同じ Fragment で同時差し替え

**データ一覧テーブル（§ 3.5 id: `device-readings-list` に含含）**
- `.data-table`: thead 固定（計測日時 / 温度(℃) / 湿度(%) / 通信遅延）
- tbody に 20 件 / ページ分のデータを行で描画
- 通信遅延列: 計算式は `created_at - recorded_at` を秒単位の整数で表示（例：2秒）
- 0 件時: `.empty-message`（R12「指定期間の計測データはありません。」）を表示、テーブルは hidden

**ページネーションセクション（§ 3.5 id: `readings-pagination`）**
- `.pagination`: 自前 templ コンポーネント `Pagination.templ` で描画
- 現在ページ数 / 総ページ数を受け取り、前へ / ページ番号 / 次へ を生成
- 各リンク（a 要素）に `hx-get="/devices/{device}/readings?from=...&to=...&page=N"` + `hx-target="#device-readings-list"` + `hx-swap="innerHTML"` を付与
- ページ番号は 1 始まり。`OFFSET = (page - 1) * 20`

### templ テンプレート・構成

**新規作成ファイル予定**

| ファイルパス | 役割 | 内容要点 |
|---------|------|--------|
| `internal/view/page/Readings.templ` | 初期表示フルページ | `App.templ` レイアウト + `.filter-form` + `DeviceReadingsList.templ` (Fragment) を内包 |
| `internal/view/component/DeviceReadingsList.templ` | HTMX 差し替え Fragment | `.summary-grid` + `.data-table` + `.pagination` を内包。id `device-readings-list` を付与 |
| `internal/view/component/Pagination.templ` | ページネーション部品 | 総ページ数・現在ページから hx-get リンク生成。id `readings-pagination` を付与（任意） |

**id 属性の付与（templ 変換時）**

モック HTML には id ゼロ件。以下を付与する：

| id 名 | 対象要素 | 用途 |
|------|---------|------|
| `device-readings-list` | フィルタ結果領域（`.summary-grid` + `.data-table` + `.pagination` を内包する div）| hx-target アンカーポイント |
| `readings-summary`（任意） | `.summary-grid` / `.summary-box` のラッパー | 集計を個別 OOB 更新する場合（通常は不要） |
| `readings-pagination`（任意） | `.pagination` のラッパー | ページネーションを個別 OOB 更新する場合（通常は不要） |

### ハンドラ実装ポイント

**Handler シグネチャ**
```
func (h *ReadingsHandler) Index(c *gin.Context) {
    deviceID := c.Param("device")
    from := c.Query("from")
    to := c.Query("to")
    page := c.DefaultQuery("page", "1")
    
    // from/to を time.Time に変換（失敗時インラインエラー）
    // page を int に変換（page<1 なら 1）
    // ListSensorReadingsPaginated（LIMIT 20 OFFSET (page-1)*20）を実行
    // GetSensorReadingsSummary（from-to 期間）を実行
    // CountSensorReadingsInRange（from-to 期間）で総ページ数を計算
    
    isHTMX := c.GetHeader("HX-Request") != ""
    if isHTMX {
        c.Header("HX-Push-Url", "true")  // ブラウザ履歴に URL を反映
        c.HTML(200, "DeviceReadingsList.templ", data)  // Fragment 返却
    } else {
        c.HTML(200, "Readings.templ", data)  // フルページ返却
    }
}
```

**エラーハンドリング方針**
- 日付形式エラー: 400 ではなく HTTP 200 + インラインエラー（§ 7 GET 検索バリデーション）
- エラー は `map[string]string` で templ に明示的に引数で渡す（$errors 共有バッグなし）
- デバイス ID 不正・非所有: 404 または 403（S1 で実装のため本セッションスコープ外）

### 使用する sqlc クエリ

| クエリ名 | 用途 | パラメータ |
|---------|------|----------|
| `ListSensorReadingsPaginated` | 指定デバイス・期間のデータ取得 | device_id, from(recorded_at >=), to(recorded_at <=), limit(20), offset((page-1)*20) |
| `GetSensorReadingsSummary` | 期間内の平均/最高/最低を集計 | device_id, from, to |
| `CountSensorReadingsInRange` | 期間内の総件数（ページ数計算用） | device_id, from, to |

## スコープ外（このセッションでやらないこと）

- **デバイス認証・アクセス制御**: S1 で準備予定（SessionLoad / MethodOverride ミドルウェア、ユーザー・デバイス所有者確認）
- **デバイス詳細の最新10件テーブル**: S5 で別実装（本セッション対象の「全期間・ページネーション」とは別）
- **グラフ表示**: 後セッション（サーバサイド SVG 生成は別担当）
- **レイアウト・CSS**: 素のモダンCSS（自前トークン + `@layer reset, base, components, utilities`）によるモックが既存。CSSフレームワークは使わない（CSS方針は `.kiro/steering/tech.md` 参照）。コンポーネント固有CSSは `@layer components` へ追記し、templ の css スコープスタイル式は使わない（生成 `<style>` が unlayered となりカスケードが崩れるため）。templ レイアウト（`internal/view/layout/App.templ`）は S1 で用意
- **CSRF ミドルウェア**: S1 で確認予定（Gin ライブラリ・実装方式）

## 技術制約・準拠事項

### Gin / templ / sqlc / scs

- **Gin**: `c.Query()` / `c.Param()` / `c.GetHeader()` / `c.HTML()` / `c.Redirect()` で制御
- **templ**: コンポーネント関数（`templ.Component` 返却）で再利用。`.Render(c.Request.Context(), c.Writer)` で Fragment 差し替え
- **sqlc**: 生成クエリ（`internal/repository/*.go`）から Querier インターフェース経由で呼び出し。クエリパラメータ・戻り型は `db/queries/*.sql` で定義済み
- **scs Session**: S1 準備予定。本セッションでは SessionLoad ミドルウェアが既に適用されていることを前提

### HTMX

- **フォーム送信**: `.filter-form` の `method="GET"` を `hx-get`（R17）で自動昇華。`hx-target="#device-readings-list"` + `hx-swap="innerHTML"`
- **ページネーション**: 各ページリンク（a 要素）に `hx-get` + `hx-target="#device-readings-list"` + `hx-swap="innerHTML"`。`hx-boost` 使用（R19）
- **HX-Request 判定**: `c.GetHeader("HX-Request") != ""` で分岐。HTMX なら Fragment 返却
- **HX-Push-Url**: ブラウザ履歴に URL を反映するため `c.Header("HX-Push-Url", "true")` を応答ヘッダに付与

### 日本語・コード規約

- **応答・コメント・ラベル・エラーメッセージ**: 日本語
- **構造体フィールド・パラメータ名・クエリ関数名**: 英語（sqlc 生成が基準）
- **通信遅延表示**: 整数+秒（例：「2秒」）

### テスト・カバレッジ

- **テスト対象**: バリデーション（日付形式）、Handler（初期表示 / フィルタ実行 / ページネーション）、ページ数計算ロジック
- **カバレッジ**: 80% 以上（S1 基盤と合算で達成予定）
- **テストファイル**: `internal/handler/readings_test.go`（新規作成予定）

### エラーハンドリング

- **日付形式エラー**: `errors.New()` で定義し templ に `map[string]string` 経由で引数渡し
- **DB エラー**: 500 エラーを表示（詳細はログのみ）
- **0 件時**: テーブル hidden + `.empty-message` 表示（R12）

### イミュータブル方針

- Handler 引数・戻り値は値受取（ポインタ受取は避ける）
- 集計データ構造 `SensorReadingsSummary` は値型で設計

## 受け入れ基準（概略）

1. **初期表示（日付未指定）**: GET `/devices/{device}/readings` でフルページ返却、全期間最新順に最新20件のセンサーデータを表示。集計情報（平均/最高/最低）も表示
2. **期間フィルタ検索**: フィルターフォームで開始日・終了日を指定して検索ボタンを押下。HTMX リクエストで部分更新（Fragment 返却）。URL はブックマーク可能に（hx-push-url）
3. **ページネーション**: ページリンク click でページ送り。HTMX で差し替え。URL にクエリパラメータ反映
4. **集計情報連動**: フィルタ条件に応じて集計結果も同時更新。一覧と同じ Fragment 返却で実現
5. **通信遅延表示**: テーブル最右列に整数秒で表示。計算式 `(created_at - recorded_at).Seconds()` → 四捨五入
6. **0 件表示**: 指定期間で該当データなし → テーブル hidden + 「指定期間の計測データはありません。」を表示
7. **ページネーション自動計算**: sqlc `CountSensorReadingsInRange` で総件数取得 → `totalPages := (totalCount + 19) / 20` で計算
8. **エラー表示（形式エラー時）**: 同画面に HTTP 200 + インラインエラーメッセージを表示。一覧は空

## 未確定事項・要確認（あれば）

- **ページネーション UI の詳細**: 「前へ / ページ番号 / 次へ」の実装形式（モック行1292-1301 の HTML がダミー）。最大表示ページ数（3 ページ区間？ ...表示？）は要確認
- **日付形式のブラウザネイティブバリデーション**: `<input type="date">` のデフォルト動作でブラウザが形式検証するが、Server 側のバリデーション方式（`time.Parse` のエラー処理）確定の後に実装
- **CSRF ミドルウェア具体方式**: S1 で確認予定。meta タグ + htmx:configRequest 方式か、他か
- **MethodOverride ミドルウェア実装タイミング**: 本セッション対象は GET のみだが、他 CRUD セッションと同時期の準備を要確認

--- spec-init 本文 ここまで ---
