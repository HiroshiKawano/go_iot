# ギャップ分析: sensor-readings-history

> 生成日: 2026-06-08 / 対象: `.kiro/specs/sensor-readings-history/requirements.md`
> 種別: ブラウンフィールド（バックエンド完成・S1基盤実装済み・S5 device-detail 実装済み）

## 調査サマリ

- **基盤は完備**: S1（middleware: SessionLoad/CSRF/MethodOverride/RequireAuth、auth.UserID、authz.RequireDeviceOwner）と S5（DeviceHandler/DeviceRepo、LatestReadingsTable、共通レンダラ）が実装済み。本機能は既存パターンの素直な横展開で実装できる。
- **sqlc クエリは3本とも実装済み**: `ListSensorReadingsPaginated` / `GetSensorReadingsSummary` / `CountSensorReadingsInRange`。ただし**3本とも `recorded_at BETWEEN $2 AND $3` で from/to が必須**であり、要件「日付未指定＝全期間」とのギャップを Handler 側のセンチネル境界で吸収する必要がある（最重要論点）。
- **「もっと見る」導線は配線待ち**: `LatestReadingsTable` のリンクが既に `/devices/{id}/readings` を指す。ルート追加とハンドラ実装で結線完了。
- **新規が必要なのは3点**: ① Readings ハンドラ（`ReadingsHandler.Index` 相当）、② view 3点（`Readings.templ` / `DeviceReadingsList.templ` / 初の再利用 `Pagination.templ`）、③ GET 検索型バリデーション（形式エラー→200+インライン、§7/TL14。既存フォームの 422 とは別系統）。
- **見積り**: 全体 **M（3〜7日）** / リスク **Low〜Medium**。新規ロジックの肝は「BETWEEN 用の日付境界（未指定補完・終端の end-of-day・JST）」「0件時の集計 NULL 表示」「ページ番号ウィンドウUI」の3点。

---

## 1. 現状調査（再利用可能な既存資産）

### 1.1 基盤（S1・そのまま利用）

| 資産 | 所在 | 用途 |
|------|------|------|
| `middleware.SessionLoad / CSRF / MethodOverride / RequireAuth` | `internal/middleware/` | web グループに適用済み。GET 経路は `RequireAuth()` を個別付与 |
| `auth.UserID(c)` | `internal/auth/session_auth.go` | ログインユーザーID取得 |
| `authz.RequireDeviceOwner(ctx, repo, id, uid)` | `internal/authz/ownership.go` | 所有者認可。`ErrUnauthenticated`/`pgx.ErrNoRows`/`ErrNotOwner` を返す。**閲覧系は不在/非所有とも 404（列挙防止）** |
| `DeviceGetter`（最小 consumer interface） | 同上 | `GetDevice` のみ要求。新ハンドラの repo もこれを満たせる |

### 1.2 ハンドラ層パターン（S5・写経元）

- `internal/handler/device_show.go` が本機能に最も近い実装例（デバイス詳細＝デバイス＋最新10件＋期間グラフ）。
  - `c.Param("device")` → `strconv.ParseInt` → 非数値は `renderError(c, 400)`
  - `authz.RequireDeviceOwner` → `renderDeviceReadError`（不在/非所有 404）
  - `h.Repo.GetUser` でレイアウト用ユーザー名取得
  - `HX-Request` ヘッダ有無でフラグメント/フルページ分岐（`Chart` メソッドが部分返却例）
- **共通レンダラ（同一 package handler のため直接再利用可）**: `renderPage` / `renderComponent` / `renderError`（`auth.go`）、`formatActual`（`dashboard.go`、numeric→小数2桁文字列）。
- **package-level `var jst`**（`device_show.go`、JST FixedZone）も同パッケージなので再利用可。
- **`DeviceHandler{ Repo DeviceRepo }`** 構造。`DeviceRepo` は consumer interface で、既に `GetUser`/`GetDevice`/各 SensorReading 系メソッドを含む。

### 1.3 view 層（3層構成・写経元）

- `internal/view/page/`（フルページ）/ `component/`（部品・HTMX ターゲット）/ `layout/App.templ`（認証後レイアウト）。
- `component/LatestReadingsTable.templ` が `.data-table` テーブルの写経例（ただし3列・通信遅延なし、行 struct は `ReadingRow{RecordedAt,Temp,Humidity}` の3項目）。
- **Pagination コンポーネントは未存在**（本機能が初のページング画面 → 新規作成 `Pagination.templ`）。

### 1.4 変換・整形ヘルパ

| ヘルパ | 所在 | 用途 |
|--------|------|------|
| `pgconv.Timestamptz(t) / TimestamptzToTime(ts)` | `internal/infra/pgconv/` | time.Time ↔ pgtype.Timestamptz |
| `pgconv.NumericToFloat(n)` | 同上 | numeric→float |
| `formatActual(n)` | `handler/dashboard.go` | numeric→"28.50" 形式（温湿度表示） |
| `aggregateToFloat(v interface{})` | `handler/device_show.go` | **集計の interface{} 型（MAX/MIN）を安全に float 化**。Summary の Max/Min 表示で再利用可 |
| `timefmt.DateTimeMinuteJP(t)` | `internal/timefmt/` | "YYYY-MM-DD HH:MM"（計測日時表示） |

### 1.5 ルーティング（`cmd/server/main.go`）

```
web := engine.Group("/", middleware.SessionLoad(sm), middleware.CSRF(cfg))
deviceH := &handler.DeviceHandler{Repo: q}
web.GET("/devices/:device",        middleware.RequireAuth(), deviceH.Show)
web.GET("/devices/:device/chart",  middleware.RequireAuth(), deviceH.Chart)
...
```
- ルートノード `:device` は既に `/edit` `/chart` 等の子を持つ。`/devices/:device/readings` の追加は競合なし。
- 合成順: 外側 `MethodOverride` → `scs LoadAndSave` → engine。

---

## 2. Requirement-to-Asset Map（ギャップ標識）

| 要件 | 既存資産 | ギャップ | 標識 |
|------|----------|----------|------|
| R1 初期表示（全期間・最新20件） | `ListSensorReadingsPaginated`（BETWEEN）/ `renderPage` / App.templ | クエリが from/to 必須 → 未指定時の境界補完が必要。`Readings.templ` 新規 | **Constraint** + **Missing** |
| R1.4 当該デバイス限定 | `authz.RequireDeviceOwner`（device_id スコープ） | なし（そのまま） | OK |
| R2 期間フィルタ検索 | `ListSensorReadingsPaginated` | from/to の time.Parse＋JST＋終端 end-of-day 補正、`from>to`時の扱い | **Constraint** |
| R3 集計（平均/最高/最低） | `GetSensorReadingsSummary` / `aggregateToFloat` / `formatActual` | 0件時 AVG/MAX/MIN が NULL → 表示方針未定（"-"等） | **Unknown** |
| R3.3 集計は期間全体（ページ非依存） | Summary クエリは LIMIT/OFFSET 非依存 | なし（クエリ仕様が合致） | OK |
| R4 ページネーション（20件/総ページ） | `CountSensorReadingsInRange` / OFFSET=(page-1)*20 | `Pagination.templ` 新規。ページ番号ウィンドウUI仕様が未確定 | **Missing** + **Unknown** |
| R4.4 page 正規化（未指定/1未満/非数値→1） | — | Handler で `strconv.Atoi`＋下限クランプ | Missing(軽) |
| R5 通信遅延（整数秒・四捨五入） | SensorReading 行に `created_at`/`recorded_at` あり | `(created_at - recorded_at)` の秒丸め整形ヘルパが無い。行 struct に第4列 Delay 追加（既存 ReadingRow は3列） | **Missing** |
| R6 日付バリデーション（形式・GET検索型） | — | 形式エラー→**200+インライン+空一覧**（§7/TL14）。既存フォームの 422+binding とは別系統。`map[string]string` 引数渡し | **Missing** |
| R7 0件表示（テーブル hidden＋メッセージ） | LatestReadingsTable の `if len==0` パターン | Alpine の `x-show` ではなく Go 条件分岐で写経 | Missing(軽) |
| R8 部分更新＋URL状態 | device_show `Chart` の HX-Request 分岐 | `HX-Push-Url` ヘッダ付与＋ページリンク/フォームの hx 属性は新規 | **Missing** |

---

## 3. 実装アプローチ Options

### Option A: DeviceHandler を拡張（device_show.go と同居）

`DeviceHandler` に `Index` メソッドを追加し、`DeviceRepo` に Summary/Count の2メソッドを追記（Paginated 系は未追加なので実質 `ListSensorReadingsPaginated`/`GetSensorReadingsSummary`/`CountSensorReadingsInRange` を追加）。

- ✅ device_show.go が既に DeviceHandler を共有拡張した前例どおりで一貫。配線は `deviceH` 流用で最小。
- ✅ `jst`/`formatActual`/`aggregateToFloat`/レンダラを同 struct 内で自然に再利用。
- ❌ DeviceHandler / DeviceRepo がさらに肥大化（責務集中）。
- ❌ readings 固有テストが device テストと混在しやすい。

### Option B: ReadingsHandler を新設（session-06 プロンプト準拠・推奨）

`internal/handler/readings.go` に `ReadingsHandler{ Repo ReadingsRepo }` を新設。`ReadingsRepo` は consumer 最小 interface（`GetUser`/`GetDevice`/`ListSensorReadingsPaginated`/`GetSensorReadingsSummary`/`CountSensorReadingsInRange`）。

- ✅ 責務分離が明確・テスト隔離が容易（`readings_test.go` 単独）。session-06 の命名（`ReadingsHandler.Index`）とも一致。
- ✅ 同一 package handler のため `renderPage`/`renderComponent`/`renderError`/`formatActual`/`aggregateToFloat`/`jst` を**コード変更なしで再利用**。
- ✅ authz は `DeviceGetter`（GetDevice のみ）を満たすので流用可。
- ❌ `main.go` に `readingsH := &handler.ReadingsHandler{Repo: q}` の1行と GET 1経路の追加（軽微）。

### Option C: ハイブリッド（推奨実体）

実体としては **B（新ハンドラ）＋ 再利用最大化 ＋ Pagination は汎用 component として新設** が最良。

- ハンドラ＝B（新設・最小 interface）。
- 横断ヘルパ（レンダラ/整形/JST/authz/pgconv/timefmt/aggregateToFloat）＝既存を流用（新規作成しない）。
- view＝`Readings.templ`（page）/ `DeviceReadingsList.templ`（fragment, id=`device-readings-list`）/ `Pagination.templ`（汎用 component、現在ページ・総ページ・base URL を受け取り hx 属性付きリンク生成）を新規。
- 通信遅延整形・日付境界補完・GET 検索バリデーションは readings.go 内のローカル関数として実装（汎用化は2例目が出てから）。

**推奨: Option C（= B をベースに既存資産を最大流用）。**

---

## 4. Effort & Risk

| 区分 | 評価 | 根拠（一行） |
|------|------|------------|
| 全体 Effort | **M（3〜7日）** | 既存パターン横展開が主だが、Pagination 新設・日付境界補完・GET検索バリデーションの新規ロジックが3点ある |
| ハンドラ/ルーティング | S | device_show.go の写経＋GET 1経路追加 |
| view（page/fragment） | S | LatestReadingsTable 等の写経。モック readings.html 完成済み |
| **Pagination.templ（新規・汎用）** | M | 初の部品。ページ番号ウィンドウ/省略表示の仕様確定が必要 |
| **日付境界 + BETWEEN 補完ロジック** | M | 未指定補完・終端 end-of-day・JST 解釈・from>to。テスト要 |
| GET 検索バリデーション | S〜M | §7/TL14 の 200+インライン方式。既存 422 と別系統で新規 |
| リスク | **Low〜Medium** | 技術は既知・スコープ明確（Low）。ただし日付境界/集計NULL/ページUI に仕様判断が残る（Medium 要素） |

---

## 5. 設計フェーズへの申し送り（Research Needed / 設計判断）

1. **【最重要】BETWEEN 用の日付境界マッピング**（クエリ仕様 vs 要件のギャップ吸収）
   - `from` 未指定 → 下限センチネル（例: `0001-01-01` 相当 / 十分過去の固定時刻）。
   - `to` 未指定 → 上限（`time.Now()` か十分未来）。
   - **`to` 指定時の終端**: `time.Parse("2006-01-02", to)` は 00:00:00 を返すため、その日を含めるには **end-of-day（+24h 未満 or `to.AddDate(0,0,1)` 未満 / 23:59:59.999）** へ補正が必要。BETWEEN は両端含むため要注意。
   - **TZ**: ユーザー入力日付は JST 暦日とみなし `jst` で解釈してから `pgconv.Timestamptz`（recorded_at は timestamptz=instant）。
   - 設計の Boundary Commitments に「日付→区間」変換規約を明記する。

2. **0件期間の集計表示**: `GetSensorReadingsSummary` は 0 件で AVG/MAX/MIN が NULL（`pgtype.Numeric` invalid / interface{} nil）、`sample_count=0`。集計ボックスの表示値（"-" / "—" / "0.00" のいずれか）と R7（テーブル hidden）との整合方針を決める。`aggregateToFloat` は nil→0 にフォールバックする点に留意（0.00 と表示されうる）。

3. **ページネーション UI 仕様**（session-06 未確定事項）: 最大表示ページ数（current±N）・省略記号（`...`）の閾値・前へ/次への無効化条件。`Pagination.templ` の引数設計（CurrentPage/TotalPages/BaseURL/クエリ保持）に直結。`HTMX実装ガイド §10` を設計時に参照。

4. **`from > to` の扱い**: 要件 R2.4 は「0件扱い」。BETWEEN が自然に空を返すため特別なエラーにしない方針で良いか design で確定（バリデーションは形式のみ＝意味検証しない、と整合）。

5. **HX 属性とフォーム**: フィルタフォーム（method=GET→`hx-get` 昇華）・ページリンク（`hx-get`+`hx-target="#device-readings-list"`+`hx-swap="innerHTML"`）・`HX-Push-Url` ヘッダの付与方式。`HTMX実装ガイド §4 readings 操作仕様 / §7 / §10 / hx-push-url 方針`を design で参照。

6. **通信遅延の負値・桁**: クロックずれで `created_at < recorded_at`（負の遅延）になり得る場合の表示（0秒クランプ等）を design で一言決める。

7. **テスト方針**（design の Testing Strategy / tasks 粒度向け、`テストガイダンス集` 参照）: Querier 手書きモックで日付境界・page 正規化・通信遅延整形・0件・形式エラーを単体検証。templ は `Render→bytes.Buffer→strings.Contains`。httptest+gin で HX-Request 分岐（フル/フラグメント）と HX-Push-Url を検証。

---

## 設計ディスカバリ所見（design フェーズ追記・2026-06-08）

> Discovery 種別: **Extension**（既存 S1 基盤 + S5 パターンへの統合）。軽量ディスカバリ。
> 参照正典: `2cc_sdd/HTMX実装ガイド(動的).md` §3.5 / §4 readings / §7 GET検索バリデーション / §10・C05 ページネーション、`docs/database_snapshot/`、`2cc_sdd/テストガイダンス集.md` 索引。

### HTMX実装ガイドからの確定（design 判断の根拠）

- **§3.5 / §4 readings**: fragment id は `device-readings-list`（集計+一覧+ページネーションを内包する**フィルタ結果領域**）。`readings-summary`/`readings-pagination` は OOB 個別更新時のみの任意 id（本機能は単一メイン更新で十分＝**OOB 不使用**）。初期表示=フルページ `Readings.templ`、期間検索/ページ送り=fragment `DeviceReadingsList.templ`。
- **フィルタ form は結果領域の外**（`.filter-form` method=GET → `hx-get` + `hx-target="#device-readings-list"` + `hx-swap="innerHTML"` + `hx-push-url="true"`）。swap 領域外に置くことで入力値がブラウザ DOM 上に保持される。
- **§7 GET 検索画面のバリデーション**: HTML/HTMX は **200 + インラインエラー + 空一覧**（CRUD フォームの 422 とは別系統）。エラー（`map[string]string`）は templ へ**引数で明示渡し**（Go に共有バッグ無し）、**fragment（DeviceReadingsList）の内側**に表示（フルページのレイアウトバナーは fragment 応答に含まれないため）。本機能は from/to が任意のため「初期表示スキップ判定」は不要で、各日付を個別にパースしエラーを収集する。
- **§10・C05 ページネーション**: canonical は**簡易ページャ**（`前へ`(HasPrev時) / `N / M ページ` / `次へ`(HasNext時)）を自前 templ で生成。`.pagination` + `.btn btn-small btn-secondary`（既存クラス）。nav に `hx-boost="true"` + `hx-target="#device-readings-list"` + `hx-swap="innerHTML"`。ページャは fragment 内に含めて返す。
- **CSRF**: 本機能は GET のみ＝ CSRF トークン送信不要。App レイアウトの meta + `htmx:configRequest`（S1 既設）はミューテーション用で GET には無関係。`AppLayoutData.CSRFToken` は meta 用に `csrf.Token(c.Request)` を渡すだけ（device_show.go 同様）。

### Design Decisions（research テンプレ準拠）

#### Decision: ページネーション UI は簡易ページャを採用（番号ウィンドウ不採用）
- **Context**: session-06「未確定事項」でページ番号ウィンドウ/省略記号の要否が未決。モック readings.html は番号リンク（2,3,…,10）を持つ。
- **Alternatives**: (A) モック踏襲の番号ウィンドウ + 省略記号 / (B) ガイド C05 の簡易ページャ（前へ・「N/M」・次へ）。
- **Selected**: **B（簡易ページャ）**。
- **Rationale**: ガイド C05/§10 が canonical で「モックのページネーションは仮マークアップ」と明記。番号ウィンドウのアルゴリズム（current±N・省略閾値）を導入せずに済み、小規模・初速重視の方針に合致。既存 `.pagination`/`.btn` クラスのみで §31（独自クラス新設禁止）も満たす。
- **Trade-offs**: 任意ページへの直接ジャンプ UI は無い（前後送りのみ）。URL の `?page=N` 直打ちは可能なので機能要件 R4 は満たす。
- **Follow-up**: 将来番号ジャンプが必要になれば `PaginationView` を拡張。

#### Decision: 「日付未指定＝全期間」を BETWEEN 用センチネル境界で吸収
- **Context**: sqlc 3クエリは `recorded_at BETWEEN $2 AND $3`（from/to 必須）。要件は from/to 任意。
- **Selected**: Handler で `from` 未指定→**遠い過去センチネル**（`1970-01-01 JST`）、`to` 未指定→**遠い未来センチネル**（`9999-12-31 JST`）。`to` 指定時は **end-of-day**（`parsedTo + 24h - 1ns`）まで含める。日付文字列は `time.ParseInLocation("2006-01-02", v, jst)` で JST 暦日として解釈。3クエリ（List/Summary/Count）は同一 (from,to) 境界を共有。
- **Rationale**: クエリ改修なしで要件を満たす。BETWEEN が両端含むため end-of-day 補正で「to 当日」を漏らさない。now() 非依存のセンチネルでテストを安定化。
- **Trade-offs**: 1970 以前のデータは全期間検索から漏れる（IoT 開始 2026 のため実害なし）。
- **Follow-up**: 境界・end-of-day・JST のテストを単体で固める。

#### Decision: 0件期間の集計表示は「—」、ページは [1, totalPages] にクランプ
- **Context**: `GetSensorReadingsSummary` は 0 件で AVG/MAX/MIN が NULL（`sample_count=0`）。`aggregateToFloat` は nil→0 にフォールバックするため、素直に出すと「0.00℃」と誤表示。
- **Selected**: `sample_count == 0` のとき集計6項目を `—`（単位なし）で表示。ページは count 取得後 `totalPages = max(1, ceil(total/20))` を算出し、`page` を `[1, totalPages]` にクランプしてから OFFSET を計算。
- **Rationale**: 「データなし」を 0.00 と誤認させない。ページャの非現実的状態（例 5/1）を防ぐ。R4.4（未指定/1未満/非数値→1）と整合し、高すぎるページは最終ページを表示。
- **Trade-offs**: 高ページのクランプは要件に明記なしの UX 補強（R7 空状態の範囲内で許容）。

### Testing Strategy 所見（テストガイダンス集 索引より）
- DB 非依存: `ReadingsRepo` を**手書きモック**（Querier の最小サブセット）で差し替え。引数（from/to 境界・limit/offset）を captor で検証。
- HTTP: `httptest`+gin。`HX-Request` ヘッダ有無でフル/フラグメント分岐をアサート。templ は `Render→bytes.Buffer→strings.Contains`。
- 認可: 非所有/不在は 404（列挙防止）、非数値 ID は 400、DB エラーは 500。
- 純関数: 日付境界・page 正規化/クランプ・totalPages・通信遅延整形（四捨五入/負値クランプ）・集計フォーマット（NULL→—）を table-driven。
