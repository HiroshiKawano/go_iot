-- name: GetAlertRule :one
SELECT * FROM alert_rules
 WHERE id = $1 AND deleted_at IS NULL;

-- name: ListAlertRulesByDevice :many
SELECT * FROM alert_rules
 WHERE device_id = $1 AND deleted_at IS NULL
 ORDER BY created_at ASC;

-- name: ListEnabledAlertRulesByDevice :many
-- アラート判定ロジック (センサー受信時の同期処理) で使用
SELECT * FROM alert_rules
 WHERE device_id  = $1
   AND is_enabled = TRUE
   AND deleted_at IS NULL;

-- name: CreateAlertRule :one
INSERT INTO alert_rules (device_id, metric, operator, threshold, is_enabled)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateAlertRule :one
UPDATE alert_rules
   SET metric     = $2,
       operator   = $3,
       threshold  = $4,
       is_enabled = $5,
       updated_at = NOW()
 WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: ToggleAlertRule :one
UPDATE alert_rules
   SET is_enabled = NOT is_enabled,
       updated_at = NOW()
 WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteAlertRule :exec
UPDATE alert_rules
   SET deleted_at = NOW(),
       updated_at = NOW()
 WHERE id = $1 AND deleted_at IS NULL;
