# フェーズ3（分析ロードマップ）spec-init プロンプト: VPD適正帯ダッシュボード（飽差VPD時系列・適正帯markArea・滞在率gauge・時間帯別逸脱・VPD移動平均）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: vpd-dashboard
> 位置づけ: [分析アイデアメモ.md](../分析アイデアメモ.md) 第1章「実装ロードマップ」の**フェーズ3**〔強く示唆〕。新規画面ではなく、実装済み device-show（S5）＋ ECharts 移行済みグラフ（E1）＋ 統計オーバーレイ（P2 ＝ temp-humidity-chart-stats）への、**派生指標 VPD（飽差）パネルの追加**。生 RH でなく植物生理を支配する VPD で環境を語る＝Ambient にない差別化の核（「ただのグラフ」→「農業に効くダッシュボード」）。温湿度グラフ・期間切替 UX・URL は維持し、別パネルで VPD を足す。
> 確度: 〔強く示唆〕（引継ぎメモの確定依頼ではなくスタンプ側の仮説だが、**提案型案件ゆえ積極採用**＝分析アイデアメモ 付録C-3 の方針）。ゴールが「環境最適化（飽差を適正帯に）」なら付録B-3(c) によりフェーズ2と入替えて最初に置いてもよい位置づけ。
> 前提セッション: S5（device-detail。device-show・期間切替フラグメント・最新計測）／ E1（device-chart-echarts。go-echarts による option JSON 構築・クライアント ECharts 描画・温湿度2グラフ connect 連動）／ **P2（temp-humidity-chart-stats。複数系列 ChartSpec/ChartOptionJSON・純粋統計層 internal/chart/stats.go・数値カード/日次集計の handler パターン。本フェーズはこの基盤の直接の拡張）**／ **S4（device-create-edit。DeviceForm・フルページ POST・入力値復元・Tom Select。作物 select を足すため）**／ **P1（device-location-select。`devices.locality` 列・Go 定数マスタ・00008 の先行事例。作物マスタを同型で実装）**
> **VPD 本体はスキーマ変更なし**（P2 同様）。VPD・適正帯滞在率・時間帯別逸脱・VPD移動平均は既存 `sensor_readings`（temperature＋humidity の2列）から**読み取り時に計算**する（ガードレール②＝派生列は必要になった時に足す YAGNI／ガードレール⑧＝軽い指標はアプリ内計算）。**唯一のスキーマ拡張＝作物マスタ**: 作物別の VPD 適正帯切替のため `devices.crop`（nullable VARCHAR）を1列追加する（goose **00009**・P1 の locality と同型・後 `make db-snapshot`）。**未選択（NULL）は既定しきい値 0.3〜1.5 kPa にフォールバック**（ガードレール⑥「無ければ既定しきい値」）。
> 設計フェーズで参照:
> - 上位ロードマップ: 分析アイデアメモ.md フェーズ3（◎ VPD時系列＋**適正帯 markArea**＋滞在率 gauge ／ 線を増やさず帯で見せる）／ §2-2 クラッタ対策（主役1〜2系列・派生指標は**温湿度と別パネル/タブ**・線より「帯」で見せる）／ §2-3 表現テク早見表（固定しきい値の帯＝**markArea**(yAxis 範囲指定)／滞在率%・単一スコア＝**gauge**／値で線色を変える＝**visualMap**／期間ズーム＝dataZoom）／ §2-4（フェーズ3 ◎主役＝VPD＋適正帯markArea＋滞在率gauge）／ 第3章ガードレール ④研究用画面と農家共有画面の分離（農家共有＝フェーズ13・本フェーズは**研究用**に特化）・⑤「時間割合（滞在率）」の発想（瞬間値/平均でなく「1日の何%が適正帯か」）・⑥作物メタデータの保持（VPD適正帯は作物依存・無ければ既定しきい値）・⑧計算層と描画層の分離／ 付録B-3(c)（環境最適化ゴールなら前倒し可）・B-4（施設果菜＝ゴーヤ/インゲンは VPD 本命／露地＝サトウキビ/米は VPD 非適用＝GDD・THI 前面。作物マスタで切替える前提）
> - 数式（権威）: 分析アイデアメモ.md 付録A —「D①飽差 VPD（最重要）」。**飽和水蒸気圧 es(T)=0.6108·exp(17.27·T/(T+237.3)) [kPa]（Tetens式）／ 実水蒸気圧 ea=es(T)·RH/100 ／ 飽差 VPD=es(T)−ea=es(T)·(1−RH/100) [kPa]**。定数 0.6108・17.27・237.3 は Tetens 式の確定値（変更禁止）。多くの作物で適正帯 **0.3〜1.5 kPa**。VPD移動平均は付録A「B. 時系列」の SMA（**P2 の `chart.SMA` をそのまま流用**）。
> - 現スキーマ（権威）: docs/database_snapshot/table_definitions.md「sensor_readings」（temperature numeric(5,2) CHECK -40〜125・humidity numeric(5,2) CHECK 0〜100・recorded_at timestamptz・(device_id, recorded_at DESC) 部分索引 WHERE deleted_at IS NULL）。VPD は temperature＋humidity から読み取り時計算できる＝**スキーマ非変更**で足る。「devices」（**locality VARCHAR(20) は P1/00008 で追加済・crop 列は未実装**）。goose 連番の最新は **00008**。
> - マスタ規約（必読・本フェーズで作物マスタを新設）: .kiro/steering/structure.md §98-100（**マスタは DB テーブルでなく Go 定数＋VARCHAR＋CHECK／FK 張らない**）。手本 = internal/domain/metric.go（type string＋const＋Label()/Valid()/All...()・fmt のみ依存）。先行事例 = P1 の internal/domain/locality.go・municipality.go ＋ db/migrations/00008_add_locality_to_devices.sql（Go 定数集合を CHECK にミラー・列追加後 `make db-snapshot`）。**作物（9）＝ ゴーヤ／インゲン／サトウキビ／マンゴー／パイナップル／ウリ（瓜類）／米／いも（芋類）／葉野菜**（ユーザー指定。識別子は英語）。施設果菜（ゴーヤ/インゲン/ウリ/マンゴー/葉野菜）は VPD 本命・露地（サトウキビ/米/パイナップル/いも）は VPD 適用性が低い（付録B-4）＝適正帯の具体値は research で確定（**沖縄の実地知見＝ユーザー言明を権威**／文献）。
> - 移行元・拡張対象コード（実コード確認済み・P2 マージ後の現状）:
>   - `internal/chart/stats.go` … 純粋統計層（`math` のみ依存・[]float64 入出力・gin/DB/templ/time 非依存）。**既存** `SMA(values,window)`・`MovingStdDev`・`Band(sma,sigma,k)`・`Deviation(values,sma,eps)`・`Mean`・`MinMax`・`DiurnalRange`・`StdDev`・`CV(values,eps)→(float64,bool)`。→ **VPD 純関数（`VPD(temp,rh)`／系列版）・適正帯滞在率（在帯割合）・時間帯別逸脱集計の純関数を増設する中心ファイル**。
>   - `internal/chart/echarts.go` … **唯一の公開 option 関数** `ChartOptionJSON(spec ChartSpec) (string, error)`（複数系列・凡例・第2y軸・stack 積み上げ area・markPoint・HTML 安全 JSON）。series[0]=生実測線＋markPoint(max/min)、SMA/正常帯/乖離率は `legend.selected:false` で既定オフ。**markArea（固定しきい値帯）も gauge も未実装＝本フェーズで新規追加**。
>   - `internal/chart/series.go` … 入力契約 `ChartSpec`（Labels/Unit/Color/Raw/SMA/BandLower/BandWidth/Deviation の8フィールド）。→ VPD 用に拡張するか **VPD 専用 spec 型/関数を足すか**は設計判断（後方互換＝温湿度2グラフのテスト資産を壊さない）。
>   - `internal/handler/device_show.go` … `buildChartArea(ctx,deviceID,period,now)→component.DeviceChartAreaView`。`ListRecentSensorReadings`（昇順生データ）→ `pgconv.NumericToFloat` で float 列化 → `overlaySpec(...)` で `ChartSpec` 組立 → `ChartOptionJSON` ／ `statCard()`（現在値/最高/最低/日較差）／ `dailyStatRows()`（JST 暦日バケット・平均/最高/最低/日較差/σ/CV）。固定値 `bandSigmaK=2.0`・`statEpsilon=1e-9`・`smaWindowFor(period)`（24h=12/3d=36/7d=72/30d=288点・約5分間隔前提）・`jst=time.FixedZone("JST",9*60*60)`・`statEmptyMark="—"`。`Show`（GET /devices/:device）と `Chart`（GET /devices/:device/chart?period=・oneof=24h 3d 7d 30d）が両者とも buildChartArea を呼ぶ。→ **VPD 系列・滞在率・時間帯別逸脱を計算して View へ詰める拡張点**（時刻が要る時間帯バケットは dailyStatRows と同様 handler 境界で行い、stats 層は []float64 純関数に保つ）。
>   - `internal/view/component/views.go` … `DeviceChartAreaView`（DeviceID/Period/HasData/温湿度 OptionJSON・Unit・Color・Card・ShowDaily・Daily 行）／ `StatCardView`（Current/Max/Min/Diurnal）／ `DailyStatRow`（Date/Avg/Max/Min/Diurnal/Sigma/CV）／ `optionScript(id,json)`（`<script type="application/json">` 埋込）。→ VPD OptionJSON・滞在率・VPD カード・時間帯別逸脱の DTO を**イミュータブルに追加**。
>   - `internal/view/component/DeviceChartArea.templ` … 期間ボタン（.period-selector）＋数値カード（@statCards・.summary-grid-4）＋グラフ器（.linked-charts/.chart-wrapper・`#temperature-chart`/`#humidity-chart`＝`data-echarts data-unit data-color`＋兄弟 option script）＋日次集計表（@dailyTable・.data-table）。→ **VPD パネル（別ブロック・別 chart 器）と滞在率 gauge/カードと時間帯別逸脱表を追加描画**。器はモック準拠（独自クラス新設は最小）。
>   - `internal/view/layout/App.templ` の `EChartsInitializer`（L124-203）… `[data-echarts]` を走査し option JSON を parse→`dispose`→`init`→**series[0] のみ** endLabel/sampling("lttb") 付与→tooltip.formatter 上書き→`setOption`、複数インスタンスを `echarts.connect()` で連動。htmx:beforeSwap/afterSwap で dispose/再 init。→ **line 専用の初期化**ゆえ、gauge を ECharts で出すなら初期化経路の一般化が要る（→未確定事項）。VPD line chart の connect 参加可否（kPa は別 y スケール）も判断。
> - 集計クエリ（参考・本フェーズは読み取り時計算が既定）: `db/queries/sensor_readings.sql` の `ListRecentSensorReadings`（生データ・P2/本フェーズが使用）／`ListDailySensorAggregates`・`GetSensorReadingsSummary`（既存集計・VPD は2列導出ゆえ SQL 事前集計に不向き＝生データ取得→Go で VPD 化が素直）。
> - モック（単一ソース運用の境界に注意）: `mocks/html/device-show.html`・`mocks/html/style.css`（P2 で .summary-grid-4 カード・.data-table 日次集計表は反映済み）。**VPD パネルの器・VPD 数値カード・滞在率 gauge の枠・時間帯別逸脱表は静的な「器」＝HTML/CSS ゆえモックに反映必須**（feedback_mock_reflects_impl_visual）。一方**グラフ内部の VPD 線・適正帯 markArea 塗り・gauge 値・legend 状態の描画は反映対象外＝グラフ描画例外**（feedback_mock_graph_rendering_exception）。既使用色＝温度 `#e8590c`／湿度 `#1971c2`。**VPD 用の新色を定義**。
> - 命名・依存規約: .kiro/steering/structure.md（依存方向＝下向き一方向・`internal/chart` は gin/DB/templ/pgtype/time を import しない最下流純粋ユーティリティ・view→domain 表示メソッドのみ）／ tech.md（sqlc・データアクセス方針）／ CLAUDE.md（**マイグレーション変更後は** `make db-snapshot`＝VPD本体は変更なし・作物マスタの `devices.crop` 追加で 00009＋snapshot）。

--- spec-init 本文 ここから ---

## 機能概要

デバイス詳細画面（device-show）に、温度・湿度から導出する**飽差 VPD（Vapor Pressure Deficit）の適正帯ダッシュボード**を追加する。生の相対湿度ではなく、植物の蒸散・気孔開閉を支配し施設園芸の環境制御が事実上の目標値とする **VPD** で環境を語ることで、Ambient にない差別化の核（「ただのグラフ」から「農業に効くダッシュボード」へ）を作る。具体的には (1) **VPD 時系列の折れ線**と、それに重ねる **適正帯の塗り分け（markArea）**（乾きすぎ ＜下限／適正／湿りすぎ ＞上限。多くの作物で 0.3〜1.5 kPa）、(2) **適正帯滞在率（日次%＝1日の計測のうち適正帯に入っていた割合）を gauge（または数値カード）で**、(3) **時間帯別の VPD 逸脱**（時刻バケットごとの在帯/逸脱）、(4) **VPD 移動平均トレンド（SMA・既定オフのトグル）**を加える。VPD パネルは温湿度グラフのクラッタを避けるため**温湿度と別パネル**に置き、適正帯は線を増やさず「**固定しきい値帯 markArea**」で見せる（分析アイデアメモ §2-2）。VPD・滞在率・逸脱・VPD移動平均はすべて既存 `sensor_readings`（temperature＋humidity）から**読み取り時に計算**し、**VPD 本体のスキーマ変更・マイグレーションは行わない**（ガードレール②/⑧）。適正帯のしきい値は**パラメータ化**（下限/上限 kPa）したうえで、**作物マスタを新設して作物別に切替える**: device に設定された作物（9種＝ゴーヤ／インゲン／サトウキビ／マンゴー／パイナップル／ウリ／米／いも／葉野菜）の適正帯を使い、**未選択なら既定 0.3〜1.5 kPa にフォールバック**（ガードレール⑥・「無ければ既定しきい値」）。本フェーズは**研究用画面**に特化し、農家向けの平易表示（信号色・適正帯バンド）はフェーズ13へ分離する（ガードレール④）。E1/P2 までの機能（温湿度2グラフ・統計オーバーレイ・期間切替 24h/3d/7d/30d・URL 同期・connect 連動）は**無回帰で維持**する。

## 背景・現状

P2（temp-humidity-chart-stats）マージ後（commit 3e17a73）の現状は以下（実コード確認済み）。

- **計算層（純粋）**: `internal/chart/stats.go` が `math` のみ依存の純関数群（`SMA`/`MovingStdDev`/`Band`/`Deviation`/`Mean`/`MinMax`/`DiurnalRange`/`StdDev`/`CV`）を提供。すべて `[]float64` 入出力で gin/DB/templ/time 非依存。**VPD・滞在率・時間帯別逸脱の関数はまだ無い**。
- **描画層（option 構築）**: `internal/chart/echarts.go` の唯一の公開関数 `ChartOptionJSON(spec ChartSpec) (string, error)` が複数系列の line option（series[0]=生実測線＋markPoint(max/min)、SMA/正常帯/乖離率は `legend.selected:false` で既定オフ、正常帯＝下限透明＋帯幅半透明 area の stack 積み上げ、乖離率＝第2y軸）を HTML 安全 JSON で返す。**markArea（固定しきい値帯）も gauge も未実装**。
- **入力契約**: `internal/chart/series.go` の `ChartSpec`（Labels/Unit/Color/Raw/SMA/BandLower/BandWidth/Deviation の8フィールド）。
- **handler**: `device_show.go` の `buildChartArea` が `ListRecentSensorReadings`（昇順生データ）を取得→`pgconv.NumericToFloat` で float 化→`overlaySpec` で `ChartSpec` 組立→`ChartOptionJSON`、`statCard`（現在値/最高/最低/日較差）と `dailyStatRows`（JST 暦日バケットの 平均/最高/最低/日較差/σ/CV、複数日のみ）。`Show` と `Chart`（期間フラグメント）が共通でこれを呼ぶ。固定値 `bandSigmaK=2.0`・`statEpsilon=1e-9`・`smaWindowFor`（24h=12/3d=36/7d=72/30d=288点）・JST・`statEmptyMark="—"`。
- **View/templ**: `DeviceChartAreaView`／`StatCardView`／`DailyStatRow`／`optionScript`。`DeviceChartArea.templ` が期間ボタン・数値カード（.summary-grid-4）・温湿度2グラフ（`#temperature-chart`/`#humidity-chart`＝`data-echarts`）・日次集計表（.data-table）を描く。
- **クライアント**: `App.templ` の `EChartsInitializer` が `[data-echarts]` を走査し、dispose/init・**series[0] のみ** endLabel/sampling 付与・tooltip.formatter 上書き・`echarts.connect()`。HTMX swap 前後で dispose/再 init。**line 前提の初期化**で gauge は想定外。
- **DB**: `sensor_readings`（temperature/humidity＝numeric(5,2)・recorded_at＝timestamptz）。`devices` は P1 で `locality VARCHAR(20)` を追加済み（00008）・**`crop` 列は無い**。VPD は temperature＋humidity の2列から導出でき、**スキーマ非変更**で計算可能。
- **VPD パネル・滞在率・時間帯別逸脱は device-show に存在しない**（現状は温湿度グラフ＋統計カード/表＋最新計測テーブルのみ）。

## このセッションのスコープ（実装対象）

### 計算層（純粋 Go・スキーマ非変更・読み取り時計算）

- **配置**: `internal/chart` の最下流純粋性（gin/DB/templ/pgtype/**time** 非依存・`[]float64`/スカラ入出力）を維持。VPD・滞在率は**軽い指標**ゆえアプリ内計算（ガードレール⑧）。時刻が要る計算（時間帯バケット）は **handler 境界**で行い、stats 層には持ち込まない（dailyStatRows のパターン踏襲）。
- **VPD 純関数**: `VPD(temp, rh float64) float64`（Tetens 式 `es(T)=0.6108·exp(17.27·T/(T+237.3))`、`VPD=es(T)·(1−RH/100)`）と、温湿度の同長スライスから VPD 系列を返す版。**定数 0.6108/17.27/237.3 は確定値**。RH の 0/100、氷点下温度（物理的に有効だが数値安定性）、欠測点の扱いを定義。
- **適正帯滞在率**: `TimeInRange(values []float64, lower, upper float64) float64`（適正帯 [lower,upper] に入った点の割合＝0〜1）。日次%は handler で JST 暦日バケットごとに算出（dailyStatRows と同様）。瞬間値・平均でなく**在帯割合**（ガードレール⑤）。
- **時間帯別逸脱**: 時刻バケット（例 1時間 or 時間帯区分）ごとに、VPD の 平均／在帯率／逸脱方向（乾き/湿り）を集計。**バケット化は handler**（時刻を持つ）・各バケットの統計は stats 純関数で。
- **VPD 移動平均**: 既存 `chart.SMA(vpdValues, window)` を**そのまま流用**（窓幅は `smaWindowFor` を共用）。
- **適正帯しきい値**: 下限/上限 kPa を**パラメータ**として受ける（既定 0.3/1.5）。**ハードコードせず**計算・描画の両層に引き回す。

### 描画層（echarts.go の markArea／gauge 拡張）

- **markArea（新規）**: 固定しきい値帯を yAxis 範囲指定で塗る汎用機構を `echarts.go` に追加。VPD 適正帯を **3 ゾーン**（＜下限＝乾きすぎ／[下限,上限]＝適正／＞上限＝湿りすぎ）に塗り分ける（色は控えめ・適正＝中立、逸脱＝警告色）。**SMA±kσ の「2系列積み上げ area（追従帯）」とは別手法**＝固定帯は markArea（混同しない）。
- **VPD line option**: VPD 系列（◎・主役・現在値 endLabel）＋適正帯 markArea を持つ option を構築。**滞在率に応じた線色変化（visualMap）は線を増やさず帯で見せる方針の補助**で△（任意・設計判断）。
- **gauge（新規・または非 ECharts 代替）**: 滞在率%を単一スコアで示す gauge を追加。**`EChartsInitializer` が line 前提**ゆえ、(a) 初期化経路を gauge 対応へ一般化するか、(b) 滞在率を**数値カード＋簡易バー**（CSS）で見せて gauge を見送るかは設計判断（→未確定事項）。◎ は gauge だがカード化は許容。
- HTML 安全化（`encoding/json`・`</script>` 不混入）は現状方針を踏襲。VPD chart の y 軸単位は **kPa**。

### View / templ / クライアント

- `DeviceChartAreaView` を **VPD パネル分の DTO**（VPD OptionJSON・滞在率値/gauge OptionJSON・VPD 数値カード〔現在VPD・期間平均VPD・適正帯滞在率・最大逸脱 等〕・時間帯別逸脱行）で拡張（イミュータブル＝handler で組み立て）。
- `DeviceChartArea.templ`: 温湿度グラフ・統計カード・日次表の**下（または別タブ）に VPD パネルを追加**（§2-2＝派生指標は別パネル）。`#vpd-chart`（`data-echarts data-unit="kPa" data-color=<新色>`）＋兄弟 option script、滞在率 gauge/カード、時間帯別逸脱表。器（カード/表/gauge 枠）は**モック反映対象**・独自クラス新設は最小。
- `App.templ` の `EChartsInitializer`: VPD line chart は既存走査で init される（series[0] のみ endLabel/sampling は無回帰）。**connect 連動に VPD を含めるか**（時間軸 axisPointer 共有は有用だが y スケールが kPa で別）と、**gauge を ECharts で出す場合の初期化分岐**を必要最小で調整。温湿度2グラフの既存挙動は壊さない。

### 作物マスタ・作物別 VPD 適正帯（本フェーズに含む）

structure.md §98-100 と P1（locality）の先行事例に厳密に従う。**作物マスタは P3/P6/P7/P13 が共有する基盤**だが、本フェーズでは VPD 適正帯の切替に必要な最小範囲で起こす（GDD の Tbase・病害モデル等は P6/P7 で同じ `Crop` 型へ非破壊追加）。

- **`internal/domain/crop.go`（Go 定数マスタ・テーブル化しない）**: `type Crop string` ＋ **9 作物定数** ＋ `Label()`（日本語表示名）／`Valid()`／`VPDRange() (lower, upper float64)`（作物別 VPD 適正帯 kPa）／`AllCrops()`（フォーム選択肢・集計軸）。`fmt` のみ依存・`internal/domain/metric.go` と P1 の `locality.go` を写経。
  - **作物（9・ユーザー指定）**: ゴーヤ／インゲン／サトウキビ／マンゴー／パイナップル／ウリ（瓜類）／米／いも（芋類）／葉野菜。識別子は英語（例 goya・ingen・sugarcane・mango・pineapple・uri・rice・imo・leafy_vegetable）。
  - **VPD 適正帯**: 既定（`crop` 未選択 = NULL）は **0.3〜1.5 kPa**。施設果菜（ゴーヤ/インゲン/ウリ/マンゴー/葉野菜）は VPD 本命（付録B-4）ゆえ作物別の適正帯を持たせる。露地・VPD 適用性が低い作物（サトウキビ/米/パイナップル/いも）は VPD が環境制御指標として効きにくい（付録B-4）ため、**適正帯は既定踏襲か参考表示**とする。**各作物の具体的な下限/上限 kPa はユーザー（沖縄の実地知見＝権威）／文献で research フェーズに確定**（→ 未確定事項）。spec 段階では構造（Crop→(lower,upper)）と既定フォールバックを確定する。
- **`devices.crop`（goose 00009・DDL のみ）**: `devices` に `crop VARCHAR`（nullable）を ALTER ADD ＋ `CONSTRAINT ... CHECK (crop IS NULL OR crop IN (<9作物値>))` ＋ 日本語 COMMENT。**FK 無し**（structure.md §98-100）。Down で列 DROP。**DML（backfill）は含めない**（既定フォールバックで既存 device はそのまま動く）。変更後 `make db-snapshot`（CLAUDE.md）。**親市町村のように導出列は不要**。
- **sqlc**: `CreateDevice`/`UpdateDevice` に `crop` を追加（locality と同じ要領・空→NULL）。`make sqlc` 再生成。
- **フォーム（DeviceForm.templ・S4）**: 栽培作物の**単一の検索可能 select**（`class="js-tom-select"`・`AllCrops()` を `[]SelectOption`・先頭に空 option「選択しない（既定しきい値）」）。locality select と同じパターン（カスケード無し・App.templ 改変なし・hx 属性なし）。入力値復元・procedural 検証（`Crop.Valid()`・空許容・early-return せず errors 累積）。
- **device-show VPD パネル**: その device の `crop` から `VPDRange()` で適正帯 markArea のしきい値を決定（**未選択なら既定 0.3/1.5 kPa**）。VPD パネルに作物名（`Crop.Label()`・未設定は「既定」）を表示し、どの適正帯で見ているかを明示。

### モック反映

- **VPD パネルの器・VPD 数値カード・滞在率 gauge の枠・時間帯別逸脱表**を `mocks/html/device-show.html`＋`mocks/html/style.css`（正本）へ反映し `make sync-css`（feedback_mock_reflects_impl_visual・project_css_single_source）。**グラフ内部の VPD 線・適正帯 markArea 塗り・gauge 値・legend 状態の描画はモック反映対象外**（feedback_mock_graph_rendering_exception）。

## スコープ外（このセッションでやらないこと）

- **露点 Td・結露帯・病害リスク**（フェーズ6 ＝ dewpoint-disease-risk）・**GDD 積算/収穫予測**（フェーズ7）・**THI/熱帯夜/絶対湿度AH**（フェーズ12）。本フェーズの派生指標は **VPD（付録A D①）に限る**。
- **VPD 等の派生指標列の DB 追加・マイグレーション**（読み取り時計算で足りる＝ガードレール②）。`sensor_readings` スキーマ・受信 API・既存クエリ本体の変更。**スキーマ拡張は `devices.crop` 1列のみ**（00009）。
- **作物マスタの VPD 適正帯以外の属性**（GDD の Tbase・病害モデル・THI しきい値等）。これらは P6/P7/P12 で同じ `domain.Crop` 型へ**非破壊追加**する（本フェーズは VPD 適正帯のみ）。作物別の地域・栽培形態（施設/露地）マスタ化や圃場エンティティも対象外（P10/P13）。
- **CSV エクスポート・帳票化・滞在率の表出力**（フェーズ4 ＝ sensor-data-export）。**STL/ACF/Mann-Kendall・予測・重い統計**（ガードレール⑧で CSV 外出し・フェーズ8/15）。
- **多地点の VPD 比較（内外/東西の差分）**（フェーズ10 ＝ multipoint-compare。複数センサ採用が前提）。本フェーズは単一デバイスの VPD。
- **農家向けの平易表示（信号色・適正ラベル）・圃場共有 URL**（フェーズ13 ＝ farm-benchmark-share。ガードレール④で研究用画面と分離）。本フェーズは研究用画面に特化。
- **VPD のアラート判定・しきい値編集との連動**（適正帯は監視補助の可視化であって alert_rules とは別）。**温湿度グラフ側の SMA/正常帯/乖離率の変更**（P2 所有・無回帰維持のみ）。
- **device-show 以外の画面**（dashboard カード・readings 履歴・他画面）への VPD 展開。最新計測テーブル・情報パネル・削除モーダルの変更。
- 認証・所有者認可・MethodOverride・CSRF・期間バリデーション本体（S1/S5/E1/P2 所有・消費のみ）。

## 技術制約・準拠事項

- **計算層と描画層の分離**（ガードレール⑧）。VPD/滞在率/逸脱の計算は純粋 Go（`[]float64`/スカラ入出力・gin/DB/templ/**time** 非依存）、描画は echarts.go の option 構築に集中。時刻が要る時間帯バケットは **handler 境界**で（stats 層に time を持ち込まない）。
- **依存方向**（structure.md）: 下向き一方向。`internal/chart` の最下流純粋性を維持（外部描画ライブラリ go-echarts への依存は E1/P2 の整合範囲）。View は repository/service を import しない。pgtype→float 変換は handler 境界に留める。
- **ECharts 表現**（§2-2/§2-3/§2-4）: 派生指標は**温湿度と別パネル**。VPD line が主役（◎）、**適正帯は固定しきい値帯 markArea**（線を増やさない）、滞在率は **gauge**（または数値カード＋簡易バー）。VPD移動平均は `legend.selected:false` で既定オフ（○）。線色変化 visualMap は△（任意）。
- **数式の確定値**: Tetens 式の `0.6108`・`17.27`・`237.3` は変更禁止（付録A D①）。適正帯 0.3/1.5 kPa は「多くの作物」の既定で**パラメータ化**して持つ。
- **go-echarts v2**: option は型安全に組める範囲で構築し、HTML 安全 JSON で供給（E1/P2 方式踏襲）。markArea/gauge も go-echarts の API（`opts.MarkAreas`/`charts.NewGauge` 等）で構築し、型で出せない属性はクライアント付与の既存パターンに合わせる。
- **HTMX**: 期間切替（hx-get/hx-target/hx-swap(innerHTML)/hx-push-url）は現状維持。swap 後も VPD chart の init/dispose・適正帯 markArea・滞在率が正しく再現されること。温湿度2グラフの connect は無回帰。
- **イミュータブル**: sqlc 生成構造体は読取専用。handler で View を組み立てる。
- **CSS**: 独自クラスの新設は最小化しモック単一ソース（VPD パネル器/カード/gauge 枠/逸脱表は先行反映→`make sync-css`）。グラフ内部描画は反映例外。VPD 用の新色を `:root` トークンへ。
- **マスタ＝Go 定数**（structure.md §98-100）: `domain.Crop` はテーブル/FK/seed CLI を作らず Go 定数＋`devices.crop VARCHAR`＋CHECK。`domain` は `fmt` のみ依存（純粋性）。CHECK の作物集合は `domain.Crop` 定数と手で同期（locality と同じ二重ミラー）。
- **マイグレーション**: VPD 本体は**なし**。作物マスタの `devices.crop` のみ goose **00009**・DDL のみ・後 `make db-snapshot`（CLAUDE.md）。
- **言語**: 日本語コメント・エラー・コミット。コード識別子は英語。
- **TDD**: 80% 以上。VPD 計算（es/ea/VPD・RH=0/100・氷点下・既知の手計算値との一致）・滞在率（在帯割合・空データ・全在帯/全逸脱境界）・時間帯別逸脱・markArea/gauge option 構築・handler 回帰（Show/Chart の期間・空データ・温湿度無回帰）・VPD カード/逸脱表の描画。

## 受け入れ基準（概略）

1. **VPD 計算正当性**: 任意の (温度, 湿度) で VPD が付録A D① の定義（Tetens es(T)・VPD=es(T)(1−RH/100)）どおり算出され、RH=0/100・氷点下・欠測が安全に扱われる（既知の手計算ケースと一致）。
2. **適正帯の塗り分け（◎）**: VPD 時系列に適正帯 markArea が **3 ゾーン**（乾きすぎ/適正/湿りすぎ）で重なり、しきい値が**パラメータ**として効いている（ハードコードでない）。
2b. **作物別しきい値**: `domain.Crop` が 9 作物（ゴーヤ/インゲン/サトウキビ/マンゴー/パイナップル/ウリ/米/いも/葉野菜）を持ち、各 `VPDRange()` が適正帯を返す。device に設定された作物の適正帯が VPD パネルに効き、**未選択（NULL）は既定 0.3/1.5 kPa にフォールバック**する。DeviceForm の作物 select で選択・保存・復元でき、存在しない作物は弾かれる。
3. **滞在率（⑤）**: 適正帯滞在率（日次%＝在帯割合）が gauge または数値カードで確認でき、瞬間値/平均でなく「**1日の何%が適正帯か**」を表す。
4. **時間帯別逸脱**: 時刻バケットごとの VPD 在帯/逸脱（乾き/湿り）が確認できる。
5. **VPD 移動平均（○）**: VPD の SMA が凡例トグルで既定オフから任意表示でき、`chart.SMA` を流用している。
6. **無回帰**: 温湿度2グラフ・統計オーバーレイ（SMA/正常帯/乖離率）・期間切替（24h/3d/7d/30d・アクティブ往復・URL 同期）・connect 連動・空データ表示が P2 同等に動作する。
7. **不変条件・規約**: **VPD 本体はスキーマ変更なし**（読み取り時計算）。スキーマ拡張は `devices.crop` 1列のみ（goose **00009**・DDL のみ・CHECK で 9 作物をミラー・FK 無し・後 `make db-snapshot`）。作物マスタはテーブル化せず `domain.Crop` Go 定数（structure.md §98-100）。
8. **モック整合**: VPD パネル器・VPD カード・滞在率 gauge 枠・逸脱表はモック（device-show.html＋style.css 正本）に反映。グラフ内部描画（VPD 線/markArea/gauge 値）は反映例外。
9. **研究用特化**: 農家向け平易表示（信号色・共有 URL）は含めない（フェーズ13）。
10. **テスト 80% 以上**。

## 未確定事項・要確認（設計フェーズで決定）

- **作物別 VPD 適正帯の具体値（最重要・要確認）**: 9 作物（ゴーヤ/インゲン/サトウキビ/マンゴー/パイナップル/ウリ/米/いも/葉野菜）それぞれの下限/上限 kPa を確定する。**ユーザー（沖縄の実地知見＝権威）の言明を最優先**し、無ければ文献値。施設果菜（ゴーヤ/インゲン/ウリ/マンゴー/葉野菜）は VPD 本命ゆえ昼間で 0.3〜1.5 kPa より狭めの適正帯が一般的だが**確定はユーザー確認**。露地・VPD 適用性が低い作物（サトウキビ/米/パイナップル/いも）は**適正帯を既定踏襲（0.3/1.5）か「VPD 参考」表示にとどめるか**を design で決める（VPD が効きにくい旨の注記）。spec 段階では構造（Crop→(lower,upper)・既定フォールバック）まで、具体値は research で埋める。
- **既定（未選択）適正帯**: 0.3〜1.5 kPa を本命（多くの作物・付録A D①）。下限/上限の保持形（`VPDRange()` の戻り値＝Go 定数）。
- **マンゴー表記**: ユーザー指定は「マンゴウ」だが標準表記「マンゴー」を Label に採用（識別子は mango）。要確認なら戻す。
- **滞在率の見せ方**: ECharts gauge（`EChartsInitializer` の一般化が要る）か、数値カード＋簡易バー（CSS・初期化不要）か。期間内全体の滞在率か当日（最新日）か、日別表も出すか。
- **時間帯別逸脱の見せ方**: 時間帯×日の heatmap（§2-3）か、時刻バケットの表/バーか。バケット幅（1時間 or 昼夜区分）。
- **VPD chart の connect 参加**: 温湿度グラフと時間軸 axisPointer を共有するか（y は kPa で別軸）。connect 群に入れるか独立させるか。
- **VPD の y 軸スケール**: 固定（例 0〜3 kPa）か auto か。適正帯 markArea が常に見える固定が読みやすい可能性。
- **期間別の出し分け**: 24h（カード/瞬間）と複数日（日次滞在率表）で VPD/滞在率の粒度を変えるか（P2 の ShowDaily=period!=24h と整合）。
- **氷点下・高温端の VPD**: Tetens 式は氷点下で氷面飽和を使わない近似。沖縄前提では実害小だが、温度域の注記を design に残す。

--- spec-init 本文 ここまで ---
