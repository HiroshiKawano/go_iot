-- +goose Up
-- +goose StatementBegin
-- MACアドレス形式の検証は SQLite に正規表現 CHECK がないためアプリ層
-- (device.go の isValidMacFormat) へ委譲する。
CREATE TABLE devices (
    id                   INTEGER      PRIMARY KEY,
    user_id              INTEGER      NOT NULL,
    name                 VARCHAR(255) NOT NULL,
    mac_address          VARCHAR(17)  NOT NULL,
    location             VARCHAR(255),
    is_active            BOOLEAN      NOT NULL DEFAULT TRUE,
    last_communicated_at DATETIME,
    created_at           DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at           DATETIME
);

-- MACアドレスは論理削除されていないデバイス間でユニーク
-- (削除済みデバイスが複数存在しても新規デバイス登録を妨げない)
CREATE UNIQUE INDEX devices_mac_address_unique_active
    ON devices(mac_address) WHERE deleted_at IS NULL;

CREATE INDEX devices_user_id_idx
    ON devices(user_id) WHERE deleted_at IS NULL;

CREATE INDEX devices_is_active_idx
    ON devices(is_active) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS devices;
-- +goose StatementEnd
