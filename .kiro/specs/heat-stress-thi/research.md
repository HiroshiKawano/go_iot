# Gap Analysis — heat-stress-thi（Phase 12 高温ストレス）

> 目的: requirements（R1〜R11）と P8 マージ後の既存コードベースの差分を洗い出し、design 戦略へ引き継ぐ。
> 結論先取り: 本フェーズは **P6（露点）/P7（GDD）パネルの写経による新規ファイル追加（Option B）** がほぼ自明な最適解。**最大の不確実性は「新 ECharts 系列型（calendar/heatmap/visualMap）」だったが、go-echarts v2.7.2 にネイティブ型が揃っていることを確認でき、設計リスクは大幅に低下した**。残る論点は (a) 「夜温」定義（→データアクセス方式を規定）、(b) 年スケールデータ取得方式、(c) connect 連動の扱い、(d) `--color-heat` 配色、の4点。

---

## 1. 現状調査（Current State）— 検証済みシグネチャ

### 1.1 純粋計算層 `internal/chart/`（math/sort のみ・time/gin/DB 非依存 ✓）

| 資産 | シグネチャ（ファイル:行） | 本フェーズでの再利用 |
|---|---|---|
| 飽和水蒸気圧 | `saturationVaporPressure(tempC float64) float64` [kPa]（vpd.go:21）／`tetensA=0.6108`/`tetensB=17.27`/`tetensC=237.3`（vpd.go:13-17） | **AH の実水蒸気圧 `ea = saturationVaporPressure(T)·RH/100` に再利用**（Tetens 重複定義しない＝R1-2） |
| 系列変換 | `VPDSeries(temps, hums []float64) []float64`（vpd.go:41）／`TimeInRange(values, lower, upper) float64`（vpd.go:57） | `THISeries`/`AHSeries` の同型実装の手本 |
| 日較差 | `DiurnalRange(values []float64) float64`＝`max−min`（stats.go:93） | **日較差ΔT にそのまま再利用（新規実装しない＝R5-2）** |
| 基本統計 | `MinMax(values)→(min,max)`（stats.go:76）／`Mean`（:71）／`StdDev`（:102）／`CV(values, epsilon)→(cv,ok)`（:107） | 夜温の夜間最低/平均の集約に `MinMax`/`Mean` |
| 連続ラン | `StuckRuns(values, minRun int) []bool`（quality.go:131・**マスク形式**）／`runsFromMask` パターン／`Run{StartIdx,EndIdx}`（series.go:71） | **熱帯夜「連続日数」は同型の新関数**（「同値」でなく「閾値超え日が連続」・最長/末尾連続を返す＝R2-3） |
| トレンド | `MannKendall(xs []float64) MKResult{S,VarS,Z,PValue,N}`（trend.go:52）／`SensSlope(xs) SenResult{Slope,Intercept,Lower,Upper,HasCI}`（:98） | **年系列 `[]float64`（各年1値）を渡すだけ＝再利用・新規実装禁止（R6）** |
| 別型隔離 | `ChartSpec`/`VPDChartSpec`/`DewpointChartSpec`/`GDDChartSpec`/`TrendChartSpec`（series.go） | **`HeatStressChartSpec` を別型で追加**（無回帰隔離＝R9） |

### 1.2 ECharts option 構築層 `internal/chart/*_echarts.go`

- **inject パターン（確立手法）**: go-echarts option → `json.Marshal(line.JSON())` → `map[string]any` へ戻し → ECharts 準拠の**小文字キー**を series[0] へ自前注入 → 再 `json.Marshal`（`SetEscapeHTML=true` で HTML 安全化）。動機は go-echarts の `MarkAreaData.YAxis`/`XAxis` が大文字キーを吐く JSON タグ不具合（vpd_echarts.go:114・gap_echarts.go:29・gdd_echarts.go:122）。
- **ビルダ**: `GDDChartOptionJSON(spec GDDChartSpec) (string, error)`（gdd_echarts.go:29）等。戻り値は **HTML 安全 JSON 文字列**（`template.JS` でなく `string`・`</script>` 不混入）。
- **共通ヘルパは無し**（各ビルダ独立）。`optionScript()`（views.go:244）が `<script type="application/json" id="{id}-option">` を生成。

### 1.3 handler 配線・view DTO・domain

- `buildChartArea(ctx, device, period, now) (DeviceChartAreaView, error)`（device_show.go:261）: `ListRecentSensorReadings`→float 変換→温湿度 option→`statCard`→`dailyStatRows`（JST 暦日・:568）→`buildVPDPanel`→`buildDewpointPanel`→`DeviceChartAreaView` 組立。`jst`（:52）・`deviceCrop(device)`（:516・NULL→`""`→既定フォールバック）・`statEmptyMark="—"`（:238）・`applyGapGrid`（:406）。
- `buildGDDPanel(ctx, device, now) (GDDPanelView, error)`（device_show_gdd.go:42）: deviceCrop→`GDDModel()`→**前提欠落判定（Stages 空 or `!PlantingDate.Valid`→Guidance 注記を error 無しで返す）**→`ListDailySensorAggregates(plantDay→now)`（**日次 max/min 気温の集約クエリが既存**）→純粋層→`GDDChartOptionJSON`→`GDDPanelView`。**period 非連動（Show からのみ呼び Chart フラグメントでは呼ばない）**＝熱帯夜 calendar の手本。
- バケットヘルパ: `vpdHourlyRows`（device_show_vpd.go:127・JST hour-of-day 0-23）＝**THI hour×day の土台**／`dailyStatRows`（JST 暦日）＝熱帯夜日次の土台。
- `DeviceChartAreaView`（views.go:59-94）は末尾が `VPD VPDPanelView`／`Dewpoint DewpointPanelView`／`HasGap bool`＝**末尾非破壊追加**の形。`GDDPanelView{OptionJSON,Color,CropLabel,Card,Stages,Guidance,Note}`（:183）。
- `crop.go`: `type Crop string`＋9作物／`VPDRange()→(下限,上限)`／`DiseaseModel() struct`／`GDDModel() struct{Tbase, Stages[]}`（rice のみ具体・他は Stages nil で既定縮退）／`AllCrops()`／`ParseCrop()`・**import は fmt のみ**。→ **`HeatStressModel()` を同型で非破壊追加可（R8）**。

### 1.4 クライアント側 `App.templ` EChartsInitializer（:127-211）

- `initContainer(el)`: `{id}-option` の JSON を `JSON.parse`→既存 dispose→init→endLabel/sampling／`data-no-connect` なら endLabel 除外／tooltip formatter で帯下限除外。
- `initScope(scope)`: `[data-echarts]` 走査→**`data-no-connect` の無いものだけ** connectable に集めて `echarts.connect()`。フック=`DOMContentLoaded`／`htmx:beforeSwap`(dispose)／`htmx:afterSwap`(reinit)。
- **⚠️ 重要制約**: connect は時刻 category 軸の同時ホバー前提。**calendar（暦日×月）/heatmap（visualMap）を connect に混ぜると軸ミスマッチでホバーが破綻する**。→ 本フェーズの heatmap/calendar コンテナは **`data-no-connect` 必須**（GDD が既に `data-no-connect` を使う前例）。

### 1.5 モック（単一ソース）

- 派生指標カラートークン: `--color-vpd:#0ca678`(緑)／`--color-dewpoint:#4263eb`(青)／`--color-gdd:#e03131`(**赤=暖色**)／`--color-trend:#7048e8`(紫)（style.css:71-74）。→ **`--color-heat` を追加（暑熱=暖色）**だが、**温度系列(橙)・GDD(#e03131 赤) と衝突しない暖色の選定が必要**（design 論点）。
- パネル器: `id="…-chart" data-echarts data-unit data-color class="chart-placeholder"`＋`summary-grid summary-grid-4` カード（device-show.html）。GDD は `data-no-connect` 付き（:295）。

---

## 2. 外部依存リサーチ（最重要・design 論点①の解消）

**go-echarts v2.7.2 は calendar/heatmap/visualMap をネイティブに型で表現できる**（モジュールキャッシュを直接確認）:

| 必要要素 | go-echarts v2.7.2 ネイティブ対応 | 備考 |
|---|---|---|
| heatmap 系列 | `charts.NewHeatMap()`／`AddSeries(name, []opts.HeatMapData)`（charts/heatmap.go） | `opts.HeatMapData`（opts/charts.go:242） |
| calendar 座標 | `HeatMap.AddCalendar(...*opts.Calendar)`（hasXYAxis=false に切替）／`opts.Calendar{Range,CellSize,Orient,DayLabel,MonthLabel,YearLabel,ItemStyle}`（opts/calendar.go） | **prompt が懸念した range/cellSize/orient は全てネイティブ** |
| 系列↔calendar 束縛 | `WithCoordinateSystem("calendar")`＋`WithCalendarIndex(i)`（charts/series.go:210-218） | `CoordSystem`/`CalendarIndex` フィールド |
| visualMap | `opts.VisualMap{Type:"continuous"/"piecewise", Min,Max, InRange.Color[], Pieces[]Piece, Calculable, Text}`（opts/visual_map.go） | **piecewise 区分・inRange.color もネイティブ** |
| 直列化 | `base.go` の GlobalOpts が `Calendar []*opts.Calendar`／`VisualMapList []opts.VisualMap` を保持し、独自 MarshalJSON visitor で `obj["calendar"]`/`obj["visualMap"]` を**正しい camelCase** で出力（base.go:194-227） | go-echarts 自身が JSON タグ不具合を visitor で回避済み |

**含意**: 「go-echarts が新系列型を表現できるか」という prompt の第一論点は **YES**。inject パターンは markArea/markLine と同様、**ニッチ属性の微調整がある場合のみ**補助として使えばよく、系列・座標・visualMap の骨格はネイティブ型で組める。ただし本リポジトリの確立した直列化経路（go-echarts MarshalJSON → map → inject → 再 JSON）に calendar/visualMap を**正しく通せるか**は design で小さく実証する（残リスク・下記 Research Needed）。

---

## 3. Requirement → Asset マップ（gap タグ: ✅再利用 / 🆕新規 / ⚠️制約 / ❓要研究）

| Req | 必要技術 | 既存資産 | gap |
|---|---|---|---|
| R1 THI/AH 算出 | THI=`0.8T+(RH/100)(T−14.4)+46.4`／AH=`217·ea/(T+273.15)` | `saturationVaporPressure`+Tetens（✅再利用） | 🆕 純関数 `THI/AbsoluteHumidity/THISeries/AHSeries`（新 `heatstress.go`）。RH クランプ・氷点下安全・RH=0→AH=0。低リスク |
| R2 熱帯夜/夜温/連続日数 | 夜温≥閾値判定・最長/現在連続 | `MinMax`/`Mean`（✅）・`StuckRuns`/`Run`（✅型の手本） | 🆕 閾値連続ラン関数＋夜間バケット（handler）。⚠️**「夜温」定義（②）がデータアクセスを規定**。欠測夜を熱帯夜0と誤計上しない |
| R3 熱帯夜 calendar（◎） | calendar+heatmap+visualMap | go-echarts ネイティブ（§2 ✅）・inject 手法（✅） | 🆕 `heatstress_echarts.go` の calendar option。⚠️`data-no-connect` 必須。⚠️年スケールデータ（§4）。中リスク（初導入） |
| R4 THI hour×day heatmap（○） | heatmap+visualMap（直交 x=hour,y=day） | go-echarts ネイティブ（✅）・`vpdHourlyRows`（✅） | 🆕 hour×day option。`data-no-connect`。中〜低 |
| R5 AH/ΔT 表示 | AH 時系列・ΔT 日次 | `DiurnalRange`（✅再利用）・line 系列（✅確立） | 🆕 AH 系列（R1 由来）。低 |
| R6 年間日数トレンド（○） | 各年1値→MK/Sen | `MannKendall`+`SensSlope`（✅**再利用**）・`TrendChartSpec`（手本） | 🆕 年集計（handler）。⚠️**多年データ希少（G-8）→Sen 傾き＋符号＋記述統計に留め断定しない**。❓配置（device-show ミニ vs P8 系列＝⑤）。検証は seed-trendsensor（多年） |
| R7 暑熱=暖色 | `--color-heat`・visualMap 暖色 | 物理規約（project_vpd_physics_convention） | 🆕 トークン追加。⚠️**温度(橙)・GDD(#e03131赤)と衝突回避の暖色選定**。実機スモーク必須 |
| R8 しきい値（既定/作物別） | 物理定数 or 作物別 | `crop.go` の `VPDRange`/`GDDModel` パターン（✅） | 🆕 `HeatStressModel()` Go 定数（DB 列増やさない）。低 |
| R9 スキーマ非変更/fallback/安定動作 | 読み取り時計算・dispose/connect | 読み取り経路（✅）・EChartsInitializer（✅） | ⚠️`data-no-connect`／❓年スケール取得クエリ（§4）。中 |
| R10 無回帰 | 既存パネル温存 | `buildChartArea`＋末尾追加（✅確立） | ✅ 低（VPD/Dewpoint/GDD/SMA/gap 回帰テスト緑維持） |
| R11 認可/研究スコープ | 所有者認可 | `authz.RequireDeviceOwner`（既に Show 経路・✅） | ✅ パネルは Show ハンドラの認可に相乗り。新規なし。低 |

---

## 4. データアクセスの gap（design の核 — ②と直結）

- **既存クエリ**: `ListRecentSensorReadings`（期間生行）／`ListDailySensorAggregates`（**日次 max/min 気温**・GDD が使用）。
- **THI hour×day（R4）**: 期間生行＋handler の hour バケットで足りる → `ListRecentSensorReadings` 再利用で可。
- **熱帯夜 calendar/年間日数（R3/R6）**: 対象は**年スケール**。ここが分岐:
  - もし **「夜温＝日最低気温」**（気象庁公式定義に近い）と確定するなら → **`ListDailySensorAggregates` の日次 min 気温を1年範囲で再利用できる可能性が高い**（新規クエリ不要）。
  - もし **「夜温＝夜間窓（例 18:00〜翌06:00）の最低/平均」**（JST 日跨ぎ）なら → 日次暦日 min とは別物。①生行を1年取得して handler で夜間窓バケット（**5分間隔×1年≈10万行/台＝重い**）か、②**夜間窓の日次集約 SELECT を新設**（DDL なし・SELECT のみ・goose 00010 据置）かが必要。
- **⚠️ よって「夜温」定義（②）は単なる UI 文言でなく、新規クエリの要否・データ転送量を決める設計分岐**。design で②を確定してからデータアクセスを決める。
- スキーマ変更・マイグレーションは**不要**（goose 00010 据置・`make db-snapshot` 不要）。新規 SQL を起こす場合も SELECT のみ。

---

## 5. 実装アプローチ（Options A/B/C）

### Option A — 既存ファイルへ拡張
温湿度/VPD ロジックを抱える `device_show.go`・`series.go`・既存 `*_echarts.go` へ直接追記。
- ✅ 新ファイル最小。 ❌ device_show.go/series.go の肥大（既に大）・無回帰リスク増・凝集低下。**不採用寄り**。

### Option B — 新規ファイル（P6/P7 写経）★推奨
P3/P6/P7 と同じ分割で新規追加:
- `internal/chart/heatstress.go`（THI/AH/熱帯夜判定/閾値連続ラン・純粋）
- `internal/chart/heatstress_echarts.go`（calendar/heatmap/visualMap option・inject 補助）
- `internal/chart/series.go` に `HeatStressChartSpec`（別型隔離）
- `internal/handler/device_show_heatstress.go`（`buildHeatStressPanel`・夜間/暦日/年バケット）
- `internal/view/component/views.go` に `HeatStressPanelView` 他 DTO（DeviceChartAreaView 末尾へ非破壊追加）
- `internal/domain/crop.go` に `HeatStressModel()`（必要時）
- `DeviceChartArea.templ` に THI heatmap/熱帯夜 calendar/夜温/AH/ΔT ブロック（`data-echarts data-no-connect`）／`mocks/html/{device-show.html,style.css}` に器＋`--color-heat`
- **年間日数トレンドは `trend.go` 再利用（新規統計なし）**。
- ✅ 確立パターンに完全整合・凝集高・無回帰隔離・テスト容易。 ❌ ファイル数増（許容）。

### Option C — ハイブリッド/段階
Option B を基本に、判断の割れる2点を段階化:
- (i) 年間日数トレンドの配置（⑤）: 既定は device-show ミニ表示、本格多年横断は P8 系列へ後送り（計算は trend.go 共通ゆえ重複なし）。
- (ii) データアクセス: フェーズ1で `ListDailySensorAggregates` 流用（夜温＝日最低）で MVP→必要なら夜間窓集約 SELECT を追加。
- ✅ リスクを段階回収。 ❌ 計画やや複雑。

**推奨**: **Option B**（P6/P7 が直近の写経前例で確立済み）。配置⑤とデータアクセスは Option C の段階観点を design 判断として併記。

---

## 6. Effort / Risk

- **Effort: M（3〜7日）** — 純関数（THI/AH/閾値ラン）は小（既存式・既存パターン）。律速は **2つの新 ECharts 系列型（calendar/heatmap/visualMap）の初導入**＋年スケールのデータアクセス決定＋モック器整備＋向き（暖色）。
- **Risk: Medium** — 根拠: (1) calendar/heatmap/visualMap はネイティブ型確認で**大幅減**だが本リポジトリ初導入で直列化経路の実証が要る、(2) EChartsInitializer の connect 連動を heatmap/calendar で破綻させない（`data-no-connect`）、(3) 「夜温」定義がデータアクセス量を左右、(4) 暑熱=暖色の向き取り違え（VPD 前例）。いずれも既知パターン＋小さな実証で御せる範囲。

---

## 7. Research Needed（design へ持ち越す確認事項）

1. **【②・最優先】「夜温」の確定定義**（日最低気温≥25℃ か 夜間窓 18:00〜翌06:00 の最低/平均≥25℃ か）。**ユーザーの沖縄実地知見＝権威で確定**。これがデータアクセス方式（既存日次集約の流用 vs 夜間窓集約 SELECT 新設 vs 年生行 handler バケット）を規定。閾値帯（25℃/真夏日30℃/猛暑日等の追加要否）も。
2. **【データアクセス】年スケール取得方式**: `ListDailySensorAggregates` の1年流用可否、夜間窓集約 SELECT（DDL なし）新設の要否、5分×1年≈10万行の転送/計算負荷。
3. **【直列化実証】** go-echarts ネイティブ `HeatMap.AddCalendar`＋`WithCoordinateSystem("calendar")`＋`opts.VisualMap` を、本リポジトリの「option→map→inject→再 JSON・HTML 安全」経路に正しく通せるか（visualMap/calendar キーが camelCase で残るか）。ニッチ属性で inject 補助が要る箇所の特定。
4. **【connect】** heatmap/calendar コンテナに `data-no-connect` を付与し、時系列 line（温湿度/VPD/露点/AH/ΔT）の連動と分離する設計。AH/ΔT line は連動に含めるか否か。
5. **【③】THI ストレス帯境界**（visualMap の piecewise 区分値 or continuous）。家畜由来 THI 帯（68/72/80）は施設果菜にそのまま当たらず暫定。研究者本人/文献で確定。
6. **【⑤】年間日数トレンドの配置**（device-show ミニ vs 統計分析ページ＝P8 系列）。既定は device-show ミニ＋本格多年は P8 系列。
7. **【R7 配色】`--color-heat` の暖色選定** — 温度系列(橙)・GDD(#e03131 赤)と判別可能な暑熱色。visualMap カラースケールの向き（高 THI/高夜温＝濃い暖色）。
8. **【calendar 期間】** 対象年（選択中の年 or 直近1年）の決め方・年切替 UI の要否・短期ビュー（24h/3d）での calendar 縮退表示。
9. **【⑧/欠測】** 5分間隔で夜間窓集約が安定するか、夜間欠測日を「判定不能（—）」とし熱帯夜0と誤計上しない扱い。
10. **【作物別の要否④】** 当面は作物非依存の既定で足りるか、`HeatStressModel()` を起こすか（施設果菜と露地で持つ属性が割れる前提）。
11. **テスト指針**: `2cc_sdd/テストガイダンス集.md`（templ Render→buffer→Contains・Querier 手書きモック・カバレッジ80%設計）を design の Testing Strategy で参照。calendar/heatmap option の小文字キー・空データ・暑熱=暖色の向きを符号化。多年トレンド検証は `make seed-trendsensor`（project_trend_verify_multiyear_seed）。

---

## 8. design への推奨

- **アプローチ**: Option B（P6/P7 写経・新規ファイル分割）。配置⑤とデータアクセスは Option C の段階観点で併記。
- **確定すべき設計判断**: ②夜温定義（→データアクセス）、直列化実証、`data-no-connect`、③THI 帯、⑤配置、`--color-heat`、calendar 対象年。
- **持ち越す不変条件**: スキーマ非変更（goose 00010）・`trend.go`/`saturationVaporPressure`/`DiurnalRange` 再利用（重複実装禁止）・別型隔離（`HeatStressChartSpec`）・末尾非破壊追加・所有者認可相乗り・暑熱=暖色を spec/テスト明記＋実機スモーク。
</content>

---

## 9. Design 合成（kiro-spec-design・2026-06-30 追記）

discovery 追補（design フェーズ）で判明した決定的事実と synthesis 結果。

### 9.1 追加で判明した既存資産（design を確定させた発見）

- **`ListDailySensorAggregatesJST`（P8 新設・既存）**: `DATE(recorded_at AT TIME ZONE 'Asia/Tokyo')` で JST 暦日バケットし、avg/max/**min** temperature・avg/max/min humidity・sample_count を JST 昇順で返す。**欠測日は行を返さない（handler が欠測扱い・0 補完しない）**契約。`db/queries/sensor_readings.sql`。
  - → **「夜温＝JST 日最低気温≥25℃」を採れば、熱帯夜 calendar・夜温推移・ΔT・年間日数トレンドは新規クエリ不要で本クエリ再利用で足りる**（research §4 の最大論点が解消）。
  - 既存 `ListDailySensorAggregates`（`DATE()`＝UTC バケット）は 3d/7d/30d グラフ用に温存（混同しない）。
  - `MaxTemperature`/`MinTemperature` は sqlc が `interface{}` 生成（MAX/MIN of numeric の型推論不可）。GDD の `aggregateToFloat` 系で float 化する前例あり。
- **go-echarts v2.7.2 ネイティブ対応の確証**: `charts.NewHeatMap()`／`AddSeries([]opts.HeatMapData)`／`AddCalendar(*opts.Calendar)`（hasXYAxis=false 化）／`WithCoordinateSystem("calendar")`＋`WithCalendarIndex`／`opts.VisualMap{Type,Min,Max,InRange.Color,Pieces}`。`base.go` が `Calendar`/`VisualMapList` を独自 MarshalJSON visitor で camelCase（`obj["calendar"]`/`obj["visualMap"]`）出力。→ inject 補助はニッチ属性のみ。
- **EChartsInitializer の line 向け後処理**: `initContainer` は endLabel/sampling を付与するが **`data-no-connect` で gate**。`initScope` は `data-no-connect` の無いものだけ `echarts.connect`。→ heatmap/calendar は `data-no-connect` 必須（時刻軸前提の connect と非互換）。
- **色トークン現状**: `--color-vpd`(緑)/`--color-dewpoint`(青)/`--color-gdd`(#e03131 赤)/`--color-trend`(紫)。温度=橙・湿度=青。→ `--color-heat` は橙・赤と判別する暑熱色（design 既定 `#d6336c`・実機スモークで確定）。

### 9.2 Synthesis（Generalization / Build-vs-Adopt / Simplification）

- **Build-vs-Adopt**: calendar/heatmap/visualMap は go-echarts ネイティブを**採用**（自前 SVG/全面 inject を作らない）。トレンド統計は `trend.go` を**採用**（新規実装しない）。AH の es(T) は `saturationVaporPressure` を**採用**（Tetens 重複定義しない）。
- **Simplification（YAGNI）**: (1) ΔT は日次集約の max/min から直接差分し `DiurnalRange` を呼ばない（生値経路は不要）。(2) `HeatStressModel()`（作物別）は当面不要＝作物非依存の Go 定数のみ。(3) THI 帯は連続色のみ（piecewise 段階色は文献確定後）。(4) 年間日数トレンドは device-show ミニ表示のみ（P8 系列への本格配置は繰り延べ）。(5) 新規クエリを起こさない（既存2クエリで全系列を賄う）。
- **Generalization**: 派生指標パネルの5点セット（純粋層＋`*_echarts`＋`build*Panel`＋`*PanelView`＋templ ブロック・別型 Spec 隔離・末尾非破壊追加・period 非連動）を P3/P6/P7 と同形で踏襲＝インターフェースを一般化済み、実装スコープは本要件に限定。

### 9.3 解決済み Research Needed（§7 との対応）

| §7 項目 | design での解決 |
|---|---|
| ②夜温定義 | **D2**: JST 日最低気温≥25℃（既存 JST 日次集約再利用・新規クエリなし）。夜間窓別定義は将来・ユーザー権威 |
| データアクセス | **D2/D3**: `ListDailySensorAggregatesJST`＋`ListRecentSensorReadings` の2クエリ・新規 SQL ゼロ |
| 直列化実証 | **D1/Q1**: ネイティブ型＋base.go MarshalJSON。実装初手で camelCase 残存を実証 |
| connect | **D4**: 全 heat チャートに `data-no-connect` |
| ③THI 帯 | **D5**: 連続暖色のみ（piecewise 繰り延べ） |
| ⑤配置 | **D7**: device-show ミニ表示・P8 系列は繰り延べ |
| 配色 | **Q3**: `--color-heat` 既定 #d6336c・実機スモーク確定 |
| calendar 期間 | **D8**: 直近1年固定・year 切替 UI なし |
| ⑧欠測 | **D9**: 欠測日は熱帯夜0と誤計上しない（クエリ契約） |
| ④作物別 | **D6**: 既定定数のみ・`HeatStressModel()` は将来拡張点 |
