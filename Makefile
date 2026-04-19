.PHONY: help setup up down dev build run tidy migrate-up migrate-down migrate-create migrate-status sqlc templ clean

# .env を読み込む (存在すれば)
ifneq (,$(wildcard .env))
  include .env
  export
endif

help: ## ヘルプ表示
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## --- 初回セットアップ ---
setup: ## 初回セットアップ (依存とツールをダウンロード)
	go mod download
	@test -f .env || cp .env.example .env && echo ".env を作成しました"
	@echo "次のステップ: make up && make dev"

## --- 開発サーバ ---
up: ## docker-compose で PostgreSQL を起動
	docker compose up -d

down: ## PostgreSQL を停止
	docker compose down

dev: ## air でホットリロード開発サーバを起動
	go tool air

## --- ビルド / 実行 ---
build: ## 本番用バイナリをビルド
	go tool templ generate
	go build -o ./tmp/main ./cmd/server

run: build ## バイナリを実行
	./tmp/main

## --- Go 関連 ---
tidy: ## go.mod / go.sum を整理
	go mod tidy

## --- DB マイグレーション (goose) ---
migrate-up: ## マイグレーションを全て適用
	go tool goose -dir db/migrations postgres "$(DATABASE_URL)" up

migrate-down: ## マイグレーションを1つ戻す
	go tool goose -dir db/migrations postgres "$(DATABASE_URL)" down

migrate-status: ## マイグレーションの適用状況を表示
	go tool goose -dir db/migrations postgres "$(DATABASE_URL)" status

migrate-create: ## 新規マイグレーション作成 (例: make migrate-create name=add_users)
	go tool goose -dir db/migrations create $(name) sql

seed: ## 開発用テストデータを投入 (既存のアプリデータは削除される)
	go run ./cmd/seed

## --- コード生成 ---
sqlc: ## sqlc でリポジトリコード生成
	go tool sqlc generate

templ: ## templ でテンプレートコード生成
	go tool templ generate

## --- クリーンアップ ---
clean: ## ビルド成果物を削除
	rm -rf tmp
	find . -name "*_templ.go" -delete
