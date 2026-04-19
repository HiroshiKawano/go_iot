# HTMX実装ガイド(動的)

---

## cc-sdd参照ガイド

本設計書をcc-sdd（詳細設計書）から参照する際に価値の高いセクションと用途を示す。

| 優先度 | セクション | cc-sddでの用途 |
|:------:|-----------|---------------|
| ★★★ | [Blade Fragmentアプローチ](#blade-fragmentアプローチ) | Controller の `->fragment()` / `->fragments()` 返却パターンの実装根拠。部分ビューファイル分離方式との違い。誤ったパターンを選ぶとBladeファイル数・Controller構造・テストがすべて変わる |
| ★★★ | [Fragment命名規約](#fragment-命名規約) | Fragment名・`hx-target` id・Controller指定名を統一するルール。名前不一致による実装ミスを防ぐ唯一の根拠 |
| ★★★ | [id属性一覧](#2-id-属性一覧) | Controller（Fragment指定）・Blade（Fragment定義）・HTMX属性（hx-target指定）の唯一の定義元。HTMLモックにはidが含まれないためこの一覧が必須 |
| ★★★ | [画面別HTMX操作仕様](#3-画面別-htmx-操作仕様) | 各画面のトリガー・URL・ターゲットのプロジェクト固有仕様。ルーティング・Controller実装設計の根拠 |
| ★★★ | [バリデーションエラー表示方針](#6-バリデーションエラー表示方針) | HTMXフォームは手動バリデーション+422+Fragment返却、通常フォームはリダイレクト+`$errors` の使い分け根拠。FormRequest不使用の理由。422のswap設定方式。誤ると全フォームのエラー表示が機能しない |
| ★★★ | [CSRF対応方針](#7-csrf対応方針) | グローバルmeta tag + `htmx:configRequest` 方式（フォームごとの `@csrf` 不使用）の実装根拠。未設定だと全ミューテーションリクエストが403になる |
| ★★ | [OOB同時更新エンドポイント一覧](#4-oob-同時更新エンドポイント一覧) | `->fragments([])` を使うエンドポイントの定義。単一Fragment設計にすると更新されない要素が生じる |
| ★★ | [期間パラメータと集計ルール](#期間パラメータperiodと集計ルール) | 24h=生データ・7d/30d=日次集計の違い。DBクエリ・Controller実装・テスト内容に直接影響する |
| ★★ | [HX-Redirect使用方針](#8-hx-redirect-使用方針) | デバイス削除後のページ遷移実装根拠。`redirect()` と `HX-Redirect` の使い分け。HTMXリクエストに通常の `redirect()` は効かない |
| ★ | [自動更新（ポーリング）](#5-自動更新ポーリング) | 60秒ポーリング間隔の根拠。非機能要件・パフォーマンステスト設計に影響 |
| ★ | [ページネーションのHTMX統合方針](#9-ページネーションの-htmx-統合方針) | `hx-boost` + `hx-target` によるページネーション部分更新の実装パターン。カスタムPaginationビュー不使用の根拠 |
| ★ | [削除確認方針](#10-削除確認方針) | `hx-confirm` 使用方針の統一根拠。Alpine.jsモーダル不使用の根拠 |

> ※ cc-sddのController実装・Blade設計・ルーティングを記述する際は、まず本書のid属性一覧・画面別操作仕様・バリデーションエラー方針・CSRF対応を確認すること。

### 次回プロジェクトでの記載チェックリスト

HTMX実装ガイド(動的)を新規作成する際に以下が揃っているか確認する：

- [ ] 部分ビューファイルを分離するかBlade Fragmentを使うかを明記（Controller・Bladeファイル構成・テスト構造が変わる）
- [ ] Fragment名とhx-target idの対応ルール（命名規約）を記載
- [ ] 全画面のid属性一覧（Controller・Blade・HTMX属性の「共通語彙」）を記載。HTMLモックにはidが含まれないため必須
- [ ] 画面ごとのHTMX操作仕様（メソッド・URL・ターゲット・トリガー）を網羅
- [ ] 1リクエストで複数要素を更新するOOBエンドポイントを明記（`->fragments([])` 使用箇所）
- [ ] 期間パラメータ等、データ取得方式が変わるクエリパラメータとその集計ルールを記載
- [ ] HTMX未使用の画面・操作を明記（誤ってHTMXを適用させないため）
- [ ] 自動更新（ポーリング）の間隔と対象エンドポイントを記載
- [ ] HTMXフォームのバリデーションエラー返却方式（422+Fragment vs リダイレクト）を明記
- [ ] 422レスポンスのswap設定方式（`responseHandling` / `htmx:responseError`）を明記
- [ ] HTMXフォームのバリデーション方式（手動バリデーション vs FormRequest）を明記
- [ ] CSRF対応方式（グローバルmeta tag方式 vs フォームごと `@csrf` 方式）を明記
- [ ] `HX-Redirect` を使う操作とリダイレクト先を記載
- [ ] 削除操作の確認方法（`hx-confirm` / Alpine.jsモーダル等）を明記

---

## 1. HTMX 実装規約

> **cc-sdd への価値:**
> Blade には「部分ビューファイルを分離する」パターンと「フルページテンプレート内に Fragment を定義する」パターンの2種類がある。本プロジェクトは後者を採用しているが、どちらを選ぶかは設計上の判断であり、他のドキュメントに記述がない。誤ったパターンで設計すると、Controller の実装方式・Blade ファイル数・テストの構造がすべて変わるため、spec-design の段階で必ず参照する必要がある。

### Blade Fragment アプローチ

部分テンプレートファイルを分離せず、フルページテンプレート内に `@fragment` / `@endfragment` で部分更新領域を定義する。Controller は `->fragment()` で返却範囲を切り出す。

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

---

### Fragment 命名規約

> **cc-sdd への価値:**
> Fragment 名はターゲット id と同名にするプロジェクト規約がある。規約がないと cc-sdd が Controller・Blade・HTMX 属性でそれぞれ異なる名前を生成し、実装時に不整合が発生する。

**規則:** Fragment 名はターゲット id と同一にする。

| 例 | HTMX 属性 | Fragment 名 |
|---|----------|------------|
| デバイスカード | `hx-target="#device-cards"` | `@fragment('device-cards')` |
| アラートバナー | `hx-target="#alert-banner"` | `@fragment('alert-banner')` |

---

### Fragment の2つの配置パターン

> **cc-sdd への価値:**
> メイン Fragment と OOB Fragment では Blade 内での配置ルールが逆になる。間違えると部分更新時の返却範囲が変わるか、フルページ表示時に OOB 属性が誤動作する。どちらのパターンを使うかは id 属性一覧と OOB エンドポイント一覧（セクション 4）で定義している。

**パターン 1: メイン Fragment（innerHTML swap 用）**

ターゲット要素の **内側** に配置。中身だけが HTMX レスポンスとして返される。

```blade
<div id="device-cards">
    @fragment('device-cards')
    {{-- この範囲のみが HTMX レスポンスとして返される --}}
    @endfragment
</div>
```

**パターン 2: OOB Fragment（outerHTML swap 用）**

`id` と `hx-swap-oob="true"` を持つ要素 **全体** を囲む。

```blade
@fragment('readings-summary')
<div id="readings-summary" hx-swap-oob="true">
    {{-- 要素全体が OOB で差し替えられる --}}
</div>
@endfragment
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
> id 属性は Controller（Fragment 指定）・Blade（Fragment 定義）・HTMX 属性（hx-target 指定）の3箇所で同じ名前を参照する「共通語彙」になる。この一覧なしに設計すると、画面ごとに命名が揺れて実装時の統一が崩れる。なお、デザイナーのHTMLモックには id 属性が含まれていないため、この一覧が唯一の定義元となる。

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
> 24時間と7日・30日でデータ取得方式が異なる（生データ vs 日次集計）。この違いはDBクエリ・Controller 実装・テスト内容に直接影響するが、DB設計書.md には記載がない。

| 表示 | 値 | データ範囲 |
|------|-----|----------|
| 24時間 | 24h | 直近24時間の**生データ** |
| 7日間 | 7d | 直近7日間の**日次集計** |
| 30日間 | 30d | 直近30日間の**日次集計** |

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
> 1リクエストで複数要素を同時更新するエンドポイントでは、Controller で `->fragments([])` を使い、Blade に2種類の Fragment（メイン + OOB）を配置する必要がある。どのエンドポイントがこのパターンを使うかは設計上の判断であり、他のドキュメントから推測できない。誤ってシングル Fragment で設計すると、更新されない要素が生じてテストが通らなくなる。

| エンドポイント | メイン Fragment | OOB Fragment |
|-------------|--------------|-------------|
| `/devices/{id}/readings`（フィルター検索） | readings-table → #readings-table | readings-summary → #readings-summary |
| `/alerts/rules?device_id={id}`（デバイス切替） | rules-list → #rules-list | rule-form → #rule-form |
| `/alerts/rules` POST（ルール追加） | rules-list → #rules-list | rule-form → #rule-form |
| `/alerts/rules/{id}` PUT（ルール更新） | rules-list → #rules-list | rule-form → #rule-form |

**Blade テンプレート構造の具体例（readings/index.blade.php）:**

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
</div>
@endfragment
```

**Controller（ReadingController::index）:**

```php
if ($request->header('HX-Request')) {
    return view('readings.index', compact('readings', 'summary'))
        ->fragments(['readings-table', 'readings-summary']);
}
return view('readings.index', compact('readings', 'summary', 'device'));
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
> HTMXフォームでバリデーションエラー（422）が発生した場合の返却形式はプロジェクト固有の設計決定。他のドキュメントに記載がなく、誤ると全フォームのエラー表示が機能しない。

### フォーム種別による方針の違い

| フォーム種別 | エラー返却方式 |
|------------|-------------|
| **HTMXフォーム**（アラートルール追加・更新） | フォーム Fragment を **422ステータス** で返却 |
| **通常フォーム**（デバイス登録・編集） | Laravel標準: リダイレクト + `$errors`（セクション3と同じ） |

### HTMXフォームのエラー返却パターン

**採用方針:** Controller 内で手動バリデーション（`Validator` ファサード）を使用する。FormRequest は使用しない。

> FormRequest はバリデーション失敗時に Controller メソッドに到達する前に ValidationException を throw し、自動的にリダイレクトレスポンスを返す。HTMX リクエストに対して Fragment を返却するには、Controller 内でバリデーション結果を制御する必要がある。

```php
// Controller 内で手動バリデーション
public function store(Request $request)
{
    $validator = Validator::make($request->all(), [
        'metric' => 'required|in:temperature,humidity',
        'min_value' => 'nullable|numeric',
        'max_value' => 'nullable|numeric',
    ]);

    if ($validator->fails()) {
        if ($request->header('HX-Request')) {
            return view('alerts.rules.index', [
                'errors' => $validator->errors(),
                'old' => $request->all(),
                'device' => $device,
                'rules' => $rules,
            ])
                ->fragment('rule-form')
                ->withStatus(422);
        }
        return back()->withErrors($validator)->withInput();
    }

    // バリデーション成功後の処理...
}
```

### 422レスポンスのswap設定

**採用方針:** レイアウト Blade で `htmx.config.responseHandling` を設定し、422をスワップ対象に含める。

```html
<!-- layouts/app.blade.php の <body> 末尾（CSRF設定と同じ箇所） -->
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

> エラーメッセージは独立したid要素ではなく、**フォームFragment内にインラインで含める**（`#rule-form` の再返却でエラーを含んだフォームを差し替える）。

---

## 7. CSRF対応方針

> **cc-sdd への価値:**
> Laravel の CSRF 保護は POST / PUT / DELETE / PATCH すべてに適用される。HTMX でこれらを使う際の設定方法はプロジェクト固有の選択であり、未記録だと全ミューテーションリクエストが403エラーになる。

**採用方針:** レイアウト Blade に `<meta name="csrf-token">` を配置し、`htmx:configRequest` イベントでグローバルにヘッダーをセットする。**フォームごとに `@csrf` を書く方式は使用しない。**

```blade
{{-- layouts/app.blade.php の <head> 内 --}}
<meta name="csrf-token" content="{{ csrf_token() }}">
```

```html
<!-- <body> 末尾 -->
<script>
    document.addEventListener('htmx:configRequest', function(event) {
        event.detail.headers['X-CSRF-TOKEN'] =
            document.querySelector('meta[name="csrf-token"]').content;
    });
</script>
```

> Laravel の `VerifyCsrfToken` Middleware は `X-CSRF-TOKEN` ヘッダーを認識する。グローバル設定によりフォームに `@csrf` を書く必要はない。

---

## 8. `HX-Redirect` 使用方針

> **cc-sdd への価値:**
> HTMX リクエストに対して通常の `redirect()` は効かない（HTMX は HTML を swap するため）。`HX-Redirect` レスポンスヘッダーを使う必要があるが、どの操作で使うかはプロジェクト固有の設計決定。

```php
// Controller での HX-Redirect 返却例（デバイス削除後）
public function destroy(Request $request, Device $device): Response
{
    $device->delete();

    if ($request->header('HX-Request')) {
        return response()->noContent()
            ->withHeaders(['HX-Redirect' => route('dashboard')]);
    }
    return redirect()->route('dashboard');
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
> Laravel の `$paginator->links()` が生成するリンクはデフォルトでフルページ遷移になる。HTMX で部分更新するには追加の設定が必要。どの方式を採用するかはプロジェクト固有の設計決定。

**採用方針:** ページネーションコンテナに `hx-boost` + `hx-target` + `hx-swap` を設定する。カスタム Pagination ビューは作成しない。

```blade
{{-- readings/index.blade.php 内 --}}
<div id="readings-table">
    @fragment('readings-table')
    <table>...</table>

    <div hx-boost="true"
         hx-target="#readings-table"
         hx-swap="innerHTML">
        {{ $readings->links() }}
    </div>
    @endfragment
</div>
```

> `hx-boost` はコンテナ内のリンクを自動的に HTMX リクエストに変換する。`hx-target` を明示しないと `<body>` 全体が対象になるため必ず指定する。

**本プロジェクトでのページネーション使用箇所:**

| 画面 | ページネーション対象 | ターゲット id |
|------|-----------------|------------|
| センサーデータ履歴 | `$readings` | `#readings-table` |
| アラート履歴 | `$histories` | `#history-table` |

---

## 10. 削除確認方針

> **cc-sdd への価値:**
> 削除操作の確認方法はプロジェクト固有の設計決定。Alpine.jsモーダルか `hx-confirm` かで Blade テンプレートの構造と依存ライブラリが変わる。

**採用方針:** `hx-confirm` 属性（HTMX組み込み）を使用する。Alpine.js カスタムモーダルは使用しない。

```html
<button
  hx-delete="/devices/1"
  hx-confirm="「ハウスA温湿度計」を削除しますか？">
  削除
</button>
```

**ルール:**
- 確認メッセージは日本語とし、削除対象の名前を含める
- デバイス削除・アラートルール削除の両方に適用する

---

更新日時: 2026-02-25
