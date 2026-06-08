# 実装ギャップ分析（research.md） — alert-history

> 対象: `.kiro/specs/alert-history/requirements.md`（R1〜R9）
> 前提: ブラウンフィールド。DB・sqlc クエリ・domain 層は実装済み、Web UI 層が未着手。
> 本書は「情報と選択肢」を提示するもので、最終決定は design フェーズに委ねる。

## 1. 結論サマリ

- **データ層・domain 層は完全に揃っている**。アラート履歴に必要な sqlc クエリ（`ListAlertHistoriesPaginated` / `CountAlertHistoriesInRange` / `ListDevicesByUser`）と表示用ロジック（`Metric.Label()/Unit()`・`ComparisonOperator.Label()`）が実装済みで、新規 migration・新規クエリは**原則不要**。
- **基盤（S1相当）も完成済み**。Session 認証・`SessionLoad`・`RequireAuth`・`MethodOverride`・共通レイアウト `App.templ`・CSS 配信（go:embed + `CSSURL()`）が稼働しており、`/alerts/history` は既存の認証必須グループ（`web`）にルート1本を足すだけで載る。
- **ギャップは Web UI 層のみ**: ハンドラ `internal/handler/alert_history.go`、ページ `page/AlertHistory.templ`、一覧 Fragment、ページネーション部品（番号リンク対応）が未実装。**兄弟画面 readings（センサーデータ履歴）が「一覧＋フィルタ＋ページネーション＋GET検索」というほぼ同型の課題を既に解決済み**で、これが第一の写経元になる。
- **設計フェーズで決める論点が3つ**:（a）ページネーションの番号リンク対応（既存 `Pagination.templ` は前/次のみ）、（b）OOB swap を使うか（session-08 prompt は OOB 指示だが、既存コードベースは OOB 不使用＝Readings は一覧Fragmentにページャを内包して単一 innerHTML swap で解決）、（c）期間バリデーション失敗時のステータス（Readings=200+インライン vs session-08 の 422 言及）。
- 推奨アプローチは **Option A（既存 readings パターンの拡張・写経）+ ページネーション部品のみ Option B（番号対応の新規 or 拡張）**。総じて **Effort S（1〜3日）・Risk Low**。

## 2. Requirement → 既存資産マップ（ギャップタグ: ✅充足 / ⚠️判断要 / ❌未実装）

| 要件 | 必要な技術要素 | 既存資産 | ギャップ |
|---|---|---|---|
| R1 初期表示（全件最新順・20件/頁） | 一覧取得・最新順・LIMIT/OFFSET | `ListAlertHistoriesPaginated`（`triggered_at DESC`・JOIN で `device_name` 込み、`internal/repository/alert_histories.sql.go`） | ✅ クエリ充足 / ❌ ハンドラ・ページ未実装 |
| R2 デバイス・期間フィルタ＋部分更新＋URL反映 | device_id(nullable)・from/to・HX分岐・push-url | `ListAlertHistoriesPaginated`（`DeviceID *int64`＝全デバイス対応、from/to 引数）、readings ハンドラの `c.GetHeader("HX-Request")` 分岐 | ✅ クエリ充足 / ❌ ハンドラ・hx属性付与未実装 |
| R3 ページネーション（番号・前/次・条件保持） | 件数・総頁計算・番号リンク | `CountAlertHistoriesInRange`、readings の `totalPagesOf()`/`parsePage()`/ページクランプ、`component/Pagination.templ` | ✅ 件数・計算ロジック流用可 / ⚠️ **既存 Pagination は前/次のみ＝番号リンク非対応** |
| R4 表示フォーマット（日時/条件/実測値/通知/非正規化） | 日時整形・`> 35.00℃`・`38.50℃`・済/未 | `Metric.Label()`("温度"/"湿度")・`Metric.Unit()`("℃"/"%")、行に `Metric/Operator/Threshold/ActualValue/IsNotified/TriggeredAt` 含む（非正規化値そのまま） | ✅ domain メソッド充足 / ❌ templ 整形・`ComparisonOperator` 記号表示・`pgtype.Numeric→%.2f` 変換未実装 |
| R5 デバイス選択肢（全デバイス＋絞り込み入力） | 本人デバイス一覧・Tom Select | `ListDevicesByUser`、`App.templ` に Tom Select グローバル初期化（`select.js-tom-select`） | ✅ 充足 / ❌ select の templ 化（option 描画）未実装 |
| R6 0件表示 | empty-message 分岐 | readings の `if v.HasData {…} else {<p class="empty-message">…}` パターン | ✅ パターン流用可 / ❌ templ 未実装 |
| R7 期間バリデーション（from≤to、初期表示はスキップ） | 日付パース・範囲検証 | readings の `parseDateBounds()`（形式エラー検出、検索スキップ＋インライン表示で 200 継続） | ⚠️ **from≤to 範囲検証は要追加**（parseDateBounds は形式のみ） / ⚠️ ステータス方式（200 vs 422）要決定 |
| R8 アクセス制御・テナント分離 | 認証必須・本人デバイス限定・BOLA防止 | `RequireAuth`（未認証→/login）、`ListAlertHistoriesPaginated` が `UserID` で JOIN スコープ＝非所有 device_id は自動的に0件、`authz.RequireDeviceOwner`（明示404にしたい場合） | ✅ R8-1/R8-2 は基盤で充足、R8-3 はクエリで自動充足 / ⚠️ 非所有 device_id を「空表示」か「404」か要決定 |
| R9 取得失敗時のエラー応答 | DBエラー→500 | readings/alert-rule の `renderError(c, http.StatusInternalServerError)` パターン | ✅ パターン流用可 / ❌ ハンドラ未実装 |

### 新規作成が必要なファイル（Web UI 層のみ）
- ❌ `internal/handler/alert_history.go`（`AlertHistoryHandler` + consumer interface `AlertHistoryRepo`）
- ❌ `internal/view/page/AlertHistory.templ`（フルページ: h1＋フィルタ＋一覧＋ページネーション）
- ❌ `internal/view/component/AlertHistoryList.templ`（Fragment: tbody＋空状態＋ページネーション。readings の `DeviceReadingsList` 写経）
- ⚠️ ページネーション番号リンク対応（既存 `Pagination.templ` 拡張 or 新規 `AlertHistoryPagination.templ`）
- ⚠️（任意）`DeviceSelectOption.templ`（option 描画の再利用部品。複数画面共用を狙うなら）
- ❌ ルート登録 `web.GET("/alerts/history", middleware.RequireAuth(), alertHistoryH.Index)`（`cmd/server/main.go`）

## 3. 実装アプローチの選択肢

### Option A: 既存 readings パターンを写経・拡張（推奨の主軸）
- **適合理由**: readings（`/devices/{device}/readings`）が「フィルタ＋一覧＋ページネーション＋HX-Request 分岐＋GET 検索＋empty-message」というアラート履歴とほぼ同型の課題を既に解決済み。ハンドラ構造（`ReadingsRepo` interface・`renderPage/renderComponent/renderError`・ページクランプ・`parseDateBounds`）をそのまま型として流用できる。
- **差分**: readings は単一デバイス固定（`/devices/:device/…`）だが、アラート履歴は device_id を**フィルタ**（nullable・全デバイス可）として扱う。所有者認可は readings が `RequireDeviceOwner`（パスのデバイス）なのに対し、アラート履歴は user_id スコープのクエリで自動分離（§R8 判断参照）。
- ✅ 既存の実証済みパターン・最小の新規概念 / ✅ テストも readings/alert-rule のテストを写経可能（テストガイダンス集 + 既存 `*_test.go`） ❌ readings との微妙な差（フィルタ vs パス固定）を取り違えると BOLA リスク

### Option B: ページネーション部品のみ新規 or 拡張
- **対象**: R3 の番号リンク（1,2,3）。既存 `Pagination.templ` は前/次＋"X / Y ページ"のみでモック（`mocks/html/alert-history.html`）の番号リンクを満たさない。
- **選択肢 B-1（拡張）**: `Pagination.templ` に番号ウィンドウ（現在頁周辺 N 個）を追加し、readings 側も恩恵を受ける形へ汎用化。`PaginationView` に `Pages []int` 等を追加。
- **選択肢 B-2（新規）**: `AlertHistoryPagination.templ` を新規作成しモック準拠の番号リンクを実装。readings には影響させない。
- ✅ B-1 は再利用性向上 / ❌ B-1 は readings の既存テスト・表示への回帰リスク。✅ B-2 は影響局所 / ❌ B-2 はページャ実装の二重化。
- **付帯論点（OOB）**: session-08 prompt は `AlertHistoryPagination` を `hx-swap-oob="true"` で別途返す指示だが、**既存コードベースに OOB 使用例は無い**。readings は「ページネーションを一覧 Fragment（`#device-readings-list`）の内側に内包し、単一 innerHTML swap で一覧とページャを同時更新」する方式で OOB を回避済み。アラート履歴も同方式なら OOB 不要で既存流儀に一致する。→ **design で「OOB を避け readings 方式に揃える」案を第一候補として評価推奨**。

### Option C: ハイブリッド（採用形）
- 本機能の現実解は **ハンドラ・ページ・一覧 Fragment = Option A（readings 写経）**、**ページネーション = Option B（番号対応）** の組み合わせ。新規概念はページネーション番号リンクのみで、他は既存パターンの再適用に収まる。

## 4. Effort / Risk

- **Effort: S（1〜3日）** — データ層・基盤・兄弟画面パターンが揃い、新規ロジックはページネーション番号リンクと表示整形（`%.2f`＋単位）程度。TDD 1〜2サイクル × 数コンポーネント。
- **Risk: Low** — 未知技術なし・アーキ変更なし・既存パターンの拡張。唯一の注意点は「device_id をフィルタ扱いにする際のテナント分離」を取り違えないこと（クエリが user_id スコープ済みのため正しく使えば自動で安全＝Medium 未満）。

## 5. design フェーズへの申し送り（決定事項と Research Needed）

### 設計で確定すべき決定事項
1. **ページネーション番号リンク**: `Pagination.templ` 拡張（B-1・汎用化）か新規 `AlertHistoryPagination.templ`（B-2）か。モック（`mocks/html/alert-history.html`）は番号＋前/次。**写経の単一ソース原則（モック準拠）を満たすこと。**
2. **OOB の採否**: session-08 prompt の OOB 方式 vs 既存 readings の「一覧 Fragment 内包・単一 innerHTML swap」方式。**既存流儀（OOB 不使用）への統一を第一候補に評価**。OOB を採る場合は本プロジェクト初の OOB 使用となり、`hx-swap-oob` の作法を HTMX実装ガイド §2/§3 で要確認。
3. **期間バリデーション失敗時のステータス**: readings 流の **200＋インラインエラー（検索スキップ）** か、session-08 が言及する **422＋Fragment** か。GET フィルタフォームである点・既存 readings 先例との一貫性から 200＋インラインを推すが、要件 R7 はステータスを規定していないため design で確定。
4. **非所有 device_id の扱い**: クエリの user_id スコープで自動的に「空表示」（R8-3 を自然に充足）とするか、alert-rules 同様 `authz.RequireDeviceOwner` で明示 **404** とするか。BOLA 集約方針（structure.md ルール5）との整合を取りつつ決定。
5. **`pgtype.Numeric → 表示文字列` 変換**: `Threshold`/`ActualValue` の `%.2f`＋単位整形をどこで行うか（ハンドラで view 用 DTO に整形 vs templ 内）。既存 readings の温湿度表示・`internal/infra/pgconv` の変換ヘルパ有無を design で確認し方式統一。

### Research Needed（design で要確認）
- `internal/infra/pgconv` に `pgtype.Numeric → float64/string` 変換ヘルパが既にあるか（readings の温湿度整形が何を使っているか）。
- `ListAlertHistoriesPaginated` の from/to 引数の境界（`to` は当日終端を含むか＝`<= to` か `< to+1day` か）。requirements R2-3「指定期間内」を満たす境界を SQL 本文で確認。
- `DeviceSelectOption.templ` を新設して alert-rules 等と共用する価値があるか（alert-rules の device 選択描画と重複しないか）。

### 参照すべき正典（design フェーズ）
- HTMX実装ガイド(動的).md: §2（モック→templ+HTMX 変換・分割・命名）, §3（id 属性一覧）, §4 alert-history（検索・ページネーション・Tom Select）, §7（バリデーション表示）, §10（ページネーション LIMIT/OFFSET・総頁計算）, §16/C12（Tom Select ライフサイクル）
- 既存実装の写経元: `internal/handler/readings.go`, `internal/view/component/DeviceReadingsList.templ`, `internal/view/component/Pagination.templ`, `internal/handler/alert_rule.go`（device 選択・422 パターン）
- テストガイダンス集.md: HTTP（httptest+gin）・templ（Render→buffer→Contains）・ページネーション/CRUD・Querier 手書きモック

---

# Design フェーズ追記（discovery light + synthesis） — 2026-06-08

> Step 2 (light discovery) と Step 3 (synthesis) の所見。§1〜§5 のギャップ分析で挙げた4決定を確定した。

## Research Log（design discovery）

### HTMX 操作仕様の確定（§3.7 / §4 alert-history / §5 OOB）
- **Sources**: `2cc_sdd/HTMX実装ガイド(動的).md` §3.7（1435-1444）, §4 alert-history（1683-1701）, §5 OOB 一覧（1705-1717）。
- **Findings**:
  - §4 操作表（1688-1692）: 初期表示=フルページ `AlertHistory.templ`、検索/ページ送り=HTMX `hx-get` → `#alert-history-list` を **innerHTML** swap、ページ送りは `<a href="?page=N">` + `hx-boost`。
  - §5（1710-1717）: OOB 同時更新の対象は `/alerts/check`（ダッシュボードのしきい値チェック）のみ。alert-history **画面自体は OOB 非対象**。「本プロジェクトでは OOB 使用は最小限」と明記。
  - §3.7（1440）は `alert-history-pagination` を OOB と記すが、これは §5 と矛盾。§5「OOB 最小限」と既存 readings 実装（`Pagination` を結果領域 fragment に内包し単一 innerHTML swap）が優先される。
- **Implications**: alert-history は **OOB を使わず**、`#alert-history-list` 一つを innerHTML swap し、その中に一覧テーブル＋ページネーションを内包する（readings 完全踏襲）。→ Decision「OOB 不採用」を確定。

### 写経元（readings）の構造確認
- **Sources**: `internal/handler/readings.go`, `internal/view/page/Readings.templ`, `internal/view/component/DeviceReadingsList.templ`, `internal/view/component/Pagination.templ`, `internal/view/{page,component}/views.go`。
- **Findings**:
  - フィルタフォームは結果領域 fragment の**外**（page templ 内）に配置 → swap 対象外 → 入力状態保持 & Tom Select 再初期化不要。
  - 再利用ヘルパ: `parseDateBounds`（日付→区間境界・format 検証・to は当日終端まで・未指定はセンチネル全期間）, `parsePage`（不正→1）, `totalPagesOf`（0件でも1頁・ceil overflow 対策）, `pageSize=20`, `formatActual`（`pgtype.Numeric→%.2f`, dashboard.go）, `jst`（device_show.go）, `timefmt.DateTimeMinuteJP`（"YYYY-MM-DD HH:MM"）。
  - render ヘルパ（§56.4）: 成功部分返却=`renderComponent`（200固定）、検証失敗の部分返却=`renderPage(c, 422, comp)`。GET フィルタ検証失敗は readings 流に **200+インラインエラー**（fragment 内 `.error-message`）。
  - view モデル struct は `internal/view/page/views.go` と `internal/view/component/views.go` に集約（新規 struct は既存 views.go に追記）。
  - 既存 `Pagination.templ` は **前/次のみ・番号リンク無し**かつ `hx-target="#device-readings-list"` 固定 → alert-history では再利用不可。

### データ層・表示メソッドの確定
- **Sources**: `internal/repository/alert_histories.sql.go`, `internal/repository/devices.sql.go`, `internal/domain/{metric,comparison_operator}.go`, `docs/database_snapshot/table_definitions.md`。
- **Findings**:
  - `ListAlertHistoriesPaginated`（Params: `UserID int64` / `DeviceID *int64` / `FromAt,ToAt pgtype.Timestamptz` / `OffsetN,LimitN int32`、Row: `Metric/Operator string` `Threshold/ActualValue pgtype.Numeric` `IsNotified bool` `TriggeredAt` `DeviceID` `DeviceName`）。WHERE 句に `d.user_id=$1` と `($2 IS NULL OR d.id=$2)`、`triggered_at BETWEEN $3 AND $4`、`ORDER BY triggered_at DESC`、`LIMIT/OFFSET`。**テナント分離はクエリに内在**。
  - `CountAlertHistoriesInRange`（Params: UserID / DeviceID *int64 / FromAt / ToAt）。
  - `ListDevicesByUser(ctx, userID int64) ([]Device, error)`。
  - 表示: operator は値そのものが記号（`">"` 等）→ そのまま表示（templ が `>`→`&gt;` 自動エスケープ）。`Metric(row.Metric).Label()`="温度"/"湿度"、`.Unit()`="℃"/"%"。
  - **migration 新規不要**（必要なカラム・索引はすべて実在）。

## Design Decisions（確定）

### Decision: OOB を使わず単一 innerHTML swap（readings 踏襲）
- **Alternatives**: (A) session-08 prompt の OOB（`AlertHistoryList`＋`AlertHistoryPagination` を `hx-swap-oob`）/ (B) `#alert-history-list` 1つを innerHTML swap しページネーションを内包。
- **Selected**: B。`#alert-history-list` 結果領域（一覧テーブル＋空状態＋ページネーション）を単一 innerHTML swap。
- **Rationale**: §5「OOB 最小限」・既存コードベースに OOB 使用例ゼロ・readings が同型問題を B で解決済み。R2.4/R3.4（全画面再読込なしの一覧＋ページャ更新）を満たす。
- **Trade-offs**: thead も毎回再描画（§3.7 の "tbody のみ" 記述とは差異）だが、readings 同様コストは無視可能で写経の一貫性を優先。

### Decision: 番号付きページネーション部品を新規作成（`AlertHistoryPagination.templ`）
- **Alternatives**: (B-1) 既存 `Pagination.templ` を番号対応へ汎用化 / (B-2) 新規部品。
- **Selected**: B-2。`hx-boost` + `hx-target="#alert-history-list"`、前へ/番号リンク（現在は active）/次へ。モック `alert-history.html`（番号 1,2,3 ＋前/次）を写経。
- **Rationale**: 既存 `Pagination` は `#device-readings-list` 固定かつ番号無し。汎用化は readings への回帰リスク。R3.1（番号・前・次）を満たすため番号リンクは必須。
- **Trade-offs**: ページャ実装の二重化。ただし View 契約（`Pages []PageLink` のスライス）にしておけば将来 readings 側も同契約へ寄せられる（interface を一般化・実装は最小）。
- **Follow-up**: ページ番号は現状 1..Last を列挙（IoT 小規模・モック準拠）。多数ページ時の窓化は View 契約を変えずに後日対応可。

### Decision: 期間検証失敗は 200＋インラインエラー（422 不採用）
- **Alternatives**: (A) session-08 が言及した 422＋fragment / (B) readings 流 200＋インライン（検索スキップ）。
- **Selected**: B。`from`/`to` の形式エラー（`parseDateBounds`）に加え、**両指定かつ from>to** のとき `errs["to"]` に「終了日は開始日以降の日付を指定してください」を積み、クエリを呼ばず空一覧＋インラインで 200。
- **Rationale**: GET フィルタフォームの定石（readings 先例）。§56.4 の 422 はミューテーションフォーム（alert-rules POST）向けで GET フィルタには不適。R7.1（矛盾条件で一覧確定しない）を満たす。
- **Trade-offs**: from>to の範囲検証は readings 共有の `parseDateBounds` に足すと readings が回帰するため、**alert-history ハンドラ内のローカル検証**として追加（共有関数は不変）。

### Decision: device_id はユーザースコープ・クエリで分離（RequireDeviceOwner 非経由）
- **Alternatives**: (A) クエリの `d.user_id` スコープに委ねる（非所有/不在→空） / (B) `authz.RequireDeviceOwner` で明示 404。
- **Selected**: A。`device_id` 空→nil（全デバイス）、非空かつ数値→`*int64` を渡す（クエリが user_id でスコープ＝非所有は空）。非数値→400。
- **Rationale**: 所有者チェックは sqlc クエリ WHERE に集約済み（structure.md ルール5「散在させない」の意図に合致）。空表示は「不在」と「非所有」を区別せず**列挙防止**にもなり R8.3 を自然充足。device select は本人デバイスのみ提示（R5.3）のため通常は非所有 id を選べない。
- **Trade-offs**: alert-rules の 404 とは挙動差（あちらは単一デバイス前提の必須選択のため妥当）。

## Synthesis 結果
- **Generalization**: フィルタ＋一覧＋ページネーションは readings と同一問題。View 契約（`*ListView`＋`PaginationView`相当）を踏襲し、ページネーションのみ番号対応の新契約 `PageLink` を導入（interface を一般化、実装は最小）。
- **Build vs Adopt**: 全面 Adopt（既存クエリ・domain メソッド・ヘルパ・レイアウト・Tom Select グローバル初期化）。新規ビルドは Web UI 4ファイル＋ルート1本のみ。
- **Simplification**: 共有 `DeviceSelectOption.templ` は新設せず page templ にインライン（alert-rules も device option をインライン。投機的な再利用部品化を回避）。migration/新規クエリ無し。
