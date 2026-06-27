# フェーズ2（分析ロードマップ）spec-init プロンプト: 温湿度グラフ拡張（移動平均 SMA・正常帯 SMA±kσ・乖離率・日較差/数値カード・日次集計）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: temp-humidity-chart-stats
> 位置づけ: [分析アイデアメモ.md](../分析アイデアメモ.md) 第1章「実装ロードマップ」の**フェーズ2**〔明示〕。新規画面ではなく、実装済み device-show（S5）＋ ECharts 移行済みグラフ（E1）への**分析補助線・統計カードの上載せ拡張**。グラフのデータ源・期間切替 UX・URL は維持し、表現を足す。
> 確度: 〔明示〕（引継ぎメモ由来。SMA・乖離率は河野案の思いつき要素を含むが、提案型案件ゆえ積極採用＝分析アイデアメモ 付録C-3 の方針）。
> 前提セッション: S5（device-detail。device-show・期間切替フラグメント・最新計測）／ E1（device-chart-echarts。go-echarts による option JSON 構築・クライアント ECharts 描画・温湿度2グラフ connect 連動）
> **スキーマ変更なし**（P1 と異なりマイグレーション不要）。派生指標（SMA/σ/乖離率/日較差/CV）は既存 `sensor_readings` から**読み取り時に計算**する（ガードレール②＝派生列は必要になった時に足す YAGNI／ガードレール⑧＝軽い統計はアプリ内計算）。
> 設計フェーズで参照:
> - 上位ロードマップ: 分析アイデアメモ.md フェーズ2（◎ 実測線＋日較差カード ／ ○ SMA＋正常帯（2系列の積み上げ area 帯）・乖離率は legend トグル ／ △ EMA/WMA 不採用・3本並べない）／ §2-2 クラッタ対策（主役1〜2系列・legend `selected:false`・線より「帯」で見せる）／ §2-3 表現テク早見表（曲線追従帯＝**2系列の積み上げ area**／既定で畳む＝`legend.selected:false`）／ 第3章ガードレール⑧（計算層と描画層の分離・重い統計は CSV 外出しだが SMA/σ/CV は軽く**アプリ内可**）
> - 数式（権威）: 分析アイデアメモ.md 付録A —「A. 基本統計量」（平均μ・標準偏差σ・日較差ΔT=Tmax−Tmin・変動係数CV=σ/μ）／「B. 時系列・移動平均系」（SMA_t=(1/N)Σx・乖離率=(x−SMA)/SMA×100[%]・正常帯=SMA±kσ）。**EMA/WMA は採用しない**。
> - 現スキーマ（権威）: docs/database_snapshot/table_definitions.md「sensor_readings」（temperature numeric(5,2)・humidity numeric(5,2)・recorded_at timestamptz・device_id+recorded_at の部分索引あり）
> - 移行元・拡張対象コード（実コード確認済み）:
>   - `internal/chart/echarts.go` … `LineOptionJSON(series []Series, unit, color)`。**現状は series[0] の単一折れ線＋markPoint(max/min)＋tooltip(axis/cross) のみ**を option 化。endLabel/sampling はクライアント付与。→ **複数系列（生線＋SMA線＋帯）対応へ拡張する中心ファイル**。
>   - `internal/chart/series.go` … `Point{Label, Y}` / `Series{Name, Dashed, Points}`（`Name`＝凡例名・`Dashed`＝破線。現状未活用の余地あり）。
>   - `internal/handler/device_show.go` … `buildChartArea`（`ListRecentSensorReadings` で生データ取得→`rawSeries` で温度/湿度の単一系列へ写像→`LineOptionJSON` 2本）／`rawSeries`／`Show`・`Chart`（期間切替フラグメント）。**全期間（24h/3d/7d/30d）が生データ単一折れ線に統一済み**（直近コミット 9261f9d）。`aggregateToFloat`（NUMERIC→float 安全変換）も同 package にあり流用可。
>   - `internal/view/component/views.go` … `DeviceChartAreaView`（TemperatureOptionJSON/HumidityOptionJSON/Unit/Color/HasData）／`optionScript()`（`<script type="application/json">` 埋込ヘルパ・§10-E）。
>   - `internal/view/component/DeviceChartArea.templ` … 期間ボタン群＋`#temperature-chart`/`#humidity-chart`（`data-echarts data-unit data-color`）＋兄弟 option script。器（`.linked-charts`/`.chart-wrapper`/`<h2>`）はモック準拠。
>   - `internal/view/layout/App.templ` の `EChartsInitializer`（インライン script・約 L124-185）… `[data-echarts]` を走査し option JSON を `JSON.parse`→`dispose`→`init`→**endLabel/sampling をクライアント付与**→`setOption`、2インスタンスを `echarts.connect()` で連動。htmx:beforeSwap/afterSwap で dispose/再 init。
> - 集計クエリ（既存・拡張候補）: `db/queries/sensor_readings.sql` の `ListDailySensorAggregates`（日別 AVG/MAX/MIN/COUNT・現状未配線の可能性）／`GetSensorReadingsSummary`（期間 AVG/MAX/MIN/COUNT・S6 履歴で使用）。日次の σ/CV は STDDEV 追加 or Go 集計で補う。
> - モック（単一ソース運用の境界に注意）: `mocks/html/device-show.html`・`mocks/html/style.css`。**数値カード・日次集計表は静的な「器」＝HTML/CSS ゆえモックに反映必須**（feedback_mock_reflects_impl_visual）。一方**グラフ内部の SMA 線・正常帯・乖離率の描画は反映対象外＝グラフ描画例外**（feedback_mock_graph_rendering_exception）。
> - 命名・依存規約: .kiro/steering/structure.md（依存方向＝下向き一方向・`internal/chart` は gin/DB/templ/pgtype を import しない最下流純粋ユーティリティ・view→domain 表示メソッドのみ）／ tech.md（sqlc・データアクセス方針）／ CLAUDE.md。

--- spec-init 本文 ここから ---

## 機能概要

デバイス詳細画面（device-show）の温度・湿度グラフ（E1 で Apache ECharts へ移行済み・全期間とも生データ単一折れ線）に、**日常監視の補助線と統計サマリ**を上載せする。具体的には (1) **単純移動平均線 SMA（1本）** と、それを中心とする **正常帯バンド SMA±kσ**、(2) **移動平均からの乖離率（%）**、(3) **現在値／最高／最低／日較差（ΔT＝最高−最低）の数値カード**、(4) **日次の 平均／最高／最低／日較差／標準偏差σ／変動係数CV の集計表**を加える。グラフはクラッタを避けるため「**主役は生実測線（◎・既定表示）＋日較差カード**」を基本とし、**SMA・正常帯・乖離率は凡例（legend）トグルで既定オフ（`selected:false`）にして任意表示**（○）とする。**EMA/WMA・3本以上の移動平均は採用しない**（△）。正常帯は線を増やさず「**2系列の積み上げ area 帯**」で見せる。派生指標はすべて既存 `sensor_readings` から**読み取り時に計算**し、**スキーマ変更・マイグレーションは行わない**（ガードレール②/⑧）。期間切替（24h/3d/7d/30d）・URL 同期・温湿度2グラフ連動など E1 までの機能は**無回帰で維持**する。

## 背景・現状

E1（device-chart-echarts）完了後の現状は以下（実コード確認済み）。

- **計算層（描画前）**: `device_show.go` の `buildChartArea` が `ListRecentSensorReadings`（指定時刻以降・昇順の生データ）を取得し、`rawSeries` で温度・湿度それぞれ**単一系列**（`chart.Series{Points:[]Point{Label,Y}}`）へ写像する。**24h/3d/7d/30d すべて生データ単一折れ線**に統一済み（コミット 9261f9d。`ListDailySensorAggregates` は現状未配線の可能性）。
- **描画層（option 構築）**: `internal/chart/echarts.go` の `LineOptionJSON(series, unit, color)` が **series[0] のみ**を 1 本の line（＋markPoint max/min＋tooltip axis/cross）として ECharts option を組み、`encoding/json` で HTML 安全化した JSON を返す。endLabel（右端現在値）と sampling("lttb") は option に含めず、クライアントが付与する。
- **View/templ**: `DeviceChartAreaView` が温度/湿度の option JSON・単位・色・HasData を持ち、`DeviceChartArea.templ` が `#temperature-chart`/`#humidity-chart`（`data-echarts data-unit data-color`）＋兄弟 `<script type="application/json">` を描く。
- **クライアント**: `App.templ` の `EChartsInitializer` が `[data-echarts]` を走査し、option JSON を parse → `dispose`→`init`→**endLabel/sampling 付与**→`setOption`、温湿度2チャートを `echarts.connect()` で連動。HTMX swap 前後で dispose/再 init。
- **DB**: `sensor_readings`（temperature/humidity＝numeric(5,2)・recorded_at＝timestamptz）。集計は `ListDailySensorAggregates`（日別 AVG/MAX/MIN/COUNT）・`GetSensorReadingsSummary`（期間 AVG/MAX/MIN/COUNT）が既存。**σ・CV を出す集計は未整備**。
- **数値カード・日次集計表は device-show に存在しない**（現状はグラフと最新計測テーブルのみ）。

## このセッションのスコープ（実装対象）

### 計算層（純粋 Go・スキーマ非変更・読み取り時計算）

- **配置**: `internal/chart` は gin/DB/templ/pgtype 非依存の最下流純粋ユーティリティ。SMA・σ・乖離率・日較差・CV は**軽い統計**ゆえアプリ内計算とし（ガードレール⑧）、`internal/chart` に統計ヘルパを増設するか、同層の純粋サブ package（例 `internal/chart` 内 stats 関数群／新規 `internal/analysis` 等）に置く。**配置は依存方向ルール（structure.md）に従い設計フェーズで確定**（pgtype 変換は handler 側に留め、計算層は `[]float64`/`[]Point` を受ける純関数にする）。
- **SMA**: 単純移動平均 `SMA_t = (1/N)Σ_{i=t-N+1}^{t} xᵢ`。窓幅 N（または時間窓）は period／サンプリング間隔に応じて決める（→未確定事項）。立ち上がり（先頭 N−1 点）の扱い（NaN/欠落 or 部分平均）を定義。
- **正常帯 SMA±kσ**: 移動窓の標準偏差σで上下限 `SMA±k·σ`（既定 k=2≈95%／→未確定事項）。**描画は「2系列の積み上げ area」**前提で、下限系列（SMA−kσ・透明）＋帯幅系列（2kσ・半透明塗り）の2系列を生成する（§2-3）。
- **乖離率（%）**: `乖離率 = (x_t − SMA_t)/SMA_t × 100`。SMA が極小/0 近傍のゼロ除算ガードを設ける。
- **日較差・期間カード**: 現在値（最新点）／期間 最高・最低／日較差 ΔT＝最高−最低 を温度・湿度別に算出。
- **日次集計（複数日期間）**: 日別の 平均μ／最高／最低／日較差ΔT／標準偏差σ／変動係数 CV＝σ/μ。

### 描画層（echarts.go の複数系列拡張）

- `LineOptionJSON` を**複数系列対応へ拡張**（後方互換を保つか新関数を足すかは設計判断）。1チャートにつき:
  - **生実測線（◎・既定表示）**＝従来どおり markPoint(max/min)。
  - **SMA 線（○・`legend.selected:false` で既定オフ）**。
  - **正常帯（○・既定オフ）**＝下限系列（areaStyle 透明・stack 同一グループのベース）＋帯幅系列（areaStyle 半透明・showSymbol:false・lineStyle 透明）の **2系列積み上げ area**。
  - **乖離率（○・既定オフ）**＝別 y 軸 or 別パネルで（同一スケール混在を避ける。設計で y 軸構成を決定）。
- これに伴い `Series`/option 生成に **per-series 属性**（凡例名・stack 名・areaStyle 色/不透明度・lineStyle 色/破線・showSymbol・legend 既定可視）を持たせる。`legend` を option に追加し、○ 系列は `selected` で既定オフにする。
- HTML 安全化（`encoding/json`・`</script>` 不混入）は現状方針を踏襲。

### View / templ / クライアント

- `DeviceChartAreaView` を拡張系列を含む option JSON＋**数値カード値（現在値/最高/最低/日較差）＋日次集計表行**を保持する型へ（イミュータブル＝handler で組み立て View へ詰める）。
- `DeviceChartArea.templ`: グラフ器は維持しつつ、**数値カード（温度/湿度の 現在値・最高・最低・日較差）と日次集計表**を追加描画（HasData=false 時は従来同様の空表示）。カード/表は新規の静的「器」＝**モック反映対象**。
- `App.templ` の `EChartsInitializer`: legend 既定状態は option JSON（サーバ構築・`selected:false`）で表現されるため**原則クライアント変更は最小**。endLabel を生実測線のみに付ける（SMA/帯/乖離率には付けない）点と、複数系列でも connect 連動が壊れないことを確認。必要最小の調整に留める。

### 集計クエリ（必要なら拡張）

- 日次の σ/CV を SQL で出すなら `ListDailySensorAggregates` に `STDDEV_*`（→σ）を追加し CV=σ/μ を算出（あるいは生データから Go で日次集計）。**SQL を変えたら `make sqlc`**。**DDL は変更しない**（集計クエリのみ）。

### モック反映

- **数値カード・日次集計表**を `mocks/html/device-show.html`＋`mocks/html/style.css`（正本）へ反映し `make sync-css`（feedback_mock_reflects_impl_visual・project_css_single_source）。**グラフ内部の SMA 線・正常帯・乖離率の描画はモック反映対象外**（feedback_mock_graph_rendering_exception）。

## スコープ外（このセッションでやらないこと）

- **EMA/WMA・3本以上の移動平均**（△・不採用）。複数窓幅の同時表示。
- **派生指標列の DB 追加・マイグレーション**（読み取り時計算で足りる＝ガードレール②）。`sensor_readings` スキーマ・受信 API・既存クエリ（`ListRecentSensorReadings` 本体）の変更。
- **VPD・露点・GDD・THI 等の農学派生指標**（フェーズ3/6/7/12 ＝別 spec）。本フェーズは生温湿度＋基本統計（付録A の A/B）に限る。
- **CSV エクスポート・帳票化**（フェーズ4）。**STL分解・ACF・Mann-Kendall・回帰トレンド・予測**（重い統計＝ガードレール⑧で CSV 外出し・フェーズ8/15）。
- **アラート判定・しきい値編集との連動**（正常帯は監視補助の可視化であって alert_rules とは別）。
- **device-show 以外の画面**（dashboard カード・readings 履歴・他画面）へのグラフ/統計展開。最新計測テーブル・情報パネル・削除モーダルの変更。
- **作物別の適正帯切替**（作物マスタ＝フェーズ3）。正常帯は SMA±kσ の統計的帯であって作物適正帯ではない。
- 認証・所有者認可・MethodOverride・CSRF・期間バリデーション本体（S1/S5/E1 所有・消費のみ）。

## 技術制約・準拠事項

- **計算層と描画層の分離**（ガードレール⑧）。計算は純粋 Go（`[]float64`/`[]Point` 入出力・gin/DB/templ 非依存）、描画は echarts.go の option 構築に集中。重い統計は持ち込まない。
- **依存方向**（structure.md）: 下向き一方向。`internal/chart` の最下流純粋性（外部描画ライブラリ go-echarts への依存は E1 の整合範囲）を維持。View は repository/service を import しない。pgtype→float 変換は handler 境界に留める。
- **ECharts 表現**（§2-2/§2-3）: 1チャート＝主役1〜2系列。○ 系列は `legend.selected:false` で既定オフ。正常帯は **2系列積み上げ area**（線を増やさない）。生実測線が常に主役。
- **go-echarts v2**: option は型安全に組める範囲で構築し、HTML 安全 JSON で供給（E1 方式踏襲）。型で出せない属性はクライアント付与（endLabel/sampling の既存パターン）。
- **HTMX**: 期間切替は現状の hx-get/hx-target/hx-swap(innerHTML)/hx-push-url を維持。swap 後も ECharts init/dispose・legend 既定状態が正しく再現されること。
- **イミュータブル**: sqlc 生成構造体は読取専用。handler で View を組み立てる。
- **CSS**: 独自クラスの新設は最小化しモック単一ソース（カード/表は先行反映→`make sync-css`）。グラフ内部描画は反映例外。
- **言語**: 日本語コメント・エラー・コミット。コード識別子は英語。
- **TDD**: 80% 以上。計算（SMA・σ・乖離率・日較差・CV・立ち上がり/ゼロ除算/空データ境界）・option 構築（系列数・legend selected・stack/areaStyle・乖離率の y 軸分離）・handler 回帰（Show/Chart の期間・空データ・無回帰）・カード/表の描画。

## 受け入れ基準（概略）

1. **計算正当性**: SMA・SMA±kσ・乖離率(%)・日較差ΔT・σ・CV が付録A の定義どおり算出され、立ち上がり（先頭 N−1 点）・空データ・SMA≈0 のゼロ除算が安全に扱われる。
2. **既定表示（◎）**: 初期表示・期間切替後とも**生実測線＋日較差/現在値/最高/最低の数値カード**が既定で見える。クラッタなく主役が生線。
3. **トグル表示（○）**: 凡例から **SMA 線・正常帯（SMA±kσ の積み上げ area 帯）・乖離率(%)** を既定オフから任意で表示でき、正常帯は塗り帯として見える。
4. **日次集計表**: 複数日期間（3d/7d/30d）で日別 平均/最高/最低/日較差/σ/CV が表で確認できる（24h の扱いは設計どおり）。
5. **無回帰**: 期間切替（24h/3d/7d/30d・アクティブ往復・URL 同期）・温湿度2グラフ連動・空データ表示が E1 同等に動作する。
6. **不変条件**: **スキーマ変更/マイグレーションなし**（派生指標は読み取り時計算）。○ 系列は EMA/WMA を含まず SMA 1本のみ。
7. **モック整合**: 数値カード・日次集計表はモック（device-show.html＋style.css 正本）に反映。グラフ内部描画（SMA/帯/乖離率）は反映例外。
8. **テスト 80% 以上**。

## 未確定事項・要確認（設計フェーズで決定）

- **SMA の窓幅 N（または時間窓）**: サンプリング間隔（約5分想定・分析アイデアメモ第4章で未確定）と period に依存。点数固定 N か時間窓（例 24h は数時間・30d は日単位）か。立ち上がり処理（部分平均 or 欠落）も併せて決定。
- **k（±kσ）の既定値**: 既定 k=2（≈95%）を本命に、設定可否（固定 or 軽い切替）を判断。
- **乖離率の見せ方**: 別 y 軸（同チャート右軸）か別パネルか。スケール混在を避ける y 軸構成。
- **24h と複数日での出し分け**: 日次集計表は複数日のみか、24h は時別/カードのみか。SMA/帯は全期間で出すか短期のみか。
- **σ/CV の算出経路**: SQL（`STDDEV_*` 追加）で日次集計するか、生データから Go 計算か（母/標本標準偏差の別も）。
- **`LineOptionJSON` の拡張形**: 既存シグネチャ拡張か新関数追加か（E1 のテスト資産との整合・後方互換）。
- **`Series` の per-series 属性**: 既存 `Name`/`Dashed` を活かしつつ stack/areaStyle/legend 既定可視/showSymbol をどう表現するか。
- **クライアント endLabel**: 生実測線のみへ付与し SMA/帯/乖離率には付けない出し分けを option/初期化のどちらで担保するか。

--- spec-init 本文 ここまで ---
