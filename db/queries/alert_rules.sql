-- name: GetAlertRule :one
SELECT * FROM alert_rules
 WHERE id = ? AND deleted_at IS NULL;

-- name: ListAlertRulesByDevice :many
SELECT * FROM alert_rules
 WHERE device_id = ? AND deleted_at IS NULL
 ORDER BY created_at ASC;

-- name: ListEnabledAlertRulesByDevice :many
SELECT * FROM alert_rules
 WHERE device_id  = ?
   AND is_enabled = TRUE
   AND deleted_at IS NULL;

-- name: CreateAlertRule :one
INSERT INTO alert_rules (device_id, metric, operator, threshold, is_enabled)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateAlertRule :one
UPDATE alert_rules
   SET metric     = ?,
       operator   = ?,
       threshold  = ?,
       is_enabled = ?,
       updated_at = datetime('now')
 WHERE id = ? AND deleted_at IS NULL
RETURNING *;

-- name: ToggleAlertRule :one
UPDATE alert_rules
   SET is_enabled = NOT is_enabled,
       updated_at = datetime('now')
 WHERE id = ? AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteAlertRule :exec
UPDATE alert_rules
   SET deleted_at = datetime('now'),
       updated_at = datetime('now')
 WHERE id = ? AND deleted_at IS NULL;
