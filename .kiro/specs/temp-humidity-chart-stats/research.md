# Gap Analysis: temp-humidity-chart-stats

> 要件（requirements.md・R1〜R9）と既存コードベースの差分分析。design フェーズの実装戦略判断に供する。
> 分析日: 2026-06-27 ／ 対象: S5 device-detail ＋ E1 device-chart-echarts への統計上載せ拡張。

## 1. 現状調査（既存資産の棚卸し）

### 1.1 計算層（描画前データ整形）

| 資産 | 場所 | 現状 | 本フェーズでの位置づけ |
|---|---|---|---|
| `Point{Label, Y}` / `Series{Name, Dashed, Points}` | `internal/chart/series.go` | 1点・1系列の最小型。`Name`（凡例名）/`Dashed`（破線）は**現状未活用** | per-series 属性の拡張余地（stack/areaStyle/legend 既定可視/showSymbol） |
| `buildChartArea` / `rawSeries` | `internal/handler/device_show.go` | `ListRecentSensorReadings`（指定時刻以降・昇順の生データ）を取得→温度/湿度それぞれ**単一系列**へ写像。**全期間（24h/3d/7d/30d）が生データ単一折れ線に統一済み**（コミット 9261f9d） | SMA/帯/乖離率の系列生成・カード値・日次集計をここで組み立てて View へ詰める中心 |
| `aggregateToFloat(v interface{})` | `internal/handler/device_show.go:334` | NUMERIC/MAX/MIN の `any` を float64 へ安全変換（`pgtype.Numeric`/`float64`/想定外→0）。readings.go でも流用 | 日次集計（σ含む）の float 化に流用可 |
| `pgconv.NumericToFloat` | `internal/infra/pgconv` | numeric→float の境界変換 | handler 境界で消費（計算層は `float64`/`Point` を受ける純関数に保つ） |

**重要: 統計算出関数（SMA・σ・乖離率・日較差・CV）は一切存在しない（=新規）。** `internal/chart` は `LineOptionJSON` と型定義のみ。

### 1.2 描画層（ECharts option 構築）

- `LineOptionJSON(series []Series, unit, color string) (string, error)`（`internal/chart/echarts.go`）。
  - **series[0] のみ**を 1 本の line（＋`markPoint` max/min＋tooltip axis/cross）として option 化。`go-echarts/v2` の `charts.NewLine()` ＋ `opts` で型安全に構築し、`json.Marshal(line.JSON())` で HTML 安全 JSON 化。
  - **legend なし**・**areaStyle なし**・**第2 y軸なし**・**stack なし**。複数系列・帯・別軸は未対応。
  - endLabel（右端現在値）/ sampling("lttb") は option に含めず**クライアント付与**。
  - テスト資産: `internal/chart/echarts_test.go` あり（拡張時に無回帰必須）。

### 1.3 View / templ / クライアント

- `DeviceChartAreaView`（`internal/view/component/views.go:49`）: option JSON×2・Unit×2・Color×2・HasData のみ。**カード値・日次集計行のフィールドなし**。
- `DeviceChartArea.templ`: 期間ボタン群＋`.linked-charts`/`.chart-wrapper`/`#temperature-chart`/`#humidity-chart`（`data-echarts data-unit data-color`）＋兄弟 option script。器はモック準拠。**カード・表のマークアップなし**。
- `EChartsInitializer`（`App.templ` L124-185）: `[data-echarts]` 走査→`JSON.parse`→`dispose`→`init`→**`option.series[0]` へ endLabel/sampling 付与**→`setOption`、`instances.length > 1` で `echarts.connect()`。htmx:beforeSwap/afterSwap で dispose/再 init。
  - **制約: endLabel/sampling を無条件に `series[0]` へ付ける**。複数系列化後、生実測線が series[0] であり続ければ概ね温存できるが、SMA/帯/乖離率には付けない出し分けの担保が必要。

### 1.4 集計クエリ・スキーマ

- `sensor_readings`: `temperature`/`humidity` = `numeric(5,2)`、`recorded_at` = `timestamptz`、部分索引 `(device_id, recorded_at DESC) WHERE deleted_at IS NULL` あり。**スキーマ変更しない**（R8）。
- `ListDailySensorAggregates`（`db/queries/sensor_readings.sql:29`）: 日別 **AVG/MAX/MIN/COUNT**。`DeviceRepo` interface（`device.go:50`）に**宣言済みだが実呼び出しなし＝未配線確定**。**STDDEV なし → σ/CV は出ない**。
- `GetSensorReadingsSummary`: 期間 AVG/MAX/MIN/COUNT（readings.go で使用・S6）。
- `make sqlc` で再生成する方針（DDL は不変）。

### 1.5 CSS（モック単一ソース・再利用可能な器）

| クラス | 定義 | 再利用先 |
|---|---|---|
| `.summary-grid` + `.summary-box`（`.label`/`.value`） | `style.css:591-603`（readings 履歴の集計6項目で実績・3列グリッド・狭幅1列） | **数値カード**（現在値/最高/最低/日較差）にほぼそのまま流用可 |
| `.data-table`（thead/tbody・`.table-wrapper`） | `style.css:368-380` | **日次集計表**に流用可 |
| `.card` サーフェス | `style.css:355` | カード/表のセクション枠 |

**所見: 新規 CSS クラスはほぼ不要**（structure.md「独自クラス新設を最小化」に合致）。既存 `.summary-grid`/`.data-table` の写経で器を作れる。

### 1.6 モックの現状（注意）

- `mocks/html/device-show.html`（148 行）は **E1 移行が未反映**（グラフ内部が旧 SVG プレースホルダ・期間切替が `<a href="?period=">`）。これは**グラフ描画例外**（feedback_mock_graph_rendering_exception）で意図的に放置されている領域。
- 本フェーズで追加する**数値カード・日次集計表は静的な器ゆえモック反映必須**（R9・feedback_mock_reflects_impl_visual）。`make sync-css` で本番へ反映。

## 2. 要件→資産マップ（Gap タグ: Missing / Unknown / Constraint）

| 要件 | 必要技術 | 既存資産 | Gap |
|---|---|---|---|
| R1 数値カード（現在値/最高/最低/日較差） | 期間データから集計・View フィールド・templ・CSS | `.summary-grid` 流用可・`buildChartArea` 拡張 | **Missing**（カードの算出・描画一式） |
| R2 SMA 線（トグル・既定オフ・SMA1本） | SMA 純関数・複数系列 option・legend selected | なし（計算）・`LineOptionJSON` 単系列 | **Missing**（SMA計算）＋**Constraint**（描画層単系列） |
| R3 正常帯 SMA±kσ（塗り帯・既定オフ・k=2） | 移動σ純関数・2系列積み上げ area・legend | なし・areaStyle/stack 未対応 | **Missing**＋**Constraint**（go-echarts での area/stack 表現） |
| R4 乖離率%（トグル・別尺度・ゼロ除算ガード） | 乖離率純関数・第2 y軸 or 別パネル | なし・第2 y軸未対応 | **Missing**＋**Unknown**（軸構成は設計判断） |
| R5 日次集計表（平均/最高/最低/日較差/σ/CV・複数日のみ） | 日次集計（σ含む）・View・templ・CSS | `ListDailySensorAggregates`(未配線・σ無)・`.data-table`・`aggregateToFloat` | **Missing**（σ/CV）＋**Unknown**（SQL STDDEV 追加 vs Go 計算） |
| R6 算出正当性・境界（立ち上がり/空/ゼロ除算） | 純関数のテスト網羅 | なし | **Missing**（計算層 TDD） |
| R7 無回帰（期間切替/URL/連動/空/繰り返し） | 既存フロー温存 | `Show`/`Chart`/`EChartsInitializer`/既存テスト | **Constraint**（無回帰死守・追加系列で connect/endLabel/dispose を壊さない） |
| R8 スキーマ非変更・読取時計算・SMA1本 | DDL 不変・派生列なし | 既存スキーマ | **Constraint**（マイグレーション禁止・EMA/WMA 不採用） |
| R9 モック整合（カード/表は反映・グラフ内部は例外） | モック編集＋`make sync-css` | `.summary-grid`/`.data-table` | **Missing**（モックにカード/表追加） |

## 3. 実装アプローチ案

### Option A: 既存コンポーネント拡張（全面）
`series.go`/`echarts.go`/`device_show.go`/`views.go`/`templ` を直接拡張し、統計計算も handler 内に書く。
- ✅ 新規ファイル最小・既存パターン踏襲。
- ❌ **計算ロジックが handler に混入**するとガードレール⑧（計算層と描画層の分離）・structure.md（domain/純粋層の純度）に反する。`echarts.go` も肥大化リスク。

### Option B: 新規コンポーネント分離（全面）
統計計算を新 package（例 `internal/analysis`）、描画も新関数群へ。
- ✅ 関心の分離が最もクリーン・単体テスト容易。
- ❌ E1 のテスト資産（`echarts_test.go`）・既存 `LineOptionJSON` consumer との整合コストが増える。IoT 小規模・cc-sdd 回転数重視（product.md）に対し過剰分割の懸念。

### Option C: ハイブリッド（**推奨**）
- **計算層は新規・純関数**（`internal/chart` 内に stats 関数群を増設、または同層サブ package）。`[]float64`/`[]Point` を受け `gin`/DB/`templ`/`pgtype` 非依存（structure.md 依存方向・最下流純粋性を維持）。SMA・移動σ・乖離率・日較差・CV を個別純関数化し TDD（R6）。
- **描画層は echarts.go を拡張**（既存 `LineOptionJSON` の後方互換を保つ新関数 or 可変オプション化）。生実測線＝series[0] を維持しつつ、SMA 線・帯（2系列積み上げ area）・乖離率（別軸）を足し、`legend.selected:false` で ○ 系列を既定オフ。
- **View/templ はカード・表フィールドを追加**（`.summary-grid`/`.data-table` 写経）。handler が計算結果を View へ詰める（イミュータブル）。
- **クライアントは最小調整**（endLabel を生実測線のみへ・connect 健全性確認）。
- ✅ ガードレール⑧・structure.md 依存方向に最も忠実。E1 資産を温存しつつ段階追加。
- ✅ 「計算（純・テスト容易）／描画（option 集中）／表示（器）」の三分割が要件 R6/R7 の無回帰要請に合致。

## 4. 工数・リスク

| 領域 | 工数 | リスク | 根拠 |
|---|---|---|---|
| 計算層（SMA/σ/乖離率/日較差/CV・純関数＋TDD） | S | Low | 定義は付録A で確定・純関数・境界条件も明確。立ち上がり/空/ゼロ除算の場合分けのみ |
| 描画層（複数系列・legend・積み上げ area 帯・第2軸） | M | **Medium** | go-echarts/v2 で areaStyle/stack/第2 y軸/legend selected を型安全に出せるか要検証。型で出せない属性はクライアント付与の前例あり |
| View/templ/カード/表 | S〜M | Low | `.summary-grid`/`.data-table` 流用・既存写経パターン |
| 日次 σ/CV 経路（SQL STDDEV 追加 or Go 計算） | S | Low | どちらも軽い。SQL なら `make sqlc`・Go なら `aggregateToFloat` 流用 |
| クライアント endLabel 出し分け・無回帰 | S | Low〜Medium | series[0]=生線を保てば最小変更。connect/dispose の回帰に注意 |
| モック反映（カード/表）＋ `make sync-css` | S | Low | 静的器のみ |

**総合: M（3〜7 日相当）／リスク Medium（律速は描画層の go-echarts 表現力）。**

## 5. design フェーズへの引き継ぎ

### 推奨アプローチ
**Option C（ハイブリッド）**。計算層＝新規純関数（依存方向死守）、描画層＝`echarts.go` 拡張（後方互換 or 新関数）、View＝カード/表フィールド追加（器は既存 CSS 流用）、クライアント＝最小調整。

### 主要な設計判断（requirements の Unknown を design で確定）
1. **SMA 窓幅 N**: サンプリング間隔（約5分想定・未確定）と period 依存。点数固定 N か時間窓か／立ち上がり処理（部分平均 or 欠落）。
2. **k（±kσ）既定値**: k=2（≈95%）本命。ユーザー可変化の要否（要件では既定値提供・トグル禁止はしていない）。
3. **乖離率の軸構成**: 同チャート第2 y軸 か 別パネル か（実測値スケールと混在回避）。
4. **σ/CV 算出経路**: `ListDailySensorAggregates` に `STDDEV_*` 追加（→`make sqlc`・母/標本の別）か、生データから Go 集計か。
5. **`LineOptionJSON` 拡張形**: 既存シグネチャ拡張か新関数追加か（`echarts_test.go` 資産・後方互換）。
6. **`Series` per-series 属性表現**: 既存 `Name`/`Dashed` を活かしつつ stack/areaStyle/legend 既定可視/showSymbol をどう持たせるか。
7. **クライアント endLabel 出し分け**: 生実測線のみへ付与する担保を option（系列メタ）側 か 初期化スクリプト側 か。
8. **2系列積み上げ area 帯**: 下限系列（SMA−kσ・透明）＋帯幅系列（2kσ・半透明）の stack 同一グループを go-echarts で型安全に組めるか／不可ならクライアント付与。
9. **24h の日次集計表**: R5-3 で「単日は表なし・カードで把握」と確定済み（design はこれを実装に落とす）。

### Research Needed（design で軽検証）
- **go-echarts/v2 の表現力**: `opts` で `AreaStyle`/`Stack`/第2 `YAxis`/`Legend{Selected}` を型安全に設定できる範囲。出せない属性のクライアント付与パターンの追加可否。
- **`echarts_test.go` の既存アサーション**: 複数系列化で壊れる箇所の特定（無回帰の影響範囲）。
- **`ListDailySensorAggregates` の `STDDEV_*` 追加可否**（NUMERIC 母数・NULL/単一サンプル日の挙動）。

### 参照（権威）
- 数式: `2cc_sdd/分析アイデアメモ.md` 付録A（A. 基本統計量 / B. 移動平均系）。EMA/WMA 不採用。
- ECharts 表現方針: 同メモ §2-2 クラッタ対策 / §2-3 表現テク早見表（曲線追従帯＝2系列積み上げ area・既定で畳む＝`legend.selected:false`）。
- スキーマ: `docs/database_snapshot/table_definitions.md`（sensor_readings）。
- 依存規約: `.kiro/steering/structure.md`（依存方向・`internal/chart` 最下流純粋性・view→domain 表示メソッドのみ）。

---

# Discovery & Synthesis（design フェーズ・2026-06-27）

> Extension 型のため light discovery を実施。最大の Research Needed（go-echarts/v2 の表現力）を実コードで検証済み。

## 技術検証: go-echarts/v2 v2.7.2 の表現力（描画層設計の要）

モジュールキャッシュの型定義を実地確認した結果、**本機能の全要素がサーバ側・型安全に構築可能**と確定（クライアント `EChartsInitializer` の改変は不要）:

| 必要機能 | go-echarts API（v2.7.2） | 用途 |
|---|---|---|
| 既定オフ凡例 | `opts.Legend.Selected map[string]bool` + `WithLegendOpts` | SMA/正常帯/乖離率を option JSON で既定非表示（R2.2/3.3/4.2） |
| 積み上げ area 帯 | `opts.LineChart.Stack string` + `WithAreaStyleOpts(opts.AreaStyle{Opacity})` | 正常帯=帯下限(透明)＋帯幅(半透明塗り)の2系列（R3.2） |
| 透明ベース線 | `opts.LineStyle.Opacity types.Float` | 帯下限の線を消す |
| マーカー抑止 | `opts.LineChart.ShowSymbol types.Bool` | SMA/帯の点を消す |
| 第2 y軸 | `ExtendYAxis(opts.YAxis)` + `opts.LineChart.YAxisIndex int` | 乖離率%を右軸へ（実測スケール非混在・R4.3） |

- 注意（実装時の罠）: `omitempty` により `Opacity:0`/`ShowSymbol:false` が省略され意図せず描画される可能性。明示的な型値設定＋option JSON 構造アサートで担保。
- HTML 安全化は既存方針（`json.Marshal(line.JSON())`）を踏襲（§10-E・`</script>` 不混入）。

## 既存資産の配線状況（再確認）
- `ListDailySensorAggregates`: `DeviceRepo` interface（`device.go:50`）に宣言済みだが**実呼び出しなし＝未配線**。かつ **STDDEV なし**（σ/CV が出ない）。
- `EChartsInitializer`（`App.templ` L130-184）: endLabel/sampling を `option.series[0]` へ無条件付与。**生実測線を series[0] に保てば無変更で温存**（R7.3/7.5）。
- CSS 再利用: `.summary-grid`/`.summary-box`（readings 履歴で実績）→数値カード、`.data-table`→日次集計表。新規クラス最小（§31/§40-B）。

## Synthesis 結果

### 1. Generalization（一般化）
- R2(SMA)/R3(帯)/R4(乖離率) は「凡例既定オフのオーバーレイ系列」の変種 → **単一の複数系列ビルダー `ChartOptionJSON(ChartSpec)`** で統合（系列ごとの y軸/stack/areaStyle/legend 既定可視を `ChartSpec` で型表現）。
- R1(カード)/R5(日次表) は「生行からの集計」 → **共通の純関数群**（Mean/MinMax/DiurnalRange/StdDev/CV）を期間カードと日次表で再利用。

### 2. Build vs Adopt
- **統計＝自作**（`internal/chart/stats.go`）。gonum 等の統計ライブラリは過剰（軽量・ガードレール⑧「軽い統計はアプリ内計算」）。新規依存を増やさない。
- **描画＝go-echarts 採用**（既存スタック・全機能を型安全に提供と検証済）。
- **σ/CV＝SQL STDDEV を不採用、Go 計算を採用**。理由: (a) 生行は SMA/帯のため既に Go 側に取得済み、(b) 統計定義の単一源を保つ、(c) sqlc 改変・`any` 型 STDDEV ハンドリングを回避、(d) `ListDailySensorAggregates` は未配線・σ 非対応。→ **追加クエリ・`make sqlc`・DDL 変更ゼロ**（R8 整合）。

### 3. Simplification
- 新 package（`internal/analysis` 等）を作らず `internal/chart/stats.go` に集約（chart が既に最下流純粋層・消費者は1つ）。
- `ListDailySensorAggregates` の配線は据え置き（生行 Go 集計で代替）。
- オーバーレイは**全期間で生成・既定オフ**（短期限定の分岐を作らない＝簡素）。日次表のみ複数日限定（R5.3）。
- ユーザーによる N/k 編集 UI を持たない（要件外・固定既定 k=2・窓は period 別定数表）。
- `EChartsInitializer` 無変更（client リスク除去）。
- 正常帯は帯下限を凡例から除外し帯幅のみトグル → 1概念=1凡例項目（2項目化を回避）。

## 確定した設計判断（requirements の Unknown を解決）
1. SMA 窓幅 N: **点数窓**・period 別定数表（24h=12/3d=36/7d=72/30d=288, ~5分間隔前提）。立ち上がり=expanding window 部分平均。
2. k: **固定 k=2**（≈95%）。ユーザー編集 UI なし。
3. 乖離率の軸: **同チャート第2 y軸**（YAxisIndex=1・右軸%）。
4. σ/CV 経路: **Go 計算**（母標準偏差・N 除算）。SQL STDDEV 不採用。
5. `LineOptionJSON` 拡張形: **`ChartOptionJSON(ChartSpec)` へ置換**（唯一の呼び出し元 buildChartArea・テスト更新）。
6. per-series 属性: 新 `ChartSpec`（Labels/Unit/Color/Raw/SMA/BandLower/BandWidth/Deviation）で型表現。既存 `Point`/`Series` は温存。
7. endLabel 出し分け: 生線=series[0] 固定で **client 無変更**（series[1..] には付かない）。
8. 24h の日次表: **出さない**（カードで把握・R5.3）。

## 残リスク（design.md Open Questions と同期）
- 実測サンプリング間隔の確定後に `smaWindowFor` 要調整。
- 温度0℃近傍/負値で乖離率%が不安定（epsilon ガードで未定義化・湿度で特に有意）。
- go-echarts `omitempty` による透明/非表示指定の省略 → JSON 検証テストで担保。
- カード4項目のレイアウト（`.summary-grid` 3列流用 or 4列調整）は実装時にモックで確定。
