-- name: GetDevice :one
SELECT * FROM devices
 WHERE id = $1 AND deleted_at IS NULL;

-- name: GetDeviceByMacAddress :one
SELECT * FROM devices
 WHERE mac_address = $1 AND deleted_at IS NULL;

-- name: ListDevicesByUser :many
SELECT * FROM devices
 WHERE user_id = $1 AND deleted_at IS NULL
 ORDER BY created_at DESC;

-- name: CreateDevice :one
INSERT INTO devices (user_id, name, mac_address, location, is_active, locality, crop, planting_date)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateDevice :one
UPDATE devices
   SET name          = $2,
       mac_address   = $3,
       location      = $4,
       is_active     = $5,
       locality      = $6,
       crop          = $7,
       planting_date = $8,
       updated_at    = NOW()
 WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateDeviceLastCommunicated :exec
UPDATE devices
   SET last_communicated_at = NOW(),
       updated_at           = NOW()
 WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteDevice :exec
UPDATE devices
   SET deleted_at = NOW(),
       updated_at = NOW()
 WHERE id = $1 AND deleted_at IS NULL;

-- name: ListAllDevices :many
-- 全ユーザー横断で有効なデバイスを列挙する (所在地 backfill 専用)。
SELECT * FROM devices
 WHERE deleted_at IS NULL
 ORDER BY id;

-- name: UpdateDeviceLocality :exec
-- locality 列のみを更新する (backfill 用・他フィールドと location は不変)。
UPDATE devices
   SET locality   = $2,
       updated_at = NOW()
 WHERE id = $1 AND deleted_at IS NULL;
