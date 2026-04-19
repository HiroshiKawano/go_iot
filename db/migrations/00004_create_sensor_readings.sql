-- +goose Up
-- +goose StatementBegin
CREATE TABLE sensor_readings (
    id          BIGSERIAL     PRIMARY KEY,
    device_id   BIGINT        NOT NULL,
    temperature NUMERIC(5, 2) NOT NULL,
    humidity    NUMERIC(5, 2) NOT NULL,
    recorded_at TIMESTAMPTZ   NOT NULL,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    CONSTRAINT sensor_readings_temperature_range CHECK (temperature BETWEEN -40 AND 125),
    CONSTRAINT sensor_readings_humidity_range    CHECK (humidity    BETWEEN 0   AND 100)
);

-- グラフ表示クエリ (直近24時間) の高速化用 複合インデックス
CREATE INDEX sensor_readings_device_id_recorded_at_idx
    ON sensor_readings(device_id, recorded_at DESC) WHERE deleted_at IS NULL;

-- 期間指定検索 (全デバイス横断) 用
CREATE INDEX sensor_readings_recorded_at_idx
    ON sensor_readings(recorded_at DESC) WHERE deleted_at IS NULL;

COMMENT ON TABLE sensor_readings IS 'SHT31 からの温湿度計測データ (システムの中核データ)';
COMMENT ON COLUMN sensor_readings.temperature IS '温度 (℃) -40.00 〜 125.00';
COMMENT ON COLUMN sensor_readings.humidity IS '湿度 (%) 0.00 〜 100.00';
COMMENT ON COLUMN sensor_readings.recorded_at IS 'デバイス側での計測日時';
COMMENT ON COLUMN sensor_readings.created_at IS 'サーバ受信日時 (通信遅延の計算に使用)';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sensor_readings;
-- +goose StatementEnd
