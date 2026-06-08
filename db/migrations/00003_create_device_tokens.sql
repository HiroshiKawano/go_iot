-- +goose Up
-- +goose StatementBegin
-- Laravel Sanctum の personal_access_tokens を Go 向けに簡略化した構造
-- デバイスAPI (POST /api/sensor-data) の Bearer 認証に使用
CREATE TABLE device_tokens (
    id           INTEGER      PRIMARY KEY,
    user_id      INTEGER      NOT NULL,
    name         VARCHAR(255) NOT NULL,
    token_hash   VARCHAR(64)  NOT NULL,
    abilities    json         NOT NULL DEFAULT '[]',
    last_used_at DATETIME,
    expires_at   DATETIME,
    created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX device_tokens_token_hash_unique ON device_tokens(token_hash);
CREATE INDEX device_tokens_user_id_idx ON device_tokens(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS device_tokens;
-- +goose StatementEnd
