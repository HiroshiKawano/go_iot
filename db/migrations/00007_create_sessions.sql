-- +goose Up
-- +goose StatementBegin
-- scs (alexedwards/scs) の SQLite ストア (sqlite3store) が要求するスキーマ。
-- ストアはテーブルを自動生成しないため、ここで明示的に作成する。
-- アプリは scs API 経由でのみ read/write し、sqlc (repository.Querier) では扱わない。
CREATE TABLE sessions (
    token  TEXT PRIMARY KEY,
    data   BLOB NOT NULL,
    expiry REAL NOT NULL
);

-- 期限切れセッションのクリーンアップ (sqlite3store のバックグラウンド goroutine が利用) を効率化する。
CREATE INDEX sessions_expiry_idx ON sessions (expiry);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sessions;
-- +goose StatementEnd
