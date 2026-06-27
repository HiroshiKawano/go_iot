# フェーズ1（分析ロードマップ）spec-init プロンプト: 圃場所在地の地域セレクト（自由入力 → 単一の検索可能「地域」セレクト・平坦化）

> **【2026-06-26 改訂】本プロンプトは平坦化モデルに更新済み。** 当初は「市町村→地区の2段カスケード＋市町村/地区マスタ**テーブル**」案だったが、**沖縄の農家≒60歳以上で市町村合併前(旧町村)の呼び名で土地を認識する**実地知見（ユーザー言明・権威）を受け、**旧町村名で直接選べる単一セレクト＝平坦化**へ転換した。新合併名を先に選ばせるカスケードは当人の認識と逆順で、実装も単純（カスケード/HTMX swap/地区フラグメント端点 不要）。**本 spec は既に requirements→design→tasks まで生成済み**（`.kiro/specs/device-location-select/`）。本プロンプトは経緯記録＋再生成時の種。確定データ・最終設計は同 spec の design.md / research.md が正。

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: device-location-select
> 位置づけ: [分析アイデアメモ.md](../分析アイデアメモ.md) 第1章「実装ロードマップ」の**フェーズ1（最初の spec）**。実装済みデバイス登録/編集・詳細画面のデータモデル拡張（新規画面ではない）。
> 前提セッション: S4（device-create-edit。DeviceForm・フルページ POST・入力値復元・バリデーション表示）／ S1（App レイアウト・MethodOverride・CSRF・Tom Select 初期化）
> 設計フェーズで参照:
> - 上位ロードマップ: 分析アイデアメモ.md フェーズ1／ 設計ガードレール①集計軸・②スキーマ拡張余地／ 付録B-4 沖縄主要作物
> - 確定データ（権威）: `.kiro/specs/device-location-select/research.md`（沖縄41市町村・平成の大合併5件・地域マスタ53件＝未合併36＋旧町村17・同名注意）
> - 現スキーマ（権威）: docs/database_snapshot/table_definitions.md「devices」（location VARCHAR(255) nullable 自由入力）
> - 移行元コード: db/migrations/00002_create_devices.sql（location 列）／ db/queries/devices.sql（CreateDevice・UpdateDevice）／ internal/view/component/DeviceForm.templ（設置場所 = text input）／ DeviceInfoPanel.templ・DeviceCard.templ（所在地表示）／ internal/handler/device.go・device_form.go
> - マスタ規約（必読）: .kiro/steering/structure.md §98-100（**FK 張らない／マスタは DB テーブルでなく Go 定数+VARCHAR+CHECK**）。手本 = internal/domain/metric.go・comparison_operator.go
> - Tom Select: internal/view/layout/App.templ（CDN 2.3.1・`select.js-tom-select` を `initTomSelect()` で初回初期化）。**本 spec は単一 select・swap 無しゆえ App.templ 改変不要**。
> - 命名・依存規約: .kiro/steering/structure.md（依存方向・view 層）／ tech.md（データアクセス方針・sqlc）／ CLAUDE.md（マイグレーション変更後 `make db-snapshot` 必須）

--- spec-init 本文 ここから ---

## 機能概要

デバイス登録・編集時の「設置場所」は現在 `devices.location`（VARCHAR(255) nullable）への**自由入力テキスト1列**で、詳細住所も GPS も誰も入れず、地点を集計・比較できるキーになっていない。本フェーズでは設置場所を **沖縄の地域を選ぶ単一の検索可能セレクト**に置き換える。**沖縄の農家はほぼ60歳以上で、市町村合併前（旧町村）の呼び名で土地を認識する**ため、農家が知っている名前（旧町村・未合併は市町村名）を1つ選ぶ平坦なモデルとする。地域マスタ（53）と親市町村（41）を **`internal/domain` の Go 定数**として持ち（既存 `Metric`/`ComparisonOperator` enum 規約＝structure.md §100 の踏襲）、`devices` に `locality`（nullable VARCHAR）を1列追加する。フォームは Tom Select の検索可能 select 1つ。device-show 情報パネル・dashboard カードの所在地表示を**認識名**に切り替え、既存 `location` をベストエフォートで非破壊移行する。**県は沖縄固定。字・番地・緯度経度・字レベル細粒度は取らない。**

## 背景・現状

- **現スキーマ**: `devices.location` は `VARCHAR(255)` nullable の自由入力。構造化された所在地キーは無い。
- **現 UI（S4/S5/S3 実装済み）**: 編集は DeviceForm.templ の text input（フルページ POST・`gorilla.csrf.Token` hidden・編集 `_method=put`・エラーは `map[string]string` を templ 引数）。表示は DeviceInfoPanel.templ・DeviceCard.templ。sqlc の CreateDevice/UpdateDevice が location を受ける。
- **Tom Select は配線済み（S1）**: `initTomSelect()` が `select.js-tom-select` を初回初期化。本 spec の地域 select はフルページフォーム内の通常 select（swap されない）ゆえ初回初期化で足り、**App.templ は改変しない**。
- **マスタ規約（最重要・structure.md §98-100）**: 選択肢系は DB テーブルでなく `domain` の Go 定数 + `VARCHAR + CHECK`。FK・マスタテーブルは現状ゼロ。→ 地域もテーブル化せず Go 定数で持つ。
- **確定データ（research.md・confidence high）**: 沖縄41市町村（11市11町19村）。平成の大合併は**5件のみ**（久米島町←仲里/具志川、うるま市←石川/具志川/与那城/勝連、宮古島市←平良/城辺/下地/上野/伊良部、八重瀬町←東風平/具志頭、南城市←佐敷/知念/玉城/大里）。残り36市町村は未合併（現名＝旧名）。**地域マスタ = 未合併36 + 旧町村17 = 53**。注意: 豊見城市は単独市制（合併でない）／旧具志川市≠旧具志川村（同名異所）。

## このセッションのスコープ（実装対象）

### ドメイン定数（Go 定数マスタ・テーブル化しない）
- `internal/domain/municipality.go`: `type Municipality string`（41）＋ `Label/Valid/AllMunicipalities`（集計軸）。
- `internal/domain/locality.go`: `type Locality string`（53・値は一意＝合併旧町村は正式名/未合併は市町村名）＋ `Municipality()`（合併＝旧町村→現市町村・未合併＝自身）＋ `Label()`（合併＝「旧町村（現市町村）」例「佐敷（南城市）」/未合併＝市町村名）＋ `Valid()` ＋ `ParseLocality()`（旧名/正式名/現市町村名のエイリアス解決・移行で利用）＋ `AllLocalities()`。`metric.go` を写経。同名（具志川）は値で区別・Label で現市町村併記。

### スキーマ・マイグレーション（goose 連番 00008・DDL のみ）
- `devices` に `locality VARCHAR(20)`（nullable）を ALTER ADD ＋ `CONSTRAINT devices_locality_valid CHECK (locality IS NULL OR locality IN (<53地域値>))` ＋ 部分索引 `devices_locality_idx ... WHERE deleted_at IS NULL` ＋ 日本語 COMMENT。**DML（backfill）は含めない。** Down で列 DROP。
- **親市町村は DB 列に持たない**（`Locality.Municipality()` で導出。将来 SQL 集計で必要なら非破壊追加可＝YAGNI）。
- 変更後 `make db-snapshot` 再生成（CLAUDE.md 必須）。

### sqlc クエリ・リポジトリ
- `CreateDevice`/`UpdateDevice` に `locality` を追加。backfill 用に `ListAllDevices`（全ユーザー横断）と `UpdateDeviceLocality(id, locality)`（locality のみ更新）を追加。`make sqlc` 再生成。

### フォーム（DeviceForm.templ）の単一地域 select 化
- 設置場所の text input を **単一 `<select name="locality" class="js-tom-select">`**（`Localities []SelectOption` を `<option value selected?>`・先頭に空 option「選択してください」）へ置換。**hx 属性なし**（カスケード無し）。
- 入力値復元: 選択値を `selected` で復元。エラーは `.error-message` にインライン。独自 CSS クラス新設禁止（§31・モック先行反映）。

### 表示（device-show / dashboard）・ハンドラ
- DeviceInfoPanel.templ・DeviceCard.templ の所在地を `Locality.Label()`（認識名）へ。未設定は従来同等の空。
- `ShowCreateForm`/`ShowEditForm` で `domain.AllLocalities()` を `[]SelectOption{Value,Label,Selected}` に整形して View へ（編集は保存済み地域を Selected）。`Create`/`Update` で `locality` を `*string`（空→nil）で params へ。`deviceForm` に `Locality` 追加・procedural 存在検証（`Locality.Valid()`・空は許容・**early-return せず errors 累積**）・`deviceFieldKey`/`deviceValidationMessage` の2 switch に case 追加。

### 既存データ移行（Go backfill・migration 内 DML にしない）
- `internal/locationbackfill/backfill.go` の `BackfillLocations(ctx, q)`: `ListAllDevices` で走査し、`locality` 未設定かつ `location` が `ParseLocality` 成功の行のみ `UpdateDeviceLocality` で設定（冪等・非破壊）。fakeRepo で DB 非依存テスト。`cmd/migrate-locations` で一度きり実行（Makefile ターゲット追記）。

## スコープ外（このセッションでやらないこと）

- **2段カスケード・市町村別の連動更新**（平坦化により作らない）。DistrictSelect・ShowDistrictOptions・`GET /devices/districts`・Tom Select swap ライフサイクルも不要。
- ハウス内センサ位置（内/外・東/西）＝フェーズ10。圃場エンティティ・ベンチマーク・農家共有＝フェーズ13。作物マスタ＝フェーズ3/6/7。
- 地域別の集計表示・グラフ・CSV 出力＝フェーズ4/13（本フェーズは集計**キー**のみ）。
- 緯度経度・字・番地・字レベル細粒度／都道府県マスタ（沖縄固定）／自由記述補足欄。
- 親市町村の DB 列保持（導出で足りる）。sensor_readings・受信 API・グラフ基盤（E1）の変更。認証・所有者認可・MethodOverride・CSRF の本体（S1/authz 所有・消費のみ）。

## 技術制約・準拠事項

- **マスタ＝Go 定数**（structure.md §100）。テーブル/FK/seed CLI を作らない。`domain` は `fmt` のみ依存（純粋性）。
- **Gin**: `c.ShouldBind` ＋ 地域存在検証は handler 内 procedural（oneof タグは使わず `Locality.Valid()`）。
- **sqlc**: 生成構造体は読取専用。`repository.Querier` ポート経由。依存方向（structure.md §35-57）厳守・view→domain 表示メソッドのみ。
- **templ**: 単一 select に `class="js-tom-select"`（既存 `initTomSelect` に乗る・App.templ 改変なし）。
- **CSRF/MethodOverride**: 既存維持。**新規エンドポイントなし。**
- **CSS**: 独自クラス新設禁止・モック単一ソース（先行反映→`make sync-css`）。
- **マイグレーション**: goose 連番 00008・DDL のみ・後 `make db-snapshot`。
- **言語**: 日本語コメント・エラー・コミット。地域名は日本語。
- **TDD**: 80% 以上。domain（53件・親対応・同名区別・エイリアス）・フォーム描画/復元・procedural 検証・Create/Update・backfill（fakeRepo）・表示。

## 受け入れ基準（概略）

1. **マスタ**: `domain.AllLocalities()` が53件、各地域の親市町村が正しく、同名（具志川市/具志川村）が Label の現市町村併記で区別される。
2. **フォーム**: 設置場所が単一の検索可能 地域 select になり、認識名（「佐敷（南城市）」等）で絞り込み選択できる。
3. **保存・復元**: 選択地域が保存され、編集・エラー再描画で選択が復元される。存在しない地域は弾かれる。
4. **表示**: device-show・dashboard が認識名で所在地を表示（未設定は空）。
5. **移行**: 既存 `location` は非破壊移行（一致→設定・不能→未設定・件数不変）。
6. **規約・snapshot**: マスタはテーブル化せず Go 定数・FK 無し・`devices` 追加は `locality` 1列のみ。00008 後に `make db-snapshot` 反映。
7. **テスト 80% 以上**。

## 未確定事項・要確認（設計フェーズで確定済み・参考）

- ~~地区の粒度・出典・件数~~ → **解消**（旧町村レベル・research.md の確定データ・53件）。
- ~~カスケード実現方式~~ → **解消**（平坦化により不要）。
- 親市町村の DB 列化は本フェーズ非対象（導出）。将来 SQL 集計で非破壊追加。
- `location`（旧自由入力列）は残置（移行元）。撤去は別途。

--- spec-init 本文 ここまで ---
