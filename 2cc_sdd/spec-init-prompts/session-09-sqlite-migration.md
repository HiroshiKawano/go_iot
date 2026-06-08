# セッション9 spec-init プロンプト: SQLite 移行（基盤）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: sqlite-migration
> 実行例: /kiro-spec-init "（本文を貼り付け）"
> 前提セッション: S1〜S8（全 Web UI 画面が PostgreSQL 構成で実装・動作済み）。本セッションは画面追加ではなく **DB 基盤の差し替え（PostgreSQL → SQLite）**。
> 後続セッション: S10 desktop-exe-packaging（単一 Windows .exe 化）。本セッションが完了しないと S10 は着手不可（厳密直列）。
> 設計フェーズで参照: `2cc_sdd/SQLite化・単一exe化_実現可能性調査.md`（権威ある調査結果。観点A/B/C/E と統合判定 §4 を必読）、`docs/database_snapshot/table_definitions.md`、`db/migrations/*.sql`、`db/queries/*.sql`、`sqlc.yaml`、`internal/infra/pgconv/pgconv.go`、`internal/repository/`（sqlc 生成物）、sqlc 公式 Datatypes リファレンス（SQLite 型マップ）

--- spec-init 本文 ここから ---

## 機能概要

本プロジェクト（農業IoT）を、サーバ不要・圃場でノートPC + ESP32 だけで完結するデスクトップアプリへ進化させる第一歩として、永続化層を **PostgreSQL（pgx/v5）から SQLite（pure-Go ドライバ modernc.org/sqlite + database/sql）へ全面移行**する。
本セッションは UI 機能の追加ではなく、既存の全機能（認証・ダッシュボード・デバイス管理・センサーデータ履歴・アラートルール・アラート履歴・デバイス API）を **SQLite 上でビット等価に動作させる基盤差し替え**である。単一 .exe 化（起動時自動マイグレーション・ブラウザ自動オープン・配布）は後続の S10 desktop-exe-packaging に委ねる。

移行方針の権威ある根拠は `2cc_sdd/SQLite化・単一exe化_実現可能性調査.md`（総合判定 CONDITIONAL_GO）。設計時は同レポートの観点A（スキーマ/クエリ変換）・観点B（sqlc 生成コード差分）・観点C（pgtype 結合全数）・観点E（セッションストア/テスト）と §4 統合判定を必ず参照すること。

## 背景・現状

**現状アーキテクチャ:**
- DB: PostgreSQL（docker-compose）。接続は `internal/infra/db/pool.go` の pgxpool.Pool。
- ORM/クエリ: sqlc（`sqlc.yaml`: engine=postgresql, sql_package=pgx/v5）が `internal/repository/` を生成。生成型に `pgtype.Numeric`/`pgtype.Timestamptz`/`pgtype.Date` が露出（models.go で29箇所）。
- 型変換ハブ: `internal/infra/pgconv/pgconv.go`（Numeric2/NumericToFloat/Timestamptz/TimestamptzToTime の4関数）が pgtype↔float64/time.Time を相互変換。handler/service がこれを経由して整形。
- セッション: scs/pgxstore（`internal/auth/session_auth.go`、pgxpool.Pool 依存）。`sessions` テーブルは `db/migrations/00007` で手動作成（data BYTEA / expiry TIMESTAMPTZ）。
- マイグレーション: goose（Makefile から `goose ... postgres ...` CLI 実行。Go コード内に goose 呼び出しは無い）。
- 既に存在する追い風: `modernc.org/sqlite v1.46.1`（pure-Go）が go.mod に indirect で存在。view/page・component・chart 層は AST テストで pgtype 非依存を強制済み（実コード結合ゼロ）。`WithTx(pgx.Tx)` は呼び出し元ゼロ。統合テストは in-memory fake で docker 非依存。

**移行の本質（調査レポート §4 より）:**
- 最大コストは「スキーマ書換」ではなく **「ドライバ層・型層の総入れ替え」**。sqlc engine 切替で生成型が `pgtype.* → float64/time.Time/sql.Null*` 系へ全面変化し、非生成7ファイル + テスト17ファイル + pgx.ErrNoRows 10箇所 + main.go 配線が連鎖改修となる。
- ただし view層無汚染・WithTx 未使用・統合テスト docker非依存が改修面を限定し、いずれも確立した変換パターンがあるため構造的障壁ではない。

## このセッションのスコープ（実装対象）

**Task 0（最優先ゲート）— sqlc SQLite codegen の実通過:**
> このタスクが通るまで下流（pgconv/handler/test）に着手しない。生成型をここで実機確定する。
- `db/migrations/*.sql` 全7本を SQLite 方言へ書換:
  - `BIGSERIAL PRIMARY KEY` → `INTEGER PRIMARY KEY`（rowid 別名・int64 互換）
  - `TIMESTAMPTZ` → `DATETIME`（datetime/timestamp affinity。**TEXT 宣言にすると sqlc が string 生成になり time.Time が維持できないため避ける**）
  - `NUMERIC(5,2)` → `REAL`（**【オーナー決定 2026-06-08】REAL 採用で確定**。sqlc が float64 生成。温湿度値域で2桁は安全表現可能。現行 pgconv の float 往復・`%.2f` 表示ロジックを流用＝改修小。スケール済 INTEGER 案は不採用）
  - `JSONB`（device_tokens.abilities）→ `json`/`jsonb` affinity（sqlc が json.RawMessage 生成。gen-token は既に json.RawMessage 使用）
  - `BYTEA`（sessions.data）→ `BLOB`
  - `DEFAULT NOW()` → `DEFAULT CURRENT_TIMESTAMP`（または Go 側で明示セット）
  - `COMMENT ON TABLE/COLUMN` 全削除（SQLite 構文エラーになる）
  - `devices_mac_address_format` の正規表現 CHECK（`mac_address ~ '...'`）削除 → アプリ層検証（`internal/handler/device.go:84 isValidMacFormat`）へ委譲（既に検証ロジックが存在＝実質冗長）
  - 維持するもの: 部分インデックス（`WHERE deleted_at IS NULL` 等7本）、DESC 複合インデックス、`BETWEEN`/`IN` 系 CHECK は SQLite で動作するため**書換不要**
- `db/queries/*.sql` を SQLite 方言へ書換:
  - `::NUMERIC(5,2)` / `::BIGINT` キャスト → `CAST(... AS REAL)` / `CAST(... AS INTEGER)` または除去（`COUNT(*)::BIGINT` は SQLite で INTEGER のため単純削除可）
  - `NOW()` → `datetime('now')` / `CURRENT_TIMESTAMP`
  - `DATE(recorded_at)`（ListDailySensorAggregates の日次バケット）→ **`date(recorded_at, '+9 hours')`（【オーナー決定 2026-06-08】JST(+9h) で日境界を補正する）**。実装注意: SELECT / GROUP BY / ORDER BY の**3箇所すべて**を同一式 `date(recorded_at, '+9 hours')` に統一すること（不一致だと集計バケットがズレる）。結果列 `reading_date` は既に JST 日付文字列 'YYYY-MM-DD' になるため、消費側の表示関数（`device_show.go` の `monthDayLabel`）では**再度 TZ 変換しない**こと（二重補正防止）。保存自体は引き続き UTC（TZ補正は集計クエリ内に閉じる）。
  - `sqlc.narg('device_id')::BIGINT IS NULL` → `CAST(sqlc.narg('device_id') AS INTEGER) IS NULL`（**`::` は SQLite parser がトークンを持たず generate 失敗するため必須**）
  - `$N` プレースホルダは sqlc が `?` へ自動変換するため原則そのまま（CAST 併用部のみ手当て）
  - `RETURNING *` は SQLite 3.35+ で対応・modernc v1.46.1 で利用可（要動作確認）
- `sqlc.yaml`: engine を `postgresql` → `sqlite` に変更、`sql_package: pgx/v5` 行を削除（database/sql 既定化）。emit_interface / emit_pointers_for_null_types / emit_json_tags は維持。
- `go tool sqlc generate` を実走し、`internal/repository/`（models.go + 各 *.sql.go + db.go + querier.go）が pgtype 非依存・database/sql ベースへ再生成されることを確認。**特に NUMERIC/AVG/MAX/MIN 集計・DATE・RETURNING の生成型を実機で記録**（下流改修量が決まる）。

**接続層・配線の差し替え:**
- `internal/infra/db/pool.go`: pgxpool.Pool 構築 → `sql.Open("sqlite", dsn)`（modernc。driver 名は `sqlite`）+ `*sql.DB`。PRAGMA を設定（`journal_mode=WAL` / `busy_timeout` / `foreign_keys=ON`）。SQLite は単一 writer のため接続数方針を見直し（`SetMaxOpenConns` を絞る。現状 MaxConns=10 は不適切）。
- `cmd/server/main.go`: `repository.New(pool)` の引数を `*sql.DB` へ、health の `pool.Ping` を `db.PingContext` へ、`auth.NewSessionManager` への引数を `*sql.DB` へ追従。`_ "modernc.org/sqlite"` を blank import し go.mod を direct require に格上げ（go mod tidy）。
- `internal/repository/db.go` の DBTX interface は sqlc が database/sql 版（ExecContext/QueryContext/QueryRowContext）で再生成（自動）。

**型層（pgtype 結合）の書き換え:**
- `internal/infra/pgconv/pgconv.go`: 新型（float64/time.Time/sql.Null*）前提に再実装、または削除して呼び出し側を直アクセス化（推奨は集約を1箇所に残す）。
- handler 本体30箇所/6ファイル（device_show.go=8, sensor_api.go=7, dashboard.go=4, readings.go=4, alert_rule.go=4, alert_history.go=3）の pgconv 呼び出しを新型へ。pgtype を引数/戻り値に持つ関数（dashboard.go formatReadingText/formatThreshold、device_show.go hourMinuteLabel/monthDayLabel/aggregateToFloat、readings.go formatDelay）のシグネチャ修正。
- `internal/service/alert_evaluator.go`: actualValueFor が pgtype.Numeric を返す箇所を新型へ。
- **silent failure 対策（重要）**: `device_show.go:328 aggregateToFloat` の `pgtype.Numeric` case が dead 化し default の 0 フォールバックで**グラフが黙って平坦化する**リスクがある。MAX/MIN 集計列に明示 `CAST(... AS REAL)` を付けて型を float64/sql.NullFloat64 に確定させ、型スイッチを silent fallback でなく明示エラー/期待型に更新する。
- `pgx.ErrNoRows` の `errors.Is` 判定（本体10箇所: alert_rule.go:382, auth.go:86/139, device_auth.go:63, device.go:92/183/279, dashboard.go:73, sensor_api.go:105, device_show.go:344）を `sql.ErrNoRows` へ置換。

**セッションストアの差し替え:**
- `internal/auth/session_auth.go`: `pgxstore.New(pool)` → `sqlite3store.New(sqlDB)`、`NewSessionManager` の引数を `*pgxpool.Pool` → `*sql.DB` へ。Login/Logout/UserIDFromSession のロジックは無変更で流用可。
- go.mod に `github.com/alexedwards/scs/sqlite3store` を追加（go get。モジュールキャッシュ未取得）。
- `db/migrations/00007`: `sessions(token TEXT PRIMARY KEY, data BLOB NOT NULL, expiry REAL NOT NULL)` + `sessions_expiry_idx` へ作り替え（sqlite3store は `julianday()` を使い expiry は REAL。テーブルは自動作成されないため migration で用意する）。
- **検証タスク**: sqlite3store は内部で `$1` 形式プレースホルダ + `julianday()` を使う。modernc で INSERT/SELECT/DELETE が通ることを実機で1回確認（調査レポート §6 参照）。

**テスト層の書き換え:**
- pgconv/pgtype を参照するテスト17ファイル（最多: dashboard_format_test.go=15, alert_evaluator_test.go=12, device_show_test.go=11, alert_rule_test.go=11）のフィクスチャ（`pgconv.Numeric2(値)`/`pgconv.Timestamptz(値)` 組立、`pgtype.Timestamptz{Valid:false}` 直接生成）を primitive 直値 / `sql.NullTime{Valid:false}` 等へ書換。
- 統合テスト（cmd/server/*_integration_test.go）の fake が返す repository 型・`pgx.ErrNoRows` を新型・`sql.ErrNoRows` へ。
- 全テスト green + 80%以上カバレッジ維持。

**開発・運用ツールの追従:**
- `cmd/seed/main.go`: pgxpool→*sql.DB、`TRUNCATE ... RESTART IDENTITY CASCADE` → `DELETE FROM ...` + `DELETE FROM sqlite_sequence`、pgtype ヘルパを新型へ。
- `cmd/gen-token/main.go`（**現場で ESP32 トークン発行に使う重要 CLI**）: pgxpool→*sql.DB、pgtype.Timestamptz を新型へ。確実に動作させる。
- `Makefile`: goose dialect を `postgres` → `sqlite3`、DATABASE_URL を `file:...`/SQLite パスへ。`up`/`down`（docker-compose）ターゲットと `docker-compose.yml` の扱いは S10 で整理（本セッションでは開発が SQLite ファイルで回ることを担保）。
- `internal/dbsnapshot` + `cmd/db-snapshot`: pg_catalog 内省（information_schema/pg_*）→ sqlite_master + PRAGMA への移植。**開発専用ツールで本番非同梱のため優先度低・後回し可**（初期は機能縮退を許容してよいが、CLAUDE.md がスナップショット参照を強制する点に留意し、最低限の再生成手段は確保する）。

## スコープ外（このセッションでやらないこと）

- **単一 Windows .exe 化に関する全て**（起動時自動マイグレーション・go:embed migrations・config の必須env緩和とローカルDBパス既定化・ブラウザ自動オープン・build-windows ターゲット・docker-compose.yml 削除）→ **S10 desktop-exe-packaging** で実施。本セッションは「SQLite で全機能が動作し全テストが green」までをゴールとする（DB ファイルは開発者が手動指定・手動マイグレーションで可）。
- 新しい画面・機能の追加。
- ESP32 ファームウェア側の変更（宛先 IP 設定等は運用設計）。

## 技術制約・準拠事項

**準拠ドキュメント:**
1. `2cc_sdd/SQLite化・単一exe化_実現可能性調査.md` 観点A/B/C/E + §4 統合判定 + §6（scs sqlite3store の CGO 非依存）— **最重要**
2. `docs/database_snapshot/table_definitions.md` — 現状スキーマの権威。移行後は `make db-snapshot`（SQLite 対応後）で再生成
3. sqlc 公式 Datatypes リファレンス（SQLite 型マップ: numeric→float64, datetime→time.Time, json→json.RawMessage）
4. `.kiro/steering/tech.md`（DB/アーキ方針）

**実装言語・フレームワーク:**
- Go 1.26 + Gin v1.12 + templ v0.3 + HTMX。DB は SQLite（modernc.org/sqlite, pure-Go, CGO 不要）+ database/sql + sqlc(engine=sqlite)。
- ドライバ登録名は `sqlite`（`sql.Open("sqlite", dsn)`）。mattn/go-sqlite3（CGO 必須）は **採用しない**（単一 .exe 要件と衝突）。

**移行の不変条件（回帰防止）:**
- 既存の全 Web UI / デバイス API の振る舞いは SQLite 上で等価であること。
- 日時は引き続き UTC で保存し、表示直前に JST 変換する現行戦略を維持（datetime affinity 列 + time.Time マッピング）。
- テナント分離（`d.user_id` スコープ）・所有者認可（authz）・CSRF・Bearer 認証は無変更で機能すること。

**日本語コメント・ラベル:** すべて日本語。コード識別子は英語。

## 受け入れ基準（概略）

1. **codegen**: `go tool sqlc generate`（engine=sqlite）が成功し、生成型に pgtype が一切残らない（float64/time.Time/sql.Null*/json.RawMessage）。
2. **ビルド**: `go build ./...` がローカル（macOS/Linux）で成功。
3. **全テスト green**: ユニット + 統合テストが SQLite（in-memory or tmpfile）で全通過、80%以上カバレッジ。
4. **機能等価**: ログイン・ダッシュボード・デバイス CRUD・センサーデータ履歴（期間/ページング/集計）・アラートルール CRUD・アラート履歴（フィルタ/ページング）・デバイス API（Bearer POST + アラート同期評価）が SQLite 上で従来どおり動作。
5. **集計の正当性**: MAX/MIN/AVG/日次バケットが正しい値を返す（aggregateToFloat の silent 平坦化が起きていないことをテストで保証）。
6. **セッション**: scs/sqlite3store でログイン状態が保持され、cleanup goroutine が SQLITE_BUSY を起こさない（WAL + busy_timeout 設定）。
7. **現場 CLI**: `make gen-token` で ESP32 用 Bearer トークンが SQLite 上で発行でき、その token で `/api/sensor-data` が 201 を返す。

## 未確定事項・要確認（あれば）

1. ~~**NUMERIC 精度**~~ → **【決定済み 2026-06-08】REAL（float64）採用で確定**。現行 float 往復・`%.2f` 表示ロジックを流用（改修小）。スケール済 INTEGER 案は不採用。
2. ~~**日次バケットの TZ**~~ → **【決定済み 2026-06-08】JST(+9h) 補正で確定**（`date(recorded_at, '+9 hours')`）。Task 0 の DATE 書換に反映済み（SELECT/GROUP BY/ORDER BY を同式統一・消費側で二重補正しない）。
3. **sqlc 生成型の実機確定**: NUMERIC/集計/DATE/RETURNING/部分INDEX/CAST化 narg の最終生成型は Task 0 の sqlc generate 実走でのみ確定。想定（float64/time.Time）と異なれば下流改修量が変動する。
4. **modernc の time.Time パース挙動**: CURRENT_TIMESTAMP（T無し表記）と datetime affinity 列の string/time.Time マッピングが現行 UTC 保存 + JST 表示と噛み合うか実機確認。
5. **DATABASE_URL の扱い**: SQLite パスへ転用するか新キー（DB_PATH）を導入するか（.env.example/steering 整合）。デスクトップ向けの既定パス・必須env緩和は S10 に委ねる。
6. **dbsnapshot の移植優先度**: 初期は機能縮退を許容するか、本セッション内で sqlite_master+PRAGMA 対応まで行うか。

--- spec-init 本文 ここまで ---
