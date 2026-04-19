
---

# 農業IoTシステム Blade実装仕様書

## 本ドキュメントについて

**目的:** 外部デザイナーが作成したHTMLモックをBladeテンプレートに変換し、HTMX属性を付与するための内部実装仕様書
**対象読者:** バックエンド実装者
**関連文書:**
- HTMLモック依頼書.md — 外部デザイナーへの画面設計書（ワイヤーフレーム・ダミーデータ定義）
- DB設計書.md — テーブル・カラム定義、Enum定義、クエリパターン

### 実装手順の概要

1. デザイナーから納品されたHTMLモックをBlade構文に変換する
2. 共通レイアウト（`layouts/app.blade.php`, `layouts/guest.blade.php`）を作成し、各画面をセクション化する
3. ダミーデータをBlade変数（`{{ $variable }}`）に置き換える
4. 下記「id属性一覧」に従い、HTMX部分更新のターゲットとなる要素にid属性を付与する
5. 本仕様書に従いHTMX属性（`hx-get`, `hx-post` 等）を付与する
6. 部分HTMLレスポンスを返すControllerを実装する

### HTMX実装規約

**Blade Fragment アプローチ:** 本システムでは部分テンプレートファイルを分離せず、フルページテンプレート内に `@fragment` / `@endfragment` で部分更新領域を定義する。Controller は `->fragment()` で返却範囲を切り出す。

**Blade テンプレート側:**

```blade
{{-- dashboard.blade.php --}}
<div id="device-cards">
    @fragment('device-cards')
    @foreach ($devices as $device)
        <div class="card">{{ $device->name }}</div>
    @endforeach
    @endfragment
</div>
```

**Controller 側:**

```php
if ($request->header('HX-Request')) {
    // @fragment('device-cards') の範囲のみ返す（レイアウト自動除外）
    return view('dashboard', compact('devices', 'alerts'))
        ->fragment('device-cards');
}
// 通常リクエスト → フルページを返す
return view('dashboard', compact('devices', 'alerts'));
```

**複数 Fragment の同時返却（OOB Swap 用）:**

```php
return view('readings.index', compact('readings', 'summary'))
    ->fragments(['readings-table', 'readings-summary']);
```

**Fragment 命名規約:** Fragment 名はターゲット id と同一にする（例: `#device-cards` → `@fragment('device-cards')`）。

**hx-swap デフォルト動作:**
- HTMX の既定の swap は `innerHTML`（ターゲット要素の中身を置換）。本システムでもこれを標準とする
- `hx-swap` 属性を明示する必要があるのは `outerHTML` を使う場合のみ
- OOB Swap（`hx-swap-oob="true"`）は常に `outerHTML` で動作する（id が一致する要素全体を差し替え）

**Fragment の2つの配置パターン:**

1. **メイン Fragment（innerHTML swap 用）:** ターゲット要素の **内側** に配置。中身だけが返される。

```blade
<div id="device-cards">
    @fragment('device-cards')
    {{-- この範囲のみが HTMX レスポンスとして返される --}}
    @endfragment
</div>
```

2. **OOB Fragment（outerHTML swap 用）:** `id` と `hx-swap-oob="true"` を持つ要素 **全体** を囲む。

```blade
@fragment('readings-summary')
<div id="readings-summary" hx-swap-oob="true">
    {{-- 要素全体が OOB で差し替えられる --}}
</div>
@endfragment
```

> ※ `hx-swap-oob` 属性はフルページ表示時にはブラウザに無視されるため、常に付与して問題ない。

---

## 画面別実装仕様

---

### 1. ログイン

**HTMLモック:** `login.html`
**Bladeファイル:** `resources/views/auth/login.blade.php`
**URL:** `/login`
**認証:** 不要（未ログイン用）
**レイアウト:** `layouts/guest.blade.php`

> Breeze標準のログイン画面。デザイナーのモックでスタイルをカスタマイズする。

**フォーム送信:**

| 操作 | メソッド | URL | 成功時 |
|------|---------|-----|-------|
| ログイン | POST | /login | /dashboard にリダイレクト |

---

### 2. ユーザー登録

**HTMLモック:** `register.html`
**Bladeファイル:** `resources/views/auth/register.blade.php`
**URL:** `/register`
**認証:** 不要（未ログイン用）
**レイアウト:** `layouts/guest.blade.php`

**フォーム送信:**

| 操作 | メソッド | URL | 成功時 |
|------|---------|-----|-------|
| 登録 | POST | /register | /dashboard にリダイレクト |

---

### 3. ダッシュボード

**HTMLモック:** `dashboard.html`
**Bladeファイル:** `resources/views/dashboard.blade.php`
**URL:** `/dashboard`
**認証:** 必要（Session）
**レイアウト:** `layouts/app.blade.php`

**データソース:**

| 表示項目 | データソース | 備考 |
|---------|------------|------|
| アラート通知 | alert_histories (is_notified=false) | 未通知のもの。表示文は下記参照 |
| デバイス名 | devices.name | |
| 設置場所 | devices.location | |
| 稼働状態 | devices.is_active | ● 稼働中 / ○ 停止中 |
| 最新温度 | sensor_readings.temperature | 最新1件 |
| 最新湿度 | sensor_readings.humidity | 最新1件 |
| 最終通信 | devices.last_communicated_at | 相対時間表示（例: 2分前） |

**アラート通知文の組み立て:**

表示例: `⚠ ハウスA温湿度計: 温度が35℃を超えました (38.50℃)`

| 構成要素 | データソース |
|---------|------------|
| デバイス名 | alert_rules → devices.name（alert_rule_id経由） |
| 指標名 | alert_histories.metric → Metric Enumの `label()` |
| ルール条件 | alert_histories.operator → ComparisonOperator Enumの `label()` + alert_histories.threshold |
| 実測値 | alert_histories.actual_value + Metric Enumの `unit()` |

**画面遷移リンク:**

| ボタン/リンク | 遷移先URL |
|-------------|----------|
| [+ デバイス登録] | /devices/create |
| [詳細を見る] | /devices/{id} |

**HTMX操作:**

| 操作 | メソッド | URL | ターゲット | トリガー |
|------|---------|-----|----------|---------|
| デバイスカード自動更新 | GET | /dashboard/devices | #device-cards | every 60s |
| アラートバナー自動更新 | GET | /dashboard/alerts | #alert-banner | every 60s |

---

### 4. デバイス詳細

**HTMLモック:** `device-show.html`
**Bladeファイル:** `resources/views/devices/show.blade.php`
**URL:** `/devices/{device}`
**認証:** 必要（Session）
**レイアウト:** `layouts/app.blade.php`

**データソース:**

| 表示項目 | データソース | 備考 |
|---------|------------|------|
| デバイス情報 | devices テーブル | name, mac_address, location, is_active, last_communicated_at |
| 温度グラフ | sensor_readings.temperature | SVGグラフ（goat1000/svggraph）。期間に応じたデータ |
| 湿度グラフ | sensor_readings.humidity | SVGグラフ（goat1000/svggraph）。期間に応じたデータ |
| 計測データ一覧 | sensor_readings | recorded_at, temperature, humidity。直近20件 |

**画面遷移リンク:**

| ボタン/リンク | 遷移先URL |
|-------------|----------|
| [編集] | /devices/{id}/edit |
| [もっと見る →] | /devices/{id}/readings |

**HTMX操作:**

| 操作 | メソッド | URL | ターゲット | トリガー |
|------|---------|-----|----------|---------|
| 期間切替（温度グラフ） | GET | /devices/{id}/chart/temperature?period={値} | #temperature-chart | click |
| 期間切替（湿度グラフ） | GET | /devices/{id}/chart/humidity?period={値} | #humidity-chart | click |
| 最新データ自動更新 | GET | /devices/{id}/latest-readings | #latest-readings | every 60s |
| デバイス削除 | DELETE | /devices/{id} | - | click（確認ダイアログ後）→ /dashboard にリダイレクト |

**期間パラメータ（period）の値:**

| 表示 | 値 | データ範囲 |
|------|-----|----------|
| 24時間 | 24h | 直近24時間の生データ |
| 7日間 | 7d | 直近7日間の日次集計 |
| 30日間 | 30d | 直近30日間の日次集計 |

---

### 5. デバイス登録 / 編集

**HTMLモック:** `device-create.html`, `device-edit.html`
**Bladeファイル:**
- 登録: `resources/views/devices/create.blade.php`
- 編集: `resources/views/devices/edit.blade.php`

**URL:**
- 登録: `/devices/create`
- 編集: `/devices/{device}/edit`

**認証:** 必要（Session）
**レイアウト:** `layouts/app.blade.php`

**フォーム項目のDB対応:**

| 項目 | name | バリデーション | DB対応 |
|------|------|-------------|--------|
| デバイス名 | name | 必須、255文字以内 | devices.name |
| MACアドレス | mac_address | 必須、形式: XX:XX:XX:XX:XX:XX、ユニーク | devices.mac_address |
| 設置場所 | location | 任意、255文字以内 | devices.location |
| ステータス | is_active | 必須 | devices.is_active |

**フォーム送信仕様:**

> ※ この画面はHTMXではなく通常のフォーム送信（フルページ遷移）を使用する。

| 操作 | メソッド | URL | 成功時の遷移先 |
|------|---------|-----|-------------|
| 新規登録 | POST | /devices | /devices/{新規ID}（デバイス詳細） |
| 更新 | PUT | /devices/{id} | /devices/{id}（デバイス詳細） |
| キャンセル | - | - | 前の画面に戻る（ブラウザバック） |

---

### 6. センサーデータ履歴

**HTMLモック:** `readings.html`
**Bladeファイル:** `resources/views/readings/index.blade.php`
**URL:** `/devices/{device}/readings`
**認証:** 必要（Session）
**レイアウト:** `layouts/app.blade.php`

**データソース:**

| 表示項目 | データソース | 備考 |
|---------|------------|------|
| 集計（平均/最高/最低） | sensor_readings の集計クエリ | フィルター期間内の温度・湿度 |
| 計測日時 | sensor_readings.recorded_at | |
| 温度 | sensor_readings.temperature | 小数2桁 |
| 湿度 | sensor_readings.humidity | 小数2桁 |
| 通信遅延 | created_at - recorded_at の差分 | 秒単位で表示 |

**HTMX操作:**

| 操作 | メソッド | URL | ターゲット | トリガー |
|------|---------|-----|----------|---------|
| 期間フィルター検索 | GET | /devices/{id}/readings?from=...&to=... | #readings-table, #readings-summary | submit |
| ページネーション | GET | /devices/{id}/readings?page=2 | #readings-table | click |

---

### 7. アラートルール管理

**HTMLモック:** `alert-rules.html`
**Bladeファイル:** `resources/views/alerts/rules.blade.php`
**URL:** `/alerts/rules`
**認証:** 必要（Session）
**レイアウト:** `layouts/app.blade.php`

**データソース:**

| 表示項目 | データソース | 備考 |
|---------|------------|------|
| 有効/無効 | alert_rules.is_enabled | チェックボックスで切替 |
| 指標 | alert_rules.metric | Metric Enumの `label()` で表示 |
| 条件 | alert_rules.operator | ComparisonOperator Enumの `label()` で表示 |
| 閾値 | alert_rules.threshold | Metric Enumの `unit()` を末尾に付与 |

**フォーム項目のDB対応:**

| 項目 | name | 選択肢 | バリデーション | DB対応 |
|------|------|--------|-------------|--------|
| デバイスID | device_id | （デバイス選択と連動） | 必須、devices テーブルに存在 | alert_rules.device_id |
| 指標 | metric | 温度 / 湿度 | 必須、Metric Enum の値のみ | alert_rules.metric (Metric Enum) |
| 条件 | operator | より大きい / より小さい / 以上 / 以下 | 必須、ComparisonOperator Enum の値のみ | alert_rules.operator (ComparisonOperator Enum) |
| 閾値 | threshold | - | 必須、数値 | alert_rules.threshold |

**HTMX操作:**

| 操作 | メソッド | URL | ターゲット | トリガー |
|------|---------|-----|----------|---------|
| デバイス切替 | GET | /alerts/rules?device_id={id} | #rules-list, #rule-form (OOB) | change |
| ルール追加 | POST | /alerts/rules | #rules-list | submit |
| ルール編集フォーム取得 | GET | /alerts/rules/{ruleId}/edit | #rule-form | click |
| ルール更新 | PUT | /alerts/rules/{ruleId} | #rules-list, #rule-form (OOB) | submit |
| 有効/無効切替 | PATCH | /alerts/rules/{ruleId}/toggle | #rules-list | change |
| ルール削除 | DELETE | /alerts/rules/{ruleId} | #rules-list | click（確認後） |

> ※ 編集時は追加フォームを編集フォームとして再利用する。GETで既存値をフォームにロードし、PUTで更新。更新完了後は空の追加フォームに OOB Swap で戻す。

---

### 8. アラート履歴

**HTMLモック:** `alert-history.html`
**Bladeファイル:** `resources/views/alerts/history.blade.php`
**URL:** `/alerts/history`
**認証:** 必要（Session）
**レイアウト:** `layouts/app.blade.php`

**データソース:**

| 表示項目 | データソース | 備考 |
|---------|------------|------|
| 発火日時 | alert_histories.triggered_at | |
| デバイス名 | alert_rules → devices.name | alert_rule_id経由 |
| 指標 | alert_histories.metric | Metric Enumの `label()` |
| ルール条件 | alert_histories.operator, alert_histories.threshold | 非正規化カラム。Enumの `label()` + `unit()` |
| 実測値 | alert_histories.actual_value | Metric Enumの `unit()` を付与 |
| 通知状態 | alert_histories.is_notified | 済 / 未 |

**HTMX操作:**

| 操作 | メソッド | URL | ターゲット | トリガー |
|------|---------|-----|----------|---------|
| フィルター検索 | GET | /alerts/history?device_id=...&from=...&to=... | #history-table | submit |
| ページネーション | GET | /alerts/history?page=2 | #history-table | click |

---

## id属性一覧

HTMLモックをBladeに変換する際に付与するid属性。HTMX部分更新のターゲットとして使用する。

> ※ デザイナーのHTMLモックにはid属性は含まれていない。Blade変換時にこちらで付与する。

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

## HTMX部分更新仕様まとめ（横断リファレンス）

> ※ このセクションは画面別実装仕様の HTMX 情報を横断的に一覧化したリファレンスである。
> **正（権威あるソース）は各画面の「HTMX操作」テーブル。** 矛盾がある場合は画面別仕様を優先する。
> Controller やルーティングの実装時に、全エンドポイントを俯瞰する目的で使用する。

バックエンドが返す部分HTMLレスポンスの一覧。
各レスポンスはHTMLフラグメント（`<html>` や `<body>` を含まない部分HTML）を返す。

### 複数ターゲット同時更新（hx-swap-oob）

1つのリクエストで複数の要素を更新する場合、HTMXの **Out of Band Swap（hx-swap-oob）** を使用する。

**仕組み:** メインターゲットのHTMLに加えて、`hx-swap-oob="true"` 属性を持つ要素をレスポンスに含めると、その要素は対応するidの位置で自動的に差し替わる。

**Blade テンプレート構造の具体例（センサーデータ履歴: readings/index.blade.php）:**

```blade
{{-- メイン Fragment: #readings-table の innerHTML として差し込まれる --}}
<div id="readings-table">
    @fragment('readings-table')
    <table>
        @foreach ($readings as $reading)
        <tr>
            <td>{{ $reading->recorded_at->format('Y-m-d H:i') }}</td>
            <td>{{ $reading->temperature }}</td>
            <td>{{ $reading->humidity }}</td>
        </tr>
        @endforeach
    </table>
    {{ $readings->links() }}
    @endfragment
</div>

{{-- OOB Fragment: #readings-summary を要素ごと差し替え --}}
@fragment('readings-summary')
<div id="readings-summary" hx-swap-oob="true">
    <p>平均温度: {{ $summary->avg_temperature }}℃</p>
    <p>最高温度: {{ $summary->max_temperature }}℃</p>
    {{-- ... --}}
</div>
@endfragment
```

**Controller:**

```php
// ReadingController::index()
if ($request->header('HX-Request')) {
    return view('readings.index', compact('readings', 'summary'))
        ->fragments(['readings-table', 'readings-summary']);
}
return view('readings.index', compact('readings', 'summary', 'device'));
```

> ※ メイン Fragment はターゲット要素の **内側** に配置し中身だけ返す。OOB Fragment は `id` + `hx-swap-oob` 付きの要素 **全体** を囲む。

**対象エンドポイント:**

| エンドポイント | メイン Fragment | OOB Fragment |
|-------------|--------------|-------------|
| `/devices/{id}/readings` (フィルター検索) | readings-table → #readings-table | readings-summary → #readings-summary |
| `/alerts/rules?device_id={id}` (デバイス切替) | rules-list → #rules-list | rule-form → #rule-form |
| `/alerts/rules/{id}` PUT (ルール更新) | rules-list → #rules-list | rule-form → #rule-form |

### 部分HTMLエンドポイント一覧

| エンドポイント | メソッド | 返却内容 | Fragment | ターゲットid |
|-------------|---------|---------|----------|------------|
| `/dashboard/devices` | GET | デバイスカード一覧 | device-cards | #device-cards |
| `/dashboard/alerts` | GET | 未対応アラートバナー | alert-banner | #alert-banner |
| `/devices/{id}/chart/temperature` | GET | 温度SVGグラフ | temperature-chart | #temperature-chart |
| `/devices/{id}/chart/humidity` | GET | 湿度SVGグラフ | humidity-chart | #humidity-chart |
| `/devices/{id}/latest-readings` | GET | 最新計測データテーブル | latest-readings | #latest-readings |
| `/devices/{id}/readings` | GET | センサーデータテーブル + 集計 | readings-table, readings-summary (OOB) | #readings-table, #readings-summary |
| `/alerts/rules` | GET | ルール一覧 + フォーム（デバイス切替時） | rules-list, rule-form (OOB) | #rules-list, #rule-form |
| `/alerts/rules` | POST | ルール一覧（追加後） | rules-list | #rules-list |
| `/alerts/rules/{id}/edit` | GET | 編集フォーム（既存値ロード済み） | rule-form | #rule-form |
| `/alerts/rules/{id}` | PUT | ルール一覧 + フォームリセット | rules-list, rule-form (OOB) | #rules-list, #rule-form |
| `/alerts/rules/{id}/toggle` | PATCH | ルール一覧（切替後） | rules-list | #rules-list |
| `/alerts/rules/{id}` | DELETE | ルール一覧（削除後） | rules-list | #rules-list |
| `/alerts/history` | GET | アラート履歴テーブル | history-table | #history-table |

### 自動更新（ポーリング）

| 画面 | エンドポイント | 間隔 | 備考 |
|------|-------------|------|------|
| ダッシュボード | `/dashboard/devices` | 60秒 | hx-trigger="every 60s" |
| ダッシュボード | `/dashboard/alerts` | 60秒 | hx-trigger="every 60s" |
| デバイス詳細 | `/devices/{id}/latest-readings` | 60秒 | hx-trigger="every 60s" |

> ※ ポーリング間隔は運用要件に応じて調整可能。必須ではなく手動リロードでも可。

---

## Bladeファイル一覧

### フルページテンプレート

HTMLモックから変換するテンプレート。各画面のメインビュー。

| ファイルパス | 画面 | HTMLモック対応 |
|------------|------|-------------|
| `layouts/app.blade.php` | 共通レイアウト（認証済み） | 各HTML共通部分 |
| `layouts/guest.blade.php` | ゲストレイアウト（未認証） | login.html / register.html 共通部分 |
| `auth/login.blade.php` | ログイン | login.html |
| `auth/register.blade.php` | ユーザー登録 | register.html |
| `dashboard.blade.php` | ダッシュボード | dashboard.html |
| `devices/show.blade.php` | デバイス詳細 | device-show.html |
| `devices/create.blade.php` | デバイス登録 | device-create.html |
| `devices/edit.blade.php` | デバイス編集 | device-edit.html |
| `readings/index.blade.php` | センサーデータ履歴 | readings.html |
| `alerts/rules.blade.php` | アラートルール管理 | alert-rules.html |
| `alerts/history.blade.php` | アラート履歴 | alert-history.html |

### Fragment 定義一覧（HTMX部分更新用）

フルページテンプレート内に `@fragment` で定義する部分更新領域。部分テンプレートファイルは作成しない。

| 所属テンプレート | Fragment 名 | 用途 | パターン |
|---------------|------------|------|---------|
| `dashboard.blade.php` | device-cards | デバイスカード一覧 | メイン |
| `dashboard.blade.php` | alert-banner | 未対応アラートバナー | メイン |
| `devices/show.blade.php` | temperature-chart | 温度SVGグラフ | メイン |
| `devices/show.blade.php` | humidity-chart | 湿度SVGグラフ | メイン |
| `devices/show.blade.php` | latest-readings | 最新計測データテーブル | メイン |
| `readings/index.blade.php` | readings-table | センサーデータテーブル | メイン |
| `readings/index.blade.php` | readings-summary | 集計情報 | OOB |
| `alerts/rules.blade.php` | rules-list | ルール一覧テーブル | メイン |
| `alerts/rules.blade.php` | rule-form | ルール追加/編集フォーム | OOB |
| `alerts/history.blade.php` | history-table | アラート履歴テーブル | メイン |

> ※ メイン: ターゲット要素の内側に配置（innerHTML swap）。OOB: `id` + `hx-swap-oob="true"` 付きの要素全体を囲む（outerHTML swap）。

---

更新日時: 2026-02-20
