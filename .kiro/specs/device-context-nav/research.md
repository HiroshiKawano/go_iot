# Gap Analysis — device-context-nav

要件（`requirements.md`）と既存コードベースの乖離を分析し、design フェーズの実装戦略を示す。

## Analysis Summary

- 本機能は**新規層を作らない純拡張**。中核 seam は `App.templ:48` の唯一の `@component.Sidebar()` 呼び出しと、App が受け取る `layout.AppLayoutData`。ここにナビ文脈（現在ページ識別子＋選択中デバイス ID）を足し、`Sidebar` 引数へ流すのが最小侵襲。
- 変更の主な広がりは **`AppLayoutData` 構築の7箇所**（共通ビルダが無く各ハンドラが個別構築）と、**モック正本7ファイル**＋ Sidebar の templ/テスト。
- CSS は **追加不要**（`.sidebar li a.active`＝style.css:186-190 で active 表現が完結。要件「独自クラス新設なし」と整合）。
- device-show・readings は**既に URL から device id を取得済み**で、要件の「文脈設定元はこの2画面のみ」と実装現状が一致。device-edit も id を持つが要件上は文脈に**入れない**（フィールドを設定しないだけ）。
- 既存モックは device-show/readings/create/edit の4枚が **active=dashboard 固定の誤り**を抱えており（本機能の動的化で同時に是正）、デグレ修正としての価値も併せ持つ。

## 1. Current State Investigation

### 中核アセットと seam

| アセット | 所在 | 役割 / 本機能との関係 |
|---|---|---|
| `AppLayoutData` struct | `internal/view/layout/App.templ:10-16` | 認証後全画面の共通レイアウトデータ（Title/UserName/CSRFToken/CSSURL/Flash）。**ナビ文脈フィールドの追加先候補**。 |
| `App(data AppLayoutData)` | `App.templ:27` / `Sidebar()` 呼び出しは `:48` | サイドバー描画の**唯一の合流点**。`@component.Sidebar()` を引数なしで呼ぶ→ここを `Sidebar(navCtx)` に変える。 |
| `Sidebar()` | `internal/view/component/Sidebar.templ:6-16` | 引数なしの固定 HTML。active は `/dashboard` ハードコード（:10）。コメントに「active は後続セッションで動的化する想定」と明記＝本機能が回収。 |
| `.sidebar li a.active` 等 | `mocks/html/style.css:170-190`（モバイルは :682） | active 表現（背景＋左ボーダー primary＋太字600）。**新規クラス不要**。 |

### AppLayoutData を構築する7ハンドラ（変更波及先）

| 画面 | ハンドラ:行 | 構築方式 | device id |
|---|---|---|---|
| dashboard | `dashboard.go:84`（`buildDashboardView`） | ヘルパ経由 | なし |
| device-show | `device_show.go:135` | インライン | ✓ URL param 取得済み（**文脈設定元**） |
| readings | `readings.go:112` | インライン | ✓ URL param 取得済み（**文脈設定元**） |
| device-create | `device.go:287`（`buildCreateView`） | ヘルパ経由 | なし |
| device-edit | `device.go:315`（`buildEditView`） | ヘルパ経由 | ✓ 保持するが文脈に**入れない**（要件 Out of scope） |
| alert-rules | `alert_rule.go:307-308`（`layoutData`） | ヘルパ経由 | クエリで任意・文脈外 |
| alert-history | `alert_history.go:224` | インライン | クエリで任意フィルタ・文脈外 |

- **共通 AppLayoutData ビルダは存在しない**。各 page templ は `@layout.App(v.Layout){...}` 形式（全7 page templ で確認）で、ハンドラが自前 ViewModel の `Layout` フィールドを埋める。
- ナビ文脈フィールドを足すと、**設定し忘れた画面はゼロ値＝「active なし・文脈リンクなし」に縮退**する（fail-safe）。これは要件 R2.6（create/edit は active なし）と同じ無害な振る舞いで、デグレではない。

### 規約・依存方向

- structure.md: `view → repository/service 禁止`。Sidebar は handler が渡す表示データのみ描画＝本機能は規約内。
- `id` はスタイル用途禁止（HTMX 差し替え専用）。active は **class** で表現（既存どおり）。
- ナビ文脈は**画面間フルページ遷移**で毎描画されるため陳腐化しない。画面内 HTMX 部分更新（`#device-chart-area`/`#device-readings-list`/`#alert-history-list`/インラインCRUD）はサイドバーを swap 対象に含めない＝要件 R4.3 は現状構造で自動的に満たされる（確認のみ）。

### モック正本（7枚）の現状

全7枚が同一の3リンク固定サイドバー（dashboard/alert-rules/alert-history）。

| モック | 現状 active | 文脈リンク | 本機能での修正 |
|---|---|---|---|
| dashboard.html | dashboard ✓ | なし | 維持（active=dashboard） |
| device-show.html | dashboard ✗ | なし | active=デバイス詳細＋文脈2リンク追加 |
| readings.html | dashboard ✗ | なし | active=履歴＋文脈2リンク追加 |
| device-create.html | dashboard ✗ | なし | active 除去（どれも非 active） |
| device-edit.html | dashboard ✗ | なし | active 除去（どれも非 active） |
| alert-rules.html | alert-rules ✓ | なし | 維持 |
| alert-history.html | alert-history ✓ | なし | 維持 |

### 既存テスト

- `component/component_test.go:39` `TestSidebar_ナビゲーションリンクを描画`：href 3つ＋Alpine `:class` のみ検証、active 未検証。**Sidebar 引数追加でシグネチャ変更→このテストの呼び出し更新が必須**。
- `layout/layout_test.go`：App 全体テスト（Sidebar 個別なし）。
- 形式は全て `templ Render → strings.Contains`。ハンドラ側は httptest 形式（テストガイダンス集の定石）。

## 2. Requirement-to-Asset Map（ギャップ tag: Missing / Constraint / OK）

| 要件 | 対応アセット | ギャップ |
|---|---|---|
| R1 文脈リンク条件表示 | `Sidebar.templ`（条件描画追加）/ device id は device_show・readings が保持 | **Missing**: ナビ文脈フィールド＋条件 `if` 描画。device id→リンク URL 整形。 |
| R1.5 認可済みデバイスのみ | device id は「現在表示中＝認可済み」画面からのみ供給 | **OK/Constraint**: 新規データアクセス無し。文脈外画面は id を渡さない設計を死守。 |
| R2 active 動的化 | `Sidebar.templ` active 条件化／`.active` CSS は既存 | **Missing**: 現在ページ識別子の受領と active 付与ロジック。CSS は **OK**（流用）。 |
| R2.6 create/edit は active なし | ゼロ値縮退で自然達成 | **OK**: 文脈/識別子を設定しなければ非 active。 |
| R2.7 同時 active ≤1 | 識別子は単一値 | **Constraint**: 識別子を「単一の現在ページ」にする設計で保証。 |
| R3 詳細↔履歴往復 | 既存ルート `/devices/{id}`・`/devices/{id}/readings` | **OK**: 既存ルート消費のみ。リンク URL に同一 id を埋める。 |
| R4.1/4.2 常時項目維持 | 現 Sidebar の3リンク | **OK**: 構造維持。 |
| R4.3 HTMX 部分更新で不変 | swap 対象にサイドバー不在 | **OK**（確認のみ）。 |
| R4.4 モバイルドロワー維持 | Alpine `:class="{ 'is-open': navOpen }"`（Sidebar.templ:7 / CSS :682） | **Constraint**: 引数化後も `aside` の Alpine 属性を保持。 |
| R4.5 本体/ルート/スキーマ不変 | — | **OK**: 新規ルート/クエリ/migration なし。 |
| モック正本同期 | `mocks/html/*.html`（7枚）＋ `make sync-css` | **Constraint**（プロジェクト規約）: templ 変更を7モックへ写経、CSS 追記時のみ sync-css。 |

> DB スキーマ変更は**一切不要**（`docs/database_snapshot/` 参照不要レベル。既存 device id を URL から消費するのみ）。

## 3. Implementation Approach Options

### Option A: `AppLayoutData` 拡張＋各ハンドラ設定（推奨）
`AppLayoutData` に「現在ページ識別子」と「選択中デバイス ID（任意）」を追加し、`App.templ` が `Sidebar(navCtx)` へ流す。各ハンドラは自分の画面に対応する識別子を設定（device_show・readings のみ device id も設定）。

- ✅ 既存 seam（AppLayoutData→App→Sidebar）に素直に乗る。最小の新規概念。
- ✅ 設定し忘れ＝ゼロ値縮退で無害（R2.6 と同じ振る舞い）。
- ✅ テストは templ Render＋ハンドラ httptest で素直に書ける（既存定石）。
- ❌ 構築7箇所に触れる（共通ビルダが無いため）。
- ❌ 識別子の取り違え（コピペ）リスク→ 識別子を型安全な定数群（後述）にして緩和。

### Option B: ルートメタ駆動（middleware で文脈注入）
`RequireAuth` 群の後段に薄い middleware を置き、リクエストパスから現在ページ識別子と device id を導出して context に積み、App 描画時に読む。

- ✅ ハンドラ7箇所に触れない（集中管理）。
- ✅ 識別子の取り違えが起きにくい。
- ❌ パスマッチング（`/devices/:device` か `/devices/:device/readings` か `/devices/:device/edit` か等）の分岐ロジックが**新たな真実の所在**になり、ルート変更に追従が要る（脆い）。
- ❌ 「edit は文脈に入れない／show・readings のみ入れる」の境界判定が暗黙化し読みにくい。view 用データを context 経由で運ぶのは既存パターン外。
- ❌ AppLayoutData を経由しないため、テストでの値検証が回りくどい。

### Option C: Hybrid（識別子は各ハンドラ／device id 整形は共通ヘルパ）
Option A をベースに、文脈リンク URL の整形や AppLayoutData への文脈セットを小さな共通ヘルパ（例: `withDeviceNav(data, deviceID)` / `withNav(data, page)`）へ括り出す。

- ✅ A の素直さを保ちつつ、設定の重複と取り違えを縮小。
- ✅ 「文脈に入る画面」を呼び出し側で明示でき境界が読める。
- ❌ ヘルパの置き場所（view か handler か）を決める設計判断が1つ増える。

## 4. Effort & Risk

- **Effort: S（1–3日）** — 新規層・新規ルート・スキーマ・CSS いずれも不要。既存 seam の拡張＋7モック写経＋templ/handler テスト。
- **Risk: Low** — 確立パターンの拡張のみ。主リスクは(1)構築7箇所の設定漏れ＝ゼロ値で無害縮退、(2)モック↔templ↔テストの三者同期忘れ（プロジェクト規約のデグレ防止対象）。いずれも致命でない。

## 5. Recommendations for Design Phase

- **推奨アプローチ: Option A（必要に応じ C のヘルパを軽く併用）**。理由＝既存 AppLayoutData seam に最小侵襲で乗り、ゼロ値縮退が要件 R2.6 と一致し fail-safe。middleware 集中化（B）はルート分岐の脆さと境界の暗黙化が割に合わない小規模機能。
- **design で確定すべき判断（要件ではなく設計）**:
  1. ナビ文脈の表現＝`AppLayoutData` への (a) `CurrentPage` 識別子（文字列/列挙）＋ (b) `DeviceID`（任意・`*int64` か `0=なし`）の2フィールド追加 vs 専用 `NavContext` struct 埋め込み。
  2. **現在ページ識別子の定義場所と型**＝view 層の定数群（例 `nav.PageDashboard` …）。domain 純粋層には置かない（view 関心）。値の取り違えを型で防ぐ。
  3. 文脈リンク URL（`/devices/{id}`・`/devices/{id}/readings`）の整形責務＝Sidebar 内で id から組むか、ハンドラ/ヘルパで組んで渡すか。
  4. `Sidebar` の新シグネチャと、それに伴う既存 `TestSidebar_...` の更新方針。
  5. モック7枚の更新範囲（show/readings に文脈2リンク追加・create/edit の active 除去）と、CSS 追記の要否（原則なし＝既存 `.active` 流用）。
- **Research Needed**: なし（外部依存・未知技術なし）。design は HTMX実装ガイド §2（変換ルール）・§3（id 一覧）の「サイドバーは HTMX 差し替え対象外」である点の確認に留まる。

---

# Design Discovery & Synthesis（design フェーズ追記）

## Summary
- **Discovery Scope**: Extension（既存 templ レイアウト/サイドバーの純拡張）
- **Key Findings**:
  - サイドバー描画の合流点は `App.templ:48` の単一 `@component.Sidebar()` のみ。ここを拡張すれば全7画面へ波及する。
  - **import 方向の制約が設計を決める**: `layout` が `component` を import する（App.templ が Sidebar/SiteHeader を呼ぶ）ため、**`component` は `layout` を import できない**（循環。`component/views.go:275` に明記）。→ ナビ文脈の共有型は **`component` 側**に置く（`layout.AppLayoutData` が `component.SidebarNav` を保持する向きは既存 import と一致）。
  - DB スキーマ変更・新規ルート・新規クエリ・CSS 変更はいずれも不要（既存 `.sidebar li a.active` と既存ルートを消費）。

## Research Log

### サイドバー描画経路と AppLayoutData 構築箇所
- **Context**: ナビ文脈をどこから流すか確定する。
- **Sources Consulted**: `internal/view/layout/App.templ`、`internal/view/component/{Sidebar.templ,views.go}`、`internal/handler/{dashboard,device_show,readings,device,alert_rule,alert_history}.go`、`internal/view/page/views.go`、steering（structure.md / tech.md）、`2cc_sdd/HTMX実装ガイド(動的)` §31/§40-B。
- **Findings**:
  - 全7 page templ が `@layout.App(v.Layout){...}` 形式。`AppLayoutData` の共通ビルダは無く、各ハンドラが個別構築（device_show.go:135 / readings.go:112 / alert_history.go:224 はインライン、dashboard.go:84・device.go:287/315・alert_rule.go:307 はヘルパ）。
  - device_show・readings は URL から `id int64` を取得済み（`DeviceShowView.DeviceID`・`ReadingsView.DeviceID` に既存）。文脈設定元として過不足なし。
  - 画面内 HTMX 部分更新（Chart / readings HX / alert-history HX / alert-rule インライン CRUD）は `renderComponent` で**フラグメントのみ**返し、App レイアウト（サイドバー）を含めない。→ R4.3 は現状構造で自動充足（サイドバーを fragment に足さないことを不変条件として守るのみ）。
- **Implications**: ナビ文脈は `AppLayoutData` に1フィールド追加＋7ハンドラで設定。設定漏れは**ゼロ値＝active なし・文脈リンクなし**で無害縮退（R2.6 と同じ振る舞い・fail-safe）。

### CSS / モック正本
- **Context**: active 表現と文脈リンクに新規 CSS が要るか。
- **Sources Consulted**: `mocks/html/style.css:170-190,682`、`2cc_sdd/HTMX実装ガイド(動的)` §31（モッククラスそのまま使用・独自クラス新設禁止）・§40-B（CSS 単一ソース）。
- **Findings**: `.sidebar li a.active`（背景＋左ボーダー primary＋太字）で active を表現済み。文脈リンクは既存 `nav li a` で賄える。**新規クラス不要＝CSS 変更なし＝`make sync-css` 不要**。
- **Implications**: モック7枚はサイドバーの状態（active 位置・文脈2リンク有無）のみを画面別に更新（templ 写経の正本）。CSS は触らない。

## Architecture Pattern Evaluation

| Option | Description | Strengths | Risks / Limitations | Notes |
|--------|-------------|-----------|---------------------|-------|
| A: AppLayoutData 拡張＋各ハンドラ設定（採用） | `AppLayoutData` に `Nav component.SidebarNav` を追加し App→Sidebar へ流す。各ハンドラが自画面の `Current`（＋show/readings は `DeviceID`）を設定 | 既存 seam に最小侵襲・ゼロ値縮退が fail-safe・テストが素直 | 構築7箇所に触れる（取り違えは型付き定数で緩和） | research §3 で推奨 |
| B: middleware でパス駆動注入 | ルートパスから現在ページ/ID を導出し context 注入 | ハンドラ非変更・集中管理 | パス分岐が新たな真実の所在＝脆い／「edit は文脈外」境界が暗黙化／view 値を context で運ぶのは既存外 | 不採用 |
| C: A＋共通ヘルパ括り出し | A に文脈セットの小ヘルパを併用 | 重複/取り違え縮小 | ヘルパ置き場の判断が増える | 7箇所が各1行で十分小さく、現時点ではヘルパ不要と判断（将来必要なら追加可） |

## Design Decisions

### Decision: ナビ文脈の共有型は `component.SidebarNav`（typed・int64 0-sentinel）
- **Context**: 現在ページ識別子＋選択中デバイス ID を、循環 import なしで layout↔component 間で共有する。
- **Alternatives Considered**:
  1. primitive 2引数 `Sidebar(current string, deviceID int64)` — 型が弱く取り違えやすい。
  2. `nav` 新パッケージに型を置く — 小機能に対しパッケージ追加は過剰。
  3. `component` に typed struct を置く（採用）。
- **Selected Approach**: `component` パッケージに `type NavPage string` ＋定数（`NavDashboard`/`NavDeviceShow`/`NavReadings`/`NavAlertRules`/`NavAlertHistory`）と `type SidebarNav struct { Current NavPage; DeviceID int64 }` を定義。`layout.AppLayoutData` に `Nav SidebarNav` を追加（layout→component の既存 import 方向に一致）。`Sidebar(nav SidebarNav)` が文脈リンク条件描画と active 付与を行う。
- **Rationale**: import 方向制約を満たす唯一の素直な置き場が component。`DeviceID int64` の 0-sentinel は既存 `DeviceShowView.DeviceID int64` と同型で、`DeviceID > 0` を「文脈あり」の明確な述語にできる。`Current` 単一フィールドが「同時 active ≤1」（R2.7）を構造的に保証。
- **Trade-offs**: ✅ 型安全・単一抽象で R1〜R3 を一括充足・fail-safe。❌ 構築7箇所の更新が必要（ただし各1行）。
- **Follow-up**: device-edit は `id` を持つが `DeviceID` を**設定しない**（要件 Out of scope）。実装時にコメントで明示し、テストで「edit は文脈リンクなし」を固定する。

### Decision: 文脈リンク URL は Sidebar が `DeviceID` から整形
- **Context**: `/devices/{id}`・`/devices/{id}/readings` をどこで組むか。
- **Selected Approach**: Sidebar templ 内で `fmt.Sprintf` ＋ `templ.SafeURL` により `DeviceID` から組む（既存 `DeviceChartArea` 等と同パターン）。ハンドラは ID のみ渡す。
- **Rationale**: 単一ソース・渡す値を最小化（synthesis simplification）。
- **Trade-offs**: ✅ 重複なし。❌ URL 形が Sidebar に閉じる（既存ルート前提なので問題なし）。

## Synthesis Outcomes
- **Generalization**: R1（文脈リンク）・R2（active）・R3（往復）は「現在のナビ文脈でサイドバーを描く」単一能力の側面。`SidebarNav` 1 struct＋Sidebar 条件描画で一括実現（別機構を作らない）。
- **Build vs Adopt**: 条件 class は templ ネイティブの `templ.KV` を採用（自作しない）。外部依存ゼロ。
- **Simplification**: middleware（B）・新パッケージ・新 CSS・新ルートをすべて排除。最小構成＝1 struct＋Sidebar シグネチャ変更＋7ハンドラ各1行＋モック4枚更新。

## Risks & Mitigations
- 構築7箇所の設定漏れ — ゼロ値縮退で無害（active/文脈なし）。テストで主要画面の active/文脈を固定。
- モック↔templ↔テストの三者乖離（プロジェクト規約のデグレ温床） — モック正本を先に更新し templ を写経、Render テストで一致を固定。
- 既存 `TestSidebar_...`（component_test.go:39）がシグネチャ変更で破綻 — 同タスク内で新引数へ更新＋assertion 拡張。

## References
- `internal/view/layout/App.templ:48` — 唯一の `@component.Sidebar()` 呼び出し（seam）
- `internal/view/component/views.go:275` — component が layout を import しない（循環回避）の根拠
- `mocks/html/style.css:186-190` — `.active` 表現（CSS 変更不要の根拠）
- `2cc_sdd/HTMX実装ガイド(動的).md` §31（モッククラス流用・独自クラス禁止）/ §40-B（CSS 単一ソース）
