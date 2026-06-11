# ギャップ分析: desktop-exe-packaging

> 実施日: 2026-06-08 / 対象: requirements.md 全10要件
> 手法: 現状コードベース直接検証（config.go / main.go / pool.go / Makefile / migrations / go.mod を実読）＋ 権威ドキュメント `2cc_sdd/SQLite化・単一exe化_実現可能性調査.md`（§3.D / §4.3② / §4.4 / §4.5）照合 ＋ pure-Go mDNS ライブラリの Web 調査。

## 1. 調査サマリ（要点）

- **S9（sqlite-migration）は完了済み**。`internal/infra/db/pool.go` は既に `sql.Open("sqlite", …)`（modernc・CGO不要）で、DSN に `journal_mode(WAL) / busy_timeout(5000) / foreign_keys(1)` を付与し `SetMaxOpenConns(4)` 済み。`session_auth.go` は `sqlite3store`、`db/migrations/*.sql` は SQLite 方言、`migrate-*` は goose `sqlite3` dialect。本機能 S10 は **bootstrap/packaging 層の差分のみ**で、型層・SQL 方言・クエリ生成は対象外（権威doc §4.4 の直列前提どおり）。
- **net-new は実質 3 点**: (R4) 起動時マイグレーション自動適用（go:embed + goose ライブラリ）、(R5) ブラウザ自動オープン＋ポート自動採番、(R7) **mDNS 公開**（requirements で当初スコープ外から in-scope へ変更された唯一の新規外部依存）。
- **先行実装で既に満たす要件**: R1（`build-windows` / `build-windows-gui` ターゲット・CGO=0 クロスビルド成功・`*.exe` は .gitignore 済）、R8（WAL+busy_timeout+接続数調整は実装済、残るは負荷検証）、R9（`internal/applog` + GUI ターゲット完成、残るは実 Windows 実機確認＝残存リスク）、R6（`/api/sensor-data` + DeviceAuth Bearer・全NIC listen・外部非依存は成立、残るはオフライン結合検証）。
- **緩和（config の必須フェイル）と撤去（docker 残滓）が確実なギャップ**: R2/R3（`config.Load` が DATABASE_URL/SESSION_SECRET を今も必須ハードフェイル）、R10（`docker-compose.yml` 残存・Makefile `up`/`down` が docker compose）。
- **最大リスクは R7（mDNS）**: 外部依存追加・Windows 挙動・ESP32 の `.local` 解決・DHCP 変動時の再告知ライフサイクルが未知数。pure-Go 実装は複数存在（後述）するが、実機確認まで Unknown が残る。

## 2. 要件 ↔ 資産マップ（ギャップタグ: DONE / Missing / Constraint / Unknown）

| 要件 | 既存資産（file:line） | ギャップ | タグ |
|---|---|---|---|
| **R1** クロスビルド/配布 | `Makefile:37-43`（build-windows / build-windows-gui, CGO=0）, `.gitignore:4`（`*.exe`） | Version 注入（`-X internal/view.Version`）が build-windows 未付与（任意）。単一ファイル/別端末コピー起動は実機確認＝残存リスク | **DONE**（残: 任意のVersion注入） |
| **R2** ゼロ設定起動/DB配置 | `config.go:26-39`（DATABASE_URL 必須ハードフェイル）, `pool.go:20`（file DSN を開く）, `applog.DefaultPath`（Windows=`%LOCALAPPDATA%\go_iot\app.log` の解決先例） | DATABASE_URL 未設定時の既定 `%LOCALAPPDATA%\go_iot\app.db` 解決・ディレクトリ自動作成・env 上書きが無い | **Constraint**（必須フェイル緩和） |
| **R3** セッション鍵 自動生成/永続化 | `config.go:27,34-43`（SESSION_SECRET 必須・本番32字検証） | 未設定時のランダム生成＋ローカル平文ファイル永続化＋再起動再利用が無い | **Missing** |
| **R4** 起動時マイグレーション自動適用 | `db/migrations/*.sql`（SQLite方言・goose注釈・7本）, `go.mod`（goose v3.27.0）, `docs.go`（同階層 go:embed 先例） | `cmd/server/main.go` に goose 呼出無し。embed 用に migrations をパッケージ配下へ配置する net-new パッケージ（例 `internal/migrate`）が必要 | **Missing**（net-new） |
| **R5** ブラウザ自動表示/ポート自動採番 | `main.go:74-86`（固定 `:APP_PORT` listen + ListenAndServe）, `go.mod`（`github.com/cli/browser v1.3.0` が **indirect で既存**） | listen 後のブラウザ自動オープン無し。ポート競合時の空きポート自動採番（`net.Listen` 先取り→実ポート取得→serve）無し | **Missing** |
| **R6** オフライン/LAN 受信 | `main.go:143-145`（`/api/sensor-data` + deviceAuth・CSRF対象外）, `device_auth.go`（Bearer SHA-256照合・外部非依存）, 全NIC `:port` listen | 機能は成立。インターネット遮断＋LAN別端末からの 201・保存・アラート同期評価の **結合検証**のみ | **DONE**（残: 検証） |
| **R7** mDNS 安定ホスト名公開 | なし（mDNS/zeroconf 依存ゼロ） | 公開ホスト名（例 `go-iot.local`）の mDNS 告知・現在LAN IP 応答・DHCP 変動追従・pure-Go 維持が全て net-new | **Missing + Unknown**（最大リスク） |
| **R8** 並行アクセス安定性 | `pool.go:30,52-57`（WAL+busy_timeout(5000)+SetMaxOpenConns(4)）, `session_auth.go`（scs cleanup goroutine） | 設定は実装済。連続POST×Web UI×cleanup 同時で 500 が出ないことの**負荷検証**と、必要なら writer 接続数の最終調整 | **DONE**（残: 検証・微調整） |
| **R9** GUIコンソール窓非表示/ログ出力 | `applog.go`（Setup/Destination/DefaultPath, lumberjack, cov88.2%）, `main.go:39-46`（差替済）, `Makefile:41-43`（`-H windowsgui` + `applog.Mode=file`） | コードは完成。実 Windows でコンソール非表示・`app.log` 書出の**実機確認**＝残存リスク（別フェーズ） | **DONE（コード）**（残: 実機検証） |
| **R10** docker/PostgreSQL 残滓撤去 | `docker-compose.yml`（残存・470B）, `Makefile:19-23`（up/down=docker compose, L65「S10へ据置き」明記）, README/.env.example | docker-compose.yml 削除・up/down ターゲット削除・配布文書の PostgreSQL→SQLite 起動手順更新 | **Missing** |

> 補足: `.gitignore` の `*.exe` が `dist/go_iot.exe` を既にカバーするため `dist/` 追記は任意（必須でない）。`migrate-up` は goose CLI（`sqlite3` dialect）で正しいが、DATABASE_URL 依存のため R2 の既定パス解決とは別系統である点に留意。

## 3. 実装アプローチ（A / B / C）

### Option A: 既存ファイル拡張（main.go / config.go へ直接）
`run()` 内にマイグレーション・ポート採番・ブラウザオープン・mDNS 起動をインライン追加し、config.go に既定パス/鍵生成を足す。
- ✅ 新規ファイル最小・最速の初期実装。既存配線をそのまま活かす。
- ❌ `run()` が肥大化し packaging の関心が main に集中。mDNS/migrate の単体テストがしづらい。プロジェクト規約（小さなファイル・高凝集）と逆行。

### Option B: 機能別に新規パッケージを切る
`internal/migrate`（embed+goose）, `internal/mdns`（告知）, `internal/desktop`（ブラウザオープン+ポート採番）を新設し、config を拡張、`cmd/server/main.go`（合成ルート）が各々を呼ぶ。
- ✅ 関心分離・単体テスト容易（structure.md の「cmd=合成ルート / 下位層を配線」と整合、ユーザーの小ファイル志向に合致）。
- ✅ R7 の不確実性を `internal/mdns` に隔離でき、失敗時の切離し・差替が容易。
- ❌ ファイル数増。各パッケージの最小 interface 設計が要る。

### Option C: ハイブリッド（推奨）＋ 2 フェーズ
net-new で凝集する単位だけ新規パッケージ化し、小さな起動補助は config / 薄い起動ヘルパへ寄せる。さらに「ダブルクリック起動の核」を先に通す。
- **新規パッケージ**: `internal/migrate`（go:embed + goose.SetBaseFS/SetDialect("sqlite3")/Up）、`internal/mdns`（pure-Go zeroconf ラッパ・起動/停止・IP変動再告知）。
- **config 拡張 + 薄い起動ヘルパ**: 既定 DB パス解決（applog の `%LOCALAPPDATA%` 解決と共通化）、SESSION_SECRET 自動生成・永続化、ポート自動採番（`net.Listen`）＋ブラウザオープン（既存 `cli/browser` を direct 昇格）。
- **フェーズ分割**:
  - **P1（核）**: R2/R3 config 緩和 → R4 自動マイグレーション → R5 ブラウザ＋ポート採番。これだけで「env 無し .exe ダブルクリック→DB自動作成→スキーマ適用→ブラウザ表示」が成立（受け入れ基準2・3・4の中核）。
  - **P2（圃場運用＋仕上げ）**: R7 mDNS → R8/R6 並行・オフライン検証 → R10 docker 撤去・文書更新 →（R1 Version 注入は任意でここに統合）。
- ✅ 規約整合・リスク隔離・段階検証。P1 完了時点で動くデモが出せる。
- ❌ フェーズ間の調整計画がやや必要（とはいえ直列で単純）。

## 4. 工数（S/M/L/XL）・リスク（High/Med/Low）

| 要件 | 工数 | リスク | 一言根拠 |
|---|---|---|---|
| R1 | S | Low | ターゲット完成済。Version 注入は ldflags 1 行追加 |
| R2 | S | Low–Med | config 緩和＋パス解決。applog の既存 `%LOCALAPPDATA%` 解決を再利用すれば確実 |
| R3 | S–M | Med | 鍵生成は容易だが、保存先・ファイル権限・平文許容（オーナー決定済）と main 配線の整合に注意 |
| R4 | M | Med | net-new だが goose ライブラリ API は定石。migrations の embed 配置（`..` 不可）と冪等/失敗中断の設計が要 |
| R5 | M | Med | ポート採番（`net.Listen`→実ポート→serve）とブラウザオープンの順序・クロス挙動。`cli/browser` 既存で依存追加ほぼ無し |
| R6 | S | Low | 機能成立済。オフライン結合テストの追加のみ |
| R7 | M–L | **High** | 外部依存追加・Windows mDNS 挙動・ESP32 `.local` 解決・DHCP 再告知が未知。実機確認まで Unknown |
| R8 | S | Low–Med | 設定済。負荷検証で 500 不発を確認、必要なら writer 接続数を最終調整 |
| R9 | S | Low | コード完成。実 Windows 確認は別フェーズ（残存リスク） |
| R10 | S | Low | ファイル削除＋Makefile/文書編集 |

**総合: 工数 M（核は小粒の集合、実新規は R4/R5/R7）／リスク Med（R7 が押し上げ要因）。**

## 5. design フェーズへの申し送り

### 推奨アプローチ
**Option C（ハイブリッド・2フェーズ）。** `internal/migrate` と `internal/mdns` を新規パッケージ化、config 拡張＋薄い起動ヘルパで R2/R3/R5 を吸収。P1（核）→P2（運用/仕上げ）の直列。

### 設計時に決める主要事項（Boundary Commitments 候補）
1. **DB 既定パス解決の共通化**: Windows の `%LOCALAPPDATA%` は `os.UserConfigDir()`（=Roaming `%APPDATA%`）では取れない。`applog.DefaultPath` が既に `%LOCALAPPDATA%\go_iot\` を解決しているため、**DB パスもこの解決ロジックを共通ヘルパ化して再利用**する（`app.db` と `app.log` を同一ディレクトリへ）。env 上書きキー名（`DATABASE_URL` 流用 or 新規 `DB_PATH`）も確定する。
2. **migrations の embed 配置方式**: go:embed は `..` 不可。(a) 正本を `internal/migrate/migrations/` へ移設し `Makefile migrate-* -dir` を追従、(b) 正本は `db/migrations/` のまま `make`/`go:generate` で `internal/migrate/migrations/` へ複製（sync-css と同型の一方向同期）。**単一ソース原則（CSS 運用に倣う）から (b) を軸に検討**、ただし二重管理回避のため (a) も比較。
3. **マイグレーション失敗時の UX**: GUI ビルドではコンソールが無いため、失敗は `app.log` 記録＋（可能なら）ユーザーに見える形での中断（終了コード/簡易ダイアログの要否）。最低限ログ＋起動中断（R4-AC4）。
4. **ポート自動採番の伝達**: `net.Listen("tcp", ":<port>")` が EADDRINUSE なら `:0` で空きポート取得→実ポートを `http.Server` に渡し、その実ポートでブラウザを開く＋ログ/UI へ表示（R5-AC3/4）。
5. **mDNS ライブラリ選定**（最重要・後述 Research Needed）。
6. **SESSION_SECRET 保存形式**: ローカル平文ファイル（オーナー決定済）。保存パス（DB と同ディレクトリ）・ファイル権限・env 指定時は生成スキップを確定。
7. **R8 writer 接続数**: 現状 `SetMaxOpenConns(4)`。WAL 下で読取並行・書込直列の前提と負荷検証結果で 4 維持か 1 へ絞るかを design の Testing Strategy（負荷シナリオ）で裏取り。

### Research Needed（design/PoC で確定）
- **R7 mDNS ライブラリ**: pure-Go 候補は `github.com/grandcat/zeroconf`（RFC6762/6763・Register でサービス告知・Avahi 互換確認/Bonjour 未確認）、派生の `betamos/zeroconf`（2023 リライト）, `hashicorp/mdns`。確認事項: (1) `CGO_ENABLED=0 GOOS=windows` でビルド可、(2) ホスト名 `<name>.local` の A レコード応答を ESP32 mDNS クライアントが解決できるか、(3) DHCP で IP 変動した際の再告知（インターフェース変化検知/定期再Register）方法、(4) Windows のファイアウォール/UDP5353・既存 mDNS レスポンダ（Bonjour）との共存。→ design で 1 候補に絞り、実機 PoC は別フェーズの残存リスクとして明記。
- **R5 ブラウザオープン**: `github.com/cli/browser`（既存 indirect）を direct 昇格して採用するか、`rundll32 url.dll,FileProtocolHandler` / `cmd /c start` を使うか。pure-Go 維持は両者とも可。
- **R4 goose ライブラリ API**: v3.27.0 の `SetBaseFS(embed.FS)` + `SetDialect("sqlite3")` + `Up(*sql.DB, "migrations")` の正確なシグネチャと、embed ルート（`migrations` サブディレクトリ）指定の整合を PoC で 1 度確認。
- **R8 並行**: 負荷スクリプト（複数 goroutine の連続 POST × GET × scs cleanup 強制）で SQLITE_BUSY 由来 500 が出ないことを measure。

## 6. 既存実装への影響・隣接リスク（スコープ外だが design で留意）

- **steering が陳腐化**: `.kiro/steering/tech.md` / `structure.md` は今も **PostgreSQL/pgx/pgxpool/docker-compose 前提**の記述（S9 反映漏れ）。design フェーズは steering を読むため、誤った DB 前提を引かないよう **doc-updater で SQLite 反映が望ましい**（R10 は README/.env.example のみ対象。steering 更新は本機能スコープ外の別タスク提案）。
- **db-snapshot 陳腐化リスク**: `Makefile db-snapshot` は「要 make up + migrate-up」（docker 前提）の記述。権威doc §4.5-6 のとおり、`internal/dbsnapshot` の pg_catalog 内省が SQLite 未移植なら `docs/database_snapshot/` が陳腐化し、以後の cc-sdd 設計の権威スキーマ参照が劣化する。本機能の直接スコープ外だが docker 撤去（R10）と併せて状態確認を推奨。
- **マージ衝突**: 本機能は config.go / main.go(run) / Makefile / go.mod を触る。並行する別作業がこれらに触れる場合は P1 を一気に通すのが安全（権威doc §4.4 と同じ注意）。

---

# 設計フェーズ discovery・意思決定ログ（2026-06-08）

## Summary
- **Feature**: `desktop-exe-packaging`
- **Discovery Scope**: Extension（S9 完了済み SQLite アプリへの bootstrap/packaging 層追加）
- **Key Findings**:
  - 3 ライブラリの正確 API を裏取り（goose v3.27 global API 非 deprecated / mDNS は hostName A 応答が要点 / cli/browser は既存 indirect）。
  - **Windows の `file:` DSN はバックスラッシュ生パスだと SQLite URI パーサが誤解し DB が意図しない場所に黙って作られる**（PoC 実証・critical）。forward-slash URI 必須。
  - **`SESSION_SECRET` は実体が CSRF 認証鍵**で、scs セッション（再起動維持）とは無関係（DB セッションストア＋不透明 cookie トークンが担う）。

## Research Log

### 外部ライブラリ API 裏取り（goose / mDNS / cli-browser）
- **Context**: design の Technology Stack 確定のため。
- **Sources Consulted**: pkg.go.dev・GitHub 実ソース（goose dialect.go / hashicorp mdns zone.go,server.go / cli browser.go）、modernc conn.go、SQLite 公式 uri.html。
- **Findings**:
  - goose v3.27: `SetBaseFS(fs.FS)` / `SetDialect("sqlite3")`（"sqlite" も可）/ `Up(*sql.DB, "migrations")`。global API は非 deprecated（単一 DB で十分。Provider API は複数 DB 向け）。各マイグレーションファイルを単一 Tx で実行し、失敗時はファイル単位ロールバック（先行 commit は巻き戻らない＝部分適用あり）。`StatementBegin/End` はパーサ注釈でロールバック境界ではない。
  - mDNS: `hashicorp/mdns` は `MDNSService` の HostName（末尾ドット付き FQDN `go-iot.local.`）が A 応答に直結し、本要件（任意ホスト名 A 応答）に最適。`grandcat/zeroconf` は `RegisterProxy` の issue #74 で素の A 解決に hostname-prefix 回避策が要る。いずれも IP 変動を自動追従しない（再登録要）。`Config.Iface` nil は既定 multicast IF のみ使用→告知 IP と応答 IF の整合が要る。RFC6762 §8/§9 プローブ/衝突解決は hashicorp/mdns 未実装。
  - cli/browser v1.3.0: `OpenURL(string) error`、`Stdout/Stderr` を `io.Discard` 可。pure-Go（OS 別 exec）。既存 indirect → direct 昇格。
- **Implications**: mDNS を `Advertiser` interface で隔離し差替可能に。FQDN 正規化・マルチ NIC・衝突・再登録を実装ノート/残存リスクへ。

### Windows `file:` DSN のバックスラッシュ問題（PoC 実証）
- **Context**: 既定 DB パスを `appdata.Path("app.db")`（Windows で `C:\…\app.db`）から `file:` DSN 構築する設計の妥当性検証。
- **Sources Consulted**: modernc `conn.go:43-64`（DSN 分岐→`SQLITE_OPEN_URI`）、SQLite `uri.html`、darwin 実機 PoC（modernc v1.46.1）。
- **Findings**: `file:<path>\…\app.db?_pragma=…` は SQLite URI パーサが `\` を区切りとして扱わず literal 化し、**意図したディレクトリでなく CWD に「バックスラッシュ込みファイル名」の単一ファイルを作成**。`Ping` は遅延接続のため成功し欠陥を隠す。`filepath.ToSlash` + 先頭 `/` の `file:/C:/…/app.db` で正しく解決。
- **Implications**: config の DSN 構築を forward-slash URI に確定（Decision 5）。Testing に DSN round-trip 検証を必須化。

### SESSION_SECRET の実体（CSRF 認証鍵）
- **Context**: R3.1/R3.2/R3.3 のコンポーネント帰属・脅威モデル確定。
- **Sources Consulted**: `internal/auth/session_auth.go:27-31`、`internal/middleware/csrf.go:15-34`、`session_store_test.go`、grep（cfg.SessionSecret 消費先）。
- **Findings**: scs は不透明トークン＋DB セッションストア（sqlite3store）で動き SESSION_SECRET 非使用。`cfg.SessionSecret` の唯一の非テスト消費先は gorilla/csrf 認証鍵導出（csrf.go）。再起動後のセッション維持は DB ファイル＋sessions テーブルの永続で成立し鍵の同一性とは独立。鍵永続化の真の効用は「再起動後も既発行 CSRF トークンを有効に保つ」こと。
- **Implications**: 用語を「CSRF 認証鍵」に正名。R3.2 の実現主体を DB セッションストア（S9）＋安定 DB パス（2.2）に修正。脅威モデルを「鍵漏洩→CSRF トークン偽造」に。

## Design Decisions
- **Decision 1（appdata 単一解決源）**: `%LOCALAPPDATA%\go_iot\` 解決を `internal/appdata` に集約し、applog（DefaultPath 委譲）・config（DB パス）・鍵パスが共有。三次フォールバックは CWD 相対でなく `os.Executable()` 隣に統一（GUI の CWD 不定対策）。
- **Decision 2（sync-migrations 一方向同期）**: `db/migrations` を canonical とし `make sync-migrations` で `internal/migrate/migrations/`（gitignore）へ複製・go:embed。既存 CSS（sync-css）/templ 生成物と同じ「生成物 gitignore＋make 前段＋embed」慣習に整合。代替（migrations を移設し canonical 化）は db/ 慣習変更が大きく不採用。
- **Decision 3（ブラウザ自動オープン）**: `cli/browser.OpenURL` を採用し pure-Go 維持（Wails/WebView2 は CGO＋ランタイム依存で不採用）。`Stdout/Stderr=io.Discard` で GUI ログ汚染防止。
- **Decision 4（mDNS ライブラリ）**: 主候補 `hashicorp/mdns`（HostName が A 応答直結・本要件に最適）。`Advertiser` interface で隔離し `grandcat/betamos` へ差替可能。IP 変動再登録・マルチ NIC IF 整合・FQDN 末尾ドット正規化を実装側で担保。
- **Decision 5（DB DSN = forward-slash file: URI）**: `config` が `filepath.ToSlash`＋ドライブ用先頭 `/` で `file:/C:/…/app.db` を構築（バックスラッシュ生パス不可）。必要に応じ URL パーセントエンコード。
- **Decision 6（将来 WithTx 時の `_txlock=immediate`）**: 現状 R8 保証は単一文 auto-commit 経路に限定。将来 read-then-write の明示 Tx を導入する場合は modernc 既定 DEFERRED の書き昇格デッドロック回避のため DSN に `_txlock=immediate` 付与（S9 所有の `withPragmas` 層改修）。`SetMaxOpenConns` 4→1 は writer 直列化用で昇格デッドロックとは別ノブ。

## 敵対的レビュー結果（Workflow: 47 エージェント・5 次元 × 各指摘 2 名検証）
- **確定 11 件すべてを design.md に反映済み**。内訳: critical 1（Windows DSN）、high 2（SESSION_SECRET 誤称・R3.2 機序）、medium 6（mDNS マルチNIC/衝突、goose ロールバック記述、鍵ファイル形式・破損・0o600、R8 保証範囲）、low 2（mDNS A/SRV ポート、壊れ埋込テスト表現）、加えて mDNS テスト flaky 分離。
- **要検討（割れ）2 件も軽微修正として反映**: (C1) Advertiser.Start 引数の図/契約整合＋FQDN 正規化ノート、(C2) appdata フォールバック規則の一意化（実行ファイル隣・単一規則共有）。
- いずれも design.md 内で完結する文書・契約レベル修正で、要件フェーズへ戻す真のギャップは無し。

## Risks & Mitigations（設計フェーズ追加分）
- Windows DSN 誤配置（critical）→ forward-slash URI＋round-trip テスト（Decision 5）。
- mDNS の実機到達性（A 解決・マルチ NIC・衝突）→ Advertiser 隔離＋残存リスク明記＋別フェーズ E2E。
- マイグレーション部分適用 → fail fast＋残存リスク明記（自動修復はしない）。
- 鍵ファイル平文・Windows 0o600 no-op → base64 text＋NTFS ACL 依存を脅威整理に明記。

## References
- [goose embed migrations](https://pressly.github.io/goose/blog/2021/embed-sql-migrations/) — SetBaseFS/SetDialect/Up
- [hashicorp/mdns zone.go](https://github.com/hashicorp/mdns/blob/main/zone.go) — HostName/A 応答
- [grandcat/zeroconf issue #74](https://github.com/grandcat/zeroconf/issues/74) — RegisterProxy の A 解決問題
- [cli/browser](https://github.com/cli/browser) — OpenURL/Stdout/Stderr
- [SQLite URI filenames](https://www.sqlite.org/uri.html) — forward-slash 要求
- modernc.org/sqlite `conn.go`（DSN→SQLITE_OPEN_URI 分岐）
