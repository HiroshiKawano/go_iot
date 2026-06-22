-- +goose Up
-- +goose StatementBegin
CREATE TABLE alert_histories (
    id            BIGSERIAL     PRIMARY KEY,
    alert_rule_id BIGINT        NOT NULL,
    metric        VARCHAR(20)   NOT NULL,
    operator      VARCHAR(5)    NOT NULL,
    threshold     NUMERIC(5, 2) NOT NULL,
    actual_value  NUMERIC(5, 2) NOT NULL,
    is_notified   BOOLEAN       NOT NULL DEFAULT FALSE,
    triggered_at  TIMESTAMPTZ   NOT NULL,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,
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

COMMENT ON TABLE alert_histories IS '発火したアラートの履歴';
COMMENT ON COLUMN alert_histories.metric IS 'ルール発火時点の指標 (alert_rules の metric を非正規化保持)';
COMMENT ON COLUMN alert_histories.operator IS 'ルール発火時点の演算子 (非正規化保持)';
COMMENT ON COLUMN alert_histories.threshold IS 'ルール発火時点の閾値 (非正規化保持)';
COMMENT ON COLUMN alert_histories.actual_value IS '発火時の実測値';
COMMENT ON COLUMN alert_histories.is_notified IS '通知送信完了フラグ';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS alert_histories;
-- +goose StatementEnd
