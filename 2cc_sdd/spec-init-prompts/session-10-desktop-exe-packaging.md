# セッション10 spec-init プロンプト: 単一 Windows .exe 化（デスクトップアプリ梱包）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: desktop-exe-packaging
> 実行例: /kiro-spec-init "（本文を貼り付け）"
> 前提セッション: **S9 sqlite-migration が完了していること（必須・厳密直列）**。DB が SQLite（modernc.org/sqlite + database/sql）で動作し、*sql.DB 配線・scs/sqlite3store・全テスト green が確立済みであることが本セッションの絶対前提。
> 設計フェーズで参照: `2cc_sdd/SQLite化・単一exe化_実現可能性調査.md`（観点D + §4.3 spec_breakdown ② + §4.4 sequence_note を必読）、`cmd/server/main.go`、`internal/view/static.go`、`internal/docs/docs.go`、`internal/config/config.go`、`Makefile`、`.gitignore`、`docker-compose.yml`、goose v3 ライブラリ API（SetBaseFS/SetDialect/Up）

--- spec-init 本文 ここから ---

## 機能概要

S9（sqlite-migration）で永続化層を SQLite に差し替えた本プロジェクトを、**Gin + HTMX + API + SQLite が1つの実行ファイルに同梱された単一 Windows `.exe`** として配布できるデスクトップアプリに仕立てる。
目的は「サーバを使わず、畑でノートPCと ESP32 に接続されたセンサーだけで完結する Windows デスクトップアプリ化」。利用者は `.exe` をダブルクリックするだけで、DB ファイルの自動作成・スキーマの自動適用・ブラウザでの UI 自動表示までが完了し、ESP32 は同一 LAN 上のノートPC へ Bearer 付き POST でセンサーデータを送れる。インターネット接続・docker・別途 DB サーバは一切不要とする。

本セッションは DB 層の移行（S9）の上に乗る **梱包（packaging）と起動 UX（bootstrap）の層**であり、SQL 方言書換や sqlc 再生成・型層改修は扱わない（それらは S9 完了済み前提）。

実装方針の権威ある根拠は `2cc_sdd/SQLite化・単一exe化_実現可能性調査.md` 観点D（判定 green）および §4.3 spec_breakdown ②。

## 背景・現状

**S9 完了時点で確立しているはずの前提:**
- DB は SQLite（modernc.org/sqlite, pure-Go, CGO 不要、driver 名 `sqlite`）+ database/sql + sqlc(engine=sqlite)。`*sql.DB` が `repository.New` / `auth.NewSessionManager` に配線済み。
- セッションは scs/sqlite3store。`db/migrations/*.sql` は SQLite 方言。
- 全テスト green。

**実機検証で確認済みの追い風（調査レポート 観点D）:**
- 現状の `cmd/server` は既に `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./cmd/server` で **37.4MB の .exe をクロスコンパイル成功**（`import "C"` ゼロ、cgo 必須依存なし）。modernc.org/sqlite も同条件でビルド成功。
- go:embed は既に3系統稼働: 静的 CSS（`internal/view/static.go:18 //go:embed all:public`）、docs（`internal/docs/docs.go:8-12`）、templ 生成物（*_templ.go 28ファイルが Go コードとしてコンパイル同梱）。→ **UI・静的資産は既に1バイナリに同梱済み**。残課題は db/migrations の embed と起動 UX のみ。
- 圃場オフライン構成は現状コードで成立: `cmd/server/main.go:58` が `:<APP_PORT>`（全 NIC）で listen、`/api/sensor-data` は `engine.Group("/api", deviceAuth)` 配下で CSRF 対象外・DeviceAuth は Bearer を SHA-256 照合し device_tokens を引くだけ（外部サービス非依存）。

**未着手の net-new 部分（本セッションのスコープ）:**
- 起動時マイグレーション自動適用（現状 goose は Makefile CLI のみ。Go コード内に embed/SetBaseFS/Up なし）。
- config の DATABASE_URL/SESSION_SECRET 必須ハードフェイル（`config.go:37-38`）。デスクトップではゼロ設定起動が必要。
- ブラウザ自動オープン・SQLite 並行アクセス設定・Windows ビルドターゲット。

## このセッションのスコープ（実装対象）

**1. 起動時マイグレーション自動適用（go:embed + goose ライブラリ）:**
- 新規パッケージ（例 `internal/migrate`）に `db/migrations/*.sql` を `//go:embed migrations/*.sql` で同梱（embed は親 `..` を辿れないため、migrations をパッケージ配下に配置/コピーする。`internal/docs` が openapi.yaml/index.html を同階層 embed している先例を踏襲）。
- 起動時（`cmd/server/main.go` の run 内、DB オープン直後）に `goose.SetBaseFS(embeddedFS)` + `goose.SetDialect("sqlite3")` + `goose.Up(sqlDB, "migrations")` を実行。goose/v3 v3.27.0 は既に依存にありライブラリ呼び出し可能。
- 初回起動: DB ファイルが無ければ modernc が自動作成 → goose.Up で全テーブル作成。2回目以降: 差分のみ適用（冪等）。
- マイグレーション失敗時はユーザーに分かるエラー表示（ログ + 起動中断）。

**2. config のデスクトップ向け緩和（ゼロ設定起動）:**
- `internal/config/config.go`: DATABASE_URL / SESSION_SECRET の必須ハードフェイルを緩和。
  - DB パス既定値: `%LOCALAPPDATA%\go_iot\app.db`（`os.UserConfigDir()` 等。Program Files 配下は UAC で書込不可リスクのため避ける）。env で上書き可。
  - SESSION_SECRET: 未設定なら初回起動時にランダム生成し永続化（保存先・平文可否は要確認）。
  - ディレクトリが無ければ作成。
- env / .env が無くても起動できること（圃場ノートPC でのダブルクリック起動を担保）。

**3. ブラウザ自動オープン（pure-Go 維持）:**
- サーバ listen 開始後に、既定ブラウザで `http://localhost:<PORT>` を自動オープン。
- Windows 実装: `exec.Command("rundll32", "url.dll,FileProtocolHandler", url)` または `cmd /c start`、もしくは `github.com/cli/browser` を direct 依存追加。
- **Wails / webview2 は採用しない**（WebView2 Runtime + CGO 依存で「CGO=0 pure-Go 単一 .exe」前提を壊す）。アドレスバーが見える没入感の低下は、配布・保守・圃場運用の堅牢性で勝るため許容。
- （任意・将来）systray 常駐でデスクトップ感を補う（cgo 無し選択肢あり。初版スコープに含めるかは要確認）。

**4. SQLite 並行アクセス設定（運用安定性）:**
- DSN に `_pragma=journal_mode(WAL)` + `_pragma=busy_timeout(5000)` を付与（または接続後 PRAGMA 実行）。
- `db.SetMaxOpenConns` を SQLite 単一 writer 前提で適正化（書込は実質1接続に絞る）。
- 競合対策の検証: ESP32 複数台の `POST /api/sensor-data` + アラート同期評価（書込）× Web UI 読取 × scs cleanup goroutine の定期 DELETE が SQLITE_BUSY を起こさないこと。

**5. Windows ビルドターゲット:**
- `Makefile` に `build-windows` 追加: `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X github.com/HiroshiKawano/go_iot/internal/view.Version=$(SHA)" -o dist/go_iot.exe ./cmd/server`（`-s -w` でサイズ削減、Version 注入は static.go の既存 ldflags 機構を流用）。
- コンソール窓を消す場合は `-ldflags "-H windowsgui"` を追加 → ただし `log.Printf` 出力先が失われるため、ログを `%LOCALAPPDATA%\go_iot\` 配下のファイルへリダイレクトする処理を併設（要確認）。
- `make build`（既存）は `sync-css` + `templ generate` を前段に持つ。build-windows も同様に templ/CSS 生成を前提にする。
- `.gitignore` は既に `*.exe` を無視済み（`dist/` も無視対象に追加）。

**6. docker / PostgreSQL 残滓の撤去:**
- `docker-compose.yml` 削除。`Makefile` の `up`/`down`（docker compose）ターゲット削除、`migrate-*` の goose dialect が sqlite3 になっていることを確認（S9 で済の想定。漏れがあれば本セッションで整理）。
- README / .env.example の PostgreSQL 前提記述を SQLite / デスクトップ起動手順へ更新。

## スコープ外（このセッションでやらないこと）

- **SQLite 移行本体**（SQL 方言書換・sqlc engine 切替・pgtype 型層改修・scs ストア差替・テスト書換）→ **S9 sqlite-migration** で完了済み前提。
- 新しい UI 画面・業務機能の追加。
- ESP32 ファームウェア側の実装（宛先 IP 設定・mDNS 対応はファーム/運用設計の範囲）。
- 自動更新（配布後アップデート）機構・コード署名・インストーラ作成（必要なら別セッション）。

## 技術制約・準拠事項

**準拠ドキュメント:**
1. `2cc_sdd/SQLite化・単一exe化_実現可能性調査.md` 観点D + §4.3 ② + §4.4 + §4.5 残存リスク — **最重要**
2. goose v3 ライブラリ API（`SetBaseFS` / `SetDialect("sqlite3")` / `Up`）
3. `.kiro/steering/tech.md`（ビルド/配布方針）

**実装言語・フレームワーク:**
- Go 1.26 + Gin v1.12 + templ v0.3 + HTMX + SQLite（modernc.org/sqlite, pure-Go）。
- ビルドは `CGO_ENABLED=0`（cgo 一切不使用）。クロスコンパイルのみで Windows .exe を生成（Windows 実機ビルド環境不要）。

**単一バイナリ不変条件:**
- 生成物は **追加ランタイム不要の単一 .exe**（WebView2・別 DB サーバ・docker いずれも不要）。
- 初回起動で DB ファイル自動作成 + マイグレーション自動適用 + ブラウザ自動表示まで、ユーザー操作はダブルクリックのみ。

**日本語コメント・ラベル:** すべて日本語。コード識別子は英語。

## 受け入れ基準（概略）

1. **クロスビルド**: `make build-windows` が macOS/Linux 上で成功し、`dist/go_iot.exe`（単一ファイル）が生成される。`import "C"` 由来のリンクエラーが無い。
2. **ゼロ設定起動**: env / .env / docker が無い状態で .exe を起動すると、`%LOCALAPPDATA%\go_iot\app.db` が自動作成され、マイグレーションが自動適用され、サーバが listen を開始する。
3. **ブラウザ自動表示**: 起動後に既定ブラウザで `http://localhost:<PORT>` が自動で開き、ログイン画面が表示される。
4. **マイグレーション冪等性**: 2回目以降の起動で既存 DB を壊さず差分のみ適用（または no-op）。
5. **オフライン疎通**: 同一 LAN の別端末（ESP32 相当）から `Authorization: Bearer <token>` 付きで `POST /api/sensor-data` が 201 を返し、データが SQLite に保存され、アラート同期評価が走る（インターネット遮断状態で成立）。
6. **並行安定性**: 連続 POST（複数デバイス相当）+ Web UI 操作 + scs cleanup を同時に与えても SQLITE_BUSY による 500 が出ない（WAL + busy_timeout + 接続数調整）。
7. **撤去確認**: docker-compose.yml が削除され、PostgreSQL/docker への依存がビルド・起動・テストのいずれにも残っていない。
8. **配布性**: 生成 .exe を別 Windows 端末にコピーしても、ランタイム追加なしで 1〜7 が成立する（実機 E2E は残存リスクとして明記の上、可能なら検証）。

## 未確定事項・要確認（あれば）

1. **コンソール窓の扱い**: `-H windowsgui` でコンソールを隠すか（その場合ログのファイルリダイレクトが必須）、コンソール表示のまま運用するか。**要オーナー確認**。
2. **SESSION_SECRET 自動生成の保存**: 初回生成した秘密鍵の保存先・平文ファイル可否（圃場単一ユーザー前提なら許容か）。**要オーナー確認**。
3. **DB ファイル配置**: `%LOCALAPPDATA%\go_iot\app.db` を既定とするか、exe 隣を選べるようにするか（可搬性 vs UAC 書込制約）。
4. **systray 常駐の要否**: 初版はブラウザ自動オープンのみで十分か、常駐トレイアイコン（終了操作・再オープン）まで含めるか。
5. **ポート競合**: 既定 PORT が使用中の場合の挙動（自動採番してブラウザにそのポートで開く等）。
6. **ESP32 宛先 IP**: ノートPC の LAN IP は DHCP で変動しうる。固定IP / mDNS（例: iot.local）/ アプリ内に自端末 IP を表示する補助 UI のいずれを現場手順とするか（本リポジトリ改修 or 運用設計の切り分け）。
7. **実機 E2E**: 実 Windows + 実 ESP32 での通し検証（配布→起動→自動マイグレーション→ブラウザ→Bearer POST 到達）は本セッションで実施するか、別途検証フェーズとするか。

--- spec-init 本文 ここまで ---
