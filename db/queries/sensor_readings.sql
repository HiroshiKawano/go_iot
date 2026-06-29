-- name: CreateSensorReading :one
INSERT INTO sensor_readings (device_id, temperature, humidity, recorded_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetLatestSensorReading :one
-- ダッシュボードでデバイスごとの最新値表示に使用
SELECT * FROM sensor_readings
 WHERE device_id = $1 AND deleted_at IS NULL
 ORDER BY recorded_at DESC
 LIMIT 1;

-- name: ListLatestSensorReadings :many
-- デバイス詳細の最新計測テーブル用: 最新10件を降順で取得 (期間に非連動・固定10件)
-- 既存 ListRecentSensorReadings (時刻以降・昇順=24hグラフ用) とは役割が異なるため Latest で命名分離
SELECT * FROM sensor_readings
 WHERE device_id = $1 AND deleted_at IS NULL
 ORDER BY recorded_at DESC
 LIMIT 10;

-- name: ListRecentSensorReadings :many
-- 24時間グラフ用: 指定時刻以降の生データを昇順で取得
SELECT * FROM sensor_readings
 WHERE device_id   = $1
   AND recorded_at >= $2
   AND deleted_at IS NULL
 ORDER BY recorded_at ASC;

-- name: ListDailySensorAggregates :many
-- 3日/7日/30日グラフ用: 日別の平均/最大/最小を集計 (24h以外の複数日はこの集計経路)
SELECT
    DATE(recorded_at)                       AS reading_date,
    AVG(temperature)::NUMERIC(5, 2)         AS avg_temperature,
    MAX(temperature)                        AS max_temperature,
    MIN(temperature)                        AS min_temperature,
    AVG(humidity)::NUMERIC(5, 2)            AS avg_humidity,
    MAX(humidity)                           AS max_humidity,
    MIN(humidity)                           AS min_humidity,
    COUNT(*)::BIGINT                        AS sample_count
  FROM sensor_readings
 WHERE device_id   = $1
   AND recorded_at >= $2
   AND deleted_at IS NULL
 GROUP BY DATE(recorded_at)
 ORDER BY DATE(recorded_at) ASC;

-- name: ListDailySensorAggregatesJST :many
-- 統計分析ページ(長期トレンド/季節サマリ)用: JST 暦日でバケットした日次集計。
-- 月次/年次ロールアップは handler 境界で本クエリの日次行を二段集約する(月次ΔT=日次ΔTの平均)。
-- 既存 ListDailySensorAggregates(DATE() の TZ 非明示=UTC バケット)は 3d/7d/30d グラフ用に無改変で温存し、
-- 長期トレンド専用に JST 暦境界版を別名で追加する(SELECT のみ・DDL なし)。
-- 事前条件: device_id は呼び出し前に RequireDeviceOwner で所有検証済み。$2=取得下限(期間×バッファ)。
-- 事後条件: JST 暦日昇順・欠測日は行なし(handler が欠測扱いし 0 補完しない)。
SELECT
    DATE(recorded_at AT TIME ZONE 'Asia/Tokyo') AS reading_date,
    AVG(temperature)::NUMERIC(5, 2)             AS avg_temperature,
    MAX(temperature)                            AS max_temperature,
    MIN(temperature)                            AS min_temperature,
    AVG(humidity)::NUMERIC(5, 2)                AS avg_humidity,
    MAX(humidity)                               AS max_humidity,
    MIN(humidity)                               AS min_humidity,
    COUNT(*)::BIGINT                            AS sample_count
  FROM sensor_readings
 WHERE device_id   = $1
   AND recorded_at >= $2
   AND deleted_at IS NULL
 GROUP BY DATE(recorded_at AT TIME ZONE 'Asia/Tokyo')
 ORDER BY DATE(recorded_at AT TIME ZONE 'Asia/Tokyo') ASC;

-- name: GetSensorReadingsSummary :one
-- センサーデータ履歴画面の集計ボックス用
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

-- name: ListSensorReadingsPaginated :many
-- センサーデータ履歴画面のテーブル用 (期間指定 + ページング)
SELECT * FROM sensor_readings
 WHERE device_id   = $1
   AND recorded_at BETWEEN $2 AND $3
   AND deleted_at IS NULL
 ORDER BY recorded_at DESC
 LIMIT $4 OFFSET $5;

-- name: CountSensorReadingsInRange :one
SELECT COUNT(*)::BIGINT AS total
  FROM sensor_readings
 WHERE device_id   = $1
   AND recorded_at BETWEEN $2 AND $3
   AND deleted_at IS NULL;

-- name: ListSensorReadingsInRange :many
-- CSV エクスポート / 集計帳票用: 期間内の全行を昇順で取得 (ページングなし)。
-- 既存 Paginated は DESC+LIMIT、本クエリは ASC+LIMIT なし (帳票バケット/CSV 昇順出力に使う)。
SELECT * FROM sensor_readings
 WHERE device_id   = $1
   AND recorded_at BETWEEN $2 AND $3
   AND deleted_at IS NULL
 ORDER BY recorded_at ASC;
