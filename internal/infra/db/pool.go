package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite" // database/sql ドライバ "sqlite" を登録(pure-Go・CGO 不要)
)

// NewPool は SQLite ファイル DSN から database/sql の *sql.DB を構築し、
// WAL / busy_timeout / foreign_keys の PRAGMA を全接続へ適用したうえで疎通確認する。
//
// 関数名は呼び出し側(cmd/server)互換のため NewPool を維持するが、SQLite では
// 「コネクションプール」概念は薄く、単一 writer 前提で接続数を絞る。
// 本プロジェクトでは sqlc(database/sql) を採用しており、この *sql.DB をそのまま
// repository.New() / セッションストアへ渡して使用する。
func NewPool(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", withPragmas(dsn))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite は単一 writer。書込はシリアライズされるため接続を絞り(従来の pgx
	// MaxConns=10 は SQLite には不適切)、WAL により読取は writer と並行できる。
	// busy_timeout(withPragmas)で scs cleanup × ESP32 INSERT の競合(SQLITE_BUSY)を
	// 待機吸収するため、接続数を控えめにしても読取がブロックされ続けることはない。
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxIdleTime(5 * time.Minute)
	// SetConnMaxLifetime は設定しない: ローカルファイル接続でネットワーク切断リスクがなく、
	// 接続を定期的に張り直す必要がない(pgx の MaxConnLifetime=30min 相当は SQLite に不要)。

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	return db, nil
}

// withPragmas は modernc.org/sqlite の DSN に PRAGMA を _pragma クエリとして付与し、
// プール内の各接続生成時に WAL / busy_timeout / foreign_keys が適用されるようにする。
//
// modernc は _pragma=NAME(VALUE) 形式を独自にパースするため括弧は URL エンコードしない。
// journal_mode=WAL は DB ファイル属性として永続化され、busy_timeout / foreign_keys は
// 接続ごとに必要なため _pragma で全接続へ確実に適用する。
func withPragmas(dsn string) string {
	pragmas := strings.Join([]string{
		"_pragma=journal_mode(WAL)",
		"_pragma=busy_timeout(5000)",
		"_pragma=foreign_keys(1)",
	}, "&")

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + pragmas
}
