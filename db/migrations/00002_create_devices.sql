-- +goose Up
-- +goose StatementBegin
CREATE TABLE devices (
    id                   BIGSERIAL    PRIMARY KEY,
    user_id              BIGINT       NOT NULL,
    name                 VARCHAR(255) NOT NULL,
    mac_address          VARCHAR(17)  NOT NULL,
    location             VARCHAR(255),
    is_active            BOOLEAN      NOT NULL DEFAULT TRUE,
    last_communicated_at TIMESTAMPTZ,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMPTZ,
    CONSTRAINT devices_mac_address_format CHECK (
        mac_address ~ '^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$'
    )
);

-- MACアドレスは論理削除されていないデバイス間でユニーク
-- (削除済みデバイスが複数存在しても新規デバイス登録を妨げない)
CREATE UNIQUE INDEX devices_mac_address_unique_active
    ON devices(mac_address) WHERE deleted_at IS NULL;

CREATE INDEX devices_user_id_idx
    ON devices(user_id) WHERE deleted_at IS NULL;

CREATE INDEX devices_is_active_idx
    ON devices(is_active) WHERE deleted_at IS NULL;

COMMENT ON TABLE devices IS 'ESP8266デバイス管理';
COMMENT ON COLUMN devices.mac_address IS 'MACアドレス (例: AA:BB:CC:DD:EE:FF)';
COMMENT ON COLUMN devices.deleted_at IS '論理削除日時 (NULL = 有効)';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS devices;
-- +goose StatementEnd
