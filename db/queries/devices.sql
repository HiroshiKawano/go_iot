-- name: GetDevice :one
SELECT * FROM devices
 WHERE id = ? AND deleted_at IS NULL;

-- name: GetDeviceByMacAddress :one
SELECT * FROM devices
 WHERE mac_address = ? AND deleted_at IS NULL;

-- name: ListDevicesByUser :many
SELECT * FROM devices
 WHERE user_id = ? AND deleted_at IS NULL
 ORDER BY created_at DESC;

-- name: CreateDevice :one
INSERT INTO devices (user_id, name, mac_address, location, is_active)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateDevice :one
UPDATE devices
   SET name        = ?,
       mac_address = ?,
       location    = ?,
       is_active   = ?,
       updated_at  = datetime('now')
 WHERE id = ? AND deleted_at IS NULL
RETURNING *;

-- name: UpdateDeviceLastCommunicated :exec
UPDATE devices
   SET last_communicated_at = datetime('now'),
       updated_at           = datetime('now')
 WHERE id = ? AND deleted_at IS NULL;

-- name: SoftDeleteDevice :exec
UPDATE devices
   SET deleted_at = datetime('now'),
       updated_at = datetime('now')
 WHERE id = ? AND deleted_at IS NULL;
