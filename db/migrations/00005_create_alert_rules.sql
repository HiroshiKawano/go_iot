-- +goose Up
-- +goose StatementBegin
CREATE TABLE alert_rules (
    id         INTEGER     PRIMARY KEY,
    device_id  INTEGER     NOT NULL,
    metric     VARCHAR(20) NOT NULL,
    operator   VARCHAR(5)  NOT NULL,
    threshold  REAL        NOT NULL,
    is_enabled BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    CONSTRAINT alert_rules_metric_valid   CHECK (metric   IN ('temperature', 'humidity')),
    CONSTRAINT alert_rules_operator_valid CHECK (operator IN ('>', '<', '>=', '<='))
);

CREATE INDEX alert_rules_device_id_is_enabled_idx
    ON alert_rules(device_id, is_enabled) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS alert_rules;
-- +goose StatementEnd
