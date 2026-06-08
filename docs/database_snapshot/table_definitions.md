# DBスナップショット — テーブル定義

> このファイルは `make db-snapshot` で自動生成される。手動編集しない（スキーマ変更後に再生成すること）。
> 実DBへ接続しなくても、本ファイルを読むだけでテーブル・カラム・制約・リレーションを把握できることを目的とする。
> ※ 外部キー制約は張らない方針（参照整合性はアプリ層で担保）。Mermaid の関連は `<table>_id` 命名から推論した論理リレーション。

**テーブル数:** 7

## 目次

- [alert_histories](#alert_histories)
- [alert_rules](#alert_rules)
- [device_tokens](#device_tokens)
- [devices](#devices)
- [sensor_readings](#sensor_readings)
- [sessions](#sessions)
- [users](#users)

---

## alert_histories

| カラム | 型 | NULL | デフォルト | 説明 |
|--------|----|------|-----------|------|
| id | INTEGER | NO | - | PK |
| alert_rule_id | INTEGER | NO | - | - |
| metric | VARCHAR(20) | NO | - | - |
| operator | VARCHAR(5) | NO | - | - |
| threshold | REAL | NO | - | - |
| actual_value | REAL | NO | - | - |
| is_notified | BOOLEAN | NO | FALSE | - |
| triggered_at | DATETIME | NO | - | - |
| created_at | DATETIME | NO | CURRENT_TIMESTAMP | - |
| updated_at | DATETIME | NO | CURRENT_TIMESTAMP | - |
| deleted_at | DATETIME | YES | - | - |

**索引**

- `alert_histories_alert_rule_id_idx`: `CREATE INDEX alert_histories_alert_rule_id_idx
    ON alert_histories(alert_rule_id) WHERE deleted_at IS NULL`
- `alert_histories_triggered_at_idx`: `CREATE INDEX alert_histories_triggered_at_idx
    ON alert_histories(triggered_at DESC) WHERE deleted_at IS NULL`
- `alert_histories_unnotified_idx`: `CREATE INDEX alert_histories_unnotified_idx
    ON alert_histories(triggered_at DESC)
    WHERE is_notified = FALSE AND deleted_at IS NULL`

**CHECK 制約**

- `alert_histories_metric_valid`: `CHECK (metric   IN ('temperature', 'humidity'))`
- `alert_histories_operator_valid`: `CHECK (operator IN ('>', '<', '>=', '<='))`

---

## alert_rules

| カラム | 型 | NULL | デフォルト | 説明 |
|--------|----|------|-----------|------|
| id | INTEGER | NO | - | PK |
| device_id | INTEGER | NO | - | - |
| metric | VARCHAR(20) | NO | - | - |
| operator | VARCHAR(5) | NO | - | - |
| threshold | REAL | NO | - | - |
| is_enabled | BOOLEAN | NO | TRUE | - |
| created_at | DATETIME | NO | CURRENT_TIMESTAMP | - |
| updated_at | DATETIME | NO | CURRENT_TIMESTAMP | - |
| deleted_at | DATETIME | YES | - | - |

**索引**

- `alert_rules_device_id_is_enabled_idx`: `CREATE INDEX alert_rules_device_id_is_enabled_idx
    ON alert_rules(device_id, is_enabled) WHERE deleted_at IS NULL`

**CHECK 制約**

- `alert_rules_metric_valid`: `CHECK (metric   IN ('temperature', 'humidity'))`
- `alert_rules_operator_valid`: `CHECK (operator IN ('>', '<', '>=', '<='))`

---

## device_tokens

| カラム | 型 | NULL | デフォルト | 説明 |
|--------|----|------|-----------|------|
| id | INTEGER | NO | - | PK |
| user_id | INTEGER | NO | - | - |
| name | VARCHAR(255) | NO | - | - |
| token_hash | VARCHAR(64) | NO | - | - |
| abilities | json | NO | '[]' | - |
| last_used_at | DATETIME | YES | - | - |
| expires_at | DATETIME | YES | - | - |
| created_at | DATETIME | NO | CURRENT_TIMESTAMP | - |
| updated_at | DATETIME | NO | CURRENT_TIMESTAMP | - |

**索引**

- `device_tokens_token_hash_unique`: `CREATE UNIQUE INDEX device_tokens_token_hash_unique ON device_tokens(token_hash)`
- `device_tokens_user_id_idx`: `CREATE INDEX device_tokens_user_id_idx ON device_tokens(user_id)`

---

## devices

| カラム | 型 | NULL | デフォルト | 説明 |
|--------|----|------|-----------|------|
| id | INTEGER | NO | - | PK |
| user_id | INTEGER | NO | - | - |
| name | VARCHAR(255) | NO | - | - |
| mac_address | VARCHAR(17) | NO | - | - |
| location | VARCHAR(255) | YES | - | - |
| is_active | BOOLEAN | NO | TRUE | - |
| last_communicated_at | DATETIME | YES | - | - |
| created_at | DATETIME | NO | CURRENT_TIMESTAMP | - |
| updated_at | DATETIME | NO | CURRENT_TIMESTAMP | - |
| deleted_at | DATETIME | YES | - | - |

**索引**

- `devices_is_active_idx`: `CREATE INDEX devices_is_active_idx
    ON devices(is_active) WHERE deleted_at IS NULL`
- `devices_mac_address_unique_active`: `CREATE UNIQUE INDEX devices_mac_address_unique_active
    ON devices(mac_address) WHERE deleted_at IS NULL`
- `devices_user_id_idx`: `CREATE INDEX devices_user_id_idx
    ON devices(user_id) WHERE deleted_at IS NULL`

---

## sensor_readings

| カラム | 型 | NULL | デフォルト | 説明 |
|--------|----|------|-----------|------|
| id | INTEGER | NO | - | PK |
| device_id | INTEGER | NO | - | - |
| temperature | REAL | NO | - | - |
| humidity | REAL | NO | - | - |
| recorded_at | DATETIME | NO | - | - |
| created_at | DATETIME | NO | CURRENT_TIMESTAMP | - |
| updated_at | DATETIME | NO | CURRENT_TIMESTAMP | - |
| deleted_at | DATETIME | YES | - | - |

**索引**

- `sensor_readings_device_id_recorded_at_idx`: `CREATE INDEX sensor_readings_device_id_recorded_at_idx
    ON sensor_readings(device_id, recorded_at DESC) WHERE deleted_at IS NULL`
- `sensor_readings_recorded_at_idx`: `CREATE INDEX sensor_readings_recorded_at_idx
    ON sensor_readings(recorded_at DESC) WHERE deleted_at IS NULL`

**CHECK 制約**

- `sensor_readings_temperature_range`: `CHECK (temperature BETWEEN -40 AND 125)`
- `sensor_readings_humidity_range`: `CHECK (humidity    BETWEEN 0   AND 100)`

---

## sessions

| カラム | 型 | NULL | デフォルト | 説明 |
|--------|----|------|-----------|------|
| token | TEXT | NO | - | PK |
| data | BLOB | NO | - | - |
| expiry | REAL | NO | - | - |

**索引**

- `sessions_expiry_idx`: `CREATE INDEX sessions_expiry_idx ON sessions (expiry)`

---

## users

| カラム | 型 | NULL | デフォルト | 説明 |
|--------|----|------|-----------|------|
| id | INTEGER | NO | - | PK |
| name | VARCHAR(255) | NO | - | - |
| email | VARCHAR(255) | NO | - | - |
| password_hash | VARCHAR(255) | NO | - | - |
| email_verified_at | DATETIME | YES | - | - |
| created_at | DATETIME | NO | CURRENT_TIMESTAMP | - |
| updated_at | DATETIME | NO | CURRENT_TIMESTAMP | - |

**索引**

- `users_email_unique`: `CREATE UNIQUE INDEX users_email_unique ON users(email)`

---

## 論理リレーション

外部キー制約は張らないため、以下は `<table>_id` カラム名から推論した参照関係。

| 子テーブル | カラム | → | 親テーブル |
|------------|--------|---|------------|
| alert_histories | alert_rule_id | → | alert_rules |
| alert_rules | device_id | → | devices |
| device_tokens | user_id | → | users |
| devices | user_id | → | users |
| sensor_readings | device_id | → | devices |

