
---

## cc-sdd参照ガイド

本設計書をcc-sdd（詳細設計書）から参照する際に価値の高いセクションと用途を示す。

| 優先度 | セクション | cc-sddでの用途 |
|:------:|-----------|---------------|
| ★★★ | [Enum定義](#enum定義) | ビジネスロジック実装の核。`Label()` / `Unit()` / `Evaluate()` のシグネチャ確認 |
| ★★★ | [テーブル定義](#テーブル定義) | sqlc 生成構造体・validator バリデーションタグ・goose マイグレーションの仕様根拠 |
| ★★★ | [主要クエリパターン](#主要クエリパターン) | sqlc クエリ / Service 層の実装仕様。グラフ・アラート判定ロジックの参照元 |
| ★★★ | [sqlc リレーション方針](#sqlc-リレーション方針) | JOIN / 先行取得クエリの命名方針。Handler・Service での呼び出し根拠 |
| ★★ | [設計方針](#設計方針) | 「外部キー非設定」「論理削除適用範囲」「集計テーブルなし」など実装判断の根拠 |
| ★★ | [Go 構造体でのフィールド型](#go-構造体でのフィールド型) | `pgtype.Numeric` / `pgtype.Timestamptz` の扱い、`pgconv` ヘルパーの使用根拠 |
| ★★ | [アラート判定の実行タイミング](#アラート判定の実行タイミング) | Handler 内で同期実行する方針の根拠。Queue 不使用の理由 |
| ★ | [バリデーションルール定義](#バリデーションルール定義) | `go-playground/validator` タグの定義とセンサー値の範囲バリデーション定義 |
| ★ | [データフロー対応表](#データフロー対応表) | ユースケース別の処理フロー設計の参照元 |
| ★ | [テーブル定義 > インデックス](#テーブル定義) | 各テーブルの「インデックス:」欄。複合インデックスとクエリの対応関係 |

> ※ cc-sddの sqlc クエリ / Service 層 / バリデーションを記述する際は、まず本書の Enum 定義・テーブル定義・sqlc リレーション方針を確認すること。

### 次回プロジェクトでの記載チェックリスト

DB設計書を新規作成する際に以下が揃っているか確認する：

- [ ] 全テーブルのカラム定義（PostgreSQL 型・制約・インデックス）を記載
- [ ] マスターデータとして使用する Enum の値・`Label()`・追加メソッド（`Unit()` / `Evaluate()` 等）を Go 用に定義
- [ ] Enum カラムを持つテーブルの `CHECK` 制約（PostgreSQL 標準）を記載
- [ ] テーブル間の JOIN / 先行取得クエリの sqlc 関数名を一覧化
- [ ] バリデーションルール（`binding:"required,oneof=temperature humidity"` / `gte=-40,lte=125` 等）を記載（実行方式は実装設計側で定義）
- [ ] 主要クエリパターン（グラフ表示 / 期間集計 / アラート判定等）を SQL / sqlc で記載
- [ ] 設計方針（外部キー非設定 / 論理削除適用範囲 / 集計テーブルの有無）を明記
- [ ] 非同期 / 同期処理方針（Queue 使用の有無 / 実行タイミング）を明記
- [ ] データフロー対応表（ユースケース別の関連テーブル）を記載

---

# 農業IoTシステム DB設計書

## DB接続設定

- **RDBMS**: PostgreSQL 16+
- **Docker コンテナ**: `go_iot_db`（`docker-compose.yml` で定義）
- **接続設定（`.env`）**:
  ```
  DATABASE_URL=postgres://go_iot:go_iot_dev@localhost:5432/go_iot?sslmode=disable
  ```
- **マイグレーション適用**: `make migrate-up` (goose)
- **sqlc コード生成**: `make sqlc`（`db/queries/*.sql` → `internal/repository/*.go`）

---

## 設計方針

- **リレーション**: 外部キー制約はDB上に設定しない。参照整合性はアプリケーション層（sqlc クエリ + Service）で管理する
- **削除戦略**: 主要テーブルに論理削除を採用。`deleted_at` カラムで管理する（適用範囲は下記「論理削除の適用範囲」を参照）。sqlc クエリでは常に `WHERE deleted_at IS NULL` を付与する
- **マスターデータ**: DB テーブルではなく Go の型付き定数（`type X string` + const）で管理する。DB カラムには `VARCHAR` として値を格納し、CHECK 制約で範囲を保証する
- **認証**: ESP32 は自作 Bearer トークンミドルウェア + `device_tokens` テーブル、ブラウザは Session（`alexedwards/scs` + PostgreSQL セッションストア）
- **集計テーブル**: 持たない。24 時間グラフは `sensor_readings` の生データ取得、7 日/30 日グラフは日次集計クエリ（`GROUP BY DATE(recorded_at)`）で実装する
- **UNIQUE 制約と論理削除**: soft-deleted 行との衝突を避けるため、UNIQUE は部分インデックス `WHERE deleted_at IS NULL` で定義する

### 論理削除の適用範囲

| テーブル | 論理削除 | 理由 |
|---------|:-------:|------|
| users | - | 物理削除で運用 |
| devices | 適用 | デバイス履歴の保全 |
| device_tokens | - | 物理削除で運用 (失効は expires_at で管理) |
| sensor_readings | 適用 | 計測データの保全 |
| alert_rules | 適用 | ルール変更履歴の追跡 |
| alert_histories | 適用 | 通知履歴の保全 |

---

## ER図

```mermaid
erDiagram
    users ||--o{ devices : "所有"
    users ||--o{ device_tokens : "発行"
    devices ||--o{ sensor_readings : "計測"
    devices ||--o{ alert_rules : "設定"
    alert_rules ||--o{ alert_histories : "発火"

    users {
        bigserial id PK
        varchar name
        varchar email UK
        varchar password_hash
        timestamptz email_verified_at
        timestamptz created_at
        timestamptz updated_at
    }

    devices {
        bigserial id PK
        bigint user_id
        varchar name
        varchar mac_address "部分UNIQUE (deleted_at IS NULL)"
        varchar location
        boolean is_active
        timestamptz last_communicated_at
        timestamptz created_at
        timestamptz updated_at
        timestamptz deleted_at
    }

    device_tokens {
        bigserial id PK
        bigint user_id
        varchar name
        varchar token_hash UK "SHA-256 hex(64)"
        jsonb abilities
        timestamptz last_used_at
        timestamptz expires_at
        timestamptz created_at
        timestamptz updated_at
    }

    sensor_readings {
        bigserial id PK
        bigint device_id
        numeric temperature "-40..125"
        numeric humidity "0..100"
        timestamptz recorded_at
        timestamptz created_at
        timestamptz updated_at
        timestamptz deleted_at
    }

    alert_rules {
        bigserial id PK
        bigint device_id
        varchar metric "CHECK: temperature/humidity"
        varchar operator "CHECK: > < >= <="
        numeric threshold
        boolean is_enabled
        timestamptz created_at
        timestamptz updated_at
        timestamptz deleted_at
    }

    alert_histories {
        bigserial id PK
        bigint alert_rule_id
        varchar metric
        varchar operator
        numeric threshold
        numeric actual_value
        boolean is_notified
        timestamptz triggered_at
        timestamptz created_at
        timestamptz updated_at
        timestamptz deleted_at
    }
```

> ※ ER 図のリレーション線は sqlc JOIN クエリでの論理関係を示す。DB 上の外部キー制約は設定しない。

---

## Enum定義

マスターデータは DB テーブルではなく Go の型付き文字列定数で管理する。
DB カラムには `VARCHAR` として値を格納し、CHECK 制約で許容値を制限する。

### Metric（計測指標）

`internal/domain/metric.go`

```go
package domain

type Metric string

const (
    MetricTemperature Metric = "temperature"
    MetricHumidity    Metric = "humidity"
)

func (m Metric) Label() string {
    switch m {
    case MetricTemperature:
        return "温度"
    case MetricHumidity:
        return "湿度"
    }
    return string(m)
}

func (m Metric) Unit() string {
    switch m {
    case MetricTemperature:
        return "℃"
    case MetricHumidity:
        return "%"
    }
    return ""
}

func (m Metric) Valid() bool {
    switch m {
    case MetricTemperature, MetricHumidity:
        return true
    }
    return false
}

func ParseMetric(s string) (Metric, error) { /* ... */ }
func AllMetrics() []Metric                  { /* ... */ }
```

| 値 | ラベル | 単位 |
|----|--------|------|
| temperature | 温度 | ℃ |
| humidity | 湿度 | % |

---

### ComparisonOperator（比較演算子）

`internal/domain/comparison_operator.go`

```go
package domain

type ComparisonOperator string

const (
    OpGreaterThan        ComparisonOperator = ">"
    OpLessThan           ComparisonOperator = "<"
    OpGreaterThanOrEqual ComparisonOperator = ">="
    OpLessThanOrEqual    ComparisonOperator = "<="
)

func (op ComparisonOperator) Label() string {
    switch op {
    case OpGreaterThan:
        return "より大きい"
    case OpLessThan:
        return "より小さい"
    case OpGreaterThanOrEqual:
        return "以上"
    case OpLessThanOrEqual:
        return "以下"
    }
    return string(op)
}

// Evaluate は actual を threshold と比較し、演算子の条件が成立するかを返す。
// アラート判定ロジックの中核。
func (op ComparisonOperator) Evaluate(actual, threshold float64) bool {
    switch op {
    case OpGreaterThan:
        return actual > threshold
    case OpLessThan:
        return actual < threshold
    case OpGreaterThanOrEqual:
        return actual >= threshold
    case OpLessThanOrEqual:
        return actual <= threshold
    }
    return false
}
```

| 値 | ラベル | 説明 |
|----|--------|------|
| > | より大きい | 閾値を超過 |
| < | より小さい | 閾値を下回る |
| >= | 以上 | 閾値以上 |
| <= | 以下 | 閾値以下 |

---

### Go 構造体でのフィールド型

sqlc は `NUMERIC` を `pgtype.Numeric`、`TIMESTAMPTZ` を `pgtype.Timestamptz` として生成する。
Service / Handler 層で float64 / time.Time として扱う際は `internal/infra/pgconv` のヘルパーを使う。

```go
// internal/repository/models.go (sqlc 生成)
type AlertRule struct {
    ID        int64
    DeviceID  int64
    Metric    string             // domain.Metric にキャストして使用
    Operator  string             // domain.ComparisonOperator にキャストして使用
    Threshold pgtype.Numeric     // pgconv.NumericToFloat() で float64 へ
    IsEnabled bool
    CreatedAt pgtype.Timestamptz // pgconv.TimestamptzToTime() で time.Time へ
    UpdatedAt pgtype.Timestamptz
    DeletedAt pgtype.Timestamptz // NULL 可
}

// Handler での使い方
import "github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
import "github.com/HiroshiKawano/go_iot/internal/domain"

m := domain.Metric(rule.Metric)
op := domain.ComparisonOperator(rule.Operator)
threshold := pgconv.NumericToFloat(rule.Threshold)

fired := op.Evaluate(actual, threshold)
label := m.Label() + op.Label() + fmt.Sprintf("%.2f", threshold) + m.Unit()
```

---

## バリデーションルール定義

> **cc-sdd への価値:**
> Enum 値を含むカラムのバリデーションには `binding:"oneof=..."` タグが必要。テーブル定義の型情報だけでは導出しにくい。センサー値の物理的な範囲（温湿度の上下限）もここで定義する。

> **注意:** 本セクションはバリデーション**ルール**（何を検証するか）を定義する。バリデーションの**実行方式**（Gin の `ShouldBind` 系 API が `binding` タグを解釈して内蔵 `go-playground/validator` を起動）は `2cc_sdd/HTMX実装ガイド(動的).md` の「バリデーションエラー表示」を参照。

### AlertRule 用（Web UI フォーム）

```go
// internal/handler/alert_rule.go
type CreateAlertRuleRequest struct {
    DeviceID  int64   `form:"device_id" binding:"required,min=1"`
    Metric    string  `form:"metric"    binding:"required,oneof=temperature humidity"`
    Operator  string  `form:"operator"  binding:"required,oneof=> < >= <="`
    Threshold float64 `form:"threshold" binding:"required"`
    IsEnabled bool    `form:"is_enabled"`
}
```

### SensorReading 用（ESP32 からの API 受信）

```go
// internal/handler/sensor_api.go
type CreateSensorReadingRequest struct {
    DeviceID    int64     `json:"device_id"    binding:"required,min=1"`
    Temperature float64   `json:"temperature"  binding:"gte=-40,lte=125"`
    Humidity    float64   `json:"humidity"     binding:"gte=0,lte=100"`
    RecordedAt  time.Time `json:"recorded_at"  binding:"required"`
}
```

---

## テーブル定義

### 1. users（ユーザー）

Web UI の Session 認証対象。パスワードは bcrypt でハッシュ化して保存する。

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| id | BIGSERIAL | PK | 自動採番 |
| name | VARCHAR(255) | NOT NULL | ユーザー名 |
| email | VARCHAR(255) | NOT NULL, UNIQUE | メールアドレス |
| password_hash | VARCHAR(255) | NOT NULL | bcrypt ハッシュ |
| email_verified_at | TIMESTAMPTZ | NULLABLE | メール確認日時 |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |

> ※ 物理削除で運用（論理削除は適用しない）。

---

### 2. devices（デバイス）

ESP32 デバイスの管理テーブル。

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| id | BIGSERIAL | PK | |
| user_id | BIGINT | NOT NULL | 所有ユーザー（users.id） |
| name | VARCHAR(255) | NOT NULL | デバイス名（例: ハウスA温湿度計） |
| mac_address | VARCHAR(17) | NOT NULL + 部分 UNIQUE | MACアドレス。CHECK 制約で形式検証 |
| location | VARCHAR(255) | NULLABLE | 設置場所（例: ビニールハウスA） |
| is_active | BOOLEAN | NOT NULL, DEFAULT TRUE | 有効/無効フラグ |
| last_communicated_at | TIMESTAMPTZ | NULLABLE | 最終通信日時 |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |
| deleted_at | TIMESTAMPTZ | NULLABLE | 論理削除日時 |

**CHECK 制約:**
- `devices_mac_address_format` — `mac_address ~ '^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$'`

**インデックス:**
- `devices_mac_address_unique_active` (UNIQUE, partial) → `mac_address WHERE deleted_at IS NULL`
- `devices_user_id_idx` → `user_id WHERE deleted_at IS NULL`
- `devices_is_active_idx` → `is_active WHERE deleted_at IS NULL`

---

### 3. device_tokens（API トークン）

デバイス API 用 Bearer トークン（API トークン認証を自作）。
**平文トークンは DB に保存しない。SHA-256 ハッシュのみ保存。**

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| id | BIGSERIAL | PK | |
| user_id | BIGINT | NOT NULL | トークン保有者（users.id） |
| name | VARCHAR(255) | NOT NULL | トークン名 ※デバイス名と合わせる |
| token_hash | VARCHAR(64) | NOT NULL, UNIQUE | SHA-256 ハッシュ（hex 64 文字） |
| abilities | JSONB | NOT NULL, DEFAULT '[]' | 権限（例: `["sensor:write"]`） |
| last_used_at | TIMESTAMPTZ | NULLABLE | 最終使用日時 |
| expires_at | TIMESTAMPTZ | NULLABLE | 有効期限 |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |

**デバイスとトークンの紐づけ運用ルール:**

トークン作成時、`name` カラムにデバイス名を設定して運用的に紐づける。`make gen-token` CLI で発行。

```bash
make gen-token user=1 name="ハウスA温湿度計"
# → 平文トークンが標準出力に一度だけ表示される
```

---

### 4. sensor_readings（センサー計測データ）

SHT31 から取得した生データの蓄積テーブル。システムの中核データ。

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| id | BIGSERIAL | PK | |
| device_id | BIGINT | NOT NULL | 計測デバイス（devices.id） |
| temperature | NUMERIC(5, 2) | NOT NULL | 温度（℃） |
| humidity | NUMERIC(5, 2) | NOT NULL | 相対湿度（%） |
| recorded_at | TIMESTAMPTZ | NOT NULL | センサーの計測日時 |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | サーバー受信日時 |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |
| deleted_at | TIMESTAMPTZ | NULLABLE | 論理削除日時 |

**CHECK 制約:**
- `sensor_readings_temperature_range` — `temperature BETWEEN -40 AND 125`
- `sensor_readings_humidity_range` — `humidity BETWEEN 0 AND 100`

**インデックス（いずれも `deleted_at IS NULL` の部分インデックス）:**
- `sensor_readings_device_id_recorded_at_idx` → `(device_id, recorded_at DESC)` — グラフ表示クエリの高速化
- `sensor_readings_recorded_at_idx` → `recorded_at DESC` — 期間指定検索用

**設計メモ:**
- `recorded_at` はデバイス側の計測時刻、`created_at` はサーバー受信時刻
- 両者の差分から通信遅延を分析可能
- グラフ生成（サーバサイド SVG）では `recorded_at` を時間軸に使用
- データ量が増大した場合は PostgreSQL パーティショニング（RANGE / LIST）を検討

---

### 5. alert_rules（アラートルール）

異常値検知の閾値設定テーブル。

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| id | BIGSERIAL | PK | |
| device_id | BIGINT | NOT NULL | 対象デバイス（devices.id） |
| metric | VARCHAR(20) | NOT NULL | 計測指標 → domain.Metric。DB 格納値: `temperature` / `humidity` |
| operator | VARCHAR(5) | NOT NULL | 比較演算子 → domain.ComparisonOperator。DB 格納値: `>`, `<`, `>=`, `<=` |
| threshold | NUMERIC(5, 2) | NOT NULL | 閾値 |
| is_enabled | BOOLEAN | NOT NULL, DEFAULT TRUE | 有効/無効 |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |
| deleted_at | TIMESTAMPTZ | NULLABLE | 論理削除日時 |

**CHECK 制約:**
- `alert_rules_metric_valid` — `metric IN ('temperature', 'humidity')`
- `alert_rules_operator_valid` — `operator IN ('>', '<', '>=', '<=')`

**インデックス:**
- `alert_rules_device_id_is_enabled_idx` → `(device_id, is_enabled) WHERE deleted_at IS NULL`

**設定例:**

| device_id | metric | operator | threshold | 意味 |
|-----------|--------|----------|-----------|------|
| 1 | temperature | > | 35.00 | 温度が 35℃ を超えたら警告 |
| 1 | humidity | < | 30.00 | 湿度が 30% を下回ったら警告 |

---

### 6. alert_histories（アラート履歴）

発火したアラートの記録テーブル。

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| id | BIGSERIAL | PK | |
| alert_rule_id | BIGINT | NOT NULL | 発火したルール（alert_rules.id） |
| metric | VARCHAR(20) | NOT NULL | 計測指標。発火時点の値を**非正規化**して保持 |
| operator | VARCHAR(5) | NOT NULL | 比較演算子。発火時点の値を**非正規化**して保持 |
| threshold | NUMERIC(5, 2) | NOT NULL | 閾値。発火時点の値を**非正規化**して保持 |
| actual_value | NUMERIC(5, 2) | NOT NULL | 検知時の実測値 |
| is_notified | BOOLEAN | NOT NULL, DEFAULT FALSE | 通知済みフラグ |
| triggered_at | TIMESTAMPTZ | NOT NULL | アラート発火日時 |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |
| deleted_at | TIMESTAMPTZ | NULLABLE | 論理削除日時 |

**CHECK 制約:**
- `alert_histories_metric_valid` — `metric IN ('temperature', 'humidity')`
- `alert_histories_operator_valid` — `operator IN ('>', '<', '>=', '<=')`

**インデックス（いずれも `deleted_at IS NULL` の部分インデックス）:**
- `alert_histories_alert_rule_id_idx` → `alert_rule_id`
- `alert_histories_triggered_at_idx` → `triggered_at DESC`
- `alert_histories_unnotified_idx` → `triggered_at DESC WHERE is_notified = FALSE AND deleted_at IS NULL` — ダッシュボード未通知バナー用

**設計メモ:**
- `metric`, `operator`, `threshold` は alert_rules から**非正規化**して保持。ルールが変更・削除されても履歴の意味が失われない

---

## sqlc リレーション方針

> **cc-sdd への価値:**
> Handler / Service 実装で他テーブルデータを取得する際の sqlc 関数名が必要。ER 図には構造は示されているが、実装では JOIN クエリをあらかじめ名前で用意しておく。これにより N+1 問題を構造的に防ぐ。

| 取得パターン | sqlc 関数名 | 主なクエリ |
|------------|------------|----------|
| デバイスの最新センサー 1 件 | `GetLatestSensorReading` | `SELECT * FROM sensor_readings WHERE device_id = $1 ORDER BY recorded_at DESC LIMIT 1` |
| ユーザーの全デバイス | `ListDevicesByUser` | `SELECT * FROM devices WHERE user_id = $1 AND deleted_at IS NULL` |
| デバイスの有効なアラートルール | `ListEnabledAlertRulesByDevice` | アラート判定の中核 |
| 未通知アラート + デバイス名 | `ListUnnotifiedAlertHistoriesWithDevice` | alert_histories × alert_rules × devices の JOIN |
| アラート履歴 + デバイス名（ページング） | `ListAlertHistoriesPaginated` | 同上 + フィルタ + LIMIT/OFFSET |

> ORM の Eager Loading に頼らず、**必要な JOIN を最初から sqlc クエリで書く** のが Go 流のアプローチ（N+1 を構造的に防ぐ）。

---

## 主要クエリパターン

### グラフ表示用（24h 向け生データ）

```sql
-- name: ListRecentSensorReadings :many
SELECT * FROM sensor_readings
 WHERE device_id   = $1
   AND recorded_at >= $2
   AND deleted_at IS NULL
 ORDER BY recorded_at ASC;
```

### デバイス詳細グラフ用（日次集計 — 7日/30日表示）

```sql
-- name: ListDailySensorAggregates :many
SELECT
    DATE(recorded_at)               AS reading_date,
    AVG(temperature)::NUMERIC(5, 2) AS avg_temperature,
    MAX(temperature)                AS max_temperature,
    MIN(temperature)                AS min_temperature,
    AVG(humidity)::NUMERIC(5, 2)    AS avg_humidity,
    MAX(humidity)                   AS max_humidity,
    MIN(humidity)                   AS min_humidity,
    COUNT(*)::BIGINT                AS sample_count
  FROM sensor_readings
 WHERE device_id   = $1
   AND recorded_at >= $2
   AND deleted_at IS NULL
 GROUP BY DATE(recorded_at)
 ORDER BY DATE(recorded_at) ASC;
```

### センサーデータ履歴用（期間集計）

```sql
-- name: GetSensorReadingsSummary :one
SELECT
    AVG(temperature)::NUMERIC(5, 2) AS avg_temperature,
    MAX(temperature)                AS max_temperature,
    MIN(temperature)                AS min_temperature,
    AVG(humidity)::NUMERIC(5, 2)    AS avg_humidity,
    MAX(humidity)                   AS max_humidity,
    MIN(humidity)                   AS min_humidity,
    COUNT(*)::BIGINT                AS sample_count
  FROM sensor_readings
 WHERE device_id   = $1
   AND recorded_at BETWEEN $2 AND $3
   AND deleted_at IS NULL;
```

### アラート判定用

```sql
-- name: ListEnabledAlertRulesByDevice :many
SELECT * FROM alert_rules
 WHERE device_id  = $1
   AND is_enabled = TRUE
   AND deleted_at IS NULL;
```

**Go でのアラート判定ロジック例:**

```go
// internal/service/alert_evaluator.go
func (s *AlertEvaluator) EvaluateAndNotify(ctx context.Context, reading *repository.SensorReading) ([]repository.AlertHistory, error) {
    rules, err := s.Repo.ListEnabledAlertRulesByDevice(ctx, reading.DeviceID)
    if err != nil {
        return nil, err
    }

    var fired []repository.AlertHistory
    temp := pgconv.NumericToFloat(reading.Temperature)
    hum  := pgconv.NumericToFloat(reading.Humidity)

    for _, rule := range rules {
        metric := domain.Metric(rule.Metric)
        op     := domain.ComparisonOperator(rule.Operator)
        thr    := pgconv.NumericToFloat(rule.Threshold)

        actual := temp
        if metric == domain.MetricHumidity {
            actual = hum
        }

        if op.Evaluate(actual, thr) {
            hist, err := s.Repo.CreateAlertHistory(ctx, repository.CreateAlertHistoryParams{
                AlertRuleID:  rule.ID,
                Metric:       rule.Metric,
                Operator:     rule.Operator,
                Threshold:    rule.Threshold,
                ActualValue:  pgconv.Numeric2(actual),
                TriggeredAt:  pgconv.Timestamptz(time.Now()),
            })
            if err != nil {
                return nil, err
            }
            fired = append(fired, hist)
        }
    }
    return fired, nil
}
```

---

## データフロー対応表

| フロー | 関連テーブル |
|--------|-------------|
| ESP32 → API → DB保存 | sensor_readings, devices (last_communicated_at 更新) |
| DB → Handler → サーバサイド SVG → templ | sensor_readings (SELECT) |
| templ (hx-post) → Handler → DB | alert_rules (CRUD 操作) |
| 異常値検知 → Notifier サービス | alert_rules, alert_histories, sensor_readings |

---

## アラート判定の実行タイミング

> **cc-sdd への価値:**
> データフロー対応表には「異常値検知 → Notifier サービス」とあるが、「いつ・誰が」判定を実行するかが未定義。Queue 使用の有無は Handler の責務範囲・テスト構造に直接影響する。

**採用方針:** センサーデータ受信 Handler 内で**同期的**に実行する。非同期キュー（Redis/AMQP 等）は使用しない。

```
ESP32 → POST /api/sensor-data → Handler（同期処理）
    ├── 1. sensor_readings に INSERT (CreateSensorReading)
    ├── 2. devices.last_communicated_at を UPDATE (UpdateDeviceLastCommunicated)
    └── 3. 有効な alert_rules を取得 (ListEnabledAlertRulesByDevice)
           → domain.ComparisonOperator.Evaluate() で判定
           └── 条件一致 → alert_histories に INSERT (CreateAlertHistory)
                       → Notifier.Send() で通知
```

**設計根拠:**
- センサー送信間隔が十分に長い（数分〜数十分）ため、同期処理でもリクエスト遅延は許容範囲
- Queue インフラ（Redis 等）を追加しないことでシステム構成をシンプルに保つ
- Go の goroutine で軽量並列化は可能だが、初期リリースでは直列実行で十分

---

更新日時: 2026-06-02
