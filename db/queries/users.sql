-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: CreateUser :one
INSERT INTO users (name, email, password_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
   SET name          = $2,
       email         = $3,
       password_hash = $4,
       updated_at    = NOW()
 WHERE id = $1
RETURNING *;

-- name: MarkUserEmailVerified :exec
UPDATE users
   SET email_verified_at = NOW(),
       updated_at        = NOW()
 WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
