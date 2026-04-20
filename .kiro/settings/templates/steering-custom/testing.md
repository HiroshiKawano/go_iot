# テスト標準

[目的: 何をテストするか、テストをどこに置くか、どう構成するかを示す]

## 思想
- 振る舞いをテストする。実装詳細に依存しない
- 速く・壊れにくいテストを優先。モックは必要最小限に
- クリティカルパスを重点的に。カバレッジ 100% の追求より深さを優先

## 配置
Go の慣例に従う:
- **同居配置 (デフォルト)**: `device.go` + `device_test.go` を同じパッケージに置く
- **外部テスト (ブラックボックス)**: 必要なときだけ `package device_test` を使う
- **統合テスト**: `*_integration_test.go` に分離し、ビルドタグ `//go:build integration` で切替

命名:
- ファイル: `*_test.go`
- テスト関数: `func TestXxx(t *testing.T)` / `func BenchmarkXxx(b *testing.B)`
- サブテスト名: 期待する振る舞いを記述（`t.Run("returns error when token is expired", ...)`）

## テスト種別
- **ユニット**: 単一関数・単一型。外部依存はモックまたはフェイク。ミリ秒オーダー
- **統合**: 複数パッケージまたぎ。DB/HTTP は実体（testcontainers 等）、外部 API のみモック
- **E2E**: 重要ユーザーフローのみ。本番に近い構成で実行

## 構造 (Table-driven + AAA)
```go
func TestCalculateTotal(t *testing.T) {
    tests := []struct {
        name string
        in   []Item
        want int
    }{
        {"空リスト", nil, 0},
        {"消費税込み合計", []Item{{Price: 100}, {Price: 200}}, 330},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Arrange & Act
            got := CalculateTotal(tt.in)
            // Assert
            if got != tt.want {
                t.Errorf("got %d, want %d", got, tt.want)
            }
        })
    }
}
```

## モック & テストデータ
- 外部依存 (DB/HTTP/ファイル) はモックまたはフェイク。**テスト対象本体はモックしない**
- モックはインタフェース越しに。具象型のモックは避ける
- DB はできるだけ実体で（docker-compose / testcontainers）。sqlc 生成コードをモックするより、トランザクション + ロールバックで分離
- ファクトリ関数でテストデータを生成。テスト間で状態を引きずらない
- `t.Cleanup(...)` で後始末。テストデータは意図が読める最小限に

## カバレッジ
- 目標: 主要パッケージ 80% 以上。ドメインコアはより高く
- 計測: `go test -cover -coverprofile=cover.out ./...`
- CI でしきい値を強制。例外はレビューで根拠を残す

---
_パターンと意思決定に集中。ツール固有の設定は別所に置く_
