# プロジェクト構造

## 組織化の思想

[例: 標準 Go プロジェクトレイアウト (`cmd/` + `internal/`) + 機能ドメインごとのパッケージ分割。外部公開しないコードは `internal/` 配下に置く]

## ディレクトリパターン

### エントリポイント
**場所**: `cmd/<binary-name>/main.go`
**目的**: 各実行バイナリの `main` パッケージ。設定読込と DI 組立のみ
**例**: `cmd/server/main.go`（Gin サーバー起動）

### 非公開アプリケーションコード
**場所**: `internal/<domain>/`
**目的**: ドメインごとに handler / service / repository を同居配置
**例**: `internal/device/` 配下に `handler.go` / `service.go` / `repository.go`

### DB 関連資産
**場所**: `db/`
**目的**: マイグレーション (`db/migrations/`) と sqlc クエリ (`db/queries/`)、生成コード (`internal/db/sqlc/`)
**例**: `db/queries/devices.sql` → sqlc が `internal/db/sqlc/devices.sql.go` を生成

### テンプレート / 静的資産
**場所**: `internal/view/` (templ), `public/` (静的ファイル)
**目的**: `.templ` ファイルから `_templ.go` を生成、CSS/JS/画像は `public/` から配信

## 命名規則

- **ファイル**: `snake_case.go`（`device_handler.go`）。テストは `*_test.go`
- **パッケージ**: 小文字単語のみ、アンダースコアやキャメルケース禁止（`device`, `sensor`）
- **エクスポート識別子**: `PascalCase`（`DeviceService`）
- **非エクスポート識別子**: `camelCase`（`newDeviceService`）
- **インタフェース**: 振る舞いを表す名詞 + `-er`（`Reader`, `DeviceFinder`）。実装に寄せた `IDevice` 接頭辞は使わない

## Import 構成

```go
import (
    // 1. 標準ライブラリ
    "context"
    "fmt"
    "net/http"

    // 2. サードパーティ
    "github.com/gin-gonic/gin"
    "github.com/jackc/pgx/v5/pgxpool"

    // 3. 自プロジェクト
    "github.com/HiroshiKawano/go_iot/internal/device"
    "github.com/HiroshiKawano/go_iot/internal/db/sqlc"
)
```

- `goimports` がグループを自動整列。手で並べ替えない
- モジュールルートは `go.mod` の `module` 宣言（`github.com/HiroshiKawano/go_iot`）
- パス別名は使わない（Go の仕様上不要）

## コード組織の原則

- **依存方向**: `handler` → `service` → `repository`。逆流禁止
- **`internal/` 境界**: 他モジュールから import されてはならないコードは必ず `internal/` 配下
- **`cmd/` は薄く**: ビジネスロジックを置かず、組立と起動のみ
- **循環依存禁止**: パッケージ境界で切る。共通モデルは `internal/domain/` のような中立パッケージへ
- **sqlc 生成コードの扱い**: 直接編集せず、`.sql` ファイルと `sqlc.yaml` を編集して再生成

---
_パターンを文書化する。ファイルツリー全件は記載しない。パターンに沿う新規ファイル追加で更新不要な粒度を保つ_
