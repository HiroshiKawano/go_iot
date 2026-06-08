# Task 1 実装メモ — SQLite codegen ゲート（生成型の実機確定）

> 本書は tasks.md「1. Foundation: SQLite codegen ゲート」の観測可能完了条件
> 「NUMERIC/集計/DATE/RETURNING の生成型を実装メモに記録」に対応する実機記録。
> design.md の Revalidation Triggers が参照する「生成型の形」を確定値として固定する。
> 実走日: 2026-06-08 / sqlc v1.30.0(engine=sqlite) / goose v3.27(dialect=sqlite3) / modernc.org/sqlite v1.46.1

## 完了状態

- **1.1**: migrations 7本を SQLite 方言へ書換 → goose(sqlite3) で空インメモリ DB へ全7本適用でき、
  全7テーブル + 全13索引が作成される（`internal/infra/db/sqlite_migration_test.go` で green）。
- **1.2**: queries 6本書換 + sqlc.yaml engine=sqlite 切替 + `go tool sqlc generate` exit 0。
  再生成された `internal/repository` に pgtype/pgx 依存ゼロ（grep 0件・`no_pgtype_import_test.go` で green）。
- 生成型は `internal/repository/generated_types_test.go` でコンパイル時に型レベル固定済み。

> **注意（design 冒頭の既知事項）**: 1.2 で repository が新型再生成されたため、これを使う
> handler/service/cmd は一斉にコンパイルエラー（`go build ./...` は赤）。これは設計上の想定であり、
> 全体ビルド green は Task 6.1 のマイルストーン。本タスクの検証は repository/infra-db パッケージ単体で行う。

## 実機確定した生成型マッピング（design データ型マッピング表の「想定」を確定値で更新）

| 現行 PG 型/構文 | SQLite 方言 | sqlc 生成 Go 型（**実機確定**） |
|---|---|---|
| `BIGSERIAL PRIMARY KEY` / `BIGINT` | `INTEGER` | `int64` |
| `NUMERIC(5,2) NOT NULL`（temp/humidity/threshold/actual_value） | `REAL` | `float64` |
| `TIMESTAMPTZ NOT NULL` | `DATETIME` | `time.Time` |
| `TIMESTAMPTZ NULL`（deleted_at/last_communicated_at/expires_at/email_verified_at/last_used_at） | `DATETIME` | `*time.Time` |
| `VARCHAR NULL`（location） | `VARCHAR` | `*string` |
| `JSONB`（abilities） | `json` | `json.RawMessage` |
| `BYTEA`（sessions.data） | `BLOB` | `[]byte` |
| `TIMESTAMPTZ`（sessions.expiry） | `REAL` | `float64`（sqlite3store 要求） |
| `BOOLEAN` | `BOOLEAN` | `bool` |
| `AVG/MAX/MIN(x)::NUMERIC` → `CAST(... AS REAL)` | — | **`float64`**（NOT NULL 扱い・silent 平坦化防止 R5.1/5.3） |
| `COUNT(*)::BIGINT` → `COUNT(*)` | — | `int64` |
| `DATE(recorded_at)` → `CAST(date(recorded_at,'+9 hours') AS TEXT)` | — | **`string`**（JST 日次バケット R5.2） |
| `sqlc.narg('device_id')::BIGINT` → `CAST(sqlc.narg('device_id') AS INTEGER)` | — | `*int64` |
| `RETURNING *` | — | sqlc が全列を展開し対応モデル型を返す（CreateXxx → 各 model 型）。modernc 実機動作は Task 2 以降で検証（R-4） |

## 実装中に確定した重要な落とし穴（sqlc SQLite engine 固有・下流/再生成時の必読事項）

1. **`$1` プレースホルダ非対応**: sqlc の SQLite engine は PostgreSQL 形式の `$1` を受理しない
   （`no viable alternative at input 'VALUES ($'`）。→ 位置 `?` または `sqlc.arg()/sqlc.narg()` 名前付きへ全面置換。

2. **クエリ内マルチバイト(日本語)コメントでパーサが破綻**: `?`/`sqlc.arg` 等のパラメータ書換を伴うクエリで、
   sqlc の "edited query" 再パースがバイト/ルーンのオフセットずれを起こし、クエリ文字列を誤った位置で切断する
   （クエリ名が途中で切れる: `DeleteDeviceToken`→`DeleteDevice` 等）。
   → **`db/queries/*.sql` 内のクエリ説明コメントを削除**（`-- name:` ディレクティブのみ ASCII で維持）。
   削除した役割説明は下記「クエリ役割」へ転記して保全（language.md の日本語要求は本メモで充足）。

3. **`BETWEEN ? AND ?` の bind parameter 欠落バグ**: sqlc SQLite engine は BETWEEN の上下限プレースホルダを
   Params に取り込まない（from/to が欠落し実行時にプレースホルダ数不一致で壊れる）。
   → **`col BETWEEN X AND Y` を `col >= X AND col <= Y` に展開**して回避。
   展開により sensor_readings は `RecordedAt/RecordedAt_2`、alert_histories は `FromAt/ToAt` が Params へ復活し、
   既存 handler のフィールド名と一致（下流改修を型追従のみに抑制）。

4. **`date(...)` は `interface{}` 生成**: 関数戻り型を sqlc が推論できず `interface{}` になる。
   → `CAST(date(recorded_at,'+9 hours') AS TEXT)` で `string` を明示確定（SELECT/GROUP BY/ORDER BY を同一式統一）。

5. **空集合の `:one` 集計が NULL→float64 Scan で失敗**: `GetSensorReadingsSummary`（`:one`・GROUP BY なし）は
   対象期間が空集合のとき AVG/MAX/MIN が NULL を返し、`CAST(... AS REAL)` の `float64` への Scan が失敗する
   （移行前は pgtype.Numeric の Valid=false で保持できた・履歴画面のデータなし正常系が 500 になる回帰）。
   `HAVING COUNT(*) > 0` は sqlc SQLite parser が GROUP BY 無しでは拒否、CAST を外すと
   AVG=`*float64`/MAX・MIN=`interface{}` の混在になり不適。
   → **`GROUP BY device_id`** を付与（device_id は WHERE で固定＝最大1グループ）。空集合は 0 行＝`sql.ErrNoRows`、
   非空はグループ内に行があり `CAST` で `float64` 確定。型 `float64`（silent 平坦化防止）を保ちつつ空集合を安全化。
   実機固定: `internal/repository/aggregate_summary_test.go`。

6. **`id INTEGER PRIMARY KEY` に `AUTOINCREMENT` は付けない**（SQLite 推奨）。自動採番は機能するが、
   PostgreSQL の BIGSERIAL と異なり削除済み id を再利用しうる。id を外部公開・再利用前提にしていないため無害。
   なお AUTOINCREMENT 不使用のため `sqlite_sequence` テーブルは存在しない。seed の冪等化（PG の
   `TRUNCATE ... RESTART IDENTITY` 相当）は **`DELETE FROM <table>` で空化するだけ**で id が 1 から振り直される
   （`DELETE FROM sqlite_sequence` は「no such table」エラーになるため使わない）。実機固定: `cmd/seed/seed_test.go`。

7. **【重要・Task 7.2 で発覚した Task 1.2 由来の潜在バグ】modernc は `time.Time` を Go の `String()` 形式で
   TEXT 格納し、SQLite の `date()`/`strftime()` が解釈できず NULL を返す**。
   modernc.org/sqlite は `time.Time` バインドを `"2006-01-02 15:04:05.999999999 -0700 MST"`
   （例: `2026-06-01 13:00:00 +0000 UTC`）で格納する。一方 read 時は DATETIME affinity 列を RFC3339
   （`2026-06-01T13:00:00Z`）へ**再変換して返す**ため Go 側からは正常に見え、発見が遅れた。
   この格納値を `date(recorded_at, '+9 hours')` に渡すと**先頭が parse 不能で NULL**になり、
   日次集計（7日/30日グラフ＝`ListDailySensorAggregates`）が静かに壊れていた（migration 前の検証は
   比較クエリのみで date() 経路を踏んでいなかった）。
   → **`date(substr(recorded_at, 1, 19), '+9 hours')`** で先頭19桁 `YYYY-MM-DD HH:MM:SS`（UTC 壁時計・
   ナノ秒/オフセット手前まで）を抽出してから date() に渡す。SELECT/GROUP BY/ORDER BY の3箇所を同一式へ統一済み。
   比較系（`recorded_at >= ? AND recorded_at <= ?`）は文字列辞書順で正しく動くため影響なし（date() を使うクエリのみ該当）。
   アプリは recorded_at を常に UTC で保存する前提（substr 抽出値を UTC 壁時計として扱う）。実機固定: `internal/repository/daily_aggregates_jst_test.go`。

## 削除した日本語クエリ説明コメントの役割転記（落とし穴2の保全）

- `device_tokens.GetDeviceTokenByHash`: デバイスからの Bearer リクエスト受信時に token_hash で検索し認証に使用
- `alert_rules.ListEnabledAlertRulesByDevice`: アラート判定ロジック（センサー受信時の同期処理）で使用
- `sensor_readings.GetLatestSensorReading`: ダッシュボードでデバイスごとの最新値表示に使用
- `sensor_readings.ListLatestSensorReadings`: デバイス詳細の最新計測テーブル用（最新10件・降順・期間非連動）
- `sensor_readings.ListRecentSensorReadings`: 24時間グラフ用（指定時刻以降の生データを昇順）
- `sensor_readings.ListDailySensorAggregates`: 7日/30日グラフ用（日別の平均/最大/最小集計・JST 日境界）
- `sensor_readings.GetSensorReadingsSummary`: センサーデータ履歴画面の集計ボックス用
- `sensor_readings.ListSensorReadingsPaginated`: センサーデータ履歴画面のテーブル用（期間指定 + ページング）
- `alert_histories.CreateAlertHistory`: 発火時に metric/operator/threshold を alert_rules から非正規化して保存
- `alert_histories.ListUnnotifiedAlertHistoriesWithDevice`: ダッシュボードのアラート通知バナー表示用
- `alert_histories.ListAlertHistoriesPaginated`: アラート履歴画面の一覧用（デバイス + 期間フィルタ + ページング）

## 下流タスクへの申し送り

- **Task 3（pgconv 再実装・完了）**: NULL 許容列は `*time.Time`/`*string` 生成 → ヘルパは `*T` 受領で確定
  （`sql.NullT` ではない。emit_pointers_for_null_types 維持の結果）。NUMERIC=float64・datetime=time.Time は直接来る。
  pgconv は `Format2`(%.2f 表示)・`Quantize2`(保存時2桁量子化)・`TimeOrZero`(NULL 合体) の3関数へ痩せ実装済み。
  - **【Task 4.1 必須】保存時の数値量子化(R4.1)**: 旧 sensor_api/seed は `pgconv.Numeric2` で保存時に2桁化していた。
    SQLite の REAL は丸めず float64 を保持するため、**温度/湿度/閾値/実測値の INSERT/UPDATE 直前に `pgconv.Quantize2`**
    を適用し「移行前と同一の値として保存」を保つこと。未適用だと SHT31 の3桁目入力で表示が偶数丸め差(±0.01)を
    起こす(code-review HIGH)。表示は `pgconv.Format2`(%.2f)。
- **Task 4（handler 型追従）**: フィールド名は維持されたため改修は型のみ。
  - sensor_readings: `RecordedAt/RecordedAt_2` は `pgtype.Timestamptz` → `time.Time`、`Limit/Offset` は `int32` → `int64`。
  - alert_histories: `FromAt/ToAt` は `time.Time`、`DeviceID` は `*int64`、`LimitN/OffsetN` は `int64`。
  - `ListDailySensorAggregatesRow.ReadingDate` は `string`（Task 4.3 の monthDayLabel を string 入力へ）。
  - 集計 `*Row` の Avg/Max/Min は `float64`（Task 4.3 の aggregateToFloat は float64 を期待型として受理）。
  - **`GetSensorReadingsSummary` は空集合期間で `sql.ErrNoRows` を返す**（GROUP BY device_id 化の結果）。
    Task 4.2/4.4 の handler は ErrNoRows を「データなし(空サマリ表示)」にマップし 404 にはしないこと
    （旧 `buildSummary` の `SampleCount == 0` 分岐の置換に相当）。
    **観測可能完了条件に「対象期間が空集合でも 500 にならず移行前と等価な空表示になる」を含めること**（code-review HIGH 指摘）。
