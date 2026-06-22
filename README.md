# 農業IoTシステム（go_iot）

**Go 1.26 + Gin + HTMX + PostgreSQL 16** で構成された農業IoTシステムです。
ESP8266（ESP-WROOM-02 系）+ 温湿度センサー（SHT31 等）のデータを Go バックエンドで受信・蓄積し、ブラウザで可視化・アラート管理します。

> **アーキテクチャの要点**
> フロント用 JSON API を持たず、**templ が HTML を直接返す**サーバサイドレンダリング（SSR）構成です。
> 部分更新は **HTMX**、軽い UI 状態は **Alpine.js** で扱い、JS ビルドは行いません。
> データは一方向パイプライン **デバイス →（HTTP + Bearer）→ API → PostgreSQL → templ → ブラウザ** で流れます。
> 詳細な方針は `.kiro/steering/`（`product.md` / `tech.md` / `structure.md`）が唯一の正です。

---

## 必要なソフトウェア

事前に以下をインストールしてください。

| ソフトウェア | 用途 | 全OS共通 |
|---|---|---|
| **Git** | バージョン管理 | ✅ |
| **Go 1.26 以上** | アプリのビルド・実行・テスト | ✅ |
| **Docker Desktop** | PostgreSQL を起動するため（DB のみ。アプリは Docker 外で動く） | ✅ |
| **Make** | `make` コマンド（macOS / Linux は標準。Windows は要インストール） | Windows のみ要追加 |

> **PHP / Composer / Node.js は不要です。**
> このプロジェクトは Laravel ではなく **Go** で書かれています。フロントは HTMX + Alpine.js を CDN で読み込むため **npm ビルドもありません**。
> 開発ツール（air / goose / sqlc / templ）は **`go.mod` の `tool` ディレクティブ**でプロジェクトローカルに管理しており、グローバルインストールは不要です（`go tool air` のように実行します。後述）。

> **macOS をお使いの方へ**
> macOS には `make` がプリインストールされています。**Go と Docker Desktop と Git** を入れれば、`make setup` ですぐ始められます。

> **Go のインストール（macOS / Linux）**
> [https://go.dev/dl/](https://go.dev/dl/) から 1.26 以上を入れるか、Homebrew で `brew install go` でも可。
> インストール後 `go version` で `go1.26` 以上が表示されればOKです。

---

## Windows 環境のセットアップ（Windows ユーザー必読）

Windows では `make` が標準で入っていません。また Go の PATH 設定とシェル環境の選択に注意が必要です。以下の手順で準備してから「環境構築」に進んでください。

### 1. シェル環境を選択する

**推奨: Git Bash**（Git for Windows に同梱）

| シェル | 推奨度 | 理由 |
|--------|--------|------|
| **Git Bash** | ⭐ 推奨 | Unix コマンド（`make`、`cp`、`rm` 等）がそのまま使える。Makefile との相性が最も良い |
| **PowerShell** | △ 可能 | 使えるが、Makefile 内のシェルコマンドが失敗する場合がある |
| **コマンドプロンプト (cmd)** | ✕ 非推奨 | Unix コマンドが使えないため Makefile の実行に支障が出る |

> **重要**: 開発中はシェルを **1つに統一** してください。Git Bash と PowerShell を混在させると、パスの解釈やエスケープの違いで予期しないエラーが発生します。

### 2. Git for Windows をインストールする

[https://gitforwindows.org/](https://gitforwindows.org/) からダウンロードしてインストールしてください。インストール時の設定推奨値：

| 設定項目 | 推奨値 |
|----------|--------|
| Default editor | 使い慣れたエディタ（VS Code 等） |
| PATH environment | **Git from the command line and also from 3rd-party software** |
| Line ending conversions | **Checkout as-is, commit Unix-style line endings** |

### 3. Go をインストールする

[https://go.dev/dl/](https://go.dev/dl/) から **Windows 用インストーラ（1.26 以上）** をダウンロードして実行します。
インストーラが自動で PATH を設定します。Git Bash を**開き直して**から確認してください。

```bash
go version   # 例: go version go1.26.2 windows/amd64
```

> **`go` が見つからない場合**: PATH が反映されていません。Git Bash を再起動するか、`C:\Program Files\Go\bin` と `%USERPROFILE%\go\bin`（`go install` の出力先）を環境変数 PATH に追加してください。

### 4. Docker Desktop をインストールする

[https://www.docker.com/products/docker-desktop/](https://www.docker.com/products/docker-desktop/) からインストールします。

> **WSL 2 バックエンドについて**: インストーラが WSL 2 を要求する場合は、管理者 PowerShell で `wsl --install` を実行し、PC を再起動してから Docker Desktop を起動してください。

### 5. Make をインストールする

以下のいずれかで `make` を入れてください（管理者権限の PowerShell で実行）。

```powershell
# 方法A: Chocolatey（推奨）。未導入なら先に Chocolatey を入れる
Set-ExecutionPolicy Bypass -Scope Process -Force; `
  [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; `
  iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
choco install make

# 方法B: winget
winget install GnuWin32.Make

# 方法C: scoop
scoop install make
```

> **winget / GnuWin32 の場合の注意**: PATH が自動設定されないことがあります。`make --version` が見つからなければ `C:\Program Files (x86)\GnuWin32\bin` を PATH に追加してください。
> Git Bash で `make` が認識されない場合は `~/.bashrc` に `export PATH="/c/ProgramData/chocolatey/bin:$PATH"` を追記し、`source ~/.bashrc` を実行します。

### 6. インストールの確認

Git Bash を開いて、以下がすべて動作することを確認します。

```bash
git --version            # 例: git version 2.45.0.windows.1
go version               # 例: go version go1.26.2 windows/amd64
docker --version         # 例: Docker version 27.x.x
docker compose version   # 例: Docker Compose version v2.x.x
make --version           # 例: GNU Make 4.x
```

> **Windows 固有のトラブルシューティング**
>
> | 症状 | 原因と対処 |
> |------|------------|
> | `make` で `\r: command not found` | 改行コードが CRLF。`git config --global core.autocrlf input` を設定してリポジトリを再 clone する |
> | Docker が `permission denied` | Docker Desktop 未起動、または WSL 2 統合が無効。Settings → Resources → WSL Integration を確認 |
> | `docker compose` が見つからない | Docker Desktop が古い。最新版にアップデート |
> | `localhost:8080` にアクセスできない | Windows Firewall がポートをブロック。ポート 8080・5432 の受信を許可する |

---

## 環境構築

このリポジトリを clone したら、以下の手順でローカル開発環境を立ち上げます。
**アプリ自体は Docker の外**（ホストの Go）で動き、Docker は **PostgreSQL のためだけ**に使います。

### ステップ1: 初回セットアップ

```bash
make setup
```

このコマンドで以下を実行します：

1. Go の依存モジュールをダウンロード（`go mod download`）
2. `.env` が無ければ `.env.example` からコピーして作成

> **開発ツールはここで個別インストールしません。** air / goose / sqlc / templ は `go.mod` の `tool` ディレクティブに登録済みで、`go tool <name>` で実行すると Go が自動で解決します（グローバル汚染なし）。Makefile も内部で `go tool` を呼んでいます。

### ステップ2: PostgreSQL を起動する

```bash
make up
```

`docker compose up -d` で PostgreSQL 16（コンテナ名 `go_iot_db`）を起動します。`healthcheck` 付きなので、`docker compose ps` で `healthy` になるまで数秒待ちます。

### ステップ3: マイグレーションを適用する

```bash
make migrate-up
```

`goose`（`go tool goose`）が `db/migrations/*.sql` を順に適用し、6テーブル（users / devices / device_tokens / sensor_readings / alert_rules / alert_histories）を作成します。
`DATABASE_URL` は `.env` から読み込まれます（デフォルトは `postgres://go_iot:go_iot_dev@localhost:5432/go_iot?sslmode=disable`）。

### ステップ4: 開発用データを投入する（任意）

```bash
make seed
```

テストユーザー1名・デバイス2台・直近24時間分のセンサーデータ・アラートルール・履歴を冪等に投入します（`cmd/seed`）。
**既存のアプリデータは TRUNCATE されます**（`goose_db_version` は温存）。

### ステップ5: 開発サーバを起動する

```bash
make dev
```

`air`（`go tool air`）でホットリロード開発サーバが起動します。`.go` / `.templ` / `.sql` などを保存するたびに、**`templ generate` → ビルド → 再起動**が自動で走ります（`.air.toml` の `pre_cmd` で templ 生成）。

起動後、ブラウザで **http://localhost:8080** にアクセスできます（ポートは `.env` の `APP_PORT`）。

---

### まとめ（初回 clone から起動まで）

```bash
git clone <このリポジトリ>
cd go_iot
make setup        # 依存DL + .env 作成
make up           # PostgreSQL 起動
make migrate-up   # スキーマ適用
make seed         # 開発データ投入（任意）
make dev          # air でホットリロード起動 → http://localhost:8080
```

> **デバイス API を試すには**: Bearer トークンが必要です。`make gen-token user=1 name="ハウスA温湿度計"` で発行し、表示された平文トークンを `Authorization: Bearer <token>` ヘッダに付けて `POST /api/sensor-data` を叩きます（API 仕様は **http://localhost:8080/docs** の Scalar UI を参照）。

---

## プロジェクト構成

```
go_iot/
├── cmd/
│   ├── server/main.go        エントリポイント（合成ルート。API・Web UI・静的・docs・health を配線）
│   ├── seed/main.go          開発用シードデータ投入 CLI（make seed）
│   ├── gen-token/main.go     デバイス API 用 Bearer トークン発行 CLI（make gen-token）
│   └── db-snapshot/main.go   DB スキーマ内省 CLI（make db-snapshot）
├── internal/
│   ├── config/               環境変数の読込・検証
│   ├── domain/               Metric / ComparisonOperator Enum（純粋・無依存層）
│   ├── infra/{db,pgconv,token}/  pgxpool / pgtype 変換 / トークン生成
│   ├── auth/                  認証(authN)：device_auth.go（Bearer）/ session_auth.go（scs Session）
│   ├── authz/                 認可(authZ)：所有者認可(BOLA 防止)を集約
│   ├── handler/               HTTP ハンドラ（Gin。sensor_api / auth / dashboard / device / readings / alert_*）
│   ├── repository/            sqlc 生成コード（全37クエリ。Querier interface）
│   ├── dbsnapshot/            DB 内省＋スナップショット整形（introspect=DB依存 / render=純粋・テスト済）
│   ├── docs/                  OpenAPI YAML + Scalar UI（go:embed）
│   ├── service/               アラート判定サービス（alert_evaluator.go）
│   ├── chart/                 サーバサイド SVG 線グラフ生成（stdlib のみ・純粋層）
│   ├── timefmt/               相対時刻の日本語整形（「N分前」等・純粋層）
│   ├── middleware/            SessionLoad / MethodOverride / CSRF / RequireAuth
│   └── view/                  templ（layout / component / page の3層・28ファイル）+ static.go（go:embed 配信）
├── db/
│   ├── migrations/            goose マイグレーション（*.sql）
│   └── queries/               sqlc 入力クエリ（*.sql）
├── docs/database_snapshot/    ★DB スナップショット（table_definitions.md / er_diagram.mmd。自動生成）
├── scripts/                   補助シェルスクリプト（db-snapshot.sh 等）
├── mocks/html/                全9画面の静的 HTML モック + style.css（素のモダンCSS）
├── .kiro/steering/            プロジェクトメモリ（product.md / tech.md / structure.md）★必読
├── .kiro/specs/               cc-sdd の仕様書ワークスペース（feature 単位）
├── .claude/skills/kiro-*/     cc-sdd スラッシュコマンド定義（/kiro-spec-init 等）
├── 2cc_sdd/                   設計書群（DB設計・画面設計・HTMX実装ガイド・実装計画 等）
├── other/                     補助ドキュメント
├── Makefile                   主要コマンド
├── docker-compose.yml         PostgreSQL（db サービスのみ）
├── go.mod / go.sum            依存 + tool ディレクティブ（air/goose/sqlc/templ）
├── sqlc.yaml                  sqlc 設定（emit_interface=true）
└── .air.toml                  air 設定（pre_cmd で templ generate）
```

> **アーキテクチャ方針（実務的 Layered-lite）**
> 厳格 Clean Architecture は不採用。依存は下向き一方向（`cmd → handler/middleware/auth → service → repository → infra`、`domain` は純粋層）で、隣接層スキップを許容します。
> DIP は **2点限定**：① DB ポート = sqlc 生成の `repository.Querier`（handler/auth は具象 `*Queries` ではなく interface に依存＝テストでモック可）、② service の consumer interface は必要時のみ。
> 所有者認可（BOLA 防止）は `internal/authz` に集約します。詳細の正は `.kiro/steering/structure.md`・`tech.md`。

---

## よく使うコマンド

`make help` で一覧が出ます。主なものは以下です。

```bash
# --- 開発サーバ / DB ---
make up            # PostgreSQL を起動（docker compose up -d）
make down          # PostgreSQL を停止・削除
make dev           # air でホットリロード開発サーバを起動

# --- ビルド / 実行 ---
make build         # templ generate → 本番用バイナリをビルド（./tmp/main）
make run           # ビルドして実行
make tidy          # go.mod / go.sum を整理

# --- マイグレーション（goose） ---
make migrate-up                         # 全マイグレーションを適用
make migrate-down                       # 1つ戻す
make migrate-status                     # 適用状況を表示
make migrate-create name=add_xxx        # 新規マイグレーション作成

# --- コード生成 ---
make sqlc          # db/queries/*.sql → internal/repository/*.go を再生成
make templ         # *.templ → *_templ.go を再生成

# --- DB スナップショット（AI / 開発者向けスキーマ資産） ---
make db-snapshot   # 実DBを内省し docs/database_snapshot/ にテーブル定義 + ER図を生成（要 make up + migrate-up）

# --- データ / トークン ---
make seed                                       # 開発用データ投入（既存データは削除）
make gen-token user=1 name="ハウスA温湿度計"    # デバイス API 用トークン発行

# --- モックプレビュー / クリーン ---
make mocks-preview # mocks/html/ をブラウザでプレビュー（http://localhost:8000）
make clean         # ビルド成果物と *_templ.go を削除
```

### テストの実行（`go test`）

このプロジェクトにテスト用の `make` ターゲットはありません。Go 標準の `go test` を直接使います。

```bash
go test ./...                       # 全パッケージのテストを実行
go test ./internal/handler/ -v      # 特定パッケージを詳細表示で
go test ./... -run TestSensorAPI    # テスト名で絞り込み
go test ./... -race                 # データ競合検出（推奨）
go test ./... -cover                # カバレッジ率を表示

# カバレッジ HTML レポート（coverage.* は .gitignore 済み）
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

> **目標カバレッジ 80% 以上**。`/tdd`（後述）はこの基準を強制します。

---

## Docker サービス構成

`docker-compose.yml` で定義しているサービスは **PostgreSQL の1つだけ**です。Nginx / PHP-FPM / phpMyAdmin のような Web・PHP 系コンテナはありません（アプリは Docker 外の Go バイナリとして動きます）。

| サービス | コンテナ名 | イメージ | 用途 |
|----------|------------|----------|------|
| db | `go_iot_db` | `postgres:16-alpine` | アプリケーション DB |

| 項目 | 値（デフォルト） |
|------|------------------|
| データベース名 | `go_iot` |
| ユーザー | `go_iot` |
| パスワード | `go_iot_dev` |
| ポート | `5432` |
| タイムゾーン | `Asia/Tokyo` |

> 接続情報を変える場合は `docker-compose.yml` の `environment` と `.env` の `DATABASE_URL` を**両方**合わせてください。

---

## アクセス先

`make dev` 起動中（または `make run`）にアクセスできます。

| エンドポイント | URL | 内容 |
|---|---|---|
| トップ | http://localhost:8080/ | ログイン状態に応じて `/dashboard` または `/login` へリダイレクト |
| ヘルスチェック | http://localhost:8080/health | DB 疎通込み（`{"status":"ok"}` / 失敗時 503） |
| **API ドキュメント** | http://localhost:8080/docs | **Scalar UI**（OpenAPI 3.0.3、`go:embed` 同梱） |
| OpenAPI 生 YAML | http://localhost:8080/docs/openapi.yaml | 機械可読な仕様 |
| デバイス API | `POST http://localhost:8080/api/sensor-data` | Bearer トークン認証（`make gen-token` で発行） |

### DBeaver / psql から接続する場合

`make up` でコンテナ起動中に、以下の接続設定を使います。

| 項目 | 値（デフォルト） |
|------|------------------|
| ホスト | `localhost` |
| ポート | `5432` |
| データベース | `go_iot` |
| ユーザー | `go_iot` |
| パスワード | `go_iot_dev` |
| ドライバー | **PostgreSQL** |

```bash
# psql で直接入る例
docker compose exec db psql -U go_iot -d go_iot
```

---

## AI へのコンテキスト供給（Laravel Boost に相当する仕組み）

Laravel プロジェクトでは AI に DB スキーマやルートを伝えるために **Laravel Boost（MCP サーバー）** を使いますが、**本プロジェクトでは MCP サーバーを使いません**。
コンテキスト消費の観点から、同等機能なら **Skill / プロジェクト内ドキュメントを第一候補**にする方針です。AI は以下を権威ある文脈として参照します。

| 供給源 | 役割 | Laravel Boost との対応 |
|---|---|---|
| **`.kiro/steering/`**（product / tech / structure） | プロジェクト全体のルール・技術スタック・依存方向・CSS方針 | プロジェクトメモリ |
| **`CLAUDE.md`** | Spec-Driven Development のワークフロー定義 | ガイドライン |
| **`docs/database_snapshot/`** | 自動生成の DB スナップショット（全テーブルのカラム・型・索引・CHECK・論理リレーション）。**実DBを見ずに把握できる** | DB スキーマ（Boost の db-schema 相当） |
| **`2cc_sdd/`** の設計書 | DB設計・画面設計・HTMX実装ガイド・実装計画・実装現状サマリ | DB スキーマ / ルート情報 |
| **`internal/repository/`**（sqlc 生成） + **`db/migrations/`** | 実際の DB スキーマとクエリ（型付き Go コード） | スキーマ自己記述 |
| **`/docs`（Scalar + OpenAPI）** | API エンドポイント仕様 | ルート一覧 |

> **DB スキーマを知りたい AI / 開発者へ**: まず **`docs/database_snapshot/table_definitions.md`**（テーブル定義）と **`er_diagram.mmd`**（Mermaid ER 図）を読んでください。`make db-snapshot` で実 DB（PostgreSQL）を内省して再生成される静的ファイルで、**DB に接続しなくてもカラム・型・デフォルト・索引・CHECK 制約・論理リレーションを把握できます**。マイグレーション（`db/migrations/`）を変更したら `make db-snapshot` で更新してください（生成ツールは `cmd/db-snapshot` + `internal/dbsnapshot`、整形ロジックはユニットテスト済み）。

> **再調査の前に読むべき1冊**: `2cc_sdd/実装現状サマリ.md`。
> 「今コードがどうなっているか」（あるべき姿ではなく現状スナップショット）を全サブシステム横断でまとめてあり、別セッション・別担当者がゼロから読み解く手間を削減します。**サーバ側（バックエンド + Web UI 層 S1〜S8）は実装完了**。残るのは **デバイス側ファームウェア（ESP8266 から実センサー値を読み自前 Go API へ送信）と現地実証**で、ロードマップは `2cc_sdd/実装計画.md` の「デバイス連携・現地実証フェーズ」を参照してください。

---

## AI 支援開発フロー（cc-sdd + /tdd）★このプロジェクトの中核

**仕様駆動開発（cc-sdd）** と **テスト駆動開発（/tdd）** を組み合わせます。
「**何を作るか**」を cc-sdd で固め、「**どう作るか**」を `/tdd`（tdd-guide エージェント）で品質保証しながら実装します。

### 役割分担

| フェーズ | ツール | 目的 |
|---|---|---|
| 要件定義 → 設計 → タスク分解 | **cc-sdd**（`/kiro-*`） | 「何を作るか」を明確にする |
| 実装 | **`/tdd`**（tdd-guide エージェント） | RED → GREEN → REFACTOR で品質保証しながら作る |

> **コマンド形式に注意**: 本プロジェクトの cc-sdd コマンドは **ハイフン区切り**（`/kiro-spec-init`）です。Laravel 時代の `/kiro:spec-init`（コロン区切り）ではありません。定義は `.claude/skills/kiro-*/SKILL.md` にあります。

### ステップ1: 仕様書を作成する（cc-sdd）

各セッション（= 1 spec = `.kiro/specs/{feature}/`）を以下の流れで進めます（`CLAUDE.md` の Minimal Workflow 準拠）。

| コマンド | 引数 | オプション | 説明 |
|----------|------|------------|------|
| `/kiro-spec-init` | `"何を作るかの説明"` | — | 機能仕様を初期化 |
| `/kiro-spec-requirements` | `{feature}` | — | 要件を EARS 形式で生成（人間がレビュー・承認） |
| `/kiro-validate-gap` | `{feature}` | — | 既存コードと要件のギャップ分析（**BE が既存のため推奨**） |
| `/kiro-spec-design` | `{feature}` | `[-y]` | 技術設計を作成。`-y` で自動承認 |
| `/kiro-validate-design` | `{feature}` | — | 設計レビュー（任意） |
| `/kiro-spec-tasks` | `{feature}` | `[-y]` | 実装タスクに分解（tasks.md 生成）。`-y` で自動承認 |
| `/kiro-spec-status` | `[feature]` | — | 進捗確認（いつでも） |

通常は各ステップで **人間が承認してから次へ**進みます。`-y` は意図的にショートカットするときだけ使います。
時短したい場合は `/kiro-spec-quick {feature} [--auto]` で init→requirements→design→tasks を一気通貫できます（重要セッションでは段階実行推奨）。

### ステップ2: タスクを TDD で実装する（/tdd）

`/kiro-spec-tasks` で生成された `tasks.md` の各タスクを、`/kiro-impl` ではなく **`/tdd`** で実装します（理由は後述）。

```
/tdd .kiro/specs/{feature}/tasks.md の「{タスク名}」を design.md を参照して実装
```

`/tdd` は以下のサイクルを強制します（Go 版）：

```
RED      失敗するテストを書く          → go test ./...（失敗を確認）
  ↓
GREEN    テストを通す最小限の実装       → go test ./...（合格を確認）
  ↓
REFACTOR コードを改善（重複除去・命名）  → go test ./...（緑のまま維持）
  ↓
カバレッジ確認（80% 以上）            → go test ./... -cover
```

### テストの配置場所（Laravel との最大の違い）

Laravel は `tests/Unit` / `tests/Feature` という**専用ディレクトリ**にテストを置きますが、**Go はテスト対象と同じパッケージに `*_test.go` を co-located（同居）させます**。`/tdd` もこの規約に従って生成します。

| 観点 | Laravel | **Go（このプロジェクト）** |
|---|---|---|
| 配置 | `tests/Unit`・`tests/Feature` に集約 | テスト対象と**同じディレクトリ**に `xxx_test.go` |
| 命名 | `XxxTest.php` | `xxx_test.go`（関数は `func TestXxx(t *testing.T)`） |
| 流儀 | PHPUnit | 標準 `testing` パッケージ + **テーブル駆動テスト** |
| 実行 | `php artisan test` | `go test ./...` |

既存テストが手本になります（規約はこれに合わせる）：

```
internal/auth/device_auth_test.go         # Bearer 認証
internal/authz/ownership_test.go          # 所有者認可（BOLA 防止）
internal/domain/comparison_operator_test.go
internal/domain/metric_test.go            # Enum・Evaluate()
internal/handler/sensor_api_test.go        # ハンドラ（Querier をモック）
internal/infra/pgconv/pgconv_test.go
internal/infra/token/token_test.go
```

> **モックは interface 差し替えで行います**。DB ポートは sqlc 生成の `repository.Querier`（interface）なので、`handler` / `auth` のテストでは最小モックを渡せます（`SensorAPI.Repo` / `DeviceAuthConfig.Repo`）。専用モックライブラリは不要です。

### 承認プロンプトを減らす（`permissions.allow` の設定）

`/tdd` 実行中、Claude Code は Bash コマンド（テスト実行・コード生成など）のたびに承認を求めます。
`~/.claude/settings.json` の `permissions.allow` に**Go 向けのパターン**を登録すると自動承認され、TDD サイクルを止めずに回せます。

```json
{
  "permissions": {
    "allow": [
      "Bash(go test*)",
      "Bash(go build*)",
      "Bash(go run*)",
      "Bash(go vet*)",
      "Bash(go tool*)",
      "Bash(go mod*)",
      "Bash(make *)",
      "Bash(docker compose up*)",
      "Bash(docker compose ps*)",
      "Bash(docker compose exec*)",
      "Bash(docker compose logs*)",
      "Bash(git status*)",
      "Bash(git log*)",
      "Bash(git diff*)",
      "Bash(git add*)",
      "Bash(git commit*)"
    ]
  }
}
```

> **VSCode 拡張の実行モード**: 入力欄の **Ask / Edit / Plan** いずれのモードでも、`permissions.allow` に登録した Bash コマンドは自動承認されます（Plan は読み取り専用）。
>
> **複数行コマンドは自動承認されない**: グロブ `*` は改行にマッチしません。`docker compose exec db psql ...` のような実行は**1行で**書いてください。

### なぜ `/kiro-impl` ではなく `/tdd` を使うのか

cc-sdd には実装フェーズ用の `/kiro-impl`（TDD 内包）もありますが、本プロジェクトでは **`/tdd`（tdd-guide エージェント）** を主に使います。理由：

- **80% 以上のカバレッジを強制**する独自エージェントを使える
- **テストファースト（RED → GREEN → REFACTOR）を厳格に強制**する
- `/code-review`（コードレビュー）との統合ワークフローが確立されている

> cc-sdd・`/tdd` は**干渉しません**。動作レイヤーが異なるためです。
> cc-sdd = 設計層（要件・仕様・タスク生成）、`/tdd` = 実装層（RED→GREEN→REFACTOR）。
> `/kiro-spec-design` 実行中に AI が `db/migrations` / `internal/repository`（sqlc）/ `/docs`（OpenAPI）を参照することで、現状と整合した設計が生成されます。

---

### 実装ロードマップ（cc-sdd 8セッション分割）

Web UI 層を **画面（または独立機能）単位の 8 セッション**に分割しています。各セッションの spec-init プロンプトは `2cc_sdd/spec-init-prompts/` に用意済みです。全体方針・依存関係は `2cc_sdd/実装計画.md` を参照してください。

| #  | feature-name | セッション名 | 前提 |
|----|--------------|-------------|------|
| S1 | `web-foundation-auth` | アプリ基盤＋認証（Walking Skeleton：session/レイアウト/login·register·logout） | なし |
| S2 | `alert-evaluation` | アラート判定ロジック（`POST /api/sensor-data` への判定接続。非UI） | なし（S1 と並行可） |
| S3 | `dashboard` | ダッシュボード（デバイス一覧カード・未対応アラート） | S1 |
| S4 | `device-create-edit` | デバイス登録・編集（共有フォーム） | S1 |
| S5 | `device-detail` | デバイス詳細（SVG グラフ・期間切替・最新計測・削除） | S1, S4 |
| S6 | `sensor-readings-history` | センサーデータ履歴（フィルタ・集計・ページング） | S1 |
| S7 | `alert-rules` | アラートルール管理（インライン CRUD） | S1 |
| S8 | `alert-history` | アラート履歴（フィルタ・ページング） | S1, S2 |

**実行例（S1 を段階実行する場合）:**

```bash
# 1. spec 初期化（spec-init-prompts/session-01-*.md の本文を貼り付け）
/kiro-spec-init "（session-01-web-foundation-auth.md の spec-init 本文）"

# 2〜5. 要件 → ギャップ確認 → 設計 → タスク分解
/kiro-spec-requirements web-foundation-auth
/kiro-validate-gap     web-foundation-auth     # BE 既存のため推奨
/kiro-spec-design      web-foundation-auth
/kiro-spec-tasks       web-foundation-auth

# 6. 実装は /tdd で（tasks.md のタスクを 1 つずつ）
/tdd .kiro/specs/web-foundation-auth/tasks.md の「scs セッション認証基盤」を design.md を参照して実装
```

> 各タスク完了後は `go test ./...` で緑を確認してからコミットします。

```bash
go test ./...
git add -A
git commit -m "feat: scsセッション認証基盤を実装（web-foundation-auth）"
```

全タスク完了後は `/kiro-spec-status web-foundation-auth` で完了状況を確認します。すべて `[x]` なら当該 spec は完了です。

---

### コピペ用プロンプト集（1セッションを通しで回す）

他プロジェクトの cc-sdd と同様に、各フェーズで実際に打つプロンプトをコピペ集としてまとめます。例は **S1 `web-foundation-auth`** ですが、`{feature}` を差し替えれば全セッション共通です。

> **このプロジェクト固有の3点（Laravel 版との違い）**
> 1. コマンドは **ハイフン区切り**（`/kiro-spec-init`）。`/kiro:spec-init`（コロン区切り）ではありません。
> 2. spec-init プロンプトは `2cc_sdd/spec-init-prompts/session-NN-*.md` に **8セッション分が作成済み**。計画済みセッションは「作成」不要で、`@` 参照するだけです（Laravel 版の `.kiro/prompts/wXXX_spec-init.md を作成` に相当する手順は、下の **B. 計画外の新規 spec** のときだけ必要）。
> 3. `/kiro-validate-gap` の成果物は **`research.md`** に書き込まれます（`gap-analysis.md` は作られません）。design フェーズの所見も同じ `research.md` に追記されるため、`/tdd` では `research.md` を参照します。

#### A. 計画済みの8セッション（spec-init プロンプトは既存）

```
✅[spec-init 実行時]
 /kiro-spec-init "S1 アプリ基盤＋認証。詳細は @2cc_sdd/spec-init-prompts/session-01-web-foundation-auth.md を参照"

✅[spec-requirements 実行時]
 /kiro-spec-requirements web-foundation-auth

✅[validate-gap 実行時（BE 既存のため推奨）]
 /kiro-validate-gap web-foundation-auth

✅[spec-design 実行時]
 /kiro-spec-design web-foundation-auth -y

✅[spec-tasks 実行時]
 /kiro-spec-tasks web-foundation-auth -y

✅[/tdd 実行時（tasks.md のタスクを1つずつ）]
 /tdd .kiro/specs/web-foundation-auth/tasks.md の「scs セッション認証基盤」を requirements.md・research.md・design.md を参照して実装
```

> `@2cc_sdd/spec-init-prompts/session-01-web-foundation-auth.md` を参照する代わりに、同ファイルの「--- spec-init 本文 ここから ---」以降を直接コピペして引数に渡してもかまいません。
> `-y` は design / tasks の人間レビューを飛ばす自動承認です。重要セッション（特に S1・S5）では `-y` を外して段階承認することを推奨します。
> requirements / research / design / tasks はいずれも `.kiro/specs/web-foundation-auth/` 配下に生成されます（`/tdd` の参照先も同フォルダ内）。

**全8セッションの `/kiro-spec-init` 行（コピペ用）:**

```
S1 /kiro-spec-init "S1 アプリ基盤＋認証。詳細は @2cc_sdd/spec-init-prompts/session-01-web-foundation-auth.md を参照"
S2 /kiro-spec-init "S2 アラート判定ロジック。詳細は @2cc_sdd/spec-init-prompts/session-02-alert-evaluation.md を参照"
S3 /kiro-spec-init "S3 ダッシュボード。詳細は @2cc_sdd/spec-init-prompts/session-03-dashboard.md を参照"
S4 /kiro-spec-init "S4 デバイス登録・編集。詳細は @2cc_sdd/spec-init-prompts/session-04-device-create-edit.md を参照"
S5 /kiro-spec-init "S5 デバイス詳細。詳細は @2cc_sdd/spec-init-prompts/session-05-device-detail.md を参照"
S6 /kiro-spec-init "S6 センサーデータ履歴。詳細は @2cc_sdd/spec-init-prompts/session-06-sensor-readings-history.md を参照"
S7 /kiro-spec-init "S7 アラートルール管理。詳細は @2cc_sdd/spec-init-prompts/session-07-alert-rules.md を参照"
S8 /kiro-spec-init "S8 アラート履歴。詳細は @2cc_sdd/spec-init-prompts/session-08-alert-history.md を参照"
```

各セッションの feature-name は `web-foundation-auth` / `alert-evaluation` / `dashboard` / `device-create-edit` / `device-detail` / `sensor-readings-history` / `alert-rules` / `alert-history` です（`/kiro-spec-requirements` 以降の引数に使います）。

#### B. 計画外の新規 spec（プロンプトを新規作成する場合）

8セッションに無い画面・機能を起こすときは、まず spec-init プロンプトファイルを作ってから回します（Laravel 版の `.kiro/prompts/wXXX_spec-init.md を作成` に相当）。

```
✅[spec-init プロンプト作成時]
 2cc_sdd/spec-init-prompts/session-09-{feature}.md を作成
  （who/current/change・スコープ・スコープ外・受け入れ基準・設計フェーズで参照する設計書の節番号を記載。
   既存の session-01〜08 が雛形になります）

✅[spec-init 実行時]
 /kiro-spec-init "{機能名}。詳細は @2cc_sdd/spec-init-prompts/session-09-{feature}.md を参照"

  …以降は A と同じ（requirements → validate-gap → design → tasks → /tdd）
```

---

### cc-sdd のタスク生成カスタマイズ（TDD 順序・逐次実行）

cc-sdd のデフォルトでは、タスク生成時に並列マーカー `(P)` が付き、実装タスクがテストタスクの前に並ぶことがあります。本プロジェクトでは **TDD（テストファースト）順序** と **逐次実行** をデフォルトに調整済みです（設定は `.claude/skills/kiro-spec-tasks/` 配下）。

| 項目 | cc-sdd 初期状態 | 本プロジェクト |
|------|-----------------|----------------|
| タスク実行順序 | 並列可（`(P)` 付与） | 逐次（上から順番） |
| テストと実装の順序 | 実装 → テスト | **テスト → 実装（TDD）** |
| 並列実行 | デフォルト有効 | `--parallel` フラグ指定時のみ |

> **並列にしたい場合**: `/kiro-spec-tasks {feature} --parallel` で従来どおり `(P)` マーカーが付きます（worktree 隔離による並行実装も利用可）。

---

## バージョン管理（gitignore / ローカル除外）

### `.gitignore` で管理しているもの（チーム共有）

`.gitignore` は **再生成可能な成果物**と**機密**を除外します。主なもの：

```
/tmp/  /bin/  *.exe  *.out      # ビルド成果物
*.test  coverage.out  coverage.html   # テスト・カバレッジ
*_templ.go  *_templ.txt          # templ 生成コード（make templ で再生成）
.env  .env.local  .env.*.local   # 環境変数（機密。絶対にコミットしない）
.idea/  .vscode/  .DS_Store      # IDE / OS
```

> **`*_templ.go` はコミットしません**。templ のソース（`*.templ`）のみをコミットし、生成物は `make templ`（または `make dev` / `make build` の `pre_cmd`）で都度生成します。
> **`.kiro/` `.claude/` `CLAUDE.md` はコミット対象**です（cc-sdd の仕様書・コマンド定義・プロジェクトメモリはチームで共有する資産のため）。流用元の Laravel 版ガイド（本 README の元になった旧ドキュメント）ではこれらをローカル除外していましたが、本プロジェクトでは方針が逆です。

### ローカル専用の除外（`.git/info/exclude`）

自分だけが使うエディタ設定など、チームの `.gitignore` に入れたくないファイルは `.git/info/exclude` に書きます（書き方は gitignore 形式。コミットされません）。

| | `.gitignore` | `.git/info/exclude` |
|---|---|---|
| コミットされるか | ✅ される（チーム共有） | ❌ されない（自分のローカルのみ） |
| 用途 | プロジェクト共通の除外 | 個人のエディタ・ツール設定 |

---

## 本番デプロイ（参考・将来）

運用も小さく保つ方針です。**PostgreSQL + Go バイナリ**を **AWS Lightsail + docker-compose** で動かす想定です（IoT の規模に対し Kubernetes は過剰として不採用）。

```bash
make build        # templ generate → ./tmp/main を生成
./tmp/main        # 環境変数（.env / 環境）を読んで起動
```

> `APP_ENV=production` のとき Gin は ReleaseMode で起動します。`SESSION_SECRET` / `DEVICE_TOKEN_SECRET` は本番で必ずランダムな長い値に差し替えてください。SSH/サーバ準備のメモは `other/backlog_ssh_setup.md` にあります。

---

## 困ったとき

### 特定のコミットまで戻す（作業が途中で止まった場合）

AI の制限時間などでフェーズ途中の作業が止まり、**直前のコミット状態に戻したい**ときの手順です。

```bash
# 1. 戻したいコミットのハッシュを確認
git log --oneline -10

# 2. HEAD をそのコミットに合わせ、未コミットの変更も破棄
git restore .
git clean -fd
git reset --hard <hash>

# 3. 確認（working tree clean かつ HEAD が目的のコミットなら OK）
git status
git log --oneline -3
```

> **注意**: `git reset --hard` は指定コミットより後の履歴と未コミット変更を捨てます。残したい変更は先に `git stash` か別ブランチへ退避してください。

### よくあるトラブル

| 症状 | 対処 |
|------|------|
| `make migrate-up` が接続エラー | `make up` で DB 起動済みか、`docker compose ps` が `healthy` か、`.env` の `DATABASE_URL` を確認 |
| `make dev` でビルドが落ちる | `*.templ` の構文エラーが多い。`make templ` 単体で生成を試す。`build-errors.log` を確認 |
| `go: command not found` | Go の PATH 未設定。`go version` を確認（Windows は §3 参照） |
| `*_templ.go` が見つからない型エラー | 生成漏れ。`make templ`（または `make dev`）を実行して再生成 |
| ポート 8080 が使用中 | `.env` の `APP_PORT` を変更するか、使用中プロセスを停止 |

---

> **このファイルについて**: 本ドキュメントは Go(Gin)+HTMX+API 環境向けのオンボーディング兼開発フローガイド（リポジトリ正式版 README）です。スタック・コマンド・ディレクトリ構成を変更したら、本 README も併せて更新してください。
