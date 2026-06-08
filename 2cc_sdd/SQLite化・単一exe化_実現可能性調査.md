# SQLite化 ＋ 単一Windows .exe化 実現可能性調査レポート

> 調査日: 2026-06-08
> 調査手法: コードベース全数調査（並列11エージェント・5観点 × 敵対的検証 + 統合判定）＋ 外部技術事実の Web 裏取り
> 対象改修: (1) DB を PostgreSQL → SQLite 化、(2) go:embed 等で Gin + HTMX + API + SQLite を単一 Windows `.exe` にし、サーバ不要・圃場でノートPC + ESP32 だけで完結するデスクトップアプリ化
> **総合判定: CONDITIONAL_GO（実現可能・条件付き）**
> 本レポートは調査結果を省略なく記録する。後続の cc-sdd（`/kiro-spec-init` → requirements → design → tasks → impl）の権威ある根拠資料として参照する。

---

## 0. エグゼクティブサマリ

- **両改修とも技術的に実現可能**。無条件 GO ではなく **CONDITIONAL_GO** とする理由は、SQLite化の本当のコストが「スキーマ書き換え」ではなく **「ドライバ層・型層の総入れ替え」** にあり、前提条件（後述ブロッカー）を解消しない限り着手しても動作しないため。
- **単一 .exe 化（観点D）は green（ほぼ自明に可能）**。決定的証拠として、現状の `cmd/server` は既に `CGO_ENABLED=0 GOOS=windows GOARCH=amd64` で 37.4MB の `.exe` をクロスコンパイル成功し、`import "C"` はゼロ。
- **pure-Go SQLite ドライバ `modernc.org/sqlite v1.46.1` が既に go.mod 依存グラフに存在**（goose 経由の indirect）。mattn/go-sqlite3（CGO 必須）を避けられる。
- **scs セッションストアの SQLite版（`sqlite3store`）は CGO を壊さない**。一部の検証エージェントが「CGO衝突」を赤寄りブロッカーとして挙げたが、統合判定と Web 裏取りで **反証**（`sqlite3store` は `database/sql/fmt/log/time` のみ import の純Go・ドライバ非依存）。
- **最大コストは型層**: sqlc を `engine: sqlite` に切替えると生成型が `pgtype.Numeric/Timestamptz/Date` → `float64/time.Time/sql.Null*` へ全面変化。連鎖改修は非生成7ファイル + テスト17ファイル + `pgx.ErrNoRows → sql.ErrNoRows` 10箇所 + main.go 配線。
- **緩衝材（改修面を限定する好材料）**: view層は AST テストで pgtype 非依存を強制済み・`WithTx` 呼び出し元ゼロ・統合テストは既に docker 非依存（in-memory fake）。
- **cc-sdd 分割案**: 2スペック厳密直列 ── `sqlite-migration`（基盤・Task0 に sqlc codegen PoC ゲート）→ `desktop-exe-packaging`（梱包・起動UX）。

---

## 1. 調査方法とスコープ

5つの観点を並列ファンアウトし、各観点について「調査担当 → 別の懐疑的レビュアーによる敵対的再検証」のパイプラインを通した後、全結果を統合判定エージェントが合成した。

| 観点 | 内容 |
|---|---|
| A | DBスキーマ / マイグレーション / クエリの SQLite 変換 |
| B | sqlc エンジン切替（postgresql/pgx/v5 → sqlite/database/sql）と生成コード差分 |
| C | アプリ層の pgx/pgtype/pgconv 結合 全数調査 |
| D | 単一Windows .exe化 / go:embed / CGO / クロスコンパイル / デスクトップUX |
| E | セッションストア / オフライン現場構成 / テスト・開発ツール影響 |

各エージェントは実ファイルを Read/Grep/Bash で読み、推測ではなく `file:line` の根拠に基づいて回答した。一部エージェントは `CGO_ENABLED=0 GOOS=windows go build` や `go mod why` を実走している。

---

## 2. 外部技術事実の検証（Web 裏取り済み）

| 検証項目 | 結論 | 根拠 |
|---|---|---|
| `modernc.org/sqlite` が pure-Go・CGO不要で Windows クロスコンパイル可 | ✅ 確定 | CGoフリーの C SQLite3 移植。`CGO_ENABLED=0 GOOS=windows GOARCH=amd64` で静的リンク単一バイナリ生成可。go.mod に既に indirect で存在 |
| sqlc が SQLite エンジンをサポート | ✅ 確定 | postgresql / mysql / **sqlite** の3エンジン対応。NULL許容列は `sql.NullTime` 等、または `emit_pointers_for_null_types`（SQLite でも有効）でポインタ化 |
| scs `sqlite3store` が存在し `*sql.DB` を受ける（ドライバ非依存） | ✅ 確定 | `func New(db *sql.DB) *SQLite3Store`。modernc 登録で CGO不要に動く |
| scs `sqlite3store` の要求スキーマ | ✅ 確定 | `julianday()` 使用のため expiry は **REAL**。下記参照 |

**scs sqlite3store が要求する sessions スキーマ（裏取り済み）:**
```sql
CREATE TABLE sessions (
	token  TEXT PRIMARY KEY,
	data   BLOB NOT NULL,
	expiry REAL NOT NULL
);
CREATE INDEX sessions_expiry_idx ON sessions(expiry);
```
現状の PG版（`db/migrations/00007_create_sessions.sql`: `data BYTEA` / `expiry TIMESTAMPTZ`）から作り替えが必須。なお `sqlite3store` の README は mattn/go-sqlite3 でのテストを明記するが、パッケージ実体は `database/sql` のみ依存のため modernc で動く（プレースホルダは `$1` 形式・`julianday()` 使用 → SQLite は `$N` を名前付き引数として解釈するため modernc でも動作見込み。実機で1回確認したい軽微点）。

**参考資料（Sources）:**
- modernc.org/sqlite (pkg.go.dev): https://pkg.go.dev/modernc.org/sqlite
- "You don't need CGO to use SQLite in your Go binary": https://til.andrew-quinn.me/posts/you-don-t-need-cgo-to-use-sqlite-in-your-go-binary/
- sqlc Datatypes リファレンス: https://docs.sqlc.dev/en/stable/reference/datatypes.html
- scs/sqlite3store (pkg.go.dev): https://pkg.go.dev/github.com/alexedwards/scs/sqlite3store

---

## 3. 観点別 詳細調査結果（全件・省略なし）

各観点は「要約 → 詳細所見 → 必要な変更 → 未解決の問い → 敵対的検証（同意可否・見落とし・訂正）」の順で記載する。難易度は low / med / high。

---

### 3.A DBスキーマ / マイグレーション / クエリの SQLite 変換

**判定: yellow**

**要約:** SQLite 化は技術的に十分実現可能で、デスクトップ .exe 化との親和性も高い（modernc.org/sqlite が既に go.mod に入っており純Go=cgo不要でWindowsクロスコンパイル容易）。ただし「スキーマ/クエリ書き換え」単独では完結せず、最大の作業はドライバ層の総入れ替えである。現状は pgx/v5 専用に sqlc 生成され（DBTX=pgx インターフェース, pgtype.Numeric/Timestamptz/Date が models 全体に染み出し, $N プレースホルダ, RETURNING *）、ハンドラ/サービス/auth/seed/test の広範囲が pgtype と pgx.ErrNoRows に依存している。スキーマ自体の PG固有構文はいずれも SQLite 等価に変換可能だが、NUMERIC(5,2)精度・日時TZ・部分INDEX・narg の `::BIGINT IS NULL` に個別注意が要る。難易度の主因はスキーマではなくアプリ全層の型/エラー依存の波及。

**詳細所見:**

- **【low】BIGSERIAL → INTEGER PRIMARY KEY AUTOINCREMENT（または INTEGER PK）**: 全7テーブルの id が `BIGSERIAL PRIMARY KEY`。SQLite では `id INTEGER PRIMARY KEY` が rowid 別名で自動採番（64bit）。AUTOINCREMENT は通常不要。生成Go型は int64 のままで互換。seed の `TRUNCATE ... RESTART IDENTITY`（`cmd/seed/main.go:105`）は SQLite に無いため `DELETE FROM` + `DELETE FROM sqlite_sequence` 等へ要変更。根拠: `db/migrations/00001_create_users.sql:4`, `cmd/seed/main.go:102-115`
- **【med】NUMERIC(5,2) 温湿度の精度保持**: temperature/humidity/threshold/actual_value が NUMERIC(5,2)。SQLite は NUMERIC を型アフィニティとして扱い内部は REAL（IEEE754 倍精度）か INTEGER で保持し、固定小数点を真には保たない。本プロジェクトは既に `pgconv.Numeric2` で float64×100 を四捨五入して扱い、表示も `%.2f` / `NumericToFloat` 経由。-40.00〜125.00・0.00〜100.00・閾値という値域では REAL（倍精度）で2桁精度は安全に表現可能。推奨は REAL 列 + 表示側で丸め継続（現行ロジックそのまま活用）。完全な10進固定が要件なら「スケール済INTEGER（×100）」も可だが現行コードの float 往復前提とは不整合で改修増。TEXT保存は集計（AVG/MAX/MIN）が壊れるので不可。根拠: `db/migrations/00004_create_sensor_readings.sql:6-7`, `internal/infra/pgconv/pgconv.go:15-34`
- **【med】TIMESTAMPTZ の保存形式とタイムゾーン戦略**: recorded_at/created_at/updated_at/deleted_at/expires_at 等が TIMESTAMPTZ。SQLite に時刻型は無く TEXT(ISO8601)/INTEGER(unix)/REAL(julian)のいずれか。現行は全て UTC で書込み（seed: `time.Now().UTC()`、device_show.go は表示直前に JST FixedZone へ変換）。よって「UTC を ISO8601 TEXT で保存」が現行戦略と完全整合。sqlc(database/sql) では time.Time にマップ可能（datetime オーバーライド設定）。NOW() デフォルトは SQLite の CURRENT_TIMESTAMP（UTC・秒精度・空白区切り）へ要置換。注意: CURRENT_TIMESTAMP は `YYYY-MM-DD HH:MM:SS`（T無し）で driver の time パースと表記揺れが起きうるため、created_at/updated_at は Go 側で明示セットするか strftime で揃えるのが安全。根拠: `db/migrations/00004_create_sensor_readings.sql:9-10`, `cmd/seed/main.go:164,228`, `internal/handler/device_show.go:42`
- **【low】NOW() デフォルト → CURRENT_TIMESTAMP / Go側生成**: `DEFAULT NOW()` が全テーブルの created_at/updated_at に、クエリ内 NOW() が users/devices/device_tokens/alert_rules/alert_histories の updated_at 等で多用。SQLite は NOW() 非対応。DDL は `DEFAULT (datetime('now'))` または `CURRENT_TIMESTAMP`、クエリ内 NOW() は `datetime('now')` へ一括置換が必要。根拠: `db/queries/users.sql:17,23`, `db/queries/devices.sql:25,32,38`, `db/queries/alert_rules.sql:28,35,42`
- **【med】BYTEA（sessions.data, device_tokens.abilities=JSONB）**: sessions.data BYTEA → SQLite BLOB（直接対応、`models.Session.Data` は []byte で互換）。ただし sessions は sqlc 対象外で scs/pgxstore が管理。pgxstore は pgxpool 依存なので SQLite では別ストアが必須: `alexedwards/scs/sqlite3store`（database/sql）が存在。device_tokens.abilities は JSONB → SQLite には JSONB型無し。TEXT/JSON1（modernc 同梱）で扱う。コンシューマは `gen-token/main.go:84` のみで影響は小さい。根拠: `db/migrations/00007_create_sessions.sql:6-13`, `internal/auth/session_auth.go:27-32`, `db/migrations/00003_create_device_tokens.sql:10`
- **【low】部分インデックス `WHERE deleted_at IS NULL` / `is_notified=FALSE`**: devices/sensor_readings/alert_rules/alert_histories に WHERE 付き部分インデックスが計7本。SQLite は部分インデックスを 3.8.0+ で正式サポートしており構文ほぼそのまま移植可。UNIQUE 部分インデックス `devices_mac_address_unique_active` も同様に動く。DESC 付き複合INDEX も SQLite 3.8.3+ で可。総じて移植容易。根拠: `db/migrations/00002:21-28`, `00004:17-22`, `00006:19-28`
- **【low】CHECK 制約（IN リスト / 数値範囲 / 正規表現）**: `metric IN` / `operator IN`（alert_rules, alert_histories）と temperature/humidity BETWEEN（sensor_readings）は SQLite でそのまま動作。問題は `devices_mac_address_format` の `mac_address ~ '正規表現'`（`00002:14-16`）。`~`（POSIX正規表現）は SQLite 非対応。SQLite の REGEXP は関数未登録だと実行時エラー。対処: (a) この CHECK を削除しアプリ検証に委譲（既に `device.go:84 isValidMacFormat` で正規表現検証済 = 実質冗長）, (b) GLOB/LIKE で近似, (c) modernc に regexp 関数登録。**推奨は (a)**。根拠: `db/migrations/00002:14-16`, `internal/handler/device.go:83-85`
- **【med】::型キャスト（`::NUMERIC(5,2)`, `::BIGINT`）**: sensor_readings.sql で `AVG(temperature)::NUMERIC(5,2)`・`COUNT(*)::BIGINT`、alert_histories.sql で `COUNT(*)::BIGINT`、さらに `narg('device_id')::BIGINT IS NULL`。SQLite に `::` キャスト構文は無い → `CAST(x AS ...)` もしくはキャスト除去。COUNT(*) は SQLite で INTEGER(int64) になるので `::BIGINT` は単純削除可。`AVG()::NUMERIC(5,2)` は `CAST(AVG(...) AS REAL)` か丸めを Go 側に寄せる。これに伴い sqlc 生成型が変わる（現行 AvgTemperature=pgtype.Numeric, MaxTemperature=interface{} → sqlite では sql.NullFloat64/float64 等）ため `device_show.go:243-264,328 aggregateToFloat` の interface{} 分岐も要見直し。根拠: `db/queries/sensor_readings.sql:33,39,72`, `db/queries/alert_histories.sql:50,60,65`
- **【high】DATE(recorded_at) 日次集計バケット**: `ListDailySensorAggregates` が `DATE(recorded_at)` で GROUP BY。SQLite に DATE() 関数は存在するが引数が timestamptz ではなく TEXT/日時表現前提で、戻りは `YYYY-MM-DD` TEXT。sqlc 生成の `ReadingDate` は現状 pgtype.Date（`device_show.go:317 monthDayLabel` が消費）→ SQLite では string になる見込みで `monthDayLabel(d.Time.Format)` を string パースへ要改修。さらにバケット境界は SQLite では保存TZ（=UTC運用）基準になり、現行 design でも「接続TZ=Asia/Tokyo前提=Out of Boundary」と既知の未解決点。UTC 運用だと日境界が JST とズレる懸念が SQLite 化で顕在化するため、`date(recorded_at, '+9 hours')` 等の明示オフセットが要検討。根拠: `db/queries/sensor_readings.sql:32,44`, `internal/handler/device_show.go:313-322`
- **【med】AVG()::NUMERIC / MAX/MIN の型**: 集計列のうち AVG は `::NUMERIC` キャストで pgtype.Numeric、MAX/MIN はキャスト無しで sqlc が interface{} 生成。`aggregateToFloat`（device_show.go:328-）が pgtype.Numeric/float64/nil を防御的に処理。SQLite + database/sql ではこれらが float64/sql.NullFloat64 に変わるので型スイッチ更新が必要（難易度自体は低いが見落とすと実行時に 0 フォールバックで黙ってグラフが平坦化するリスク）。根拠: `internal/repository/sensor_readings.sql.go:114-172`, `internal/handler/device_show.go:324-329`
- **【low】RETURNING *（および RETURNING 列）**: Create/Update/Toggle 系9クエリが RETURNING *。SQLite は 3.35.0(2021) 以降 RETURNING をサポートし、modernc.org/sqlite v1.46.1 は新しいため利用可。sqlc(sqlite engine) も RETURNING 生成に対応。よって書き換え不要で動作する見込みだが、要バージョン確認。根拠: `db/queries/users.sql:10`, `db/queries/alert_rules.sql:20,30,37`
- **【low】$N プレースホルダ → ?（sqlc が自動処理）**: 全クエリが $1,$2 形式。sqlc は engine=sqlite に切替えると生成SQL内のプレースホルダを `?` に自動変換するため、`db/queries/*.sql` の $N 自体は基本そのままで再生成すれば足りる。ただし `sqlc.narg/sqlc.arg` は sqlite engine でも対応するが `::BIGINT` キャスト併用部は CAST へ要修正。手書きの生SQL以外で $N をGoから直書きしている箇所は無い（全て sqlc 生成経由）。根拠: `db/queries/sensor_readings.sql:17-19`, `db/queries/alert_histories.sql:50-57`
- **【med】goose dialect postgres → sqlite3**: goose の dialect は Makefile の CLI 第1引数 `postgres` で指定（`Makefile:42,45,48`）。SQLite 化は `go tool goose -dir db/migrations sqlite3 "<file.db>" up` へ変更するだけ。ただし上記DDL を SQLite 構文へ書換えた新マイグレーションが前提。COMMENT ON TABLE/COLUMN は SQLite 非対応のため全削除（機能影響なし）。.env の DATABASE_URL も `postgres://...` → `file:...` へ。embed .exe 化では起動時に goose をライブラリ（pressly/goose/v3 は既に依存）で自前 Up 実行 + go:embed したマイグレーションFSを使う構成が定石。根拠: `Makefile:41-48`, `db/migrations/00001:15-16`
- **【high】sqlc 設定とドライバ層の総入れ替え（最大の波及）**: sqlc.yaml は engine=postgresql, sql_package=pgx/v5。SQLite 化は engine=sqlite, sql_package=database/sql へ変更し再生成。これにより (a) db.go の DBTX が pgx インターフェース→ database/sql 互換(*sql.DB) に, (b) models の pgtype.Numeric/Timestamptz/Date が string/float64/sql.Null* 等に, (c) pgconv ヘルパが無意味化, (d) pgx.ErrNoRows（全コードに散在）が sql.ErrNoRows へ, (e) infra/db/pool.go の pgxpool→sql.Open, (f) session_auth.go の pgxstore→sqlite3store へ。pgtype 消費は handler 群・service・seed・gen-token・多数のテストに及ぶ。スキーマ変換より遥かに大きい改修面。根拠: `sqlc.yaml:3-10`, `internal/repository/db.go:14-18`, `internal/infra/db/pool.go:8-29`
- **【med】単一 .exe 化の素地は良好（純Go SQLite が既に依存）**: go.mod に modernc.org/sqlite v1.46.1（cgo不要・純Go=wazero/libc 経由）が既に間接依存で存在し、sqlite.go は `import "C"` を含まない。GOOS=windows GOARCH=amd64 のクロスコンパイルが cgo無しで通り、SQLite を同梱した単一 .exe が作れる。静的アセットは既に go:embed 採用。残課題は (1) DBファイルパス決定, (2) 起動時マイグレーション自動実行, (3) ブラウザ自動起動程度。根拠: go.mod, `internal/view/static.go:16-19`, `internal/docs/docs.go:6-11`

**必要な変更:**
1. sqlc.yaml を engine: sqlite / sql_package: database/sql へ変更し sqlc generate を再実行
2. `db/migrations/*.sql` 全7本を SQLite 構文へ書換え（BIGSERIAL→INTEGER PK、NOW()→CURRENT_TIMESTAMP/datetime('now')、TIMESTAMPTZ→TEXT(ISO8601 UTC) または datetime affinity、NUMERIC(5,2)→REAL、JSONB→TEXT/json affinity、BYTEA→BLOB、COMMENT ON 全削除、devices の正規表現CHECK削除しアプリ検証へ委譲）
3. `db/queries/*.sql` の NOW()→datetime('now')、`::NUMERIC(5,2)`/`::BIGINT`→CAST または除去、DATE()→date()（必要なら +9 hours で JST 日境界補正）、`sqlc.narg(...)::BIGINT IS NULL` を CAST 形へ修正
4. `internal/infra/db/pool.go` を pgxpool から database/sql（sql.Open with modernc）へ書換え、PRAGMA（journal_mode=WAL, busy_timeout, foreign_keys）追加
5. `internal/auth/session_auth.go` の scs ストアを pgxstore → sqlite3store へ変更、go.mod 追加。sessions テーブルを sqlite3store 要求スキーマへ調整
6. `internal/infra/pgconv/pgconv.go` の役割を再定義（pgtype 依存除去）または削除
7. `pgx.ErrNoRows → sql.ErrNoRows` を全箇所で実施
8. handler/dashboard.go・device_show.go・readings.go 等の pgtype 消費を新生成型に合わせて書換。aggregateToFloat の型スイッチ更新
9. cmd/seed・cmd/gen-token の pgtype 構築と TRUNCATE RESTART IDENTITY を SQLite 流へ
10. Makefile の goose dialect を postgres→sqlite3、DATABASE_URL を file:... へ。db-snapshot 生成も SQLite 内省へ対応または別系統化
11. cmd/db-snapshot / internal/dbsnapshot の PostgreSQL カタログ依存を SQLite（sqlite_master/pragma）へ移植、または機能縮退を許容

**未解決の問い:**
- NUMERIC(5,2) を REAL で受けるか（現行 float 往復ロジック流用=改修小）、スケール済INTEGER（真の固定小数=改修大）か。温湿度に厳密10進精度が必要かの確認
- 日時のTZ戦略: DATE(recorded_at) 日次バケットが UTC基準だと JST 日境界とズレる。`date(recorded_at,'+9 hours')` で JST 境界に寄せるか、UTC据置きで許容するか
- 本タスクは観点A=スキーマ変換に限定されているが、実際の最大コストはドライバ層・コンシューマ層。改修範囲をAだけに区切ると動作しない点の合意が必要
- scs/sqlite3store の選定可否（モジュール追加）と既存 sessions マイグレーションの要求スキーマ一致の検証
- modernc + sqlc(sqlite engine) で RETURNING・部分INDEX・narg が期待通り生成/動作するかの実機検証
- 同時書込みポリシー: ESP32 複数台の POST と Web UI 並行。SQLite は単一writer のため WAL + busy_timeout 設定とトランザクション方針確認

**敵対的検証（同意: yes / yellow 維持）:**
- *見落とし*: (1) `repository.New()` の引数が pgxpool→*sql.DB へ変わる影響が cmd/server/main.go 本番配線に及ぶ点が独立項目として無い。(2) scs/sqlite3store の driver 名レジストレーション不一致リスク未指摘（modernc は driver 名 `sqlite`、従来 `sqlite3` 前提の例が多い）。(3) aggregateToFloat の pgtype.Numeric case が dead化し default の 0 フォールバックに落ちる silent failure は高リスク → MAX/MIN に明示 CAST を格上げすべき。(4) `TRUNCATE ... RESTART IDENTITY` 後続の `CASCADE` も SQLite 非対応。
- *訂正*: (1) 「生成型が pgtype群→string/sql.Null* に総替え」は不正確。sqlc v1.30.0 の SQLite 型マップは numeric/decimal を **float64/(*float64)/sql.NullFloat64** にマップ（string ではない）。現行 float 往復ロジックとむしろ整合的で改修はやや軽い。(2) 「TIMESTAMPTZ→TEXT」は time.Time 利用と矛盾。sqlc SQLite は date/datetime/timestamp affinity の列のみ time.Time を生成し、TEXT 宣言だと string になる。time.Time 維持なら列型を datetime/timestamp affinity で宣言する必要。(3) 「JSONB→TEXT」は最適でない。sqlc SQLite は json/jsonb affinity を `json.RawMessage` にマップし、gen-token は既に json.RawMessage 使用。列型を jsonb/json のまま宣言すれば改修が小さい。(4) `::` は SQLite lexer にトークン自体が存在せず、`::` を含むクエリは sqlc generate 時点でパースエラー。`sqlc.narg('device_id')::BIGINT IS NULL` は `CAST(sqlc.narg('device_id') AS INTEGER) IS NULL` 等へ必須の手書き修正。

---

### 3.B sqlc エンジン切替と生成コード差分

**判定: yellow**

**要約:** sqlc の engine を sqlite に切り替えること自体は sqlc.yaml の数行修正で可能で、SQLite ドライバ（modernc.org/sqlite）は既に go.mod の indirect 依存に存在する。ただし生成コードの型が pgtype.* から database/sql 系へ全面的に変わり、波及が広い。最大の論点は (1) DBTX/New/WithTx シグネチャが pgx 系→database/sql 系へ変わり main.go の配線（`repository.New(pool)`→`*sql.DB`）修正が必要、(2) pgtype.Numeric / pgtype.Timestamptz が消えて float64/sql.NullFloat64 や time.Time/sql.NullTime になり、これに依存する非生成コードが 99箇所（pgtype直接）+103箇所（pgconvヘルパー経由）存在。ただし型変換が internal/infra/pgconv に集約されており、ここを書き換えれば呼び出し側の多くは pgconv 経由で吸収できる構造になっているのが救い。SQL 側に PostgreSQL 固有構文が多数あり、SQLite 方言への書き換えが生成前提として必須。緑でないが赤でもない（黄）。

**詳細所見:**

- **【med】DBTX/New/WithTx シグネチャの変化**: 現状 db.go は pgx 専用（DBTX が `Exec(ctx,string,...)(pgconn.CommandTag,error)` / `Query(...)(pgx.Rows,error)` / `QueryRow(...)pgx.Row`、New(db DBTX)、WithTx(tx pgx.Tx)）。engine=sqlite では sqlc が DBTX を `ExecContext/QueryContext/QueryRowContext/PrepareContext`（database/sql 標準）で再生成し、New は `*sql.DB`/`*sql.Tx` を受ける形に。main.go の `repository.New(pool)` は `*pgxpool.Pool` → `*sql.DB` へ変更必須。WithTx も `pgx.Tx`→`*sql.Tx`。根拠: `internal/repository/db.go:14-32`, `cmd/server/main.go:48`, `internal/infra/db/pool.go:14`
- **【med】pgtype.Numeric → 数値型への変化**: NUMERIC(5,2)（temperature/humidity/threshold/actual_value）は database/sql エンジンでは sqlc が通常 float64（NOT NULL）、NULL 可なら `*float64` か sql.NullFloat64 を生成。`pgconv.Numeric2/NumericToFloat` の存在意義を消し、Create系の Params も float64 を直接渡す形に。根拠: `models.go:20,22,40,84,86`, `sensor_readings.sql.go:43-45`, `pgconv.go:15-34`
- **【med】pgtype.Timestamptz → time.Time/sql.NullTime への変化**: TIMESTAMPTZ カラムは database/sql + SQLite では NOT NULL は time.Time、NULL 可は `*time.Time` か sql.NullTime に。`.Valid` フィールド参照（例 `d.LastCommunicatedAt.Valid`）が型に応じ nil 比較や .Valid へ書き換え必要。根拠: `models.go:25-28,...`, `internal/handler/dashboard.go:149`, `pgconv.go:37-48`
- **【med】非正規化された pgtype 依存の波及範囲**: 非生成コードでの pgtype. 直接出現が 99行、pgconv. ヘルパー経由が 103行。ただし変換はほぼ全て pgconv に集約されており、pgconv の4関数を新型に合わせ書き換えれば、float を返す API はシグネチャ変更なしで吸収できる箇所が多い。一方 pgtype を関数引数の型に直接書いている箇所（dashboard.go:141/193/198、device_show.go:309/317、readings.go:264、alert_evaluator.go:113）はシグネチャ書換が必要。根拠: grep集計, `pgconv.go`, `dashboard.go:141,193,198`, `device_show.go:309,317,328-336`, `alert_evaluator.go:82,113-121`
- **【low】DeviceToken.Abilities ([]byte) と BYTEA/JSONB**: abilities は JSONB で sqlc は []byte を生成。SQLite には JSONB が無くTEXT/BLOB。BLOB なら []byte 維持で波及小。sessions の data BYTEA も同様（scs 管理で sqlc 対象外、別途差替）。根拠: `00003:10`, `models.go:72`, `00007:8`
- **【low】BIGINT/BIGSERIAL → int64 維持（影響小）**: 主キー id 等は int64 維持見込み。LIMIT/OFFSET の int32 がエンジン差で int64 になる可能性があり呼び出し側キャスト調整が要りうる。device_id の nullable narg は `*int64` 維持見込み。根拠: `models.go:13,33,49`, `sensor_readings.sql.go:298-299`, `alert_histories.sql.go:29,117`
- **【high】SQL 内 PostgreSQL 固有構文の書換（生成の前提）**: 型差分以前に、クエリ/スキーマが PG 方言依存で sqlc(sqlite parser) が解析失敗する。`::BIGINT`/`::NUMERIC(5,2)`, `COUNT(*)::BIGINT`, NOW(), `DATE(recorded_at) GROUP BY`, `sqlc.narg('device_id')::BIGINT`, BIGSERIAL/JSONB/BYTEA/TIMESTAMPTZ、正規表現 CHECK。これらを CAST/strftime/CURRENT_TIMESTAMP/INTEGER PK/TEXT 等へ書換が必須。根拠: `db/queries/sensor_readings.sql:32-56`, `db/queries/alert_histories.sql:50,51,65,66`, `00002:14-16`, `00004:6-7`
- **【low】GetSensorReadingsSummary 等の interface{} 集計列**: MAX/MIN(temperature) 等キャスト無し集計は sqlc が any を生成。SQLite エンジンでは float64 か interface{} に。aggregateToFloat の `case pgtype.Numeric` が死にコード化するだけで動作は壊れにくいが、型変更に追従が要る。根拠: `sensor_readings.sql.go:116-120,166-170`, `device_show.go:324-337`
- **【med】repository.Querier interface の安定性（波及の緩衝材）**: handler/service/authz/auth は具象 *Queries ではなく `repository.Querier`（emit_interface=true）経由で DB を受けており、メソッドシグネチャが pgtype→新型に変わるとこの interface も自動再生成される。実装する手書きモック（各 _test.go の Querier 埋め込み）は埋め込みなので壊れにくいが、Params/Row 型を組み立てるテストデータ生成箇所は新型へ全面書換が必要。根拠: `querier.go:11-66`, `cmd/server/main.go:98`, pgtype使用テスト4ファイル
- **【low】DBTX を repository 外で直接使う箇所はほぼ無い**: pgx の Query/Exec/Begin を repository パッケージ外で直接呼ぶ本番コードは `cmd/seed/main.go:105` の pool.Exec(truncate) のみ。handler 群の `.Query` は Gin の `c.Query`（DB 無関係）。トランザクション配線の波及は seed と main の pool 生成部に限局。根拠: grep, `readings.go:81-95`, `device_show.go:81`
- **【low】SQLite ドライバは既に依存ツリーに存在**: modernc.org/sqlite v1.46.1（Pure-Go, CGO 不要）が go.mod に indirect で既存。Windows 単一 .exe 目標と整合し、mattn/go-sqlite3（CGO 必須）を避けられる。生成コードは database/sql 標準 interface なのでドライバ非依存で、driver 登録だけ main で行えばよい。根拠: go.mod, `Makefile:31`

**必要な変更:**
1. sqlc.yaml: engine を postgresql→sqlite、sql_package(pgx/v5) 行を削除（database/sql 既定化）。emit_interface/emit_pointers_for_null_types/emit_json_tags は維持可
2. db/migrations/*.sql を SQLite 方言へ書換（BIGSERIAL→INTEGER PK, TIMESTAMPTZ→DATETIME/TEXT, NUMERIC(5,2)→REAL, JSONB→TEXT/BLOB or json affinity, BYTEA→BLOB, NOW()→CURRENT_TIMESTAMP, 正規表現 CHECK 撤去, goose dialect も postgres→sqlite3）
3. db/queries/*.sql を SQLite 構文へ書換（::CAST→CAST, DATE()→date()/strftime, COUNT(*)::BIGINT→CAST, narg キャスト調整, NOW()→CURRENT_TIMESTAMP, RETURNING * 対応確認）
4. make sqlc 再生成 → models.go/*.sql.go/db.go/querier.go が pgtype 非依存・database/sql ベースへ全面差替（自動）
5. pgconv.go を新型前提に全面書換、または不要化して呼び出し側を直接型に移行
6. 非生成コードの pgtype を引数/戻り値に持つ関数のシグネチャ修正（dashboard.go, device_show.go, readings.go, alert_evaluator.go）
7. cmd/server/main.go: NewPool(*pgxpool.Pool)→*sql.DB へ。pool.Ping→db.PingContext。pool.go も sql.Open ベースへ
8. scs セッションストア差替: pgxstore→sqlite3store。auth/session_auth.go の *pgxpool.Pool 依存除去
9. cmd/seed・cmd/gen-token: pgxpool/pgtype 依存を新型へ、pool.Exec(truncate) を *sql.DB ベースへ
10. 各 _test.go の Params/Row テストデータ生成を新型へ書換
11. docs/database_snapshot 再生成系を SQLite 用に作り直すか無効化

**未解決の問い:**
- sqlc v1.30.0 の sqlite エンジンが NUMERIC(5,2) をどの Go 型にマップするか、RETURNING * / sqlc.narg を完全サポートするかを実機の sqlc generate で要検証
- NUMERIC を REAL(float64)化すると小数2桁の精度・丸めが pgtype.Numeric（big.Int+Exp）と変わる。閾値比較の正確性の業務要件確認が必要
- SQLite の DATETIME は TZ を持たない。recorded_at/triggered_at の JST 表示と保存値の TZ 解釈をどう統一するか。modernc ドライバの time.Time パース挙動の確認が必要
- 集計列が SQLite で float64 か interface{} か。aggregateToFloat の switch をどこまで残すか
- Windows 単一 .exe で SQLite ファイルの配置とマイグレーション適用タイミングの設計（観点A/D で別途）
- 正規表現 CHECK を SQLite で再現できないため MAC 形式検証をアプリ層へ移す必要（既存検証の有無確認）

**敵対的検証（同意: yes / yellow 維持）:**
- *見落とし*: (1) **埋め込みマイグレーションランナーが現状ゼロ**。goose は Makefile で外部 CLI 起動 + `postgres` 方言ハードコードで、Go コード内に goose.SetDialect/embed は存在しない。単一 .exe では起動時に migrations を embed（`//go:embed` + `goose.SetBaseFS` + `goose.SetDialect("sqlite3")` + `goose.Up`）する **net-new 実装が必須**。(2) **scs sqlite3store の存在・CGO 依存性が未検証のまま楽観**（このエージェントは「mattn/go-sqlite3 依存で CGO 衝突」を懸念。→ **後の統合判定で反証。下記 §6 参照**）。(3) Windows 単一 .exe での SQLite ファイル配置・WAL モード・ESP32 同時書込の競合（SQLite は単一ライタ）について未言及。`pool.go` の MaxConns=10 を `SetMaxOpenConns` 等へ最適化する判断が要る。
- *訂正*: (1) modernc が indirect なのは sqlc/goose の推移依存。アプリが driver として使うには main で `_ "modernc.org/sqlite"` を blank import し direct require へ昇格。**driver 登録名は `sqlite`** → `sql.Open("sqlite", path)`。(2) seed/gen-token の TRUNCATE は SQLite に文が無い → `DELETE FROM` + `DELETE FROM sqlite_sequence` への **SQL 文書換**。(3) 部分インデックス・部分 UNIQUE インデックスは SQLite で移植可能で「そのまま生かせる」ことを明記すべき。(4) CHECK のうち撤去が必要なのは正規表現のみで、`BETWEEN`/`IN` 等は移植可（全 CHECK を剥がす事故を防ぐ）。(5) emit_pointers_for_null_types は SQLite でも機能（`Device.Location` が `*string` に生成済）。

---

### 3.C アプリ層の pgx/pgtype/pgconv 結合 全数調査

**判定: yellow**

**要約:** pgx/pgtype/pgconv のアプリ層結合は「handler 層 + テスト層」に集中している。総件数: pgtype=131行（うち repository/sqlc生成=68, テスト=15, handler本体=18, service=4）, pgconv=123行（本体32 / テスト71）, jackc/pgx=39行, pgxpool=9行。結合の根源は repository/models.go（sqlc 生成、pgtype.Numeric/Timestamptz/Date カラム29箇所）で、これを handler が pgconv 4関数で primitive 化するチェーン構造。SQLite移行では sqlc 生成型が pgtype系から sql.NullTime/float64/string 系へ全面変化するため、pgconv.go は不要化（削除）、それを呼ぶ全箇所（本体32+テスト71=103箇所）が書換対象になる。view/page・chart 層は設計上 pgtype を持ち込まない純粋層で実コード結合ゼロなので影響なし。最も書換量が多い層は「テストフィクスチャ層（71箇所/9ファイル）」、本体では「handler 層（30箇所/6ファイル）」。

**詳細所見:**

- **【low】全件集計（シンボル別）**: pgtype=131行/21ファイル, pgconv=123行/19ファイル, jackc/pgx=39行/35ファイル, pgxpool=9行/4ファイル。pgconv関数別総使用: Numeric2=64, Timestamptz=21, NumericToFloat=16, TimestamptzToTime=12。根拠: grep -rn 集計
- **【med】結合の根源 = sqlc生成型**: repository/models.go が pgtype.Numeric（Threshold/ActualValue/Temperature/Humidity）・pgtype.Timestamptz（各種日時）・pgtype.Date を露出（29箇所）。各クエリ .sql.go の引数/結果型も同様。SQLite化で sqlc は database/sql 型で再生成し、型が総入れ替えになる。根拠: `internal/repository/models.go:20-111`
- **【low】pgconv.go の役割（変換ハブ）**: pgtype↔float64/time.Time の相互変換4関数のみ。SQLite移行で sqlc が NUMERIC→float64/REAL, TIMESTAMP→time.Time/sql.NullTime を直接生成するため、変換ハブ自体が不要化し丸ごと削除候補。呼び出し側を primitive 直渡しへ書換える。根拠: `internal/infra/pgconv/pgconv.go:15-48`
- **【med】最大書換層 = テストフィクスチャ**: pgconv 使用71箇所/9テストファイルが sqlc型をモック生成する際に pgconv.Numeric2/Timestamptz で組み立てている。型変更で全フィクスチャが primitive 直値へ書換必須。最多は handler/dashboard_format_test.go(15), service/alert_evaluator_test.go(12), device_show_test.go(11), alert_rule_test.go(11)。pgtype直接生成もテスト5ファイルに15箇所。根拠: `dashboard_format_test.go:28-173`, `alert_evaluator_test.go:64-315`
- **【med】本体 handler 層 = 30箇所/6ファイル**: device_show.go(8), sensor_api.go(7), dashboard.go(4), readings.go(4), alert_rule.go(4), alert_history.go(3)。いずれも「sqlc型→primitive」の整形で、SQLite型では pgconv 呼び出しを除去し直接フィールドアクセスへ単純化できる（書換は機械的）。根拠: `sensor_api.go:117-139`, `device_show.go:203-331`, `dashboard.go:141-199`
- **【low】SQLite考慮済みの痕跡（防御コード）**: device_show.go の aggregateToFloat が MAX/MIN集計の any 戻り値を pgtype.Numeric/float64/default でswitchしており、SQLiteで float64 を返すケースが既にコメントで想定済み。移行後はpgtype caseを削れる。根拠: `internal/handler/device_show.go:324-337`
- **【low】pgx.ErrNoRows の置換**: 本体で `errors.Is(err, pgx.ErrNoRows)` を使用。SQLite(database/sql)では sql.ErrNoRows へ全置換が必要。機械的だが漏れると not-found 判定が壊れる。根拠: grep pgx.ErrNoRows
- **【med】接続層 pgxpool**: infra/db/pool.go, auth/session_auth.go, cmd/seed/main.go, dbsnapshot/introspect.go の4ファイルのみ。SQLite化で *sql.DB へ。session_auth は scs/sqlite3store へ差替。repository/db.go の DBTX interface も sqlc-sqlite が database/sql 版を再生成。根拠: `infra/db/pool.go:14-42`, `auth/session_auth.go:14-29`, `repository/db.go:16-28`
- **【low】view/page・chart層は無汚染**: view/page/views.go, view/component/views.go, chart/svg.go の pgtype 言及はすべて「pgtype/repository型を持ち込まない（view純粋性）」という設計コメントで、実コード結合ゼロ。整形は handler 完結のため、これら下流層は SQLite移行で一切変更不要。アーキ分離が移行コストを大幅に抑えている。根拠: `view/page/views.go:12,22,37,95` ほか

**必要な変更:**
1. sqlc.yaml を engine: sqlite へ変更し internal/repository/ 全体（models.go + 4 *.sql.go + db.go）を再生成（型が pgtype系→database/sql系へ全入替）
2. pgconv.go を削除（変換ハブ不要化）。呼び出し本体32箇所を sqlc新型の直接フィールドアクセス/標準変換へ書換 — handler層30箇所（device_show=8, sensor_api=7, dashboard=4, readings=4, alert_rule=4, alert_history=3）, service層1箇所
3. テストフィクスチャ71箇所/9ファイルの pgconv.Numeric2/Timestamptz 組立を primitive 直値へ書換。pgtype直接生成テスト15箇所/5ファイルも sql.NullTime{Valid:false} 等へ
4. `errors.Is(err, pgx.ErrNoRows)` 本体を sql.ErrNoRows へ置換
5. infra/db/pool.go を pgxpool.Pool構築から *sql.DB（modernc.org/sqlite）へ。auth/session_auth.go の scs/pgxstore を scs/sqlite3store へ差替
6. cmd/seed・cmd/gen-token の pgtype/pgxpool 依存を新型へ書換
7. service/alert_evaluator.go の actualValueFor が pgtype.Numeric を返す箇所を新型シグネチャへ変更

**未解決の問い:**
- SQLite用 sqlc ドライバ選定: modernc.org/sqlite（pure-Go, 単一exe化容易）必須（mattn は cgo でクロスビルド面倒）
- NUMERIC(5,2)精度の扱い（アプリ側丸めで代替可能か）
- 集計クエリ（MAX/MIN/AVG）の sqlc-sqlite 生成型を実生成で確認要
- db/queries の PG方言が SQLite方言で動くか、再生成前にクエリSQL自体の方言書換が別途必要か
- scs セッションストアを sqlite3store にした場合 cgo 依存が混入しないか（→ §6 で反証）

**敵対的検証（同意: no / yellow 維持・理由づけを強化すべき）:**
- *見落とし*: (1) **マイグレーション層が changes_required から完全欠落**。db/migrations 7本が PG 専用方言で SQLite で起動不能（BIGSERIAL/DEFAULT NOW()/TIMESTAMPTZ/BYTEA/JSONB/COMMENT ON 全面書換が必須）。(2) **CHECK制約の正規表現が SQLite で動作不能**（`mac_address ~ '...'`、SQLite に `~` 演算子無し）。MAC検証をアプリ層へ移設要。(3) **pgtype.Date が pgconv 4関数の対象外で見落とし**（`monthDayLabel(d pgtype.Date)` と `ListDailySensorAggregatesRow.ReadingDate`）。DATE() の SQLite 生成型未確定で `.Time.Format` も書換対象。(4) **scs sqlite3store の CGO 衝突懸念**（このエージェントは mattn/go-sqlite3 必須で pure-Go 単一exe と衝突、modernc 対応の scs ストアは無く自作 or JWT 等への変更が必要と主張。→ **§6 で反証**）。(5) **デスクトップ起動UX変更が全面欠落**（config の DATABASE_URL 必須env撤廃→ローカルDBパス既定化、起動時 DB自動作成+初回マイグレーション自動適用、go:embed マイグレーション同梱）。(6) **pgxpool固有のヘルスチェック波及**（`pool.Ping`→`PingContext`、`NewSessionManager` シグネチャ変更が main.go に波及）。
- *訂正*: (1) `pgx.ErrNoRows` の本体置換数「14箇所」は過大、実数 **10箇所**（残りは authz/ownership.go のコメント言及で置換不要）。(2) modernc が indirect にあるのは tool の goose が引き込む推移的依存で「アプリの DB ドライバとして配線済み」ではない。「既に用意されている」という含意は誤り。(3) chart/svg.go・view層の「無汚染」は正確だが、handler 層は監査範囲が広く config.go・main.go の配線変更も移行スコープに含めるべき。(4) feasibility yellow は妥当だが理由は「量」ではなく「マイグレーション全方言書換・CHECK正規表現の代替設計・scsストアの代替実装・起動UX再設計という構造的未解決項目が複数あること」。これらが片付かないと red 寄り。単一exe は modernc + go:embed migrations + cgoフリーstore が揃えば達成可能なので red ではない。

> 注: この観点Cの検証エージェントが挙げた「scs sqlite3store は CGO 必須」は、後述 §6 のとおり**事実誤認**であることが統合判定と Web 裏取りで確定している。

---

### 3.D 単一Windows .exe化 / go:embed / CGO / クロスコンパイル / デスクトップUX

**判定: green（観点A/B 完了を前提とする条件付き green）**

**要約:** 単一Windows .exe 化は技術的に高い確度で実現可能（green）。決定的証拠として、現状の server バイナリは既に `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./cmd/server` で 37.4MB の .exe をクロスコンパイル成功する（cgo 依存ゼロ。`import "C"` なし）。SQLite 化に使う `modernc.org/sqlite` v1.46.1 は pure-Go で、単体でも同条件でコンパイル成功を確認済み。同パッケージは database/sql ドライバを登録するため、goose の自動マイグレーションと scs/sqlite3store の双方に流用できる。go:embed は既に CSS・docs・templ生成物（28ファイルはGoコードとしてコンパイル同梱）で稼働しており、db/migrations を embed.FS 化して `goose.SetBaseFS` + `goose.Up` を起動時に呼ぶ自動マイグレーション方式が確立できる。重大トレードオフは Wails/webview を採るとCGO+WebView2依存で「pure-Go単一.exe」前提が崩れるため、起動時ブラウザ自動オープン方式を強く推奨。

**詳細所見:**

- **【low】modernc.org/sqlite が pure-Go で CGO=0 windows/amd64 ビルド可能か**: go.mod:141 に `modernc.org/sqlite v1.46.1 // indirect`（依存元は goose ツール）。実際に `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build modernc.org/sqlite` がエラーなくコンパイル成功を確認。純Go実装（modernc.org/libc, wazero ベース）。アプリ側で使うには indirect を direct require に格上げ（go mod tidy）するだけ。根拠: go.mod:141, `go mod why modernc.org/sqlite`, ビルド実走成功
- **【low】サーバ本体に他のCGO必須依存が無いことの実証**: 現状の cmd/server を `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build` した結果、37.4MB の .exe 生成成功（PostgreSQL/pgx 構成のまま）。`go list -deps ./cmd/server` でも sqlite/cgo/mattn-sqlite3 等は出現せず（mattn/go-isatty のみ＝pure-Go版あり）。`import "C"` も無し。SQLite を modernc に差し替えてもこの状態が維持される。根拠: ビルド実走, `go list -deps`, grep
- **【low】go:embed の現状同梱範囲**: 既に3系統が同梱: (1) static CSS → `internal/view/static.go:18 //go:embed all:public`、MountStatic で /static 配信。(2) docs → `internal/docs/docs.go:8-12`。(3) templ ページは *_templ.go（28ファイル）としてGoコードに変換されバイナリへ直接コンパイル。UI/静的資産は既に完全に1バイナリへ同梱済み。残課題は db/migrations の embed 追加のみ。根拠: `static.go:18-40`, `docs.go:6-13`, `find internal/view -name *_templ.go` = 28
- **【med】db/migrations の embed.FS 同梱 + goose.SetBaseFS 起動時自動マイグレーション**: db/migrations は 00001〜00007 の7ファイル。新規パッケージ（例 internal/migrate）に `//go:embed migrations/*.sql` を置き、`goose.SetBaseFS(embeddedFS)`（goose.go:33）+ `goose.SetDialect("sqlite3")`（dialect.go:41）+ `goose.Up(sqlDB, "migrations")`（up.go:161）を起動時に呼ぶだけで自動適用できる。goose/v3 v3.27.0 は既に依存にあり、ライブラリ呼び出しに切替可能。embed は親(..)を辿れないため migrations をパッケージ配下に置く構成が必要（static.go:16・docs.go の先例と同じ）。根拠: goose.go:33, up.go:161, dialect.go:41, static.go:16
- **【med】セッションストアの SQLite 化（scs）**: 現状 `session_auth.go:29` で `sm.Store = pgxstore.New(pool)`、sessions テーブルは migration 00007 で手動作成。SQLite では `github.com/alexedwards/scs/sqlite3store`（go.mod追加要）へ差し替え、`*sql.DB`（modernc）を渡す。modernc は database/sql ドライバを登録するため scs/sqlite3store と直結可能。sessions の DDL も SQLite方言へ要書換。根拠: `session_auth.go:14,29`, `00007`, modernc sqlite.go:57
- **【med】SQLite DBファイル配置・書込権限・初回作成**: config.go は DATABASE_URL を必須env として読み込む設計。SQLite化では DSN を `file:...sqlite?...` 形式に変えるか専用 DBパス解決ロジックを追加する。配置の選択肢: (a) exe隣（`os.Executable()`）→ Program Files 配下だと書込不可リスク（UAC）。(b) `%LOCALAPPDATA%\go_iot\app.db`（`os.UserConfigDir()`）→ 書込確実・**推奨**。初回起動時は modernc が DSN にファイルが無ければ自動作成、その後 goose.Up で全テーブル作成。config の必須env検証はデスクトップではフォールバック既定値が要る（SESSION_SECRET も初回自動生成→ファイル保存等）。根拠: `internal/config/config.go:24,31-39`, `cmd/server/main.go:42`
- **【low】デスクトップ体験: ブラウザ自動オープン vs Wails/webview**: **推奨は「起動時に既定ブラウザで http://localhost:PORT を自動オープン」方式**。pure-Go単一.exe を維持でき、追加ランタイム不要。Windows実装は `exec.Command("rundll32", "url.dll,FileProtocolHandler", url)` か `cmd /c start`、または `github.com/cli/browser` を direct 追加。Wails/webview2 は WebView2 Runtime + cgo を要し「CGO=0 pure-Go 単一.exe」前提を壊す → **不採用が妥当**。systray常駐（getlantern/systray 等）を足せばデスクトップ感を補える（任意・cgo無し選択肢あり）。根拠: `go list -deps`, `main.go:55-70`
- **【low】Windows向け Makefile / ビルドコマンド案**: 現行 build ターゲット（Makefile:29-31）に環境変数を足すだけ: `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X github.com/HiroshiKawano/go_iot/internal/view.Version=$(SHA)" -o dist/go_iot.exe ./cmd/server`。`-s -w` でサイズ削減、Version注入は static.go:22 の既存 ldflags 機構を流用。.gitignore は既に `*.exe` を無視。GUIアプリ化でコンソール窓を消すなら `-ldflags "-H windowsgui"` を追加。CI/goreleaser は現状なし。根拠: `Makefile:25-34`, `static.go:21-24`, `.gitignore:4`

**必要な変更:**
1. go.mod: modernc.org/sqlite を indirect→direct require へ格上げ、scs/sqlite3store を追加、不要になる pgx/pgxstore/goose-postgres 依存の整理
2. internal/infra/db: pgxpool ベースの NewPool を database/sql + modernc ベースの接続生成へ置換。repository.New が *sql.DB を受ける形へ（sqlc 再生成）
3. db/migrations/*.sql: PG方言を SQLite方言へ書換（※観点A/B範囲）
4. 新規パッケージ（例 internal/migrate）で db/migrations を //go:embed 同梱し、起動時に goose.SetBaseFS + SetDialect("sqlite3") + Up を main.go(run内) へ追加
5. internal/auth/session_auth.go: pgxstore.New(pool) → sqlite3store.New(sqlDB) へ差替
6. internal/config/config.go: DATABASE_URL/SESSION_SECRET の必須検証をデスクトップ向けに緩和（SQLiteパス既定=%LOCALAPPDATA%、SESSION_SECRET 初回自動生成・永続化）
7. cmd/server/main.go: サーバ起動後に既定ブラウザで http://localhost:PORT を自動オープン
8. Makefile: Windows .exe 用 build-windows ターゲット追加（CGO_ENABLED=0 GOOS=windows GOARCH=amd64 + -ldflags -s -w -X Version、任意で -H windowsgui）

**未解決の問い:**
- 観点D の前提である「SQLite移行（観点A/B）」が完了していること
- コンソール窓を消す（-H windowsgui）場合、log.Printf 出力先が失われるためログをファイル（%LOCALAPPDATA%配下）へリダイレクトする方針が必要か
- SESSION_SECRET をデスクトップで初回自動生成・保存する場合の保存先と保護方法（平文ファイル可否）
- SQLite の WAL モード有効化とセンサー受信 + Web UI 読取の並行アクセス耐性の検証
- systray 常駐や自動更新まで含めるか、初版はブラウザ自動オープンのみで十分か

**敵対的検証（同意: yes / green 維持）:**
- *見落とし*: (1) green は「観点A/B が完了している」ことに完全依存する**条件付き green** である点を強調すべき（DBTX interface が pgx 専用、pgtype が業務ロジック48箇所に波及）。(2) modernc 固有の並行アクセス注意点が欠落。WAL 有効化だけでなく `db.SetMaxOpenConns(1)` もしくは `_pragma=busy_timeout(5000)` の DSN 指定が事実上必須。設定漏れは運用時に断続的 500 を招く → changes_required へ昇格が妥当。
- *訂正*: (1) バイナリサイズは正確には 37,412,864 バイト（約 35.7MiB / 37.4MB）。(2) docs.go が openapi.yaml/index.html をパッケージ同階層 embed で実現している先例があり、同方式（新パッケージ internal/migrate 配下に migrations/ を配置）で確実に踏襲可能。(3) **`WithTx(tx pgx.Tx)` はアプリ・cmd 全体で呼び出し元ゼロ（grep 0件）** → トランザクション周りの移行波及は無く、database/sql 化で署名が変わっても実害が出ない（A/B のリスクを下げる方向の事実）。(4) scs/sqlite3store は go.mod 追加が必要（モジュールキャッシュ未存在を確認）であると同時に、sessions テーブルを自動作成しない（DDL を migration 側で SQLite 方言で用意する必要がある）点を明記すべき。

---

### 3.E セッションストア / オフライン現場構成 / テスト・開発ツール影響

**判定: green（調査担当）→ yellow（敵対的検証による補正）**

**要約:** 観点E の範囲では SQLite 化・単一exe 化は実現可能性が高い（green）。決定的な追い風が3点ある: (1) 統合テスト群はすべて in-memory fake（fakeUserQuerier/fakeDeviceQuerier）で動作し docker PG に一切依存しない。「テストの docker 不要化」は移行コストではなく既に達成済み。(2) modernc.org/sqlite v1.46.1 が既に go.mod のモジュールグラフに入っており、CGO 不要の純Go SQLite ドライバが追加 fetch なしで利用可能。(3) ESP32→ノートPC の Bearer POST 構成は main.go が `:port`（全インタフェース）で listen し DeviceAuth が DB 参照のみで完結するため、インターネット不要・現状の認証機構そのままで成立する。主な作業は scs ストア差し替え・sessions DDL の型書換・config の DATABASE_URL 解釈変更・cmd/seed と cmd/gen-token の pgtype 依存の置換。dbsnapshot の pgx 内省→SQLite PRAGMA 書換は開発専用ツールのため優先度低。

**詳細所見:**

- **【low】(1) scs/pgxstore → scs/sqlite3store 切替の成立性**: 現状 `session_auth.go:29` で `sm.Store = pgxstore.New(pool)`、`NewSessionManager(pool *pgxpool.Pool, ...)` が pgxpool.Pool を受ける。pgxstore.New は `func New(pool *pgxpool.Pool)` で pgxpool 固定。一方 `alexedwards/scs/sqlite3store` の New は標準 `*sql.DB` を取りドライバ非依存のため、`sql.Open("sqlite", path)` で modernc を登録（driver 名 `sqlite`）すれば CGO 不要で動く。NewSessionManager の引数型を `*pgxpool.Pool` → `*sql.DB` に変える小規模シグネチャ変更で対応可。scs API（RenewToken/Put/GetInt64 等, Login/Logout/UserIDFromSession）は Store 抽象の上なのでロジックは無変更。sessions DDL は data BYTEA→BLOB、expiry TIMESTAMPTZ→REAL へ。sessions は sqlc 対象外なので生成コードへの波及なし。根拠: `session_auth.go:27-31`, `00007:6-13`, go.mod, pgxstore.go:33
- **【low】(2) 統合テストの docker PG 依存**: cmd/server の統合テスト6本はすべて fakeUserQuerier/fakeDeviceQuerier の in-memory 実装と scs.New()（メモリストア）で newHTTPHandler を直接叩いており、docker PG に一切接続しない。CSRF往復・ログイン・セッション・所有者認可まで全て fake で検証済み。docker を実際に必要とするのは make seed / gen-token / db-snapshot / 手動 make up の開発ランタイム経路のみ。SQLite 化すればこれらも tmpfile DB で docker 完全不要になる利点が大きい。根拠: `integration_test.go:89-95`, `device_integration_test.go:122-136`, `docker-compose.yml:1-22`
- **【med】(3) internal/dbsnapshot の pgx 内省→SQLite PRAGMA 書換**: introspect.go は pg_class/pg_attribute/pg_index/pg_constraint/pg_indexes 等 PostgreSQL catalog 専用クエリで全テーブル/カラム/PK/索引/CHECK を内省し、Querier interface（pgx.Rows を返す Query）に依存。SQLite 化では sqlite_master + PRAGMA table_info/index_list/index_info/foreign_key_list への全面書換が必要。ただし dbsnapshot は AI/新規参入者向けドキュメント生成の開発専用ツールであり本番・現場 exe に同梱不要。優先度は低く、移行初期はスキップ可。根拠: `introspect.go:8-15,...`, `cmd/db-snapshot/main.go:53-59`, `scripts/db-snapshot.sh:3-7`
- **【med】(4) cmd/seed の pgtype 依存**: cmd/seed/main.go は pgxpool.Pool・pool.Exec で PG専用 `TRUNCATE ... RESTART IDENTITY CASCADE`・pgtype.Numeric/Timestamptz の自前ヘルパに依存。SQLite には RESTART IDENTITY/CASCADE が無いので `DELETE FROM ...` + `DELETE FROM sqlite_sequence` 等へ書換要。pgtype 構築は sqlc engine 切替後の新型に合わせて差し替え。開発専用ツールで本番 exe 非同梱。根拠: `cmd/seed/main.go:30,50,102-116,264-274`
- **【med】(4b) cmd/gen-token の pgtype 依存（現場運用上重要）**: gen-token は ESP32 に設定する Bearer トークン発行 CLI で、現場オフライン運用の鍵。pgtype.Timestamptz を expires_at 構築に直接使用し infradb.NewPool(pgx) に依存。SQLite 化では NewPool 経路と pgtype を置換。abilities は JSONB→生成 model では []byte で扱われており、TEXT 列へ DDL 変更すれば []byte マッピングはそのまま通る。トークン認証ロジック自体は DB 型非依存で無変更。**現場で使う点に留意**。根拠: `cmd/gen-token/main.go:24,59,72-86`, `models.go:72`, `00003:10`
- **【low】(5) config の DATABASE_URL → SQLite ファイルパス設定 + docker-compose 不要化**: config.go は DATABASE_URL（必須・空ならエラー）と SESSION_SECRET を env から読む。SQLite 化では DATABASE_URL を `sqlite:///path/to/iot.db` 形式 or 新キー DB_PATH に解釈変更し、NewPool を sql.Open へ置換。Makefile の up/down（docker-compose）・migrate-up/down/status の goose dialect を sqlite3 へ。docker-compose.yml は削除可。SESSION_SECRET は CSRF 用に維持。単一 exe 化では DB ファイルを実行ファイル隣 or %APPDATA% に置く既定パスのフォールバックを config に足すとゼロ設定起動できる。根拠: `config.go:26-39`, `pool.go:14-42`, `Makefile:19-22,40-51`
- **【low】(6) 圃場オフライン構成（ESP32→ノートPC Bearer POST）の成立性**: **成立する**。main.go:58 で `srv.Addr = :<APP_PORT>`（全 NIC で listen）なので ノートPC の LAN IP:port に ESP32 が WiFi 経由で到達可能、インターネット不要。POST /api/sensor-data は `engine.Group("/api", deviceAuth)` 配下で CSRF 対象外・DeviceAuth は Authorization: Bearer をSHA-256ハッシュ照合し device_tokens を引くだけで外部サービス非依存。SQLite ローカル DB でもこの照合経路は無変更で動く。所有者認可・アラート同期評価も DB ローカル完結。ESP32 側はリクエスト先 IP を可変設定にする必要があるがファーム側の設定で本リポジトリ改修対象外。注意点: ノートPC の LAN IP が DHCP で変動しうるので固定IP or mDNS 運用を現場手順として用意すると堅牢。根拠: `main.go:58,124-129`, `device_auth.go:42-77`, `sensor_api.go:100,131`
- **【low】(参考) pgx.ErrNoRows sentinel の移植性**: 認証・セッション経路を含め、`pgx.ErrNoRows` を errors.Is で判定する箇所が非テスト8ファイル14箇所（device_auth.go:63, ownership.go, sensor_api.go:105, auth.go, dashboard.go ほか）。sqlc SQLite engine へ切る場合 sql.ErrNoRows へ一括置換 or 両対応ヘルパ化が必要。観点E単体では device_auth.go:63 の1箇所だが、移行全体では機械的だが見落とすと 500 誤返しになる。根拠: `device_auth.go:63`, `sensor_api.go:105`

**必要な変更:**
1. session_auth.go: NewSessionManager の引数型を *pgxpool.Pool → *sql.DB に変更し、pgxstore.New(pool) を sqlite3store.New(db) に差し替え。Login/Logout/UserIDFromSession のロジックは無変更で流用可
2. go.mod: alexedwards/scs/sqlite3store を require に昇格。modernc.org/sqlite を direct 化
3. 00007_create_sessions.sql: data BYTEA→BLOB、expiry TIMESTAMPTZ→REAL、sessions_expiry_idx は維持。goose dialect を sqlite3 で再適用
4. 00001-00006: BIGSERIAL→INTEGER PK、NUMERIC(5,2)→REAL、TIMESTAMPTZ→datetime affinity、JSONB→json affinity/TEXT、DEFAULT NOW()→CURRENT_TIMESTAMP 等へ書換
5. config.go: DATABASE_URL を SQLite ファイルパス解釈へ変更（or DB_PATH 追加）。単一 exe 用に既定パスのフォールバックを追加
6. pool.go: pgxpool を sql.Open("sqlite", path)+SetMaxOpenConns 等へ置換（SQLite は単一 writer のため接続数方針見直し）
7. cmd/server/main.go: NewPool 戻り値型変更に追従、NewSessionManager への引数を *sql.DB に、health の ping を sql.DB.PingContext へ
8. cmd/seed: pgxpool→sql.DB、TRUNCATE→DELETE+sqlite_sequence、pgtype ヘルパ置換
9. cmd/gen-token: pgxpool→sql.DB、pgtype.Timestamptz を移行後型へ置換（現場運用 CLI のため確実に対応）
10. internal/dbsnapshot + cmd/db-snapshot: pg catalog→sqlite_master+PRAGMA。開発専用ツールのため優先度低・後回し可
11. Makefile + docker-compose.yml: goose の postgres→sqlite3、docker 関連削除/置換
12. sqlc.yaml + 生成 repository / pgtype 依存ファイル: engine 切替に伴う再生成と置換（移行全体の最大コスト中心）
13. pgx.ErrNoRows の errors.Is 判定を sql.ErrNoRows へ移植

**未解決の問い:**
- scs/sqlite3store はバックグラウンド cleanup goroutine が定期 DELETE を走らせる。SQLite の単一 writer ロックと WAL モード設定（busy_timeout, PRAGMA journal_mode=WAL）を明示しないと、ESP32 の同時 INSERT と cleanup/Web UI 読みが SQLITE_BUSY で衝突しうる
- DATABASE_URL を SQLite パスへ転用するか、新キー（DB_PATH）を導入するか
- 単一 exe 化で DB ファイルの配置先と初回起動時の自動 migrate の方針
- ESP32 から見たノートPC の宛先 IP は DHCP で変動しうる。固定IP / mDNS / 設定UI のいずれを現場手順とするか（運用設計の論点）
- sqlc を SQLite engine に切替えると abilities や NUMERIC の生成型が変わり pgconv も再設計が必要。まず sqlc engine 方針の確定が前提

**敵対的検証（同意: no / yellow へ補正）:**
- *見落とし*: (1) **scs/sqlite3store はモジュールキャッシュに存在しない**（新規 go get が必須。modernc は go.sum 解決済みで追加fetch不要なのとは異なる）。(2) **「統合テストは docker 非依存ゆえ移行コストはゼロ」は過大評価**。pgx./pgtype./pgconv. を参照するテストは16ファイル。fake 自身が pgx.ErrNoRows を返し、戻り値の struct は pgtype フィールドを持つ。sqlc engine 切替で全 fake の戻り型と sentinel が破綻するため、テスト群は無視できない実移行コスト。**docker非依存 ≠ 移行非依存**。(3) sqlc SQLite engine の query 書換ブロッカー（RETURNING 15箇所、`::` キャスト、`sqlc.narg()::BIGINT`、DATE() 集計、NOW() 28箇所）を「別観点」へ丸投げ。codegen 自体が失敗しうる。(4) DDL 側の SQLite 非互換要素の網羅不足（COMMENT ON 全削除・複合INDEX DESC・JSONB→TEXT・BIGSERIAL→INTEGER AUTOINCREMENT・TIMESTAMPTZ→TEXT/INTEGER の全テーブル書換、00003-00007 で計68箇所）。
- *訂正*: (1) summary の「実現可能性が高い（green）」「移行コストはゼロ/既に達成」という framing は楽観的すぎる。最大コスト中心を繰り返し「観点E外/別観点」へ退避させた上で残渣を green と判定しており、**総合は yellow が妥当**。(2) 統合テストは scs メモリストアを使う点は正しいが pgx 型依存は残り、sqlc engine 切替時に16テストファイルの修正が必要。(3) finding(1) は sqlite3store の実在を確認しておらず go get 前提を明記すべき（API 仮説 `New(*sql.DB)` 自体は妥当）。(4) 追加すべき項目: sqlc.yaml の SQLite engine 切替で codegen が通るかの事前 PoC、全テスト fake の pgtype/ErrNoRows 置換、DDL の COMMENT ON 全削除等、scs/sqlite3store の go get。(5) WAL/SQLITE_BUSY 懸念（scs cleanup × ESP32 同時 INSERT × Web UI 読み）は的確で、`PRAGMA journal_mode=WAL` + busy_timeout + `SetMaxOpenConns(1)` 方針の決定が必須（pool.go の MaxConns=10/MinConns=2 は SQLite 単一writer には不適切で要見直し）。

---

## 4. 統合判定

### 4.1 総合評価: **CONDITIONAL_GO**

**根拠:** 技術的には2件の改修（SQLite化 / 単一Windows .exe化）とも実現可能。決定的な追い風を実機検証で確認した:
1. 現状の cmd/server は既に `CGO_ENABLED=0 GOOS=windows GOARCH=amd64` で 37.4MB の .exe をクロスコンパイル成功し、`import "C"` もゼロ。
2. modernc.org/sqlite v1.46.1（純Go）が go.mod 依存グラフに既存で、同条件でビルド成功。
3. 複数の検証で「red 寄り」の主因とされた **scs/sqlite3store の CGO 衝突説は誤り**で、sqlite3store のソースは `database/sql/fmt/log/time` のみ import する純Go実装でありドライバ非依存=modernc と直結でき、要求スキーマ（sessions: token TEXT PK / data BLOB / expiry REAL）も単純。これにより単一exe化（観点D）は green、セッションストア差替（観点E）も low difficulty で確定。

GO ではなく CONDITIONAL_GO とする理由は、SQLite化の本体コストが「スキーマ書換」ではなく **「ドライバ層・型層の総入れ替え」** にあり、前提条件を解消しない限り着手しても動作しないため。具体的には sqlc を engine=sqlite/database/sql へ切替えると生成型が `pgtype.Numeric/Timestamptz/Date` → `float64/time.Time/sql.Null*` 系へ全面変化し、これに依存する非生成・非テストの7ファイル（handler 3 / pgconv / service / seed / gen-token）+ テスト17ファイル + pgx.ErrNoRows 10箇所が連鎖改修対象となる。加えて db/queries の PG固有構文と migrations の PG方言を sqlc generate が通る形へ手書き書換しないと codegen 自体が失敗する。これらは「再生成すれば足りる」範疇を超えるが、いずれも確立した変換パターンがあり構造的障壁ではない。

良い緩衝材として **view層が AST テストで pgtype 非依存を強制済み・WithTx 呼び出し元ゼロ・統合テストが docker非依存（in-memory fake）** である点が改修面を限定する。総じて中規模だが見通しの立つ改修であり、**PoC（sqlc SQLite codegen の実通過確認）を gate に置けば GO 可能**。

### 4.2 着手前ブロッカー（重大度順）

**【rank 1 / high】sqlc engine=sqlite での codegen 実通過が未検証**
db/queries に PostgreSQL 固有構文が残ったままでは sqlc generate が失敗する: `::`キャスト（sensor_readings.sql:33/36/39/50/53/56/72, alert_histories.sql:50/60/65）、`sqlc.narg('device_id')::BIGINT`（alert_histories.sql:50/65）、NOW()（5ファイル）、`DATE(recorded_at)`集計（sensor_readings.sql:32/44/45）、RETURNING（6ファイル）。SQLite parser は `::` トークン自体を持たないため、これらは「再生成で吸収」ではなく必須の手書き書換であり、NUMERIC/集計/DATE の最終生成型（float64 か sql.Null* か interface{}）が下流改修量を左右する。
- *緩和策*: 最初に小さな PoC（`sqlite-migration` スペックの Task 0）で、(a) migrations を SQLite方言へ書換 (b) queries の `::` を CAST へ・NOW() を CURRENT_TIMESTAMP/datetime('now') へ・narg を `CAST(sqlc.narg('device_id') AS INTEGER) IS NULL` へ・DATE() を date() へ書換 (c) sqlc.yaml を engine:sqlite へ変更 (d) `go tool sqlc generate` を実走し、NUMERIC/MAX/MIN/AVG/RETURNING/部分INDEX の生成型を実機確定する。型が確定してから下流（pgconv/handler/test）を着手することで手戻りを防ぐ。

**【rank 2 / high】型層の総入れ替え波及**
sqlc再生成で pgtype.Numeric/Timestamptz/Date が消え、非生成・非テストの7ファイル（handler/dashboard.go・device_show.go・readings.go、service/alert_evaluator.go、infra/pgconv/pgconv.go、cmd/seed、cmd/gen-token）+ テスト17ファイル + main.go の配線（repository.New が *pgxpool.Pool→*sql.DB、health の pool.Ping→PingContext、NewSessionManager 引数型）が一斉にコンパイルエラー化する。特に **device_show.go:328 aggregateToFloat の pgtype.Numeric case が dead化し default の 0 フォールバックでグラフが黙って平坦化する silent failure リスク**。
- *緩和策*: 変換ハブ pgconv.go を新型前提に再実装（または削除して直アクセス化）し、変換を1箇所に集約してシグネチャ波及を最小化する。MAX/MIN 集計列に明示 `CAST(... AS REAL)` を付けて float64/sql.NullFloat64 に型を確定させ、aggregateToFloat の型スイッチを silent fallback でなく明示エラー/期待型に更新。pgx.ErrNoRows→sql.ErrNoRows は10箇所を一括置換。view層は AST テストで pgtype 非依存が保証済みのため下流は無改修で済む点を活かす。

**【rank 3 / med】日時TZ戦略と NUMERIC 精度の業務要件未確定**
SQLite に TZ型・固定小数点型が無く、DATE(recorded_at) 日次バケットが保存TZ（UTC運用）基準だと JST 日境界とズレる（現design でも Out of Boundary 既知）。NUMERIC(5,2) を REAL(float64)化すると閾値比較（alert_evaluator）の精度が pgtype.Numeric（big.Int+Exp）から変わる。
- *緩和策*: 温湿度値域（-40.00〜125.00 / 0.00〜100.00 / 閾値）では REAL 倍精度で2桁は安全表現可能、現行の float 往復・%.2f 表示ロジックを流用して REAL 採用が低コスト。日次バケットは `date(recorded_at,'+9 hours')` で JST 境界に明示補正するか、UTC据置きを許容するかを要件オーナーに1問確認。datetime/timestamp affinity 列で宣言し time.Time マッピングを維持する（TEXT宣言だと string化して改修増）。

**【rank 4 / med】デスクトップ起動UXの net-new 実装が未着手**
(a) 起動時マイグレーション自動適用（現状 goose は Makefile CLI のみ、Goコード内に SetDialect/SetBaseFS/Up・embed なし=ゼロから新規）、(b) db/migrations を go:embed 同梱（現状 embed は public/docs のみ）、(c) config の DATABASE_URL/SESSION_SECRET 必須ハードフェイル（config.go:37-38）を %LOCALAPPDATA% 既定パス・SESSION_SECRET 初回自動生成へ緩和、(d) 既定ブラウザ自動オープン、(e) WAL/busy_timeout/SetMaxOpenConns 設定。
- *緩和策*: goose v3.27.0 がライブラリAPI（SetBaseFS/SetDialect('sqlite3')/Up）を備え既に依存にあるため、新パッケージ internal/migrate に migrations を go:embed して起動時 Up を1箇所追加すれば確立（docs.go の embed 先例を踏襲）。ブラウザ自動オープンは rundll32/start で pure-Go維持（Wails/WebView2 は CGO+ランタイム依存で不採用）。SQLite 単一writer 対策として DSN に `_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)`、書込競合（ESP32 POST × Web UI × scs cleanup goroutine の定期DELETE）に備え SetMaxOpenConns を絞る。これらは .exe化スペックに集約。

**【rank 5 / low】開発・運用周辺ツールの PG依存**
cmd/seed の `TRUNCATE ... RESTART IDENTITY CASCADE`（SQLite非対応）、cmd/gen-token の pgtype（現場でESP32トークン発行に使う重要CLI）、internal/dbsnapshot の pg_catalog 内省、docker-compose.yml、Makefile の goose postgres dialect。
- *緩和策*: seed の TRUNCATE→`DELETE FROM` + `DELETE FROM sqlite_sequence` へ書換。gen-token は本番exe非同梱だが現場運用CLIのため確実に新型対応。dbsnapshot は開発専用ツールで本番非同梱のため sqlite_master+PRAGMA 移植は後回し可（初期は機能縮退許容）。Makefile の goose dialect を sqlite3 へ、docker-compose.yml は削除。これらは型層改修（rank2）の生成型確定後に機械的に追従できる。

### 4.3 cc-sdd スペック分割案

**2スペックに分割・厳密直列**を推奨（1本に束ねると型層改修と起動UX改修が混在しレビュー粒度が粗くなりすぎる）。

#### ① `sqlite-migration`（基盤・最優先 / depends: なし）
SQLite化の本体スペック。
1. （PoC兼基盤）db/migrations 7本を SQLite方言へ書換（BIGSERIAL→INTEGER PK、TIMESTAMPTZ→datetime affinity、NUMERIC(5,2)→REAL、JSONB→json affinity、BYTEA→BLOB、NOW()→CURRENT_TIMESTAMP、COMMENT ON 7本削除、devices 正規表現CHECK削除しアプリ検証 device.go:84 へ委譲。部分INDEX/通常CHECK/DESC複合INDEXは移植可で維持）
2. db/queries の `::` キャスト→CAST/除去・NOW()→datetime('now')・DATE()→date()・narg の ::BIGINT→CAST形へ書換
3. sqlc.yaml を engine:sqlite/sql_package削除へ変更し sqlc generate を実通過させ生成型を確定
4. pgconv.go を新型前提に再実装or削除
5. pgx.ErrNoRows→sql.ErrNoRows 10箇所
6. handler 3ファイル+service+pgtype依存コンシューマの型追従（MAX/MIN に明示CASTで silent平坦化を防止）
7. infra/db/pool.go を sql.Open('sqlite',...)へ・main.go の repository.New 配線と health Ping を *sql.DB へ
8. scs を pgxstore→sqlite3store（純Go確認済）へ差替・sessions DDLを sqlite3store要求スキーマ（token TEXT PK / data BLOB / expiry REAL）へ
9. テスト17ファイルの pgtype/ErrNoRows フィクスチャを新型へ
10. seed/gen-token の TRUNCATE・pgtype 書換

日時TZ・NUMERIC精度の要件確認を含む。

#### ② `desktop-exe-packaging`（後段固定 / depends: ①）
単一Windows .exe化スペック。
1. 新パッケージ internal/migrate に db/migrations を go:embed 同梱し起動時 goose.SetBaseFS+SetDialect('sqlite3')+Up を main の run内へ配線（初回起動でDBファイル自動作成→全テーブル自動適用）
2. config.go の DATABASE_URL/SESSION_SECRET 必須ハードフェイルをデスクトップ向け緩和（SQLiteパス既定=%LOCALAPPDATA%/go_iot/app.db、SESSION_SECRET 初回自動生成・永続化）
3. 起動後に既定ブラウザで http://localhost:PORT を自動オープン（rundll32/start で pure-Go維持、Wails不採用）
4. SQLite並行アクセス設定（DSN に WAL+busy_timeout、SetMaxOpenConns 見直し。ESP32 POST × Web UI × scs cleanup の競合対策）
5. Makefile に build-windows ターゲット（CGO_ENABLED=0 GOOS=windows GOARCH=amd64 -ldflags '-s -w -X Version'、任意で -H windowsgui+ログのファイルリダイレクト）
6. docker-compose.yml 削除・Makefile up/down 整理

圃場オフライン構成（ESP32→ノートPC Bearer POST は現状の :port listen + DeviceAuth で成立、宛先IPの固定IP/mDNS は運用手順）。

### 4.4 実装順序と既存実装への影響（sequence_note）

実装順序は **`sqlite-migration` → `desktop-exe-packaging` の厳密な直列を推奨**。`sqlite-migration` の内部でも **Task 0 として「migrations+queries の SQLite方言書換 + sqlc generate 実通過」を最初の gate** に置き、ここで生成型（NUMERIC/集計/DATE/RETURNING）を実機確定してから下流（pgconv/handler/test）へ進むことで手戻りを防ぐ。`desktop-exe-packaging` は SQLite で *sql.DB 配線と sqlite3store が動いて初めて embed マイグレーション・起動UX・ブラウザオープンが意味を持つため後段固定。

**既存 Web UI 実装（BE完成・FE進行中=alert-history まで実装済）への影響タイミングが最重要注意点**: SQLite化は repository 生成型・main.go 配線・pgconv・handler を広く触るため、FE（templ画面）の追加実装と並行すると衝突が起きやすい。view層は AST テストで pgtype 非依存が保証されており handler 整形層で吸収されるため、FE（templ/HTMX）側のマージ衝突は限定的だが、handler の戻り値整形（formatReadingText/aggregateToFloat 等）は両者が触る境界なので、進行中FE画面が一段落（キリの良いコミット）した時点で sqlite-migration を一気に通すのが安全。

**理想は「現行 FE 残作業を PG 構成のまま完了 → sqlite-migration を全 handler/test 横断で実施 → desktop-exe-packaging → 以降の FE/新機能は SQLite 前提」**。SQLite化を FE 完了前に挟む場合は、テストフィクスチャ17ファイルの新型書換が FE 側テストにも波及する点を見込むこと。

### 4.5 残存リスク（PoC/実機でのみ確定）

1. sqlc v1.30.0 の SQLite engine が NUMERIC(5,2)/AVG()::CAST/MAX/MIN 集計/DATE() 集計/RETURNING/部分INDEX/CAST化した narg を期待通り codegen するかは PoC 実走でのみ確定する（本調査はファイル根拠と modernc/scs のビルド・import 検証まで。sqlc generate 自体は未実走）。生成型が想定（float64/time.Time）と違えば下流改修量が変動する。
2. modernc.org/sqlite ドライバの time.Time パース挙動（CURRENT_TIMESTAMP は `YYYY-MM-DD HH:MM:SS` で T無し）と datetime/timestamp affinity 列の string/time.Time マッピングが現行の UTC保存+表示直前JST変換ロジックと噛み合うかは実機確認が必要。表記揺れで created_at/updated_at のパースが崩れる懸念。
3. 日次バケット（DATE）の JST境界補正（date(...,'+9 hours')）採用可否と NUMERIC精度の業務要件（閾値比較に厳密10進が必要か）は要件オーナー確認待ち。REAL採用で実害が出ないことの最終合意が必要。
4. SQLite 単一writer 環境での実運用同時性（ESP32 複数台の POST + アラート同期評価 + Web UI 読取 + scs cleanup goroutine の定期DELETE）が WAL+busy_timeout+接続数調整で十分かは負荷条件次第。圃場ノートPC単独・低同時実行前提なら許容見込みだがESP32台数が多いと要再評価。
5. コンソール窓を消す -H windowsgui を採る場合 log.Printf 出力先が失われるため、ログのファイルリダイレクト（%LOCALAPPDATA%配下）が別途必要。SESSION_SECRET 初回自動生成の平文ファイル保存の可否も運用判断。
6. dbsnapshot の SQLite 内省移植（sqlite_master+PRAGMA）は後回し可だが、CLAUDE.md が「実DB接続前に docs/database_snapshot を読む」を強制しているため、スナップショット生成が機能縮退したまま放置すると以後の cc-sdd 設計フェーズの権威スキーマ参照が陳腐化するリスク。
7. 実Windows実機（ESP32実機含む）での E2E 動作（.exe 配布→初回起動→自動マイグレーション→ブラウザ起動→ESP32 Bearer POST 到達）は本調査では未検証。クロスビルド成功はバイナリ生成までの保証であり、Windows ランタイム挙動・WAL ファイルロック・LAN到達性は実機検証が残る。

---

## 5. オーナー確認が必要な論点（GO する際に確定）

> **【2026-06-08 オーナー決定済み】以下3点はすべて確定。S9/S10 spec-init プロンプトに反映済み。**

1. **温湿度の NUMERIC 精度** → **REAL（float64往復・改修小）で確定**。現行 pgconv の float 往復・`%.2f` 表示ロジックを流用。スケール済 INTEGER 案は不採用。
2. **日次バケットの TZ** → **`date(recorded_at,'+9 hours')` で JST 境界に補正することで確定**（UTC据置きは不採用）。集計クエリの SELECT/GROUP BY/ORDER BY を同式統一し、消費側で二重補正しない。
3. **デスクトップ UX のスコープ** → **初版は「ブラウザ自動オープン」のみで確定**。systray常駐は初版スコープ外（将来拡張）。**コンソール窓非表示（`-H windowsgui`＋ログのファイル出力）も採用決定し、2026-06-08 に先行実装済み**（ログのファイル出力基盤 `internal/applog` を TDD 実装＝カバレッジ88.2%、`build-windows`/`build-windows-gui` ターゲット追加、GUI/console 両 .exe のクロスコンパイル確認済み。実 Windows での GUI モード実機検証のみ S10 に残る）。

---

## 6. 重要な論点メモ ── 「scs sqlite3store は CGO 必須」説の反証

調査の過程で、複数の検証エージェント（観点B・観点C・観点E）が **「scs/sqlite3store は mattn/go-sqlite3（CGO 必須）に依存するため、pure-Go 単一 .exe 要件と衝突する。自作ストアや JWT 等への変更が必要」** と懸念を表明し、これが「red 寄り」評価の主因となっていた。

統合判定と独立した Web 裏取りにより、この懸念は **事実誤認**であることが確定した:
- `alexedwards/scs/sqlite3store` のパッケージ実体は `database/sql` / `fmt` / `log` / `time` のみを import する純Go実装であり、**特定の SQLite ドライバには依存しない**。
- 公開 API は `func New(db *sql.DB) *SQLite3Store`（および `NewWithCleanupInterval`）で、呼び出し側が渡す `*sql.DB` のドライバを使う。`sql.Open("sqlite", path)`（modernc, driver 名 `sqlite`）で開いた `*sql.DB` を渡せば **CGO なしで動く**。
- README は mattn/go-sqlite3 でのテストを明記するが、これは「テスト環境」であって「依存」ではない。
- 要求スキーマは `sessions(token TEXT PRIMARY KEY, data BLOB NOT NULL, expiry REAL NOT NULL)` + expiry索引。`julianday()` を使うため **expiry は REAL**（一部エージェントが「DATETIME」と記載したのは不正確）。

唯一の軽微な留意点は、`sqlite3store` が内部で `$1`/`$2`/`$3` 形式のプレースホルダと `julianday()` を使う点。SQLite は `$N` を名前付き引数として解釈するため modernc でも動作する見込みだが、**実機で1回 INSERT/SELECT/DELETE を確認**しておくと万全（`sqlite-migration` スペックの検証タスクに含める）。

---

*本レポートは 2026-06-08 時点のコードベース（main ブランチ）に対する調査結果である。実装着手前に最新状態との差分を確認すること。*
