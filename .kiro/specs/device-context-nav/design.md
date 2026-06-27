# Technical Design — device-context-nav

## Overview

**Purpose**: 認証後レイアウトの左サイドバーを、現在表示中の画面の文脈に応じて変わる動的ナビゲーションへ拡張する。デバイスを見ている間だけ「📟 デバイス詳細」「📈 センサーデータ履歴」の文脈リンクを出し、選択中デバイスの詳細↔履歴をサイドバーだけで往復できるようにする。あわせて全メニュー項目の active ハイライトを現在ページ連動で動的化し、これまで「ダッシュボード固定」だった active を是正する（既存 `Sidebar.templ` が予告していた動的化の回収）。

**Users**: 自分のデバイスを運用する利用者（眞境名さん／テストユーザー）が、全7認証後画面で利用する。

**Impact**: 引数なし固定 HTML の `component.Sidebar()` を、ナビ文脈（現在ページ識別子＋選択中デバイス ID）を受け取る形へ変える。共有レイアウトデータ `layout.AppLayoutData` に1フィールドを追加し、App を描画する7ハンドラが自画面のナビ文脈を埋める。新規ルート・新規クエリ・DB スキーマ・CSS の変更は伴わない。

### Goals
- デバイス詳細・センサーデータ履歴の2画面でのみ文脈リンクを表示し、リンク先を「いま見ているデバイス」に向ける（1.x）。
- 全メニュー項目の active を現在ページに連動させ、同時 active を1つ以下に保つ（2.x）。
- 選択中デバイスの詳細↔履歴をサイドバーから相互往復できるようにする（3.x）。
- 常時表示3項目・画面内 HTMX 部分更新・モバイルドロワーを無回帰維持する（4.x）。

### Non-Goals
- デバイス登録・デバイス編集画面をデバイス文脈に入れること（編集は device id を持つが文脈リンクは出さない＝確定済みのユーザー判断）。
- 各画面のコンテンツ本体（グラフ・表・フォーム）の変更。
- 新規ルート・新規クエリ・DB スキーマ変更・サイドバー用独自 CSS クラスの新設。
- ヘッダー・フラッシュ通知・既存 HTMX 部分更新の振る舞い変更。

## Boundary Commitments

### This Spec Owns
- `component.Sidebar` の描画契約（新シグネチャ）と、ナビ文脈型 `component.SidebarNav`（現在ページ識別子＋選択中デバイス ID）。
- `layout.AppLayoutData` への `Nav` フィールド追加と、App→Sidebar への受け渡し（`App.templ`）。
- App を描画する7ハンドラがナビ文脈を埋める責務。
- モック正本 `mocks/html/{device-show,readings,device-create,device-edit}.html` のサイドバー状態の更新（active 位置・文脈2リンク）。

### Out of Boundary
- デバイス登録・編集のデバイス文脈化（明示的に文脈へ入れない）。
- 各画面のコンテンツ本体・既存ルート・クエリ・DB スキーマ・CSS（`.sidebar`/`.active` は流用のみ・変更しない）。
- 所有者認可ロジック本体（`internal/authz` 所有・消費のみ）。
- 画面内 HTMX 部分更新の対象（`#device-chart-area`/`#device-readings-list`/`#alert-history-list`/インライン CRUD）の挙動。

### Allowed Dependencies
- 既存ルート `/devices/{id}`・`/devices/{id}/readings`・`/dashboard`・`/alerts/rules`・`/alerts/history`（リンク先として消費）。
- 既存 `layout.AppLayoutData` / `layout.App` / `component.Sidebar` / 既存 CSS `.sidebar li a.active`。
- device-show（`DeviceHandler.Show`）・readings（`ReadingsHandler.Index`）が URL から取得済みの `id int64`。
- **依存方向の制約**: `layout` → `component` のみ（既存）。`component` は `layout` を import しない（循環回避・`component/views.go:275`）。よってナビ文脈型は `component` 側に置く。

### Revalidation Triggers
- `component.SidebarNav` のフィールド構成・`NavPage` 定数の変更（全7ハンドラの設定箇所に影響）。
- `component.Sidebar` のシグネチャ変更（App.templ・既存 Sidebar テストに影響）。
- App を描画する画面の追加／削除（新画面はナビ文脈の設定が必要）。
- 文脈リンク先ルート（`/devices/{id}`・`/devices/{id}/readings`）のパス変更。

## Architecture

### Existing Architecture Analysis
- view 3層（layout / page / component）。認証後画面は全て `page.XxxPage` が `@layout.App(v.Layout){...}` を呼び、`App.templ` が `@component.SiteHeader(...)` と `@component.Sidebar()` を描画する。`App.templ:48` がサイドバー描画の唯一の合流点。
- `layout.AppLayoutData` は認証後全画面の共通レイアウトデータ（Title/UserName/CSRFToken/CSSURL/Flash）。共通ビルダは無く、各ハンドラが個別構築する。
- **import 方向**: `layout` が `component` を import する（App が Sidebar/SiteHeader を呼ぶ）。逆向き（component→layout）は循環のため禁止。
- 画面内 HTMX 部分更新は `renderComponent` で**フラグメントのみ**返し、App レイアウト（サイドバー）を含めない。→ R4.3 は現状構造で充足し、本機能は「フラグメントにサイドバーを足さない」不変条件を守るだけでよい。
- active 表現は既存 CSS `.sidebar li a.active`（style.css:186-190）で完結。新規クラス不要。

### Architecture Pattern & Boundary Map

```mermaid
graph LR
    Handlers[App描画7ハンドラ] -->|Nav SidebarNav 設定| AppLayoutData[layout AppLayoutData]
    AppLayoutData -->|v.Layout| PageTempl[page XxxPage]
    PageTempl -->|@layout.App data| AppTempl[layout App]
    AppTempl -->|@component.Sidebar data.Nav| Sidebar[component Sidebar]
    Sidebar -->|DeviceID gt 0 で文脈リンク / Current で active| HTML[認証後サイドバーHTML]
```

**Architecture Integration**:
- Selected pattern: research §3 Option A（`AppLayoutData` 拡張＋各ハンドラ設定）。既存 seam（AppLayoutData→App→Sidebar）に最小侵襲で乗る。middleware パス駆動（B）はルート分岐の脆さと境界の暗黙化により不採用。
- Domain/feature boundaries: ナビ文脈型は `component` が所有（import 方向制約）。`layout` は文脈を保持・受け渡すのみ。各ハンドラは自画面の文脈値を設定するのみ。
- Existing patterns preserved: typed パラメータ struct で templ コンポーネントへ受け渡し（design-principles「共通パラメータ struct」）／view は handler が渡す表示データのみ描画（structure.md ルール④）／active は class 表現・`id` はスタイル非使用。
- New components rationale: `component.SidebarNav`＝循環なしで layout↔component 間を橋渡しする唯一の置き場。
- Steering compliance: 実務的 Layered-lite・CSS 単一ソース（変更なし）・モック写経（§31）・HTMX フラグメントにレイアウト非混入。

### Technology Stack

| Layer | Choice / Version | Role in Feature | Notes |
|-------|------------------|-----------------|-------|
| Frontend | templ v0.3 + HTMX + Alpine.js | Sidebar の条件描画（`if`）と active（`templ.KV`）、ドロワー開閉（Alpine `navOpen` 既存） | 新規ライブラリなし |
| Backend | Go 1.26 + Gin v1.12 | 7ハンドラが `AppLayoutData.Nav` を設定 | 新規ルート・依存なし |
| Data / Storage | （変更なし） | DB アクセスなし（既存 device id を URL から消費） | migration 不要 |
| CSS | 素のモダン CSS（既存） | `.sidebar li a.active` 流用 | `mocks/html/style.css` 変更なし＝`make sync-css` 不要 |

## File Structure Plan

### New Files
```
internal/view/component/
└── sidebar.go        # ナビ文脈型: NavPage(string)+定数, SidebarNav struct。Sidebar.templ と同パッケージ
```

### Directory Structure（変更が及ぶ範囲のみ）
```
internal/
├── view/
│   ├── layout/App.templ            # AppLayoutData に Nav 追加 / @component.Sidebar(data.Nav)
│   └── component/
│       ├── sidebar.go              # (新規) NavPage + SidebarNav
│       └── Sidebar.templ           # 新シグネチャ Sidebar(nav SidebarNav)・条件描画・動的 active
└── handler/                        # 7ハンドラが AppLayoutData.Nav を設定
mocks/html/                         # サイドバー状態の更新（4枚）+ 確認（3枚）
```

### Modified Files
- `internal/view/component/Sidebar.templ` — `Sidebar()` → `Sidebar(nav SidebarNav)`。文脈リンク（`if nav.DeviceID > 0`）を dashboard と alert-rules の間へ挿入、各項目の active を `templ.KV("active", nav.Current == 該当NavPage)` で付与、Alpine `:class="{ 'is-open': navOpen }"` 保持。`fmt`/`templ.SafeURL` で `/devices/{id}`・`/devices/{id}/readings` を整形。
- `internal/view/layout/App.templ` — `AppLayoutData` に `Nav component.SidebarNav` 追加、`@component.Sidebar()` → `@component.Sidebar(data.Nav)`。
- `internal/handler/dashboard.go` — `buildDashboardView` 呼び出しの `AppLayoutData` に `Nav: component.SidebarNav{Current: component.NavDashboard}`。
- `internal/handler/device_show.go:135` — `Nav: component.SidebarNav{Current: component.NavDeviceShow, DeviceID: id}`。
- `internal/handler/readings.go:112` — `Nav: component.SidebarNav{Current: component.NavReadings, DeviceID: id}`。
- `internal/handler/device.go` — `buildCreateView`/`buildEditView` の `AppLayoutData` に `Nav: component.SidebarNav{}`（ゼロ値＝active なし・文脈なし）。**edit は id を持つが DeviceID を設定しない**旨をコメント明示。
- `internal/handler/alert_rule.go:307` — `layoutData` の `AppLayoutData` に `Nav: component.SidebarNav{Current: component.NavAlertRules}`。
- `internal/handler/alert_history.go:224` — `Nav: component.SidebarNav{Current: component.NavAlertHistory}`。
- `mocks/html/device-show.html` — サイドバーに文脈2リンク（`/devices/1`・`/devices/1/readings`）追加、active を「📟 デバイス詳細」へ。
- `mocks/html/readings.html` — 文脈2リンク追加、active を「📈 センサーデータ履歴」へ。
- `mocks/html/device-create.html` / `mocks/html/device-edit.html` — `/dashboard` の `class="active"` を除去（どれも非 active）。
- `mocks/html/dashboard.html` / `alert-rules.html` / `alert-history.html` — 既に正（active 位置一致）。templ と一致するか確認のみ。
- 生成物再生成: `make templ`（App_templ.go / Sidebar_templ.go）。CSS 変更なしのため `make sync-css` 不要。

## Requirements Traceability

| Requirement | Summary | Components | Interfaces | Flows |
|-------------|---------|------------|------------|-------|
| 1.1, 1.2 | 詳細/履歴で文脈2リンク表示 | Sidebar, SidebarNav | `Sidebar(nav)` の `if nav.DeviceID > 0` | Handler→AppLayoutData.Nav→App→Sidebar |
| 1.3 | 文脈外画面で文脈リンク非表示 | Sidebar, 各ハンドラ | `DeviceID == 0`（dashboard/create/edit/alerts） | 同上（DeviceID 未設定） |
| 1.4 | リンク先は現在表示中デバイス | Sidebar | `fmt.Sprintf("/devices/%d…", nav.DeviceID)` | device_show/readings が id を Nav に設定 |
| 1.5 | 認可済みデバイスのみ参照 | device_show/readings ハンドラ | URL の id（認可後）を Nav.DeviceID へ | 新規データアクセスなし |
| 2.1–2.5 | 現在ページに連動した active | Sidebar, 7ハンドラ | `templ.KV("active", nav.Current == NavXxx)` | 各ハンドラが Current を設定 |
| 2.6 | create/edit は active なし | Sidebar, device.go | `nav.Current == ""`（ゼロ値） | buildCreate/EditView が空 Nav |
| 2.7 | 同時 active ≤1 | SidebarNav | `Current` 単一フィールド | 構造的に保証 |
| 3.1, 3.2 | 詳細↔履歴 相互往復 | Sidebar | 文脈リンク href（同一 id） | フルページ遷移で再描画 |
| 3.3 | 往復中も同一デバイス | Sidebar | 両リンクが同じ `nav.DeviceID` | — |
| 4.1, 4.2 | 常時3項目維持・遷移先不変 | Sidebar | 固定 href（dashboard/alerts） | — |
| 4.3 | HTMX 部分更新で不変 | 部分更新ハンドラ群 | `renderComponent` はフラグメントのみ（サイドバー非含） | 不変条件（確認テスト） |
| 4.4 | モバイルドロワー維持 | Sidebar | `:class="{ 'is-open': navOpen }"` 保持 | Alpine 既存 |
| 4.5 | 本体/ルート/スキーマ不変 | — | 変更なし | — |

## Components and Interfaces

| Component | Domain/Layer | Intent | Req Coverage | Key Dependencies (P0/P1) | Contracts |
|-----------|--------------|--------|--------------|--------------------------|-----------|
| `SidebarNav` / `NavPage` | View (component) | ナビ文脈の typed 値（現在ページ＋選択中デバイス ID） | 1, 2, 3 | なし | State |
| `Sidebar` | View (component/templ) | ナビ文脈に応じてサイドバーを描画（文脈リンク・active・ドロワー） | 1, 2, 3, 4 | SidebarNav (P0) | View/Template |
| `AppLayoutData.Nav` | View (layout) | App→Sidebar へナビ文脈を受け渡す共通フィールド | 1, 2, 3, 4 | component.SidebarNav (P0) | State |
| App描画7ハンドラ | Backend (handler) | 各自の画面の `Nav` を AppLayoutData に設定 | 1, 2 | SidebarNav (P0) | View/Template |

### View (component)

#### SidebarNav / NavPage

| Field | Detail |
|-------|--------|
| Intent | サイドバーの現在ナビ文脈を型安全に表す値オブジェクト |
| Requirements | 1.1, 1.3, 1.4, 2.1, 2.6, 2.7, 3.3 |

**Responsibilities & Constraints**
- `Current NavPage`：現在ページ識別子。ゼロ値 `""` は「どの項目も active にしない」（create/edit 用・R2.6）。単一フィールドゆえ同時 active ≤1（R2.7）を構造保証。
- `DeviceID int64`：選択中デバイス ID。`> 0` のとき文脈リンクを表示（R1.1/1.2）、`0` で非表示（R1.3）。既存 `DeviceShowView.DeviceID int64` と同型・0-sentinel。
- `component` パッケージに定義（import 方向制約のため layout には置けない）。

**Contracts**: View/Template [ ] / Service [ ] / API (JSON) [ ] / Event [ ] / Batch [ ] / State [x]

##### State Management
- State model（typed 値・不変）:
```go
package component

// NavPage は認証後サイドバーの「現在表示中の画面」を表す識別子。
// ゼロ値 "" はどのメニュー項目も active にしない (デバイス登録/編集など対応項目なし)。
type NavPage string

const (
    NavDashboard    NavPage = "dashboard"
    NavDeviceShow   NavPage = "device-show"
    NavReadings     NavPage = "readings"
    NavAlertRules   NavPage = "alert-rules"
    NavAlertHistory NavPage = "alert-history"
)

// SidebarNav はサイドバー描画に必要なナビ文脈。
// Current=active 判定、DeviceID>0 でデバイス文脈リンク (詳細/履歴) を表示する。
type SidebarNav struct {
    Current  NavPage // 現在ページ。"" はどれも非 active
    DeviceID int64   // >0 で文脈リンク表示・リンク先 ID。0 は文脈なし
}
```
- Persistence & consistency: 永続化なし。リクエストごとにハンドラが構築し描画時に消費。
- Concurrency strategy: 値型・共有なし。

#### Sidebar（templ）

| Field | Detail |
|-------|--------|
| Intent | ナビ文脈に応じて常時3項目＋文脈2リンクを描画し、active とドロワーを制御 |
| Requirements | 1.1, 1.2, 1.3, 1.4, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 3.1, 3.2, 3.3, 4.1, 4.2, 4.4 |

**Responsibilities & Constraints**
- メニュー順: 🏠 ダッシュボード（常時）→ 📟 デバイス詳細（文脈）→ 📈 センサーデータ履歴（文脈）→ 🔔 アラートルール（常時）→ 🕐 アラート履歴（常時）。
- 文脈2リンクは `if nav.DeviceID > 0` のブロックで dashboard と alert-rules の間に描画（R1.1/1.2/1.3）。href は `nav.DeviceID` から整形（R1.4・同一 id ＝ R3.3）。
- 各 `<a>` の active は `templ.KV("active", nav.Current == 該当NavPage)` で付与（R2.1–2.6）。
- `<aside class="sidebar" :class="{ 'is-open': navOpen }">` を保持（R4.4）。独自クラス新設なし（既存 `.sidebar`/`.active`/`nav li` 流用）。

**Dependencies**
- Inbound: `layout.App` — `@component.Sidebar(data.Nav)` で描画（P0）
- Outbound: `component.SidebarNav` — 受け取るナビ文脈（P0）
- External: なし

**Contracts**: View/Template [x] / Service [ ] / API (JSON) [ ] / Event [ ] / Batch [ ] / State [ ]

##### View / Template Contract
| Trigger | Method | Path | 認証 | 返却モード | 返却 templ コンポーネント | 入力 | エラー時 |
|---------|--------|------|------|-----------|--------------------------|------|----------|
| 認証後全画面の描画に内包 | GET | /dashboard, /devices/:device, /devices/:device/readings, /devices/create, /devices/:device/edit, /alerts/rules, /alerts/history | session | full page（App レイアウト内） | `Sidebar(nav)`（`@layout.App` 経由） | `SidebarNav` | — |

- 返却モード: サイドバーは **full page（App レイアウト）の一部**のみ。**HTMX partial に Sidebar を含めない**（R4.3）。OOB 更新なし。
- HTMX トリガ: なし（サイドバーリンクは素の `<a href>`＝フルページ遷移）。
- CSRF: 該当なし（GET 遷移のみ）。
- 参照: `2cc_sdd/HTMX実装ガイド(動的).md` §31（モッククラス流用）/ §40-B（CSS 単一ソース）。

**Implementation Notes**
- Integration: `App.templ` の `AppLayoutData` に `Nav component.SidebarNav` を追加し `@component.Sidebar(data.Nav)` へ。既存 `TestSidebar_...`（component_test.go:39）を新シグネチャへ更新。
- Validation: 入力検証なし（型で制約）。ゼロ値は安全縮退。
- Risks: 文脈リンクを HTMX フラグメントに誤混入させない（不変条件・テストで固定）。

### Backend (handler)

#### App描画7ハンドラのナビ文脈設定

| Field | Detail |
|-------|--------|
| Intent | 各画面のハンドラが自画面に対応する `SidebarNav` を `AppLayoutData.Nav` に設定 |
| Requirements | 1.3, 1.4, 1.5, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6 |

**Responsibilities & Constraints**
- dashboard→`{Current: NavDashboard}` / device-show→`{Current: NavDeviceShow, DeviceID: id}` / readings→`{Current: NavReadings, DeviceID: id}` / create・edit→`{}`（ゼロ値） / alert-rules→`{Current: NavAlertRules}` / alert-history→`{Current: NavAlertHistory}`。
- device-show・readings の `id` は所有者認可後に確定済みのもの（R1.5・新規データアクセスなし）。
- edit は `id` を保持するが `DeviceID` を**設定しない**（R1.3 boundary・コメント明示）。

**Contracts**: View/Template [x] / Service [ ] / API (JSON) [ ] / Event [ ] / Batch [ ] / State [ ]

**Implementation Notes**
- Integration: 既存の `AppLayoutData{...}` リテラルに1行追加するのみ。返却 templ・ルート・クエリは不変。
- Validation: なし。
- Risks: 設定漏れはゼロ値縮退（active/文脈なし）で無害。主要画面はテストで固定。

## Data Models

本機能は **DB スキーマ・クエリを変更しない**（`docs/database_snapshot/` 参照不要）。新規データは描画用 ViewModel のみ:
- `layout.AppLayoutData` に `Nav component.SidebarNav` を追加（表示用）。
- `component.SidebarNav{Current NavPage; DeviceID int64}`（表示用・永続化なし）。

既存 device id を URL から消費するのみで、論理データモデル・参照整合性に影響なし。

## Error Handling

- 新規エラー経路なし。ナビ文脈はゼロ値が安全縮退（active なし・文脈リンクなし）するため、未設定でも 5xx/例外を生まない（fail-safe・design-principles 6 Graceful Degradation）。
- 文脈リンク先（既存ルート）への遷移時のエラー（404 等）は既存ハンドラ・既存グローバルトーストの責務で、本機能は所有しない。

## Testing Strategy

> `2cc_sdd/テストガイダンス集.md` の Go 定石に沿う（templ は `Render`→`strings.Contains`、ハンドラは `httptest`+gin、`Querier` 手書きモックで DB 非依存、gorilla/csrf 往復、scs in-memory、列挙防止）。

### Unit Tests（templ Render → strings.Contains・`internal/view/component`）
1. `Sidebar(SidebarNav{Current: NavDashboard})`：`/dashboard` に `class="active"`、文脈リンク（`📟 デバイス詳細`/`📈 センサーデータ履歴`/`/devices/`）を **含まない**、`:class="{ 'is-open': navOpen }"` を含む（2.1, 1.3, 4.4）。
2. `Sidebar(SidebarNav{Current: NavDeviceShow, DeviceID: 42})`：`href="/devices/42"` の 📟 が `class="active"`、`href="/devices/42/readings"` の 📈 が非 active で存在、dashboard 非 active（1.1, 1.4, 2.2, 3.3）。
3. `Sidebar(SidebarNav{Current: NavReadings, DeviceID: 42})`：📈 が active、📟 が非 active で存在、両 href が `/devices/42…`（1.2, 1.4, 2.3, 3.1, 3.2）。
4. `Sidebar(SidebarNav{})`（create/edit 相当）：`class="active"` を **含まない**、文脈リンクを **含まない**、常時3項目は存在（2.6, 1.3, 4.1）。
5. `Sidebar(SidebarNav{Current: NavAlertRules})` / `{Current: NavAlertHistory}`：それぞれ該当項目のみ active（2.4, 2.5）。
6. 同時 active 不変条件：`strings.Count(html, \`class="active"\`)` が active あり画面で 1、create/edit 相当で 0（2.7）。
7. 既存 `TestSidebar_ナビゲーションリンクを描画`（component_test.go:39）を新シグネチャへ更新し、常時3項目 href の存在を維持（4.1, 4.2）。

### Integration Tests（httptest + Querier モック・`internal/handler`）
1. `DeviceHandler.Show`（非 HX, フルページ）：応答 HTML に 📟/📈 文脈リンクが**要求 device id** 付きで含まれ、device-show が active（1.1, 1.4, 2.2, 3.x）。
2. `ReadingsHandler.Index`（非 HX, フルページ）：文脈2リンク＋readings active（1.2, 2.3）。
3. `ReadingsHandler.Index`（`HX-Request` あり）：応答が `DeviceReadingsList` フラグメントのみで、**サイドバー markup（`class="sidebar"`）を含まない**（4.3）。
4. dashboard / alert-rules / alert-history：文脈リンクを含まず、各 active が正（1.3, 2.1, 2.4, 2.5）。
5. `DeviceHandler.ShowEditForm`（edit, フルページ）：URL に device id があっても文脈リンクを**含まない**、active なし（1.3 boundary, 2.6）。

### Coverage
- 対象は Sidebar templ（条件分岐・active）と7ハンドラの Nav 設定。小さな表面積で 80%+ を満たす設計（既存ハンドラテストへ assertion 追加＋Sidebar 専用テスト新設）。

## Security Considerations
- 新規認可経路なし。文脈リンクは **現在表示中＝既に所有者認可済みのデバイス**（device-show/readings は描画前に `authz` で認可）への ID をそのまま使うのみで、他デバイスへの導線・新規データアクセスを生まない（1.5）。BOLA 面の新たな攻撃面は増えない。
