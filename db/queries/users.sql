-- name: GetUser :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = ?;

-- name: CreateUser :one
INSERT INTO users (name, email, password_hash)
VALUES (?, ?, ?)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
   SET name          = ?,
       email         = ?,
       password_hash = ?,
       updated_at    = datetime('now')
 WHERE id = ?
RETURNING *;

-- name: MarkUserEmailVerified :exec
UPDATE users
   SET email_verified_at = datetime('now'),
       updated_at        = datetime('now')
 WHERE id = ?;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = ?;
