-- name: CreateDeviceToken :one
INSERT INTO device_tokens (user_id, name, token_hash, abilities, expires_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetDeviceTokenByHash :one
SELECT * FROM device_tokens
 WHERE token_hash = ?
   AND (expires_at IS NULL OR expires_at > datetime('now'));

-- name: UpdateDeviceTokenLastUsed :exec
UPDATE device_tokens
   SET last_used_at = datetime('now'),
       updated_at   = datetime('now')
 WHERE id = ?;

-- name: ListDeviceTokensByUser :many
SELECT id, user_id, name, abilities, last_used_at, expires_at, created_at
  FROM device_tokens
 WHERE user_id = ?
 ORDER BY created_at DESC;

-- name: DeleteDeviceToken :exec
DELETE FROM device_tokens WHERE id = ?;
