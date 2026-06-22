# Implementation Plan — alert-history（アラート履歴）

> 実装方針: 既存の兄弟画面 readings（センサーデータ履歴）を写経し、`/alerts/history` を templ + HTMX で実装する。新規 migration・新規 sqlc クエリ・新規 CSS 配線は無し（すべて実装済みを再利用）。タスクは上から1行ずつ `/tdd`（RED→GREEN→REFACTOR）で実装する逐次順。

- [x] 1. Foundation: 既存基盤の再利用確定（新規インフラなし）
- [x] 1.1 基盤・配信・ヘルパの再利用前提を確定しビルドベースラインを確認する
  - CSS 配信（`CSSURL()` ＋ `internal/view/static.go` の go:embed ＋ `make sync-css` 生成物）・共通レイアウト `App.templ`（Tom Select グローバル初期化 `select.js-tom-select`）・`RequireAuth`・既存 sqlc クエリ（`ListAlertHistoriesPaginated` / `CountAlertHistoriesInRange` / `ListDevicesByUser`）・再利用ヘルパ（`parseDateBounds` / `parsePage` / `totalPagesOf` / `formatActual` / `jst` / `timefmt.DateTimeMinuteJP`）は実装済みであり、本 spec は**再利用のみ**（新規に作り直さない）ことを確定する
  - 写経源は `mocks/html/alert-history.html`、HTMX 部分更新ターゲット id は `alert-history-list`（§3）と確定する。新規 migration/クエリが不要であることを `docs/database_snapshot/table_definitions.md` で確認する
  - 観測可能な完了: 実装着手前に `make templ && go build ./... && go test ./...` がグリーンであることを確認できる
  - _Requirements: 5.2, 8.2_

- [x] 2. Core: templ コンポーネント（下位部品から順に写経）
- [x] 2.1 番号付きページネーション部品を実装する
  - `AlertHistoryPaginationView` / `PageLink`（現在/総ページ・前後リンク・ページ番号配列）を view モデルに定義する
  - `nav.pagination` に `hx-boost="true"` ＋ `hx-target="#alert-history-list"` ＋ `hx-swap="innerHTML"` を付与し、前へ（無効時 `.disabled`）／番号リンク（現在ページは `.current`・他は `<a>`）／次へ（無効時 `.disabled`）を描画する。リンクは handler 生成の信頼 URL を `templ.SafeURL` で埋める。モック `alert-history.html` を写経し独自 CSS クラスを新設しない（§31）
  - 観測可能な完了: templ render テストで、総3ページ・現在2 のとき「前へ」リンク・番号 `1`/`3` のリンク・`2` が `.current`・「次へ」リンクが HTML に含まれ、最初のページでは「前へ」が `.disabled`、最後のページでは「次へ」が `.disabled` になる。ページ送りリンクが `hx-boost` で `#alert-history-list` を対象にする
  - _Requirements: 3.1, 3.4, 3.5, 3.6_
  - _Boundary: AlertHistoryPagination_

- [x] 2.2 結果領域フラグメント（履歴一覧＋空状態＋インラインエラー）を実装する
  - `AlertHistoryListView` / `AlertHistoryRow`（整形済み6項目・`HasData`・`HasPagination`・`Errors`）を view モデルに定義する
  - ルート `<div id="alert-history-list">`（id は HTMX ターゲット専用・スタイリング非使用）に、`Errors` 非空時の `.error-message` インライン表示 → `if HasData` で `table.data-table`（thead: 発火日時/デバイス/指標/ルール(条件/閾値)/実測値/通知 ＋ tbody の行ループ）／ `else` で `.empty-message`「指定期間のアラート履歴はありません。」→ `if HasPagination` で前タスクのページネーション部品を内包する（readings の `DeviceReadingsList` を写経）
  - 観測可能な完了: render テストで、行ありなら6列の `<tr>` が出力され、0件なら `.empty-message` が表示されページネーションが出力されず、`Errors["to"]` 指定時に `.error-message` 内へ該当文言が出る
  - _Requirements: 4.1, 4.5, 4.6, 6.1, 6.2, 7.1_
  - _Boundary: AlertHistoryList_
  - _Depends: 2.1_

- [x] 2.3 フルページ（共通レイアウト＋フィルタフォーム＋デバイス選択肢）を実装する
  - `AlertHistoryView`（レイアウトデータ・本人デバイス一覧・選択中 device_id・from・to・結果領域 View）を view モデルに定義する
  - `@layout.App` 内に `page-header h1`「アラート履歴」、`section.card > form.filter-form`、結果領域フラグメントを配置する。フォームは `method="get"` ＋ `hx-get="/alerts/history"` ＋ `hx-target="#alert-history-list"` ＋ `hx-swap="innerHTML"` ＋ `hx-push-url="true"` を付与し、**結果領域の外**に置く（swap 対象外＝入力保持・Tom Select 再初期化不要）。デバイス select は `class="js-tom-select"`、先頭に `<option value="">全デバイス</option>`、続けて本人所有デバイスを `for` で option 描画（選択中 device_id に `selected`）。from/to は `input type="date"`
  - 観測可能な完了: render テストで、フォームに `hx-get="/alerts/history"`・`hx-target="#alert-history-list"`・`hx-push-url="true"` が付き、「全デバイス」＋渡したデバイス名が `<option>` として出力され、選択中 device_id の option に `selected` が付き、select に `js-tom-select` クラスが付く
  - _Requirements: 2.4, 2.5, 5.1, 5.2, 5.3_
  - _Boundary: AlertHistory (page)_
  - _Depends: 2.2_

- [x] 3. Core: ハンドラ（整形・正規化の純関数 → HTTP 境界）
- [x] 3.1 入力正規化と表示整形の純関数群を実装する
  - device_id 解釈（空→nil＝全デバイス／数値→`*int64`／非数値→不正フラグ）、from≤to のローカル範囲検証（`parseDateBounds` の形式検証に加え、from・to 両指定かつ from>to のとき `errs["to"]` に「終了日は開始日以降の日付を指定してください」を積む。片方のみ・未指定はスキップ）、履歴行の表示整形（ルール条件 `> 35.00℃`・実測値 `38.50℃`・発火日時 `YYYY-MM-DD HH:MM`(JST)・指標ラベル 温度/湿度・通知 済/未・発火時点の非正規化値）、ページネーション組み立てと URL 生成（device_id/from/to を保持し page のみ差替・端の前後可否・現在ページ判定。`parsePage`/`totalPagesOf` を再利用）を実装する
  - 観測可能な完了: table-driven 単体テストがグリーン（各写像の入出力、from>to で `errs["to"]` 設定・from==to は許容・片方のみ指定は範囲検証スキップ、ページURL が検索条件を保持し page のみ差し替わる）
  - _Requirements: 1.4, 2.3, 3.2, 3.3, 4.2, 4.3, 4.4, 4.7, 7.1, 7.2_
  - _Boundary: AlertHistoryHandler_
  - _Depends: 2.1, 2.2_

- [x] 3.2 一覧ハンドラ（HTTP 境界・フルページ／フラグメント分岐）を実装する
  - 消費 interface `AlertHistoryRepo`（ユーザー取得・本人デバイス一覧・履歴ページング・件数）を定義し、`auth.UserID` 取得 → device_id 解釈 → 日付境界＋from≤to 検証（NG は 200＋インラインで一覧クエリを呼ばずスキップ）→ 件数取得 → ページクランプ → 一覧取得 → 表示 DTO 整形 → `HX-Request` 有無で分岐（有=結果領域フラグメントのみ／無=フルページ。フルページ時のみユーザー名とデバイス選択肢を取得）する。テナント分離のため `auth.UserID` を必ずクエリの user_id に渡す。DB エラー→500、device_id 非数値→400
  - 観測可能な完了: httptest＋Querier 手書きモックで、初期表示（パラメータ無し）=200・フルページ・最新順1ページ目・`DeviceID=nil`／検索（`HX-Request`）=`#alert-history-list` フラグメントのみ・`*int64` 指定／`page=2`=OFFSET 20＋条件保持 URL／0件=`.empty-message`・ページャ非出力／from>to=200＋インライン・一覧クエリ未呼出／非所有 device_id（モックが空返却）=空表示／件数・一覧クエリの error=500／device_id 非数値=400
  - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2, 2.3, 2.4, 3.2, 3.3, 6.1, 6.2, 7.1, 8.1, 8.3, 9.1_
  - _Boundary: AlertHistoryHandler_
  - _Depends: 3.1_

- [x] 4. Integration: ルーティング配線
- [x] 4.1 ルート登録とサイドバー導線を配線する
  - `cmd/server/main.go` で `AlertHistoryHandler{Repo: q}` を生成し、認証必須グループに `web.GET("/alerts/history", middleware.RequireAuth(), …)` を登録する（`/alerts/rules` 群に隣接）。共通レイアウト／サイドバーの「🕐 アラート履歴」リンクが `/alerts/history` を指しアクティブ表示されることを確認する
  - 観測可能な完了: 認証済みセッションで `GET /alerts/history` が 200・フルページを描画し、未認証アクセスは `/login` へ 302 リダイレクトする。`go build ./...` がグリーン
  - _Requirements: 8.2_
  - _Depends: 3.2_

- [x] 5. Validation: 結合・カバレッジ・E2E
- [x] 5.1 結合テストとカバレッジ（80%+）を確認する
  - ハンドラ→templ の結合（フルページ HTML／HTMX フラグメント）を Querier 差し替えで網羅し、全受け入れ基準の経路（初期表示・検索・ページ送り・0件・from>to・テナント分離・500・400）を横断的にカバーする。`go test ./... -cover` で対象パッケージ（handler / view）のカバレッジが 80% 以上であることを確認する
  - 観測可能な完了: 対象パッケージのカバレッジが 80% 以上で全テストがグリーン
  - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2, 2.3, 2.4, 2.5, 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 5.1, 5.2, 5.3, 6.1, 6.2, 7.1, 7.2, 8.1, 8.2, 8.3, 9.1_
  - _Depends: 4.1_

- [ ]* 5.2 （任意）主要ユーザーフローの E2E を作成する
  - Playwright で、検索フォーム submit により `#alert-history-list` のみが差し替わり URL に検索条件が反映されること、ページ番号リンクのクリックで検索条件を保持したままページ送りされること、Tom Select でデバイス選択肢を絞り込み選択できることを検証する
  - 観測可能な完了: 上記3フローの E2E がグリーン（または同等の手動確認手順を記録）
  - _Requirements: 2.4, 2.5, 3.2, 3.4, 5.2_
  - _Depends: 4.1_
