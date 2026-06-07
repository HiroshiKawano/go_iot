-- +goose Up
-- +goose StatementBegin
-- scs (alexedwards/scs) の PostgreSQL ストア (pgxstore) が要求するスキーマ。
-- ストアはテーブルを自動生成しないため、ここで明示的に作成する。
-- アプリは scs API 経由でのみ read/write し、sqlc (repository.Querier) では扱わない。
CREATE TABLE sessions (
    token  TEXT        PRIMARY KEY,
    data   BYTEA       NOT NULL,
    expiry TIMESTAMPTZ NOT NULL
);

-- 期限切れセッションのクリーンアップ (pgxstore のバックグラウンド goroutine が利用) を効率化する。
CREATE INDEX sessions_expiry_idx ON sessions (expiry);

COMMENT ON TABLE sessions IS 'Web UI の Session 認証データ (scs/pgxstore が管理。sqlc 対象外)';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sessions;
-- +goose StatementEnd
