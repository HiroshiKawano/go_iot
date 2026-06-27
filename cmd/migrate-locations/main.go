// 既存デバイスの自由入力「設置場所」(devices.location) を構造化所在地 (devices.locality)
// へ非破壊・冪等に移行する一度きりの実行ツール。
//
// 重要な設計方針 (既存データ保護):
//   - 全テーブルを TRUNCATE する cmd/seed とは異なり、本ツールは一切 TRUNCATE しない。
//   - location が沖縄の地域に対応付けられる行のみ locality を設定する (UPDATE は locality 列のみ)。
//     location・他フィールド・件数は不変、削除を伴わない (非破壊)。
//   - locality が既に設定済みの行はスキップするため、再実行は冪等 (更新 0 件)。
//   - 移行ロジック本体は internal/locationbackfill.BackfillLocations (DB 非依存にテスト済み)。
//
// 使い方:
//
//	make migrate-locations
//
// 前提: make up + make migrate-up 済み (00008 で locality 列が追加されていること)。
package main

import (
	"context"
	"log"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/config"
	infradb "github.com/HiroshiKawano/go_iot/internal/infra/db"
	"github.com/HiroshiKawano/go_iot/internal/locationbackfill"
	"github.com/HiroshiKawano/go_iot/internal/repository"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("migrate-locations failed: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := infradb.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	q := repository.New(pool)

	updated, err := locationbackfill.BackfillLocations(ctx, q)
	if err != nil {
		return err
	}

	log.Printf("  ✓ locality backfill: %d 件のデバイスに所在地を設定", updated)
	log.Println("migrate-locations complete")
	return nil
}
