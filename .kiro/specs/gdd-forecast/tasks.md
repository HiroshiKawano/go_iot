# 実装計画 — gdd-forecast（GDD 積算・収穫予測）

> 逐次（sequential）。番号の昇順がそのまま実装順。各 executable サブタスクは `/tdd` の1サイクル（RED→GREEN→REFACTOR）で完結する単一責務。`(P)` マーカーは付けない。
> 生成依存順（tasks §2.1）: goose migration → `make db-snapshot` → `devices.sql` 改修 → `make sqlc` → templ → 消費実装。モック反映は templ 写経の前。

- [x] 1. Foundation: 定植日スキーマと生成基盤
- [x] 1.1 00010 マイグレーションで定植/播種日列を追加しスナップショットを再生成
  - `db/migrations/00010_add_planting_date_to_devices.sql` で `devices.planting_date`（DATE・nullable・COMMENT「GDD 積算の起点・NULL=未設定」）を expand-contract で追加。down は `DROP COLUMN IF EXISTS`（CHECK・索引は張らない＝自由日付・絞込キーでない）。00008 locality／00009 crop を写経
  - `make migrate-up` で適用後、`make db-snapshot` を実行
  - 観測: `docs/database_snapshot/table_definitions.md` の devices テーブルに `planting_date | date | YES` 行が現れ、既存行は NULL のまま破綻しない
  - _Requirements: 2.2_
- [x] 1.2 devices クエリへ定植日を追加し sqlc を再生成
  - `db/queries/devices.sql` の `CreateDevice`/`UpdateDevice` の列・パラメータに `planting_date`（$8）を追加（SELECT 系は `SELECT *` で自動同期）。その後 `make sqlc`
  - `make sqlc` 実行は `devices.sql` の `$8` 追加が前提（Params 生成のため）
  - 観測: 生成された `repository.Device` に `PlantingDate *pgtype.Date`、`CreateDeviceParams`/`UpdateDeviceParams` に `PlantingDate` が現れ `go build ./...` が通る
  - _Requirements: 2.2_

- [x] 2. 純粋計算層（GDD・累積・残り積算・線形回帰）
- [x] 2.1 最小二乗の線形回帰を追加
  - `internal/chart/stats.go` に `LinearFit(xs, ys []float64) (slope, intercept float64, ok bool)`。x の分散0 または `len<2` で `ok=false`。`math` のみ・入力非破壊
  - 観測: 既知データ点で傾き・切片が手計算値に一致し、x 分散0で `ok=false` を返す table-driven テストが緑
  - _Requirements: 1.4_
- [x] 2.2 日次 GDD・累積・残り積算を追加
  - `internal/chart/gdd.go` 新設に `DailyGDD`（`max((Tmax+Tmin)/2−Tbase, 0)`・負はゼロクランプ）・`CumulativeGDD`（前方累積和・単調非減少・`len(out)==len(daily)`）・`RemainingGDD`（`max(target−最新累積, 0)`・到達済み0・空入力は target）。`math` のみ・time 非依存・入力非破壊
  - 全ゼロ（生育せず）・単一日・氷点下・空・`len(tMax)!=len(tMin)` を数値安全に処理
  - 観測: `(Tmax=30,Tmin=20,Tbase=10)→15`、累積が単調非減少の前方和、到達済みで残り=0、既知手計算ケース一致がテストで緑
  - _Requirements: 1.1, 1.2, 1.3, 1.7, 1.8_
- [x] 2.3 到達日外挿と生育ステージ判定を追加
  - `internal/chart/gdd.go` に `ForecastDaysToTarget(xs, cumulative, target)`（`LinearFit` 利用・傾き≤0／x 分散0（1点）／到達済みで `ok=false`・過去には外挿しない）と `GrowthStageIndex(cumulative, stageGDD)`（昇順しきい値・最終超え・未到達 -1・空 -1）
  - 観測: 通常データで到達経過日が外挿され、傾き≤0・到達済み・1点で `ok=false`、ステージ閾値境界が正しく判定されテストが緑
  - _Requirements: 1.5, 4.2, 4.4_

- [x] 3. ドメイン作物マスタへ GDD モデルを非破壊追加
- [x] 3.1 作物別 Tbase・生育ステージ・収穫目標 GDD を Go 定数で追加
  - `internal/domain/crop.go` に `GrowthStage{Name; GDD}`・`GDDModel{Tbase; Stages}`・`DefaultGDDModel`（Tbase 既定 例10℃）と `func (c Crop) GDDModel() GDDModel`（`DiseaseModel` 同型の switch＋既定フォールバック）。最低1作物（米）に具体値（暫定）。作物集合・並びは `AllCrops()`／`devices.crop` CHECK と一致を維持。型コメントの GDD フックを解消
  - 観測: `CropRice.GDDModel()` が非空 `Stages` と Tbase 具体値、未設定・未定義作物が `DefaultGDDModel` を返し、既存 `VPDRange`/`DiseaseModel` テストが無回帰で緑
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6_

- [x] 4. 描画層（GDD option と目標/予測マーク注入）
- [x] 4.1 GDD チャート入力契約型を追加
  - `internal/chart/series.go` に `GDDChartSpec{ElapsedDays; Cumulative; Color; TargetGDD; ForecastDay; HasForecast}`（別型隔離）
  - 観測: `GDDChartSpec` が定義され、既存 `ChartSpec`/`VPDChartSpec`/`DewpointChartSpec` のテストが無回帰で緑
  - _Requirements: 3.1_
- [x] 4.2 GDD option 生成と目標/予測 markLine・markPoint を自前注入
  - `internal/chart/gdd_echarts.go` 新設に `GDDChartOptionJSON(spec)`（累積曲線 series[0]・**x=経過日数の value 軸**・dataZoom inside/slider）と `injectGDDMarks`（目標 GDD 水平 markLine=小文字 `yAxis`／`HasForecast` 時の予測到達日 垂直 markLine=小文字 `xAxis`＋markPoint=`coord`）。`vpd_echarts.go` の `Validate→Marshal→map→注入→再Marshal` を写経し `SetEscapeHTML=true` で HTML 安全化
  - 観測: 生成 JSON に小文字 `yAxis`/`xAxis` キーの markLine が目標/予測の本数どおり含まれ、`HasForecast=false` で予測マークが出ず、`</script>` 不混入がテストで緑
  - _Requirements: 3.1, 3.2, 3.3, 3.6, 3.7_

- [x] 5. View 層（DTO・モック・templ・connect 除外）
- [x] 5.1 GDD パネル View DTO と定植日フォーム DTO を追加
  - `internal/view/component/views.go` に `GDDPanelView`（OptionJSON/Color/CropLabel/Card/Stages/Guidance/Note）・`GDDCardView`（Cumulative/Remaining/ForecastDate/Stage/ElapsedDays）・`GrowthStageRow`（Name/GDD/Current）を整形済み string primitive で追加。`DeviceFormView` に `PlantingDate string` を追加
  - 観測: DTO が定義され templ から参照可能、既存 `views.go` 利用テストが無回帰で緑
  - _Requirements: 3.4, 4.1, 4.2_
- [x] 5.2 モックへ GDD パネル器・定植日入力・色トークンを反映
  - `mocks/html/device-show.html` の露点パネル後に GDD パネル器（`#gdd-chart`・`.summary-grid-4` カード・生育ステージ `.data-table`）を追加。`mocks/html/device-create.html`/`device-edit.html` の作物 select 隣に定植日 `<input type="date">`。`mocks/html/style.css` の `:root` に `--color-gdd`（暖色・`--color-vpd`/`--color-dewpoint` と区別）を追加。`make sync-css` 実行。グラフ内部の累積曲線/外挿/markLine は動的描画ゆえモック反映の例外
  - 観測: `mocks/html/device-show.html` に `#gdd-chart` と生育ステージ表の器が存在し、`style.css` の `--color-gdd` が `--color-vpd`(#0ca678)/`--color-dewpoint`(#4263eb) と異なる暖色値で定義され、`make sync-css` で本番 `internal/view/public/css/style.css` に反映される
  - _Requirements: 3.5, 4.1, 4.2, 2.1_
- [x] 5.3 GDD パネル templ をページへ結線し connect から除外
  - `gddPanel` templ（モック写経・`#gdd-chart data-echarts data-no-connect data-color data-unit="℃·日"`・`optionScript`・GDD カード・生育ステージ表・`Guidance` 非空時は注記のみ／`Note` 近似注記表示）を追加。`DeviceShow.templ` で `@DeviceChartArea` の**後ろ**に `@gddPanel(v.GDD)`（period 非連動の兄弟）。`App.templ` の `initScope` の `echarts.connect` 収集対象から `[data-no-connect]` を除外（init/dispose は全 `[data-echarts]` のまま・後方互換）
  - 観測: DeviceShow ページ templ 描画に `#gdd-chart` が現れ `data-no-connect` が付与され、既存4チャート（温度/湿度/VPD/露点）の connect が従来どおり機能することがテストで緑
  - _Requirements: 3.4, 4.1, 4.2, 7.4_
- [x] 5.4 デバイスフォーム templ に定植日入力を追加
  - `DeviceForm.templ` の作物 select 隣に `<input type="date" name="planting_date" value={...}>`（任意・空可・編集時プリセット・バリデーションエラー時の入力値復元）。Tom Select 非対象の素の input
  - 観測: device 登録/編集フォーム templ 描画に `planting_date` の date input が現れ、保存値が `value` でプリセットされる
  - _Requirements: 2.1, 2.3, 2.4_

- [x] 6. Handler 結線（GDD パネル・ページ経路・定植日フォーム）
- [x] 6.1 GDD パネル組立の正常系を実装（period 非連動）
  - `internal/handler/device_show_gdd.go` 新設に `buildGDDPanel(ctx, device, now)`（写経 `buildVPDPanel`）。`crop.GDDModel()`＋`device.PlantingDate` から `ListDailySensorAggregates(since=定植日00:00 JST)` を**再利用**取得→present 日の `max/min_temperature` と経過日数（`ReadingDate`−定植日）を算出→純粋層で日次 GDD→累積→残り積算→到達日外挿→生育ステージ→`GDDChartOptionJSON`→`GDDPanelView`。period 引数を取らず定植日→現在で固定。時刻バケットは handler 境界・純粋層へは `[]float64`/スカラのみ
  - 観測: `Querier` モックで米＋定植日＋日次データを与えると累積 GDD・残り積算・予測到達日・現在ステージ・経過日数が具体値で埋まった `GDDPanelView`（`OptionJSON` 非空）が返ることがテストで緑
  - _Requirements: 1.6, 2.7, 4.1_
  - _Boundary: buildGDDPanel_
- [x] 6.2 GDD パネル組立の縮退系を実装（前提欠落・予測不能）
  - `buildGDDPanel` に分岐を追加: Tbase を持つ作物でない／`planting_date` NULL → `Guidance`（「作物と定植日を設定すると GDD を表示します」）付き空パネル。定植日が未来日 → 未開始注記。定植日以降データ0件 → データ未到着。予測不能（傾き≤0・データ不足・到達済み）→ 予測カード "—"＋理由注記。いずれも `error` を返さず縮退
  - 観測: crop 未設定 or planting_date NULL で `Guidance` 非空・`OptionJSON` 空、空データで未到着表示、予測不能で予測カード "—"＋注記が返ることがテストで緑
  - _Requirements: 4.3, 6.3, 6.4_
  - _Boundary: buildGDDPanel_
- [x] 6.3 ページ経路 Show へ GDD パネルを period 非連動で結線
  - `Show`（GET /devices/{device} フルページ）で `buildGDDPanel(ctx, device, now)` を呼びページ ViewModel へ `GDDPanelView` を渡す。期間フラグメント `Chart`/`buildChartArea`（GET /devices/{device}/chart）は**無改修**。所有者認可は既存 `RequireDeviceOwner`（非所有/不在→404 列挙防止）を踏襲。GDD は表示のみ＝アラート判定/通知を発火せず農家向け平易表示・共有 URL・他指標を含めない
  - 観測: GET /devices/{id} の HTML に GDD パネルが現れ、`?period=24h` と `?period=30d` で GDD パネル部分が同一（period 非連動）、非所有 device で 404 がテストで緑
  - _Requirements: 6.1, 6.2, 8.1, 8.2, 8.3, 8.4, 8.5_
  - _Boundary: Show, buildChartArea_
- [x] 6.4 定植日フォームのバインド・検証・保存・復元
  - device 登録/更新ハンドラ（`device.go`/`device_form.go`）でフォーム struct と `DeviceFormView` に `PlantingDate` を結線。`time.Parse("2006-01-02", ...)` で形式検証・**未来日（>今日 JST）は field error「定植日は未来日にできません」で同フォーム再描画**・空→NULL（`*pgtype.Date` nil）・編集時は既存値を `"YYYY-MM-DD"` でプリセット。1サイクル分の単一列のみ保持
  - 観測: 正常な定植日が保存され編集で復元、未来日・不正形式で field error 付き同フォーム再描画（入力値保持）、空で NULL 保存されることがテストで緑
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6_
  - _Boundary: device create/update handler, DeviceForm_

- [x] 7. 統合・無回帰検証
- [x] 7.1 GDD 機能の統合テスト（重要フロー）
  - `httptest`+gin で、定植日を設定した米の device の GET /devices/{id} に GDD 累積曲線（`#gdd-chart`）・予測到達 markLine・残り積算/予測収穫日/生育ステージのカード・生育ステージ表が描画されることを templ `Render`→`strings.Contains` で検証。期間切替（24h↔30d）で GDD パネル部分が変化しない（period 非連動）。crop/定植日 欠落で導線注記、空データで未到着。`Querier` モックで DB 非依存
  - 観測: 定植日設定済み device の GET で GDD パネル要素群が描画され、24h↔30d 切替で GDD 部分の HTML が不変、前提欠落で導線注記が出ることがテストで緑
  - _Requirements: 2.1, 2.2, 2.3, 3.1, 3.2, 3.3, 4.1, 6.1, 6.2, 6.3, 6.4_
  - _Depends: 6.3, 6.4_
- [x] 7.2 既存可視化の無回帰テスト
  - 温湿度2グラフ・統計オーバーレイ（移動平均/正常帯/乖離率）・VPD パネル・露点パネル・欠測ギャップ・品質メタ・期間切替/URL 同期・空データ表示が従来どおり動作することを既存テスト群で確認。既存4チャート（温度/湿度/VPD/露点）の `echarts.connect` 連動が維持され、GDD チャートは `data-no-connect` で connect に参加しないことを検証
  - 観測: 既存 device-show テスト群が全緑のまま、GDD チャートが既存4チャートの connect グループに含まれないことがテストで緑
  - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5_
  - _Depends: 5.3, 6.3_
