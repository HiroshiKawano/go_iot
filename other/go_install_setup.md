# Go 1.26 インストール手順

作業日: 2026-04-20

---

## 概要

macOS に Go 1.26 を導入する手順を記録する。

本プロジェクト（go_iot）では Echo / templ / sqlc / goose といった周辺ツールが Go 1.21 以降を要求するため、最新安定版（Go 1.26）を採用する。

本書では **2種類のインストール方法** を解説する:

| 方法 | 所要時間 | 難易度 | 採用判断 |
|---|---|---|---|
| **A. Homebrew（brew upgrade go）** | 5〜10分 | ★ 最も楽 | **第1候補**。brew が動く環境ならこれ |
| **B. 公式 pkg インストーラ** | 2〜3分 | ★★ ダブルクリックのみ | brew が動かない場合のフォールバック |

### 本環境での採用結果

2026-04-20 時点の本環境（macOS 12.x Monterey / Intel Mac）では **方法A（Homebrew）で成功**。Go 1.20.6 から Go **1.26.2** へアップグレード完了。

> macOS Monterey は Homebrew の **Tier 3 サポート**扱い（公式ビルド済みバイナリ配布の対象外）のため、Go はソースからビルドされる。ビルドには 2〜3分かかるが、最終的に正常動作することを確認済み。

### なぜ方法Bも併記するか

macOS のバージョンがさらに古い／brew のセットアップ自体が壊れている等の理由で brew が使えない場合に備えて、公式 pkg による手順も記録しておく。

---

## 1. 前提環境の確認

### 1-1. macOS のバージョンとCPUアーキテクチャを確認

```bash
sw_vers
uname -m
```

**期待される出力例:**

```
ProductName:        macOS
ProductVersion:     12.x.x     ← macOS Monterey
BuildVersion:       21G...

x86_64                          ← Intel Mac（Apple Silicon の場合は arm64）
```

> `uname -m` の結果で、後にダウンロードするインストーラの種類が決まる。
> - `x86_64` → **darwin-amd64** 版
> - `arm64` → **darwin-arm64** 版（Apple Silicon: M1/M2/M3/M4）

### 1-2. 既存の Go インストール状況を確認

```bash
go version 2>/dev/null
which -a go
```

**本環境での確認結果（2026-04-20時点）:**

```
go version go1.20.6 darwin/amd64
/usr/local/bin/go
/usr/local/go/bin/go
```

→ Homebrew 経由で Go 1.20.6 が `/usr/local/Cellar/go/1.20.6/` にインストールされている。公式 pkg で 1.24 をインストールすると `/usr/local/go/` に展開され、PATH の優先順位に応じて切り替わる。

---

---

## 方法A: Homebrew でアップグレード（推奨・最も楽）

既に Homebrew がインストールされている環境では、以下の1コマンドで完了する。

### A-1. 現在のバージョン確認

```bash
go version
brew list --versions go
```

**本環境での出力例:**

```
go version go1.20.6 darwin/amd64
go 1.20.6
```

### A-2. アップグレード実行

```bash
brew upgrade go
```

**本環境での実行ログ（要約）:**

```
==> Auto-updating Homebrew...
Warning: You are using macOS 12.
We (and Apple) do not provide support for this old version.
This is a Tier 3 configuration:
  https://docs.brew.sh/Support-Tiers#tier-3

==> Upgrading 1 outdated package:
go 1.20.6 -> 1.26.2
==> Fetching downloads for: go
==> Upgrading go
  1.20.6 -> 1.26.2
==> ./make.bash
🍺  /usr/local/Cellar/go/1.26.2: 14,948 files, 233.4MB, built in 2 minutes 34 seconds
==> Running `brew cleanup go`...
Removing: /usr/local/Cellar/go/1.20.6... (11,997 files, 245.7MB)
```

> **所要時間:** 約5〜10分（ビルド2分34秒 + ダウンロード・依存解決時間）。
> macOS Monterey は Tier 3 サポートのため配布済みバイナリがなく、ソースから自動ビルドされる。

### A-3. インストール後の確認

```bash
go version
go env GOROOT GOPATH
```

**本環境での出力:**

```
go version go1.26.2 darwin/amd64
/usr/local/Cellar/go/1.26.2/libexec
/Users/c/go
```

> GOROOT が `/usr/local/Cellar/go/.../libexec` になるのが Homebrew 版の特徴。公式 pkg 版の `/usr/local/go` とはパスが異なるが、動作上の違いはない。

### A-4. Tier 3 警告について

アップグレード中に以下の警告が出るが、**無視してよい**:

```
Warning: You are using macOS 12.
We (and Apple) do not provide support for this old version.
This is a Tier 3 configuration
```

これは「macOS 12 向けの事前ビルド済みバイナリを Homebrew は配布しません。ソースからビルドします」という意味。**インストール自体は成功し、正常に動作する**。ただし、将来 Homebrew が macOS 12 のサポートを完全に打ち切った場合は方法B（公式 pkg）に切り替える必要がある。

### A-5. Homebrew がうまく動かない場合

以下のエラーで方法Aが失敗する場合は、**方法B（公式 pkg）に切り替える**:

- `Error: uninitialized constant` 系のRuby エラー
- `curl: (35) LibreSSL SSL_connect` 系のダウンロードエラー
- `clang: error` / `make: *** [all] Error` 系のビルドエラー
- `brew` コマンド自体が存在しない

方法Aで成功した場合は、**「6. 開発ツールの導入方針」** へ直接進む。「2. 公式 pkg ダウンロード」「3. インストール実行」「4. インストール後の確認」「5. 旧 Homebrew 版の処理」はスキップしてよい。

> 本プロジェクトでは `~/.zshrc` への PATH 追加は**不要**。開発ツール (air / templ / sqlc / goose) は `go.mod` の `tool` ディレクティブでプロジェクト内に閉じ込める方針のため。

---

## 方法B: 公式 pkg インストーラ（brew が動かない場合）

## 2. 公式 pkg インストーラのダウンロード

### 2-1. 公式ダウンロードページへアクセス

ブラウザで以下のURLを開く:

```
https://go.dev/dl/
```

### 2-2. 対応するインストーラを選択

ページ上部の「**Featured downloads**」または「**Stable versions**」セクションから、自分の環境に合うファイルを選ぶ。

| macOS の CPU | ダウンロードするファイル名 |
|---|---|
| **Intel Mac**（`uname -m` が `x86_64`） | `go1.24.X.darwin-amd64.pkg` |
| **Apple Silicon**（`uname -m` が `arm64`） | `go1.24.X.darwin-arm64.pkg` |

> `X` はマイナーバージョン（例: `go1.24.2.darwin-amd64.pkg`）。最新のパッチ版を選ぶ。
> 本環境は Intel Mac のため **`darwin-amd64.pkg`** を選ぶ。

### 2-3. ダウンロード場所

通常は `~/Downloads/` に保存される。

---

## 3. インストール実行

### 3-1. pkg ファイルをダブルクリック

Finder で `~/Downloads/go1.24.X.darwin-amd64.pkg` をダブルクリックする。

macOS のインストーラが起動し、以下の画面が順に表示される:

1. **「はじめに」** → 「続ける」をクリック
2. **「使用許諾契約」** → 「続ける」→「同意する」をクリック
3. **「インストール先」** → デフォルト（`/usr/local/go/`）のまま「続ける」
4. **「インストールの種類」** → 「インストール」をクリック
5. **管理者パスワードを要求される** → Mac のログインパスワードを入力
6. **「インストール完了」** → 「閉じる」をクリック

> pkg ファイルをゴミ箱に移動するかどうか聞かれる場合は「ゴミ箱に入れる」を選んでよい（インストール後は不要）。

### 3-2. インストール先の構造

pkg インストーラは以下の場所にファイルを配置する:

```
/usr/local/go/
├── bin/
│   ├── go           ← Goコマンド本体
│   └── gofmt        ← フォーマッタ
├── src/             ← Goの標準ライブラリソース
├── pkg/             ← コンパイル済みの標準ライブラリ
└── api/
```

シンボリックリンクも自動作成される:

```
/etc/paths.d/go      ← /usr/local/go/bin を PATH に追加する設定ファイル
```

---

## 4. インストール後の確認

### 4-1. ターミナルを再起動

すでに開いているターミナルでは PATH 設定が反映されていない可能性がある。**一度ターミナル（iTerm2 / Terminal.app）を完全に終了してから起動し直す**。

> `Cmd + Q` でターミナルアプリを終了し、改めて起動するのが確実。

### 4-2. バージョン確認

```bash
go version
```

**期待される出力:**

```
go version go1.24.X darwin/amd64
```

### 4-3. go env で環境変数を確認

```bash
go env GOROOT GOPATH
```

**期待される出力例:**

```
/usr/local/go                          ← GOROOT（Go本体の場所）
/Users/c/go                            ← GOPATH（モジュールキャッシュの配置先）
```

> **注意:** 本プロジェクトでは `$GOPATH/bin` を PATH に追加する必要は**ない**。
> プロジェクトで使う CLI ツール (air / templ / sqlc / goose) は `go.mod` の **`tool` ディレクティブ** でプロジェクトローカルに管理する。詳細は本書 **「6. 開発ツールの導入方針」** を参照。

---

## 5. 旧 Homebrew 版 Go の処理（任意）

Homebrew でインストールされた Go 1.20.6 は残したままでも、公式 pkg 版が優先されるため実害はない。ディスク容量を節約したい場合のみアンインストールする。

### 5-1. Homebrew 版のパスを確認

```bash
brew list go 2>/dev/null | head -3
```

### 5-2. Homebrew 版をアンインストール

```bash
brew uninstall go
```

> Homebrew 自体が動作しない場合はスキップしてよい。
> `/usr/local/Cellar/go/1.20.6/` ディレクトリを手動削除する方法もあるが、Homebrew の管理情報と不整合が生じるため推奨しない。

### 5-3. アンインストール後の確認

```bash
go version
```

公式 pkg 版の `/usr/local/go/bin/go` が使われ、引き続き `go version go1.24.X darwin/amd64` が表示されれば問題なし。

---

## 6. 開発ツールの導入方針（プロジェクトローカル / go.mod tool ディレクティブ）

本プロジェクト (go_iot) では、開発用 CLI ツール（air / templ / sqlc / goose）を **グローバルにインストールしない**。代わりに Go 1.24 で追加された **`tool` ディレクティブ** を使い、`go.mod` にバージョンを固定した上でプロジェクト内に閉じ込める。

### 6-1. なぜグローバル `go install` を使わないか

`go install github.com/air-verse/air@latest` のような従来の方法には以下の問題がある:

| 問題 | 詳細 |
|---|---|
| **グローバル汚染** | `~/go/bin/` にバイナリが配置され、他プロジェクトと共有されてしまう |
| **バージョン不整合** | 別プロジェクトで別バージョンの air が必要になると衝突する |
| **環境再現性の低下** | 新規メンバは `~/.zshrc` への PATH 追加＋個別 `go install` が必要 |
| **CI/CD の煩雑化** | CI上でもツール個別インストールが必要になる |

### 6-2. `tool` ディレクティブによる解決

`go.mod` の `tool` ブロックにツールを記載すると、そのプロジェクトディレクトリ内でのみ有効なツールとして登録される。実行は `go tool <ツール名>` で行う。

**本プロジェクトの `go.mod` (抜粋):**

```go
tool (
    github.com/a-h/templ/cmd/templ
    github.com/air-verse/air
    github.com/pressly/goose/v3/cmd/goose
    github.com/sqlc-dev/sqlc/cmd/sqlc
)
```

**ツールを追加する場合:**

```bash
go get -tool github.com/example/newtool@latest
```

→ 自動的に `go.mod` の `tool` ブロックへ追加され、`go.sum` にバージョンハッシュが記録される。

### 6-3. ツールの実行方法

直接コマンドを叩かず、すべて `go tool <ツール名>` 経由で呼び出す:

```bash
go tool air              # ホットリロード開発サーバ
go tool templ generate   # templ テンプレートをGoコードに変換
go tool sqlc generate    # SQL→Go リポジトリコード生成
go tool goose -dir db/migrations postgres "$DATABASE_URL" up  # DB マイグレーション適用
```

初回実行時にビルドが走り、以後はキャッシュから即時起動する。

### 6-4. Makefile 経由での起動（推奨）

毎回 `go tool xxx` と打つのは煩雑なため、Makefile に集約している:

```bash
make dev              # = go tool air
make templ            # = go tool templ generate
make sqlc             # = go tool sqlc generate
make migrate-up       # = go tool goose ... up
make migrate-create name=add_users    # 新規マイグレーション作成
```

一覧は `make help` で確認可能。

### 6-5. 新規メンバの初回セットアップ手順

リポジトリをクローンした直後にやること:

```bash
cd go_iot
make setup            # 依存ダウンロード + .env 作成
make up               # PostgreSQL コンテナ起動
make dev              # 開発サーバ起動 (air でホットリロード)
```

この3コマンドだけで開発環境が完成する。`go install` も `~/.zshrc` 編集も不要。

---

## 7. トラブルシューティング

### 問題1: `go version` を実行しても 1.20.6 のまま

**原因:** ターミナルの PATH キャッシュが古い、または `/usr/local/bin/go`（Homebrew 版シンボリックリンク）が先に解決されている。

**解決方法:**

```bash
# PATHの優先順位を確認
which -a go

# /usr/local/bin/go が /usr/local/go/bin/go より先に出る場合、
# .zshrc の先頭に以下を追記（/usr/local/go/bin を最優先にする）
export PATH="/usr/local/go/bin:$PATH"

source ~/.zshrc
go version
```

### 問題2: pkg インストーラが「開発元が未確認のため開けません」と表示される

**原因:** macOS の Gatekeeper が署名を検証できなかった。

**解決方法:**

1. `システム環境設定` → `セキュリティとプライバシー` → `一般` を開く
2. 下部に「"go1.24.X.darwin-amd64.pkg" は開発元を確認できないためブロックされました」というメッセージが表示されている
3. 「このまま開く」をクリック
4. 再度 pkg をダブルクリックして実行

> go.dev は Go 公式ドメイン（Google が管理）であり、ダウンロードした pkg 自体は Apple の Developer ID で署名されている。安全に開いてよい。

### 問題3: `go tool air` 実行時に `permission denied` エラー

**原因:** `$GOPATH`（`~/go/`）のパーミッションが正しくない、またはキャッシュディレクトリの書き込み権限不足。

**解決方法:**

```bash
# ディレクトリを手動作成
mkdir -p ~/go/pkg ~/go/cache

# 所有者を自分に変更
sudo chown -R $(whoami) ~/go

# プロジェクトに戻って再実行
cd /path/to/go_iot
go tool air
```

### 問題4: `go tool: unknown tool: air`

**原因:** `go.mod` に `tool` ディレクティブが登録されていない、または `go mod download` が実行されていない。

**解決方法:**

```bash
# プロジェクトルートで実行
cat go.mod | grep -A 5 "^tool"
# ↑ 4ツールが表示されれば OK

# tool ディレクティブが空なら以下で登録
go get -tool github.com/air-verse/air@latest
go get -tool github.com/a-h/templ/cmd/templ@latest
go get -tool github.com/sqlc-dev/sqlc/cmd/sqlc@latest
go get -tool github.com/pressly/goose/v3/cmd/goose@latest

# 依存をダウンロード
go mod download
```

### 問題5: Go 1.24 未満で `tool` ディレクティブが認識されない

**原因:** `go` ディレクティブが 1.24 未満になっている。`tool` ディレクティブは Go 1.24 以降で導入された機能。

**解決方法:** `go.mod` 冒頭の Go バージョンを確認する。

```bash
head -3 go.mod
# → go 1.24 以上になっていること

# 古い場合は Go 本体を最新にアップグレード（本書の方法A または 方法B）し、
# go.mod も更新:
go mod edit -go=1.24
go mod tidy
```

---

## 初心者向け用語説明

### pkg インストーラとは

macOS の標準インストーラ形式。ダブルクリックするとグラフィカルな画面で段階的にインストールが進む。Windows の `.msi` / `.exe` インストーラに相当する。

pkg ファイルは内部的に以下を含む:
- インストールするファイル一式
- 配置先ディレクトリの指定
- インストール前後のスクリプト
- Apple の Developer ID による署名（改ざん検知用）

### GOROOT / GOPATH / GOBIN の違い

| 環境変数 | 意味 | 典型的な値 |
|---|---|---|
| **GOROOT** | Go 本体（コンパイラ・標準ライブラリ）の場所 | `/usr/local/go` |
| **GOPATH** | 自分のコード・依存モジュール・ビルド成果物の置き場所 | `~/go` |
| **GOBIN** | `go install` で生成された実行ファイルの配置先 | 空の場合 `$GOPATH/bin` が使われる |

**通常ユーザーが触るのは GOPATH 配下（特に `$GOPATH/bin`）のみ。**
GOROOT は Go 本体のため触る必要はない。

### なぜ `$GOPATH/bin` を PATH に追加するのか

`go install <パッケージ>@latest` コマンドは、指定されたパッケージをビルドし、生成された実行ファイルを `$GOPATH/bin`（`~/go/bin`）に配置する。

このディレクトリが PATH に含まれていないと、インストールしたツール（air / templ / sqlc / goose 等）をコマンドラインから直接呼び出せない。

```bash
# PATH に $GOPATH/bin が含まれていない場合
$ air
command not found: air

# 絶対パスなら動く（が、不便）
$ ~/go/bin/air
```

PATH に追加することで、ターミナルのどこからでも `air` / `templ` / `sqlc` / `goose` が実行できるようになる。

### darwin-amd64 と darwin-arm64 の違い

- **darwin**: Apple 系 OS（macOS / iOS）のカーネル名
- **amd64**: Intel 64bit CPU（x86_64）。Intel Mac 向け
- **arm64**: ARM 64bit CPU。Apple Silicon（M1 / M2 / M3 / M4）向け

誤った方をインストールすると `bad CPU type in executable` のようなエラーで実行できない。`uname -m` で自分の CPU を確認してから選ぶ。

### .zshrc とは

zsh シェル（macOS Catalina 以降のデフォルトシェル）の起動時に読み込まれる設定ファイル。
ホームディレクトリ直下の隠しファイル `~/.zshrc`。

環境変数（PATH / GOPATH 等）や alias、関数をここに書いておくと、ターミナル起動のたびに自動的に反映される。

```bash
# .zshrc を編集する
nano ~/.zshrc
# または
code ~/.zshrc          # VSCode で開く

# 編集後の反映（ターミナル再起動なしで即時反映）
source ~/.zshrc
```

---

## 参考リンク

| リンク | 内容 |
|---|---|
| https://go.dev/dl/ | Go 公式ダウンロードページ |
| https://go.dev/doc/install | Go 公式インストールガイド |
| https://go.dev/ref/mod | Go Modules リファレンス |

---

更新日時: 2026-04-20
