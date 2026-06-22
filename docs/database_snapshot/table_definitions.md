# DBスナップショット — テーブル定義

> このファイルは `make db-snapshot` で自動生成される。手動編集しない（スキーマ変更後に再生成すること）。
> 実DBへ接続しなくても、本ファイルを読むだけでテーブル・カラム・制約・リレーションを把握できることを目的とする。
> ※ 外部キー制約は張らない方針（参照整合性はアプリ層で担保）。Mermaid の関連は `<table>_id` 命名から推論した論理リレーション。

**テーブル数:** 7

## 目次

- [alert_histories](#alert_histories) — 発火したアラートの履歴
- [alert_rules](#alert_rules) — 異常値検知の閾値設定ルール
- [device_tokens](#device_tokens) — デバイスAPI用 Bearer トークン (Sanctum相当)
- [devices](#devices) — ESP8266デバイス管理
- [sensor_readings](#sensor_readings) — SHT31 からの温湿度計測データ (システムの中核データ)
- [sessions](#sessions) — Web UI の Session 認証データ (scs/pgxstore が管理。sqlc 対象外)
- [users](#users) — ユーザー (Web UI の Session 認証対象)

---

## alert_histories

発火したアラートの履歴

| カラム | 型 | NULL | デフォルト | 説明 |
|--------|----|------|-----------|------|
| id | bigint | NO | nextval('alert_histories_id_seq'::regclass) | PK |
| alert_rule_id | bigint | NO | - | - |
| metric | character varying(20) | NO | - | ルール発火時点の指標 (alert_rules の metric を非正規化保持) |
| operator | character varying(5) | NO | - | ルール発火時点の演算子 (非正規化保持) |
| threshold | numeric(5,2) | NO | - | ルール発火時点の閾値 (非正規化保持) |
| actual_value | numeric(5,2) | NO | - | 発火時の実測値 |
| is_notified | boolean | NO | false | 通知送信完了フラグ |
| triggered_at | timestamp with time zone | NO | - | - |
| created_at | timestamp with time zone | NO | now() | - |
| updated_at | timestamp with time zone | NO | now() | - |
| deleted_at | timestamp with time zone | YES | - | - |

**索引**

- `alert_histories_alert_rule_id_idx`: `CREATE INDEX alert_histories_alert_rule_id_idx ON public.alert_histories USING btree (alert_rule_id) WHERE (deleted_at IS NULL)`
- `alert_histories_triggered_at_idx`: `CREATE INDEX alert_histories_triggered_at_idx ON public.alert_histories USING btree (triggered_at DESC) WHERE (deleted_at IS NULL)`
- `alert_histories_unnotified_idx`: `CREATE INDEX alert_histories_unnotified_idx ON public.alert_histories USING btree (triggered_at DESC) WHERE ((is_notified = false) AND (deleted_at IS NULL))`

**CHECK 制約**

- `alert_histories_metric_valid`: `CHECK (((metric)::text = ANY ((ARRAY['temperature'::character varying, 'humidity'::character varying])::text[])))`
- `alert_histories_operator_valid`: `CHECK (((operator)::text = ANY ((ARRAY['>'::character varying, '<'::character varying, '>='::character varying, '<='::character varying])::text[])))`

---

## alert_rules

異常値検知の閾値設定ルール

| カラム | 型 | NULL | デフォルト | 説明 |
|--------|----|------|-----------|------|
| id | bigint | NO | nextval('alert_rules_id_seq'::regclass) | PK |
| device_id | bigint | NO | - | - |
| metric | character varying(20) | NO | - | 計測指標 (temperature \| humidity) — domain.Metric と対応 |
| operator | character varying(5) | NO | - | 比較演算子 (> \| < \| >= \| <=) — domain.ComparisonOperator と対応 |
| threshold | numeric(5,2) | NO | - | 閾値 |
| is_enabled | boolean | NO | true | - |
| created_at | timestamp with time zone | NO | now() | - |
| updated_at | timestamp with time zone | NO | now() | - |
| deleted_at | timestamp with time zone | YES | - | - |

**索引**

- `alert_rules_device_id_is_enabled_idx`: `CREATE INDEX alert_rules_device_id_is_enabled_idx ON public.alert_rules USING btree (device_id, is_enabled) WHERE (deleted_at IS NULL)`

**CHECK 制約**

- `alert_rules_metric_valid`: `CHECK (((metric)::text = ANY ((ARRAY['temperature'::character varying, 'humidity'::character varying])::text[])))`
- `alert_rules_operator_valid`: `CHECK (((operator)::text = ANY ((ARRAY['>'::character varying, '<'::character varying, '>='::character varying, '<='::character varying])::text[])))`

---

## device_tokens

デバイスAPI用 Bearer トークン (Sanctum相当)

| カラム | 型 | NULL | デフォルト | 説明 |
|--------|----|------|-----------|------|
| id | bigint | NO | nextval('device_tokens_id_seq'::regclass) | PK |
| user_id | bigint | NO | - | - |
| name | character varying(255) | NO | - | トークン名 (デバイス名と合わせる運用) |
| token_hash | character varying(64) | NO | - | SHA-256 ハッシュ化済トークン (平文は保存しない) |
| abilities | jsonb | NO | '[]'::jsonb | 権限一覧 (例: ["sensor:write"]) |
| last_used_at | timestamp with time zone | YES | - | - |
| expires_at | timestamp with time zone | YES | - | - |
| created_at | timestamp with time zone | NO | now() | - |
| updated_at | timestamp with time zone | NO | now() | - |

**索引**

- `device_tokens_token_hash_unique`: `CREATE UNIQUE INDEX device_tokens_token_hash_unique ON public.device_tokens USING btree (token_hash)`
- `device_tokens_user_id_idx`: `CREATE INDEX device_tokens_user_id_idx ON public.device_tokens USING btree (user_id)`

---

## devices

ESP8266デバイス管理

| カラム | 型 | NULL | デフォルト | 説明 |
|--------|----|------|-----------|------|
| id | bigint | NO | nextval('devices_id_seq'::regclass) | PK |
| user_id | bigint | NO | - | - |
| name | character varying(255) | NO | - | - |
| mac_address | character varying(17) | NO | - | MACアドレス (例: AA:BB:CC:DD:EE:FF) |
| location | character varying(255) | YES | - | - |
| is_active | boolean | NO | true | - |
| last_communicated_at | timestamp with time zone | YES | - | - |
| created_at | timestamp with time zone | NO | now() | - |
| updated_at | timestamp with time zone | NO | now() | - |
| deleted_at | timestamp with time zone | YES | - | 論理削除日時 (NULL = 有効) |

**索引**

- `devices_is_active_idx`: `CREATE INDEX devices_is_active_idx ON public.devices USING btree (is_active) WHERE (deleted_at IS NULL)`
- `devices_mac_address_unique_active`: `CREATE UNIQUE INDEX devices_mac_address_unique_active ON public.devices USING btree (mac_address) WHERE (deleted_at IS NULL)`
- `devices_user_id_idx`: `CREATE INDEX devices_user_id_idx ON public.devices USING btree (user_id) WHERE (deleted_at IS NULL)`

**CHECK 制約**

- `devices_mac_address_format`: `CHECK (((mac_address)::text ~ '^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$'::text))`

---

## sensor_readings

SHT31 からの温湿度計測データ (システムの中核データ)

| カラム | 型 | NULL | デフォルト | 説明 |
|--------|----|------|-----------|------|
| id | bigint | NO | nextval('sensor_readings_id_seq'::regclass) | PK |
| device_id | bigint | NO | - | - |
| temperature | numeric(5,2) | NO | - | 温度 (℃) -40.00 〜 125.00 |
| humidity | numeric(5,2) | NO | - | 湿度 (%) 0.00 〜 100.00 |
| recorded_at | timestamp with time zone | NO | - | デバイス側での計測日時 |
| created_at | timestamp with time zone | NO | now() | サーバ受信日時 (通信遅延の計算に使用) |
| updated_at | timestamp with time zone | NO | now() | - |
| deleted_at | timestamp with time zone | YES | - | - |

**索引**

- `sensor_readings_device_id_recorded_at_idx`: `CREATE INDEX sensor_readings_device_id_recorded_at_idx ON public.sensor_readings USING btree (device_id, recorded_at DESC) WHERE (deleted_at IS NULL)`
- `sensor_readings_recorded_at_idx`: `CREATE INDEX sensor_readings_recorded_at_idx ON public.sensor_readings USING btree (recorded_at DESC) WHERE (deleted_at IS NULL)`

**CHECK 制約**

- `sensor_readings_humidity_range`: `CHECK (((humidity >= (0)::numeric) AND (humidity <= (100)::numeric)))`
- `sensor_readings_temperature_range`: `CHECK (((temperature >= ('-40'::integer)::numeric) AND (temperature <= (125)::numeric)))`

---

## sessions

Web UI の Session 認証データ (scs/pgxstore が管理。sqlc 対象外)

| カラム | 型 | NULL | デフォルト | 説明 |
|--------|----|------|-----------|------|
| token | text | NO | - | PK |
| data | bytea | NO | - | - |
| expiry | timestamp with time zone | NO | - | - |

**索引**

- `sessions_expiry_idx`: `CREATE INDEX sessions_expiry_idx ON public.sessions USING btree (expiry)`

---

## users

ユーザー (Web UI の Session 認証対象)

| カラム | 型 | NULL | デフォルト | 説明 |
|--------|----|------|-----------|------|
| id | bigint | NO | nextval('users_id_seq'::regclass) | PK |
| name | character varying(255) | NO | - | - |
| email | character varying(255) | NO | - | - |
| password_hash | character varying(255) | NO | - | bcrypt または argon2 等でハッシュ化されたパスワード |
| email_verified_at | timestamp with time zone | YES | - | - |
| created_at | timestamp with time zone | NO | now() | - |
| updated_at | timestamp with time zone | NO | now() | - |

**索引**

- `users_email_unique`: `CREATE UNIQUE INDEX users_email_unique ON public.users USING btree (email)`

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

