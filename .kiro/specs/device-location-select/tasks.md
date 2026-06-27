# Implementation Plan — device-location-select（圃場所在地の住所セレクト・平坦化版）

> 逐次実装（上から1行ずつ `/tdd`）。各サブタスクは RED→GREEN→REFACTOR の1サイクル・単一責務。
> 生成依存順を Foundation で厳守: goose migration → `make db-snapshot` → `make sqlc` → `templ generate` → 消費実装。
> 設計は `design.md`（平坦化版）、確定データ（41市町村・53地域）は `research.md`、定石は `2cc_sdd/テストガイダンス集.md`。
> **平坦化によりカスケード/HTMX swap/Tom Select swap ライフサイクル/地区端点は無し**（App.templ 改変なし）。

## 1. Foundation: マスタ定数とスキーマ

- [x] 1.1 沖縄県41市町村のドメイン定数を定義する
  - `internal/domain/municipality.go` に `type Municipality string` と41市町村の const、`Label()`/`Valid()`/`AllMunicipalities()` を `metric.go` 同型で実装する（`fmt` のみ依存）。集計軸（地域の親）として使う。
  - テーブルテストで `AllMunicipalities()` が41件、`Valid()` が定義値のみ true を検証する（先に RED）。
  - 観測可能完了: `go test ./internal/domain/` で41件アサートが緑。
  - _Requirements: 8.1, 8.2_
  - _Boundary: domain.Municipality_

- [x] 1.2 沖縄53地域のドメイン定数（親市町村対応・同名区別）を定義する
  - `internal/domain/locality.go` に `type Locality string` と53地域の const（未合併36＝市町村名／旧町村17＝正式名で一意）、`Municipality()`（合併＝旧町村→現市町村・未合併＝自身）、`Label()`（合併＝「旧町村（現市町村）」例「佐敷（南城市）」／未合併＝市町村名）、`Valid()`、`ParseLocality()`（旧名/正式名/現市町村名のエイリアス解決）、`AllLocalities()` を実装する。値・親市町村は `research.md` の確定データから転記。
  - テーブルテストで、`AllLocalities()` が53件、各地域の `Municipality()` が正しい親、**同名（具志川市/具志川村）が別 Locality で Label に現市町村が併記され区別される**こと、`Valid()`/`ParseLocality()` のエイリアス解決を検証する。
  - 観測可能完了: 53件・親対応・同名区別・エイリアス解決がテストで緑。
  - _Requirements: 8.1, 8.2, 8.3, 1.2, 2.1, 2.2, 2.3, 2.4, 5.1_
  - _Boundary: domain.Locality_

- [x] 1.3 devices に locality 列を追加するマイグレーション（DDL のみ）
  - `db/migrations/00008_add_locality_to_devices.sql`（goose・連番手動）で `locality VARCHAR(20)`（nullable）を ALTER ADD、`CONSTRAINT devices_locality_valid CHECK (locality IS NULL OR locality IN (<53地域値>))`、部分索引 `devices_locality_idx ... WHERE deleted_at IS NULL`、日本語 COMMENT を付与する。DML は含めない。Down で列 DROP。
  - `make migrate-up` 適用後 `make db-snapshot` を実行する。
  - 観測可能完了: `docs/database_snapshot/table_definitions.md` の devices に `locality`・CHECK・索引が反映され、差分が当該追加のみ。
  - _Requirements: 3.1, 8.1_
  - _Boundary: db/migrations_

- [x] 1.4 デバイスクエリに locality と backfill 用クエリを追加し sqlc を再生成する
  - `db/queries/devices.sql` の `CreateDevice`/`UpdateDevice` に `locality` を追加（INSERT 列・`SET` 句・プレースホルダ `$6`）。backfill 用に `ListAllDevices`（全ユーザー横断・`WHERE deleted_at IS NULL`）と `UpdateDeviceLocality(id, locality)`（locality 列のみ更新）を追加。`GetDevice`/`ListDevicesByUser` は `SELECT *` ゆえ自動反映。`make sqlc` で再生成する。
  - 新フィールド `Locality` は `*string`（nil 許容）ゆえ、**1.4 時点では既存 `Create`/`Update` 呼び出しは省略フィールドが nil でコンパイルを通る**（値充填は 2.5）。`ListAllDevices`/`UpdateDeviceLocality` は 4.1 が消費。
  - 観測可能完了: 再生成後 `CreateDeviceParams`/`UpdateDeviceParams` に `Locality`、`ListAllDevices`/`UpdateDeviceLocality` が生成され、既存呼び出しがコンパイルを通る。
  - _Requirements: 3.1, 3.2, 7.1_
  - _Boundary: db/queries, repository.Querier_

## 2. Core: フォーム・検証・保存

- [x] 2.1 地域選択肢 DTO とフォーム View を拡張する
  - `internal/view/component/views.go` に `SelectOption{Value,Label,Selected}` を新設（既存 `DeviceOption` は ID int64 で文字列キーに不適合）。`DeviceFormView` に `Locality string`/`Localities []SelectOption` を追加する。
  - 観測可能完了: `DeviceFormView` が地域の選択値と選択肢を保持でき、`go build ./internal/view/...` が通る。
  - _Requirements: 1.1, 4.2_
  - _Boundary: view/component DeviceFormView_

- [x] 2.2 デバイスフォームの設置場所を単一の地域 select に置き換える
  - 先にモック正本 `mocks/html/device-create.html`・`device-edit.html` の設置場所 input を単一 `<select name="locality" class="js-tom-select">`（認識名 option・先頭に空 option）へ置換（独自クラス新設禁止）。style.css 変更時のみ `make sync-css`。
  - `internal/view/component/DeviceForm.templ` を写経更新: `Localities []SelectOption` を `<option value={opt.Value} selected?={opt.Selected}>{opt.Label}</option>` で range。**hx 属性なし**（カスケード無し）。`templ generate`。
  - templ Render→`strings.Contains` で、単一地域 select と認識名 option（例「佐敷（南城市）」）・選択値 selected が出力され、hx 属性が無いことを検証する。
  - 観測可能完了: フォームが単一の地域 select（認識名・検索可能）で描画される。
  - _Requirements: 1.1, 1.3, 1.4, 1.5, 2.1, 2.2, 2.3_
  - _Boundary: view/component DeviceForm, mocks/html_

- [x] 2.3 登録・編集フォーム表示に地域選択肢と選択値復元を組み込む
  - `ShowCreateForm`/`ShowEditForm` で `domain.AllLocalities()` を `[]SelectOption{Value:string(l), Label:l.Label(), Selected:l==current}` に整形して `DeviceFormView.Localities` へ詰め、編集時は保存済み地域を `Selected` で復元する。未設定は未選択。
  - httptest+`fakeDeviceRepo` で、登録は53地域 option（認識名）を含む空選択、編集は保存値が selected、未設定は空選択を検証する。
  - 観測可能完了: 編集フォームで保存済み地域が selected 状態で描画される。
  - _Requirements: 1.1, 2.1, 2.4, 4.1, 4.3_
  - _Boundary: handler DeviceHandler_

- [x] 2.4 地域入力のバリデーションを追加する
  - `internal/handler/device_form.go` の `deviceForm` に `Locality` を追加。bind 後に **early-return せず errors マップへ累積**する procedural 検証を実装: 存在しない地域（`Locality(v).Valid()==false`）を拒否し日本語メッセージを積む（空は許容）。`deviceFieldKey`・`deviceValidationMessage` の両 switch に case 追加。
  - table-driven＋httptest で、不正地域・複数同時不正（地域＋他項目）が 200 再描画＋項目別メッセージ＋選択値復元＋`createCalled==false` を検証する。
  - 観測可能完了: 不正な地域で保存されず、所在地欄に日本語エラーが他項目エラーと同時表示される。
  - _Requirements: 5.1, 5.2_
  - _Boundary: handler device_form_

- [x] 2.5 妥当な地域をデバイスに保存する
  - `Create`/`Update` で `locality` を `*string`（空→nil・`locationPtr` 流儀）に変換し `CreateDeviceParams`/`UpdateDeviceParams` へ渡す。既存の認証必須・所有者認可・成功時 303 リダイレクトを維持する。
  - httptest+`fakeDeviceRepo` で、妥当入力時に `locality` が params に積まれ 303、未選択は nil 保存・成功、既存フロー（401/404/303）が不変であることを検証する。
  - 観測可能完了: 地域を選んで登録/更新すると所在地が保存され `/devices/{id}` へ 303 リダイレクトする。
  - _Requirements: 3.1, 3.2, 3.3_
  - _Boundary: handler DeviceHandler_

## 3. 表示（詳細・ダッシュボード）

- [x] 3.1 デバイス詳細パネルに認識名で所在地を表示する
  - `internal/handler/device_show.go` の `buildDeviceInfoView` に `Locality.Label()`（認識名）整形を追加し、`DeviceInfoView` と `DeviceInfoPanel.templ` に表示行を追加（未設定は従来同等の空）。モック `device-show.html` に反映。`templ generate`。
  - templ Render で、所在地が認識名（合併は「旧町村（現市町村）」）で表示され未設定は空を検証する。
  - 観測可能完了: device-show 情報パネルが認識名の所在地を表示する。
  - _Requirements: 6.1, 6.3_
  - _Boundary: handler device_show, view/component DeviceInfoPanel_

- [x] 3.2 ダッシュボードのデバイスカードに認識名で所在地を表示する
  - `internal/handler/dashboard.go` の `buildDashboardDevice` に `Locality.Label()` 整形を追加し、`DashboardDevice` と `DeviceCard.templ` の所在地表示を認識名へ。未設定は空。モック `dashboard.html` に反映。`templ generate`。
  - templ Render でカードの所在地表示を検証する。
  - 観測可能完了: ダッシュボードの各カードが認識名で所在地を表示する。
  - _Requirements: 6.2, 6.3_
  - _Boundary: handler dashboard, view/component DeviceCard_

## 4. 既存所在地データの非破壊移行（backfill）

- [x] 4.1 既存 location を地域へ移行する backfill ロジックを実装する
  - `internal/locationbackfill/backfill.go` に `BackfillLocations(ctx, q repository.Querier) (int, error)` を実装: **1.4 の `ListAllDevices`** で全デバイスを走査し、`locality` 未設定かつ `location` が `domain.ParseLocality`（旧名/正式名/現市町村名のエイリアス）成功の行のみ **`UpdateDeviceLocality`** で設定（他フィールド・`location` 不変）。冪等（未設定条件）・非破壊（UPDATE のみ）。
  - `fakeDeviceRepo` で、一致 location → `locality` 設定、不一致 → 無変更、`location` 不変、削除を呼ばない、再実行で更新0件（冪等）を検証する。
  - 観測可能完了: backfill 後、地域名一致の既存デバイスのみ `locality` が設定され、件数・`location` は不変。
  - _Requirements: 7.1, 7.2, 7.3_
  - _Boundary: internal/locationbackfill_

- [x] 4.2 backfill 実行 CLI を追加する
  - `cmd/migrate-locations/main.go` を `cmd/seed` 骨格（`config.Load`→`infradb.NewPool`→`repository.New`→`BackfillLocations`）で実装し、`Makefile` に `migrate-locations` ターゲットと `.PHONY` を追記する。
  - 観測可能完了: `make migrate-locations` が更新件数をログ出力して正常終了する。
  - _Requirements: 7.1, 7.3_
  - _Boundary: cmd/migrate-locations_

## 5. Integration & Validation

- [x] 5.1 登録〜表示の全体フローを疎通させる
  - 開発サーバで、地域 select から認識名を検索選択→保存→詳細/カードに認識名表示までの一連を疎通する。既存の登録・編集・詳細・ダッシュボードに無回帰であることを確認する。
  - 観測可能完了: 地域を選んで登録し、詳細とカードに認識名の所在地が表示される。
  - _Requirements: 1.1, 3.3, 6.1, 6.2_
  - _Depends: 2.5, 3.1, 3.2_
  - _Boundary: cmd/server_

- [x] 5.2 地域選択の E2E（認識名検索・同名区別）を検証する
  - 地域 select で旧町村名（例「佐敷」）を入力して候補が絞り込まれ選択・保存できること、**同名（具志川）が現市町村併記で区別表示**されること、未選択での保存、編集時の選択復元を検証する。
  - 観測可能完了: 認識名検索・同名区別・未選択保存・選択復元の E2E が緑。
  - _Requirements: 2.1, 2.3, 2.4, 4.1, 4.2_
  - _Depends: 5.1_
  - _Boundary: E2E_
