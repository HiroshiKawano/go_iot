# ギャップ分析（research.md） — vpd-dashboard

> 対象: VPD適正帯ダッシュボード（device-show への VPD パネル追加 ＋ 作物マスタ）。
> 目的: 要件（requirements.md）と既存コードの差分を洗い、design フェーズの実装戦略・技術調査項目を確定する。
> 調査基準: 実コード（P2 マージ後 HEAD 相当）を権威ある現状として直接確認した。

## 1. 現状調査（既存資産・確認済み）

### 1.1 計算層 `internal/chart`（純粋・`math` のみ依存）
- [stats.go](../../../internal/chart/stats.go): `SMA(values,window)` / `MovingStdDev` / `Band(sma,sigma,k)` / `Deviation(values,sma,eps)→[]*float64` / `Mean` / `MinMax` / `DiurnalRange` / `StdDev` / `CV(values,eps)→(float64,bool)`。すべて `[]float64`/スカラ入出力、gin/DB/templ/pgtype/**time** 非依存。立ち上がり区間は expanding window で欠落を作らない設計。
  - **VPD・適正帯滞在率（TimeInRange）・時間帯別逸脱の純関数は未実装**。`SMA` は VPD 系列にそのまま流用可。
- [series.go](../../../internal/chart/series.go): `ChartSpec`（Labels/Unit/Color/Raw/SMA/BandLower/BandWidth/Deviation の8フィールド）。nil/空オーバーレイは当該系列を出さない防御的契約。
- [echarts.go](../../../internal/chart/echarts.go): 唯一の公開関数 `ChartOptionJSON(spec ChartSpec) (string, error)`。line 専用。series[0]=生実測線＋markPoint(max/min)、SMA は細線、正常帯は「帯下限(透明 stack)＋帯幅(半透明 area stack)」、乖離率は第2y軸。オーバーレイは `legend.selected:false` で既定オフ。`encoding/json`（SetEscapeHTML=true）で HTML 安全化。
  - **markArea（固定しきい値帯）・gauge は未実装**。

### 1.2 描画ライブラリ go-echarts v2.7.2（module cache 確認済み・**全機能対応**）
- `charts/gauge.go` に `NewGauge` 実装あり → **gauge 構築可能**。
- `opts.MarkAreas` / `MarkAreaData` / `MarkAreaStyle` / `MarkAreaNameTypeItem`（`opts/series.go`）→ **markArea 構築可能**（series へ付与）。
- `charts.WithVisualMapOpts` / `opts.VisualMap`（`opts/visual_map.go`）→ **visualMap 構築可能**（線色変化は△・任意）。
- ※ go-echarts の型で出せない属性は「クライアント付与」既存パターン（後述 1.6）に合わせる方針が踏襲可能。

### 1.3 handler [device_show.go](../../../internal/handler/device_show.go)
- `buildChartArea(ctx,deviceID,period,now)→component.DeviceChartAreaView` が拡張の中心。`ListRecentSensorReadings`（昇順生行）→ `pgconv.NumericToFloat` で float 列化 → `overlaySpec(...)` で `ChartSpec` → `ChartOptionJSON`、`statCard`、`dailyStatRows`。
- 固定値: `bandSigmaK=2.0` / `statEpsilon=1e-9` / `smaWindowFor(period)`（24h=12/3d=36/7d=72/30d=288点）/ `jst=time.FixedZone("JST",9h)` / `statEmptyMark="—"`。
- **時刻が要る集計は handler 境界で実施**（`dailyStatRows` が JST 暦日バケット化＋欠測日補完）。stats 層へ time を持ち込まない原則が確立済み → 時間帯別逸脱バケットも同じ作法で書ける。
- `Show`（GET /devices/:device・任意 ?period）と `Chart`（GET /devices/:device/chart?period=・`oneof=24h 3d 7d 30d`）が両者 `buildChartArea` を共有。HasData=false（0件）で `emptyStatCard()`。所有者認可は `authz.RequireDeviceOwner`（閲覧系は不在/非所有とも404）。

### 1.4 View [views.go](../../../internal/view/component/views.go) / [DeviceChartArea.templ](../../../internal/view/component/DeviceChartArea.templ)
- `DeviceChartAreaView`（DeviceID/Period/HasData/Temperature・Humidity の OptionJSON・Unit・Color・Card・ShowDaily・Daily 行）/ `StatCardView`(Current/Max/Min/Diurnal) / `DailyStatRow`。
- templ: `.period-selector`（hx-get で `#device-chart-area` を innerHTML swap・hx-push-url はフルページ URL）→ `@statCards`（.summary-grid-4）→ `.linked-charts`（`#temperature-chart`/`#humidity-chart` ＝ `data-echarts data-unit data-color` ＋兄弟 option script）→ `@dailyTable`（.data-table）。
- **VPD パネル DTO・templ ブロックは未実装**。器（カード/表/gauge枠）追加はモック反映対象。

### 1.5 クライアント [App.templ](../../../internal/view/layout/App.templ) `EChartsInitializer`（L124-203）
- `[data-echarts]` を走査 → option script を JSON.parse → dispose → `echarts.init` → **series[0] へ無条件で `endLabel`/`sampling="lttb"` 付与** → `tooltip.formatter` 上書き（`params[0].axisValueLabel` 前提＝axis trigger 専用・「帯下限」除外）→ setOption。`instances.length>1` で `echarts.connect`。htmx:beforeSwap/afterSwap で dispose/再init。
- **VPD line chart は同じ `[data-echarts]` 経路でそのまま init される**（series[0]=VPD 線に kPa endLabel＝`data-unit="kPa"`）。connect 群へ入れれば時間軸 axisPointer 共有も可（y は kPa で別軸だが connect は軸連動でなく axisPointer/tooltip 連動なので無害）。
- **gauge をこの経路に乗せると不整合**: 無条件の series[0] endLabel/sampling は gauge では無意味（実害は小だが）、`tooltip.formatter` は axis 前提で gauge（item trigger）に不適、`connect` 群に gauge が混ざると時間軸グラフと誤連動。→ **初期化経路の一般化が必要（研究項目）**。

### 1.6 作物マスタの先例（確立パターン・厳密に踏襲可）
- [metric.go](../../../internal/domain/metric.go): `type Metric string`＋const＋`Label()/Unit()/Valid()/ParseMetric()/AllMetrics()`・`fmt` のみ依存。→ **9作物・少数定数の Crop はこの switch 駆動が最も近い手本**。`VPDRange()(lower,upper float64)` を1メソッド追加するだけ。
- [locality.go](../../../internal/domain/locality.go): table 駆動（loc→parent）。crop は親子・別名解決が不要なので metric.go 型で十分（table 不要）。
- [00008_add_locality_to_devices.sql](../../../db/migrations/00008_add_locality_to_devices.sql): `ALTER ADD COLUMN VARCHAR` ＋ `CHECK(... IS NULL OR ... IN(値ミラー))` ＋ 部分索引 ＋ `COMMENT` ／ Down で `DROP COLUMN IF EXISTS`。→ **00009 を同型でコピー**（crop の集計索引は P3 では未使用ゆえ任意＝design 判断）。
- sqlc [devices.sql](../../../db/queries/devices.sql): `CreateDevice`/`UpdateDevice` が既に `locality`($6) をパラメータ化。→ crop は `$7` 追加で同型。`UpdateDeviceLocality`（backfill 専用単列更新）に相当する crop 版は**不要**（backfill しない）。
- DeviceForm: `<select name="locality" class="js-tom-select">`＋`v.Errors["locality"]`、`DeviceFormView` に `Locality string`＋`Localities []SelectOption`。→ crop は `name="crop"`・`Crop string`・`Crops []SelectOption` で完全同型（カスケード無し・App.templ 改変不要）。

### 1.7 DB 現状（[table_definitions.md](../../../docs/database_snapshot/table_definitions.md)）
- `sensor_readings`: temperature numeric(5,2) CHECK -40〜125・humidity numeric(5,2) CHECK 0〜100・recorded_at timestamptz・(device_id,recorded_at DESC) 部分索引。**VPD は2列導出ゆえスキーマ非変更で読み取り時計算可能**。
- `devices`: `locality VARCHAR(20)` は実在（00008）・**`crop` 列は無い**。goose 連番最新 = **00008** → crop は **00009**。

## 2. 要件→資産マップ（ギャップタグ: Missing / Unknown / Constraint）

| 要件 | 既存資産 | ギャップ |
|---|---|---|
| R1 VPD算出（Tetens・読取時計算・境界） | stats.go の純関数群・`math` | **Missing**: `VPD(temp,rh)`＋系列版（RH0/100・氷点下・欠測の扱い）。Constraint: time 非依存・確定定数 |
| R2 作物別適正帯＋マスタ | metric.go/locality.go・00008・sqlc・CHECK ミラー | **Missing**: `domain.Crop`（9定数＋`VPDRange()`）・00009・CHECK・sqlc 列追加。**Unknown**: 作物別 kPa 具体値（research 要確定）。Constraint: マスタ=Go定数・FK無し・二重ミラー手同期 |
| R3 フォームでの作物選択 | DeviceForm locality select・DeviceFormView・device handler 検証 | **Missing**: crop select（同型）・DeviceFormView.Crop/Crops・handler の作物復元/`Crop.Valid()` 検証（空許容・errors 累積） |
| R4 VPD時系列＋適正帯3ゾーン | echarts.go line・ChartSpec・buildChartArea | **Missing**: markArea 3ゾーン機構・VPD 用 option 構築・VPD パネル DTO/templ。**Unknown**: y軸スケール固定/auto・ChartSpec 拡張 vs VPD 専用 spec |
| R5 適正帯滞在率（単一スコア） | stats 純関数・handler 集計境界 | **Missing**: `TimeInRange(values,lower,upper)`・滞在率 DTO・表示部品。**Unknown**: gauge か数値カード＋バーか・期間全体/当日 |
| R6 時間帯別逸脱 | dailyStatRows の handler バケット作法 | **Missing**: 時刻バケット集計（handler）＋バケット統計純関数・逸脱 DTO/表。**Unknown**: heatmap か表/バーか・バケット幅(1h/昼夜) |
| R7 VPD移動平均（既定オフ） | `chart.SMA`・legend.selected:false 既存 | 流用可（Missing 小）: VPD option へ SMA 系列＋凡例オフ |
| R8 既存温湿度の無回帰 | 温湿度2グラフ・統計・期間切替・URL同期・connect | **Constraint**: P2 のテスト資産・connect・tooltip override を壊さない（ChartSpec/echarts.go 後方互換） |
| R9 認可・研究用スコープ | authz.RequireDeviceOwner（消費のみ） | ギャップ無し（消費）。Constraint: 農家向け表示/アラート連動を入れない |

## 3. 実装アプローチ（A/B/C）

### Option A: 既存コンポーネントを全面拡張（ChartSpec/echarts.go に VPD・markArea・gauge を相乗り）
- VPD/滞在率/逸脱を `ChartSpec` と `ChartOptionJSON` に詰め込み、`DeviceChartAreaView` を VPD フィールドで膨らませる。
- ✅ 新規ファイル最小・既存配線そのまま。
- ❌ `ChartOptionJSON` は line 専用設計で markArea/gauge を相乗りさせると単一関数が肥大・P2 の line 前提テストと衝突リスク。❌ ChartSpec が用途過多に。
- 評価: **非推奨**（echarts.go の単一責務を壊す）。

### Option B: VPD を独立コンポーネントで新設（VPD 専用 spec＋専用 option 関数＋専用 stats ファイル）
- `internal/chart/vpd.go`（`VPD`/`TimeInRange`/逸脱の純関数）・`ChartOptionJSON` とは別の VPD option ビルダー（markArea 専用）・gauge ビルダー。`domain.Crop` 新設。handler に VPD 用 build を追加し `DeviceChartAreaView` へ VPD 用 DTO をイミュータブルに足す。
- ✅ 純粋層/描画層の分離維持・P2 完全無回帰・テスト独立。✅ markArea/gauge を line から隔離。
- ❌ ファイル増（だが coding-style「多数の小さなファイル」に合致）。
- 評価: **推奨の核**。

### Option C: ハイブリッド（純粋層は新ファイル B・描画は echarts.go へ汎用 markArea を最小追加・gauge は段階判断）
- 計算層は B（`vpd.go` 新設＋`SMA` 流用）。markArea は echarts.go に**汎用ヘルパとして最小追加**（VPD 専用 option から呼ぶ／温湿度 line には不付与で無回帰）。gauge は **(1) 数値カード＋CSS バー（初期化不要）で MVP → (2) 必要なら ECharts gauge＋初期化一般化** の段階導入。VPD line は既存 `[data-echarts]` 経路に相乗り（connect 参加は design 判断）。
- ✅ 既存資産（SMA・data-echarts 経路・凡例オフ・モック単一ソース）を最大流用しつつ責務分離。✅ gauge のクライアント改修リスクを後段に隔離（滞在率はまずカードで出せる）。
- ❌ markArea を echarts.go に足す分、echarts.go の責務がわずかに広がる（汎用ヘルパに留めれば許容）。
- 評価: **推奨**（B の分離 ＋ 既存経路の現実的再利用）。

## 4. 工数・リスク

| 区分 | 工数 | リスク | 根拠 |
|---|---|---|---|
| R1 VPD 純関数 | S | Low | stats.go パターン＋確定式。手計算一致テストで担保 |
| R2 作物マスタ＋00009＋sqlc | S | Low | metric.go/00008/locality の直近先例を写経。kPa 具体値のみ Unknown |
| R3 フォーム crop select | S | Low | locality select の完全同型 |
| R4 markArea ＋ VPD option | M | Medium | go-echarts は対応済だが MarkAreaData の JSON 形・3ゾーン塗りの型/クライアント付与の切り分けが要検証 |
| R5 滞在率（gauge or カード） | M | Medium | カードなら S/Low。**gauge は初期化一般化が要りクライアント改修＝Medium-High** |
| R6 時間帯別逸脱 | M | Medium | 集計は handler 作法で確立だが**見せ方未確定**（heatmap/表/バケット幅） |
| R7 VPD SMA | S | Low | `chart.SMA` 流用＋凡例オフ既存 |
| R8 無回帰維持 | S | Low | ChartSpec/echarts.go を後方互換に保てば P2 テストで検出 |
| 全体 | **M（5-7日目安）** | **Medium** | 律速は描画層（markArea/gauge クライアント整合）と「見せ方」未確定。計算層・マスタ・フォームは Low |

## 5. design フェーズへの申し送り

### 推奨アプローチ
- **Option C（ハイブリッド）**。純粋層は `internal/chart/vpd.go` 新設（`VPD`/系列版/`TimeInRange`/逸脱統計・time 非依存・`SMA` 流用）。VPD 描画は **専用 option ビルダー**（markArea 3ゾーン）。markArea は echarts.go に**汎用ヘルパ最小追加**し温湿度 line には付与しない（R8 無回帰）。`domain.Crop` は metric.go 型（switch 駆動＋`VPDRange()`）。フォーム/クエリ/migration は locality(00008) を写経し crop=00009。
- **滞在率は数値カード＋CSS バーで MVP**を第一候補（初期化不要・モック反映容易）。ECharts gauge は「初期化経路一般化」のコスト次第で design が採否判断（◎は gauge だがカード許容＝要件 R5 は表示手段非依存）。
- **後方互換の死守**: `ChartSpec`/`ChartOptionJSON` を変更する場合も温湿度2グラフのシグネチャ/出力を不変に保つ（VPD は別関数/別 spec が安全）。

### 研究項目（Research Needed・design / research で確定）
1. **【最重要】作物別 VPD 適正帯の具体 kPa 値**（9作物の lower/upper）。ユーザー（沖縄実地知見＝権威）＞文献。施設果菜（ゴーヤ/インゲン/ウリ/マンゴー/葉野菜）は本命・露地（サトウキビ/米/パイナップル/いも）は既定踏襲か「VPD参考」注記か。spec は構造＋既定 0.3/1.5 フォールバックまで確定済。
2. **markArea の go-echarts 実装形**: `MarkAreaData`(LeftTop/RightBottom)で yAxis 範囲3ゾーンを塗る型表現と、型で出せない属性のクライアント付与切り分け（既存 §10-E パターン）。
3. **gauge vs 数値カード＋バー**: ECharts gauge を出す場合の `EChartsInitializer` 一般化（chart 種別判定／line 専用ロジック・connect からの除外）。カード採用なら不要。
4. **VPD line の connect 参加可否**: 温湿度と時間軸 axisPointer 共有するか独立か（y は kPa 別軸）。`instances.length>1` 連動の挙動を踏まえる。
5. **時間帯別逸脱の表現**: heatmap（時間帯×日）か時刻バケット表/バーか、バケット幅（1h or 昼夜区分）。
6. **VPD y 軸スケール**: 固定（例 0〜3kPa・適正帯が常に見える）か auto か。
7. **期間別の粒度**: 24h（瞬間/カード）と複数日（日次滞在率）で粒度を変えるか（P2 の `ShowDaily=period!=24h` と整合）。
8. **氷点下/高温端の VPD 注記**: Tetens 式は氷点下で氷面飽和を使わない近似（沖縄前提で実害小・design に温度域注記）。
9. **crop 列の索引要否**: P3 は作物集計をしないため 00009 で索引を張るか（locality は張った）design 判断。
10. **テスト戦略**（テストガイダンス集参照）: VPD 手計算一致・滞在率境界（全在帯/全逸脱/空）・markArea/gauge option 構築・handler 回帰（Show/Chart の period・空・温湿度無回帰）・crop フォーム検証。

---

# 設計フェーズ Discovery / Synthesis 追記（design）

> Discovery 種別: **Extension（light discovery）**。既存 device-show / chart 層 / 作物マスタ先例への統合に集中。

## D-1. go-echarts v2.7.2 markArea の不具合（重要・設計を左右）
- `opts.MarkAreaData.YAxis` の JSON タグが `json:"YAxis"`（**大文字始まり＝ECharts 非準拠**。ECharts は `yAxis` を期待）。ペアビルダー `WithMarkAreaData0/1` もこの不具合型 `MarkAreaData` を用いる → **go-echarts ネイティブの y 軸範囲 markArea は実質機能しない**（塗りが出ない）。
- 正しいタグを持つ `opts.MarkAreaNameYAxisItem`（`json:"yAxis"`）は**単一点**用でペア（範囲）を構成できない。
- **設計判断**: VPD 適正帯 markArea は **正しい `yAxis` キーで自前構築**する。echarts.go の既存方式（go-echarts で line を組み `json.Marshal(line.JSON())` で再シリアライズ）を踏襲し、VPD option ビルダーが **markArea を正しいキーで option マップへ注入**してから HTML 安全 marshal する。go-echarts の markArea 型には依存しない。

## D-2. クライアント初期化（App.templ）への影響なし
- `EChartsInitializer` は `[data-echarts]` 走査で line を init し series[0] へ endLabel/sampling、tooltip(axis) override、`connect`。**VPD line は同経路でそのまま描画でき、markArea は option JSON に内包されるためクライアント改修不要**。VPD を connect 群へ含めると時間軸 axisPointer 共有（kPa は別 y 軸ゆえ無害・有益）。→ **App.templ 無改修**（R8 クライアント無回帰）。

## D-3. 滞在率の見せ方（gauge 見送り）
- ECharts gauge は `EChartsInitializer` が line 専用（series[0] endLabel/sampling・tooltip axis・connect）のため、gauge を混ぜると初期化一般化が必要＝クライアント改修リスク。**MVP は数値カード＋CSS 横バー**で滞在率を表現（初期化不要・モック反映容易・R5 は表示手段非依存）。ECharts gauge は将来拡張に保留。

## D-4. 作物マスタ＝locality の完全クローン
- `domain.Crop` は `metric.go` 型（switch 駆動＋const＋`Label()/Valid()/AllCrops()/ParseCrop()`）に `VPDRange()` を1メソッド追加。table 不要（親子・別名解決なし）。
- `devices.crop` migration は 00008(locality) の同型（ALTER ADD VARCHAR＋CHECK ミラー＋COMMENT・Down DROP）。**索引は張らない**（P3 は作物集計しない＝YAGNI）。
- sqlc は CreateDevice/UpdateDevice に crop 列追加（locality と同型・空→NULL）。backfill クエリ不要。
- device.go フォーム処理は locality を完全クローン: `validCropInput`/`cropInvalidMessage`/`errs["crop"]`/`deviceCropValue`/`cropOptions`/`DeviceFormView.Crop+Crops`/フォーム struct に `Crop string`。

## Synthesis 結論
- **Generalization**: VPD/滞在率/逸脱は「生温湿度 → 派生スカラ/系列」の純関数群に一般化（既存 stats.go の延長）。markArea は「固定 y 範囲帯」の汎用機構だが VPD 専用に閉じる（温湿度 line 無回帰のため echarts.go 不変）。
- **Build vs Adopt**: 描画は go-echarts を採用継続（line/legend/tooltip/axes）。markArea のみ自前（ライブラリ不具合ゆえ）。マスタは Go 定数（steering §98-100）。
- **Simplification**: gauge を見送りカード＋バーへ。VPD option は別関数（ChartSpec へ相乗りしない＝P2 無回帰）。新規ファイルは小さく分割（vpd.go / vpd_echarts.go / device_show_vpd.go）。
