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

## データアクセス方針（DB ポート）

**`emit_interface=true` で sqlc が生成する `repository.Querier` を、本プロジェクト唯一の DB ポートとする。**

- handler / auth / service は具象 `*repository.Queries` ではなく **`repository.Querier`（interface）に依存**する。具象は `Querier` を実装済み（`var _ Querier = (*Queries)(nil)`）なので、合成ルート `cmd/server/main.go` では `repository.New(pool)` の戻り値をそのまま渡せる（配線は無改修）。
- これにより DB 境界をテスト時に無償でモック可能になる（Clean Architecture の主要メリットを安価に回収）。
- ❌ **domain 側に DB ポートを再定義しない**。`Querier` が既にポートの役割を果たすため、`domain` にリポジトリ interface を別途切る（二重管理）ことは禁止。consumer 側で最小 interface が必要な場合のみ、その消費パッケージ内に切り出す（[structure.md「依存方向ルール」DIP 2 点限定](./structure.md) を参照）。
- CLI ツール（`cmd/seed` 等）は handler/auth コンシューマではないため、具象 `*repository.Queries` を直接使ってよい（DIP の対象外）。

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

## HTMX/templ 動的実装の正典（cc-sdd 必読・落とし穴回避）

templ + HTMX + Alpine.js の**動的振る舞い**（部分更新・モーダル・検索/フィルタ・インラインCRUD・バリデーション表示・CSRF・Tom Select ライフサイクル等）には既知の落とし穴が多数あり、**`2cc_sdd/HTMX実装ガイド(動的).md` を唯一の正典**とする。

- cc-sdd の **design / tasks** を書く際は、まず同書冒頭の `## cc-sdd参照ガイド`（優先度★付きセクション索引）を読み、対象画面の該当節を参照すること。特に ★★★: §2 モック→templ+HTMX 変換ルール / templ コンポーネント分割 / 命名規約、§3 id属性一覧、§4 画面別HTMX操作仕様、§7 バリデーションエラー表示、§8 CSRF。Tom Select を使う画面は §16・C12。
- **requirements** では、HTMX 部分更新 / フルページ遷移の別やバリデーション表示方式など「ユーザー観測可能な振る舞い・境界」の把握に留め、実装詳細は持ち込まない（WHAT/HOW 分離）。
- ガイドは約288KB。**丸読みせず索引 → 該当節に絞る**こと。画面ごとの参照節は `2cc_sdd/spec-init-prompts/session-*.md` が行番号付きで列挙している。
- 強制手段（多層）: `/kiro-spec-{requirements,design,quick,tasks}` 実行時に上記参照を `.claude/hooks/inject-htmx-guide-ref.sh`（UserPromptSubmit フック）が自動注入し、各 SKILL.md の Step 1 にも必須参照ステップを内蔵している。

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
