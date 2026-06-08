-- name: CreateAlertHistory :one
INSERT INTO alert_histories (
    alert_rule_id, metric, operator, threshold, actual_value, triggered_at
) VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListUnnotifiedAlertHistoriesWithDevice :many
SELECT
    ah.id,
    ah.alert_rule_id,
    ah.metric,
    ah.operator,
    ah.threshold,
    ah.actual_value,
    ah.triggered_at,
    d.id   AS device_id,
    d.name AS device_name
  FROM alert_histories AS ah
  JOIN alert_rules     AS ar ON ar.id = ah.alert_rule_id
  JOIN devices         AS d  ON d.id  = ar.device_id
 WHERE ah.is_notified = FALSE
   AND ah.deleted_at  IS NULL
   AND ar.deleted_at  IS NULL
   AND d.deleted_at   IS NULL
   AND d.user_id      = ?
 ORDER BY ah.triggered_at DESC
 LIMIT ?;

-- name: ListAlertHistoriesPaginated :many
SELECT
    ah.id,
    ah.alert_rule_id,
    ah.metric,
    ah.operator,
    ah.threshold,
    ah.actual_value,
    ah.is_notified,
    ah.triggered_at,
    d.id   AS device_id,
    d.name AS device_name
  FROM alert_histories AS ah
  JOIN alert_rules     AS ar ON ar.id = ah.alert_rule_id
  JOIN devices         AS d  ON d.id  = ar.device_id
 WHERE d.user_id      = sqlc.arg('user_id')
   AND (CAST(sqlc.narg('device_id') AS INTEGER) IS NULL OR d.id = sqlc.narg('device_id'))
   AND ah.triggered_at >= sqlc.arg('from_at')
   AND ah.triggered_at <= sqlc.arg('to_at')
   AND ah.deleted_at  IS NULL
   AND ar.deleted_at  IS NULL
   AND d.deleted_at   IS NULL
 ORDER BY ah.triggered_at DESC
 LIMIT  sqlc.arg('limit_n')
OFFSET sqlc.arg('offset_n');

-- name: CountAlertHistoriesInRange :one
SELECT COUNT(*) AS total
  FROM alert_histories AS ah
  JOIN alert_rules     AS ar ON ar.id = ah.alert_rule_id
  JOIN devices         AS d  ON d.id  = ar.device_id
 WHERE d.user_id      = sqlc.arg('user_id')
   AND (CAST(sqlc.narg('device_id') AS INTEGER) IS NULL OR d.id = sqlc.narg('device_id'))
   AND ah.triggered_at >= sqlc.arg('from_at')
   AND ah.triggered_at <= sqlc.arg('to_at')
   AND ah.deleted_at  IS NULL
   AND ar.deleted_at  IS NULL
   AND d.deleted_at   IS NULL;

-- name: MarkAlertHistoryNotified :exec
UPDATE alert_histories
   SET is_notified = TRUE,
       updated_at  = datetime('now')
 WHERE id = ?;
