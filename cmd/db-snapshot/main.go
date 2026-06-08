// DBスナップショット生成CLI。
// SQLite データベースを内省し、テーブル定義 (Markdown) または ER図 (Mermaid) を出力する。
//
// 主目的: 実DBに接続しなくても、生成済みファイルを読むだけでスキーマを把握できる
// ドキュメント資産を作ること (AI エージェント・新規参入者向け)。
//
// 前提: `make migrate-up` で SQLite ファイル (DATABASE_URL=file:...) にスキーマが適用済みであること。
//
// 使い方:
//
//	make db-snapshot                                  # 両形式を docs/database_snapshot/ に出力
//	go run ./cmd/db-snapshot -format=markdown          # 標準出力へ
//	go run ./cmd/db-snapshot -format=mermaid -out=er.mmd
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/dbsnapshot"
	infradb "github.com/HiroshiKawano/go_iot/internal/infra/db"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("db-snapshot failed: %v", err)
	}
}

func run() error {
	var (
		format string
		out    string
	)
	flag.StringVar(&format, "format", "markdown", "出力形式: markdown | mermaid")
	flag.StringVar(&out, "out", "", "出力先ファイル (省略時は標準出力)")
	flag.Parse()

	// このツールは DATABASE_URL のみ必要 (SESSION_SECRET 等は不要)。
	// .env は Makefile の `include .env` / シェルスクリプトの source で読み込まれる前提。
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL が未設定です (ヒント: make db-snapshot で実行するか .env を読み込んでください)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := infradb.NewPool(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()

	schema, err := dbsnapshot.Introspect(ctx, pool)
	if err != nil {
		return fmt.Errorf("introspect: %w", err)
	}
	if len(schema.Tables) == 0 {
		return fmt.Errorf("テーブルが0件です。マイグレーション未適用の可能性があります (make migrate-up を実行してください)")
	}

	var content string
	switch format {
	case "markdown", "md":
		content = dbsnapshot.RenderMarkdown(schema)
	case "mermaid", "mmd":
		content = dbsnapshot.RenderMermaid(schema)
	default:
		return fmt.Errorf("未知の -format: %q (markdown | mermaid のいずれか)", format)
	}

	if out == "" {
		_, err = fmt.Print(content)
		return err
	}
	if err := os.WriteFile(out, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	log.Printf("%s に %d テーブルのスナップショット (%s) を出力しました", out, len(schema.Tables), format)
	return nil
}
