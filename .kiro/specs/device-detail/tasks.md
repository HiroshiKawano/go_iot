# Implementation Plan — device-detail（デバイス詳細）

> 逐次実装（上から1行ずつ `/tdd` で RED→GREEN→REFACTOR）。各サブタスクは単一の観測可能な振る舞いに対応する。
> スキーマ変更なし（`devices`/`sensor_readings` の既存カラムで充足）。生成依存順: sqlc 再生成 → templ → 消費実装。
> CSS 配信（`make sync-css` + `static.go` go:embed + `CSSURL()`）と HTMX/Alpine/CSRF 配線（`App.templ`）は S1 で完成済みのため本計画に含めない（再利用のみ）。

- [x] 1. Foundation（共有ユーティリティ・生成コード）
- [x] 1.1 最新計測テーブル用クエリの追加と sqlc 再生成
  - `sensor_readings` に対する「device_id 指定・`recorded_at` 降順・最大10件・`deleted_at IS NULL`」の取得クエリを定義する（既存の「時刻以降・昇順」クエリとは別物として命名分離）。
  - スキーマ変更は不要（既存カラムのみ参照）。`make sqlc` で `repository.Querier` と生成コードへ反映する。
  - 観測可能完了: `repository.Querier` に最新10件取得メソッドが生成され、`go build ./...` が通る。
  - _Requirements: 5.1, 5.4_

- [x] 1.2 絶対時刻整形ヘルパの追加
  - 既存の相対整形（"N分前"）とは別に、最終通信用「YYYY-MM-DD HH:MM:SS」と計測テーブル用「YYYY-MM-DD HH:MM」の2書式を返す純粋関数を `timefmt` に追加する。
  - 基準時刻を引数で受ける決定的なユニットテストを先に書く。
  - 観測可能完了: 固定時刻入力に対し2書式の文字列を返すユニットテストが green。
  - _Requirements: 2.4, 5.3_

- [x] 2. SVG グラフ生成（internal/chart 純粋パッケージ・stdlib のみ依存）
- [x] 2.1 空状態・単一系列の線グラフ SVG 生成
  - 整形済み float 点列（軸ラベル文字列＋数値）を受け取り、温度/湿度いずれにも使える線グラフ SVG 文字列を生成する。色・寸法・軸ラベルは design の視覚仕様（viewBox 720x240・温度 #e8590c / 湿度 #1971c2・Y軸 min/max ラベル・X軸ラベル）に従う。
  - 有効点が0件のときは「データはまだありません」を中央に表示した空 SVG を返す。
  - gin/DB/templ/pgtype を import しない（純粋ユーティリティ）。table-driven テストを先に書く。
  - 観測可能完了: 空入力→「データはまだありません」を含み `<polyline>` を含まない、1系列入力→`<polyline>` 1本と軸ラベルを含む SVG を返すユニットテストが green。
  - _Requirements: 4.1, 4.2, 4.4, 4.5_

- [x] 2.2 2系列（日次 最大/最小）の線グラフ対応
  - 3日/7日/30日グラフ用（24h以外の複数日=日次集計）に、1つの SVG 内へ最大系列（実線）と最小系列（破線 `stroke-dasharray`）の2本を描画し、凡例を付す。
  - 観測可能完了: 2系列入力で `<polyline>` を2本（うち1本は破線指定）含む SVG を返すユニットテストが green。
  - _Requirements: 4.3_

- [x] 3. ビュー（templ コンポーネント・`mocks/html/device-show.html` を写経／独自 CSS クラス新設禁止）
- [x] 3.1 デバイス情報パネル
  - 名前・MAC・場所・稼働状態・最終通信を表示する情報パネルを写経実装する。稼働中は「● 稼働中」、停止中は停止記号（○）で区別。最終通信は引数で受けた整形済み文字列を表示し、未通信は「未通信」、場所未設定は「未設定」を表示する。編集リンク（`/devices/{id}/edit`）と削除ボタンを配置する。
  - 表示値は整形済み primitive のみを引数で受け取る（pgtype/repository 型を持ち込まない）。
  - 観測可能完了: `Render`→`strings.Contains` で状態記号・MAC・編集 URL・最終通信書式・未通信/未設定フォールバックを検証するテストが green。
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6_

- [x] 3.2 最新計測データテーブル
  - 計測日時・温度・湿度の3列テーブルを写経実装し、tbody に HTMX 差し替え専用 id（`latest-readings-table`）を付与する（スタイリングには使わない）。最大10件を行描画し、0件時は「計測データはまだありません。」を表示する。
  - 観測可能完了: `Render`→3行で日時/温度/湿度が出力され、空入力で「計測データはまだありません。」を含むテストが green。
  - _Requirements: 5.1, 5.2, 5.3, 5.5_

- [x] 3.3 グラフ領域フラグメント（期間ボタン＋温度/湿度グラフ）
  - モックの `chart-placeholder` div と `<a href="?period=">` から、design 確定の設計判断（§10-D フルフラグメント swap）に従って **HTMX 化する**: ラッパに id `device-chart-area`、温度/湿度グラフ領域に id `temperature-chart`/`humidity-chart`（モックに無い HTMX 専用 id を新設）を付与し、期間リンクを `<button type="button">` に置換して `hx-get="/devices/{id}/chart?period={p}"` + `hx-target="#device-chart-area"` + `hx-swap="innerHTML"` + `hx-push-url="/devices/{id}?period={p}"`（フルページ URL を push）を付ける。アクティブ期間は状態クラス `active` を `templ.KV` で付与（class のみ・id はスタイル非使用）。各グラフ領域へ SVG 文字列を `@templ.Raw` で埋め込む。
  - 観測可能完了: `Render`→`period=7d` で7日ボタンに `active` が付き、温度/湿度2つの SVG と各 id・上記 HTMX 属性が出力されるテストが green。
  - _Requirements: 3.1, 3.3, 4.1_

- [x] 3.4 デバイス詳細フルページ（情報パネル＋グラフ領域＋テーブル＋削除モーダル）
  - 共通レイアウト（認証後 App）を使い、見出しにデバイス名、3.1/3.3/3.2 の各部品を1ページに統合する。グラフ領域は `device-chart-area` をラップする。削除確認モーダル（対象デバイス名＋「※この操作は取り消せません。計測データも削除されます。」）を配置する。
  - 削除ボタンとモーダルは、レイアウト body とは別の**ネスト `x-data="{ deleteModalOpen:false }"` の要素内**に両方を置く（スコープ外配置による Alpine エラー回避）。削除ボタン `@click="deleteModalOpen=true"`、キャンセルで `deleteModalOpen=false`（削除は発火しない）、確認ボタンに `hx-delete="/devices/{id}"`。
  - 観測可能完了: `Render`→ページにデバイス名見出し・3部品・削除モーダルが同一 `x-data` スコープ内に出力され、キャンセル/確認ボタンが存在するテストが green。
  - _Requirements: 1.1, 1.3, 1.4, 6.1, 6.2, 6.4_

- [x] 4. ハンドラ（DeviceHandler 拡張・新ファイル device_show.go）
- [x] 4.1 DB ポート拡張とフラグメント描画ヘルパの整備（前提）
  - 既存の最小 interface `DeviceRepo` に、sqlc 生成済みメソッド（最新10件取得・24h生データ取得・日次集計取得・論理削除）を**宣言追加**する（クエリ新規作成ではなく interface へのメソッド宣言。`repository.Querier` が充足し合成ルートは無改修）。
  - レイアウトを含めず templ コンポーネントを 200 で描画するフラグメント描画ヘルパ（既存フルページ描画ヘルパと別物）を handler パッケージに追加する。
  - 観測可能完了: 拡張 interface で `go build ./...` が通り、フラグメント描画ヘルパ経由で component が HTML として出力されるテストが green。
  - _Depends: 1.1_
  - _Requirements: 3.2, 5.1_

- [x] 4.2 デバイス詳細表示ハンドラ（GET /devices/:device）
  - パスの device を int64 へ変換（非数値は 400）、所有者認可（不在→404・非所有→403・未認証経路は前置の認証ミドルウェアが処理）を既存の認可写像で行う。任意の `?period`（24h/3d/7d/30d・既定 24h・不正値は 400）を解釈し、デバイス情報＋最新10件＋期間別グラフデータ（24h=生データ／3d・7d・30d=日次集計）を取得、`internal/chart` で温度/湿度 SVG を生成して 3.4 のフルページを描画する。集計の nullable 値は安全変換し、時刻は 1.2 のヘルパで整形する。
  - Querier 手書きモックで DB 非依存にテストする。
  - 観測可能完了: `httptest` で 200 のフルページにデバイス名・MAC・既定24hアクティブ・最新計測行が含まれ、`?period=7d` で7dアクティブ、計測0件で空グラフ＋テーブル空メッセージ、非数値 id→400、他ユーザー所有→404、DB エラー注入→500 を検証するテストが green。
  - _Depends: 1.1_
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 4.2, 4.3, 4.5, 5.1, 7.1, 7.2, 8.1, 8.3_

- [x] 4.3 期間切替ハンドラ（GET /devices/:device/chart）
  - device を int64 変換（非数値→400）、`period` をクエリバリデーション（`required,oneof=24h 3d 7d 30d`・不正→400）、所有者認可（不在→404・非所有→403）を行い、期間別データから温度/湿度 SVG を再生成して **グラフ領域フラグメントのみ**（期間セレクタ＋2グラフ）を 4.1 のヘルパで返す。アクティブ期間はサーバー側で判定して往復させ、最新計測テーブルは返却に含めない（期間非連動）。
  - 観測可能完了: `httptest` で `HX-Request` 付きリクエストがグラフ領域フラグメント（`<html>`・サイドバー非包含）を返し、要求 period のボタンに `active`・温度/湿度 SVG 2つを含み `latest-readings-table` を含まない、`period` 不正→400・他ユーザー所有→404 を検証するテストが green。
  - _Depends: 1.1_
  - _Requirements: 3.2, 3.3, 3.4, 3.5, 4.2, 4.3, 5.4, 8.2_

- [x] 4.4 削除ハンドラ（DELETE /devices/:device）
  - device を int64 変換（非数値→400）、所有者認可（不在→404・非所有→403）後に論理削除を実行する。`HX-Request` 有のときは `HX-Redirect: /dashboard` ヘッダ＋200、非 HTMX（フォーム `_method=delete`）のときは 303 でダッシュボードへ遷移させる。CSRF 保護下で動作する。
  - 観測可能完了: `httptest` で HTMX リクエスト→200＋`HX-Redirect: /dashboard` かつ論理削除メソッド呼出、非HTMX→303＋`Location: /dashboard`、他ユーザー所有→403/不在→404、非数値 id→400 を検証するテストが green。
  - _Depends: 1.1_
  - _Requirements: 6.3, 6.5, 6.6, 7.3, 8.1_

- [x] 5. ルーティング配線（統合）
  - Web ルートグループに、認証必須で 3 ルート（詳細表示 GET `/devices/:device`、期間切替 GET `/devices/:device/chart`、削除 DELETE `/devices/:device`）を追加する。既存の静的経路（`/devices/create`）・パラメータ経路（`/devices/:device/edit`、PUT `/devices/:device`）と共存させる。削除は HTMX の DELETE と非 HTMX フォーム（`_method=delete` → 既存 MethodOverride）の両経路が同一ハンドラへ到達することを確認する。
  - 観測可能完了: 起動後（または `httptest`）に 3 ルートが解決し、未認証アクセスがログインへリダイレクト（302）、認証済みで詳細表示・期間切替フラグメント・削除遷移が動作する。
  - _Depends: 4.2, 4.3, 4.4_
  - _Requirements: 7.1_

- [x] 6. 統合検証とカバレッジ
  - 主要ユーザーフローを通しで検証する: 期間切替で `#device-chart-area` のみ差し替わりテーブルが据え置かれること、期間がページ URL に反映されること、削除モーダル→確認→/dashboard 遷移、CSRF トークン往復（GET でトークン取得→DELETE 成功／トークン無し DELETE→403）、計測0件の空表示、他ユーザー/不在デバイスの 404 によるユーザー列挙防止、HTMX(HX-Redirect)/非HTMX(303) の使い分け、全文言の日本語。
  - 観測可能完了: `go test ./...` が green、device-detail 関連ハンドラ/パッケージのカバレッジが 80% 以上。
  - _Depends: 5_
  - _Requirements: 3.5, 6.4, 6.6, 7.2, 8.4_
