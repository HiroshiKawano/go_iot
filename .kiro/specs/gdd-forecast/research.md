# ギャップ分析 — gdd-forecast（GDD 積算・収穫予測）

> 作成: 2026-06-29 ／ `/kiro-validate-gap gdd-forecast`
> 対象: 実装済み device-show（S5/E1/P2/P3/P5/P6）への「上載せ」フェーズ。並行調査4本（domain/純粋層・chart/echarts・handler/view/templ・スキーマ/migration/モック）の実コード確認に基づく。
> 方針: **情報提供であって決定ではない**。design で確定する論点には選択肢と trade-off を提示する。

## 分析サマリ

- **既存パターンへの素直な延長が大半**: VPD（P3）・露点（P6）が「純粋層（`internal/chart`）→別パネル handler（`build*Panel`）→`*PanelView` DTO→templ ブロック→モック器」の写経テンプレを確立済み。GDD はこの第3弾で、**写経元が全レイヤに揃っている**（後述マップ）。`crop.go` には GDD フックコメントが**現存**（逐語: 「GDD 基準温度・病害モデル等の他属性は別フェーズが非破壊的に追加する前提。」）。
- **真の新規＝3点のみ**: (1) **定植日アンカーの永続化**（`devices.planting_date`・00010・P2〜P6 に無い唯一の DDL）、(2) **線形回帰 `LinearFit`**（`stats.go` に不在）、(3) **markLine/markPoint 自前注入**（既存は markArea のみ＝新規拡張）。
- **最重要の統合摩擦＝GDD の期間非連動**: 既存パネルは選択期間の生行を流用するが、GDD は定植日→現在の全期間を要する。**`buildChartArea` の期間限定データを使えず、独立データ取得経路が必要**。長期集計の取得方式（Go 側 JST バケット／SQL 集計新設）と JST 暦日の正当性が design の核。
- **スキーマ余地は確認済み**: 最新 goose 連番 = **00009**、00010 で `planting_date DATE NULL` を expand-contract 追加（00008/00009 が写経元）。**`DATE → pgtype.Date` は実績あり**（`ListDailySensorAggregates.ReadingDate`）→ `PlantingDate *pgtype.Date`（`emit_pointers_for_null_types: true`）。
- **工数概算 = M（3〜7日）／リスク = Medium**: 純粋層・DTO・templ・フォーム・migration は Low（写経）。期間非連動データ経路と markLine 注入が Medium（新パターン）。

---

## 1. 現状コードベース調査結果（写経元の所在）

### 1.1 domain / 純粋計算層（`internal/domain`・`internal/chart`）

| 資産 | 実態 | GDD での扱い |
|---|---|---|
| `domain/crop.go` | `type Crop`＋9作物（Goya/Ingen/Sugarcane/Mango/Pineapple/Uri/Rice/Imo/LeafyVegetable）。`Label()`/`Valid()`/`VPDRange() (lower,upper float64)`/`DiseaseModel() DiseaseModel`/`AllCrops() []Crop`。switch グルーピング＋既定フォールバック作法。**GDD フックコメント現存**（L13） | `GDDModel struct{Tbase float64; Stages []GrowthStage}`＋`func (c Crop) GDDModel() GDDModel` を `DiseaseModel` と同型で非破壊追加。`DefaultGDDModel` フォールバック。フックコメントを解消する当事者 |
| `chart/stats.go` | `SMA`/`MovingStdDev`/`Band`/`Deviation`/`Mean`/`MinMax`/`DiurnalRange`/`StdDev`/`CV`。内部 `windowSlice`/`mean`/`popStdDev`。**import は `math` のみ・time 非依存**。**`LinearFit` は不在** | `LinearFit(xs, ys []float64) (slope, intercept float64, ok bool)`（最小二乗）を新設。x 分散0で `ok=false` |
| `chart/vpd.go` | `VPD`/`VPDSeries`/`TimeInRange`、Tetens 定数局所定義、RH クランプ、ゼロ境界対応 | GDD 純関数の純粋性・クランプ作法の手本 |
| `chart/dewpoint.go` | `DewPoint`/`DewPointSeries`/`DewPointSpread`（負値→0クランプ）/`CondensationRuns`/`WetnessMask`/`HighHumidityRuns`/`DiseaseScore`。内部 `runsFromMask`、`Run{StartIdx,EndIdx}` 型 | **新設 `chart/gdd.go` の直近の手本**。`DailyGDD`/`CumulativeGDD`/`RemainingGDD`/`ForecastDaysToTarget`/`GrowthStageIndex` を同じ純粋性で |

### 1.2 chart 描画層（ECharts option 生成）

| 資産 | 実態 | GDD での扱い |
|---|---|---|
| `chart/series.go` | `ChartSpec`（温湿度・SMA/Band/Deviation/RawNullable/GapBands）・`VPDChartSpec`（VPD/SMA/Lower/Upper/YMax）・`DewpointChartSpec`（Dewpoint/Temperature/Condensation []Run）。**各 Spec 独立型で無回帰隔離** | `GDDChartSpec` を別型新設（Labels/Color/Cumulative []float64/TargetGDD float64/ForecastIndex 等） |
| `chart/vpd_echarts.go` | `VPDChartOptionJSON(spec) (string,error)`＝go-echarts で line 構築→`Validate()`→`JSON()`→`json.Marshal`→`json.Unmarshal`(map)→`injectVPDMarkArea`(**小文字 `yAxis` キー自前注入**)→再 Marshal。`json.Marshal` 既定 `SetEscapeHTML=true` で `<>&` を `\uXXXX` 化（`</script>` 混入なし） | `GDDChartOptionJSON` の写経元。go-echarts の `MarkAreaData.YAxis`（大文字 JSON タグ）が ECharts 非準拠ゆえ自前マップ注入する理由も同じ |
| `chart/gap_echarts.go` | `injectGapMarkArea(optionJSON, []GapBand)`＝**小文字 `xAxis` 範囲キー**で series[0] へ注入・空 bands は原文返却（後方互換）。`nullableLineData([]*float64)`＝nil で線分断 | xAxis 範囲注入の手本 |
| `chart/dewpoint_echarts.go` | `DewpointChartOptionJSON`＝VPD 写経。`injectCondensationMarkArea`（xAxis 範囲 markArea・寒色） | パネル option の最新写経元 |
| **markLine/markPoint 注入** | **存在しない（既存は markArea のみ）** | ★**新規拡張**。`injectGDDMarkLine`（目標 GDD 水平＝小文字 `yAxis`／予測到達日 垂直＝小文字 `xAxis`）を markArea パターンから類推して起こす |

### 1.3 handler / view / templ

| 資産 | 実態（シグネチャ・行） | GDD での扱い |
|---|---|---|
| `handler/device_show.go::buildChartArea(ctx, device, period, now) (DeviceChartAreaView, error)` | 生行→`pgconv.NumericToFloat`→ラベル→温湿度 option→欠測ギャップ→`buildVPDPanel`/`buildDewpointPanel` を独立呼出→View へ詰める。`jst=FixedZone(L52)`・`statEmptyMark="—"(L229)`・`deviceCrop(device)(L440)` | `buildGDDPanel(...)` を呼び `DeviceChartAreaView` 末尾へ非破壊追加。**ただし period 引数の期間窓とは独立に定植日→現在を集計**（後述・最重要論点） |
| `dailyStatRows(rows, pick)(L492)`／`jstDay(pgtype.Timestamptz)(L552)` | **Go 側で JST 暦日 map バケット＋欠測日補完**（`emptyDailyRow`）。**P2 日次表示は SQL でなくこの Go 集計** | GDD 日次（最高/最低気温）の JST バケット写経元。**SQL の `DATE()` TZ 問題を回避する既定路線** |
| `handler/device_show_vpd.go::buildVPDPanel(labels,temps,hums,rows,crop,period)` | `crop.VPDRange()`→`VPDSeries`→空チェック`emptyVPDCard()`→`TimeInRange`→`VPDChartOptionJSON`→`vpdHourlyRows`（JST hour バケット）。`formatVPD`/`formatPercent` | `buildGDDPanel` の主写経元 |
| `handler/device_show_dewpoint.go::buildDewpointPanel(...,now)` | `crop.DiseaseModel()`→系列→`CondensationRuns`/`WetnessMask`→option→`dewpointDailyRows`（`maxWetGap` キャップ）→`highHumidityEventRows`。`Note`＝近似注記 | 別パネル＋日次表＋注記の写経元（GDD の予測近似注記もここに倣う） |
| `view/component/views.go::DeviceChartAreaView` | `...VPD VPDPanelView; Dewpoint DewpointPanelView; HasGap bool`。`VPDPanelView`/`DewpointPanelView`/各 `*CardView`/行 DTO は**整形済み string primitive のみ**（pgtype 非持込） | `GDD GDDPanelView` を末尾追加。`GDDPanelView`/`GDDCardView`/`GrowthStageRow`/`GDDDailyRow` を string DTO で |
| `view/component/DeviceChartArea.templ` | 期間セレクタ（`hx-get /devices/{id}/chart?period=X`・`hx-push-url /devices/{id}?period=X`）。`if v.HasData { charts・@dailyTable・@vpdPanel・if Dewpoint.OptionJSON!="" @dewpointPanel } else placeholder`。`id="vpd-chart"`/`id="dewpoint-chart"`・`data-echarts data-unit data-color` | 露点パネル後に `@gddPanel(v.GDD)`。`id="gdd-chart"`。**期間フラグメント再描画との関係は論点④** |
| `App.templ EChartsInitializer` | `[data-echarts]` 走査→`echarts.init`/`echarts.connect` | GDD line も自動 init。**connect 参加は論点⑦（軸スケール別ゆえ除外が自然か）** |
| `view/component/DeviceForm.templ`＋`handler/device.go`/`device_form.go` | `DeviceFormView{...Crop string; Crops []SelectOption; Errors map}`。crop は `js-tom-select` select。handler は `ShouldBind`→validate→`reRenderCreate`／編集は `ShowEditForm` プリセット。`nullableStr(s)`（空→nil）・`deviceCropValue(d)`・`cropOptions(selected)` | `PlantingDate string` を DTO へ追加。`<input type="date" name="planting_date">` を crop 下へ。handler で `time.Parse("2006-01-02", ...)`→`pgtype.Date`、空可・未来日検証 |

### 1.4 スキーマ / クエリ / sqlc / モック

| 資産 | 実態 | GDD での扱い |
|---|---|---|
| `db/migrations/` | 最新 = **00009**（00008=locality / 00009=crop）。expand-contract（列追加＋CHECK＋索引＋COMMENT、down は `DROP COLUMN IF EXISTS`） | **00010 `add_planting_date_to_devices.sql`**＝`planting_date DATE NULL`＋COMMENT。**CHECK/索引は不要**（自由日付・絞込キーでない）。down は DROP。`make db-snapshot` 再生成 |
| `db/queries/devices.sql` | `GetDevice`/`ListDevicesByUser` は `SELECT *`（**列は自動同期**）。`CreateDevice`($6=locality,$7=crop)／`UpdateDevice` はパラメータ明示 | INSERT/UPDATE に `planting_date`=$8 を明示追加。SELECT は自動 |
| `db/queries/sensor_readings.sql` | **`ListDailySensorAggregates`(L29-45) が既存**＝`GROUP BY DATE(recorded_at)` で avg/max/min temp+humidity+sample_count・`ReadingDate pgtype.Date`。`ListSensorReadingsInRange`(L78-85)=BETWEEN/ASC/LIMIT なし（P4） | GDD 日次気温の取得候補。**ただし `DATE()` はサーバ TZ＝JST 非保証**（論点②）。`AT TIME ZONE 'Asia/Tokyo'` 版の新設 or 全行取得＋Go 集計が design 判断 |
| `sqlc.yaml`／`internal/repository/models.go` | PostgreSQL・`emit_pointers_for_null_types: true`。`DATE → pgtype.Date` 実績（`ReadingDate`）。NULL 列は `*T` | `Device.PlantingDate *pgtype.Date` が生成される見込み（locality/crop の `*string` と同パターン） |
| `mocks/html/*` | `device-show.html`: `id="vpd-chart"`/`id="dewpoint-chart"`・`.summary-grid-4`・`.data-table`。`device-create/edit.html`: crop select（`js-tom-select`・空 option＋9値）。`style.css :root`(L60-73): `--color-vpd:#0ca678`・`--color-dewpoint:#4263eb`、**`--color-gdd` 未定義** | GDD パネル器（`id="gdd-chart"`・`.summary-grid-4`・生育ステージ表 `.data-table`）を露点後に。定植日 input を create/edit に。`--color-gdd`（暖色系）追加→`make sync-css` |

---

## 2. Requirement → 既存資産マッピング（ギャップタグ）

タグ: **[再利用]** 既存資産で充足 ／ **[拡張]** 既存を非破壊拡張 ／ **[新規]** 新ファイル/関数 ／ **[制約]** 既存制約に従う ／ **[未確定]** design 論点

| 要件 | 写経元/資産 | ギャップ |
|---|---|---|
| **R1** GDD/累積/残り積算/到達日算出 | `chart/dewpoint.go` 純粋作法・`stats.go` | **[新規]** `chart/gdd.go`（DailyGDD/CumulativeGDD/RemainingGDD/ForecastDaysToTarget/GrowthStageIndex）＋**[拡張]** `stats.go::LinearFit` |
| **R2** 定植日入力・保存・復元 | `DeviceForm.templ`/`device.go`/00008-9 migration | **[新規]** 00010 `planting_date`・**[拡張]** devices.sql INSERT/UPDATE・DeviceForm input・`PlantingDate` DTO／**[制約]** `pgtype.Date`・`nullableStr` 相当・**[未確定]** 未来日検証の扱い |
| **R3** 累積曲線＋目標線＋予測マーク | `vpd_echarts.go`/`series.go` | **[新規]** `GDDChartSpec`・`gdd_echarts.go`・**[新規]** markLine/markPoint 注入（既存 markArea から類推）／**[未確定]** dataZoom 採否 |
| **R4** 残り積算/予測収穫日/ステージ カード・表 | `VPDPanelView`/`dailyTable`/`*CardView` | **[拡張]** `GDDPanelView`/`GDDCardView`/`GrowthStageRow`・templ 表 |
| **R5** 作物別 Tbase・生育ステージ | `crop.go::DiseaseModel` 作法・GDD フックコメント | **[拡張]** `GDDModel()`＋`DefaultGDDModel`／**[未確定]** Tbase・収穫目標・ステージ閾値の**具体値**（最低1作物＝米）＝research |
| **R6** 期間非連動・前提欠落フォールバック | `buildChartArea`・`emptyVPDCard` | ★**[未確定/新規]** 定植日→現在の独立データ取得経路（論点②）／**[再利用]** 空パネル分岐作法 |
| **R7** 既存可視化の無回帰 | `DeviceChartAreaView` 独立フィールド・templ 独立ブロック | **[制約]** 末尾非破壊追加・既存 Spec 無改変。テストで無回帰固定 |
| **R8** 所有者認可・研究用スコープ境界 | `internal/authz::RequireDeviceOwner`・既存 device-show 認可 | **[再利用]** 認可は device-show が所有（GDD は消費）。非所有→404 |

**ギャップ集約**: 真の新規は (a) `chart/gdd.go`＋`LinearFit`、(b) `GDDChartSpec`＋`gdd_echarts.go`＋markLine 注入、(c) 00010 `planting_date`、(d) 期間非連動データ経路。残りは全レイヤに写経元あり。

---

## 3. 実装アプローチ選択肢（design 論点ごと）

### 論点①（最重要・spec-init 未確定①）定植日アンカーの保持方式

- **Option A（推奨）: `devices.planting_date` 単一 nullable 列（00010）**。P1 locality・P3 crop と完全同型。`pgtype.Date` 実績あり。現在進行中の1サイクルに限定。
  - ✅ 安定アンカー・リロードで消えない・写経元が揃う・最小 DDL。 ❌ 1サイクルのみ（作型比較は別 spec）。
- **Option B: 非永続クエリパラメータ/フォーム（URL 同期・保存しない）**。migration 不要。
  - ✅ DDL ゼロ。 ❌ **リロードで消え累積/残り積算/到達日が不安定＝GDD の前提を壊す（非推奨）**。
- **Option C: `crop_cycles` テーブル（device_id・作物・定植日・収穫日…）**。複数サイクル作型比較が可能。
  - ✅ 作型比較・収穫実績突合へ拡張可。 ❌ 重い・外部キー非採用方針との整合・本フェーズ要件を超過（既定スコープ外）。
- **推奨**: **Option A**。作型比較・収穫実績は別 spec へ defer（要件 Out of scope と一致）。

### 論点②（最重要・spec-init 未確定③・統合の核）GDD 用日次気温データの取得（期間非連動）

**問題**: 既存 `buildChartArea` は選択期間（24h-30d）の生行のみを持つ。GDD は定植日→現在（数十〜百数十日）を要し、**この生行を流用できない**。かつ既存の日次表示は Go 側 `dailyStatRows`（JST バケット）で、SQL `DATE()` は使っていない（TZ 非 JST のため）。

- **Option A: SQL 日次集計クエリ新設（JST 明示）**。`ListDailyTempAggregatesSince(device_id, since)`＝`GROUP BY (recorded_at AT TIME ZONE 'Asia/Tokyo')::date` で日次 max/min temp を返す。
  - ✅ 長期×多数行（~100日×288/日≒2.9万行）を DB 側で集約＝転送/メモリ最小。JST を SQL で明示し正当。 ❌ クエリ新規・既存 `ListDailySensorAggregates`(`DATE()`)とは別物（混同注意）。
- **Option B: `ListSensorReadingsInRange`(全行)＋Go 側 JST 集計（`dailyStatRows` 流用）**。
  - ✅ 既存クエリ＋既存 JST バケット作法の再利用・TZ 作法が handler に統一。 ❌ 定植日→現在の全生行をアプリへロード（長期で重い）。
- **Option C: 既存 `ListDailySensorAggregates` をそのまま流用**。
  - ✅ 追加クエリ不要。 ❌ **`DATE(recorded_at)` がサーバ TZ＝JST 暦日とズレうる**（日界の気温が前/翌日に混入し GDD が日単位でずれる）。本番 TZ 次第で誤り＝**非推奨（採るなら TZ を JST 固定する保証が前提）**。
- **推奨**: **Option A（JST 明示の日次集計新設）**。長期前提＋JST 正当性の両立。`since` は `planting_date`。Go 側は `pgtype.Date` 列を経過日数（`since` からの差）へ写すだけ（時刻境界は handler）。**※ design で本番 PostgreSQL の `timezone` 設定と `DATE()` 既存挙動を1点確認（Research Needed）。**

### 論点③（spec-init 未確定なし・新規パターン）markLine/markPoint 描画

- **Option A（推奨）: markArea 注入の写経で `injectGDDMarkLine` 新設**。目標 GDD＝小文字 `yAxis` 水平線、予測到達日＝小文字 `xAxis` 垂直線/点。`vpd_echarts.go`/`gap_echarts.go` の「option→map→series へ ECharts 準拠キー注入→再 Marshal」を踏襲。
  - ✅ 確立パターンの延長・HTML 安全 JSON 不変条件維持。 ❌ markLine は markArea と微妙にキー構造が違うため初回検証要（テストで小文字キー・本数固定）。
- **Option B: 予測外挿線を第2 series（破線 LineData）として足す**。markLine を使わず series で表現。
  - ✅ go-echarts の series API で完結（注入不要）。 ❌ 目標 GDD 水平線は series 化しにくく結局 markLine が要る。混在より統一が良い。
- **推奨**: **Option A**。目標線・予測到達は markLine/markPoint に統一。クラッタ回避（§2-2）で主役＝累積＋目標＋予測の最小本数、生育ステージ閾値は markLine 群にせず**表**で見せる。

### 論点④（spec-init 未確定⑦）GDD パネルの配置と HTMX 期間フラグメント

既存の期間セレクタは `hx-get /devices/{id}/chart?period=X` で **DeviceChartArea フラグメント全体を差し替える**。GDD を同フラグメント内に置くと**期間切替のたびに GDD を再計算**（定植日→現在は不変なので無駄だが冪等で無害）。

- **Option A: GDD を DeviceChartArea 内（露点の後）に置く**。templ 構造が最も単純・写経どおり。
  - ✅ 写経が素直・1フラグメント。 ❌ 期間切替ごとに GDD 再計算（長期集計を毎回）＝コスト。
- **Option B: GDD を期間フラグメント外（DeviceShow ページ直下の別ブロック）に置く**。期間切替で再描画されない。
  - ✅ 期間と無関係に1回だけ計算。 ❌ templ 配置が DeviceChartArea 写経から外れる・初期表示経路の調整要。
- **推奨**: design で計測コスト次第。**まず Option A（写経優先・冪等）**で実装し、長期集計が重ければ Option B か結果キャッシュへ。要件 R6.2（期間切替で範囲を縮めない）はどちらでも満たせる（GDD は常に定植日→現在で計算）。

### 論点⑤（spec-init 未確定②）Tbase・生育ステージ・収穫目標 GDD の値

- `GDDModel()` の**作法は確定**（`DiseaseModel` 同型・switch＋`DefaultGDDModel`・既定 Tbase 例 10℃）。**値の確定は research/ユーザー権威**（沖縄実地・前職の糖業課でサトウキビ実務）。
- **下限**: 最低1作物（**米**＝二期作の出穂予測で GDD の教科書作物）で Tbase・生育ステージ・収穫目標を具体値化し描画。他作物は既定フォールバック or 段階拡張。
- **Research Needed**: 米（Tbase≒10℃・出穂/登熟の積算目安）の文献値、サトウキビの Tbase・収穫目標。1メソッド更新で確定する作法ゆえ後追い可。

### 論点⑥（spec-init 未確定④⑤⑥）算法の細部

- **modified GDD 上限キャップ**: 既定は単純 GDD（下限ゼロクランプのみ）。沖縄夏季の高温で上限超過があり得るが、上限キャップは将来（要件 Out of scope）。
- **回帰の窓**: 全累積 vs 直近 N 日。既定は全累積（単純）。直近窓は P15 級の精緻化＝将来。
- **予測不能 UI 文言**: 傾き≤0・データ不足・到達済み・未来定植日 → カード "—"＋理由注記（露点 `Note` 作法の流用）。

---

## 4. 実装複雑度・リスク

| 領域 | 工数 | リスク | 根拠 |
|---|---|---|---|
| `chart/gdd.go`＋`stats.go::LinearFit`（純粋層） | S | Low | `dewpoint.go` 写経・`math` のみ・TDD 容易（手計算一致） |
| `domain/crop.go::GDDModel`（値1作物） | S | Low | `DiseaseModel` 同型・フックコメント解消・値は1作物で下限充足 |
| 00010 `planting_date`＋devices.sql＋sqlc | S | Low | 00008/00009 写経・`pgtype.Date` 実績・SELECT は自動同期 |
| `DeviceForm` 定植日＋device handler バインド | S | Low | crop 追加と同型・`nullableStr`/プリセット作法あり |
| `GDDChartSpec`＋`gdd_echarts.go`＋**markLine 注入** | M | Medium | markLine 注入は**新パターン**（既存は markArea のみ）・小文字キー検証要 |
| **期間非連動データ経路**（論点②）＋日次集計クエリ新設 | M | Medium | JST 集計 SQL 新設＋`buildChartArea` から period 非依存に集計＝既存フローと別経路 |
| `GDDPanelView`/templ/モック反映 | S | Low | `VPDPanelView`/`vpdPanel`/モック器 写経・`--color-gdd` 追加 |
| 無回帰（P2-P6）＋所有者認可 | S | Low | 末尾非破壊追加・認可は device-show 所有（消費のみ） |
| **総合** | **M（3〜7日）** | **Medium** | 律速は markLine 注入と期間非連動データ経路の2点。他は写経で Low |

---

## 5. design フェーズへの引き継ぎ

### 推奨アプローチ（Hybrid＝拡張＋新規の組合せ）
- **拡張**: `crop.go`（GDDModel）・`stats.go`（LinearFit）・`series.go`（GDDChartSpec）・`views.go`（GDDPanelView 末尾追加）・`device_show.go`（buildGDDPanel 呼出）・`devices.sql`・`DeviceForm`・device handler・モック。
- **新規**: `chart/gdd.go`・`chart/gdd_echarts.go`（markLine 注入含む）・`handler/device_show_gdd.go`・00010 migration・GDD 用日次集計クエリ。
- いずれも**末尾/隣接への非破壊追加**で既存 Spec/Panel/View を無改変＝無回帰を構造的に担保。

### 確定済み（design で再検討不要）
- 定植日 = **`devices.planting_date` 単一 nullable 列（00010）**（論点① Option A）。作型比較・収穫実績は別 spec defer。
- markLine/markPoint = **markArea 写経で自前注入**（論点③ Option A）。
- 純粋層 = `chart/gdd.go`＋`stats.go::LinearFit`（time 非依存・`[]float64`）。日次バケット・経過日数換算は handler 境界。
- `GDDModel()` 作法 = `DiseaseModel` 同型・既定フォールバック。最低1作物（米）で具体値。

### Research Needed（design / 実装で確認）
1. **本番 PostgreSQL の `timezone` 設定と既存 `ListDailySensorAggregates` の `DATE()` 挙動**（JST かどうか）＝論点②の前提。JST 非保証なら GDD 集計は `AT TIME ZONE 'Asia/Tokyo'` 明示の新クエリ一択。
2. **米・サトウキビ等の Tbase・収穫目標 GDD・生育ステージ閾値の具体値**（文献＋ユーザー権威）。本フェーズは米で下限充足。
3. **GDD パネル配置**（論点④ A/B）＝長期集計の計測コスト次第。まず写経優先の A、重ければ B/キャッシュ。
4. **`echarts.connect` 参加可否**（論点⑦）＝GDD は累積℃·日でスケール/x ラベルが温湿度と別ゆえ**除外が自然**（design で確認）。
5. **未来定植日・予測不能ケースの UI 文言**（"—"＋理由）と定植日バリデーション（未来日拒否 or 未開始表示）。

### テスト方針（design の Testing Strategy へ）
- 純粋層: 日次 GDD（正負/ゼロクランプ/Tbase 境界）・累積（単調増加/前方和/空）・残り積算（到達前/到達済み=0）・`LinearFit`（手計算一致/x 分散0で ok=false/傾き0・負）・到達日外挿（傾き≤0/到達済み/通常）・ステージ判定（閾値境界/最終超え/空）。
- 注入: GDD markLine（小文字キー・目標/予測の本数）・HTML 安全 JSON 不変条件。
- domain: `GDDModel`（米で具体値・未定義で既定・Tbase 既定）。
- handler/統合: planting_date バインド（空可/不正日付/未来日）・期間非連動（period を変えても定植日→現在）・前提欠落空パネル・所有者認可（非所有→404）。
- **無回帰**: P3 VPD・P6 露点・P5 欠測ギャップ・P2 統計オーバーレイ・温湿度2グラフ・期間切替/URL 同期/connect。
- **実機スモーク**（GO 後）: GDD 配色（暖色）・予測マークの向き・近似注記の表示（テストが符号化できないドメイン意味の確認＝VPD 物理規約の教訓）。

---

## 設計シンセシス（design フェーズ確定・2026-06-29）

`/kiro-spec-design` の discovery（light・Extension）と実コード精読で、ギャップ分析の論点を以下のとおり確定した。Generalization / Build-vs-Adopt / Simplification の3レンズを適用。

### 確定1（論点②の解消・最重要）: 日次気温は既存 `ListDailySensorAggregates` を再利用＝新クエリ不要
- **発見**: `docker-compose.yml:9 TZ: Asia/Tokyo`、かつ `internal/handler/device_show.go:50-51` が「日次集計の日付バケット（`DATE(recorded_at)`）は DB セッション TZ 依存・**接続 TZ=Asia/Tokyo を前提**（Out of Boundary）」と既に明記。
- **帰結**: 既存 `ListDailySensorAggregates`（`GROUP BY DATE(recorded_at)`・`max/min_temperature` を返す）は**接続 TZ=JST 前提で既に JST 暦日バケット**。GDD 用に `AT TIME ZONE 'Asia/Tokyo'` の新クエリを起こすと既存集計と二重方式になり不整合。
- **決定（Build vs Adopt / Simplification）**: GDD 日次気温は `ListDailySensorAggregates(device_id, since=定植日00:00)` を**そのまま再利用**。`sensor_readings` への新規 SELECT はゼロ。GDD は既存の「接続 TZ=JST 前提」を継承（再設計しない＝Out of Boundary）。→ research §3 論点② の旧推奨（新 JST クエリ）を**再利用に上書き**。

### 確定2（論点④の解消）: GDD は期間フラグメント**外**（ページ描画経路 `Show`）に配置
- **発見**: 既存パネル（VPD/露点）は `buildChartArea`（期間フラグメント `Chart`）内で `if v.HasData`（**選択期間窓**のデータ有無）に gating されて描画される。`App.templ` の `initScope` は htmx swap 後に**フラグメント scope 内の `[data-echarts]` のみ** init/dispose。
- **問題**: GDD を期間フラグメント内（写経 Option A）に置くと、(a) period 窓が空でも planting→now にデータがあれば GDD を見せたい要件（6.x）に対し period の `HasData` gating が誤る、(b) period 切替のたびに GDD（長期集計）を再計算。
- **決定（Simplification / 正しさ優先）**: GDD パネルは `Show`（GET /devices/{id} フルページ）が `buildGDDPanel(ctx, device, now)` で組み、`DeviceShow.templ` で `@DeviceChartArea` の**兄弟**として描画。期間フラグメント `Chart`/`buildChartArea` は**無改修**（VPD/露点/温湿度は従来どおり period 連動）。→ period 切替は GDD に触れない（6.2 を構造的に充足）＋切替コスト増なし。research §3 論点④ を **Option B 確定**に更新。

### 確定3（論点⑦の解消）: GDD は `echarts.connect` から除外（`data-no-connect`）
- **発見**: `App.templ initScope` は scope 内の**全 `[data-echarts]` を `echarts.connect`**（temp/humidity/VPD/露点が同一時刻 category 軸で連動）。
- **問題**: GDD の x 軸は**経過日数（value 軸）**で時刻 category 軸と別ゆえ、connect すると axisPointer 連動が無意味。初期ロード `initScope(document)` は GDD も拾って connect してしまう。
- **決定**: GDD コンテナに `data-no-connect` を付け、`initScope` の **connect 収集のみ**から除外（init/dispose は全 `[data-echarts]` のまま）。既存チャートは属性なしで従来連動＝後方互換（7.4）。

### 確定4（論点③）: markLine/markPoint は markArea 写経で自前注入
- 既存 `injectVPDMarkArea`/`injectGapMarkArea` は markArea のみ。GDD の目標 GDD 水平 markLine（小文字 `yAxis`）・予測到達日 垂直 markLine（小文字 `xAxis`）・予測点 markPoint（`coord`）を `series[0]` へ同型注入する `injectGDDMarks` を新設。x 軸を **value 軸（経過日数）**にすることで予測到達日（データ範囲外）を `xAxis: 数値` で表現可能にする。

### 確定5（論点①）: 定植日 = `devices.planting_date` 単一 nullable 列（00010）
- P1 locality（00008）/P3 crop（00009）と完全同型の expand-contract。`DATE→pgtype.Date` 実績（`ReadingDate`）・`emit_pointers_for_null_types` で `*pgtype.Date`。CHECK/索引なし（自由日付・絞込キーでない）。作型比較・収穫実績は別 spec defer。

### Generalization
- `LinearFit` は GDD 専用でなく**回帰の汎用純関数**として `stats.go` に置く（将来の trend 検出・P15 等が再利用可能）。GDD 固有の外挿は `gdd.go::ForecastDaysToTarget` が `LinearFit` を内部利用。

### 残課題（research/実装で確認・ブロッカーでない）
- **米・サトウキビ等の Tbase・収穫目標 GDD・生育ステージ閾値の具体値**（文献＋ユーザー権威）。design は構造を確定し、米に暫定値・他は既定フォールバック（`VPDRange`/`DiseaseModel` と同じ「1メソッド更新で確定」作法）。値の妥当性は GO 後の実機スモーク（ドメイン意味はテストで捕捉不可）。
- **gap 日の扱い**: present 日のみで累積・回帰（0 充填しない）＝欠測多時は予測が遅め（保守的・目安として許容・注記で明示）。
