# 技術スタック（tech.md）

> プロジェクト全体の技術選定とスタイリング方針。cc-sdd / templ 実装はこの方針に従う。

## 概要

ESP32 温湿度センサー（SHT31）のデータを Go バックエンドで受信・蓄積し、ブラウザで可視化・アラート管理する農業IoTシステム。フロント用 JSON API を持たず、**templ が HTML を直接返す**（グラフ含む全表示ロジックをサーバサイドに集約）。

## スタック

| 領域 | 採用技術 | 補足 |
|------|---------|------|
| 言語 | **Go 1.26** | module: `github.com/HiroshiKawano/go_iot` |
| Web フレームワーク | **Gin** v1.12 | Echo から移行済み。`ShouldBindJSON` + `binding` タグ |
| DB | **PostgreSQL 16** + pgx/v5 | docker-compose（16-alpine） |
| クエリ生成 | **sqlc** v1.30 | `db/queries/*.sql` → `internal/repository/*.go`、`emit_interface=true` |
| マイグレーション | **goose** v3 | `db/migrations/*.sql` |
| テンプレート | **templ** v0.3（a-h/templ） | Blade/Eloquent ではない。templ 用語で記述する |
| フロント動的化 | **HTMX** + **Alpine.js** | 部分更新は HTMX、軽い UI 状態は Alpine |
| **CSS / スタイリング** | **素のモダンCSS（フレームワーク非依存）** | ★下記「CSS 方針」参照。Lism CSS は採用しない |
| バリデーション | go-playground/validator v10 | Gin の binding タグ経由 |
| パスワード | golang.org/x/crypto（bcrypt） | |
| API ドキュメント | **Scalar UI** + OpenAPI 3.0.3 | go:embed 同梱 |
| ホットリロード | air | `go tool air`、pre_cmd で `templ generate` |
| 開発ツール管理 | **go tool ディレクティブ** | air/goose/sqlc/templ をプロジェクトローカル完結（グローバル非依存） |

---

## CSS 方針（★重要・レビュー観点）

**本プロジェクトは CSS フレームワークを使わず、素のモダンCSS で統一する。**
2026-06-03 に Lism CSS を完全 drop した（実利用が token+reset のみ＝約10-15%で過剰、レスポンシブ要件も軽量だったため）。**外部CSSフレームワーク（Lism 含む）を再導入しない。**

### トークン（`:root` に定義。templ 実装では `internal/view` 配下の共通CSSへ移植）

| 種類 | 変数 | 値 |
|---|---|---|
| スペース（4pxグリッド） | `--space-1..10` | 1=4px / 2=8px / 3=12px / 4=16px / 6=24px / 10=40px |
| フォントサイズ（**rem固定**） | `--fs-sm/base/lg/xl/2xl` | 14 / 16 / 18 / 21 / 25.6px |
| 角丸・影・境界・幅 | `--radius` `--shadow-sm/md` `--color-border` `--container-l/xs` | |
| 色 | `--color-primary/danger/warning/muted/bg/surface/text` | |
| レイアウト固定値 | `--sidebar-width` `--header-height` | 220px / 56px |

> フォントは **em 追従を使わず rem 固定**（親 fz 非依存でフォーム要素の高さブレを根絶）。

### カスケード（自前 `@layer`）

```css
@layer reset, base, components, utilities;  /* 後ろほど強い */
```
- **reset**: リセットCSS（`*{margin:0}`・box-sizing・`:is(ul,ol)[class]{list-style:none}`・form `font:inherit` 等）
- **base**: `:root` トークン + 要素ベース
- **components**: `.card` / `.btn` / `.site-header` 等の部品スタイル
- **utilities**: `.u-*` ヘルパ（components を上書きできる）

### レスポンシブの優先順位

1. **grid `auto-fill`/`auto-fit` + `minmax()`**（intrinsic に再流動。ブレークポイント不要）
2. **`clamp()`**（流体タイポ／余白）
3. **`@container`**（部品自身の幅で切替＝コンポーネント自己レスポンシブ。配置場所に非依存）
4. **`@media`**（ビューポート起因の切替のみ。例: サイドバー畳み）

### レビュー観点（必須チェック）

1. ❌ **templ の `css` スコープスタイル式（`css name(){...}`）を使わない。**
   生成される `<style>` は `@layer` 非所属（unlayered）で、CSS Cascade Layers 仕様上 utilities すら上書きしてしまい、カスケード設計が壊れる。
2. コンポーネント固有CSSは **`@layer components` の内側**へ追記する（`@layer` 外に書かない）。
3. 装飾はトークン（`var(--space-*)` 等）で書く。生値の散らばりを避ける。
4. `id` はスタイリングに使わない（HTMX 差し替え専用。R01/R02）。スタイルは class のみ。
5. レスポンシブは上記の優先順位で。安易に `@media` を増やさない。
6. 外部CSSフレームワーク（Lism 含む）を再導入しない。

---

## 主要コマンド（Makefile）

```
make dev          # air でホットリロード開発サーバ
make up / down    # PostgreSQL 起動 / 停止
make migrate-up   # マイグレーション適用（goose）
make sqlc         # sqlc でリポジトリ生成
make templ        # templ でテンプレート生成
make seed         # 開発用テストデータ投入
make mocks-preview  # モックHTMLをプレビュー（localhost:8000）
```
