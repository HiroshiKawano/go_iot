# Gap Analysis — sma-window-select（日スケール SMA 窓のユーザー選択）

> 目的: 既存 device-show（S5/E1）＋ P2 統計オーバーレイ＋ P5 欠測ギャップが実装済みのコードベースに対し、要件（R1〜R8）の実装ギャップと統合戦略を分析する。実装決定ではなく選択肢の提示。

## 1. 現状調査（Current State）

### 1.1 関係する既存資産（実コード確認済み）

| 層 | ファイル | 現状の責務 | 本改修との関係 |
|---|---|---|---|
| 純粋計算層 | `internal/chart/stats.go` | `SMA(values, window)`（窓引数化済み・先頭 expanding window で欠落なし・time 非依存）。`MovingStdDev`/`Band`/`Deviation`/`Mean`/`MinMax`/`DiurnalRange`/`StdDev`/`CV`/`LinearFit` も既存 | **無改修で流用可**。日スケール SMA は同関数を窓違いで複数回呼ぶだけ（R1.4/R7.1） |
| 入力契約 | `internal/chart/series.go` | `ChartSpec`（10フィールド）。`SMA []float64` は**単数**。`RawNullable`/`GapBands` は P5 が末尾非破壊追加（nil で従来挙動＝後方互換の前例） | **拡張が必要**（複数 SMA 系列の表現）。Constraint |
| option ビルダー | `internal/chart/echarts.go` | `ChartOptionJSON(spec)` が `series[0]=生実測 + SMA1本 + 正常帯 + 乖離率`。`legend.selected:false` で SMA/正常帯/乖離率を既定オフ。SMA は `seriesNameSMA="移動平均"` 固定の1系列 | **拡張が必要**（複数 SMA 系列を legendData/selected/AddSeries に追加）。Constraint |
| handler | `internal/handler/device_show.go` | `smaWindowFor(period)`=period別**点数**固定（12/36/72/288）。`overlaySpec(labels,values,unit,color,window)`=**単一窓**で SMA/σ/帯/乖離率を詰める。`buildChartArea`→`ListRecentSensorReadings`→ pgconv→ overlaySpec→（欠測時 `applyGapGrid`）→ `ChartOptionJSON`。`Show`/`Chart`（期間切替フラグメント）は `buildChartArea` 共有 | **中心改修箇所**。日スケール窓の点数換算＋複数 SMA 算出＋（必要なら）ルックバック取得・表示トリミング |
| 欠測グリッド | `device_show.go::applyGapGrid` | 単一 `spec.SMA` を欠測スロットで carry-forward 整列 | **複数 SMA の carry-forward へ拡張が必要**（P5 無回帰の要） |
| クエリ | `db/queries/sensor_readings.sql` | `ListRecentSensorReadings`: `WHERE device_id=$1 AND recorded_at>=$2 AND deleted_at IS NULL ORDER BY recorded_at ASC`（取得開始時刻パラメータあり） | **ルックバック＝開始時刻を窓ぶん手前へ広げる SELECT 調整のみ（DDL 不要・R7）** |
| 静的 UI | `internal/view/component/DeviceChartArea.templ` ＋ `mocks/html/device-show.html`/`style.css` | 期間ボタン群（hx-get/hx-target/hx-swap/hx-push-url）＋ `#temperature-chart`/`#humidity-chart`＋兄弟 option script | 窓セレクタ UI を足すなら期間ボタン群の直下（モック反映対象・R8） |
| client 初期化 | `internal/view/layout/App.templ::EChartsInitializer` | `data-echarts` 走査→ option parse→ init→ `setOption`→ series[0] に endLabel/sampling 付与→ `echarts.connect()` 連動 | **無改修**（endLabel は series[0]=生実測線のみ・追加 SMA には付かない＝R2.3 が自動的に成立） |

### 1.2 規約・パターン（structure.md / 既存実装から抽出）

- **依存方向**: 下向き一方向。`internal/chart` は最下流純粋層（`[]float64` 入出力・time 非依存）。pgtype→float 変換と点数換算は handler 境界。**day-scale SMA も計算は chart 層・点数換算は handler に置く**のが規約整合。
- **後方互換の前例**: `RawNullable`/`GapBands` は「nil/空なら完全に従来挙動」で `ChartSpec` 末尾へ非破壊追加された（series.go コメントが不変条件として明記）。**追加 SMA 系列も同方式（nil/空で従来挙動）で末尾追加**するのが既存パターン。
- **別型隔離 vs 同チャート上載せ**: VPD/露点/GDD/Trend は「別チャート・別 option 型」で隔離されている。だが**日スケール SMA は温湿度と同一チャート上に重ねる**ため別型隔離は不適。`ChartOptionJSON`/`ChartSpec` の拡張が正道。
- **legend.selected:false の確立パターン**: オーバーレイ系列は `legendData` への追加＋`selected[name]=false` で既定オフ。追加 SMA はこのパターンの素直な拡張で R2.1 を満たす。
- **テスト配置**: chart 層は純関数テスト、handler は Querier 手書きモック＋ templ Render→strings.Contains（テストガイダンス集の定石）。

## 2. 要件→資産マップ（ギャップ tag: Missing / Unknown / Constraint）

| 要件 | 必要な技術要素 | 既存資産 | ギャップ |
|---|---|---|---|
| R1 日スケール SMA 表示 | 日スケール窓集合、複数 SMA 算出、複数系列の option 化 | `chart.SMA`(流用)、`overlaySpec`/`smaWindowFor`(単数)、`ChartSpec`/`ChartOptionJSON`(単数) | **Missing**: 複数 SMA 系列の表現。**Unknown**: 窓値の確定（①） |
| R2 既定オフ・クラッタ規律（細線/symbolなし/endLabelなし/最大3本） | legend.selected 拡張、細線 symbol off | `ChartOptionJSON` の legend/AddSeries パターン、client endLabel=series[0]のみ | **Constraint**: legendData/selected を N 系列へ拡張。endLabel 非付与は client 既存挙動で自動成立 |
| R3 中期スケールの空白充足 | 日スケール窓（数日〜2週間）の供給 | 現状最長 288点≒1日 | **Unknown**: どのビューに出すか（③）。窓値（①） |
| R4 誤用防止（SMAのみ/非金融/OHLCなし/有意判定はP8） | SMA限定の維持 | `chart.SMA` のみ・EMA/WMA/OHLC 不在・P8別spec | **Constraint（維持）**: 既存が既に充足。UI 文言にシグナル語を入れないことの担保 |
| R5 長窓の左端正しさ・短期ビュー扱い | ルックバック取得＋表示トリミング、または縮退挙動 | `ListRecentSensorReadings`(開始時刻パラメータ)、`buildChartArea`(全行を表示に使用) | **Unknown（最大リスク）**: ルックバック fetch＋表示窓トリミングの設計（③）。gap grid/日次表/VPD/露点が `rows`/`labels` を共有する点との整合 |
| R6 無回帰（期間切替/連動/空データ/性能） | 既存経路の不変 | `Show`/`Chart`/`buildChartArea`、client connect、`applyGapGrid` | **Constraint**: `applyGapGrid` の複数 SMA carry-forward。広域 fetch による性能影響 |
| R7 計算/保存/受信の境界 | 読み取り時計算・DDLなし | 全統計が読み取り時・goose 00010 のまま | **充足**: クエリ変更は SELECT パラメータのみ（保存列追加なし・受信 API 不変） |
| R8 追加 UI のモック整合 | 窓セレクタ/凡例ラベルのモック反映 | `DeviceChartArea.templ`＋モック正本 | **Missing（UX 採用時）**: 窓セレクタ UI（採用案 (a) なら UI 追加なし＝凡例トグルのみ） |

## 3. 実装アプローチ選択肢

### Option A: 既存コンポーネントを全面拡張（ChartSpec/ChartOptionJSON を複数 SMA 対応へ）
- `ChartSpec.SMA []float64`（単数）を**複数系列スライス**へ作り替え、`ChartOptionJSON` をループ化。`overlaySpec`/`smaWindowFor` を複数窓返却へ変更。
- ✅ 系列構築が一箇所に集約。✅ 既存パターンの延長。
- ❌ `SMA` フィールドの型変更は**P2 の単一 SMA・P5 の `applyGapGrid` を破壊的に触る**（無回帰リスク増）。❌ `ChartOptionJSON` の肥大。

### Option B: 別型・別ビルダーで隔離（VPD/露点/GDD/Trend の踏襲）
- 日スケール SMA 専用の Spec/option 関数を新設。
- ✅ 既存 `ChartOptionJSON` 無改修。
- ❌ **不適**: 追加 SMA は温湿度と**同一チャート上**に重ねる必要があり、別 option では同一 ECharts インスタンスに同居できない（別型隔離は別チャートの場合のパターン）。採用しない。

### Option C: 加算的拡張（Hybrid・推奨）— RawNullable/GapBands 前例に倣う
- `ChartSpec` に **追加 SMA 系列のスライス**（例 `DaySMAs []NamedSeries{Name,Values,Color,Dashed}`）を**末尾へ非破壊追加**（nil/空で完全に従来挙動）。既存 `SMA []float64`（P2 単数・点数窓）は**温存**して無回帰を守る（未確定④は「残す」を既定）。
- `ChartOptionJSON` は既存 SMA/帯/乖離率の描画後に `DaySMAs` をループで `AddSeries`＋legendData/selected へ追加（細線・symbol off・endLabel 非対象）。
- `applyGapGrid` は `DaySMAs` 各系列も既存 SMA と同様 carry-forward（P5 無回帰）。
- handler は日スケール窓を「日数×推定点数/日」で点数換算する**新ヘルパ**を足し、`chart.SMA` を窓ごとに呼ぶ。
- **UX は採用案 (a)（全窓を凡例トグルで併置）を先行**＝ handler パラメータ・窓セレクタ UI なしで成立（templ 改修は最小）。窓セレクタ (b) は将来段階。
- **ルックバックは段階導入**: まず「日スケール窓は十分な期間のビュー（7d/30d）に限定し、左端は SMA の expanding-window 部分平均を許容（縮退挙動・R5.2）」で MVP、必要なら fetch 拡張＋表示トリミングを次段で追加。
- ✅ P2/P5 を破壊せず（型変更なし・加算のみ）。✅ 段階導入でリスク分散。✅ 既存後方互換前例と一致。
- ❌ `SMA` 単数と `DaySMAs` 複数が併存し概念がやや重複（④の整理が要る）。

## 4. 工数・リスク

- **総合工数: M（3〜7日）**。内訳: chart 層の加算拡張＋option ループ＋handler の点数換算＋複数 SMA 算出＋テストは S〜M。ルックバック warm-up＋表示トリミング（R5 を「正しい左端」で満たす場合）が M〜L の主因。
- **総合リスク: Medium**。
  - chart 層の加算拡張は後方互換前例（RawNullable/GapBands）があり **Low**。
  - `applyGapGrid` の複数 SMA carry-forward は P5 無回帰に直結 **Medium**。
  - ルックバック fetch が `rows`/`labels` を共有する gap 検出・日次表・VPD・露点パネルに波及しうる点が **Medium〜High**（「SMA 計算用に広く取り、表示は窓へトリム」を厳密に分離できるかが鍵）。段階導入（縮退挙動 MVP）で回避可能。
  - 窓値確定はユーザー入力待ちの **Research（技術リスク Low）**。

## 5. Research Needed（design フェーズへ持ち越す論点）

1. **① 窓値の確定（最重要）**: 1日/3日/7日 か 3日/7日/14日 か等。UECS-GEAR の 3/7/15日 に農学的一次根拠なし（2026-06-30 調査）。**沖縄の作物意思決定スケール**（ゴーヤ施設の潅水＝数日／インゲン冬作の追肥＝1〜2週間／台風・寒気回復）に照らしユーザーの実地知見（権威・付録B-4）で確定。**最大3本**でクラッタ抑制（R2.4）。
2. **② UX 方式**: (a) 全窓を凡例トグル併置（本命・UI/handler パラメータ追加なし）vs (b) 窓セレクタで1本選択。いずれも既定オフ・生実測線主役を満たすこと。(a) 先行を推奨。
3. **③ ルックバック設計（R5 の中核）**: 日スケール窓を出すビュー範囲（7d/30d 限定 vs 全ビュー）。左端 warm-up の fetch 拡張（`recorded_at >= periodSince(period) - maxWindow`）＋**表示窓トリミング**（生実測線・ラベル・日次表・VPD/露点は可視窓のまま、SMA だけ広域計算）。短期ビュー（24h/3d）で長窓をどう扱うか（出さない/部分平均で縮退/警告）。
4. **④ 既存 period 従属 SMA との関係**: 現状 `smaWindowFor`（点数窓・約1日まで）を**残して日スケール窓を追加**（無回帰最優先・推奨）か、日スケール窓選択へ**統合**するか。
5. **⑤ 点数換算方式**: 「日数×推定点数/日」を実測間隔の中央値から動的算出 vs 約5分（288点/日）固定換算。分析アイデアメモ第4章「5分維持」決定・将来の間隔変更耐性と整合（`intervalSeconds`/`MissingStats` が既に間隔中央値を算出する点を再利用可）。
6. **⑥ ChartSpec 拡張形**: 追加 SMA 系列スライス（`[]NamedSeries`）vs per-series 属性。`applyGapGrid` の複数 SMA carry-forward と P2/P3/P5 既存系列の無回帰担保。
7. **⑦ 性能**: 7日窓×lookback で点数が増大（5分間隔なら 7日=2016点、30d ビュー＋7日 lookback で 1万点超）。option JSON サイズ・client 描画コスト（`sampling:"lttb"` は client が series[0] のみ付与）。R6.5（応答が体感同等）の担保方法。

## 6. design フェーズへの推奨

- **推奨アプローチ: Option C（加算的拡張）**。`ChartSpec` への追加 SMA 系列の**末尾非破壊追加**で P2/P5 を守り、`ChartOptionJSON` をループ拡張、handler に日スケール点数換算ヘルパを足す。UX は (a) 凡例トグル併置を先行。
- **段階化**: ①窓値確定 →②加算拡張で複数 SMA を凡例トグル表示（縮退挙動 MVP・左端は部分平均許容）→③必要なら lookback fetch＋表示トリミングで左端を厳密化。
- **無回帰の最重要監視点**: `applyGapGrid` の複数 SMA carry-forward（P5）、`SMA`(単数)/帯/乖離率の温存（P2）、period 切替フラグメント・connect 連動（S5/E1）、空データ表示。
- **キー決定**: 窓値（①・ユーザー確認）、ルックバック方式（③）、ChartSpec 拡張形（⑥）。

---

## Synthesis 所見（design フェーズ・2026-06-30）

light discovery（Extension）で確定した設計判断。詳細は `design.md` を正とする。

### 1. Generalization（一般化）
- R1.1〜R3.2 は「単一 SMA 系列」を「N 本の日スケール SMA 系列」へ一般化する同一問題。**インターフェースだけ一般化**（`ChartSpec.DaySMAs []DaySMASeries` スライス）し、実装は要件が要求する最大3本（R2.4）に留める。P2 の既存単数 `SMA` は**リファクタせず温存**（無回帰優先）。

### 2. Build vs. Adopt（全て Adopt・新規 Build ゼロ）
- 計算 = `chart.SMA(values, window)`（実在・窓引数化済み）。表示丸め = `chartRound2`（seasonal_trend_handler.go:400 実在）。点数換算の間隔 = `intervalSeconds`（readings_quality.go:174 実在）の中央値。ルックバック = `ListRecentSensorReadings` の取得起点 `$2` を手前へ（**クエリ/DDL 変更なし**）。新規ライブラリ・新規アルゴリズムなし。

### 3. Simplification（簡素化）
- **窓セレクタ UI を作らない**（採用案 (a)＝全窓を凡例トグルで併置）→ 新規 templ/モック変更ゼロ・新規 HTMX 操作ゼロ（凡例トグルはクライアント ECharts）。
- **日スケール窓は 7d/30d のみ**（24h/3d は `dayScaleWindowsFor`→nil で既存パス完全分岐）→ 最強の無回帰保証・短期ビューでの巨大ルックバック回避。
- **可視窓スライスを既存パスへ渡す**→ 生実測線・overlaySpec・gap・日次表・VPD/露点は今日と同一 `rows`（バイト等価）。ルックバックは日スケール SMA warm-up 専用。
- 窓ごとの別型を作らず 1 スライスで表現（per-window 型の乱立を回避）。

### 確定した設計決定（未確定事項①〜⑦の回答）
- **① 窓値 = 3日 / 7日 / 14日**。「数日〜2週間」を直接張る。1日窓は 30d ビューの既存 P2 SMA（288点≒1日）が実質カバーするため除外（重複回避・クラッタ抑制）。ユーザー希望で `dayScaleWindowsFor` 定数差し替え可（承認時確認）。
- **② UX = (a) 全窓を凡例トグル併置**（窓セレクタ不採用）。
- **③ ルックバック = 取得起点を `periodSince - maxWindowDays` へ広げ、全系列で SMA 計算後に可視窓へスライス**。日スケール窓は 7d/30d のみ。短期ビュー（24h/3d）は窓を出さない（5.2 縮退）。データ不足は expanding window 部分平均（線欠落なし）。
- **④ 既存 period 従属 SMA（`smaWindowFor`）は残して追加**（統合せず・無回帰優先）。
- **⑤ 点数換算 = 動的中央値**（`intervalSeconds` の median→86400/median・不能時 288 フォールバック）。間隔変更耐性。
- **⑥ ChartSpec 拡張 = `DaySMAs []DaySMASeries` 末尾非破壊追加**（P5 の `RawNullable`/`GapBands` 前例に倣う）。`applyGapGrid` は各 DaySMAs も carry-forward。
- **⑦ 性能 = `chartRound2` で JSON 抑制＋既定オフ＋7d/30d 限定**。30d×14日窓は実機スモークで R6.5 を確認。間引きは将来課題（本 spec では未実施）。
