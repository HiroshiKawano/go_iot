# HTMX実装ガイド(動的)

---

## cc-sdd参照ガイド

本設計書をcc-sdd（詳細設計書）から参照する際に価値の高いセクションと用途を示す。

| 優先度 | セクション | cc-sddでの用途 |
|:------:|-----------|---------------|
| ★★★ | [templ コンポーネントアプローチ](#templ-コンポーネントアプローチ) | Handler の templ 関数呼び出しパターンの実装根拠。部分ビューファイル分離方式との違い。誤ったパターンを選ぶと templ ファイル数・Handler 構造・テストがすべて変わる |
| ★★★ | [コンポーネント命名規約](#コンポーネント-命名規約) | コンポーネント関数名・`hx-target` id を統一するルール。名前不一致による実装ミスを防ぐ唯一の根拠 |
| ★★★ | [id属性一覧](#2-id-属性一覧) | Handler（コンポーネント呼び出し）・templ（関数定義）・HTMX 属性（hx-target指定）の唯一の定義元。HTMLモックにはidが含まれないためこの一覧が必須 |
| ★★★ | [画面別HTMX操作仕様](#3-画面別-htmx-操作仕様) | 各画面のトリガー・URL・ターゲットのプロジェクト固有仕様。ルーティング・Handler 実装設計の根拠 |
| ★★★ | [バリデーションエラー表示方針](#6-バリデーションエラー表示方針) | HTMXフォームは 422+コンポーネント返却、通常フォームはリダイレクト+flash の使い分け根拠。422のswap設定方式 |
| ★★★ | [CSRF対応方針](#7-csrf対応方針) | グローバル meta tag + `htmx:configRequest` 方式（フォームごとの hidden トークン不使用）の実装根拠。未設定だと全ミューテーションリクエストが 403 になる |
| ★★ | [OOB同時更新エンドポイント一覧](#4-oob-同時更新エンドポイント一覧) | 複数コンポーネントを連続 `.Render()` するエンドポイントの定義。単一コンポーネント設計にすると更新されない要素が生じる |
| ★★ | [期間パラメータと集計ルール](#期間パラメータperiodと集計ルール) | 24h=生データ・7d/30d=日次集計の違い。DBクエリ・Handler 実装・テスト内容に直接影響する |
| ★★ | [HX-Redirect使用方針](#8-hx-redirect-使用方針) | デバイス削除後のページ遷移実装根拠。`c.Redirect()` と `HX-Redirect` の使い分け。HTMX リクエストに通常の `Redirect()` は効かない |
| ★ | [自動更新（ポーリング）](#5-自動更新ポーリング) | 60秒ポーリング間隔の根拠。非機能要件・パフォーマンステスト設計に影響 |
| ★ | [ページネーションのHTMX統合方針](#9-ページネーションの-htmx-統合方針) | `hx-boost` + `hx-target` によるページネーション部分更新の実装パターン |
| ★ | [削除確認方針](#10-削除確認方針) | `hx-confirm` 使用方針の統一根拠。Alpine.js モーダル不使用の根拠 |

> ※ cc-sddのHandler 実装・templ 設計・ルーティングを記述する際は、まず本書の id 属性一覧・画面別操作仕様・バリデーションエラー方針・CSRF 対応を確認すること。

### 次回プロジェクトでの記載チェックリスト

HTMX実装ガイド(動的)を新規作成する際に以下が揃っているか確認する：

- [ ] 部分ビューファイルを分離するか templ コンポーネント関数を使うかを明記（Handler・templ ファイル構成・テスト構造が変わる）
- [ ] コンポーネント関数名と hx-target id の対応ルール（命名規約）を記載
- [ ] 全画面の id 属性一覧（Handler・templ・HTMX 属性の「共通語彙」）を記載。HTML モックには id が含まれないため必須
- [ ] 画面ごとの HTMX 操作仕様（メソッド・URL・ターゲット・トリガー）を網羅
- [ ] 1リクエストで複数要素を更新する OOB エンドポイントを明記（連続 `.Render()` の使用箇所）
- [ ] 期間パラメータ等、データ取得方式が変わるクエリパラメータとその集計ルールを記載
- [ ] HTMX 未使用の画面・操作を明記（誤って HTMX を適用させないため）
- [ ] 自動更新（ポーリング）の間隔と対象エンドポイントを記載
- [ ] HTMX フォームのバリデーションエラー返却方式（422+コンポーネント vs リダイレクト）を明記
- [ ] 422 レスポンスの swap 設定方式（`responseHandling` / `htmx:responseError`）を明記
- [ ] CSRF 対応方式（グローバル meta tag 方式 vs フォームごと hidden トークン方式）を明記
- [ ] `HX-Redirect` を使う操作とリダイレクト先を記載
- [ ] 削除操作の確認方法（`hx-confirm` / Alpine.js モーダル等）を明記

---

## 1. HTMX 実装規約

> **cc-sdd への価値:**
> templ には「部分ビューファイルを分離する」パターンと「ページ用ファイル内にコンポーネント関数を定義する」パターンの2種類がある。本プロジェクトは後者を採用しているが、どちらを選ぶかは設計上の判断であり、他のドキュメントに記述がない。誤ったパターンで設計すると、Handler の実装方式・templ ファイル数・テストの構造がすべて変わるため、spec-design の段階で必ず参照する必要がある。

### templ コンポーネントアプローチ

部分テンプレートファイルを分離せず、ページ用 `.templ` ファイル内に細分化した `templ` 関数として部分更新領域を定義する。Handler は該当コンポーネント関数を `.Render(ctx, w)` で直接呼び出す。

**templ テンプレート側 (`internal/view/page/Dashboard.templ`):**

```templ
package page

// ページ全体
templ Dashboard(devices []repository.Device, alerts []AlertView) {
    @layout.App() {
        <div id="alert-banner">
            @AlertBanner(alerts)
        </div>
        <div id="device-cards">
            @DeviceCards(devices)
        </div>
    }
}

// 部分更新ターゲット（innerHTML swap）
templ DeviceCards(devices []repository.Device) {
    for _, d := range devices {
        <article class="device-card">
            <h3>{ d.Name }</h3>
        </article>
    }
}
```

**Handler 側 (`internal/handler/dashboard.go`):**

```go
func (h *Dashboard) Devices(c *gin.Context) {
    ctx := c.Request.Context()
    devices, err := h.Repo.ListDevicesByUser(ctx, userID)
    if err != nil {
        c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
        return
    }
    // HTMX リクエストの場合、コンポーネント単体のみを返す
    if err := page.DeviceCards(devices).Render(ctx, c.Writer); err != nil {
        c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
    }
}

func (h *Dashboard) Index(c *gin.Context) {
    ctx := c.Request.Context()
    // ... データ取得
    if err := page.Dashboard(devices, alerts).Render(ctx, c.Writer); err != nil {
        c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
    }
}
```

**複数コンポーネントの同時返却（OOB Swap 用）:**

```go
func (h *Readings) Index(c *gin.Context) {
    ctx := c.Request.Context()
    // ...
    if c.GetHeader("HX-Request") != "" {
        w := c.Writer
        if err := page.ReadingsTable(readings, pagination).Render(ctx, w); err != nil {
            c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
            return
        }
        if err := page.ReadingsSummaryOOB(summary).Render(ctx, w); err != nil {
            c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
        }
        return
    }
    if err := page.Readings(device, readings, summary, pagination).Render(ctx, c.Writer); err != nil {
        c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
    }
}
```

---

### コンポーネント 命名規約

> **cc-sdd への価値:**
> コンポーネント関数名はターゲット id を PascalCase にしたものにするプロジェクト規約がある。規約がないと cc-sdd が Handler・templ・HTMX 属性でそれぞれ異なる名前を生成し、実装時に不整合が発生する。

**規則:** コンポーネント関数名はターゲット id を PascalCase に変換したものとする。OOB 用は `OOB` サフィックスを付ける。

| 例 | HTMX 属性 | templ コンポーネント関数 |
|---|----------|------------|
| デバイスカード | `hx-target="#device-cards"` | `templ DeviceCards(...)` |
| アラートバナー | `hx-target="#alert-banner"` | `templ AlertBanner(...)` |
| 集計 OOB | `id="readings-summary" hx-swap-oob="true"` | `templ ReadingsSummaryOOB(...)` |

---

### コンポーネントの 2 つの配置パターン

> **cc-sdd への価値:**
> メインコンポーネントと OOB コンポーネントでは templ 内での配置ルールが逆になる。間違えると部分更新時の返却範囲が変わるか、フルページ表示時に OOB 属性が誤動作する。どちらのパターンを使うかは id 属性一覧と OOB エンドポイント一覧（セクション 4）で定義している。

**パターン 1: メインコンポーネント（innerHTML swap 用）**

ターゲット要素の **内側** に展開される中身のみを返す。

```templ
// ページ側: id を持つラッパー要素の中でコンポーネントを呼ぶ
<div id="device-cards">
    @DeviceCards(devices)
</div>

// コンポーネント本体: 中身だけを書く (id や wrapper は含めない)
templ DeviceCards(devices []repository.Device) {
    for _, d := range devices {
        <article class="device-card">{ d.Name }</article>
    }
}
```

**パターン 2: OOB コンポーネント（outerHTML swap 用）**

`id` と `hx-swap-oob="true"` を持つ要素 **全体** を返す。

```templ
templ ReadingsSummaryOOB(s Summary) {
    <div id="readings-summary" hx-swap-oob="true">
        <p>平均温度: { fmt.Sprintf("%.2f", s.AvgTemp) }℃</p>
    </div>
}
```

> ※ `hx-swap-oob` 属性はフルページ表示時にはブラウザに無視されるため、常に付与して問題ない。

---

### hx-swap デフォルト動作

- HTMX の既定の swap は `innerHTML`。本システムもこれを標準とする
- `hx-swap` 属性を明示する必要があるのは `outerHTML` を使う場合のみ
- OOB Swap（`hx-swap-oob="true"`）は常に `outerHTML` で動作する（id が一致する要素全体を差し替え）

---

## 2. id 属性一覧

> **cc-sdd への価値:**
> id 属性は Handler（コンポーネント呼び出し）・templ（コンポーネント関数定義）・HTMX 属性（hx-target 指定）の3箇所で同じ名前を参照する「共通語彙」になる。この一覧なしに設計すると、画面ごとに命名が揺れて実装時の統一が崩れる。なお、HTML モックには id 属性が含まれていないため、この一覧が唯一の定義元となる。

| 画面 | id | 対象要素 |
|------|-----|---------|
| ダッシュボード | `alert-banner` | 未対応アラート通知エリア |
| ダッシュボード | `device-cards` | デバイスカード一覧エリア |
| デバイス詳細 | `period-selector` | 期間選択ボタン群 |
| デバイス詳細 | `temperature-chart` | 温度グラフ表示エリア |
| デバイス詳細 | `humidity-chart` | 湿度グラフ表示エリア |
| デバイス詳細 | `latest-readings` | 最新計測データテーブル |
| センサーデータ履歴 | `readings-filter` | 期間フィルターフォーム |
| センサーデータ履歴 | `readings-summary` | 集計情報エリア |
| センサーデータ履歴 | `readings-table` | データ一覧テーブル |
| アラートルール管理 | `device-selector` | デバイス選択エリア |
| アラートルール管理 | `rule-form` | ルール追加 / 編集フォーム |
| アラートルール管理 | `rules-list` | ルール一覧テーブル |
| アラート履歴 | `history-filter` | フィルターフォーム |
| アラート履歴 | `history-table` | 履歴一覧テーブル |

---

## 3. 画面別 HTMX 操作仕様

> **cc-sdd への価値:**
> 「何をトリガーに」「どの URL へリクエストし」「どの id 要素を更新するか」の組み合わせはプロジェクト固有の設計決定であり、他のドキュメントから導出できない。特に自動更新のトリガー種別（`every 60s`）・期間パラメータの集計方式の切り替え・フォーム再利用パターンは誤りやすいため注意が必要。

### ダッシュボード

| 操作 | メソッド | URL | ターゲット | トリガー |
|------|---------|-----|----------|---------|
| デバイスカード自動更新 | GET | /dashboard/devices | #device-cards | every 60s |
| アラートバナー自動更新 | GET | /dashboard/alerts | #alert-banner | every 60s |

---

### デバイス詳細

| 操作 | メソッド | URL | ターゲット | トリガー |
|------|---------|-----|----------|---------|
| 期間切替（温度グラフ） | GET | /devices/{id}/chart/temperature?period={値} | #temperature-chart | click |
| 期間切替（湿度グラフ） | GET | /devices/{id}/chart/humidity?period={値} | #humidity-chart | click |
| 最新データ自動更新 | GET | /devices/{id}/latest-readings | #latest-readings | every 60s |
| デバイス削除 | DELETE | /devices/{id} | - | click（確認ダイアログ後）→ /dashboard にリダイレクト |

**期間パラメータ（period）と集計ルール:**

> **cc-sdd への価値:**
> 24時間と7日・30日でデータ取得方式が異なる（生データ vs 日次集計）。この違いは DB クエリ・Handler 実装・テスト内容に直接影響するが、DB設計書.md には記載がない。

| 表示 | 値 | データ範囲 | 使用クエリ |
|------|-----|----------|--------|
| 24時間 | 24h | 直近24時間の**生データ** | `ListRecentSensorReadings` |
| 7日間 | 7d | 直近7日間の**日次集計** | `ListDailySensorAggregates` |
| 30日間 | 30d | 直近30日間の**日次集計** | `ListDailySensorAggregates` |

---

### デバイス登録 / 編集

> **cc-sdd への価値:**
> このプロジェクトで HTMX を使わない唯一の CRUD 操作。記述がないと cc-sdd はすべての操作に HTMX を適用しようとする。

**この画面は HTMX ではなく通常のフォーム送信（フルページ遷移）を使用する。**

| 操作 | メソッド | URL | 成功時の遷移先 |
|------|---------|-----|-------------|
| 新規登録 | POST | /devices | /devices/{新規ID}（デバイス詳細） |
| 更新 | PUT | /devices/{id} | /devices/{id}（デバイス詳細） |
| キャンセル | - | - | 前の画面に戻る（ブラウザバック） |

> HTTP の PUT/DELETE は HTML フォームが直接サポートしないため、`<input type="hidden" name="_method" value="put">` と method override ミドルウェアを併用する。

---

### センサーデータ履歴

| 操作 | メソッド | URL | ターゲット | トリガー |
|------|---------|-----|----------|---------|
| 期間フィルター検索 | GET | /devices/{id}/readings?from=...&to=... | #readings-table, #readings-summary (OOB) | submit |
| ページネーション | GET | /devices/{id}/readings?page=2 | #readings-table | click |

---

### アラートルール管理

> **cc-sdd への価値:**
> 追加フォームと編集フォームを同一要素（`#rule-form`）で兼用し、OOB Swap でリセットするパターンは特殊な設計決定。別ページや専用モーダルを作るアプローチとも実装量が大きく変わるため、明示的な記述が必要。

| 操作 | メソッド | URL | ターゲット | トリガー |
|------|---------|-----|----------|---------|
| デバイス切替 | GET | /alerts/rules?device_id={id} | #rules-list, #rule-form (OOB) | change |
| ルール追加 | POST | /alerts/rules | #rules-list, #rule-form (OOB) | submit |
| ルール編集フォーム取得 | GET | /alerts/rules/{ruleId}/edit | #rule-form | click |
| ルール更新 | PUT | /alerts/rules/{ruleId} | #rules-list, #rule-form (OOB) | submit |
| 有効/無効切替 | PATCH | /alerts/rules/{ruleId}/toggle | #rules-list | change |
| ルール削除 | DELETE | /alerts/rules/{ruleId} | #rules-list | click（確認後） |

**フォーム再利用パターン:**
編集時は追加フォームを編集フォームとして再利用する。GET で既存値をフォームにロードし、PUT で更新。追加・更新完了後はいずれも空の追加フォームに OOB Swap で戻す。

---

### アラート履歴

| 操作 | メソッド | URL | ターゲット | トリガー |
|------|---------|-----|----------|---------|
| フィルター検索 | GET | /alerts/history?device_id=...&from=...&to=... | #history-table | submit |
| ページネーション | GET | /alerts/history?page=2 | #history-table | click |

---

## 4. OOB 同時更新エンドポイント一覧

> **cc-sdd への価値:**
> 1リクエストで複数要素を同時更新するエンドポイントでは、Handler 内で複数の templ コンポーネントを連続 `.Render()` する必要がある。どのエンドポイントがこのパターンを使うかは設計上の判断であり、他のドキュメントから推測できない。誤ってシングルコンポーネントで設計すると、更新されない要素が生じてテストが通らなくなる。

| エンドポイント | メインコンポーネント | OOB コンポーネント |
|-------------|--------------|-------------|
| `/devices/{id}/readings`（フィルター検索） | `ReadingsTable` → #readings-table | `ReadingsSummaryOOB` → #readings-summary |
| `/alerts/rules?device_id={id}`（デバイス切替） | `RulesList` → #rules-list | `RuleFormOOB` → #rule-form |
| `/alerts/rules` POST（ルール追加） | `RulesList` → #rules-list | `RuleFormOOB` → #rule-form |
| `/alerts/rules/{id}` PUT（ルール更新） | `RulesList` → #rules-list | `RuleFormOOB` → #rule-form |

**templ テンプレート構造の具体例 (`internal/view/page/Readings.templ`):**

```templ
// メインコンポーネント: #readings-table の innerHTML として差し込まれる
templ ReadingsTable(readings []repository.SensorReading, pagination Pagination) {
    <table>
        <thead>
            <tr><th>日時</th><th>温度</th><th>湿度</th></tr>
        </thead>
        <tbody>
            for _, r := range readings {
                <tr>
                    <td>{ r.RecordedAt.Format("2006-01-02 15:04") }</td>
                    <td>{ fmt.Sprintf("%.2f", r.Temperature) }</td>
                    <td>{ fmt.Sprintf("%.2f", r.Humidity) }</td>
                </tr>
            }
        </tbody>
    </table>
    @PaginationView(pagination)
}

// OOB コンポーネント: #readings-summary を要素ごと差し替え
templ ReadingsSummaryOOB(s Summary) {
    <div id="readings-summary" hx-swap-oob="true">
        <p>平均温度: { fmt.Sprintf("%.2f", s.AvgTemp) }℃</p>
        <p>最高温度: { fmt.Sprintf("%.2f", s.MaxTemp) }℃</p>
    </div>
}
```

**Handler (`internal/handler/reading.go` の `Index`):**

```go
func (h *Reading) Index(c *gin.Context) {
    ctx := c.Request.Context()
    // ... データ取得
    if c.GetHeader("HX-Request") != "" {
        w := c.Writer
        if err := page.ReadingsTable(readings, pagination).Render(ctx, w); err != nil {
            c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
            return
        }
        if err := page.ReadingsSummaryOOB(summary).Render(ctx, w); err != nil {
            c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
        }
        return
    }
    if err := page.Readings(device, readings, summary, pagination).Render(ctx, c.Writer); err != nil {
        c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
    }
}
```

---

## 5. 自動更新（ポーリング）

> **cc-sdd への価値:**
> ポーリング間隔（60秒）はビジネス要件から決まった値であり、他のドキュメントに記載がない。非機能要件の記述やパフォーマンステストの設計にも影響する。

| 画面 | エンドポイント | 間隔 | 実装 |
|------|-------------|------|------|
| ダッシュボード | `/dashboard/devices` | 60秒 | `hx-trigger="every 60s"` |
| ダッシュボード | `/dashboard/alerts` | 60秒 | `hx-trigger="every 60s"` |
| デバイス詳細 | `/devices/{id}/latest-readings` | 60秒 | `hx-trigger="every 60s"` |

---

## 6. バリデーションエラー表示方針

> **cc-sdd への価値:**
> HTMX フォームでバリデーションエラー（422）が発生した場合の返却形式はプロジェクト固有の設計決定。他のドキュメントに記載がなく、誤ると全フォームのエラー表示が機能しない。

### フォーム種別による方針の違い

| フォーム種別 | エラー返却方式 |
|------------|-------------|
| **HTMX フォーム**（アラートルール追加・更新） | フォームコンポーネントを **422 ステータス** で返却 |
| **通常フォーム**（デバイス登録・編集） | リダイレクト + flash メッセージ（scs のセッション flash を利用） |

### HTMX フォームのエラー返却パターン

**採用方針:** Handler 内で Gin の `ShouldBind` 系 API に `binding` タグを付与した構造体を渡し、組込の `go-playground/validator` に検証させる。エラー時は 422 でコンポーネントを返す。

> Gin は `ShouldBind` / `ShouldBindJSON` / `ShouldBindQuery` 等が内部で `binding` タグを解釈し、バリデーションを自動実行する。失敗時にエラーメッセージをコンポーネントにバインドしてそのまま再表示する。

```go
// internal/handler/alert_rule.go
type CreateRuleRequest struct {
    DeviceID  int64   `form:"device_id" binding:"required,min=1"`
    Metric    string  `form:"metric"    binding:"required,oneof=temperature humidity"`
    Operator  string  `form:"operator"  binding:"required,oneof=> < >= <="`
    Threshold float64 `form:"threshold" binding:"required"`
    IsEnabled bool    `form:"is_enabled"`
}

func (h *AlertRule) Store(c *gin.Context) {
    var req CreateRuleRequest
    if err := c.ShouldBind(&req); err != nil {
        if c.GetHeader("HX-Request") != "" {
            // HTMX: フォームコンポーネントを 422 で返す (エラー表示込み)
            c.Status(http.StatusUnprocessableEntity)
            if renderErr := page.RuleFormOOB(req, validationErrors(err)).Render(
                c.Request.Context(),
                c.Writer,
            ); renderErr != nil {
                c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": renderErr.Error()})
            }
            return
        }
        // 通常: リダイレクト + flash
        sessionFlashError(c, err)
        c.Redirect(http.StatusFound, "/alerts/rules")
        return
    }

    // バリデーション成功後の処理...
}
```

### 422 レスポンスの swap 設定

**採用方針:** 共通レイアウト (`App.templ`) の body 末尾で `htmx.config.responseHandling` を設定し、422 を swap 対象に含める。

```html
<!-- App.templ の body 末尾 -->
<script>
    htmx.config.responseHandling = [
        {code: "204", swap: false},
        {code: "[23]..", swap: true},
        {code: "422", swap: true},
        {code: "[45]..", swap: false}
    ];
</script>
```

> `htmx:responseError` イベントフック方式は使用しない。`responseHandling` 設定の方が宣言的であり、個別のイベントリスナー管理が不要なため。

> エラーメッセージは独立した id 要素ではなく、**フォームコンポーネント内にインラインで含める**（`RuleFormOOB` の再返却でエラーを含んだフォームを差し替える）。

---

## 7. CSRF対応方針

> **cc-sdd への価値:**
> CSRF 保護は POST / PUT / DELETE / PATCH すべてに適用される。HTMX でこれらを使う際の設定方法はプロジェクト固有の選択であり、未記録だと全ミューテーションリクエストが 403 エラーになる。

**採用方針:** Gin 用の CSRF ミドルウェア (`github.com/utrack/gin-csrf` などの外部ライブラリ) を Web ルートグループに適用し、レイアウト templ に `<meta name="csrf-token">` を配置して `htmx:configRequest` イベントでグローバルにヘッダーをセットする。**フォームごとに hidden トークンを書く方式は使用しない。**

```go
// cmd/server/main.go (Web グループの設定)
import (
    csrf "github.com/utrack/gin-csrf"
    "github.com/gin-contrib/sessions"
)

webGroup := r.Group("", csrf.Middleware(csrf.Options{
    Secret: cfg.CSRFSecret,
    ErrorFunc: func(c *gin.Context) {
        c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "CSRF token mismatch"})
    },
    TokenGetter: func(c *gin.Context) string {
        // X-CSRF-Token ヘッダ優先 (HTMX 用)。フォーム送信時は _csrf フィールドをフォールバック
        if t := c.GetHeader("X-CSRF-Token"); t != "" {
            return t
        }
        return c.PostForm("_csrf")
    },
}))
```

```templ
<!-- App.templ の head 内 -->
templ App() {
    <head>
        ...
        <meta name="csrf-token" content={ csrfToken(ctx) }/>
    </head>
    ...
}

<!-- App.templ の body 末尾 -->
<script>
    document.addEventListener('htmx:configRequest', function(event) {
        event.detail.headers['X-CSRF-Token'] =
            document.querySelector('meta[name="csrf-token"]').content;
    });
</script>
```

> `gin-csrf` ミドルウェアは `X-CSRF-Token` ヘッダを検証する。グローバル設定によりフォームに hidden トークンを書く必要はない。

> `csrfToken(ctx)` は Gin context から CSRF トークンを取り出すヘルパー関数（Handler 側で `csrf.GetToken(c)` で取得して templ に渡す）。

---

## 8. `HX-Redirect` 使用方針

> **cc-sdd への価値:**
> HTMX リクエストに対して通常の `c.Redirect()` は効かない（HTMX は HTML を swap するため）。`HX-Redirect` レスポンスヘッダーを使う必要があるが、どの操作で使うかはプロジェクト固有の設計決定。

```go
// Handler での HX-Redirect 返却例（デバイス削除後）
func (h *Device) Destroy(c *gin.Context) {
    deviceID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
    if err := h.Repo.SoftDeleteDevice(c.Request.Context(), deviceID); err != nil {
        c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
        return
    }

    if c.GetHeader("HX-Request") != "" {
        c.Header("HX-Redirect", "/dashboard")
        c.Status(http.StatusNoContent)
        return
    }
    c.Redirect(http.StatusFound, "/dashboard")
}
```

**本プロジェクトでの使用箇所:**

| 操作 | リダイレクト先 | 理由 |
|------|-------------|------|
| デバイス削除（DELETE /devices/{id}） | `/dashboard` | デバイスが存在しなくなるため詳細ページに戻れない |

> HTMX は `HX-Redirect` ヘッダーを受け取るとブラウザをフルページ遷移させる。

---

## 9. ページネーションの HTMX 統合方針

> **cc-sdd への価値:**
> sqlc クエリ + 自前のページング構造 (`LIMIT $4 OFFSET $5`) で実装するため、Laravel のような組み込みヘルパーはない。リンク生成とどう組み合わせるかはプロジェクト固有の設計決定。

**採用方針:** ページネーションコンテナに `hx-boost` + `hx-target` + `hx-swap` を設定する。自前の `PaginationView` コンポーネントでリンク群を生成する。

```templ
// internal/view/component/Pagination.templ
type Pagination struct {
    CurrentPage int
    TotalPages  int
    BaseURL     string
}

templ PaginationView(p Pagination) {
    <div hx-boost="true" hx-target="closest .table-wrapper .inner" hx-swap="innerHTML">
        if p.CurrentPage > 1 {
            <a href={ templ.URL(fmt.Sprintf("%s?page=%d", p.BaseURL, p.CurrentPage-1)) }>前へ</a>
        }
        for page := 1; page <= p.TotalPages; page++ {
            if page == p.CurrentPage {
                <span class="current">{ fmt.Sprint(page) }</span>
            } else {
                <a href={ templ.URL(fmt.Sprintf("%s?page=%d", p.BaseURL, page)) }>{ fmt.Sprint(page) }</a>
            }
        }
    </div>
}
```

> `hx-boost` はコンテナ内のリンクを自動的に HTMX リクエストに変換する。`hx-target` を明示しないと `<body>` 全体が対象になるため必ず指定する。

**本プロジェクトでのページネーション使用箇所:**

| 画面 | ページネーション対象 | ターゲット id | 件数クエリ |
|------|-----------------|------------|----------|
| センサーデータ履歴 | `ListSensorReadingsPaginated` | `#readings-table` | `CountSensorReadingsInRange` |
| アラート履歴 | `ListAlertHistoriesPaginated` | `#history-table` | `CountAlertHistoriesInRange` |

---

## 10. 削除確認方針

> **cc-sdd への価値:**
> 削除操作の確認方法はプロジェクト固有の設計決定。Alpine.js モーダルか `hx-confirm` かで templ テンプレートの構造と依存ライブラリが変わる。

**採用方針:** 簡易な確認には `hx-confirm` 属性（HTMX 組み込み）、見た目を整えたい削除（デバイス削除）には Alpine.js カスタムモーダルを使用する。

```html
<!-- 簡易 (hx-confirm) - アラートルール削除等 -->
<button
  hx-delete="/alerts/rules/1"
  hx-confirm="このアラートルールを削除しますか？">
  削除
</button>

<!-- 見た目重視 (Alpine.js モーダル) - デバイス削除 -->
<body x-data="{ deleteOpen: false }">
    <button @click="deleteOpen = true" class="btn btn-danger">削除</button>

    <div class="modal-overlay" x-show="deleteOpen" style="display:none;">
        <div class="modal-content">
            <p>「ハウスA温湿度計」を削除しますか？</p>
            <button @click="deleteOpen = false">キャンセル</button>
            <button hx-delete="/devices/1">削除する</button>
        </div>
    </div>
</body>
```

**ルール:**
- 確認メッセージは日本語とし、削除対象の名前を含める
- シンプルな行内削除（アラートルール削除）は `hx-confirm`
- 見た目を整えたい削除（デバイス削除）は Alpine.js モーダル

---

更新日時: 2026-04-20
