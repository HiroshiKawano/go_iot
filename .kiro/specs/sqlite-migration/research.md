# ギャップ分析（research.md） — sqlite-migration

> 本書は requirements.md と既存コードベースの差分を分析し、design フェーズの意思決定を支援する。
> 本機能は session-09 プロンプト + 実現可能性調査レポート（CONDITIONAL_GO）が移行対象ファイルを既に詳細列挙している brownfield 移行のため、本分析の主眼は **プロンプトの主張を実コードで検証し、誤り・見落とし・新規ギャップを確定すること** にある。
> 検証日: 2026-06-08（3探索エージェント + 直接 grep による実コード確定）。

---

## 1. 調査サマリ（実コード検証の結論）

- **移行構造はプロンプト/レポート通り**: pgtype 結合は handler 30箇所/6ファイル + service 1箇所に集約され、view/page 層は **AST テストで pgtype 非依存が強制済み**（後述）。chart 層も stdlib のみ・pgtype-free を確認。`WithTx` 呼び出し元はゼロ。統合テスト fake は in-memory（docker 非依存）。→ 改修面は限定的という総合判定は妥当。
- **【設計フェーズで訂正】`pgx.ErrNoRows` の本体 *機能的* 置換サイトは 10箇所**（`errors.Is` 比較）でプロンプト通り。`internal/authz/ownership.go` の3箇所は **すべて doc コメント言及**（36/72/83行目・`//`）で、同ファイルは pgx を import せず GetDevice/GetAlertRule の error を透過するだけ → 機能置換は不要、コメント文言のみ sql.ErrNoRows へ更新（cosmetic）。テスト側の `errors.Is` / fake の ErrNoRows 返却は別途 sql.ErrNoRows へ要置換。〔前段の「14箇所・authz 欠落」は誤りだったため取消。〕
- **【要修正】テスト改修対象の正確な数**: pgconv/pgtype を直接参照するテストは **9ファイル**（プロンプトの「17ファイル」は過大）。ただし `pgx.ErrNoRows` 参照も含めた移行対象テストは **約16ファイル**。
- **【新規ギャップ】カバレッジ計測基盤が存在しない**: Makefile に test/coverage ターゲットが無く、80%基準の現状ベースラインも取得手段も未整備。R9.3（80%維持）を判定可能にするには計測の仕組み追加が前提。
- **主要リスクは型層の silent failure**: `device_show.go:328 aggregateToFloat` の型スイッチが新型化で dead 化し、default の 0 フォールバックで集計グラフが黙って平坦化する経路（R5.3）。Task 0 の生成型確定 + 集計列への明示 CAST + テストでの担保が肝。

---

## 2. プロンプト主張 vs 実コード（検証結果）

| 項目 | プロンプト主張 | 実コード（確定値） | 判定 |
|---|---|---|---|
| models.go の pgtype 出現 | 29箇所 | **28箇所**（Numeric×5 / Timestamptz×23） | ✅ ほぼ一致（±1） |
| handler の pgconv 呼び出し | 30箇所/6ファイル（device_show=8/sensor_api=7/dashboard=4/readings=4/alert_rule=4/alert_history=3） | **30箇所/6ファイル（完全一致）** + service/alert_evaluator.go×1 | ✅ 一致 |
| chart 層の pgtype 結合 | 非依存（handler 責務） | **非依存を確認**（svg.go の "pgconv" 出現はコメント例示のみ） | ✅ 一致 |
| pgconv.go の関数 | Numeric2/NumericToFloat/Timestamptz/TimestamptzToTime の4関数 | **4関数で一致** | ✅ 一致 |
| `pgx.ErrNoRows` 本体の機能置換 | 10箇所 | **10箇所（一致）**。errors.Is 比較は body 10 + test 4 = 14。`authz/ownership.go`×3 は doc コメントのみ（置換不要） | ✅ 一致（プロンプト正・前段訂正） |
| view 層 pgtype 非依存の強制 | AST テストで強制済み | **`view/page/views_test.go` に AST 走査テストが実在**（forbidden=[internal/repository, pgtype, jackc/pgx]） | ✅ 主張は正しい |
| pgconv/pgtype 参照テスト | 17ファイル | **9ファイル**（pgconv/pgtype 直接参照）。ErrNoRows 込みで約16ファイル | ⚠️ 数値要修正 |
| `WithTx(` 呼び出し元 | ゼロ | **ゼロを確認** | ✅ 一致 |
| modernc.org/sqlite | go.mod に indirect で存在 | **v1.46.1 indirect で確認**（direct 格上げ要） | ✅ 一致 |
| scs/sqlite3store | 未取得（go get 要） | **go.mod に不在を確認**（追加要） | ✅ 一致 |
| カバレッジ計測 | （言及なし） | **基盤なし**（Makefile に test ターゲット無し） | ⚠️ **新規ギャップ** |

### 2.1 `pgx.ErrNoRows` 本体の機能置換サイト（errors.Is 比較・10箇所・全件 sql.ErrNoRows へ）

```
internal/auth/device_auth.go:63
internal/handler/alert_rule.go:382
internal/handler/auth.go:86, :139
internal/handler/dashboard.go:73
internal/handler/device.go:92, :183, :279
internal/handler/device_show.go:344
internal/handler/sensor_api.go:105
```
- **`internal/authz/ownership.go`（36/72/83行）は doc コメントのみ**で機能コードではない（pgx を import せず error 透過）→ コメント文言を sql.ErrNoRows へ更新（cosmetic・任意）。
- テスト側の `pgx.ErrNoRows` 参照（fake の not-found 返却 + assertion の errors.Is）も別途 `sql.ErrNoRows` へ置換が必要: `cmd/server/integration_test.go`, `cmd/server/device_integration_test.go`, `internal/authz/ownership_test.go`, `internal/handler/auth_test.go`, `internal/handler/device_test.go`, `internal/auth/device_auth_test.go`, `internal/handler/{alert_rule,readings_handler,alert_history_handler,dashboard,sensor_api}_test.go`。

### 2.2 pgconv/pgtype 直接参照テスト（9ファイル・フィクスチャ書換対象）

```
internal/handler/dashboard_format_test.go     （pgconv.Numeric2 / pgconv.Timestamptz / pgtype.Timestamptz{} 直接生成）
internal/handler/device_show_test.go          （pgconv.Timestamptz / pgtype.Date{Time:..,Valid:true} 直接生成）
internal/handler/alert_rule_test.go           （pgconv.Numeric2(35.00) 等）
internal/handler/alert_history_test.go
internal/handler/alert_history_handler_test.go
internal/handler/dashboard_test.go
internal/handler/readings_test.go
internal/handler/readings_handler_test.go
internal/service/alert_evaluator_test.go      （pgconv.Numeric2 / pgconv.Timestamptz）
```

---

## 3. Requirement → Asset マップ（gap タグ付き）

> タグ: **[制約]** 既存構造/オーナー決定による制約 / **[欠落]** 新規に作る/追加する必要 / **[未確定]** design で要研究

| 要件 | 関係する既存アセット | 移行で生じる差分 | gap |
|---|---|---|---|
| **R1** ローカルファイル永続化 | `internal/infra/db/pool.go`（pgxpool, MaxConns=10/MinConns=2 等）, `internal/config/config.go`（DatabaseURL 必須検証） | pgxpool.Pool → `*sql.DB`、PRAGMA（WAL/busy_timeout/foreign_keys）、単一 writer 前提で接続数を絞る、DSN を file: 形式へ | [制約] SQLite 単一 writer / [未確定] DATABASE_URL 転用 vs 新キー DB_PATH |
| **R2** Web UI 等価性 | handler 6ファイル（30 pgconv 呼び出し）, `internal/repository/`（sqlc 生成・pgtype 28箇所）, `db/queries/*.sql`, `db/migrations/*.sql` | sqlc engine=sqlite で再生成 → 生成型が float64/time.Time/sql.Null*/json.RawMessage へ全面変化、handler を新型へ追従 | [制約] sqlc 生成物が起点 / [未確定] 生成型は Task 0 実走でのみ確定 |
| **R3** デバイス API + アラート判定 | `internal/handler/sensor_api.go`（pgconv×7, ErrNoRows×1）, `internal/service/alert_evaluator.go`（NumericToFloat / actualValueFor）, `internal/auth/device_auth.go`（ErrNoRows×1） | actualValueFor の pgtype.Numeric 戻り値を新型へ、ErrNoRows→sql.ErrNoRows | [制約] Bearer 認証フローは無変更 |
| **R4** 数値・日時忠実性 | `internal/infra/pgconv/pgconv.go`（4関数）, migrations の NUMERIC(5,2)/TIMESTAMPTZ | NUMERIC→**REAL**（float64・オーナー決定）、TIMESTAMPTZ→**DATETIME**（time.Time 維持・TEXT 宣言は不可）、pgconv を新型前提に再実装 | [制約] REAL 採用確定 / [未確定] modernc の CURRENT_TIMESTAMP（T無し）×datetime affinity の time.Time パース挙動 |
| **R5** 集計の正当性 | `db/queries/sensor_readings.sql`（DATE() ×3: L32/L44/L45）, `internal/handler/device_show.go:328 aggregateToFloat`, dashboard/readings の集計整形 | `DATE(recorded_at)` → `date(recorded_at,'+9 hours')`（**SELECT/GROUP BY/ORDER BY の3箇所を同一式に統一**）、MAX/MIN 集計列に明示 `CAST(... AS REAL)`、aggregateToFloat の silent 0 フォールバックを明示エラー化 | [制約] JST(+9h) 補正確定・二重補正禁止 / **最重要リスク**: silent 平坦化 |
| **R6** セッション永続性 | `internal/auth/session_auth.go`（pgxstore.New(pool), NewSessionManager(*pgxpool.Pool)）, `db/migrations/00007`（sessions: data BYTEA / expiry TIMESTAMPTZ）, go.mod | pgxstore → **sqlite3store**（go get 要）、NewSessionManager 引数を `*sql.DB` へ、00007 を `token TEXT PK / data BLOB / expiry REAL + index` へ作り替え（Login/Logout/UserIDFromSession ロジックは無変更流用可） | [欠落] scs/sqlite3store 依存追加 / [未確定] sqlite3store の `$1` placeholder + julianday() が modernc で通るか実機1回確認 |
| **R7** セキュリティ不変条件 | `internal/authz/ownership.go`（ErrNoRows×3 は **doc コメントのみ**）, 本体 errors.Is 10箇所, CSRF（gorilla/csrf）, Bearer（device_auth.go） | 本体 errors.Is 10箇所を sql.ErrNoRows へ、ownership.go はコメント文言のみ更新（cosmetic）。CSRF/Bearer は DB 非依存で無変更 | [制約] errors.Is 10箇所の機械置換（authz は機能変更なし） |
| **R8** 現場 CLI / シード | `cmd/gen-token/main.go`（pgxpool, pgtype.Timestamptz, json.RawMessage は既に DB 非依存）, `cmd/seed/main.go`（pgxpool, `TRUNCATE ... RESTART IDENTITY CASCADE`, pgtype ヘルパ numeric2/timestamptz） | `*sql.DB` 化、TRUNCATE → `DELETE FROM ...` + `DELETE FROM sqlite_sequence`、pgtype ヘルパを新型へ | [制約] gen-token は現場運用の重要 CLI（確実動作） |
| **R9** 品質ゲート | `Makefile`（goose dialect=postgres, up/down=docker-compose）, **test/coverage ターゲット無し**, `internal/dbsnapshot` + `cmd/db-snapshot`（pg_catalog 内省: pg_class/pg_attribute/pg_index/pg_indexes/pg_constraint） | goose dialect→sqlite3、dbsnapshot を sqlite_master + PRAGMA へ移植 | [欠落] **カバレッジ計測の仕組み（go test -cover + 基準）** / [未確定] dbsnapshot 移植の本セッション範囲（機能縮退許容） |

---

## 4. 実装アプローチの選択肢（design で決める論点）

> 大半はプロンプト/オーナー決定で確定済み。ここでは **まだ開いている設計判断** に絞って A/B/C を提示する。

### 論点1: pgconv（型変換ハブ）の去就
- **Option A（推奨）— 集約を1箇所に残す**: pgconv.go を新型（float64/time.Time/sql.Null*）前提に再実装し、handler は引き続き pgconv 経由。
  - ✅ 30箇所の呼び出し側の変更が最小（関数内部だけ差し替え）/ 整形ロジック（`%.2f`・JST 変換）の集約を維持 / 差分レビューが容易。
  - ❌ sqlc が既に float64/time.Time を返すなら変換関数が薄くなり冗長に見える箇所が出る。
- **Option B — pgconv 廃止・直アクセス化**: 生成型が primitive になるため変換不要箇所は handler で直接扱う。
  - ✅ 余計な往復を削減。 ❌ 30箇所すべてに触れる広い diff、整形ロジックの散在リスク（イミュータブル/集約方針に反する）。
- **Option C — ハイブリッド**: 表示整формат（`%.2f`・JST ラベル）は集約関数として残し、単純な値取り出しのみ直アクセス。
  - ✅ バランス。 ❌ 線引き基準を design で明文化しないと一貫性を欠く。

### 論点2: DATABASE_URL の扱い（R1）
- **Option A — DATABASE_URL を file: 形式へ転用**: config 変更を最小化。 ✅ .env/steering 整合が容易。 ❌ キー名と実体（ファイルパス）の乖離。
- **Option B — 新キー DB_PATH を導入**: 意味的に明確。 ❌ config 検証・.env.example・steering の同時更新。デスクトップ向け既定パス・必須env緩和は **S10 委譲**のため、本セッションは最小変更が望ましい → A 寄りを推奨。

### 論点3: dbsnapshot 移植の範囲（R9）
- **Option A（推奨）— 最小再生成手段の確保に留める**: pg_catalog 内省を sqlite_master + PRAGMA へ最低限移植。機能縮退（コメント/一部メタ欠落）を許容。 ✅ CLAUDE.md のスナップショット参照強制を満たしつつ工数最小。 ❌ 出力が現行よりやや簡素。
- **Option B — 完全移植**: PRAGMA table_info / index_list / foreign_key_list 等でフル対応。 ❌ 開発専用・本番非同梱ツールに工数過剰（優先度低）。

### 論点4: カバレッジ計測の新設（R9・新規ギャップ）
- **Option A（推奨）— Makefile に `test`/`cover` ターゲットを追加**: `go test -cover ./...` + カバレッジ閾値確認。移行前にベースラインを取得し、移行後の維持を判定可能にする。 ✅ R9.3 を検証可能化。 ❌ 新規タスク（小）。

---

## 5. Effort / Risk（作業ストリーム別）

| 作業ストリーム | Effort | Risk | 一言根拠 |
|---|---|---|---|
| Task 0: migrations + queries の SQLite 方言書換 + `sqlc generate` 実走（生成型確定ゲート） | M | Medium | 確立パターンだが生成型は実走でのみ確定。下流改修量がここで決まる |
| 接続層・配線（pool.go / main.go / config） | S | Low | `*sql.DB` 化は定石。health は interface 互換 |
| 型層（pgconv 再実装 + handler 30箇所 + ErrNoRows 14箇所） | M | Medium | **aggregateToFloat の silent 平坦化（R5.3）が唯一の高リスク経路**。authz/ownership.go の ErrNoRows を漏らさない |
| セッションストア（sqlite3store + 00007 作り替え + go get） | S–M | Medium | sqlite3store×modernc（julianday/placeholder）の実機検証が未確定 |
| テスト書換（pgconv/pgtype 9ファイル + ErrNoRows 系 約7ファイル） | M | Low–Medium | フィクスチャ機械置換が中心。AST テストは無改修で pass 継続 |
| 開発/運用 CLI（seed / gen-token） | S | Low | TRUNCATE→DELETE+sqlite_sequence、pgtype→新型 |
| dbsnapshot 移植（最小） | S | Low | 機能縮退許容。後回し可 |
| カバレッジ計測の新設 | S | Low | Makefile ターゲット追加（新規） |

**総合: Effort = M（3–7日）/ Risk = Medium**（実現可能性調査の CONDITIONAL_GO と整合）。構造的障壁はなく、リスクは「型確定の実走依存」と「集計 silent failure」に集中。

---

## 6. design フェーズへの引き継ぎ

### 推奨アプローチ（design の Boundary Commitments 候補）
1. **Task 0 を厳密ゲート化**: migrations/queries 書換 → `go tool sqlc generate` 実走 → **生成型を実機記録（NUMERIC/集計/DATE/RETURNING/CAST化 narg/部分INDEX）** してから下流に着手。
2. **型変換は集約維持（論点1=Option A）**: pgconv を新型前提に再実装し handler 呼び出し側の diff を最小化。
3. **silent failure の明示エラー化を設計に明記**: MAX/MIN 集計列へ `CAST(... AS REAL)`、aggregateToFloat の型スイッチを期待型に更新し default 0 フォールバックを廃止。テストで非平坦を担保（R5）。
4. **authz/ownership.go を ErrNoRows 移行スコープに明示追加**（プロンプト欠落分・セキュリティの要）。
5. **DATABASE_URL は最小変更で転用（論点2=Option A）**、デスクトップ向け既定パスは S10 へ明示委譲。
6. **カバレッジ計測ターゲットを Foundation タスクに含める**（移行前ベースライン取得 → 移行後 80% 維持判定）。

### Research Needed（design で要確認）
- **R-1**: `sqlc generate`（engine=sqlite）の **実生成型**。NUMERIC/AVG/MAX/MIN・DATE・`RETURNING *`・部分インデックス・CAST化 narg が float64/time.Time/sql.Null* 通りに出るか。想定外なら下流改修量が変動。
- **R-2**: **modernc の time.Time パース挙動**。`CURRENT_TIMESTAMP`（T無し表記）と datetime affinity 列の string/time.Time マッピングが、現行「UTC 保存 + JST 表示」と噛み合うか実機確認（R4）。
- **R-3**: **scs/sqlite3store × modernc** の実走確認。`$1` placeholder + `julianday()` で INSERT/SELECT/DELETE が通り、cleanup goroutine が WAL+busy_timeout で SQLITE_BUSY を起こさないか（R6）。
- **R-4**: `RETURNING *`（SQLite 3.35+ / modernc v1.46.1）の動作確認（全 CRUD クエリが依存）。
- **R-5**: 接続数方針。SQLite 単一 writer に対する `SetMaxOpenConns` の妥当値（現行 MaxConns=10 は不適切）。
- **R-6**: dbsnapshot の移植範囲（最小 vs 完全）の最終決定（論点3）。

### 確定済み（再議論不要）
- NUMERIC → **REAL（float64）**（オーナー決定 2026-06-08）。
- 日次バケット → **JST(+9h) 補正**・SELECT/GROUP BY/ORDER BY を同一式統一・消費側で二重補正しない（オーナー決定 2026-06-08）。
- ドライバ = **modernc.org/sqlite**（pure-Go・CGO 不要・driver 名 `sqlite`）。mattn/go-sqlite3 は不採用。
- 単一 .exe 化・起動時自動マイグレーション・ブラウザ自動オープン等は **S10 委譲**（本セッション対象外）。

---

# 設計フェーズ追記（Discovery / Synthesis / Decisions） — 2026-06-08

> kiro-spec-design（light discovery: Extension / Complex Integration）の所見。前段ギャップ分析を権威ある実現可能性調査レポートに突き合わせ、設計判断を確定した。

## Summary
- **Discovery Scope**: Complex Integration（DB ドライバ層・型層の総入れ替え。新規画面ゼロ）
- **Key Findings**:
  - 権威レポート §4「統合判定 CONDITIONAL_GO」を裏付け確認。**Task 0（sqlc SQLite codegen 実通過）を gate に置けば GO 可能**。構造的障壁なし。
  - 緩衝材を実機確認: **view/page は AST テスト（`view/page/views_test.go`）で pgtype 非依存を強制**、**chart/svg.go は stdlib のみ依存**、**WithTx 呼び出し元ゼロ**、**統合テスト fake は in-memory（docker 非依存）**。
  - **silent failure が唯一の高リスク**: `device_show.go:328 aggregateToFloat` の型スイッチが新型化で dead 化し default 0 でグラフが黙って平坦化。集計列への明示 CAST + 期待型化 + テスト担保で封じる。

## Research Log

### 権威レポート §3.A/§3.B/§3.C/§3.E/§4/§6 の確認
- **Sources Consulted**: `2cc_sdd/SQLite化・単一exe化_実現可能性調査.md`（§2 外部技術事実裏取り / §3 観点別 / §4 統合判定 / §6 scs sqlite3store CGO 反証）、`docs/database_snapshot/table_definitions.md`、sqlc Datatypes リファレンス、`.kiro/steering/tech.md`・`structure.md`。
- **Findings**:
  - sqlc SQLite 型マップ（裏取り済）: `numeric/decimal → float64 / *float64 / sql.NullFloat64`、`date/datetime/timestamp affinity 列 → time.Time / sql.NullTime`、`json/jsonb affinity → json.RawMessage`、`blob → []byte`、`COUNT(*) → int64`。**TEXT 宣言にすると string 化**するため datetime affinity を維持する。
  - scs/sqlite3store は `database/sql/fmt/log/time` のみ import の純Go・**ドライバ非依存**（`func New(db *sql.DB)`）。要求スキーマ `sessions(token TEXT PK, data BLOB NOT NULL, expiry REAL NOT NULL)` + expiry 索引。`julianday()` + `$1` placeholder を使う（modernc で動作見込み・実機1回確認）。
  - **レポート内の矛盾を直接 grep で解消**: `authz/ownership.go` の pgx.ErrNoRows×3 は doc コメントのみ（§3.A 訂正が正・§3.E finding が誤記）。本体機能置換は 10箇所で確定。
- **Implications**: NUMERIC→REAL は float64 直マップで pgconv の往復が薄くなる。datetime affinity 維持で time.Time 戦略を継続。session は sqlite3store 採用で自作不要。

### HTMX 実装ガイドの適用可否（本機能固有の判断）
- **Context**: cc-sdd フックが HTMX 実装ガイド参照を要求するが、本機能は新規画面ゼロ。
- **Findings**: 本機能は templ コンポーネント・HTMX 属性・ルーティング・バリデーション表示を **新設も変更もしない**（振る舞い等価が不変条件）。handler の戻り値整形（formatReadingText/aggregateToFloat 等）だけを新型へ追従し、view へ渡す ViewModel の **形は不変**（view/page AST テストが pgtype 非流入を保証）。
- **Implications**: HTMX 実装ガイドの個別節（§2/§4/§7/§8）は本設計に直接適用しない。唯一の制約は「handler→templ に渡す整形済み primitive の値・書式（`%.2f`・JST ラベル）を移行前と等価に保つ」こと。これを Testing Strategy（templ Render→strings.Contains で HTML 等価アサート）で担保する。

## Design Synthesis

### 1. Generalization
- 9要件はすべて「**観測可能な振る舞いを保ったまま、永続化ドライバ層と型層を入れ替える**」という単一問題の変奏。一般化の核は **2つの安定シーム**: (a) DB ポート = sqlc 生成の `repository.Querier`（driver が変わっても handler/service/test は同 interface にモック）、(b) 型変換の集約点（pgtype→新型の整形を1箇所に閉じ込め、30呼び出し側の diff を内部化）。この2シームを保てば残りは「型再生成の自動波及」に還元される。

### 2. Build vs. Adopt
- **Adopt**: SQLite ドライバ=`modernc.org/sqlite`（既存・pure-Go）、セッションストア=`scs/sqlite3store`（純Go・要 go get）、マイグレーション=`goose`（dialect を sqlite3 へ・本セッションは CLI 継続）、数値=標準 `float64`（オーナー決定 REAL ＝ decimal ライブラリ不要）。
- **Build**: 新規構築はゼロ。型変換ハブのみ再実装（既存 pgconv の置換）。

### 3. Simplification
- **pgconv の去就（論点1の確定）**: sqlc が NUMERIC→float64・datetime→time.Time を直接生成すると、`Numeric2/NumericToFloat/Timestamptz/TimestamptzToTime` は実質 no-op 化する。**ただし NULL 許容列（deleted_at/last_communicated_at/expires_at/email_verified_at 等）は `*time.Time`/`sql.NullTime` で生成される**ため、「NULL→ゼロ値/表示文字列」の coalesce ロジックは残る。→ **pgconv を「NULL 合体 + 表示整形の最小ヘルパ」に痩せさせて 1 シーム維持**（削除して直アクセスにすると整形が散在し structure.md の集約方針に反する）。最終形（`*T` か `sql.NullT` か）は **Task 0 の生成型確定後に決定**（投機的に書かない）。
- 起動時 embed マイグレーション・自動オープン等の層は **作らない**（S10）。本セッションは手動マイグレーション前提に簡素化。

## Design Decisions

### Decision: Task 0 を「生成型確定ゲート」として全作業の前に固定
- **Alternatives**: (A) migrations/queries/handler/test を一括改修 / (B) Task 0 で codegen を通し生成型を実機確定してから下流着手。
- **Selected**: B。`go tool sqlc generate`（engine=sqlite）が通り、NUMERIC/集計(MAX/MIN/AVG)/DATE/RETURNING/部分INDEX/CAST化 narg の **生成型を記録**してから pgconv/handler/test へ進む。
- **Rationale**: 生成型（float64 か sql.NullFloat64 か）が下流改修量とヘルパ形を左右する（レポート §4.2 rank1）。`::` は SQLite parser がトークンを持たず codegen 自体が失敗するため、queries 書換は再生成の前提。
- **Trade-offs**: 直列性が増すが手戻りを根絶。

### Decision: silent 平坦化を「明示エラー化」で封じる（R5.3）
- **Selected**: MAX/MIN 集計列に明示 `CAST(... AS REAL)` を付け float64/sql.NullFloat64 に型確定。`aggregateToFloat` の型スイッチを **期待型のみ受理し、未知型は default 0 でなく明示エラー**へ。テストで非平坦を保証。
- **Rationale**: レポート §4.2 rank2 が指摘する最大の運用リスク（黙ってグラフ平坦化）を回帰テストで可視化。

### Decision: DATABASE_URL を SQLite ファイル DSN に最小転用（論点2）
- **Selected**: 既存 `DATABASE_URL`（必須 env）を `file:...sqlite?...` 形式に解釈変更。必須検証・既定パス・env 緩和は **S10 委譲**（本セッションは開発者が手動指定）。
- **Rationale**: 本セッションのスコープ最小化。新キー DB_PATH 導入は config/.env.example/steering 同時改修を招く。

### Decision: 接続は単一 writer 前提に絞る
- **Selected**: `sql.Open("sqlite", dsn)` + PRAGMA（journal_mode=WAL / busy_timeout / foreign_keys=ON）+ `SetMaxOpenConns` を小さく（現 MaxConns=10 は不適切）。
- **Rationale**: ESP32 POST × Web UI 読取 × scs cleanup goroutine の競合で SQLITE_BUSY を避ける（レポート §4.5 risk4）。

### Decision: カバレッジ計測を Makefile に配線（R9.3）
- **Context**: Makefile に test/coverage ターゲットが無い（`go test -cover` は手動可能・`internal/applog` は 88.2% で TDD 済の前例あり）。
- **Selected**: `make test` / `make cover`（`go test -cover ./...`）を追加し、移行前ベースライン取得→移行後 80% 維持を判定可能化。

## Risks & Mitigations（design 反映済み・実機確定は Task 0/実装で）
- sqlc 生成型が想定（float64/time.Time）と異なる → Task 0 ゲートで実機記録、pgconv ヘルパ形を後決め。
- modernc の `CURRENT_TIMESTAMP`（T無し）× datetime affinity の time.Time パース揺れ → created_at/updated_at は Go 側明示セット or strftime 統一を検討、実機確認（R-2）。
- sqlite3store × modernc（julianday/`$1`）→ INSERT/SELECT/DELETE の実機スモークを検証タスク化（R-3）。
- `RETURNING *`（9クエリ）× modernc v1.46.1 → Task 0 で動作確認（R-4）。
- dbsnapshot 機能縮退の放置で権威スキーマ陳腐化 → 最小再生成手段を確保（R9.4）。

## References
- `2cc_sdd/SQLite化・単一exe化_実現可能性調査.md` §2/§3.A-E/§4/§6 — 権威ある調査結論
- sqlc SQLite Datatypes: https://docs.sqlc.dev/en/stable/reference/datatypes.html
- scs/sqlite3store: https://pkg.go.dev/github.com/alexedwards/scs/sqlite3store
