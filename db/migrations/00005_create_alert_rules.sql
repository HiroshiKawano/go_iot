-- +goose Up
-- +goose StatementBegin
CREATE TABLE alert_rules (
    id         BIGSERIAL     PRIMARY KEY,
    device_id  BIGINT        NOT NULL,
    metric     VARCHAR(20)   NOT NULL,
    operator   VARCHAR(5)    NOT NULL,
    threshold  NUMERIC(5, 2) NOT NULL,
    is_enabled BOOLEAN       NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT alert_rules_metric_valid   CHECK (metric   IN ('temperature', 'humidity')),
    CONSTRAINT alert_rules_operator_valid CHECK (operator IN ('>', '<', '>=', '<='))
);

CREATE INDEX alert_rules_device_id_is_enabled_idx
    ON alert_rules(device_id, is_enabled) WHERE deleted_at IS NULL;

COMMENT ON TABLE alert_rules IS '異常値検知の閾値設定ルール';
COMMENT ON COLUMN alert_rules.metric IS '計測指標 (temperature | humidity) — domain.Metric と対応';
COMMENT ON COLUMN alert_rules.operator IS '比較演算子 (> | < | >= | <=) — domain.ComparisonOperator と対応';
COMMENT ON COLUMN alert_rules.threshold IS '閾値';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS alert_rules;
-- +goose StatementEnd
