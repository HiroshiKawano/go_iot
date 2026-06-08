-- +goose Up
-- +goose StatementBegin
CREATE TABLE sensor_readings (
    id          INTEGER  PRIMARY KEY,
    device_id   INTEGER  NOT NULL,
    temperature REAL     NOT NULL,
    humidity    REAL     NOT NULL,
    recorded_at DATETIME NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at  DATETIME,
    CONSTRAINT sensor_readings_temperature_range CHECK (temperature BETWEEN -40 AND 125),
    CONSTRAINT sensor_readings_humidity_range    CHECK (humidity    BETWEEN 0   AND 100)
);

-- グラフ表示クエリ (直近24時間) の高速化用 複合インデックス
CREATE INDEX sensor_readings_device_id_recorded_at_idx
    ON sensor_readings(device_id, recorded_at DESC) WHERE deleted_at IS NULL;

-- 期間指定検索 (全デバイス横断) 用
CREATE INDEX sensor_readings_recorded_at_idx
    ON sensor_readings(recorded_at DESC) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sensor_readings;
-- +goose StatementEnd
