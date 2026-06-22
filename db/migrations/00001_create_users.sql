-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id                BIGSERIAL    PRIMARY KEY,
    name              VARCHAR(255) NOT NULL,
    email             VARCHAR(255) NOT NULL,
    password_hash     VARCHAR(255) NOT NULL,
    email_verified_at TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX users_email_unique ON users(email);

COMMENT ON TABLE users IS 'ユーザー (Web UI の Session 認証対象)';
COMMENT ON COLUMN users.password_hash IS 'bcrypt または argon2 等でハッシュ化されたパスワード';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
