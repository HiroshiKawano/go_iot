-- name: CreateAlertHistory :one
-- アラート発火時に metric / operator / threshold を alert_rules から非正規化して保存
INSERT INTO alert_histories (
    alert_rule_id, metric, operator, threshold, actual_value, triggered_at
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListUnnotifiedAlertHistoriesWithDevice :many
-- ダッシュボードのアラート通知バナー表示用
-- alert_histories → alert_rules → devices の JOIN で devices.name も取得
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
  JOIN alert_rules     AS ar ON ar.id        = ah.alert_rule_id
  JOIN devices         AS d  ON d.id         = ar.device_id
 WHERE ah.is_notified = FALSE
   AND ah.deleted_at  IS NULL
   AND ar.deleted_at  IS NULL
   AND d.deleted_at   IS NULL
   AND d.user_id      = $1
 ORDER BY ah.triggered_at DESC
 LIMIT $2;

-- name: ListAlertHistoriesPaginated :many
-- アラート履歴画面の一覧用 (デバイスフィルタ + 期間フィルタ + ページング)
-- device_id が NULL の場合は全デバイス対象
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
 WHERE d.user_id      = $1
   AND (sqlc.narg('device_id')::BIGINT IS NULL OR d.id = sqlc.narg('device_id'))
   AND ah.triggered_at BETWEEN sqlc.arg('from_at') AND sqlc.arg('to_at')
   AND ah.deleted_at  IS NULL
   AND ar.deleted_at  IS NULL
   AND d.deleted_at   IS NULL
 ORDER BY ah.triggered_at DESC
 LIMIT  sqlc.arg('limit_n')
OFFSET sqlc.arg('offset_n');

-- name: CountAlertHistoriesInRange :one
SELECT COUNT(*)::BIGINT AS total
  FROM alert_histories AS ah
  JOIN alert_rules     AS ar ON ar.id = ah.alert_rule_id
  JOIN devices         AS d  ON d.id  = ar.device_id
 WHERE d.user_id      = $1
   AND (sqlc.narg('device_id')::BIGINT IS NULL OR d.id = sqlc.narg('device_id'))
   AND ah.triggered_at BETWEEN sqlc.arg('from_at') AND sqlc.arg('to_at')
   AND ah.deleted_at  IS NULL
   AND ar.deleted_at  IS NULL
   AND d.deleted_at   IS NULL;

-- name: MarkAlertHistoryNotified :exec
UPDATE alert_histories
   SET is_notified = TRUE,
       updated_at  = NOW()
 WHERE id = $1;
