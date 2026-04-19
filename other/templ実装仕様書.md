
---

# 農業IoTシステム templ実装仕様書

## 本ドキュメントについて

**目的:** HTML モックを templ テンプレートに変換し、HTMX 属性を付与するための内部実装仕様書
**対象読者:** バックエンド実装者 (Go)
**関連文書:**
- `2cc_sdd/画面設計書(静的).md` — 画面設計書 (ワイヤーフレーム・ダミーデータ定義)
- `2cc_sdd/DB設計書.md` — テーブル・カラム定義、Enum 定義、クエリパターン
- `2cc_sdd/HTMX実装ガイド(動的).md` — HTMX 部分更新の動的振る舞い仕様

### 実装手順の概要

1. `mocks/html/*.html` を templ 構文に変換する (`.templ` ファイル)
2. 共通レイアウト (`internal/view/layout/App.templ`, `internal/view/layout/Guest.templ`) を作成し、各画面をページコンポーネントとして切り出す
3. ダミーデータを templ 関数の引数 (Go struct) に置き換える
4. 下記「id 属性一覧」に従い、HTMX 部分更新のターゲットとなる要素に id 属性を付与する
5. 本仕様書に従い HTMX 属性 (`hx-get`, `hx-post` 等) を付与する
6. 部分 HTML レスポンスを返す Echo Handler を実装する

### HTMX 実装規約

**templ コンポーネント分割アプローチ:** 本システムでは部分テンプレートファイルを分離せず、ページ用 templ ファイル内に **細分化したコンポーネント関数** として部分更新領域を定義する。Handler は対象コンポーネント関数を `.Render(ctx, w)` で直接呼び出す。

**templ テンプレート側 (`internal/view/page/Dashboard.templ`):**

```templ
package page

import "github.com/HiroshiKawano/go_iot/internal/repository"

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

// 部分更新ターゲット: デバイスカード一覧 (innerHTML swap)
templ DeviceCards(devices []repository.Device) {
    for _, d := range devices {
        <article class="device-card">
            <h3>{ d.Name }</h3>
        </article>
    }
}

// 部分更新ターゲット: 未対応アラートバナー (innerHTML swap)
templ AlertBanner(alerts []AlertView) {
    if len(alerts) == 0 {
        <p class="empty-message">未対応のアラートはありません。</p>
    } else {
        <section class="alert-banner">
            <ul>
                for _, a := range alerts {
                    <li>{ a.Message }</li>
                }
            </ul>
        </section>
    }
}
```

**Echo Handler 側 (`internal/handler/dashboard.go`):**

```go
func (h *Dashboard) Index(c echo.Context) error {
    devices, _ := h.Repo.ListDevicesByUser(ctx, userID)
    alerts, _ := buildAlertViews(ctx, h.Repo, userID)

    if c.Request().Header.Get("HX-Request") != "" {
        // HTMX リクエストの場合、対象コンポーネントのみを返す
        // (ターゲット要素の innerHTML として swap される)
        return page.DeviceCards(devices).Render(c.Request().Context(), c.Response())
    }
    // 通常リクエスト → フルページを返す
    return page.Dashboard(devices, alerts).Render(c.Request().Context(), c.Response())
}
```

**複数コンポーネントの同時返却 (OOB Swap 用):**

```go
func (h *Readings) Index(c echo.Context) error {
    readings, _ := h.Repo.ListSensorReadingsPaginated(ctx, ...)
    summary, _ := h.Repo.GetSensorReadingsSummary(ctx, ...)

    if c.Request().Header.Get("HX-Request") != "" {
        // 2 つのコンポーネントを連続で書き出す
        w := c.Response().Writer
        ctx := c.Request().Context()
        if err := page.ReadingsTable(readings).Render(ctx, w); err != nil {
            return err
        }
        // OOB swap 用コンポーネント
        return page.ReadingsSummaryOOB(summary).Render(ctx, w)
    }
    return page.Readings(device, readings, summary).Render(ctx, c.Response())
}
```

**コンポーネント命名規約:** 部分更新ターゲット用の templ 関数名は、ターゲット id を PascalCase にしたものとする (例: `#device-cards` → `templ DeviceCards(...)`).

**hx-swap デフォルト動作:**
- HTMX の既定の swap は `innerHTML` (ターゲット要素の中身を置換)。本システムでもこれを標準とする
- `hx-swap` 属性を明示する必要があるのは `outerHTML` を使う場合のみ
- OOB Swap (`hx-swap-oob="true"`) は常に `outerHTML` で動作する (id が一致する要素全体を差し替え)

**コンポーネントの 2 つの配置パターン:**

1. **メインコンポーネント (innerHTML swap 用):** ターゲット要素の **内側** に展開される中身を返す。

```templ
// ページ側
<div id="device-cards">
    @DeviceCards(devices)
</div>

// Handler から HTMX レスポンスとして直接呼ぶと、中身 HTML のみが返る
// → ターゲット <div id="device-cards"> の innerHTML として swap される
```

2. **OOB コンポーネント (outerHTML swap 用):** `id` と `hx-swap-oob="true"` を持つ要素 **全体** を返す。

```templ
templ ReadingsSummaryOOB(s Summary) {
    <div id="readings-summary" hx-swap-oob="true">
        <p>平均温度: { fmt.Sprintf("%.2f", s.AvgTemp) }℃</p>
    </div>
}
```

> ※ `hx-swap-oob` 属性はフルページ表示時にはブラウザに無視されるため、常に付与して問題ない。

---

## 画面別実装仕様

---

### 1. ログイン

**HTML モック:** `mocks/html/login.html`
**templ ファイル:** `internal/view/page/Login.templ`
**URL:** `/login`
**認証:** 不要 (未ログイン用)
**レイアウト:** `internal/view/layout/Guest.templ`

> `scs` セッションを使った自作ログイン機能。bcrypt でパスワードハッシュを照合する。

**フォーム送信:**

| 操作 | メソッド | URL | 成功時 |
|------|---------|-----|-------|
| ログイン | POST | /login | /dashboard にリダイレクト |

---

### 2. ユーザー登録

**HTML モック:** `mocks/html/register.html`
**templ ファイル:** `internal/view/page/Register.templ`
**URL:** `/register`
**認証:** 不要 (未ログイン用)
**レイアウト:** `internal/view/layout/Guest.templ`

**フォーム送信:**

| 操作 | メソッド | URL | 成功時 |
|------|---------|-----|-------|
| 登録 | POST | /register | /dashboard にリダイレクト |

---

### 3. ダッシュボード

**HTML モック:** `mocks/html/dashboard.html`
**templ ファイル:** `internal/view/page/Dashboard.templ`
**URL:** `/dashboard`
**認証:** 必要 (scs セッション)
**レイアウト:** `internal/view/layout/App.templ`

**データソース:**

| 表示項目 | データソース | 備考 |
|---------|------------|------|
| アラート通知 | `alert_histories` (is_notified=false) | 未通知のもの。表示文は下記参照 |
| デバイス名 | `devices.name` | |
| 設置場所 | `devices.location` | |
| 稼働状態 | `devices.is_active` | ● 稼働中 / ○ 停止中 |
| 最新温度 | `sensor_readings.temperature` | 最新1件 (`GetLatestSensorReading`) |
| 最新湿度 | `sensor_readings.humidity` | 最新1件 |
| 最終通信 | `devices.last_communicated_at` | 相対時間表示 (Go の `time.Since` + 独自 humanize) |

**アラート通知文の組み立て:**

表示例: `⚠ ハウスA温湿度計: 温度が35℃を超えました (38.50℃)`

| 構成要素 | データソース |
|---------|------------|
| デバイス名 | JOIN 先 `devices.name` (sqlc の `ListUnnotifiedAlertHistoriesWithDevice`) |
| 指標名 | `alert_histories.metric` → `domain.Metric.Label()` |
| ルール条件 | `alert_histories.operator` → `domain.ComparisonOperator.Label()` + `alert_histories.threshold` |
| 実測値 | `alert_histories.actual_value` + `domain.Metric.Unit()` |

**画面遷移リンク:**

| ボタン/リンク | 遷移先 URL |
|-------------|----------|
| [+ デバイス登録] | /devices/create |
| [詳細を見る] | /devices/{id} |

**HTMX 操作:**

| 操作 | メソッド | URL | ターゲット | トリガー |
|------|---------|-----|----------|---------|
| デバイスカード自動更新 | GET | /dashboard/devices | #device-cards | every 60s |
| アラートバナー自動更新 | GET | /dashboard/alerts | #alert-banner | every 60s |

---

### 4. デバイス詳細

**HTML モック:** `mocks/html/device-show.html`
**templ ファイル:** `internal/view/page/DeviceShow.templ`
**URL:** `/devices/{device}`
**認証:** 必要 (scs セッション)
**レイアウト:** `internal/view/layout/App.templ`

**データソース:**

| 表示項目 | データソース | 備考 |
|---------|------------|------|
| デバイス情報 | `devices` テーブル (`GetDevice`) | name, mac_address, location, is_active, last_communicated_at |
| 温度グラフ | `sensor_readings.temperature` | サーバサイド SVG 生成。期間に応じたデータ |
| 湿度グラフ | `sensor_readings.humidity` | サーバサイド SVG 生成。期間に応じたデータ |
| 計測データ一覧 | `sensor_readings` | recorded_at, temperature, humidity。直近 20 件 |

**画面遷移リンク:**

| ボタン/リンク | 遷移先 URL |
|-------------|----------|
| [編集] | /devices/{id}/edit |
| [もっと見る →] | /devices/{id}/readings |

**HTMX 操作:**

| 操作 | メソッド | URL | ターゲット | トリガー |
|------|---------|-----|----------|---------|
| 期間切替 (温度グラフ) | GET | /devices/{id}/chart/temperature?period={値} | #temperature-chart | click |
| 期間切替 (湿度グラフ) | GET | /devices/{id}/chart/humidity?period={値} | #humidity-chart | click |
| 最新データ自動更新 | GET | /devices/{id}/latest-readings | #latest-readings | every 60s |
| デバイス削除 | DELETE | /devices/{id} | - | click (確認モーダル後) → /dashboard にリダイレクト (`HX-Redirect` ヘッダ) |

**期間パラメータ (period) の値:**

| 表示 | 値 | データ範囲 | 使用クエリ |
|------|-----|----------|----------|
| 24時間 | 24h | 直近 24 時間の生データ | `ListRecentSensorReadings` |
| 7日間 | 7d | 直近 7 日間の日次集計 | `ListDailySensorAggregates` |
| 30日間 | 30d | 直近 30 日間の日次集計 | `ListDailySensorAggregates` |

---

### 5. デバイス登録 / 編集

**HTML モック:** `mocks/html/device-create.html`, `mocks/html/device-edit.html`
**templ ファイル:**
- 登録: `internal/view/page/DeviceCreate.templ`
- 編集: `internal/view/page/DeviceEdit.templ`

**URL:**
- 登録: `/devices/create`
- 編集: `/devices/{device}/edit`

**認証:** 必要 (scs セッション)
**レイアウト:** `internal/view/layout/App.templ`

**フォーム項目のDB対応:**

| 項目 | name | バリデーション | DB 対応 |
|------|------|-------------|--------|
| デバイス名 | name | 必須、255 文字以内 | `devices.name` |
| MAC アドレス | mac_address | 必須、形式: XX:XX:XX:XX:XX:XX、ユニーク | `devices.mac_address` |
| 設置場所 | location | 任意、255 文字以内 | `devices.location` |
| ステータス | is_active | 必須 | `devices.is_active` |

> バリデーションは `go-playground/validator` を使用し、Handler 内の request struct に tag を付ける。

**フォーム送信仕様:**

> ※ この画面は HTMX ではなく通常のフォーム送信 (フルページ遷移) を使用する。

| 操作 | メソッド | URL | 成功時の遷移先 |
|------|---------|-----|-------------|
| 新規登録 | POST | /devices | /devices/{新規ID} (デバイス詳細) — `c.Redirect()` |
| 更新 | PUT | /devices/{id} | /devices/{id} (デバイス詳細) — `c.Redirect()` |
| キャンセル | - | - | 前の画面に戻る (ブラウザバック) |

> HTTP の PUT/DELETE は HTML フォームが直接サポートしないため、`<input type="hidden" name="_method" value="put">` と method override ミドルウェアを組み合わせる。

---

### 6. センサーデータ履歴

**HTML モック:** `mocks/html/readings.html`
**templ ファイル:** `internal/view/page/Readings.templ`
**URL:** `/devices/{device}/readings`
**認証:** 必要 (scs セッション)
**レイアウト:** `internal/view/layout/App.templ`

**データソース:**

| 表示項目 | データソース | 備考 |
|---------|------------|------|
| 集計 (平均/最高/最低) | `GetSensorReadingsSummary` | フィルター期間内の温度・湿度 |
| 計測日時 | `sensor_readings.recorded_at` | |
| 温度 | `sensor_readings.temperature` | 小数 2 桁 (`pgconv.NumericToFloat`) |
| 湿度 | `sensor_readings.humidity` | 小数 2 桁 |
| 通信遅延 | created_at - recorded_at の差分 | 秒単位で表示 |

**HTMX 操作:**

| 操作 | メソッド | URL | ターゲット | トリガー |
|------|---------|-----|----------|---------|
| 期間フィルター検索 | GET | /devices/{id}/readings?from=...&to=... | #readings-table, #readings-summary | submit |
| ページネーション | GET | /devices/{id}/readings?page=2 | #readings-table | click |

---

### 7. アラートルール管理

**HTML モック:** `mocks/html/alert-rules.html`
**templ ファイル:** `internal/view/page/AlertRules.templ`
**URL:** `/alerts/rules`
**認証:** 必要 (scs セッション)
**レイアウト:** `internal/view/layout/App.templ`

**データソース:**

| 表示項目 | データソース | 備考 |
|---------|------------|------|
| 有効/無効 | `alert_rules.is_enabled` | チェックボックスで切替 |
| 指標 | `alert_rules.metric` | `domain.Metric.Label()` で表示 |
| 条件 | `alert_rules.operator` | `domain.ComparisonOperator.Label()` で表示 |
| 閾値 | `alert_rules.threshold` | `domain.Metric.Unit()` を末尾に付与 |

**フォーム項目のDB対応:**

| 項目 | name | 選択肢 | バリデーション | DB 対応 |
|------|------|--------|-------------|--------|
| デバイス ID | device_id | (デバイス選択と連動) | 必須、`devices` に存在 | `alert_rules.device_id` |
| 指標 | metric | 温度 / 湿度 | 必須、Metric Enum 値のみ | `alert_rules.metric` |
| 条件 | operator | より大きい / より小さい / 以上 / 以下 | 必須、ComparisonOperator Enum 値のみ | `alert_rules.operator` |
| 閾値 | threshold | - | 必須、数値 | `alert_rules.threshold` |

**HTMX 操作:**

| 操作 | メソッド | URL | ターゲット | トリガー |
|------|---------|-----|----------|---------|
| デバイス切替 | GET | /alerts/rules?device_id={id} | #rules-list, #rule-form (OOB) | change |
| ルール追加 | POST | /alerts/rules | #rules-list | submit |
| ルール編集フォーム取得 | GET | /alerts/rules/{ruleId}/edit | #rule-form | click |
| ルール更新 | PUT | /alerts/rules/{ruleId} | #rules-list, #rule-form (OOB) | submit |
| 有効/無効切替 | PATCH | /alerts/rules/{ruleId}/toggle | #rules-list | change |
| ルール削除 | DELETE | /alerts/rules/{ruleId} | #rules-list | click (確認後) |

> ※ 編集時は追加フォームを編集フォームとして再利用する。GET で既存値をフォームにロードし、PUT で更新。更新完了後は空の追加フォームに OOB Swap で戻す。

---

### 8. アラート履歴

**HTML モック:** `mocks/html/alert-history.html`
**templ ファイル:** `internal/view/page/AlertHistory.templ`
**URL:** `/alerts/history`
**認証:** 必要 (scs セッション)
**レイアウト:** `internal/view/layout/App.templ`

**データソース:**

| 表示項目 | データソース | 備考 |
|---------|------------|------|
| 発火日時 | `alert_histories.triggered_at` | |
| デバイス名 | JOIN 先 `devices.name` | `ListAlertHistoriesPaginated` |
| 指標 | `alert_histories.metric` | `domain.Metric.Label()` |
| ルール条件 | `alert_histories.operator`, `alert_histories.threshold` | 非正規化カラム。Enum の `Label()` + `Unit()` |
| 実測値 | `alert_histories.actual_value` | `domain.Metric.Unit()` を付与 |
| 通知状態 | `alert_histories.is_notified` | 済 / 未 |

**HTMX 操作:**

| 操作 | メソッド | URL | ターゲット | トリガー |
|------|---------|-----|----------|---------|
| フィルター検索 | GET | /alerts/history?device_id=...&from=...&to=... | #history-table | submit |
| ページネーション | GET | /alerts/history?page=2 | #history-table | click |

---

## id 属性一覧

HTML モックを templ に変換する際に付与する id 属性。HTMX 部分更新のターゲットとして使用する。

> ※ モック HTML には id 属性は含まれていない (HTML作成ルール R01)。templ 変換時にこちらで付与する。

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
| アラートルール管理 | `rule-form` | ルール追加/編集フォーム |
| アラートルール管理 | `rules-list` | ルール一覧テーブル |
| アラート履歴 | `history-filter` | フィルターフォーム |
| アラート履歴 | `history-table` | 履歴一覧テーブル |

---

## HTMX 部分更新仕様まとめ (横断リファレンス)

> ※ このセクションは画面別実装仕様の HTMX 情報を横断的に一覧化したリファレンスである。
> **正 (権威あるソース) は各画面の「HTMX 操作」テーブル。** 矛盾がある場合は画面別仕様を優先する。
> Handler やルーティングの実装時に、全エンドポイントを俯瞰する目的で使用する。

Echo Handler が返す部分 HTML レスポンスの一覧。
各レスポンスは HTML フラグメント (`<html>` や `<body>` を含まない部分 HTML) を返す。

### 複数ターゲット同時更新 (hx-swap-oob)

1 つのリクエストで複数の要素を更新する場合、HTMX の **Out of Band Swap (hx-swap-oob)** を使用する。

**仕組み:** メインターゲットの HTML に加えて、`hx-swap-oob="true"` 属性を持つ要素をレスポンスに含めると、その要素は対応する id の位置で自動的に差し替わる。

**templ + Handler の具体例 (センサーデータ履歴):**

```templ
// internal/view/page/Readings.templ

// メインコンポーネント: #readings-table の innerHTML として差し込まれる
templ ReadingsTable(readings []repository.SensorReading, pagination Pagination) {
    <table>
        <thead>...</thead>
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

**Echo Handler:**

```go
// internal/handler/readings.go
func (h *Readings) Index(c echo.Context) error {
    ctx := c.Request().Context()
    readings, _ := h.Repo.ListSensorReadingsPaginated(ctx, ...)
    summary, _ := h.Repo.GetSensorReadingsSummary(ctx, ...)

    if c.Request().Header.Get("HX-Request") != "" {
        // メイン → OOB の順に連続書き込み
        w := c.Response()
        if err := page.ReadingsTable(readings, pagination).Render(ctx, w); err != nil {
            return err
        }
        return page.ReadingsSummaryOOB(summary).Render(ctx, w)
    }
    return page.Readings(device, readings, summary).Render(ctx, c.Response())
}
```

> ※ メインコンポーネントはターゲット要素の **内側** の中身を返す。OOB コンポーネントは `id` + `hx-swap-oob` 付きの要素 **全体** を返す。

**対象エンドポイント:**

| エンドポイント | メインコンポーネント | OOB コンポーネント |
|-------------|------------------|----------------|
| `/devices/{id}/readings` (フィルター検索) | `ReadingsTable` → #readings-table | `ReadingsSummaryOOB` → #readings-summary |
| `/alerts/rules?device_id={id}` (デバイス切替) | `RulesList` → #rules-list | `RuleFormOOB` → #rule-form |
| `/alerts/rules/{id}` PUT (ルール更新) | `RulesList` → #rules-list | `RuleFormOOB` → #rule-form |

### 部分 HTML エンドポイント一覧

| エンドポイント | メソッド | 返却内容 | コンポーネント | ターゲット id |
|-------------|---------|---------|----------|------------|
| `/dashboard/devices` | GET | デバイスカード一覧 | `DeviceCards` | #device-cards |
| `/dashboard/alerts` | GET | 未対応アラートバナー | `AlertBanner` | #alert-banner |
| `/devices/{id}/chart/temperature` | GET | 温度SVGグラフ | `TemperatureChart` | #temperature-chart |
| `/devices/{id}/chart/humidity` | GET | 湿度SVGグラフ | `HumidityChart` | #humidity-chart |
| `/devices/{id}/latest-readings` | GET | 最新計測データテーブル | `LatestReadings` | #latest-readings |
| `/devices/{id}/readings` | GET | センサーデータテーブル + 集計 | `ReadingsTable`, `ReadingsSummaryOOB` | #readings-table, #readings-summary |
| `/alerts/rules` | GET | ルール一覧 + フォーム (デバイス切替時) | `RulesList`, `RuleFormOOB` | #rules-list, #rule-form |
| `/alerts/rules` | POST | ルール一覧 (追加後) | `RulesList` | #rules-list |
| `/alerts/rules/{id}/edit` | GET | 編集フォーム (既存値ロード済み) | `RuleForm` | #rule-form |
| `/alerts/rules/{id}` | PUT | ルール一覧 + フォームリセット | `RulesList`, `RuleFormOOB` | #rules-list, #rule-form |
| `/alerts/rules/{id}/toggle` | PATCH | ルール一覧 (切替後) | `RulesList` | #rules-list |
| `/alerts/rules/{id}` | DELETE | ルール一覧 (削除後) | `RulesList` | #rules-list |
| `/alerts/history` | GET | アラート履歴テーブル | `HistoryTable` | #history-table |

### 自動更新 (ポーリング)

| 画面 | エンドポイント | 間隔 | 備考 |
|------|-------------|------|------|
| ダッシュボード | `/dashboard/devices` | 60秒 | `hx-trigger="every 60s"` |
| ダッシュボード | `/dashboard/alerts` | 60秒 | `hx-trigger="every 60s"` |
| デバイス詳細 | `/devices/{id}/latest-readings` | 60秒 | `hx-trigger="every 60s"` |

> ※ ポーリング間隔は運用要件に応じて調整可能。必須ではなく手動リロードでも可。

---

## templ ファイル一覧

### フルページテンプレート

HTML モックから変換するテンプレート。各画面のメインビュー。

| ファイルパス | 画面 | HTML モック対応 |
|------------|------|-------------|
| `internal/view/layout/App.templ` | 共通レイアウト (認証済み) | 各 HTML 共通部分 |
| `internal/view/layout/Guest.templ` | ゲストレイアウト (未認証) | login.html / register.html 共通部分 |
| `internal/view/page/Login.templ` | ログイン | login.html |
| `internal/view/page/Register.templ` | ユーザー登録 | register.html |
| `internal/view/page/Dashboard.templ` | ダッシュボード | dashboard.html |
| `internal/view/page/DeviceShow.templ` | デバイス詳細 | device-show.html |
| `internal/view/page/DeviceCreate.templ` | デバイス登録 | device-create.html |
| `internal/view/page/DeviceEdit.templ` | デバイス編集 | device-edit.html |
| `internal/view/page/Readings.templ` | センサーデータ履歴 | readings.html |
| `internal/view/page/AlertRules.templ` | アラートルール管理 | alert-rules.html |
| `internal/view/page/AlertHistory.templ` | アラート履歴 | alert-history.html |

### 部分更新用コンポーネント関数一覧 (HTMX 用)

フルページテンプレートと同じ `.templ` ファイル内に、細分化した `templ` 関数として部分更新領域を定義する。別ファイルには分割しない。

| 所属ファイル | コンポーネント関数 | 用途 | パターン |
|---------------|------------|------|---------|
| `Dashboard.templ` | `DeviceCards` | デバイスカード一覧 | メイン |
| `Dashboard.templ` | `AlertBanner` | 未対応アラートバナー | メイン |
| `DeviceShow.templ` | `TemperatureChart` | 温度 SVG グラフ | メイン |
| `DeviceShow.templ` | `HumidityChart` | 湿度 SVG グラフ | メイン |
| `DeviceShow.templ` | `LatestReadings` | 最新計測データテーブル | メイン |
| `Readings.templ` | `ReadingsTable` | センサーデータテーブル | メイン |
| `Readings.templ` | `ReadingsSummaryOOB` | 集計情報 | OOB |
| `AlertRules.templ` | `RulesList` | ルール一覧テーブル | メイン |
| `AlertRules.templ` | `RuleForm` / `RuleFormOOB` | ルール追加/編集フォーム | メイン / OOB |
| `AlertHistory.templ` | `HistoryTable` | アラート履歴テーブル | メイン |

> ※ メイン: ターゲット要素の内側に展開される中身を返す (innerHTML swap)。OOB: `id` + `hx-swap-oob="true"` 付きの要素全体を返す (outerHTML swap)。

---

更新日時: 2026-04-20
