.PHONY: help setup up down dev build build-windows build-windows-gui run tidy test cover migrate-up migrate-down migrate-create migrate-status sqlc templ sync-css db-snapshot clean

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

dev: sync-css ## air でホットリロード開発サーバを起動
	go tool air

## --- ビルド / 実行 ---
build: sync-css ## 本番用バイナリをビルド
	go tool templ generate
	go build -o ./tmp/main ./cmd/server

run: build ## バイナリを実行
	./tmp/main

## --- Windows 単一 .exe クロスビルド (S10 desktop-exe-packaging の先行追加。CGO不要・pure-Go) ---
build-windows: sync-css ## Windows用 .exe をクロスビルド (コンソールあり・ログは標準出力)
	go tool templ generate
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o dist/go_iot.exe ./cmd/server

build-windows-gui: sync-css ## Windows用 .exe をクロスビルド (コンソール窓なし・ログは %LOCALAPPDATA%\go_iot\app.log へ出力)
	go tool templ generate
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -H windowsgui -X github.com/HiroshiKawano/go_iot/internal/applog.Mode=file" -o dist/go_iot.exe ./cmd/server

## --- Go 関連 ---
tidy: ## go.mod / go.sum を整理
	go mod tidy

test: ## 全テストを実行
	go test ./...

# カバレッジ対象外: 生成コード (sqlc=repository / templ=*_templ.go)・CLI エントリポイント (cmd)・embed のみ (docs)。
# いずれも手書き業務ロジックではないため、品質ゲートの分母から除外する。
COVER_EXCLUDE := _templ\.go:|/internal/repository/|/cmd/|/internal/docs/

cover: ## カバレッジ計測 (生成コード/CLI/docs を除く業務ロジック総合を表示し、80%未満なら失敗)
	go test -coverprofile=coverage.out ./...
	@grep -vE '$(COVER_EXCLUDE)' coverage.out > coverage.biz.out
	@echo "--- 業務ロジック カバレッジ (生成コード sqlc/templ・cmd・docs を除外) ---"
	@go tool cover -func=coverage.biz.out | tail -1
	@total=$$(go tool cover -func=coverage.biz.out | awk '/^total:/ { gsub(/%/, "", $$3); print $$3 }'); \
	  awk -v t="$$total" 'BEGIN { if (t+0 < 80) { printf("NG: カバレッジ %s%% は閾値 80%% 未満です\n", t); exit 1 } printf("OK: カバレッジ %s%% (>= 80%%)\n", t) }'

## --- DB マイグレーション (goose) ---
# dialect は sqlite3 (DATABASE_URL は file: の SQLite DSN)。docker-compose の up/down 整理は S10 へ据置き。
# 注: CLI の sqlite3 ドライバは goose ビルド構成依存。テストの migration 適用は goose ライブラリ + modernc で実施済み。
migrate-up: ## マイグレーションを全て適用
	go tool goose -dir db/migrations sqlite3 "$(DATABASE_URL)" up

migrate-down: ## マイグレーションを1つ戻す
	go tool goose -dir db/migrations sqlite3 "$(DATABASE_URL)" down

migrate-status: ## マイグレーションの適用状況を表示
	go tool goose -dir db/migrations sqlite3 "$(DATABASE_URL)" status

migrate-create: ## 新規マイグレーション作成 (例: make migrate-create name=add_users)
	go tool goose -dir db/migrations create $(name) sql

seed: ## 開発用テストデータを投入 (既存のアプリデータは削除される)
	go run ./cmd/seed

gen-token: ## デバイスAPI用トークン発行 (例: make gen-token user=1 name=ハウスA温湿度計)
	go run ./cmd/gen-token -user=$(user) -name="$(name)"

mocks-preview: ## モックHTMLをローカルサーバでプレビュー (http://localhost:8000)
	@echo "モックサーバ起動: http://localhost:8000/login.html"
	@cd mocks/html && python3 -m http.server 8000

## --- コード生成 ---
sqlc: ## sqlc でリポジトリコード生成
	go tool sqlc generate

templ: ## templ でテンプレートコード生成
	go tool templ generate

## --- CSS 単一ソース同期 (正本: mocks/html/style.css) ---
sync-css: ## モックの style.css を本番 public/ へ複製 (本番は生成物・手編集しない)
	@mkdir -p internal/view/public/css
	@cp mocks/html/style.css internal/view/public/css/style.css
	@echo "synced: mocks/html/style.css -> internal/view/public/css/style.css"

## --- DBスナップショット (AI/開発者向けスキーマ資産) ---
db-snapshot: ## 実DBを内省し docs/database_snapshot/ にテーブル定義+ER図を生成 (要 make up + migrate-up)
	@bash scripts/db-snapshot.sh

## --- クリーンアップ ---
clean: ## ビルド成果物を削除
	rm -rf tmp
	find . -name "*_templ.go" -delete
