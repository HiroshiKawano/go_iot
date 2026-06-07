# ギャップ分析: dashboard

> 作成: 2026-06-07 / 対象: `.kiro/specs/dashboard/requirements.md`（R1–R6）
> 目的: 既存コードベース（BE 完成・S1 実装済み）と要件の差分を洗い出し、design フェーズの判断材料を提供する。

## 分析サマリ

- **本機能は「新規構築」ではなく「S1 プレースホルダの置換 + 拡張」**。GET `/dashboard` ルート・`RequireAuth`・`AuthHandler.Dashboard`・`page.DashboardPage` テンプレ・`App` レイアウト・未認証302テストは S1（web-foundation-auth）で**実装済み**。本セッションはハンドラ中身と templ を正規版へ差し替え、テストを拡張する。
- **3つの sqlc クエリ・domain Enum・pgtype 変換ヘルパは実在**し、そのまま再利用可能。データ取得層の新規実装は不要（クエリ追加なし）。
- **spec-init プロンプトの型名・シグネチャ前提に複数の乖離**がある（`domain.Device` 等の型は存在せず実体は `repository.*`、`GetLatestSensorReading` は nil でなく `pgx.ErrNoRows`、アラートクエリは `Params{UserID, Limit}`）。design で View-model 形・エラー振り分け・LIMIT 値を確定する必要がある。
- **アラート表記がモックと domain で不整合**（モック「を超えました」 vs `ComparisonOperator.Label()`「より大きい」）。要件 R4.2 の例文と R4.4 の共通ルールが衝突するため、design で表記方式を1つに決める（**要設計判断**）。
- **相対時間フォーマッタは未実装**。`go-humanize` は間接依存かつ英語表記のため、日本語「N分前」は hand-made ヘルパが妥当（新規 deps 回避・プロジェクト方針整合）。
- 総合: **Effort S（1–3日）/ Risk Low**。既存パターンの踏襲・クエリ既存・モック写経で、未知技術や新規統合はない。

## 1. 現状調査（既存資産）

### 再利用できる既存資産

| 種別 | 資産 | 場所 | 備考 |
|---|---|---|---|
| ルート | `web.GET("/dashboard", RequireAuth(), authH.Dashboard)` | `cmd/server/main.go:140` | 配線済み。本セッションで変更不要 |
| ハンドラ | `AuthHandler.Dashboard(c)` | `internal/handler/auth.go:182` | 現状は `GetUser` のみ。**ここに3クエリを追加** |
| テンプレ | `page.DashboardPage(layout.AppLayoutData)` | `internal/view/page/Dashboard.templ` | 空状態プレースホルダ。**正規版へ置換** |
| レイアウト | `layout.App(data){ children... }` | `internal/view/layout/App.templ` | `@layout.App(data){ ... }` の children パターン確立済み。踏襲する |
| 描画ヘルパ | `renderPage(c, status, comp)` / `renderError(c, status)` | `internal/handler/auth.go:206,213` | templ→HTML 描画 / 機密非漏洩500。再利用 |
| クエリ① | `ListDevicesByUser(ctx, userID) ([]Device, error)` | `internal/repository/querier.go:41` | `WHERE deleted_at IS NULL ORDER BY created_at DESC` |
| クエリ② | `GetLatestSensorReading(ctx, deviceID) (SensorReading, error)` | `internal/repository/querier.go:29` | **`:one` → 無データ時 `pgx.ErrNoRows`** |
| クエリ③ | `ListUnnotifiedAlertHistoriesWithDevice(ctx, Params{UserID, Limit}) ([]Row, error)` | `internal/repository/querier.go:50` | JOIN で `device_name` 取得済。**`LIMIT $2` 必須** |
| Enum | `Metric.Label()/Unit()` `ComparisonOperator.Label()` | `internal/domain/{metric,comparison_operator}.go` | 表示用メソッド既存（view→domain は許可） |
| 型変換 | `pgconv.NumericToFloat(n)` / `TimestamptzToTime(t)` | `internal/infra/pgconv/pgconv.go:25,43` | `pgtype.Numeric/Timestamptz` → Go 型 |
| CSS/モック | `mocks/html/dashboard.html` / `style.css`（写経元・CSS 正本） | `mocks/html/` | 全要素・クラス・文言の正 |

### 規約・パターン（structure.md / tech.md）

- **依存方向**: `handler → repository(Querier)`。view は handler が渡す表示用データのみ描画（view→repository/service 禁止）。view→domain の表示メソッド呼び出しは許可。
- **DB ポート = `repository.Querier`**。handler は `Querier` interface に依存（テスト時に手書きモックへ差替可能）。
- **templ はモック写経**。構造・クラス・文言をモックからそのまま写し、足すのは `id`・`hx-*`・`for`/`if` のみ。独自クラス新設禁止。
- **テスト**: templ は `Render→bytes.Buffer→strings.Contains`。ハンドラは httptest+gin + Querier モック。既存 `auth_test.go`/`auth_extra_test.go`/`view/*_test.go` の土台に追加。

## 2. 要件→資産マップ（ギャップ・タグ付き）

| 要件 | 必要能力 | 既存資産 | ギャップ |
|---|---|---|---|
| R1.1 200フルページ | `Dashboard` handler + `DashboardPage` templ + `App` | 全て既存 | **Constraint**: 既存関数を拡張（後述 Option） |
| R1.2 未認証302 /login | `RequireAuth` | 実装済・テスト通過 | なし（再利用） |
| R1.3 本人データのみ | `ListDevicesByUser(userID)` 等が user_id 絞り込み済 | 既存（クエリが user スコープ） | なし。`auth.UserID(c)` で取得 |
| R2.1–2.2 カード一覧 | デバイス配列＋カードcomponent | `ListDevicesByUser` 既存／`DeviceCard.templ` **未** | **Missing**: `DeviceCard` templ 新規 |
| R2.3 稼働/停止表記 | `is_active`→「● 稼働中/○ 停止中」 | `Device.IsActive bool` 既存 | なし（templ 内 if） |
| R2.4 デバイス0件 | 空メッセージ（モック全文） | モックに文言あり | なし（写経） |
| R3.1 温湿度2桁 | `pgtype.Numeric`→`%.2f`+単位 | `pgconv.NumericToFloat` 既存 | **Constraint**: View-model で float/整形文字列化 |
| R3.2 計測未受信→「ー」 | `GetLatestSensorReading` の無データ判定 | **`:one`→`pgx.ErrNoRows`** | **Unknown→設計**: ErrNoRows を正常系（「ー」）、他errを500へ振り分け |
| R3.3 相対時間 | 日本語「N分前」フォーマッタ | **未実装**（go-humanize は間接＆英語） | **Missing**: hand-made ヘルパ |
| R3.4 通信実績なし→「通信実績なし」 | `last_communicated_at` の NULL 判定 | `pgtype.Timestamptz.Valid` 既存 | なし（`.Valid` 分岐） |
| R4.1–4.3 アラート一覧/0件 | 未通知アラート配列＋バナーcomponent | `ListUnnotified...` 既存／`UnhandledAlertBanner.templ` **未** | **Missing**: バナー templ 新規。**Unknown→設計**: `Limit` 値 |
| R4.2 アラート文言 | 「{デバイス}: {指標}が{閾値}{条件}（{実測値}）」 | `Metric.Label/Unit`・`Operator.Label` 既存 | **Constraint（要注意）**: モック文言と Label() が不一致（下記） |
| R4.4 指標/演算子 表示ルール | domain Enum 準拠 | 既存 | なし |
| R5.1–5.3 遷移リンク | `/devices/create`・`/devices/{id}` href | モックに href あり | なし（写経。遷移先は別S所有） |
| R6.1 取得失敗→500 | エラー振り分け | `renderError` 既存 | **Unknown→設計**: 3クエリ各経路の err 分類（特に R3.2 の ErrNoRows 除外） |
| R6.2 0件は正常系 | 空メッセージ | 既存 | なし |
| R6.3 自動更新なし | 初期描画のみ | — | なし（HTMX ポーリング非導入） |

### 要設計判断として明示する乖離（spec-init プロンプト前提との差分）

1. **型名の乖離（Constraint）**: プロンプト props の `domain.Device` / `domain.SensorReadingRow` / `domain.UnnotifiedAlertRow` は**存在しない**。実体は `repository.Device` / `repository.SensorReading` / `repository.ListUnnotifiedAlertHistoriesWithDeviceRow`（いずれも `pgtype.*` 混在）。design で「repo 構造体を直接 templ へ渡す」か「handler で表示用 View-model に整形して渡す」かを決める。pgtype を templ に持ち込むと view 側で変換が必要になり「view は描画のみ」規約に擦れるため、**handler で整形（float/整形済み文字列・相対時間文字列・bool）した View-model 推奨**。
2. **関数名の乖離（Constraint）**: 既存は `page.DashboardPage(layout.AppLayoutData)`。正規版は引数を拡張する（`DashboardPage(layout.AppLayoutData, view DashboardView)` 等）か、`page.DashboardView` を新設して受け取る。命名は既存 `LoginView`/`RegisterView`（`views.go`）に倣う。
3. **`GetLatestSensorReading` の戻り（Unknown→設計）**: プロンプトは「nil なら未受信」と書くが、実体は `(SensorReading, error)` で無データ時 `pgx.ErrNoRows`。handler は **ErrNoRows=未受信（正常・「ー」）／他=500** に分岐する。`*SensorReading`（nil 可）へ正規化する薄いラッパを handler 側に置くのが素直。
4. **アラートバナーの `Limit`（Unknown→設計）**: `ListUnnotifiedAlertHistoriesWithDevice` は `Params{UserID, Limit}`。バナー表示の上限件数を決める（例: 全件相当の十分大きい値 / 直近 N 件 + 「他 M 件」表示）。要件は「1件以上をリスト表示」のため、**上限を設けるなら silent truncation を避け表示方針を design で確定**。
5. **アラート表記の不整合（Constraint・要注意）**: モック `dashboard.html` は「温度が**35℃を超えました** (38.50℃)」「湿度が**30%を下回りました** (25.00%)」。一方 `ComparisonOperator.Label()` は「より大きい/より小さい/以上/以下」。要件 R4.2（例=モック文言）と R4.4（共通ルール=Label()）が**表記衝突**する。design で次のいずれかに確定:
   - (a) 既存 `Label()` で構成 → 「温度が35℃より大きい（38.50℃）」（モックと文言差・実装最小）
   - (b) `ComparisonOperator` に過去形/閾値超過向け表示メソッドを追加（例 `ExceedancePhrase()` → 「を超えました/を下回りました」）→ モック準拠だが domain 拡張
   - (c) モック側を `Label()` 表記へ改訂（写経の単一ソース維持）
   - ※ 閾値の桁: モックは「35℃」（整数）だが `threshold`/`actual_value` は `numeric(5,2)`。実測値は小数2桁（38.50）。閾値の桁表記も (a)–(c) と併せて決める。

## 3. 実装アプローチ

### Option A: 既存 `Dashboard` ハンドラ／`DashboardPage` テンプレを拡張（推奨）
- **対象**: `internal/handler/auth.go` の `Dashboard` に3クエリ追加。`Dashboard.templ` を正規版へ。`page/views.go` に `DashboardView` 追加。`component/{DeviceCard,UnhandledAlertBanner}.templ` 新規。相対時間ヘルパ新規。
- **トレードオフ**: ✅ ルート/認証/描画/テスト土台を全再利用、最短。✅ 既存 `LoginView`/`RegisterView` パターンと一貫。❌ `auth.go` がやや肥大（ただし Dashboard は1関数で収まる規模）。
- **適合**: 本機能は S1 の続きであり、関数群が既に Dashboard 用に用意済みのため自然。

### Option B: Dashboard 専用ハンドラ/パッケージを新設
- **対象**: `internal/handler/dashboard.go`（`DashboardHandler`）を切り出し、`auth.go` の `Dashboard` を移設。
- **トレードオフ**: ✅ 認証フローとダッシュボード表示の責務分離。❌ 既存ルート配線・テスト（`auth_test.go` の `/dashboard` 系）を移設する波及。❌ 規模（全9画面・個人開発）に対し過剰分割。
- **適合**: 将来デバイス/履歴ハンドラが増える前提なら布石になるが、本セッション単体では over-engineering。

### Option C: ハイブリッド（Aで実装 + domain/ヘルパのみ切り出し）
- **対象**: 表示ロジックは Option A、ただし「相対時間フォーマッタ」と（採用時）「アラート文言メソッド」は再利用前提で独立配置（`internal/view` 配下のヘルパ or `domain`）。
- **トレードオフ**: ✅ 相対時間・文言は S5/S6/S8 でも再利用が見込まれるため独立化の価値。✅ A の最短性を保ちつつ将来の重複を予防。❌ 配置先（view ヘルパ vs domain）の判断が要る。
- **適合**: **本命**。本体は A、再利用見込みのヘルパだけ C で独立化。

## 4. Effort & Risk

- **Effort: S（1–3日）** — 既存クエリ・Enum・変換ヘルパ・レイアウト・テスト土台が揃い、新規はテンプレ2本＋相対時間ヘルパ＋handler 拡張のみ。モック写経で HTML/CSS の発明不要。
- **Risk: Low** — 未知技術なし・新規 deps 原則なし・統合点は確立済み。唯一の不確実性は「アラート表記方式（5項）」だが UI 文言判断であり技術リスクではない。

## 5. design フェーズへの申し送り

### 推奨アプローチ
- **Option C（本体 A + ヘルパ独立化）**。handler で `DashboardView`（表示用 primitive）に整形して templ へ渡し、`pgtype` を view に持ち込まない。

### 確定すべき設計判断（Research/Decision items）
1. **View-model の形**: `page.DashboardView`（デバイス行・各行の整形済み温湿度文字列 or float＋相対時間文字列＋未受信フラグ、アラート行の整形済み文言）の具体構造。`*SensorReading`（nil 可）正規化の置き場所。
2. **アラート表記方式 (a)/(b)/(c)** と閾値の桁表記（5項）。モック単一ソース原則との整合（(c) ならモック改訂が必要）。
3. **相対時間フォーマッタ**: hand-made 採用可否、配置（`internal/view` ヘルパ等）、粒度（秒/分/時間/日）と境界、未来時刻の扱い。
4. **アラートバナー `Limit` 値**と truncation 表示方針。
5. **エラー振り分け表**: 3クエリ × (ErrNoRows / その他 err) → (正常表示 / 500) の対応。特に `GetLatestSensorReading` の ErrNoRows を 500 にしないこと。
6. **テスト設計**: 既存 Querier モック土台で `ListDevicesByUser`/`GetLatestSensorReading`/`ListUnnotified...` をスタブし、正常（デバイス＋アラート複数）・0件×2・計測未受信（ErrNoRows→「ー」）・通信実績なし・DBエラー500 を網羅（80%）。テンプレは `Render→buffer→strings.Contains` で id（`unhandled-alert-banner`/`device-grid`/`device-card-{id}`）・文言検証。
7. **HTMX 参照**: design 時に `HTMX実装ガイド(動的).md` §3.2(id一覧)・§4(dashboard)・§5(OOB)・§6(HTMX未使用) を確認。本セッションは初期描画のみだが、`unhandled-alert-banner`/`device-grid` の id は将来 OOB ターゲットになるため命名を合わせる。

### 持ち越す調査項目
- なし（クエリ・型・変換は確認済み）。残る不確実性は上記「設計判断」で、いずれも UI/構造の意思決定であり外部調査不要。

---

# Discovery 所見（design フェーズ・light discovery）

> 追記: 2026-06-07 / `/kiro-spec-design dashboard -y`。拡張機能（S1 プレースホルダ置換）のため light discovery を実施。

## 参照した正典と確定事項

### (A) HTMX実装ガイド(動的).md（索引→該当節に限定）
- **§3.2 id一覧**: `device-grid`（`.device-grid` に付与・0件でもラッパー残置 R03）／`unhandled-alert-banner`（`.alert-banner-wrapper` に付与）／`device-card-{id}`（個別カード・主キー埋め込み）。
- **§4 dashboard**: 初期描画のみ。デバイスカード・未対応アラートは「表示中心」。ポーリング自動更新は MVP 非導入（`hx-trigger="every 30s"` 禁止）。0件は `.empty-message`。
- **§5 OOB**: 稼働切替 `/devices/{device}/toggle`・しきい値チェック `/alerts/check` 実行時にバナーを OOB 更新する将来構想。**本セッション対象外（S4/S7 が所有）**。なお §5 はバナー id を `unread-alert-banner` と記すが、§3.2・モック・session-03 は `unhandled-alert-banner`。**`unhandled-alert-banner` を正とする**（§5 の表記揺れは将来 OOB 実装時に §3.2 へ合わせる）。
- **§6 HTMX未使用**: dashboard は明示的に HTMX 未使用画面。本セッションは hx-* を一切付けない（id は将来 OOB ターゲット用に付与のみ）。
- **§31/§40-B**: モッククラスそのまま写経・独自クラス新設禁止／CSS は `mocks/html/style.css` が正本（本番は `make sync-css` 生成物）。

### (B) DBスキーマ現状（table_definitions.md）
- 使用テーブル: `devices`（location 可NULL・is_active・last_communicated_at 可NULL）／`sensor_readings`（temperature/humidity numeric(5,2)・recorded_at）／`alert_histories`（is_notified・metric/operator/threshold/actual_value・triggered_at）＋ JOIN 経由 `alert_rules`/`devices`。新規カラム/型/テーブルは不要（migration 追加なし）。
- 生成構造体の型: `Device.Location *string`・`*.LastCommunicatedAt/RecordedAt pgtype.Timestamptz`・`*.Temperature/Humidity/Threshold/ActualValue pgtype.Numeric`・`Row.Metric/Operator string`。

### (C) テストガイダンス集（Testing Strategy 導出）
- 採用定石: 手書き `fakeAuthRepo`（consumer interface 実装）で DB 非依存、`httptest`+gin、templ は `Render→bytes.Buffer→strings.Contains`、`RequireAuth` 経由の 302、決定的テスト（`now` を引数注入）、カバレッジ80%設計（成功・0件×2・ErrNoRows・DBエラー500）。

## 確定した設計判断（research item 1–7 への回答）

1. **View-model の形**: `page.DashboardView{ Layout, Devices[], Alerts[] }`（plain 表示型・repository/pgtype を import しない）。handler が repo 行→view-model に整形（view 純粋性維持）。
2. **アラート表記**: モック準拠の自然文をハンドラ helper `composeAlertMessage` で合成（`{デバイス名}: {Metric.Label()}が{閾値}{Unit()}{を超えました/を下回りました}（{実測値}{Unit()}）`）。演算子→動詞は `> >=`→「を超えました」/`< <=`→「を下回りました」。**domain には新メソッドを足さない**（この自然文は dashboard 固有。S8 履歴は記号表記 `>35.00℃` で別物 §4 alert-history）。閾値は末尾ゼロ trim（`35` ）・実測値は `%.2f`（`38.50`）でモックの桁感に一致。per-item の ⚠ は付けない（モックは h2 のみ）。
3. **相対時間**: hand-made `internal/timefmt.RelativeJP(t, now time.Time) string`（新規 deps 回避・決定的テスト用に now 注入）。粒度: <1分「たった今」/<1時間「N分前」/<24時間「N時間前」/それ以上「N日前」。未来時刻は「たった今」。`go-humanize` は英語のため不採用。
4. **アラートバナー Limit**: `const dashboardAlertLimit = 50`（triggered_at DESC）。超過時は最新50件のみ（明示的キャップ・将来「他N件」は範囲外）。
5. **エラー振り分け**: `GetUser`/`ListDevicesByUser`/`ListUnnotifiedAlertHistoriesWithDevice` の err は 500。`GetLatestSensorReading` のみ `errors.Is(err, pgx.ErrNoRows)`=未受信（正常・温湿度「ー」）、それ以外の err は 500。:many クエリの 0 行は空スライス（正常）。
6. **配置**: Dashboard は既存 `AuthHandler`（S1 doc が dashboard を所有）に残し、ロジックは新ファイル `internal/handler/dashboard.go` へ分離（auth.go の肥大回避）。`AuthRepo` interface に dashboard 用 3 クエリを追加（`q`=Querier が満たすため main.go 変更不要）。将来のハンドラ分割（devices/readings 等）は本境界外。
7. **id 命名**: `device-grid`/`unhandled-alert-banner`/`device-card-{id}`（§3.2）。将来 OOB のターゲットとして付与のみ（本セッションは hx-* なし）。

## Synthesis（3レンズ）
- **Generalization**: 温湿度の数値整形・相対時間は S5（device-show）・S6（readings）でも再利用見込み → `timefmt` を独立パッケージ化、数値整形は `pgconv.NumericToFloat`+`fmt` の薄い helper に留める（過剰一般化しない）。
- **Build vs Adopt**: 相対時間は go-humanize（英語・間接依存）を**不採用**、20行程度の hand-made を **build**（日本語粒度の制御性・決定的テスト・deps 非増加）。pgtype 変換は既存 `pgconv` を **adopt**。
- **Simplification**: 新規ハンドラ struct（DashboardHandler）は作らず既存 AuthHandler 拡張で十分（interface は1実装）。OOB 分割テンプレ関数は本セッションでは作らない（将来 S4/S7）。N+1（デバイス毎 GetLatestSensorReading）は小規模前提で許容、バッチ化は将来課題。

## 実装時の追補（2026-06-07）
- **View-model の配置変更（循環 import 回避）**: design 当初は `DashboardView`/`DashboardDevice`/`DashboardAlert` を全て `page` に置く想定だったが、`page.DashboardPage` が `component.DeviceCard`/`UnhandledAlertBanner` を描画（page→component）し、その部品が1件分 DTO を引数に取る（component→page）と循環する。実装では1件分 DTO（`DashboardDevice`/`DashboardAlert`）を `component` へ移設し、`DashboardView` は `page` に残した（詳細は design.md「View-model」節）。
- **アラート文言の括弧**: モックは半角 `(…)` だったが、design/tasks の合成書式（全角 `（…）`・スペースなし）を正準とし、モック・要件例文も全角へ統一した（`composeAlertMessage` 出力と完全一致）。
