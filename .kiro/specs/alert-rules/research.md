# 実装ギャップ分析: alert-rules

> 対象要件: `.kiro/specs/alert-rules/requirements.md`（Requirement 1〜10）
> 分析日: 2026-06-08 ／ 分析手法: 既存コードベースのパターン抽出（並列 Explore）＋ DBスナップショット／HTMX実装ガイド §4・§12 参照
> 注: 本時点で requirements は generated（未承認）。ギャップ分析は要件修正の判断材料としても利用可。

## 分析サマリー

- **基盤（S1）は全て実装済み・消費するだけ**: Session 認証ガード（`RequireAuth`→302 /login）、`App` レイアウト（CSRF meta + `htmx:configRequest` で X-CSRF-Token 自動付与 + htmx.js 同梱済み）、MethodOverride（`_method` で PUT/PATCH/DELETE 対応済み）、所有者認可 `authz.RequireDeviceOwner`、`renderPage`/`renderComponent`/`renderError` ヘルパ、項目別バリデーションエラー変換パターン。**セッションプロンプトの「未確定事項（CSRF/MethodOverride/scs）」は S1 で確定済み**。
- **全 sqlc クエリ実装済み**: `GetAlertRule`/`ListAlertRulesByDevice`/`CreateAlertRule`/`UpdateAlertRule`/`ToggleAlertRule`/`SoftDeleteAlertRule` ＋デバイス選択用 `ListDevicesByUser`。domain（`Metric`/`ComparisonOperator` の `Label()`/`Unit()`/`AllXxx()`/`ParseXxx()`）と pgconv（`Numeric2`/`NumericToFloat`）も完備。
- **新規実装はほぼ「View 層＋ハンドラ」に閉じる**: `internal/handler/alert_rule.go`（6 アクション）と 5 つの templ（`page.AlertRules` + `component.AlertRuleSection`/`AlertRuleForm`/`AlertRuleList`/`AlertRuleRow`）、配線 7 ルート。既存の device-create-edit / readings ハンドラの確立パターンをほぼ流用できる。
- **要注意ギャップ（後述）**: ①ルール単位の所有者認可ヘルパ不在（rule→device→owner の合成が必要）、②`UpdateAlertRule` が `is_enabled` 必須だが編集フォームに無い（現在値の保全が必要）、③`threshold` の `required` バインドと「0 値」問題（string バインド推奨）、④Tom Select が未配線（head アセット注入方法が要設計）、⑤非所有時のステータス方針（既存は read=404両方／mutation=403・404 と分裂）。
- **総合難易度: M（3〜7 日）／リスク: Low〜Medium**。確立パターンの横展開が大半。新規性は「インライン CRUD（テーブル全体差し替え＋行単位 outerHTML）」と「Tom Select の head 注入」のみ。

---

## 1. 現状調査（既存資産・規約）

### 1.1 ハンドラ層の確立パターン（`internal/handler/`）
- **DI**: ハンドラは消費する最小 interface（例 `DeviceRepo`）を自前定義し、`repository.Querier`（具象 `repository.New(pool)`）を構造体フィールドに inject（`device.go:38-49`、`main.go:145` `deviceH := &handler.DeviceHandler{Repo: q}`）。
- **userID 解決**: `auth.UserID(c)`（Gin context、`auth.go:88`）。ルートは `web.GET(..., middleware.RequireAuth(), h.X)` で個別ガード（`main.go:146-158`）。
- **HTMX 出し分け**: `if c.GetHeader("HX-Request") != "" { renderComponent(c, component.Xxx(...)) ; return }` → 非 HTMX は `renderPage(...)`（`readings.go:102-105`）。`renderComponent`/`renderPage` は `comp.Render(c.Request.Context(), c.Writer)` の薄いラッパ（`auth.go:195-208`）。
- **エラー→ステータス**: switch + `errors.Is`。**read 系**は列挙防止で `pgx.ErrNoRows` も `authz.ErrNotOwner` も 404（`device_show.go:339-349` `renderDeviceReadError`）。**mutation 系**は `ErrNoRows`→404 / `ErrNotOwner`→403（`device.go:275-286` `renderDeviceOwnerError`）。
- **バリデーション**: `c.ShouldBind(&form)` → `toDeviceFieldErrors(err)`（`device_form.go:107-121`）で `validator.ValidationErrors` を `map[string]string`（form名→日本語）へ変換 → フォーム再描画。`deviceFieldKey`/`deviceValidationMessage` の 2 ヘルパ構成。
- **リダイレクト**: POST/PUT/DELETE 成功は 303 SeeOther（`device.go:108,199`）。
- **パラメータ**: `strconv.ParseInt(c.Param("device"), 10, 64)` / `c.Query("period")`（`device_show.go:75,81`）。
- **テスト**: フェイクレポ（interface 実装＋エラー注入）＋ `httptest` ＋ `withUser := func(c){ auth.SetUserID(c, uid); c.Next() }` で認証注入（`device_test.go:130-138, 318-348`）。

### 1.2 View 層（`internal/view/`、3 層）
- **App レイアウト（実装済み・本画面が消費）**: `layout.AppLayoutData{Title, UserName, CSRFToken, CSSURL, Flash}`。head に `<meta name="csrf-token">`、body 末尾に `htmx:configRequest` で全 HTMX ミューテーションに `X-CSRF-Token` を自動付与、htmx.js / alpine.js 同梱済み。→ **本画面の HTMX POST/PUT/PATCH/DELETE は追加の CSRF 配線なしで動く**。
- **フォーム templ パターン**: `component.DeviceForm` が DTO（`DeviceFormView{... Errors map[string]string}`）を受け取り、`{ v.Errors["name"] }` を `<span class="error-message">` に描画、`value={ v.Name }` / `checked?={ v.IsActive=="1" }` / select の `selected?=` で入力値復元（`DeviceForm.templ:8-51`）。page は `@layout.App(v.Layout){ @component.DeviceForm(v.Form) }`。
- **非 HTMX フォームの CSRF**: hidden `<input name="gorilla.csrf.Token" value={ v.CSRFToken }>`（`Login.templ:11`）。**本画面のミューテーションは全て HTMX 化** のためヘッダ方式（自動）で足り、hidden は不要。ただしデバイス選択フォーム（GET）は CSRF 不要。
- **CSSURL**: `view.CSSURL()`（`static.go:28`、バージョンクエリ付き）をハンドラが `AppLayoutData.CSSURL` に詰める。
- **共通 SelectOption 型は無い**: domain の `AllMetrics()`/`AllComparisonOperators()` を templ でループする（汎用ヘルパなし）。デバイス選択肢は `[]repository.Device` を直接ループ。

### 1.3 ルーティング／ミドルウェア配線（`cmd/server/main.go`）
- Web グループ: `web := engine.Group("/", middleware.SessionLoad(sm), middleware.CSRF(cfg))`（`main.go:133`）。HTTP ハンドラ層で `MethodOverride` → `sm.LoadAndSave` → engine（`main.go:168`）。
- ルート登録: PUT/PATCH/DELETE は `web.PUT(...)`/`web.DELETE(...)` を**直接**使い（`main.go:152,158`）、HTML フォームは POST + `_method` を MethodOverride が当該メソッドへ昇格。
- MethodOverride 対応: `PUT`/`PATCH`/`DELETE`（`method_override.go:11-15`）、フォーム値 `_method`（`ToUpper` 正規化）、POST のみ検査。
- CSRF: `github.com/gorilla/csrf`、Web グループ限定（`/api` 除外）。

### 1.4 domain / pgconv / authz
- `domain.Metric`（string enum: `MetricTemperature`/`MetricHumidity`、`Label()`/`Unit()`/`Valid()`/`ParseMetric()`/`AllMetrics()`）。
- `domain.ComparisonOperator`（string enum: 記号 `>`/`<`/`>=`/`<=`、`Label()`/`Evaluate()`/`ParseComparisonOperator()`/`AllComparisonOperators()`）。
- `pgconv.Numeric2(float64) pgtype.Numeric`（NUMERIC(5,2) 化）／`NumericToFloat(pgtype.Numeric) float64`。
- `authz.RequireDeviceOwner(ctx, q DeviceGetter, deviceID, userID) (repository.Device, error)`。sentinel: `ErrUnauthenticated`（userID<=0 fail-closed）/ `ErrNotOwner` / 透過 `pgx.ErrNoRows`。**alert_rule 用ヘルパは存在しない**。

---

## 2. Requirement → 資産マップ（ギャップタグ）

| 要件 | 必要な技術要素 | 既存資産（再利用） | ギャップ |
|------|----------------|--------------------|----------|
| R1 初期表示 | `page.AlertRules`、デバイス選択肢、ルール一覧 | `ListDevicesByUser`/`ListAlertRulesByDevice`、App レイアウト、`renderPage` | **Missing**: templ 一式・ハンドラ Index。device_id 既定（最初の所有デバイス）・404・0件案内のロジック |
| R2 デバイス切替 | change で `#alert-rule-section` を innerHTML 差し替え | HX-Request 出し分けパターン、Tom Select | **Missing**: `AlertRuleSection` 部分返却。**Unknown**: Tom Select の head 注入と change→hx-get 連携 |
| R3 追加 | POST→`CreateAlertRule`→一覧+空フォーム返却、422 | `CreateAlertRule`(is_enabled 引数)、バリデーション変換パターン | **Missing**: ハンドラ Add・`toAlertRuleFieldErrors`。**Constraint**: 新規は is_enabled=true 固定 |
| R4 編集フォーム読込 | GET edit→既存値フォーム outerHTML | `GetAlertRule`、`AllMetrics/AllComparisonOperators` | **Missing**: `AlertRuleForm(editingRule)`・ハンドラ。所有者チェック合成 |
| R5 更新 | PUT→`UpdateAlertRule`→一覧+空フォーム、422 | `UpdateAlertRule`(is_enabled 必須) | **Constraint**: 編集フォームに is_enabled 無 → **現在値を取得して保全**（後述4.2） |
| R6 有効切替 | PATCH→`ToggleAlertRule`→当該行 outerHTML | `ToggleAlertRule`(サーバ側反転) | **Missing**: `AlertRuleRow` 単体返却・ハンドラ Toggle |
| R7 削除 | DELETE→`SoftDeleteAlertRule`→一覧 innerHTML、確認 | `SoftDeleteAlertRule`、`hx-confirm`（§11） | **Missing**: `AlertRuleList` 部分返却・ハンドラ Delete |
| R8 バリデーション | metric/operator/threshold の binding + 項目別日本語 | validator パターン、binding タグ | **Constraint**: threshold `required` と 0 値（後述4.3）。operator oneof は記号 |
| R9 一覧表示 | Label/Unit/空状態/論理削除除外 | `Label()`/`Unit()`、mock の `.empty-message` | **Missing**: `AlertRuleList`/`AlertRuleRow` templ（モック写経） |
| R10 認証・認可・CSRF | 302/login、403/404、CSRF | `RequireAuth`、`RequireDeviceOwner`、App の htmx CSRF | **Missing**: ルール所有者認可の合成（後述4.1）。**Constraint**: 403 vs 404 方針（後述4.5） |

---

## 3. 実装アプローチ Option A/B/C

### Option A: 既存ハンドラへの追記（device.go 等を拡張）
- ❌ 不採用寄り。alert_rule は独立ドメインで責務が異なり、既存ハンドラを肥大化させる。

### Option B: 新規ファイル群で独立実装（推奨）
- `internal/handler/alert_rule.go`（+`alert_rule_form.go` でフォーム DTO/エラー変換を分離、200-400 行/ファイル方針）。
- `internal/view/page/AlertRules.templ` + `internal/view/component/AlertRule{Section,Form,List,Row}.templ`。
- `internal/view/component/alert_rule_view.go`（フォーム DTO・SelectOption 風ヘルパ）。
- 既存の最小 interface DI・`renderPage/renderComponent/renderError`・バリデーション変換ヘルパ・`authz.RequireDeviceOwner` を**そのまま再利用**。
- ✅ 責務分離・テスト容易・既存への破壊変更ゼロ。✅ structure.md の「画面=feature ファイル分割」「view 3層」に合致。
- **これを推奨**。

### Option C: ハイブリッド（共有ヘルパだけ抽出）
- Tom Select の head 注入を「ページ別 head アセット」共通機構として `App` レイアウトに拡張（`AppLayoutData` に `HeadExtra`/`ScriptExtra` を追加）し、alert-rules・alert-history で共用。
- ✅ 後続の alert-history（同じく Tom Select）と一貫。❌ App レイアウト改修は全画面に影響するため、最小差分で。
- **Tom Select 注入方式のみ Option C を併用**（§4.4 で詳細）。

---

## 4. 既知の実装上の注意（落とし穴・design で確定すべき点）

### 4.1 ルール単位の所有者認可は「合成」が必要（Missing）
ルール ID 起点の操作（edit/update/toggle/delete）は、`GetAlertRule(ruleID)` → `rule.DeviceID` → `authz.RequireDeviceOwner(ctx, q, rule.DeviceID, userID)` の 2 段で所有者を判定する。**設計判断**: この合成を (a) ハンドラ内に都度書く か (b) `authz` に `RequireAlertRuleOwner(ctx, ruleGetter, deviceGetter, ruleID, userID)` ヘルパを新設して BOLA を集約する（structure.md ルール⑤「所有者認可は authz に集約」に照らすと **(b) 推奨**）。`GetAlertRule` は `deleted_at IS NULL` 前提なので、論理削除済みは `pgx.ErrNoRows`→404 に落ちる。

### 4.2 `UpdateAlertRule` は is_enabled 必須・フォームに無い（Constraint）
`UpdateAlertRule` は `(id, metric, operator, threshold, is_enabled)` を取る。編集フォームは metric/operator/threshold のみ（有効切替は別 PATCH）。→ **更新ハンドラは所有者チェックで取得した現在ルールの `is_enabled` をそのまま渡す**（更新で有効状態が意図せず反転しないよう保全）。`R5` のテストに「更新で is_enabled が保持される」観点を含めると安全。

### 4.3 `threshold` の `required` と「0 値」問題（Constraint・要設計）
float64 フィールドに `binding:"required"` を付けると、`threshold=0`（例: 温度 > 0℃）が**ゼロ値扱いで弾かれる**。一方で空送信も 0 になり区別不能。→ **推奨**: フォーム DTO で `threshold string` として受け、`binding:"required"`（空文字を弾く）→ `strconv.ParseFloat` → `pgconv.Numeric2`。パース失敗は項目別エラー（R8-3）。これにより「未入力=エラー」「0=妥当」を両立。numeric(5,2) の範囲（±999.99）超過は ParseFloat 後に範囲チェック or DB エラー→500。design の Testing Strategy で「0 を受理／空を 422」のケースを明記。

### 4.4 Tom Select の head アセット注入（Unknown・要設計）
App レイアウトは Tom Select の CSS/JS を読み込んでいない（どの実装画面でも未使用）。デバイス選択（`select.js-tom-select`）に Tom Select を適用するには (a) head に CDN（CSS+JS）、(b) ページ末尾に初期化スクリプトが要る。**設計判断**:
- 注入方式: `AppLayoutData` に任意の `HeadExtra templ.Component`/`BodyEndExtra templ.Component` を追加して alert-rules ページから差し込む（後続 alert-history と共用、Option C）。
- **ライフサイクル簡易化の好機**: 本画面のデバイス選択は swap 対象 `#alert-rule-section` の**外側**（独立 card）にあるため、device 切替時に select 自身は再描画されない。→ **ページ初期化 1 回で足り、HTMX swap 後の再 init（TS-1〜）問題を回避できる**。§16/C12 の複雑なライフサイクル管理は本画面では不要（alert-history も同様か design で確認）。
- CDN バージョンは mock と統一（tom-select@2.3.1）。Research Needed: CDN を恒久採用か self-host か（mock は CDN）。

### 4.5 非所有時のステータス: 既存規約が read=404両方／mutation=403・404 で分裂（Constraint）
`device_show` の read 系は列挙防止で `ErrNotOwner` も 404。一方 requirements（R2/R4 等）は**非所有を 403、不在を 404**と明記（セッションプロンプト「他ユーザーデバイスは 403」準拠）。→ **設計判断**: 本画面は `device.go` の mutation 系マッピング（`renderAlertRuleOwnerError`: NoRows→404 / NotOwner→403）を read/mutation 双方に適用する方針で要件と整合させる。列挙防止を優先するなら要件を 404 に修正。**要件どおり 403 を採るのが既定**だが、プロジェクト横断のセキュリティ規約（列挙防止）と衝突しうるため design で一度明示確認。

### 4.6 operator の oneof は記号・空 option（Constraint）
`binding:"required,oneof=> < >= <="`（validator は空白区切り。記号に空白を含まないので OK）。mock の select は先頭に空 option（`value=""`）→ 未選択は `required` で 422。metric も同様。templ の option 生成は `domain.AllComparisonOperators()`＋`Label()`、`selected?={ form.Operator == string(op) }` で復元。

### 4.7 インライン CRUD の返却単位（§12 準拠）
- 追加/更新/デバイス切替: `#alert-rule-section`（フォーム＋一覧）を innerHTML で**まとめて**返す（件数整合のためテーブル全体返却。§12「個別行差し替えでなくテーブル全体」）。
- 削除: `#alert-rule-list`（tbody wrapper）を innerHTML。
- 有効切替: `#alert-rule-row-{rule}` を outerHTML（当該行のみ）。
- → ハンドラは「セクション全体／リストのみ／行のみ」の 3 粒度の部分返却を持つ。templ 分割（Section ⊃ Form + List ⊃ Row）を素直に対応させる。

---

## 5. Effort / Risk

| 項目 | 評価 | 根拠 |
|------|------|------|
| 全体 Effort | **M（3〜7 日）** | 確立パターンの横展開が大半だが、templ 5 種＋ハンドラ 6 アクション＋インライン CRUD の 3 粒度返却＋テスト（80%）で量がある |
| 全体 Risk | **Low〜Medium** | 基盤・クエリ・domain・pgconv が完備で未知技術なし（Low 寄り）。Tom Select head 注入と threshold バインド設計に小さな不確実性（Medium 要素） |
| ハンドラ 6 アクション | S | device/readings の写経 |
| templ 5 種（モック写経＋分割） | M | 3 粒度返却の分割設計が新規 |
| ルール所有者認可ヘルパ | S | RequireDeviceOwner の薄い合成 |
| Tom Select head 注入 | S〜M | レイアウト最小拡張＋初期化（swap 外なので簡易） |
| テスト（80%＋認可 403/404） | M | フェイクレポ＋httptest＋templ 文字列検査の確立パターン |

---

## 6. design フェーズへの推奨・Research Needed

### 推奨アプローチ
- **Option B（新規ファイル群）を主軸 + Tom Select 注入だけ Option C（App レイアウト最小拡張）**。
- **所有者認可は `authz.RequireAlertRuleOwner` を新設**して BOLA を集約（structure.md ルール⑤準拠）。
- **threshold は string バインド**（required で空を弾き、ParseFloat→`pgconv.Numeric2`、0 を受理）。
- **更新は現在ルールの is_enabled を保全**。
- **非所有=403／不在=404** を要件どおり採用（mutation 系マッピングを read にも適用）。

### design で確定すべき主要判断（Boundary Commitments 化）
1. 所有者認可ヘルパの設置先（authz 新設 vs ハンドラ内合成）。
2. 非所有時 403 採用 vs 列挙防止 404（要件との整合確認）。
3. Tom Select の head/script 注入機構（`AppLayoutData` 拡張の形）と適用範囲（alert-rules/alert-history 共用か）。
4. threshold のバインド型（string 推奨）と numeric(5,2) 範囲超過時の扱い。
5. templ コンポーネント分割（Section/Form/List/Row）と各ハンドラの返却粒度の対応表。
6. device_id 未指定時の「最初の所有デバイス」選択順（created_at ASC 等、`ListDevicesByUser` の順序を確認）。

### Research Needed
- `ListDevicesByUser` の ORDER BY（device_id 既定選択の安定順序）。
- Tom Select の CDN 恒久採用可否（mock は CDN@2.3.1）。self-host 方針があるか tech.md と突合。
- alert-history（後続）が同じ Tom Select 注入機構を要するか（共通化の射程確認）。
- テストガイダンス集の「インライン CRUD・CSV」「バリデーション（validator 単体）」「CSRF（GET→トークン往復）」節を design の Testing Strategy 導出時に参照。

---

## Discovery & Synthesis ログ（design フェーズ・2026-06-08）

### 参照した HTMX実装ガイド節と得た設計制約
- **§4 alert-rules / §12 インラインCRUD**: 追加/更新/デバイス切替は `#alert-rule-section` を innerHTML（テーブル全体返却で件数整合）、削除は `#alert-rule-list` innerHTML、有効切替は `#alert-rule-row-{id}` outerHTML。→ ハンドラの返却粒度 3 種を確定。
- **§7 バリデーション**: HTMX フォームは 422 + フォームコンポーネント返却。`htmx.config.responseHandling` に `422:swap:true` が必要。→ App.templ に未配線（grep 0）と判明し、本 spec で加算。`AlertRuleHandler.Add` の binding 例（float64）を確認し、threshold は 0 値問題のため string バインドへ意図的逸脱。
- **§11 削除確認**: `hx-confirm` ブラウザネイティブ。Alpine モーダルは使わない。
- **§16/C12 Tom Select**: デバイス select は swap 対象 `#alert-rule-section` の**外**にあるため、swap 後の再 init（TS-1〜）が不要 → init 1 回で足り、ライフサイクル複雑性を回避。
- **§3.6 id 一覧 vs §12**: `alert-rule-list` を「tbody」か「div wrapper」かの差は、wrapper（table + empty-message を内包）方式で解消（削除で空状態へ innerHTML 遷移可能）。

### Synthesis（3 レンズ）
- **Generalization**: 追加と更新は「フォーム送信→Section 全体返却」の同一構造（EditingRuleID で分岐）。`AlertRuleForm` を追加/編集兼用の単一コンポーネントに一般化（別ページ/モーダルを作らない）。422 ハンドリングも追加/更新で共通化。
- **Build vs Adopt**: Tom Select=**Adopt**（CDN 2.3.1、モック準拠）。所有者認可ヘルパ=**Build**（`RequireAlertRuleOwner`）だが既存 `RequireDeviceOwner` の薄い合成で、structure.md ルール⑤の BOLA 集約に必要。独自 Repository interface は新設せず `Querier` を使用。
- **Simplification**: Tom Select 注入は「App レイアウトへの head-injection 抽象（HeadExtra 等）」を作らず、**App.templ への直接加算**（9 画面規模・2+ 画面で使用のため speculative 抽象を回避）。init は対象 select 不在ページで no-op。service 層は挟まない（handler→repository 直結。判定ロジックは S2 のため本画面に service 不要）。

### 確定した主要設計判断（research §6 の問いへの回答）
1. 所有者認可ヘルパ → `authz.RequireAlertRuleOwner` 新設（BOLA 集約）。
2. 非所有=403／不在=404（要件準拠、read/mutation 共通。device-show の列挙防止 404-both とは別方針＝要件が 403 を明示）。
3. Tom Select 注入 → App.templ 直接加算（head CDN + body-end init）。alert-history と共用。
4. threshold → string バインド（required で空弾き→ParseFloat→範囲 |x|≤999.99→`pgconv.Numeric2`）。0 受理。
5. templ 分割 ↔ 返却粒度 → Section(form+list)／Form／List／Row の 4 コンポーネントと §4 の swap 対応表（design.md「3 モードと返却粒度」表）。
6. device_id 既定 → `ListDevicesByUser`（created_at DESC）の先頭。0 件は案内表示。
7. 更新時 is_enabled → 所有者チェックで取得した現在ルールの値を保全（UpdateAlertRuleParams が必須のため）。
8. HTTP メソッド → HTMX は実 PUT/PATCH/DELETE（web.PUT/PATCH/DELETE 受信）、no-JS は POST+_method を MethodOverride 昇格。
