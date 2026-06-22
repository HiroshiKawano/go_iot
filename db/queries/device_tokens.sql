-- name: CreateDeviceToken :one
INSERT INTO device_tokens (user_id, name, token_hash, abilities, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetDeviceTokenByHash :one
-- デバイスからの Bearer リクエスト受信時に token_hash で検索し認証に使用
SELECT * FROM device_tokens
 WHERE token_hash = $1
   AND (expires_at IS NULL OR expires_at > NOW());

-- name: UpdateDeviceTokenLastUsed :exec
UPDATE device_tokens
   SET last_used_at = NOW(),
       updated_at   = NOW()
 WHERE id = $1;

-- name: ListDeviceTokensByUser :many
SELECT id, user_id, name, abilities, last_used_at, expires_at, created_at
  FROM device_tokens
 WHERE user_id = $1
 ORDER BY created_at DESC;

-- name: DeleteDeviceToken :exec
DELETE FROM device_tokens WHERE id = $1;
