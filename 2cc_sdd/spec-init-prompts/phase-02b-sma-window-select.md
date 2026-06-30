# フェーズ2b（分析ロードマップ・P2 小改修）spec-init プロンプト: SMA 窓のユーザー選択（日スケール SMA を legend トグルの追加系列で／「数日〜2週間」の時間スケール空白を埋める）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: sma-window-select
> 位置づけ: [分析アイデアメモ.md](../分析アイデアメモ.md) 第1章「実装ロードマップ」**フェーズ2（temp-humidity-chart-stats）への小改修（P2b）**〔2026-06-30 追記・採用〕。新規画面ではなく、実装済み device-show（S5/E1）＋ P2 統計オーバーレイ（commit 3e17a73 マージ済）への**追加系列の上載せ拡張**。**UECS-GEAR の「3/7/15日 移動平均（＝窓選択式の単一MA）」との1対1比較（2026-06-30）**を受けた採用案＝「device-show の SMA 窓をユーザーが選べるようにする（例: 1日/3日/7日窓を legend トグルの追加系列で）」の spec 化。
> 確度: 〔採用・小改修〕（分析アイデアメモ フェーズ2 追記表／引継ぎメモ §17-2／メモリ `project_uecsgear_ma_candlestick_verdict`）。
> 前提セッション: **P2（temp-humidity-chart-stats）マージ済が必須前提**。S5（device-detail。期間切替フラグメント）／ E1（device-chart-echarts。go-echarts option JSON 構築・クライアント ECharts 描画・connect 連動）。
> **スキーマ変更なし**（P2 同様マイグレーション不要・goose 最新は 00010 のまま・`make db-snapshot` 不要）。日スケール SMA は既存 `sensor_readings` から**読み取り時に計算**する（ガードレール②/⑧）。
> **この改修が埋める空白（採用の根拠・最重要）**: go_iot の時間スケールは **device-show の点数窓 SMA（24h≒1時間…30d≒約1日）** と **seasonal-trend（P8）の月次/年次ロールアップ** に二極化しており、栽培現場で意思決定頻度が最も高い **「数日〜2週間」スケールの平滑系列がアーキ上どこにも存在しない**（潅水サイクル＝数日／インゲン冬作の追肥＝1〜2週間／台風・寒気南下からの回復追跡）。本改修は **日スケールの SMA（候補 1日/3日/7日）** をこの空白に正面から当てる（コード上の事実＝`smaWindowFor` 最大 288点≒1日 vs ロールアップ最小＝月。深掘り裁定の最強論点）。
> **やらないこと（誤用防止・最重要）**: **「3本を常時併置してゴールデンクロス/デッドクロスを眼で読む」金融チャート流の使い方はしない**（UECS-GEAR も実は窓選択式の単一MA）。追加系列は **legend `selected:false`（既定オフ）で「見たい人だけ重ねる」** 探索補助であって、**長期トレンドの有無判定の主役は P8（seasonal-trend）の Mann-Kendall ＋ Sen's slope のまま**（MA 交差の目視に置き換えない）。**ローソク足（OHLC）は不採用**（日周性で陽陰線が無意味化・引継ぎメモ §17-2／分析アイデアメモ フェーズ2 追記）。
> 設計フェーズで参照:
> - 上位ロードマップ: 分析アイデアメモ.md **フェーズ2 追記（2026-06-30）**（採用＝SMA 窓のユーザー選択／不採用＝ローソク足・隠れたニーズは レンジバー・日較差ΔT推移線・P12 夜温で代替）／ §2-2 クラッタ対策（主役1〜2系列・legend `selected:false`・線より「帯」で見せる）／ §2-3 表現テク早見表（既定で畳む＝`legend.selected:false`）／ 第3章ガードレール⑧（軽い統計はアプリ内計算）。
> - 数式（権威）: 分析アイデアメモ.md 付録A「B. 時系列・移動平均系」（`SMA_t=(1/N)Σx`）。**EMA/WMA は採用しない**（フェーズ2 と同方針）。
> - 現スキーマ（権威）: docs/database_snapshot/table_definitions.md「sensor_readings」（temperature/humidity numeric(5,2)・recorded_at timestamptz・(device_id, recorded_at DESC) 部分索引）。**本改修は DDL なし**。
> - 流用元・改修対象コード（P2 マージ後の現状・実コード確認済み）:
>   - `internal/chart/stats.go` … **`SMA(values []float64, window int) []float64`**（窓は既に**引数化済み**・先頭は expanding window の部分平均で欠落なし）。→ **本改修は新ロジック不要＝同関数を複数の窓で呼ぶだけ**。`MovingStdDev`/`Band`/`Deviation`/`Mean`/`MinMax`/`DiurnalRange`/`CV` も既存。
>   - `internal/chart/echarts.go` … **`ChartOptionJSON(spec ChartSpec)`**（複数系列の唯一の公開 option 関数。生実測線 series[0] ＋ SMA1本＋正常帯＋乖離率を組み、`legend.selected:false` で ○ 系列を既定オフ化する確立パターン）。→ **追加 SMA 系列を legend トグルで足す中心ファイル**。
>   - `internal/chart/series.go` … **`ChartSpec`（現状10フィールド＝P2 の 8〔Labels/Unit/Color/Raw/SMA/BandLower/BandWidth/Deviation〕＋P5 data-quality-meta が非破壊追加した `RawNullable`/`GapBands`〔欠測ギャップ〕）**。→ 追加 SMA 系列を表現できるよう拡張するか、SMA 系列を配列化するかは design 論点。**拡張時は P2 の正常帯/乖離率に加え P5 の欠測ギャップ（`RawNullable`/`GapBands`）の無回帰も守る対象**。
>   - `internal/handler/device_show.go` … **`smaWindowFor(period)`**（period 別に **点数**で SMA 窓を返す＝24h=12点/3d=36点/7d=72点/30d=288点・「約5分間隔=12点/時」前提）／ **`overlaySpec`**（単一 window で `chart.SMA` を呼び `ChartSpec` に詰める）／ `buildChartArea`／ `Show`・`Chart`（期間切替フラグメント）。→ **日スケール窓（候補 1日/3日/7日）の点数換算と追加系列の組立を足す中心 handler**。
>   - `internal/view/component/DeviceChartArea.templ` … 期間ボタン群（`hx-get`/`hx-target`/`hx-swap=innerHTML`/`hx-push-url`）＋ `#temperature-chart`/`#humidity-chart`（`data-echarts`）＋兄弟 option script。窓セレクタ UI を足すならここ（静的な器＝モック反映対象）。
>   - `internal/view/layout/App.templ` の `EChartsInitializer` … `[data-echarts]` 走査→ option JSON parse→ `dispose`→`init`→ endLabel/sampling 付与→ `setOption`・`echarts.connect()`。**endLabel は生実測線のみ**（追加 SMA には付けない）の現方針を維持。
> - モック（単一ソース運用の境界）: `mocks/html/device-show.html`・`mocks/html/style.css`（正本）。**窓セレクタ UI・凡例ラベル等の静的な器はモック反映必須**（feedback_mock_reflects_impl_visual）／**グラフ内部の SMA 線描画は反映対象外＝グラフ描画例外**（feedback_mock_graph_rendering_exception）。`make sync-css`。
> - 命名・依存規約: .kiro/steering/structure.md（依存方向＝下向き一方向・`internal/chart` 最下流純粋層〔`[]float64` 入出力・time 非依存〕）／ tech.md（sqlc）／ CLAUDE.md。

--- spec-init 本文 ここから ---

## 機能概要

device-show（デバイス詳細）の温度・湿度グラフ（E1 で Apache ECharts へ移行済み・P2 で生実測線＋SMA1本＋正常帯＋乖離率を実装済み）に、**日スケールの単純移動平均（SMA）を「ユーザーが選べる追加系列」として上載せ**する。現状の SMA は period（24h/3d/7d/30d）に従属した**点数固定窓（最長でも約1日相当）**しか持たず、長期分析（P8 seasonal-trend）は月次/年次粒度のため、**栽培現場で意思決定頻度が最も高い「数日〜2週間」スケールの平滑系列がアーキ上どこにも存在しない**。本改修は **日スケールの SMA（候補 1日/3日/7日）** をこの空白に当て、**凡例（legend）トグルの追加系列として既定オフ（`selected:false`）で「見たい人だけ重ねる」** 形で提供する。新ロジックは不要で、**窓を引数に取る既存 `chart.SMA` を複数の窓で呼ぶだけ**＝スキーマ変更・新規クエリ・新規計算関数なしの小改修。生実測線が常に主役（◎）であること、クラッタを増やさないこと、**EMA/WMA・3本併置の金融的読み（ゴールデンクロス）を持ち込まないこと**を厳守する。これは **UECS-GEAR の「3/7/15日 移動平均（窓選択式の単一MA）」への go_iot 側の節度ある応答**（2026-06-30 の1対1比較裁定）である。

> **「トレンドの有無判定の主役は P8 のまま」（最重要・誤用防止）**: 日スケール SMA はあくまで **device-show 高解像度と P8 月次ロールアップの間を埋める中期の探索的可視化**であって、**長期トレンドの有意判定を担うものではない**。トレンドの客観判定は P8（seasonal-trend）の自己相関補正つき Mann-Kendall ＋ Sen's slope が引き続き主役で、本改修は MA 交差の目視でそれを代替しない。窓を増やす目的は「時定数の異なる平滑の対比で直近の急変が短期ノイズか持続変化かを見分ける」ことであり、金融の売買シグナル読みではない。

## 背景・現状

P2（temp-humidity-chart-stats・commit 3e17a73）で SMA1本基盤を導入後、**現状は P3〜P7 までマージ済**（`ChartSpec` には P5 で `RawNullable`/`GapBands`＝欠測ギャップが非破壊追加され現状10フィールド）。本改修の**論理前提は P2**（SMA1本基盤）だが、現状コードは以下（実コード確認済み）。

- **SMA は窓が引数化済み・ただし窓値は period 従属の点数固定**: `internal/chart/stats.go` の `SMA(values []float64, window int) []float64` は窓を引数に取り、先頭は expanding window の部分平均で欠落を作らない純関数（time 非依存）。一方 `internal/handler/device_show.go` の `smaWindowFor(period)` が period 別に**点数**で窓を返し（24h=12点/3d=36点/7d=72点/30d=288点＝「約5分間隔=12点/時」前提）、`overlaySpec` が**単一 window で 1 本だけ** `chart.SMA` を呼んで `ChartSpec` に詰める。**最長でも 30d ビューの 288点≒約1日窓**にとどまる。
- **複数窓の同時表示機能は無い**: `internal/chart/echarts.go` の `ChartOptionJSON(spec ChartSpec)` は生実測線＋SMA1本＋正常帯＋乖離率を組み、○ 系列を `legend.selected:false` で既定オフにする確立パターンを持つが、SMA は 1 系列前提。
- **長期側は月次/年次（P8）**: seasonal-trend の月次ロールアップが最小粒度＝月。**「数日〜2週間」は device-show（≦約1日）と P8（≧月）の間に挟まれて空白**。
- **クラッタ規律は確立済み**: 主役は生実測線、SMA/正常帯/乖離率は legend 既定オフ（§2-2/§2-3）。本改修はこの規律の上に追加系列を載せる。

## このセッションのスコープ（実装対象）

### 1. 日スケール SMA 窓の追加（計算は既存流用・スキーマ非変更）

- **候補 1日/3日/7日**（design/research で窓値を確定。値の根拠は下記 未確定事項①）の SMA を **既存 `chart.SMA(values, window)` を窓を変えて複数回呼ぶ**だけで生成する（新規計算関数なし）。
- 窓は **「日数 → 点数」換算**で求める（現状 `smaWindowFor` の点数固定を、サンプリング間隔に基づく時間窓→点数換算へ一般化）。サンプリングは約5分（12点/時）想定だが、**点数の決め打ちでなく「日数 × 推定点数/日」で算出**し、間隔変更にも素直に追従する形が望ましい（横断＝分析アイデアメモ第4章「サンプリング間隔 5分維持」決定と整合）。
- すべて読み取り時計算・**スキーマ変更/マイグレーションなし**（P2 同方針・goose 00010 のまま）。

### 2. 追加系列の描画と legend トグル（echarts.go / series.go）

- 日スケール SMA 各窓を **凡例トグルの追加系列**として `ChartOptionJSON` に載せる。**既定は `legend.selected:false`（オフ）** で、生実測線（◎）が常に主役。
- 追加系列は **細線・`showSymbol:false`・endLabel なし**（endLabel は生実測線のみの現方針を維持）。窓ごとに視認できる凡例ラベル（例「移動平均 3日」）と色/線種を与える。
- `ChartSpec`（series.go・現状10フィールド）を **複数 SMA 系列を表現できる形へ拡張**するか、SMA 系列をスライス化するかは design 論点（**P2/P3 系列に加え P5 の欠測ギャップ `RawNullable`/`GapBands` の無回帰も守る**）。

### 3. 窓選択 UX（device-show・任意）

- 「どの日スケール窓を出すか」を **(a) 全部を legend トグルとして並べて出す（採用案の素直な実装）** か **(b) 窓セレクタ UI（自動/1日/3日/7日 等）で 1 本を選ばせる** かは design 論点（未確定事項②）。**(a) を既定の本命**とし、クラッタが問題なら (b) を検討。
- UI を足す場合は `DeviceChartArea.templ` に静的な器として追加（モック反映対象）。期間切替（24h/3d/7d/30d）・URL 同期・connect 連動は**無回帰維持**。

### 4. 長窓のデータ供給（最重要の設計論点）

- 7日窓の SMA を 7d/24h/3d ビューで「左端から正しく」描くには、**可視期間の開始点より前のルックバック分のデータが要る**（24h ビューに 7日窓 SMA を載せるなら 7日分の遡及取得が必要）。**どのビューにどの窓を出すか**（例: 日スケール窓は 7d/30d ビューに限定）、**ルックバック取得をどうするか**（`ListRecentSensorReadings` の取得起点を窓ぶん手前へ広げる SELECT のみの拡張）を design で決める（未確定事項③）。**DDL は足さない**（取得起点を広げるのは SELECT パラメータの調整）。

### 5. モック反映

- **窓セレクタ UI・凡例ラベル等の静的な器**を `mocks/html/device-show.html`＋`mocks/html/style.css`（正本）へ反映し `make sync-css`（feedback_mock_reflects_impl_visual）。**グラフ内部の SMA 線描画はモック反映の例外**（feedback_mock_graph_rendering_exception）。

## スコープ外（このセッションでやらないこと）

- **3本以上の SMA を常時併置してゴールデンクロス/デッドクロスを眼で読む金融的運用**（追加系列は既定オフのトグル＝探索補助に限る）。**EMA/WMA**（フェーズ2 と同じく不採用）。
- **長期トレンドの有意判定**＝**P8（seasonal-trend）の Mann-Kendall ＋ Sen's slope が主役**（本改修は MA 交差の目視でトレンド判定を代替しない）。MK/Sen・自己相関補正・検定はここでは扱わない。
- **ローソク足（OHLC）**＝不採用（日周性で陽陰線が無意味化）。日次レンジの俯瞰が要るなら**レンジバー or 既存 日較差ΔT 推移**、夜温の高止まり/朝の立ち上がりは**フェーズ12（THI・熱帯夜・夜温）**で扱う（引継ぎメモ §17-2／分析アイデアメモ フェーズ2・12 追記）。
- **サンプリング間隔の 1分化**＝不採用（5分維持。分析アイデアメモ 第4章 2026-06-30 決定）。本改修は点数を「日数×推定点数/日」で算出して間隔非依存にするだけで、間隔自体は変えない。
- **派生指標列の DB 追加・マイグレーション**（読み取り時計算で足りる＝ガードレール②・goose 00010 のまま）。`sensor_readings` スキーマ・受信 API の変更。
- **VPD・露点・GDD・THI 等の派生指標**（P3/P6/P7/P12）。正常帯・乖離率の仕様変更（P2 所有・消費のみ）。
- **device-show 以外の画面**（dashboard カード・readings 履歴・統計分析ページ P8）への展開。
- 認証・所有者認可・MethodOverride・CSRF・期間バリデーション本体（S1/S5/E1 所有・消費のみ）。

## 技術制約・準拠事項

- **計算は既存流用・新ロジック最小**: 日スケール SMA は `chart.SMA(values, window)` を窓を変えて呼ぶだけ（純粋 Go・`[]float64` 入出力・time 非依存）。点数換算（日数→点数）と series 組立は handler 境界。
- **依存方向**（structure.md）: 下向き一方向。`internal/chart` 最下流純粋性を維持。pgtype→float 変換は handler 境界。View は repository/service を import しない。
- **クラッタ規律**（§2-2/§2-3）: 1チャート＝主役1〜2系列。追加 SMA は **`legend.selected:false`（既定オフ）**。生実測線が常に主役。窓を増やすほどクラッタが増えるため、既定オフ徹底と凡例の可読性を担保。
- **endLabel/sampling**: 生実測線のみに endLabel を付ける現方針を維持（追加 SMA には付けない）。複数系列でも `echarts.connect()` 連動が壊れないこと。
- **無回帰**: 期間切替（24h/3d/7d/30d・アクティブ往復・URL 同期）・温湿度2グラフ連動・空データ表示・P2 の SMA1本/正常帯/乖離率/カード/日次集計表・**P5 の欠測ギャップ（`RawNullable`/`GapBands`）** が従来どおり動作する。
- **スキーマ非変更**: DDL なし（goose 最新 00010 のまま・`make db-snapshot` 不要）。ルックバック取得の拡張は SELECT パラメータの調整に留める。
- **言語**: 日本語コメント・エラー・コミット・UI ラベル（「移動平均 3日」等）。コード識別子は英語。
- **TDD**: 80% 以上。日数→点数換算（間隔想定・空データ・短期間で窓が期間を超える場合）・複数 SMA 系列の option 構築（系列数・legend `selected:false`・凡例名・色/線種・endLabel 非付与）・handler 回帰（Show/Chart の期間切替・無回帰）・ルックバック取得（左端 warm-up）。

## 受け入れ基準（概略）

1. **日スケール SMA の追加**: device-show のグラフに日スケール SMA（design 確定の窓・候補 1日/3日/7日）が**凡例トグルの追加系列**として載り、**既定はオフ（`selected:false`）**で生実測線が主役のまま。凡例から任意に表示できる。
2. **空白を埋める**: 7d/30d ビュー等で「数日〜2週間」スケールの平滑が読め、点数窓 SMA（≒1日）と P8 月次ロールアップの間の空白が埋まる。
3. **計算流用・スキーマ非変更**: 追加 SMA は既存 `chart.SMA` の窓違い呼び出しで実装され、**新規計算関数・DDL・マイグレーションなし**（goose 00010 のまま）。
4. **誤用防止の不変条件**: EMA/WMA を含まず SMA のみ。3本併置の金融的読み（ゴールデンクロス）を前提とした UI/文言を持たない。**長期トレンドの有意判定は P8 のままで本改修は代替しない**。
5. **長窓の左端正しさ**: 日スケール窓を出すビューで、SMA が左端から（ルックバック warm-up により）正しく描かれる、または「このビューでは対象外」と設計どおりに振る舞う。
6. **無回帰**: 期間切替・URL 同期・温湿度2グラフ連動・P2 の既存オーバーレイ（SMA1本/正常帯/乖離率/カード/日次集計表）が従来どおり。
7. **モック整合**: 窓セレクタ UI・凡例ラベル等の静的な器がモック（device-show.html＋style.css 正本）に反映（グラフ内部の SMA 線描画は反映例外）。
8. **テスト 80% 以上**。

## 未確定事項・要確認（設計フェーズで決定）

- **① 窓値の確定（最重要）**: 採用案の候補は 1日/3日/7日 だが、**UECS-GEAR の 3/7/15日 には農学的根拠の一次記載が無い**（2026-06-30 調査）。**沖縄の主要作物の意思決定スケール**（ゴーヤ施設の潅水サイクル＝数日／インゲン冬作の追肥＝1〜2週間／台風・寒気回復の追跡）に照らし、研究フェーズでユーザーの実地知見（権威・付録B-4）／文献から窓値を確定する（1日/3日/7日 か 3日/7日/14日 か等）。窓は 3 本までに抑えクラッタを避ける。
- **② UX＝全窓トグル併置 vs 窓セレクタ**: (a) 全候補窓を legend トグルとして並べる（採用案の素直な実装・本命）か、(b) 窓セレクタ（自動/1日/3日/7日）で 1 本を選ばせるか。クラッタ・操作性・モック反映量で判断。
- **③ 長窓のルックバック取得**: 日スケール窓を出すビューの範囲（7d/30d に限定するか全ビューか）と、左端 warm-up のためのルックバック取得方法（`ListRecentSensorReadings` の取得起点を窓ぶん手前へ広げる SELECT 調整・DDL なし）。短期ビュー（24h/3d）で長窓 SMA をどう扱うか（出さない/警告/部分平均）。
- **④ 既存 period 従属 SMA との関係**: 現状の `smaWindowFor`（point 窓・約1日）を残して日スケール窓を**追加**するか、period 従属 SMA を日スケール窓選択に**置き換える**か（P2 の SMA1本の無回帰を壊さない範囲で）。
- **⑤ 点数換算の方式**: 「日数 × 推定点数/日」をサンプリング間隔の実測中央値から動的に求めるか、約5分（12点/時＝288点/日）固定換算にするか（分析アイデアメモ第4章「5分維持」決定と整合・将来の間隔変更耐性）。
- **⑥ series.go の拡張形**: `ChartSpec` に複数 SMA 系列を表現する形（スライス化 or per-series 属性）と、P2/P3 既存系列の無回帰の担保。

--- spec-init 本文 ここまで ---
