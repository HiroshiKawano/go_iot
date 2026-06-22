-- +goose Up
-- +goose StatementBegin
-- Laravel Sanctum の personal_access_tokens を Go 向けに簡略化した構造
-- デバイスAPI (POST /api/sensor-data) の Bearer 認証に使用
CREATE TABLE device_tokens (
    id           BIGSERIAL    PRIMARY KEY,
    user_id      BIGINT       NOT NULL,
    name         VARCHAR(255) NOT NULL,
    token_hash   VARCHAR(64)  NOT NULL,
    abilities    JSONB        NOT NULL DEFAULT '[]'::jsonb,
    last_used_at TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX device_tokens_token_hash_unique ON device_tokens(token_hash);
CREATE INDEX device_tokens_user_id_idx ON device_tokens(user_id);

COMMENT ON TABLE device_tokens IS 'デバイスAPI用 Bearer トークン (Sanctum相当)';
COMMENT ON COLUMN device_tokens.name IS 'トークン名 (デバイス名と合わせる運用)';
COMMENT ON COLUMN device_tokens.token_hash IS 'SHA-256 ハッシュ化済トークン (平文は保存しない)';
COMMENT ON COLUMN device_tokens.abilities IS '権限一覧 (例: ["sensor:write"])';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS device_tokens;
-- +goose StatementEnd
