# 技術スタック

## アーキテクチャ

[高レベルのシステム設計アプローチ。例: Gin によるレイヤード構成 (handler → service → repository)、server-rendered HTMX + templ]

## コア技術

- **言語**: Go 1.26+ （ESP32 ファームウェアとの言語統一が導入動機）
- **Web フレームワーク**: [例: gin-gonic/gin v1]
- **テンプレート**: [例: a-h/templ + HTMX]
- **データベース**: [例: PostgreSQL + pgx v5]
- **ビルドツール**: Go モジュール + `go tool` ディレクティブ（プロジェクトローカル化）

## 主要ライブラリ

[開発パターンに影響する主要なものだけ記載]

- `github.com/jackc/pgx/v5` — PostgreSQL ドライバ
- `github.com/sqlc-dev/sqlc` — SQL → Go 型コード生成（`go tool sqlc`）
- `github.com/pressly/goose/v3` — マイグレーション（`go tool goose`）
- `github.com/a-h/templ` — 型安全 HTML テンプレート（`go tool templ`）
- `github.com/air-verse/air` — ホットリロード（`go tool air`）
- `github.com/go-playground/validator/v10` — 構造体バリデーション

## 開発標準

### 型安全性
- Go の静的型付けに従う。`any` / `interface{}` の濫用は避ける
- DB 層は sqlc 生成コードを使い、手書き SQL + 手マッピングを避ける

### コード品質
- `gofmt` / `goimports` で自動整形（コミット前必須）
- `go vet` と `staticcheck` を CI で実行
- 循環 import の禁止、パッケージ間境界を明確化

### テスト
- 標準 `testing` パッケージ + テーブル駆動テスト
- カバレッジ: 主要パッケージ 80% 以上
- 統合テストは `*_integration_test.go` に分離しビルドタグで切替

## 開発環境

### 必須ツール
- Go 1.26+ （`go.mod` に宣言したバージョン）
- Docker / OrbStack（PostgreSQL コンテナ）
- `go tool` で導入される CLI は `go.mod` の `tool` ディレクティブで管理。グローバル `go install` は原則使わない

### よく使うコマンド
```bash
# 開発サーバー (ホットリロード):    go tool air
# テンプレート生成:                 go tool templ generate
# SQL コード生成:                   go tool sqlc generate
# マイグレーション:                 go tool goose -dir db/migrations postgres "$DSN" up
# テスト:                           go test ./...
# カバレッジ:                       go test -cover ./...
# ビルド:                           go build -o bin/server ./cmd/server
```

## 主要な技術的意思決定

[重要なアーキテクチャ選択と根拠を記録する]

- 例: **server-rendered HTMX + templ を採用**（SPA ではなく）— IoT 用管理画面は同期 UX で十分、ビルドパイプラインを Go に一本化できるため
- 例: **ORM ではなく sqlc を採用** — SQL を隠蔽せず、型安全なコード生成で N+1 を回避しやすい

---
_標準とパターンを文書化する。依存全件の列挙はしない_
