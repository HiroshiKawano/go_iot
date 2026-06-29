# ギャップ分析: dewpoint-disease-risk（露点・病害リスク）

> 目的: 要件（WHAT）と既存コードベースの差分を分析し、design フェーズの実装戦略に資する。本書は判断を確定せず、選択肢と根拠・research 項目を提示する。
> 実地検証日: 2026-06-29（P3 vpd-dashboard ＋ P5 data-quality-meta マージ後の HEAD）。phase-06 プロンプトが主張する全拡張点を実コードで確認済み。

## 1. 結論サマリ

- **本機能は P3（vpd-dashboard）のほぼ完全な構造的並行**である。P3 は P2 の写経で「別パネルを読み取り時に組む」作法を確立しており、露点パネルは同作法の第2適用にあたる。**新規アルゴリズム（露点 Td・結露帯・葉面湿潤・高湿度イベント）の純粋層を足し、既存への結線は非破壊追加に限る**のが自然。
- phase-06 プロンプトが列挙した拡張点（`vpd.go` の Tetens 定数・`injectGapMarkArea` の xAxis 注入・`StuckRuns`/`RapidChanges`・`buildVPDPanel`/`vpdHourlyRows`・`crop.go` の非破壊追加フック・`VPDChartSpec` 別型隔離・`DeviceChartAreaView`/`VPDPanelView`・モックの `--color-vpd`）は**すべて実在しシグネチャも一致**。発明ゼロで写経できる。
- **推奨アプローチ = ハイブリッド（新規ファイル中心＋既存への最小非破壊結線）**。`internal/chart` 純粋層・echarts 描画層・handler・view・モックの各層に新規ファイルを起こし、既存は「末尾フィールド追加・呼び出し1行・crop メソッド追加・templ 追加描画・モック追加」に留める。
- **スキーマ非変更が成立**（要件 R1.2/R6.4）。露点・結露・葉面湿潤・高湿度イベント・病害スコアは `sensor_readings`（temperature/humidity の2列）から読み取り時算出で足り、病害モデルしきい値は `domain.Crop` の Go 定数。goose 最新 00009 のまま・`make db-snapshot` 不要。**唯一のスキーマ論点は発病記録テーブル（既定スコープ外）**。
- **主リスクは2点**: ①物理規約の向き（湿り側=寒色）— P3 VPD でテスト全緑のまま実機まで誤りが残った前例（メモ project_vpd_physics_convention）。②病害スコアモデルの確度（△・research/未確定）。いずれも数値計算の難度ではなく「ドメイン意味の正しさ」に起因。

## 2. 現状調査（Current State）

### 2.1 検証した既存資産（すべて実在・シグネチャ一致）

| 層 | ファイル | 再利用する資産 | 露点での用途 |
|---|---|---|---|
| 純粋層 | `internal/chart/vpd.go` | `const tetensA=0.6108 / tetensB=17.27 / tetensC=237.3`、`saturationVaporPressure`、`VPD`/`VPDSeries`、`TimeInRange(values,lower,upper)` | 露点 γ 式で **tetensB/tetensC を再利用**（tetensA は飽和水蒸気圧専用＝露点では未使用）。`TimeInRange` を葉面湿潤割合に流用 |
| 純粋層 | `internal/chart/stats.go` | `Mean`/`MinMax`/`SMA`/`StdDev`/`DiurnalRange` | 露点カード集計・スプレッド統計に流用 |
| 純粋層 | `internal/chart/quality.go` | `StuckRuns(values,minRun)`〔同値連続ラン〕、`RapidChanges(values,maxDelta)`〔急変〕、`GapSpan`/`MissingStats` | **高湿度継続イベント**は連続ラン検出と同型（ただし「同値」でなく「しきい値以上」の述語＝新関数が要る） |
| 描画層 | `internal/chart/vpd_echarts.go` | `VPDChartOptionJSON(spec)`、`injectVPDMarkArea`〔小文字 `yAxis` キー自前注入〕、`vpdZone(lo,hi,color)` | 露点 option（露点 Td 線＋気温重ね）の写経元 |
| 描画層 | `internal/chart/gap_echarts.go` | `injectGapMarkArea(optionJSON,bands)`〔小文字 **`xAxis`** 範囲キー注入〕、`nullableLineData([]*float64)`、`gapZone(startIdx,endIdx)`、`GapBand` | **結露帯（時間区間ハイライト）は xAxis 範囲ゆえ同型**。ただし `gapZone` は灰色 `gapBandColor` をハードコードし色引数を取らない（後述ギャップ） |
| 入力契約 | `internal/chart/series.go` | `ChartSpec`（末尾に `RawNullable`/`GapBands` 非破壊追加済）、`VPDChartSpec`（VPD 専用に別型隔離）、`GapBand` | **`DewpointChartSpec` を別型で隔離**（P3 が VPDChartSpec を別型にした作法） |
| handler | `internal/handler/device_show.go` | `buildChartArea(ctx,device,period,now)`、`deviceCrop(device)`、`smaWindowFor(period)`、`jst`/`jstDay`、`dailyStatRows`、`statEmptyMark="—"`、`applyGapGrid` | `buildChartArea` 末尾で `buildDewpointPanel(...)` を呼び `DeviceChartAreaView` へ詰める拡張点。JST 暦日バケットは `jstDay` を流用 |
| handler | `internal/handler/device_show_vpd.go` | `buildVPDPanel(labels,temps,hums,rows,crop,period)`、`vpdHourlyRows`〔JST hour-of-day〕、`vpdMaxDeviation`〔逸脱量+方向・**符号付け**〕、`formatVPD`/`formatPercent`/`emptyVPDCard` | **`device_show_dewpoint.go` の `buildDewpointPanel` の写経元**。日次積算は `dailyStatRows` 作法で |
| domain | `internal/domain/crop.go` | `type Crop`＋9作物、`Label()`/`Valid()`/`VPDRange()`/`AllCrops()`/`ParseCrop`、`DefaultVPDLower/Upper`、**「病害モデル等の他属性は別フェーズが非破壊追加する前提」コメント（13行目）** | **病害モデルしきい値を Go 定数+メソッドで非破壊追加**（DB 列を増やさない） |
| view | `internal/view/component/views.go` | `DeviceChartAreaView`（末尾に `VPD VPDPanelView`/`HasGap`）、`VPDPanelView`/`VPDCardView`/`VPDHourlyRow` | **`DewpointPanelView`/`DewpointCardView`/葉面湿潤日次行/高湿度イベント行 DTO** をイミュータブル追加（末尾非破壊） |
| view | `internal/view/component/DeviceChartArea.templ` | `@vpdPanel(v.VPD)`、`#vpd-chart data-echarts data-unit="kPa" data-color`、`optionScript`、`.summary-grid-4`/`.data-table` 流用 | 露点パネル（`#dewpoint-chart`）・露点カード・葉面湿潤日次表・高湿度イベント表を VPD パネルの下に追加描画 |
| layout | `internal/view/layout/App.templ` | `EChartsInitializer`：scope 内の**全 `[data-echarts]` を init し `instances.length>1` で `echarts.connect(instances)`** | 露点グラフ追加で**自動的に connect グループへ参加**（温℃/湿%/VPD kPa が既に混在連動済み＝追加作業不要） |
| モック | `mocks/html/style.css` / `mocks/html/device-show.html` | `--color-vpd:#0ca678`（71行）、VPD パネルの器（176-210行）、`feedback_mock_graph_rendering_exception` 注記 | **`--color-dewpoint` 新色トークン追加**＋露点パネルの器をモックへ反映し `make sync-css` |

### 2.2 規約・制約（structure.md / tech.md）

- **依存方向は下向き一方向**。`internal/chart` は最下流純粋層（`math` のみ・time 非依存・gin/DB/templ/pgtype を import しない）。露点純関数も同純粋性を厳守。
- **マスタ/列挙＝Go 定数+VARCHAR+CHECK**（§100）。病害モデルしきい値は DB に持たない（CHECK 不要）。9作物の集合・並びは `devices.crop` CHECK（00009）と一致維持。
- **所有者認可は `internal/authz` 集約**（`RequireDeviceOwner`）。露点パネルは `Show`/`Chart` の既存認可フロー上で描画されるため、新規認可は不要（消費のみ・要件 R8.1/R8.2）。
- **イミュータブル**：sqlc 生成構造体は読取専用、純関数は入力スライスを破壊しない。
- **CSS 単一ソース**：正本 `mocks/html/style.css`→`make sync-css`。templ はモック写経・独自クラス新設は最小（§31）。
- **既定はマイグレーション無し**（CLAUDE.md）。発病記録テーブルを採る場合のみ expand-contract＋`make db-snapshot`。

## 3. 要件→資産マッピング（ギャップ）

| 要件 | 必要な技術要素 | 既存資産 | ギャップ種別 |
|---|---|---|---|
| R1 露点 Td 算出 | `DewPoint(temp,rh)`/`DewPointSeries`、RH 下限クランプ、RH=100→Td=T、氷点下安全 | `vpd.go` の tetensB/C・VPDSeries 同型 | **Missing**（新関数・`dewpoint.go`）。難度低（確定式） |
| R2 露点時系列＋気温重ね＋結露帯 | 露点 option（2系列）、結露帯 xAxis markArea、スプレッド T−Td、結露帯判定（代理） | `vpd_echarts.go`/`gap_echarts.go` の注入パターン、`vpdZone` | **Missing+Constraint**。`gapZone` が色引数を取らない（灰色固定）→ 汎用化 or 新 injector が必要 |
| R3 葉面湿潤時間の日次積算 | RH≧しきい値の連続→時間換算→JST 暦日積算 | `jstDay`/`dailyStatRows` 作法、`TimeInRange` | **Missing**（純関数の連続ラン＋handler 境界の暦日バケット）。難度中（日跨ぎ・間隔秒換算） |
| R4 高湿度継続イベント抽出 | RH≧しきい値が最小継続以上の `[]EventSpan`（開始/終了/継続/最小スプレッド） | `StuckRuns` と同型（述語違い） | **Missing**（しきい値述語の連続ラン新関数）。難度低〜中 |
| R5 病害スコア下地（△） | 温度帯×葉面湿潤の最小合成、最低1作物で具体値 | — | **Missing+Unknown**（確定モデルは research・下地のみ実装） |
| R6 作物別病害モデルしきい値 | `Crop` へ温度帯/湿潤しきい値、未設定/露地はフォールバック | `crop.go` 非破壊追加フック・`VPDRange` の作物グルーピング | **Missing**（`DiseaseModel()` 等の新メソッド/定数）。DB 列追加なし |
| R7 無回帰維持 | 温湿度2グラフ・統計・VPD・ギャップ・品質・期間切替・connect | 既存全機能 | **Constraint**（別型隔離＋末尾非破壊で守る。回帰テスト要） |
| R8 アクセス制御・研究用境界 | 所有者認可（消費）、アラート/通知・農家共有・THI/GDD を出さない | `authz.RequireDeviceOwner`、既存 `Show`/`Chart` | **充足**（新規不要・境界の遵守のみ） |

## 4. 実装アプローチ（A/B/C）

### Option A: 既存ファイルへ全部足す（拡張一辺倒）
- `vpd.go`/`vpd_echarts.go`/`device_show_vpd.go` に露点関数を相乗り。
- ✅ ファイル数最小　❌ VPD ファイルが露点で肥大化し単一責務が崩れる・無回帰リスク増・P3 が確立した「指標ごとに別型/別ファイル」作法に反する。**非推奨**。

### Option B: 新規ファイル中心（推奨ベース）
- 純粋層 `internal/chart/dewpoint.go`（露点 Td/スプレッド/結露帯判定/葉面湿潤/高湿度イベント/病害スコア下地）。
- 描画層 `internal/chart/dewpoint_echarts.go`（`DewpointChartOptionJSON(DewpointChartSpec)`＋結露帯 markArea）。
- handler `internal/handler/device_show_dewpoint.go`（`buildDewpointPanel`）。
- `series.go` に `DewpointChartSpec` を**別型隔離**、`views.go` に `DewpointPanelView` 等を追加。
- ✅ 単一責務・隔離テスト容易・無回帰を構造的に守る（P3 と同じ作法）。❌ ファイル増（許容範囲）。

### Option C: ハイブリッド（B＋既存への最小非破壊結線）＝**最有力**
- Option B の新規ファイル群に加え、既存への結線を**非破壊の最小差分**に限定：
  - `device_show.go` `buildChartArea` 末尾に `buildDewpointPanel(...)` 呼び出し1行＋`DeviceChartAreaView` への代入。
  - `views.go` `DeviceChartAreaView` 末尾に `Dewpoint DewpointPanelView` フィールド追加（VPD/HasGap と同様）。
  - `crop.go` に病害モデルしきい値メソッド追加（既存メソッド・定数は不変）。
  - `gap_echarts.go` の `gapZone` を色引数化して汎用化、または露点用の別 markArea 関数を新設（design 判断）。
  - `DeviceChartArea.templ` に露点パネル描画を追加（VPD パネルの下）。
  - モック `device-show.html`/`style.css` に器＋`--color-dewpoint` を反映し `make sync-css`。
- ✅ P3 と完全に整合・無回帰を最小面積で担保・写経主体で確度高い。**推奨**。

## 5. 工数・リスク

- **総合工数: M（3〜7日）**。新規アルゴリズム5種（露点/スプレッド/結露帯/葉面湿潤/高湿度イベント/病害下地）＋描画＋handler＋view＋モック＋TDD。各層が P2/P3/P5 の写経で発明が少ない。
- **リスク: Low〜Medium**。
  - 数値計算（R1）= **Low**（確定式・境界が明確：RH=100→Td=T、RH=0 クランプ、氷点下分母 T+237.3>0、物理域 γ<17.27 で分母ゼロ非到達）。
  - 物理規約の向き（R2.4）= **Medium**（過去にテスト全緑のまま実機まで誤りが残った前例＝メモ project_vpd_physics_convention。**実機スモーク目視確認を tasks 必須化**で緩和）。
  - 病害スコアモデル（R5）= **Medium**（△・確度低。下地に限定＋確定モデルを research/将来へ送ることで本フェーズのリスクを封じる）。
  - 無回帰（R7）= **Low**（別型隔離＋末尾非破壊＋既存回帰テスト群 `device_show_vpd_regression_test.go` 等が既にある）。

## 6. design へ持ち越す research 項目（未確定）

1. **発病記録テーブル `disease_observations` を起こすか（最重要・design 最初の論点）**。既定=読み取り時計算・スキーマ非変更（環境側 risk 提示まで）を強く推奨。採るなら expand-contract＋`make db-snapshot`。要件 Boundary でも既定スコープ外。
2. **結露判定の代理しきい値**：スプレッド T−Td ≦ しきい値（例 1〜2℃）か RH ≧ しきい値（例 95%）か・値。**沖縄の夜間放射冷却・高湿度常態をユーザー実地知見（権威）で確定**。
3. **葉面湿潤時間のしきい値・最小継続**：RH 何%以上を「湿潤」とするか（例 90%）・連続何分以上を1イベントとするか（サンプリング約5分＝12点/時 前提）。
4. **病害スコアのモデル**：温度帯×葉面湿潤の具体式・作物別重み。下地に留めるか特定病害1つの簡易予察まで踏むか。
5. **`domain.Crop` への病害モデル属性の持たせ方**：`DiseaseModel()` メソッド／定数表。施設果菜（ゴーヤ/インゲン等）と露地（サトウキビ/米等）で属性が割れる前提・フォールバック設計。
6. **結露帯 markArea の実装手段**：`gapZone` を色引数化して汎用化するか、露点用の別 injector を起こすか（灰色固定の現状を寒色＝湿り側へ）。
7. **露点グラフ y 軸スケール**：気温と露点を重ねる共通℃軸・固定 or auto。結露帯（スプレッド小）が見やすいスケール。
8. **RH=0 クランプの床値**：RH<1% を Td 算出対象外（欠測扱い）にするか、RH を微小正値へ床上げするか。
9. **connect 参加の妥当性確認**：現状 `EChartsInitializer` は scope 内全 `[data-echarts]` を connect する＝露点グラフは追加だけで自動連動（温℃/湿%/VPD kPa が既に混在連動済み）。意図どおりか（℃ 軸の露点と他パネルのカーソル連動）を design で確認。新規実装は不要。
10. **P4 集計帳票への結露時間列追加**：結露時間の**算出**は本フェーズ `dewpoint.go` が提供（P4 後追い時の再実装回避）、P4 帳票への**列追加（消費）**を本フェーズで行うか後追いかは design 判断。
11. **期間別の粒度**：24h は瞬間/カード・複数日は日次葉面湿潤表、と P2/P3 の ShowDaily 相当で粒度を変えるか。

## 7. design への推奨

- **アプローチ = Option C（ハイブリッド）を採用**。P3 vpd-dashboard の design・実装を直接の雛形とし、「指標ごとに別型・別ファイル・末尾非破壊結線」を踏襲する。
- **最初に決める論点 = 上記 research 項目1（発病記録テーブルの採否）と 2/3（結露・葉面湿潤しきい値）**。1 は既定 No を推奨。2/3 はユーザー（沖縄実地知見）確認が望ましいが、提案型案件ゆえ暫定値で動くデモを先行させ確認をブロッカーにしない方針（メモ project_analysis_ideas_temp_humidity）。
- **物理規約の向き（湿り側=寒色）を design / テスト / 色トークン / ラベルに一貫実装し、GO 後の実機スモーク目視確認を tasks に必須化**（自動テストは仕様前提を符号化するため向きの誤りを捕捉できない＝前例の教訓）。
- **Testing Strategy**：露点 Td（RH=100 恒等・RH=0 クランプ・氷点下 NaN/Inf なし・既知手計算一致）、スプレッド（≥0）、結露帯判定（しきい値境界・連続区間・先頭/末尾/全域）、葉面湿潤（連続ラン・日跨ぎバケット）、高湿度イベント（最小継続境界・単発除外）、病害下地（合成・空入力）、結露帯 markArea 注入（小文字 xAxis・空区間）、**物理規約の向き**、所有者認可（非所有→404）、**無回帰**（VPD/ギャップ/品質/統計/温湿度/期間切替/connect）。`2cc_sdd/テストガイダンス集.md` の純粋関数テスト・templ Render 検証・Querier モックの定石に準拠。

---

## 設計フェーズ追記（design synthesis・2026-06-29）

> discovery（light・拡張機能）で得た所見に、design synthesis の3レンズと確定した設計判断を追記する。design.md の根拠。

### Synthesis 1: 一般化（Generalization）
- **結露帯（`CondensationRuns`）と高湿度イベント（`HighHumidityRuns`）は同一問題＝「boolean mask 上の連続区間抽出（最小ラン長つき）」**。`dewpoint.go` に内部 `runsFromMask(mask []bool, minRun int) []Run` を1本起こし、結露帯（spread ≤ maxSpread・minRun=1）と高湿度イベント（RH ≥ threshold・minRun=N）の2公開関数がマスクを作って共有する。`quality.go` の `StuckRuns`（同値ラン）と同型だが述語が違い、quality.go は P5 所有ゆえ流用せず chart 層に新設（domain 非依存を維持）。
- `Run{StartIdx,EndIdx}` 型を結露帯・高湿度イベント・（将来の）結露時間で共有。

### Synthesis 2: Build vs Adopt
- **露点式＝確定式を自前実装**（付録A D③・`tetensB`/`tetensC` を `vpd.go` から再利用＝重複定義回避）。外部ライブラリ不要（go-echarts は既存・新規依存ゼロ）。
- **markArea は P5 `injectGapMarkArea` の xAxis 自前注入パターンを adopt（写経）**。ただし P5 の `gapZone`（灰色固定）は改変せず、結露帯用の専用 injector を新設（寒色・無回帰優先）。go-echarts の `MarkAreaData` 型不具合（大文字キー）回避は確立済みパターンの踏襲。
- **病害モデルしきい値は `domain.Crop` の Go 定数で保持（テーブル化しない＝§100）**。P3 `VPDRange()` の作物グルーピング作法を adopt。

### Synthesis 3: 簡素化（Simplification）
- **発病記録テーブル `disease_observations` を採らない**（環境側 risk 提示まで・スキーマ非変更・YAGNI／確度△）。唯一のスキーマ論点を本フェーズから外し、goose 00009 のまま・`make db-snapshot` 不要に確定。
- **露点 y 軸は共通℃の auto 範囲**（VPD の `YMax` 算出ロジックは露点では不要＝結露帯は xAxis 範囲ゆえ y 上限を作る必要がない）。VPD より1要素少ない。
- **病害スコアは下地（最小合成・0..1）に限定**。確定予察モデル・特定病害予察は research/将来へ。
- **handler signature は不変**（P3 で `buildChartArea(ctx, device, period, now)` 化・`deviceCrop(device)` 済み）。露点は HasData 分岐に1ブロック相乗りするだけ。

### 確定した設計判断（design.md と一対一）
| # | 論点 | 確定 |
|---|------|------|
| 1 | 発病記録テーブル | 採らない（環境側 risk 提示まで） |
| 2 | 結露判定 | スプレッド T−Td ≦ 2.0℃（葉面温度=気温近似・暫定値） |
| 3 | 葉面湿潤しきい値 | RH ≧ 90%／最小継続 1.0時間（暫定値） |
| 4 | 病害スコア | 温度帯×葉面湿潤の最小合成（下地・0..1） |
| 5 | Crop 病害属性 | `DiseaseModel` 型＋`(c Crop) DiseaseModel()`・既定フォールバック |
| 6 | 結露帯 markArea | 専用 injector 新設（P5 gapZone 非改変・寒色） |
| 7 | 露点 y 軸 | 共通℃・auto 範囲 |
| 8 | RH=0 対策 | rhFloor=1.0% へ床上げ（系列に残す） |
| 9 | connect | 自動参加（EChartsInitializer 改修なし） |
| 10 | 物理規約の向き | 湿り側=寒色を一貫・実機スモーク目視確認を tasks 必須化 |
| 11 | P4 結露時間列 | 算出は本フェーズ `dewpoint.go`・列追加（消費）は P4 後追い |

### 残リスク（実装で監視）
- **物理規約の向き（最重要）**: テストが仕様前提を符号化するため自動テストでは捕捉不可。GO 後の実機スモークで色・符号・「結露/乾燥」ラベルの向きを目視確認（P3 VPD の前例＝メモ project_vpd_physics_convention）。
- 暫定しきい値（結露 2.0℃・RH 90%・温度帯 15-25℃）はユーザー/文献で確定可。構造は確定済みゆえ値更新は `DiseaseModel` 表＋test の1点更新で済む（着手ブロッカーにしない＝提案型案件方針）。
