# プロジェクト構造（structure.md）

> ディレクトリ構成・view 層の設計・CSS 構造の規約。templ 実装はこの構造に従う。

## ディレクトリ構成

```
go_iot/
├── cmd/
│   ├── server/main.go          エントリポイント（合成ルート。API・Web UI・静的・docs・health を配線）
│   ├── seed/main.go            開発用シードデータ投入CLI
│   ├── gen-token/main.go       デバイスAPI用 Bearer トークン発行CLI
│   └── db-snapshot/main.go     DB スキーマ内省CLI（make db-snapshot）
├── internal/
│   ├── config/                 環境変数読込・検証
│   ├── domain/                 Metric / ComparisonOperator Enum
│   ├── infra/{db,pgconv,token}/ pgxpool / 型変換 / トークン
│   ├── auth/                   認証(authN): device_auth.go（Bearer）/ session_auth.go（scs Session）
│   ├── authz/                  認可(authZ): 所有者認可(BOLA防止)を集約（RequireDeviceOwner / RequireAlertRuleOwner）
│   ├── handler/                HTTP ハンドラ（sensor_api / auth / dashboard / device / readings / alert_*）
│   ├── repository/             sqlc 生成コード（全37クエリ）
│   ├── docs/                   OpenAPI YAML + Scalar（go:embed）
│   ├── service/                アラート判定サービス（alert_evaluator.go）
│   ├── chart/                  サーバサイド SVG 線グラフ生成（stdlib のみ・純粋層）
│   ├── timefmt/                相対時刻の日本語整形（純粋層）
│   ├── middleware/             SessionLoad / MethodOverride / CSRF / RequireAuth
│   └── view/                   ★templ（layout / component / page の3層・28ファイル）+ static.go。public/css/style.css は配信用CSS（make sync-css の生成物）
├── db/{migrations,queries}/    goose / sqlc 入力
├── mocks/html/                 全9画面の静的HTMLモック + style.css（CSS自前・Lism非依存）
├── .kiro/steering/             本ステアリング（product.md / tech.md / structure.md）
├── 2cc_sdd/                    設計書群
└── other/                      補助ドキュメント
```

## 依存方向ルール（アーキテクチャ規約・必須）

> 採用方針は「実務的 Layered-lite」（layer-first + 画面=feature ファイル分割）。厳格 Clean Architecture は不採用。
> 以下は安定ルール（常時順守）。**いつ `service` 層を挟むか＝層の厚みの線引き基準は、最初の 1-2 画面で SSR+HTMX 依存フローを検証してから追記する**（現時点では意図的に未定義）。下記は「依存の向き」の規約であり、どの処理で層を挟むかの基準ではない。

1. **依存は下向き一方向**。上位 → 下位のみを許可し、逆流・循環を禁止する。下向きであれば隣接層を飛ばしてよい（チェーンは「呼べる方向」であって「必ず全層を経由する」意味ではない）。
   ```
   cmd（合成ルート） → handler / middleware / auth → service → repository → infra
                                                  └─→ domain（純粋・無依存の値/ルール層。上位から参照のみ）
   ```
   - `cmd/server/main.go` が唯一の合成ルート（依存を生成し配線する）。下位層は上位層を import しない。
   - `domain` は最下流の純粋層で **`infra` も import しない**（上図のとおり repository/service とは別系統。詳細はルール②）。
   - 隣接層スキップの例: `handler` が `service` を経由せず `repository` を直接呼ぶ構成も許容する（どの処理で `service` を挟むかの基準は上記のとおり検証後に確定）。

2. **domain の純粋性を死守**。`internal/domain/` は標準ライブラリ（現状 `fmt` のみ）に依存し、`repository` / `infra` / `gin` / DB 型（pgtype 等）を一切 import しない。

3. **DIP は 2 点限定**。むやみに interface を増やさない。
   - **DB ポート = `repository.Querier`**（sqlc `emit_interface=true` の生成 interface）。handler / auth は具象 `*repository.Queries` ではなく `Querier` に依存する（テスト時に最小モックへ差し替え可能）。詳細は [tech.md の「データアクセス方針」](./tech.md) を参照。
   - **service の consumer interface は必要時のみ**定義する（消費側が必要とする最小メソッドだけを切り出す）。先回りの抽象化はしない。

4. **view → repository / service を禁止**。view（templ）は handler が渡した表示用データのみを描画する。
   - 許可: `view → domain の表示メソッド`（例: `Metric.Label()` / `Metric.Unit()` 等の表示専用メソッド）。
   - 禁止: view から DB アクセス（repository）やビジネスロジック（service）を直接呼ぶこと。

5. **所有者認可（BOLA 防止）は `internal/authz` に集約**。`device.UserID != userID` の類の所有者チェックを各ハンドラへ散らさず、`authz.RequireDeviceOwner` 等を再利用する（散在は BOLA = Broken Object Level Authorization の温床）。
   - ハンドラは sentinel error（`ErrUnauthenticated` / `pgx.ErrNoRows` / `ErrNotOwner`）を HTTP ステータス（401 / 404・422 / 403）へ写すだけにする。
   - `authz` の consumer は最小 interface（`DeviceGetter` 等）に依存し、`userID<=0`（未認証）は所有者判定の前に fail-closed する。

## view 層の3層構成（`internal/view/`）

| 層 | 役割 | 例 |
|---|---|---|
| **layout** | 共通レイアウト（children 受け取り型） | `App.templ`（認証後: header+sidebar+main枠）/ `Guest.templ`（login/register） |
| **page** | フルページ画面 | `Dashboard.templ` / `DeviceShow.templ` / `Readings.templ` 等 |
| **component** | 再利用部品・HTMX 部分更新ターゲット | `SiteHeader` / `Sidebar` / `Button` / `Card` / `DataTable` / `DeviceForm` 等 |

### templ コンポーネントの規約

- 部分テンプレートを別ファイル化せず、**ページ用 templ 内 or component に細分化した templ 関数**として定義し、Handler が `HX-Request` 有無で対象関数を直接 `Render` する。
- HTMX 部分更新ターゲットの **id（ケバブケース）** と **templ 関数名（PascalCase）** を対応させる（例: `device-cards` → `DeviceCards`）。
- 2配置パターン: メイン（innerHTML swap、コンテナ内側の中身を返す）/ OOB（outerHTML swap、`id` + `hx-swap-oob="true"` の要素全体を返す）。
- `id` は HTMX 差し替え専用。**スタイリングには使わない**（R01/R02）。

## CSS 構造（単一ソース: `mocks/html/style.css` が唯一の正本）

- **`mocks/html/style.css` を CSS の唯一の正本（single source of truth）とする。** 本番配信用 `internal/view/public/css/style.css` は `make sync-css` がモックから複製する**生成物**で、手編集しない（`.gitignore` 済み）。`build` / `dev` ターゲットは `sync-css` を前段で実行する。
- **双方向に手で直さない。** 「モック側を編集 → `make sync-css` で本番へ反映」の一方向だけ。これにより「片方だけ更新」によるモック↔本番のスタイル乖離が構造的に起きない。
- 配信は `internal/view/static.go` の `//go:embed all:public`（go:embed は親ディレクトリ `..` を辿れないため public は **`internal/view/` 配下**に置く）→ Gin `StaticFS("/static", …)` → `/static/css/style.css?v=…`。詳細は `2cc_sdd/HTMX実装ガイド(動的).md` §40-B「単一ソース運用」。
- **templ はモックの実クラスをそのまま使う**（独自クラスを新設しない＝正典 §31）。CSS をゼロから考え直さず、モックから写経する。
- **Lism 非依存の素のモダンCSS**。詳細方針は [tech.md の「CSS 方針」](./tech.md) を参照（唯一の正）。
- カスケード: `@layer reset, base, components, utilities;`
- トークン（`--space-*` / `--fs-*` / `--radius` 等）は `:root`（base 層）に定義。
- 部品スタイルは **`@layer components`**、ヘルパは **`@layer utilities`（`.u-*`）**。
- ❌ templ の `css` スコープスタイル式は使わない（unlayered 問題）。コンポーネント固有CSSは `@layer components` へ追記（編集先は正本 `mocks/html/style.css`、その後 `make sync-css`）。

### クラス命名

- 部品: `.card` / `.btn`（`.btn-primary` 等の variant）/ `.site-header` / `.data-table` …（独自命名、BEM 風）
- ユーティリティ: `.u-*`（例: `.u-d-inline`, `.u-mbe-3`）
- レイアウト固有値は `:root` のレイアウト変数（`--sidebar-width` 等）で管理。

## 設計の前提（重要）

- **外部キー制約は張らない**（参照整合性はアプリ層 JOIN で担保）。
- **論理削除**（`deleted_at`）採用。sqlc クエリは常に `WHERE deleted_at IS NULL`。
- マスターデータは DB テーブルではなく **Go 定数 + VARCHAR + CHECK 制約**。
- 認証: ESP8266 = 自作 Bearer（SHA-256）/ ブラウザ = Session（scs + pgxstore）+ CSRF（gorilla/csrf）。いずれも実装済み。
