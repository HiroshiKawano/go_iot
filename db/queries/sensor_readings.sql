-- name: CreateSensorReading :one
INSERT INTO sensor_readings (device_id, temperature, humidity, recorded_at)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetLatestSensorReading :one
SELECT * FROM sensor_readings
 WHERE device_id = ? AND deleted_at IS NULL
 ORDER BY recorded_at DESC
 LIMIT 1;

-- name: ListLatestSensorReadings :many
SELECT * FROM sensor_readings
 WHERE device_id = ? AND deleted_at IS NULL
 ORDER BY recorded_at DESC
 LIMIT 10;

-- name: ListRecentSensorReadings :many
SELECT * FROM sensor_readings
 WHERE device_id   = ?
   AND recorded_at >= ?
   AND deleted_at IS NULL
 ORDER BY recorded_at ASC;

-- name: ListDailySensorAggregates :many
SELECT
    CAST(date(substr(recorded_at, 1, 19), '+9 hours') AS TEXT) AS reading_date,
    CAST(AVG(temperature) AS REAL)              AS avg_temperature,
    CAST(MAX(temperature) AS REAL)              AS max_temperature,
    CAST(MIN(temperature) AS REAL)              AS min_temperature,
    CAST(AVG(humidity)    AS REAL)              AS avg_humidity,
    CAST(MAX(humidity)    AS REAL)              AS max_humidity,
    CAST(MIN(humidity)    AS REAL)              AS min_humidity,
    COUNT(*)                                    AS sample_count
  FROM sensor_readings
 WHERE device_id   = ?
   AND recorded_at >= ?
   AND deleted_at IS NULL
 GROUP BY CAST(date(substr(recorded_at, 1, 19), '+9 hours') AS TEXT)
 ORDER BY CAST(date(substr(recorded_at, 1, 19), '+9 hours') AS TEXT) ASC;

-- name: GetSensorReadingsSummary :one
SELECT
    CAST(AVG(temperature) AS REAL) AS avg_temperature,
    CAST(MAX(temperature) AS REAL) AS max_temperature,
    CAST(MIN(temperature) AS REAL) AS min_temperature,
    CAST(AVG(humidity)    AS REAL) AS avg_humidity,
    CAST(MAX(humidity)    AS REAL) AS max_humidity,
    CAST(MIN(humidity)    AS REAL) AS min_humidity,
    COUNT(*)                       AS sample_count
  FROM sensor_readings
 WHERE device_id   = ?
   AND recorded_at >= ?
   AND recorded_at <= ?
   AND deleted_at IS NULL
 GROUP BY device_id;

-- name: ListSensorReadingsPaginated :many
SELECT * FROM sensor_readings
 WHERE device_id   = ?
   AND recorded_at >= ?
   AND recorded_at <= ?
   AND deleted_at IS NULL
 ORDER BY recorded_at DESC
 LIMIT ? OFFSET ?;

-- name: CountSensorReadingsInRange :one
SELECT COUNT(*) AS total
  FROM sensor_readings
 WHERE device_id   = ?
   AND recorded_at >= ?
   AND recorded_at <= ?
   AND deleted_at IS NULL;
