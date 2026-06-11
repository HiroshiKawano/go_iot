# Implementation Plan

> 実装は本ファイルを上から1行ずつ `/tdd`（RED→GREEN→REFACTOR）で進める（逐次・`(P)` なし）。各サブタスクは1つの観測可能な成果を持つ単一 TDD サイクル。
> 前提: S9（sqlite-migration）完了済み（`*sql.DB` 配線・WAL/busy_timeout・sqlite3store・SQLite 方言 migrations・全テスト green）。本機能は bootstrap/packaging 層の差分のみで、スキーマ・型層・既存業務ロジックは変更しない。
> 詳細な契約・意思決定は `design.md`、根拠は `research.md` を参照（Decision 1=appdata 単一解決源 / 2=sync-migrations 一方向同期 / 3=cli/browser / 4=hashicorp/mdns / 5=forward-slash file: URI / 6=将来 WithTx の _txlock=immediate）。

- [x] 1. アプリデータ基盤と config のゼロ設定起動（Foundation）
- [x] 1.1 アプリデータディレクトリ解決の単一解決源を実装
  - Windows は `%LOCALAPPDATA%\go_iot`（`LOCALAPPDATA` 環境変数。Roaming を返す `os.UserConfigDir()` は使わない）、非 Windows は `os.UserConfigDir()/go_iot`、いずれも取得不可なら実行ファイル隣の `go_iot-data`（CWD 相対にしない）に解決する
  - ディレクトリを冪等作成し（書込制限の `Program Files` 配下等は既定にしない）、配下ファイルパスを返す
  - 観測可能完了: `LOCALAPPDATA` を一時ディレクトリへ差し替えた table-driven テストで、解決パスが当該ディレクトリ配下を返し実際に作成される（非 Windows 経路・フォールバック経路も）
  - _Requirements: 2.2, 2.3, 2.5, 9.2_
  - _Boundary: internal/appdata_

- [x] 1.2 DB パス既定化（forward-slash file: URI）と必須ハードフェイル緩和
  - `DATABASE_URL` 未設定時にアプリデータ配下の `app.db` から **forward-slash 化した file: URI**（`file:/C:/Users/.../go_iot/app.db`。`filepath.ToSlash`＋ドライブ用先頭 `/`）を構築する。バックスラッシュ生パスの `file:` DSN は不可（DB が意図しない場所に黙って作られる＝Decision 5）。環境変数指定は最優先
  - 必須欠如による起動時ハードフェイルを廃止し、env 無しでも設定が成立する
  - 観測可能完了: env 全未設定で設定読込が成功し forward-slash file: URI を返す。Windows バックスラッシュ相当パスから構築した DSN が正しい絶対パスの DB を開く round-trip テストが green
  - _Requirements: 2.1, 2.2, 2.4, 2.5_
  - _Boundary: internal/config_
  - _Depends: 1.1_

- [x] 1.3 CSRF 認証鍵（SESSION_SECRET）の自動生成・永続化
  - 鍵は「環境変数 → 鍵ファイル（アプリデータ配下）→ 乱数 32 バイト生成して base64 text で `0o600` 保存」の順で確定する。環境変数指定は最優先。鍵ファイルが破損/空/長さ不足なら警告ログを出して再生成・上書きする（既発行 CSRF トークン無効化の旨をログ）。production の長さ検証（base64 表現で 32 文字以上）は維持
  - 観測可能完了: env 未設定で設定読込を 2 回行うと同一鍵を返す（永続化再利用）。環境変数指定時はその値を使う。破損鍵ファイルで再生成＋警告ログが出るテストが green
  - _Requirements: 3.1, 3.3_
  - _Boundary: internal/config_
  - _Depends: 1.1_

- [x] 1.4 ログ既定パス解決をアプリデータ単一解決源へ委譲
  - ログ出力先の既定パス解決を 1.1 の単一解決源へ委譲し、`%LOCALAPPDATA%\go_iot\` 解決の二重実装を排除する（GUI ビルドのログを `app.db` と同一ディレクトリの `app.log` に集約）。フォールバックは実行ファイル隣に統一
  - 観測可能完了: 既存ログ基盤のテストが委譲後も green（末尾 `app.log`・`LOG_FILE` 上書き・console ビルドは標準出力）、かつ `app.log` と `app.db` が同一ディレクトリへ解決される
  - _Requirements: 9.2, 9.3, 9.4_
  - _Boundary: internal/applog_
  - _Depends: 1.1_

- [x] 2. 起動時マイグレーション自動適用
- [x] 2.1 マイグレーション同梱の一方向同期インフラを整備
  - `db/migrations` を単一正本とし、`internal/migrate/migrations` へ一方向コピーする同期ターゲットを追加する（CSS の `sync-css` と同型）。ビルド／開発／テスト／Windows ビルドの各前段で同期を実行し、複製先を版管理対象外（gitignore）にする
  - 観測可能完了: 同期実行後に複製先へ既存 7 本の `*.sql` が複製され、`make test`／`make build` 系が同期後に走る（複製先は gitignore 済み）
  - _Requirements: 4.1_
  - _Boundary: Makefile, .gitignore_

- [x] 2.2 埋め込みマイグレーションを goose ライブラリで冪等適用
  - 複製したマイグレーションを go:embed で同梱し、goose ライブラリ（`SetBaseFS`／`SetDialect("sqlite3")`／`Up`）で適用する。goose を direct 依存へ整理する（`go mod tidy`）
  - 観測可能完了: 一時ファイル DB に適用を 2 回呼ぶと (1) 初回で全テーブル作成 (2) 2 回目は no-op で既存データ不変、(3) 未疎通 DB では適用がエラーを返すテストが green
  - _Requirements: 4.1, 4.2, 4.3_
  - _Boundary: internal/migrate_
  - _Depends: 2.1_

- [x] 2.3 起動シーケンスにマイグレーション自動適用を配線（fail fast）
  - DB オープン直後・listen 開始前にマイグレーションを適用する。失敗時はログに記録して起動を中断する（部分適用が残りうる旨は残存リスク）
  - 観測可能完了: env 無し起動相当でマイグレーション適用後に listener 取得まで到達する。マイグレーション失敗（未疎通 DB 相当）で起動シーケンスが中断する
  - _Requirements: 4.4, 2.2_
  - _Boundary: cmd/server/main.go_
  - _Depends: 2.2, 1.2_

- [x] 3. デスクトップ起動 UX（ポート自動採番・ブラウザ自動表示）
- [x] 3.1 ポート自動採番（listener 先取り）
  - 既定ポートを試み、使用中なら空きポートを自動取得して listen する。確定した listener と実ポートを呼び出し側へ返す
  - 観測可能完了: 既定ポートを別 listener で占有した状態で空きポートが採番され、返す実ポートが listener と一致するテストが green
  - _Requirements: 5.3, 5.4_
  - _Boundary: internal/desktop_

- [x] 3.2 既定ブラウザ自動オープン
  - 既定ブラウザで指定 URL を開く（組込ブラウザランタイム不要の通常ブラウザ）。子プロセスの標準出力/エラーを破棄して GUI ログ汚染を防ぐ。失敗は非致命（error を返すのみ）。ブラウザ起動ライブラリを direct 依存へ昇格（`go mod tidy`）
  - 観測可能完了: ブラウザ起動が失敗しても呼び出し側が継続できる（非致命）こと、通常ブラウザ起動であることをテスト／レビューで確認
  - _Requirements: 5.1, 5.2_
  - _Boundary: internal/desktop_

- [x] 3.3 起動シーケンスにポート確定・Serve・ブラウザ起動を配線
  - 固定 listen を「ポート自動採番 → 取得 listener で Serve」に置換し、listen 開始後に既定ブラウザで `localhost:<実ポート>` を開く。実ポートと DB パスを起動ログに明示する（ブラウザは同端末向け localhost。mDNS の `go-iot.local` は ESP32 向けで別系統）
  - 観測可能完了: 起動後に `localhost:<実ポート>` がブラウザに開かれ（テストではスタブ可）、実ポートが起動ログに出る。既定ポート競合時も採番ポートで継続起動する
  - _Requirements: 5.1, 5.3, 5.4_
  - _Boundary: cmd/server/main.go_
  - _Depends: 3.1, 3.2_

- [x] 4. mDNS ホスト名公開
- [x] 4.1 mDNS 公開（hostName A+SRV 告知・IP 変動再登録）を差替可能 interface で実装
  - 公開ライフサイクルを抽象化する最小 interface（開始/停止）を定義し、主候補ライブラリで安定ホスト名 `go-iot.local`（末尾ドット付き FQDN へ正規化）の A＋SRV を LAN へ告知する。告知 IP（非ループバック IPv4）と応答インターフェースを揃える。IP 変動を定期検知して停止→再登録する。LAN 限定でインターネットへ送出しない。pure-Go（cgo 不使用）。mDNS ライブラリを direct 依存に追加（`go mod tidy`）
  - 観測可能完了: (a) インターフェース列挙をスタブ化した「IP 変動検知→再登録」の状態遷移ロジックがユニットで green。(b) 環境変数ガード時のみ実マルチキャストで `go-iot.local` の A 応答をアサート（CI 非実行）
  - _Requirements: 7.1, 7.2, 7.3, 7.4_
  - _Boundary: internal/mdns_

- [x] 4.2 起動シーケンスに mDNS 開始/停止を配線（非致命）
  - listen 開始・実ポート確定後に mDNS 公開を開始し、graceful shutdown 時に停止する。開始失敗はログのみで継続（IP 直打ちで到達可能なため）
  - 観測可能完了: 起動配線で mDNS の開始/停止が呼ばれ（テストではスタブ可）、開始失敗時もサーバが継続起動する
  - _Requirements: 7.1_
  - _Boundary: cmd/server/main.go_
  - _Depends: 4.1, 3.3_

- [x] 5. 回帰・並行検証（既存挙動の確認）
- [x] 5.1 オフライン受信の回帰テスト
  - デバイス受信エンドポイントに有効 Bearer の送信で 201・保存・アラート同期評価、不正/未登録トークンで 401 を検証する。DB 境界をモックに差し替え、外部インターネットサービス非依存であることを確認する
  - 観測可能完了: `httptest` で 201/401 とアラート評価呼び出しをアサートするテストが green（インターネット非依存）
  - _Requirements: 6.1, 6.2, 6.3, 6.4_
  - _Boundary: 既存受信経路（テストのみ）_

- [x] 5.2 セッション再起動跨ぎの回帰テスト
  - 同一 DB ファイル（sessions テーブル）に対しセッション管理を作り直しても、既存 cookie トークンでユーザーが読み戻せること（再起動後のセッション維持は DB セッションストアが担い、CSRF 認証鍵の同一性とは独立）を検証する
  - 観測可能完了: 再起動相当（別セッション管理・別 context での読み出し）で同一ユーザーが読み戻るテストが green
  - _Requirements: 3.2_
  - _Boundary: 既存 auth / DB セッションストア（テストのみ）_

- [x] 5.3 並行アクセス負荷テスト
  - 複数 goroutine の連続受信（書込）× 読取 × 期限切れセッション削除（書込）を一時 SQLite DB に同時に与え、DB ビジー由来の 500 が出ないこと・待機リトライで完了することを検証する
  - 観測可能完了: 並行負荷で 500 応答が 0 件・全書込が完了するテストが green。結果に応じて writer 接続数の調整要否を記録する（保証は単一文 auto-commit 経路に限定。将来の read-then-write トランザクション導入時の `_txlock=immediate` は Decision 6 として別途）
  - _Requirements: 8.1, 8.2_
  - _Boundary: 既存 infra db（テストのみ）_

- [x] 6. docker / PostgreSQL 撤去と配布整備
- [x] 6.1 コンテナ構成と Makefile の docker 依存を撤去
  - コンテナ構成ファイルを削除し、Makefile の docker 起動/停止ターゲットを削除する。マイグレーション系ターゲットが SQLite 方言であることを確認し、ビルド・起動・テストに docker / PostgreSQL 依存が残らないことを確認する
  - 観測可能完了: コンテナ構成ファイルが存在せず docker 起動/停止ターゲットも無く、ビルド・起動経路に docker / PostgreSQL 依存が残らない
  - _Requirements: 10.1, 10.2_
  - _Boundary: Makefile, docker-compose.yml_

- [x] 6.2 Windows 単一 .exe クロスビルドへ Version 注入し検証
  - Windows ビルドターゲットに Version 注入を加え、`CGO_ENABLED=0` のクロスビルドで単一の `.exe` を生成する。コンソール窓なし（GUI）ビルドも生成できることを確認する
  - 観測可能完了: Windows ビルド（console / GUI 両方）が macOS/Linux 上で成功し単一 `dist/go_iot.exe` を生成、`import "C"` 由来のリンクエラーが無い（生成物が PE32+ の console/GUI であることを確認）
  - _Requirements: 1.1, 1.2, 1.3, 9.1_
  - _Boundary: Makefile_
  - _Depends: 2.1_

- [x] 6.3 配布文書を SQLite・デスクトップ起動手順へ更新
  - README と設定例ファイルから PostgreSQL/docker 前提の記述を削除し、env 任意・既定 DB パス・ダブルクリック起動手順へ更新する
  - 観測可能完了: README/設定例に PostgreSQL/docker 前提が無く、SQLite ベースのデスクトップ起動手順が記載される
  - _Requirements: 10.3_
  - _Boundary: README.md, .env.example_

- [x] 7. 最終統合検証
- [x] 7.1 全テスト・カバレッジ・単一 .exe ビルドの統合確認
  - 全テスト green、業務ロジックのカバレッジ 80% 以上、Windows 単一 `.exe` ビルド成功を統合確認する。別 Windows 端末コピー起動・実 ESP32 からの `go-iot.local` 到達・GUI 実機でのコンソール非表示は残存リスク（別フェーズ）として記録する
  - 観測可能完了: テスト・カバレッジ・Windows ビルドがすべて成功し、配布性・実機検証が残存リスクとして記録される
  - _Requirements: 1.4, 8.1, 10.2_
  - _Depends: 1.1, 2.3, 3.3, 4.2, 5.3, 6.1, 6.2_
