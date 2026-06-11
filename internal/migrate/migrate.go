// Package migrate は go:embed で同梱したマイグレーションを起動時に冪等適用する。
//
// 正本は db/migrations。Makefile の sync-migrations が internal/migrate/migrations/ へ
// 一方向複製し (CSS の sync-css と同型・生成物 gitignore)、本パッケージが go:embed で同梱する。
// go:embed は親ディレクトリ (..) を辿れないため、複製先をパッケージ配下に置く必要がある
// (Decision 2)。sync-migrations 未実行時は埋め込み対象が無く compile エラーになる
// (CSS/templ 生成物と同じ make 前提)。
package migrate

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// migrationsDir は embed FS 内のマイグレーション配置ディレクトリ名。
const migrationsDir = "migrations"

// Up は embed された migrations を db に適用する（冪等）。
//
// goose のバージョン管理テーブル (goose_db_version) が冪等性を担保し、初回は全テーブル作成、
// 未適用差分のみ適用、最新時は no-op になる。db は疎通済みであること（NewPool 直後）。
// 失敗時はエラーを返し、呼び出し元 (main) が app.log 記録＋起動中断する (fail fast)。
//
// 本関数は goose のプロセスグローバル状態 (SetBaseFS/SetDialect) を設定するため逐次呼び出しを
// 前提とする（起動時 1 回・テストも逐次）。並行呼び出しはグローバル状態の競合を招くため行わない。
//
// なお goose は各マイグレーションファイルを単一トランザクションで実行するが、途中ファイルの
// 失敗時に先行 commit 済みファイルは巻き戻らず部分適用が残りうる（残存リスク・自動修復はしない）。
func Up(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose dialect 設定: %w", err)
	}
	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
