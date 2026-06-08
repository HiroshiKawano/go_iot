-- +goose Up
-- +goose StatementBegin
CREATE TABLE alert_histories (
    id            INTEGER     PRIMARY KEY,
    alert_rule_id INTEGER     NOT NULL,
    metric        VARCHAR(20) NOT NULL,
    operator      VARCHAR(5)  NOT NULL,
    threshold     REAL        NOT NULL,
    actual_value  REAL        NOT NULL,
    is_notified   BOOLEAN     NOT NULL DEFAULT FALSE,
    triggered_at  DATETIME    NOT NULL,
    created_at    DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    DATETIME,
    CONSTRAINT alert_histories_metric_valid   CHECK (metric   IN ('temperature', 'humidity')),
    CONSTRAINT alert_histories_operator_valid CHECK (operator IN ('>', '<', '>=', '<='))
);

CREATE INDEX alert_histories_alert_rule_id_idx
    ON alert_histories(alert_rule_id) WHERE deleted_at IS NULL;

CREATE INDEX alert_histories_triggered_at_idx
    ON alert_histories(triggered_at DESC) WHERE deleted_at IS NULL;

-- ダッシュボード未通知バナー表示クエリ用 (is_notified = FALSE を高速検索)
CREATE INDEX alert_histories_unnotified_idx
    ON alert_histories(triggered_at DESC)
    WHERE is_notified = FALSE AND deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS alert_histories;
-- +goose StatementEnd
