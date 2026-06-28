# ギャップ分析: data-quality-meta（フェーズ5 データ品質メタ層）

> `/kiro-validate-gap` による要件 ↔ 既存コードのギャップ分析。設計フェーズの判断材料。
> 対象要件: requirements.md（R1〜R10）。本フェーズは新規画面ではなく **device-show / readings への上載せ**。
> 調査日: 2026-06-28（P1〜P4・E1 マージ後の現状を実コードで確認）。

## 0. 分析サマリ

- **既存資産が厚い「上載せ」案件**: 純粋統計層（`internal/chart/stats.go`）・ECharts option 生成（`echarts.go`）・markArea 自前注入（`vpd_echarts.go`）・期間バケット/欠測 "—" 補完（`readings_report.go`/`device_show.go`）・通信遅延（`formatDelay`）・所有者認可（`authz.RequireDeviceOwner`）・domain 列挙パターン（`Metric`/`Locality`/`Crop`）がすべて揃っており、**品質メタは流用 + 新規純関数 + ViewModel 拡張で実装できる**。新規 SQL も原則不要（`ListSensorReadingsInRange` の全行走査で足る）。
- **新規に書くのは純関数 9 種**（Zスコア・IQR・stuck/flatline・物理範囲・急変・欠測率・間隔一貫性 + 欠測ギャップ抽出 + xAxis markArea 注入）。Zスコア/ローリングσは既存 `Mean`/`StdDev`/`SMA`/`MovingStdDev`/`Band` の再利用で組める。
- **最大の設計論点（新規発見）**: **device-show グラフと readings 画面は時間境界が別経路**。グラフは `ListRecentSensorReadings`（`recorded_at >= since` の**開区間**）、readings は `parseDateBounds` の **BETWEEN**。R3 期間欠測率（readings 側）と R5 欠測ギャップ（グラフ側）が**別の窓・別の分母**になりうる（C1）。
- **欠測点の nil 出力に型変更リスク**: グラフ series[0] の `ChartSpec.Raw` は `[]float64`（非ポインタ）。欠測点を nil で出すには `Raw` を `[]*float64` 化するか別経路が要る。乖離率（`Deviation []*float64`）に nil=欠落点の前例あり（C2）。
- **スキーマは既定で非変更**（goose 00009 のまま）。永続品質列は人手キュレーション/受信タグ付けが要るときのみ design で例外採用（研究項目①）。バッジ信号色は既存トークン（`--color-primary/warning/danger`）流用・`@layer components` へ追記（utilities ではない）。
- **総合見積り: M（3〜7 日）／リスク Medium**（大半 Low、欠測ギャップ可視化と時間境界整合のみ Medium-High）。

---

## 1. 現状資産の棚卸し（Current State）

### 1.1 純粋層 `internal/chart/`（math のみ・time 非依存）

| 資産 | シグネチャ（要点） | 品質メタでの用途 |
|---|---|---|
| `stats.go` `Mean` | `Mean([]float64) float64`（空=0） | Zスコアの μ |
| `StdDev` | `StdDev([]float64) float64`（**母σ・N除算**・空/単一点=0） | Zスコアの σ |
| `MinMax` / `DiurnalRange` | 既存 | 物理範囲・参考 |
| `CV` | `CV(values, eps) (float64, bool)`（μ≈0 で false） | 間隔一貫性の CV |
| `SMA` / `MovingStdDev` / `Band` | expanding window | ローリングσ法（移動窓 μ±kσ）= 再実装不要 |
| `Deviation` | `Deviation(values, sma, eps) []*float64`（nil=欠落点） | **nil=欠落点の前例**（欠測ギャップに転用可） |
| `echarts.go` `ChartOptionJSON` | `ChartOptionJSON(ChartSpec) (string, error)`・series[0]=Raw+markPoint(max/min)・`ShowSymbol(false)` を SMA/乖離率で使用・`connectNulls` **未使用**・`encoding/json`（SetEscapeHTML=true 既定） | 欠測ギャップを載せる対象 |
| `series.go` `ChartSpec` | `{Labels, Unit, Color, Raw []float64, SMA, BandLower, BandWidth, Deviation []*float64}` | 拡張点（欠測点 nil・gap mask） |
| `vpd_echarts.go` `injectVPDMarkArea` | JSON→map→series[0] へ**小文字 `yAxis` キー**の markArea を自前注入→再 Marshal で HTML 安全化 | **xAxis 範囲版 markArea の前例**（キーを yAxis→xAxis に置換） |

テスト体系: 表駆動 + JSON スキーマアサート + 許容誤差比較（`stats_test.go`/`echarts_test.go`/`vpd_echarts_test.go`/`vpd_test.go`）。品質純関数も同型でテスト可能。

### 1.2 ハンドラ層 `internal/handler/`

| 資産 | 要点 | 品質メタでの接続点 |
|---|---|---|
| `readings.go` `parseDateBounds(from,to)` | `→(fromTS,toTS,errs)`・YYYY-MM-DD→**BETWEEN**・JST 暦日・センチネル（distantPast/distantFuture）。Index/Export で**共有** | R3 期間メトリクスはこの区間で算出 |
| `readings.go` `fetchResults` | Count/Paginated/Summary/**InRange** を**同一境界**で呼び `DeviceReadingsListView` を組む | 品質メトリクスを View に足す注入点 |
| `readings.go` `buildReadingHistoryRows` / `formatDelay(recordedAt,createdAt)` | 通信遅延「N秒」整形・**負値0クランプ**・`math.Round` | R3 通信遅延・R1 行フラグの注入点（流用・再実装しない） |
| `readings_report.go` `readingsDailyRows`/`emptyReadingsReportRow` | JST 暦日/時バケット・**欠測日 "—" 補完**・`metricBucket.row()` が `chart.Mean/MinMax/StdDev/CV/VPDSeries/TimeInRange` を呼ぶ | 欠測「率」/ギャップ抽出はこの暦日連続走査を一般化 |
| `readings_export.go` `Export` | `parseDateBounds`→**`ListSensorReadingsInRange`**（全行 ASC）→CSV（BOM+meta+列） | R7-CSV 連携（品質列追加の余地・⑦） |
| `device_show.go` `buildChartArea` | **`ListRecentSensorReadings`（`recorded_at >= since` 開区間）** を使用・BETWEEN ではない。`dailyStatRows`（JST 暦日・"—" 補完）・`statEmptyMark="—"`・`jstDay()` | R5 欠測ギャップ・dailyStatRows に品質列 |
| `authz.RequireDeviceOwner(ctx, repo, id, uid)` | `→(device, err)`。`ErrUnauthenticated→401`/`pgx.ErrNoRows→404`/`ErrNotOwner→403`。readings.go L72・device_show.go で既呼出 | R8 をそのまま継承（新規認可なし） |

### 1.3 クエリ `db/queries/sensor_readings.sql`

| クエリ | 境界 | ORDER | LIMIT | 用途 |
|---|---|---|---|---|
| `ListSensorReadingsInRange` | **BETWEEN** | ASC | なし | CSV/帳票/**品質走査（流用）** |
| `ListSensorReadingsPaginated` | BETWEEN | DESC | あり | 一覧 |
| `GetSensorReadingsSummary` / `CountSensorReadingsInRange` | BETWEEN | — | — | 集計/件数 |
| `ListRecentSensorReadings` | **`>=`（開区間）** | ASC | なし | **グラフ（device-show）** |
| `ListDailySensorAggregates` | `>=` | ASC | — | 日次集計 |

→ **品質スキャンは `ListSensorReadingsInRange` の全行を Go 純関数で走査すれば足り、新規 SQL は原則不要**（永続列を採らない限り DDL なし）。

### 1.4 ビュー / ドメイン / CSS

- **ViewModel**（`internal/view/component/views.go`）: `ReadingHistoryRow{RecordedAt,Temp,Humidity,Delay}`・`ReadingsReportRow`（温湿度×6指標+滞在率）・`DeviceReadingsListView{Rows,Report,HasData,…}`・`DailyStatRow`。いずれも**末尾に品質フィールドを非破壊追加できる**。
- **templ**: `DeviceReadingsList.templ` が通信遅延列を描画。`DeviceShow.templ`/`Readings.templ`。handler が組んだ ViewModel を描画（view→domain 表示メソッドのみ可）。
- **domain 列挙**（`Metric`/`Locality`/`Crop`）: `type X string` + 定数 + `Label()`/`Unit()`/`Valid()`/`ParseX`/`AllX`・**fmt のみ依存・純粋**。`Locality.Municipality()` あり。→ `domain.QualityFlag` を同書式で新設可能（⑨）。
- **CSS**（`mocks/html/style.css`・単一正本）: 信号色トークン `--color-primary #28a745`（緑）/`--color-warning #ffc107`（黄）/`--color-danger #dc3545`（赤）が**既存**。`.summary-box`/`.data-table` 既存。**`.badge` クラスは未定義**（現状の状態色は `status-active/inactive`）。`@layer reset,base,components,utilities`。
  - ⚠️ **補正**: 品質バッジは部品スタイルゆえ **`@layer components`** に追記する（structure.md/tech.md）。utilities（`.u-*` ヘルパ）ではない。第3調査の「utilities へ」は規約違反なので不採用。

---

## 2. 要件 → 資産マップ（ギャップ・タグ: ✅流用 / 🟡拡張 / 🔴新規 / ⚠制約）

| 要件 | 必要能力 | 既存資産 | ギャップ |
|---|---|---|---|
| **R1 行フラグ** | 各行に欠測/stuck/物理異常/外れ値バッジ | `ReadingHistoryRow`・`DeviceReadingsList.templ`・`.data-table` | 🔴 行フラグ算出（純関数）・🟡 行 ViewModel +フラグ・🔴 `.badge` CSS・⚠ ⑧ 表示単位（全行 vs 異常行強調） |
| **R2 フラグ定義（物理異常=CHECK内側）** | Zスコア/IQR/stuck/物理範囲/急変 | `Mean`/`StdDev`（Zスコア）・`SMA`+`MovingStdDev`+`Band`（ローリング） | 🔴 IQR・stuck/flatline・物理範囲（農学）・急変の新規純関数・⚠ C3 CHECK 内側・⚠ ④しきい値・③手法 |
| **R3 期間メトリクス** | 欠測率%/間隔一貫性/通信遅延代表値 | `parseDateBounds`・`formatDelay`・`CV`・`readingsDailyRows` | 🔴 欠測率・間隔一貫性純関数・🟡 メトリクスボックス・⚠ ②期待間隔（分母） |
| **R4 総合バッジ** | 信号色・合成判定 | 色トークン既存・domain 列挙パターン | 🔴 合成式・🔴 `.badge` 信号色（components）・⚠ ⑤合成式/配置画面 |
| **R5 欠測ギャップ可視化** | 線分断+連続欠測 markArea+凡例 | `echarts.go`・`injectVPDMarkArea`（yAxis）・`Deviation []*float64`（nil 前例） | 🔴 欠測点 nil 出力・🔴 xAxis markArea 注入・🟡 凡例/注記（器）・⚠ C1 経路差・C2 Raw 型 |
| **R6 境界条件** | 0件/単一点/全欠測/σ≈0 | `CV` の bool・`statEmptyMark "—"`・空 HasData | 🔴 純関数の境界実装（表駆動テスト） |
| **R7 器差スコープ** | 単一 device 確定・限定 or P10 defer | `devices.locality`（P1） | ⚠ ⑥スコープ判断（位置軸=P10）・限定版なら「同一地域≠同一ハウス」注記 |
| **R8 所有者認可** | 非所有→404 | `authz.RequireDeviceOwner` 既呼出 | ✅ そのまま継承（新規なし） |
| **R9 無回帰** | P2/S6/P4/E1 維持 | 既存テスト（echarts/stats/readings） | ⚠ C2 ChartSpec 型変更が既存 4 チャートテスト・P2 オーバーレイに波及しないこと |
| **R10 モック整合** | 器をモック反映・描画は例外 | `mocks/html/{device_show,readings}.html`+`style.css` | 🟡 モック更新（バッジ/フラグ列/メトリクスボックス/凡例）・`make sync-css` |

---

## 3. 制約（既存アーキ由来・必読）

- **C1（最重要）時間境界の二経路**: グラフ=`ListRecentSensorReadings`（`>= since` 開区間）／readings=`parseDateBounds` BETWEEN。**R5 欠測ギャップ（グラフ窓）と R3 欠測率（readings 窓）は別窓・別分母になりうる**。design で「整合させるか（グラフにも BETWEEN 経路を用意 or readings の品質を period 切替に合わせる）」「別物として注記するか」を確定（研究項目⑩）。
- **C2 欠測点 nil 出力の型**: series[0] `ChartSpec.Raw` は `[]float64`。nil 欠測点を出すには (a) `Raw` を `[]*float64` 化、(b) 別の gap マスク列を足す、のいずれか。`Deviation []*float64` の nil=欠落点が前例。**既存 echarts テスト/P2 オーバーレイへの後方互換を壊さない実装**を design で起こす（研究項目⑪）。
- **C3 物理異常は CHECK の内側**: 受信時 CHECK（temp −40〜125・hum 0〜100）で範囲外は保存され得ない。純関数の物理範囲は**農学的下限上限を定数化**（DB CHECK の追認ではない）。
- **C4 time は handler 境界**: recorded_at 差分（間隔列・秒）は handler で算出し、純関数へ `[]float64`（秒）を渡す（`dailyStatRows`/`vpdHourlyRows` 作法）。chart 層に time を持ち込まない。
- **C5 純粋性**: `internal/chart/quality.go`（新設）は **math のみ**。`domain.QualityFlag`（採るなら）は **fmt のみ**。
- **C6 CSS 単一ソース**: 編集は `mocks/html/style.css`→`make sync-css`。バッジ信号色は `@layer components`・既存色トークン流用・独自クラス新設は最小。
- **C7 認可継承**: `RequireDeviceOwner` 既呼出のため R8 は無償。新経路を足す場合も同関数を通す。
- **C8 重い統計の除外**: STL/Mann-Kendall/回帰は CSV 外出し（ガードレール⑧・別フェーズ）。本フェーズは軽量異常検知+率集計のみ。

---

## 4. 実装アプローチ（Options）

### Option A: 既存コンポーネント拡張（stats.go 追記・ViewModel/handler 直接拡張）
- **内容**: 品質純関数を `stats.go` に追記、`echarts.go`/`series.go` を直接拡張、ViewModel に品質フィールド追加、handler 内で組み立て。
- ✅ 新規ファイル最小・既存パターン踏襲で初速最速。
- ❌ `stats.go` が肥大（基本統計と異常検知が混在）。`ChartSpec` 直接改変が既存テストへ波及。単一責務が薄れる。

### Option B: 新規コンポーネント中心（quality.go・専用 component・専用 handler ヘルパ）
- **内容**: `internal/chart/quality.go` 新設、品質専用 templ component（`QualityBadge`/`QualityMetricsBox`）、`readings_quality.go`/device 品質ヘルパ新設、`domain.QualityFlag` 新設。
- ✅ 関心の分離・単体テスト容易・`stats.go` を汚さない。
- ❌ ファイル数増・interface 設計が必要。グラフ拡張は結局 echarts.go に触れる。

### Option C: ハイブリッド（推奨）
- **純粋層**: `internal/chart/quality.go` を**新設**（IQR・stuck/flatline・物理範囲・急変・欠測率・間隔一貫性）。Zスコア/ローリングσは既存 `Mean`/`StdDev`/`SMA`/`MovingStdDev`/`Band` を**流用**（再実装しない）。`stats.go` は無改変。
- **グラフ**: `echarts.go`/`series.go` を**最小拡張**（欠測点 nil + xAxis markArea を `injectVPDMarkArea` 方式踏襲で新規注入）。C2 は後方互換を最優先。
- **ViewModel/handler**: `ReadingHistoryRow`/`ReadingsReportRow`/`DeviceReadingsListView`/`DailyStatRow` に品質フィールドを**末尾非破壊追加**。間隔列・遅延は handler 境界で算出し純関数へ `[]float64`。
- **View**: `QualityBadge`/`QualityMetricsBox` を**新規 component**化、行フラグは `DeviceReadingsList.templ` に写経追加。`domain.QualityFlag` は⑨で採否判断（採れば view→domain 表示メソッドで描ける）。
- **段階化**: 第1段=純関数 + readings 期間メトリクス/行フラグ + 総合バッジ（スキーマ非変更・低リスク）。第2段=device-show 欠測ギャップ可視化（C1/C2 を design で解決してから）。
- ✅ 純粋性・無回帰・関心分離を満たしつつ既存資産を最大流用。第2段の高リスクを切り出せる。
- ❌ 計画がやや複雑。第1/第2段の View 整合に注意。

**推奨**: **Option C**。本案件は「上載せ・流用」が前提で、純関数は新規ファイルに隔離、グラフ拡張は前例踏襲の最小改変、ViewModel は非破壊追加が自然。高リスクな欠測ギャップ（C1/C2）を第2段に分離できる。

---

## 5. 工数 / リスク（領域別）

| 領域 | 工数 | リスク | 根拠 |
|---|---|---|---|
| 品質純関数（quality.go・IQR/stuck/物理/急変/欠測率/間隔一貫性） | M | Low | 表駆動テスト前例あり・math のみ・関数数が多く境界条件が要点 |
| readings 期間メトリクス + 行フラグ + ViewModel/templ | M | Low-Med | `fetchResults`/report バケット/templ/モックに跨る・分母（②）未確定 |
| 総合バッジ + CSS | S-M | Med | 合成式（⑤）は design/research 判断・CSS 自体は小 |
| 欠測ギャップ可視化（nil 点 + xAxis markArea） | M | **Med-High** | C2 `ChartSpec.Raw` 型変更が既存テスト/P2 に波及・xAxis markArea は前例なし・C1 経路差 |
| `domain.QualityFlag` 列挙（任意） | S | Low | 既存列挙書式を踏襲・純粋 |
| モック反映 + `make sync-css` | S | Low | 器のみ（描画は例外）・既存クラス流用 |
| **合計** | **M（3〜7日）** | **Medium** | 大半 Low、ギャップ可視化と時間境界整合のみ Med-High |

---

## 6. 設計フェーズへの引き継ぎ

### 推奨アプローチ
- **Option C（ハイブリッド・段階化）**。第1段（純関数+readings メトリクス/フラグ+バッジ・スキーマ非変更）→第2段（欠測ギャップ可視化・C1/C2 解決後）。

### design の最初の主要決定
1. **スキーマ非変更を確定**（既定・研究①）。永続品質列は人手キュレーション/受信タグの確定要件が無ければ採らない。採るなら expand-contract + `make db-snapshot`。
2. **C1 時間境界の整合方針**（研究⑩・新規）: グラフ（開区間）と readings（BETWEEN）で品質窓をどう揃えるか／別物として注記するか。
3. **C2 欠測点 nil 出力の実装**（研究⑪・新規）: `Raw []*float64` 化か gap マスクか。既存 echarts テスト/P2 オーバーレイの後方互換を不変条件に。
4. **R4 バッジ合成式と配置**（研究⑤）: 緑/黄/赤の境界（欠測率/外れ値件数/stuck 有無/間隔 CV）と device-show / readings / 双方の別。
5. **R7 器差スコープ**（研究⑥）: P10 defer か locality 限定版か（限定版なら「同一地域≠同一ハウス」明示）。

### 研究項目（design / research へ持ち越し）
- ① スキーマ: 読み取り時計算（既定）vs 永続列。
- ② 期待サンプリング間隔（欠測率の分母）: 観測中央値推定 / 既定 5 分 / device 設定（避けたい）。firmware ≈5 分。
- ③ 外れ値手法: 大域 Z/IQR vs ローリングσ（昼夜変動の誤検出回避）・しきい値 k・IQR 係数。
- ④ stuck N / 農学的下限上限 / 急変率: 沖縄の高湿度長期継続を異常としない（ユーザー実地知見で確定）。
- ⑤ バッジ合成式 + 配置画面。
- ⑥ 器差スコープ（P10 defer or locality 限定）。
- ⑦ CSV 品質列連携（P4 への非破壊追加を本フェーズに含めるか）。
- ⑧ 行フラグ表示単位（全行 vs 異常行強調）。
- ⑨ `domain.QualityFlag` 列挙化の要否。
- ⑩【新規】C1 グラフ/readings の時間境界乖離の整合。
- ⑪【新規】C2 欠測点 nil 出力の型設計と既存チャートテスト/P2 後方互換。

---

# 設計シンセシス・決定事項（/kiro-spec-design・2026-06-28）

> discovery（ギャップ分析）+ HTMX ガイド §3.3/§3.5/§4 確認後の synthesis。①〜⑪を確定。design.md の根拠。

## HTMX 接続点（ガイド §3.3/§3.5/§4 所見）

- **device-show**: 期間切替は `GET /devices/{device}/chart` → `DeviceChartArea.templ`（`#device-chart-area` = 温度/湿度グラフ + 期間セレクタ）を innerHTML 差し替え。**欠測ギャップ可視化はこのフラグメント内**。新規エンドポイント不要。
- **readings**: 期間フィルタ/ページ送りは `GET /devices/{device}/readings` → `DeviceReadingsList.templ`（`#device-readings-list` = `.summary-grid`/`.data-table`/`.pagination`）を innerHTML 差し替え。**品質バッジ・期間メトリクスボックス・行フラグ列はこのフラグメント内**（フィルタ連動で再描画）。新規エンドポイント不要。GET のみ＝CSRF 対象外。
- → 品質メタは**既存の部分更新領域に相乗り**。HTMX 属性・id・ルートの新設はゼロ。

## Generalization / Build-vs-Adopt / Simplification

- **Generalization**: 5 種の異常検知（Zスコア/IQR/stuck/物理範囲/急変）と 2 種の率（欠測率/間隔一貫性）は「`[]float64`（+ 間隔は秒列）を受けてフラグ/率を返す純関数群」に一般化。新設 `internal/chart/quality.go` に集約（math のみ・time 非依存）。
- **Build-vs-Adopt**: Zスコア・ローリングσは既存 `Mean`/`StdDev`/`SMA`/`MovingStdDev`/`Band` を**流用（再実装しない）**。markArea は `injectVPDMarkArea` 方式を**流用**（xAxis 版を起こす）。通信遅延は `formatDelay` を**流用**。重い統計は不採用（CSV 外出し・別フェーズ）。
- **Simplification**: 器差は P10 へ defer（限定版も作らない）。CSV 品質列は本スペック対象外（将来の非破壊追加）。外れ値は**ローリングσ法を主**とし（昼夜変動に頑健）、IQR/Zスコアは純関数として用意するが判定の主経路に乗せない（テスト容易性と将来調整の余地のためだけに持つ）。

## 決定事項（未確定①〜⑪の確定）

- **① スキーマ = 非変更（読み取り時計算）**。goose 00009 のまま・`make db-snapshot` 不要・新規 SQL なし（`ListSensorReadingsInRange` の全行走査）。人手キュレーション/受信タグの確定要件が無いため永続列を採らない。
- **② 期待サンプリング間隔 = 観測間隔の中央値**（median）。recorded_at 差分列（秒）の中央値を期待間隔とし、欠測本数 = Σ max(0, round(interval/median) − 1)。firmware 変更・device 差・gap 自体に頑健。2 点未満は欠測率「—」。
- **③ 外れ値手法 = ローリングσ法（SMA ± k·σ, window W）を主**。k=3・W=design 定数（既定 12 点 ≈ 1h@5分）。昼夜変動を窓が追従し誤検出を抑える。warm-up 区間（index < W−1）と σ≤ε は「外れ値なし」。IQR/Zスコアは純関数として実装するが主経路外。
- **④ しきい値 = 定数化（research/沖縄ユーザー確認・コメントに根拠）**。既定: stuck N=6（2 小数まで完全同値の連続＝固着）/ 物理範囲 温度 [−10,60]℃（沖縄ハウス・据置故障疑い）/ 急変 温度 |Δ|>10℃・湿度 |Δ|>40%RH（隣接サンプル間）。**湿度の長時間高止まりは正常**（沖縄）ゆえ湿度の物理下限上限は設けず固着（exact 同値連続）でのみ検出。
- **⑤ バッジ合成式 + 配置 = readings のみ**。合成（信号色）: 赤=欠測率>30% or stuck 検出 or 物理異常>0件; 黄=欠測率>5% or 外れ値率>5% or 間隔CV>0.5; それ以外=緑。境界は定数（research 調整可）。**バッジ・期間メトリクス・行フラグは readings に集約**（BETWEEN 経路＝集計/一覧/CSV と整合する研究用詳細表示）。device-show は欠測ギャップ可視化のみ。
- **⑥ 器差 = P10 へ defer**。位置軸（内外/東西）が無く locality は粗すぎるため限定版も作らない。本スペックは単一 device に確定。
- **⑦ CSV 品質列 = 本スペック対象外（将来の非破壊追加）**。R1 の行フラグ算出を流用すれば P4 CSV へ低コストで後付け可能だが本スペックには含めない。
- **⑧ 行フラグ表示 = 異常行のみ強調**。正常行はバッジ無し（空）。フラグ列は存在するが大半空で異常が際立つ（R1.3 クラッタ回避）。
- **⑨ domain.QualityFlag 列挙 = 新設**（`internal/domain/quality_flag.go`・`type QualityFlag string`・`Label()`・純粋 fmt のみ）。加えて総合品質 `QualityLevel`（good/caution/bad・`Label()`・CSS クラス対応）も domain 列挙化。**いずれも DB 非永続**ゆえ §100 の VARCHAR+CHECK は不要＝純 Go 列挙（計算由来の表示カテゴリ）。view→domain 表示メソッドで描く。
- **⑩ C1 時間境界 = 整合させない（別ビューとして割り切る）**。readings の品質メトリクス/バッジは parseDateBounds の **BETWEEN**（一覧/集計/CSV と一致する正確な率）。device-show の欠測ギャップは既存グラフの**開区間窓**（`ListRecentSensorReadings`）の**視覚表現のみ**で率の数値を出さない。両者は矛盾する数値ではなく別目的のビュー。
- **⑪ C2 欠測点 nil 出力 = `ChartSpec` に nil 対応フィールドを追加（既存 `Raw []float64` は不変）**。`RawNullable []*float64`（仮称）を新設し、handler が欠測スロットを含む拡張グリッド（Labels + 各系列を gap スロットで nil 埋め）を組んで渡す。`echarts.go` は `RawNullable` 非 nil 時に series[0] data をそれに切替え、`connectNulls:false`（既定を明示）で線分断。**`RawNullable` nil 時は完全に従来挙動**（既存 echarts/P2 テスト不変＝後方互換の不変条件）。P2 オーバーレイは既定 off ゆえ拡張グリッドへの整合は段階対応可。欠測区間 markArea は `injectVPDMarkArea` 踏襲の新規 `injectGapMarkArea`（xAxis 範囲）。
