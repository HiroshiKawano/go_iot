.PHONY: help setup dev build build-windows build-windows-gui run tidy test cover migrate-up migrate-down migrate-create migrate-status sqlc templ sync-css sync-migrations db-snapshot clean

# .env を読み込む (存在すれば)
ifneq (,$(wildcard .env))
  include .env
  export
endif

help: ## ヘルプ表示
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## --- 初回セットアップ ---
# SQLite (組込ファイル型) のため DB サーバの起動は不要。env も任意 (未設定ならアプリデータ配下に
# app.db を自動作成し、起動時マイグレーションを自動適用する)。
setup: ## 初回セットアップ (依存をダウンロード)
	go mod download
	@echo "SQLite のため DB サーバ起動は不要です。次のステップ: make dev"

## --- 開発サーバ ---
dev: sync-css sync-migrations ## air でホットリロード開発サーバを起動
	go tool air

## --- ビルド / 実行 ---
build: sync-css sync-migrations ## 本番用バイナリをビルド
	go tool templ generate
	go build -o ./tmp/main ./cmd/server

run: build ## バイナリを実行
	./tmp/main

## --- Windows 単一 .exe クロスビルド (S10 desktop-exe-packaging。CGO不要・pure-Go) ---
# VERSION はキャッシュバスティング用に internal/view.Version へ ldflags 注入する (既定は git SHA、無ければ "dev")。
# git が無い CI 等でも失敗しないよう 2>/dev/null でフォールバックする。
VERSION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

build-windows: sync-css sync-migrations ## Windows用 .exe をクロスビルド (コンソールあり・ログは標準出力)
	go tool templ generate
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X github.com/HiroshiKawano/go_iot/internal/view.Version=$(VERSION)" -o dist/go_iot.exe ./cmd/server

build-windows-gui: sync-css sync-migrations ## Windows用 .exe をクロスビルド (コンソール窓なし・ログは %LOCALAPPDATA%\go_iot\app.log へ出力)
	go tool templ generate
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -H windowsgui -X github.com/HiroshiKawano/go_iot/internal/applog.Mode=file -X github.com/HiroshiKawano/go_iot/internal/view.Version=$(VERSION)" -o dist/go_iot.exe ./cmd/server

## --- Go 関連 ---
tidy: ## go.mod / go.sum を整理
	go mod tidy

test: sync-migrations ## 全テストを実行
	go test ./...

# カバレッジ対象外: 生成コード (sqlc=repository / templ=*_templ.go)・CLI エントリポイント (cmd)・embed のみ (docs)。
# いずれも手書き業務ロジックではないため、品質ゲートの分母から除外する。
COVER_EXCLUDE := _templ\.go:|/internal/repository/|/cmd/|/internal/docs/

cover: sync-migrations ## カバレッジ計測 (生成コード/CLI/docs を除く業務ロジック総合を表示し、80%未満なら失敗)
	go test -coverprofile=coverage.out ./...
	@grep -vE '$(COVER_EXCLUDE)' coverage.out > coverage.biz.out
	@echo "--- 業務ロジック カバレッジ (生成コード sqlc/templ・cmd・docs を除外) ---"
	@go tool cover -func=coverage.biz.out | tail -1
	@total=$$(go tool cover -func=coverage.biz.out | awk '/^total:/ { gsub(/%/, "", $$3); print $$3 }'); \
	  awk -v t="$$total" 'BEGIN { if (t+0 < 80) { printf("NG: カバレッジ %s%% は閾値 80%% 未満です\n", t); exit 1 } printf("OK: カバレッジ %s%% (>= 80%%)\n", t) }'

## --- DB マイグレーション (goose) ---
# dialect は sqlite3 (DATABASE_URL は file: の SQLite DSN)。本番起動時は internal/migrate が
# go:embed したマイグレーションを自動適用するため、これらの CLI ターゲットは開発時の手動操作用。
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

## --- マイグレーション単一ソース同期 (正本: db/migrations) ---
# 正本 db/migrations を internal/migrate/migrations/ へ一方向複製し go:embed で同梱する
# (CSS の sync-css と同型。複製先は生成物・gitignore・手編集しない)。
# 正本で削除されたファイルが複製先に残らないよう、複製前に *.sql を全消去する。
sync-migrations: ## db/migrations を internal/migrate/migrations/ へ複製 (embed 用生成物・手編集しない)
	@mkdir -p internal/migrate/migrations
	@rm -f internal/migrate/migrations/*.sql
	@cp db/migrations/*.sql internal/migrate/migrations/
	@echo "synced: db/migrations/*.sql -> internal/migrate/migrations/"

## --- DBスナップショット (AI/開発者向けスキーマ資産) ---
db-snapshot: ## 実DBを内省し docs/database_snapshot/ にテーブル定義+ER図を生成 (要 migrate-up 済みの SQLite ファイル)
	@bash scripts/db-snapshot.sh

## --- クリーンアップ ---
clean: ## ビルド成果物を削除
	rm -rf tmp
	find . -name "*_templ.go" -delete
