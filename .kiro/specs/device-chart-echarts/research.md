# Gap Analysis — device-chart-echarts（自作SVG → go-echarts 移行）

> 既存実装（S5 device-detail）のグラフ描画基盤を go-echarts v2.7.2（内部 ECharts 5.4.3）へ置換する移行スペックの gap 分析。要件は requirements.md、現状コードと go-echarts API 実ソース調査に基づく。
> 作成: 2026-06-26 / フェーズ: validate-gap（design の技術根拠用）

## 1. 現状資産の棚卸し（移行元）

### データフロー（現状）
```
Show / Chart (handler/device_show.go)
  └ buildChartArea(ctx, id, period, now)
       └ Repo.ListRecentSensorReadings(deviceID, periodSince(period))   ← 不変（データ源）
       └ rawSeries(rows, rawLabelFor(period)) → []chart.Series ×2(温/湿)  ← 再利用可
       └ chart.LineChartSVG("温度","℃", tempSeries) → SVG文字列            ← 撤去対象
       └ chart.LineChartHoverPoints + hoverJSON → ホバー点列JSON ×2        ← 撤去対象
  └ DeviceChartAreaView{ TemperatureSVG, HumiditySVG, TemperatureHoverJSON, HumidityHoverJSON }
  └ DeviceChartArea.templ : @templ.Raw(SVG) + x-data="linkedCharts({...})"
  └ App.templ : function linkedCharts(cfg){...}（インラインAlpine・約50行）
```

### 資産インベントリと移行後の扱い

| 資産 | 役割 | 移行後 |
|---|---|---|
| `internal/chart/svg.go`（約318行・LineChartSVG ほか） | SVG文字列生成 | **撤去** |
| `internal/chart/hover.go`（LineChartHoverPoints/HoverPoint/xAtIndex） | ホバー点列計算 | **撤去** |
| `internal/chart/svg_test.go`（256行）・`hover_test.go`（60行） | 上記のテスト | **撤去**（option構築テストへ置換） |
| `chart.Point` / `chart.Series` | 点列データ表現 | **再利用 or `opts.LineData` 直結に置換**（設計判断） |
| `handler/device_show.go` の `buildChartArea` / `rawSeries` / `hoverJSON` | グラフ構築・写像 | **改修**（rawSeries相当は再利用、SVG/hover呼出は go-echarts 構築へ） |
| `handler/device_show.go` の `aggregateToFloat` | 集計値のfloat変換 | **保持（撤去不可）**。device_show.go に定義されるが `readings.go`(S6履歴の集計整形, line 296) が流用中。グラフ移行とは独立 |
| `component/views.go` `DeviceChartAreaView`（4文字列フィールド） | ViewModel | **フィールド変更**（SVG/HoverJSON廃止→ECharts断片/option保持型へ） |
| `component/views.go` `hoverData()` ヘルパ | x-data埋込フォールバック | **撤去** |
| `component/views.go` `chartPeriods` | 期間ボタン定義（24h/3d/7d/30d） | **再利用**（器は不変） |
| `component/DeviceChartArea.templ` | グラフ領域フラグメント | **書換**（@templ.Raw+x-data → EChartsコンテナdiv+初期化） |
| `layout/App.templ` `linkedCharts()`（約50行） | Alpineホバー連動 | **撤去** |
| `layout/App.templ` `<head>` | CDN: tom-select/alpine/htmx | **echarts.min.js を1行追加（self-host）** |
| `view/static.go` `//go:embed all:public` | 静的配信 | **配線は不変**（`public/js/echarts.min.js` を追加配置するのみ） |
| `mocks/html/device-show.html`・`style.css` | 器（period-btn/chart-wrapper/カード）＋ホバーオーバーレイ | **器は維持**。ホバーオーバーレイ markup は要判断（後述7-G） |
| `handler/device_show_test.go`（678行） | Show/Chart回帰 | **一部改修**（SVG内容アサーションは器/option へ。ルーティング/認可/period/空データ系は維持） |

### 現状の器（モック準拠・維持対象）
- `.period-selector` ＋ `.period-btn`（active）— 期間切替ボタン（mock 62-65 行、CSS 474-487 行）。
- `.linked-charts` > `.chart-wrapper`（温度グラフ/湿度グラフの見出し `<h2>` ＋ カード）。
- モックは `.chart-placeholder`（縞模様プレースホルダ div, height:300px）でグラフ本体を**描かない**＝グラフ描画はモック反映対象外（feedback_mock_graph_rendering_exception）。
- 線色トークン: 温度 `#e8590c` / 湿度 `#1971c2`（svg.go 定数・mock のツールチップ色とも一致）。

## 2. 要件 → 資産マップ（gap タグ: ✅再利用可 / 🔶改修 / 🆕新規 / ❓Research Needed）

| Req | 必要能力 | 対応資産 / gap |
|---|---|---|
| R1 初期表示・期間切替の無回帰 | Show/Chart ハンドラ、period検証（不正→400）、innerHTML swap、push-url、active往復 | ✅ ルーティング/検証/swap機構は現状維持。🔶 フラグメント中身（SVG→ECharts）差替のみ |
| R2 最高/最低・現在値・配色・X軸 | markPoint(max/min)、endLabel相当、線色、Xカテゴリ | 🔶 最高/最低=`opts.MarkPoints`で可。🆕 配色/Xは opts可。❓**右端現在値(endLabel)は opts に無く生注入 or MarkPoint座標で代替** |
| R3 十字ホバー・温湿度連動 | tooltip(axis)+axisPointer(cross)、2グラフ連動 | 🔶 cross は opts可（自作Alpine撤去）。❓**連動 `echarts.connect()` は go-echarts に無く生JS必須** |
| R4 空データ表示 | 0件時メッセージ | 🔶 現状は chart 側で空SVG。ECharts では graphic/title かサーバ側分岐で「データはまだありません」表示（設計判断） |
| R5 self-host・単回読込・再DL無し | echarts.min.js 配信 | 🆕 `public/js/echarts.min.js` 配置＋`<head>`1回読込。✅ embed/StaticFS/CSSURL方式踏襲。❓**App.templ は全認証画面共通＝全画面で読まれる**（後述7-F） |
| R6 期間切替の繰返し健全性 | init/dispose、リスナー非蓄積 | 🆕 **dispose を手動実装**（go-echarts は面倒見ない）。swap時の旧インスタンス破棄＋再init |
| R7 応答性・転送量縮小 | 生値のみ送出、O(log N)ホバー、間引き | 🔶 ECharts標準で二分探索ホバー。✅ペイロードは option JSON のみで縮小。❓**lttb sampling は opts に無く生注入**／サーバ間引きの要否 |
| R8 JS必須化 | — | 観測可能な境界変更（実装は ECharts導入で自動的に満たす）。no-JS=非表示を許容（既定） |
| R9 器のモック準拠 | period-btn/カード/配色 | ✅ 器は維持。❓ホバーオーバーレイ markup の去就（7-G） |

## 3. go-echarts v2.7.2 API 適合性（実ソース調査の要点）

- **フラグメント出力**: `(*charts.Line).RenderSnippet()` → `render.ChartSnippet{Element, Script, Option}`。`Validate()` は内部で自動実行。テンプレ無しで Element/Script/Option を個別取得可。templ へは **`template.HTML(...)` ラップ必須**。
- **Script の中身**: `<script>let goecharts_<ChartID> = echarts.init(getElementById('<ChartID>'),...); let option_<id>={...}; goecharts_<id>.setOption(option_<id>);</script>` ＝ **init+setOption を内包**。`Option` フィールド単体で option JSON だけも取得可。
- **ChartID 固定**: `opts.Initialization.ChartID` を明示指定可（`charts.WithInitializationOpts`）。div id・JS変数名 `goecharts_<id>` が確定 → connect/dispose 制御に有利。
- **self-host**: `opts.Initialization.AssetsHost`（既定 `https://go-echarts.github.io/...`）は**フルHTML出力時のみ有効**。**Snippet には `<script src>` が出ない**ため、self-host は **App.templ `<head>` に `echarts.min.js` を手動1回**で担保する（AssetsHost 指定は本件では実質無関係）。
- **opts で出せる**: 折れ線(`AddSeries`+`opts.LineData`)、`WithLineChartOpts(Smooth/ShowSymbol)`、`MarkPoints`(Type "max"/"min")、`WithTooltipOpts(Trigger:"axis", AxisPointer:{Type:"cross"})`、`WithDataZoomOpts`、`WithLineStyleOpts(Color)`、`SetXAxis`。
- **opts に無い＝生注入 or 回避が必要**:
  - **endLabel（右端現在値）**: `opts.LineChart`/series に endLabel 無し → 生 option 注入（`series[i].endLabel`）か MarkPoint 座標指定で代替。
  - **sampling "lttb"**: `opts.LineChart` に Sampling 無し → 生注入（`series[i].sampling:'lttb'`）。
  - **echarts.connect（2グラフ連動）**: go-echarts に group/connect API 無し → 描画後に生JS `echarts.connect([goecharts_temp, goecharts_hum])` を実行。
  - 関数値（tooltip formatter 等）は `opts.FuncOpts("function(p){...}")`（`__f__` マーカー）で出せる。
- **HTMX 落とし穴（最重要）**: `hx-swap="innerHTML"` で**挿入された `<script>` は自動実行されない**。RenderSnippet の Script をそのまま入れても動かない → (a) `htmx:afterSettle` で eval、(b) Element のみ＋option を `data-*`/JSON で渡しクライアント init、のいずれかが必要。
- **dispose**: go-echarts は dispose を生成しない。再swap時 `let goecharts_<id>` 再宣言衝突＋メモリリーク回避のため **`echarts.getInstanceByDom(el)?.dispose()` を手動**で。

## 4. 実装アプローチ Options（spec の 方式A/B に対応）

### Option A: サーバで RenderSnippet（Element + init Script を生成）
go-echarts の Element/Script をそのままフラグメントに埋め、HTMX swap 後に Script を評価。
- ✅ サーバ完結度が高い・追加 JS 最小（`line.RenderSnippet()` だけ）。SSR集約方針(product.md)と親和。
- ❌ HTMX innerHTML が `<script>` を実行しない問題に afterSettle eval が必須。
- ❌ 再swap時の `let goecharts_<id>` 再宣言・dispose 未対応・connect は別途生JS。ChartID固定が前提。
- ❌ endLabel/sampling の生注入は option 文字列後加工が必要（go-echarts の opts では届かない）。

### Option B: サーバは option JSON のみ生成、クライアントで init/dispose/connect
`RenderSnippet().Option`（or `JSONNotEscaped()`）で option JSON だけ出し、固定 div に薄いクライアントラッパで `dispose()→init()→setOption()→connect()`。
- ✅ **HTMX 親和が最良**: init/dispose/connect を一箇所に集約、再swapに強い。
- ✅ endLabel/sampling/連動を JSON 後加工 or クライアント JS で素直に補完できる（opts の欠落を回避）。
- ✅ ペイロードは option JSON のみで最小（R7）。
- ❌ クライアント JS（init/dispose/connect ラッパ）を新規に書く必要＝SSR集約度は下がる。

### Option C: ハイブリッド（推奨候補）
サーバ＝go-echarts で **option JSON を構築**（series/markPoint/tooltip/axisPointer/線色/Xカテゴリは opts で、endLabel/sampling は生注入）。クライアント＝**汎用の薄い初期化スクリプト**（App.templ に1つ、旧 linkedCharts の置き換え位置）で、フラグメント内の `data-echarts-option` を読み `dispose→init→setOption`、温湿度2インスタンスを `echarts.connect`。
- ✅ Option B の HTMX 親和性＋サーバ側 go-echarts による型安全な option 構築の両取り。
- ✅ init/dispose/connect の共通ロジックを画面非依存の1スクリプトに集約（保守容易）。
- ❌ 「option をどう DOM へ渡すか（script タグ/JSON/data属性）」「afterSettle vs MutationObserver」の取り決めが要る。

## 5. Effort / Risk

- **Effort: M（3–7日）**, 一部 M–L 寄り。理由: go.mod 依存追加＋約1MBアセット埋込＋**全画面共通 App.templ の改変**＋templ/ViewModel/handler 改修＋chart 2ファイル＋テスト2ファイル撤去＋device_show_test の SVG系アサーション書換＋クライアント init/dispose/connect JS 新規。データ源・ルーティング・認可・period検証は不変で範囲は限定的。
- **Risk: Medium**。既知技術（既存 HTMX/embed/StaticFS 流用）だが、未知点は **(1) HTMX swap での ECharts init/dispose ライフサイクル、(2) 2グラフ connect、(3) snippet script 実行方式、(4) endLabel/sampling の生注入**。いずれも回避策が判明済み（生JS/option後加工）でアーキ変更は無く、Low ではないが High でもない。

## 6. design フェーズへの Research Needed（持ち越し）

1. **採用方式の確定**: Option A / B / C（推奨）のいずれか。HTMX swap 後の初期化トリガ（`htmx:afterSettle` eval か、Bの data 属性＋クライアント init か）。
2. **option の DOM 受け渡し**: RenderSnippet の Script をそのまま使うか、Option JSON を `<script type="application/json">`/`data-*` で渡すか。`template.HTML` 化の置き場所。
3. **endLabel（右端現在値）**: 生 option 注入 か MarkPoint 末尾座標 か。go-echarts opts では不可。
4. **温湿度連動**: `echarts.connect([id_temp, id_hum])` の実行タイミング（両 init 後）と ChartID 固定方針（例 `echarts-temp-<deviceID>` / `echarts-hum-<deviceID>`）。
5. **dispose 作法**: swap 直前に旧インスタンス dispose。`htmx:beforeSwap`/`beforeCleanupElement` か、コンテナ id 固定＋`getInstanceByDom`。
6. **アセット読込スコープ**: App.templ `<head>` は**全認証画面共通**。echarts.min.js（≈330KB gzip）を全画面で読むか、device-show 限定の head-extras 機構を `AppLayoutData` に新設するか（現状 head 追加フィールド無し）。R5「1回読込・再DL無し」は両案とも満たすが、全画面読込はトレードオフ。
7. **コンテナの高さ（CSS/器）**: 現状 `.chart-wrapper > svg{width:100%;height:auto}` は SVG の内在アスペクト比依存。**ECharts のコンテナは明示 height が必要**（canvas/SVGレンダラとも 0 高さ回避）。`.chart-placeholder`(300px) 相当の高さ指定をどのクラスに与えるか（正本=mocks/html/style.css・グラフ本体は反映例外だが「枠の高さ」は器）。
8. **レンダラ**: canvas（既定・多点向き）か SVG レンダラか（30d 約8,640点・モバイル負荷で判断）。
9. **ダウンサンプリング**: サーバ間引き vs `dataZoom`/`sampling("lttb")`（生注入）。閾値と見た目維持の両立。
10. **2グラフ vs 統合**: 縦並び2チャート（connect）維持（既定）か 1チャート2y軸統合か。
11. **空データ表示**: ECharts の graphic/title か、サーバ側で点0時に従来の「データはまだありません」文言ブロックを返すか（後者が無回帰に素直）。
12. **ホバーオーバーレイ markup の去就（器/モック）**: モック device-show.html の `.chart-hover`(SVG)/`.chart-hover-tip`/`.chart-hover-time` は Alpine ホバーの器。ECharts では tooltip が内部描画するため不要化する。グラフ描画反映例外の範囲（ツールチップは動的描画＝反映対象外）として markup/CSS を撤去するか、モックに静的説明を残すか（feedback_mock_graph_rendering_exception の適用範囲を design で明示）。
13. **`chart.Point`/`chart.Series` の去就**: rawSeries の中間表現として残すか、`opts.LineData` へ直結して chart パッケージ自体を空にするか（依存方向: go-echarts(render/opts) を chart 最下流に置くか handler 側に置くか＝structure.md 準拠で確定）。
14. **テスト戦略**: option 構築（series/markPoint max-min/最高最低算出/線色/空データ）のユニットテスト、Show/Chart 回帰（period 400・認可404・空データ・フラグメント返却・active往復）の維持と SVG内容アサーションの置換。go-echarts 出力の検証粒度（Option JSON 文字列 contains か、構造体検証か）。

## 7. 補足メモ（実装時の落とし穴・確認済み事実）

- A. period 検証は現状 Show=任意`?period`(不正→400)・Chart=`required,oneof`(不正/未指定→400)。**無回帰で維持**（device_show_test に既存テストあり）。
- B. Chart フラグメントは**最新計測テーブルを返さない**（期間非連動）。維持。
- C. 認可は閲覧系=不在/非所有とも404（列挙防止）。グラフ移行で不変。
- D. `view/static.go` の embed 配線は無改修で `public/js/echarts.min.js` を配信可（`/static/js/echarts.min.js`）。バージョンクエリは `CSSURL()` 同様の方式を JS にも用意するか検討。
- E. echarts.min.js は go-echarts のアセットリポジトリ（go-echarts-assets, ECharts 5.4.3）から取得しコミット。約1MB（gzip 約330KB）。feedback_project_local_setup 準拠で repo 内完結。
- F. App.templ 改変は**全認証画面に影響**するため、linkedCharts 撤去とアセット追加の回帰確認は device-show 以外（dashboard 等）でも行う。
- G. CSRF: グラフ系は GET（CSRF不要）・静的アセットも GET。既存 CSRF 機構に影響なし。

---

# Design フェーズ追補（discovery-light + synthesis）2026-06-26

## Discovery: HTMX実装ガイド 該当節の確認と設計制約

- **Sources**: `2cc_sdd/HTMX実装ガイド(動的).md` §3.3（device-show id一覧, 1382-1396）/ §4 device-show（1558-1584）/ §10-D フルフラグメント swap（2474-）/ §10-E `<script type="application/json">`+`json.Marshal`（2560-）/ §1242 swap ライフサイクル管理（Tom Select destroy/init）/ §30 after-swap グローバルハンドラ / §31.1 写経の境界 / §40-B CSS 単一ソース。
- **得られた設計制約**:
  1. **id 語彙は確定済み**: `device-chart-area`（hx-target・期間切替で innerHTML swap・期間ボタン active をサーバ往復）、内包する `temperature-chart` / `humidity-chart`、期間非連動の `latest-readings-table`。本移行は id を変えない。
  2. **期間切替の HTMX 仕様は不変**: `<button type="button">` + `hx-get=/devices/{id}/chart?period=` + `hx-target=#device-chart-area` + `hx-swap=innerHTML` + `hx-push-url=/devices/{id}?period=`（フルページURL）。フラグメント内に期間セレクタを含めて active をサーバ往復（§10-D）。
  3. **swap 後 JS の正典パターン（最重要）**: 挿入された `<script>` は HTMX で自動実行されない／`hx-on::after-swap` で `$dispatch` 不可（§30）。**App.templ のグローバル `htmx:afterSwap` ハンドラ**で初期化、`htmx:beforeSwap` で破棄するのが確立パターン（§1242 の Tom Select destroy/init と同型）。
  4. **クライアントへのデータ供給は `<script type="application/json">` + `json.Marshal`（§10-E）**: `encoding/json` が `< > &` をエスケープし `</script>` 混入を防ぐ。属性直書き（旧 x-data）より安全・正典。
  5. **CSS/クラスは単一ソース（§31.1/§40-B）**: 器（`.period-selector`/`.period-btn`/`.chart-wrapper`/`.card`）はモック正本（mocks/html/style.css）準拠。独自クラス新設禁止。グラフ本体描画は反映例外（feedback_mock_graph_rendering_exception）。

## Architecture Pattern Evaluation（採用方式）

| Option | 説明 | 長所 | リスク/制約 | 採否 |
|--------|------|------|-------------|------|
| A: RenderSnippet（Element+Script を埋込） | go-echarts の init+setOption 入り `<script>` をフラグメントに埋め HTMX swap 後に評価 | サーバ完結・追加JS最小 | **挿入`<script>`非実行（§30）**・`let goecharts_<id>`再宣言衝突・dispose未対応・connect別途生JS | ✗ |
| B: option JSON のみ + クライアント init | サーバは option JSON、固定divにクライアントで dispose→init→setOption | HTMX親和最良・dispose/connect集約・endLabel/sampling補完容易 | クライアントJS新規 | △（基盤） |
| **C: go-echarts で option構築 → §10-E でJSON供給 → App.templグローバル初期化（採用）** | サーバ=go-echarts で typed に option 構築し HTML安全JSONを `<script type=application/json>` で供給。クライアント=App.templ のグローバル `htmx:afterSwap/beforeSwap` で dispose→init→setOption→connect（§1242/§30 転用） | 型安全な option 構築（テスト可）＋ HTMX親和＋既存 swap ライフサイクル流用＋ connect/endLabel/sampling をクライアント1箇所に集約 | クライアント初期化JSの新規実装（Goユニットテスト対象外） | ✓ |

## Design Decisions

### Decision: 描画方式は「サーバ go-echarts option 構築 × クライアント初期化（Option C）」
- **Selected**: handler が `internal/chart` の go-echarts ベース option ビルダを呼び、温度/湿度それぞれの ECharts option を HTML安全 JSON 文字列で得る → ViewModel に格納 → DeviceChartArea.templ が `#temperature-chart`/`#humidity-chart` コンテナ div ＋ 各 `<script type="application/json" id="...-option">` を出力。App.templ のグローバル初期化 script が DOMContentLoaded と `htmx:afterSwap` で各コンテナを `dispose()`→`echarts.init()`→`setOption()`、温湿度2インスタンスを `echarts.connect()`、`htmx:beforeSwap` で `dispose()`。
- **Rationale**: 挿入 `<script>` 非実行・`$dispatch` 不可（§30）を回避し、§1242 の destroy/init ライフサイクルをそのまま転用。go-echarts は HTMX 非親和な Render/Script を使わず **option 構築器としてのみ**採用（型安全＋テスト可、spec の go-echarts 移行意図も充足）。
- **Trade-offs**: クライアント初期化 JS は Go ユニットテスト対象外（手動/視覚検証）。その代わり option 構築はサーバで table-driven テスト可能。

### Decision: go-echarts opts の3つの欠落（endLabel / sampling / connect）の扱い
- **endLabel（右端現在値・R2.3）/ sampling("lttb")（R7）**: go-echarts opts に無いため、**クライアント初期化 script で setOption 前に series へ付与**（unit は container の `data-unit` から）。サーバ option は go-echarts が出せる範囲（series・markPoint max/min・tooltip axis+cross・線色・xAxis）に限定し純粋に保つ。代替（サーバで option JSON を unmarshal→キー追加→再marshal）は脆く不採用。
- **connect（温湿度連動・R3.3）**: go-echarts に group/connect API が無いため、両 init 後にクライアントで `echarts.connect([temp, hum])`。

### Decision: echarts.min.js は App.templ `<head>` で全認証画面共通読込（self-host）
- **Selected**: `internal/view/public/js/echarts.min.js`（ECharts 5.4.3・go-echarts-assets 由来）を `go:embed` 配信、`/static/js/echarts.min.js?v=<Version>` を App.templ `<head>` で1回読込。`view.JSURL()` を `CSSURL()` 同様に新設。
- **Rationale**: login/register は `Guest.templ` で App 非経由のため影響なし。認証画面（dashboard/readings/alerts）は未使用でも読むが、Tom Select も同様に全画面共通読込済みで方針一貫。`AppLayoutData` に画面別 head-extras 機構を新設するのは scope creep。初回以降はブラウザキャッシュ＋バージョンクエリで再DLされない（R5.3）。
- **Trade-offs**: device-show 以外でも ~330KB(gzip) 読込。許容（小規模内部ツール）。重すぎれば将来 spec で条件読込（本 spec の Out of Boundary）。

### Decision: ダウンサンプリングは ECharts `sampling:"lttb"`（クライアント）／サーバ間引き・クエリ変更はしない
- データ源 `ListRecentSensorReadings` は不変（boundary）。R7.2 のペイロード縮小は「SVG＋ホバーJSON二重送出 → option JSON（生値のみ）」で構造的に達成。描画負荷は lttb で低減。サーバ側間引きは不要と判断。

### Decision: 空データ（R4）はサーバ側分岐で従来同等メッセージ
- 0件時は DeviceChartArea がチャートコンテナ／option script を出さず「データはまだありません」ブロックを描画（現行の空SVGと同じ文言・無回帰）。クライアント初期化は option script 不在のコンテナを skip。

### Decision: internal/chart の責務と依存方向（structure.md 整合）
- go-echarts（render/opts）は外部描画ライブラリで、`internal/chart` が import してよい（gin/DB/templ/pgtype を import しない最下流ユーティリティの不変条件は維持）。`chart.Point`/`chart.Series` は入力型として存続（`series.go` へ移動）、option ビルダは `echarts.go`。`svg.go`/`hover.go`（LineChartSVG・HoverPoint 等）と各 test は撤去。pgtype→float（rawSeries）は handler の責務のまま。

## Risks & Mitigations
- **swap 時の init/dispose リーク（R6）** → §1242 と同型のグローバル `beforeSwap`(dispose)/`afterSwap`(init) ハンドラ。`echarts.getInstanceByDom(el)?.dispose()` を init 前に必ず実行。
- **connect の二重登録** → afterSwap 毎に新インスタンスで connect しなおす（古いインスタンスは dispose 済み）。
- **App.templ 改変の全画面波及（F）** → linkedCharts 撤去とアセット追加後、device-show 以外（dashboard 等）の回帰確認をテスト対象に含める。
- **`<script type=application/json>` のエスケープ** → §10-E に従い `encoding/json`（SetEscapeHTML=true）でシリアライズ。go-echarts の RenderSnippet().Option は html-unescape 済みのため、埋込用 JSON は encoding/json で再シリアライズして安全化。

## References
- go-echarts v2.7.2 実ソース（render/engine.go・templates/base.tpl・opts/*）— §3 API 適合性（本書 上部）
- `2cc_sdd/HTMX実装ガイド(動的).md` §3.3/§4/§10-D/§10-E/§1242/§30/§31.1/§40-B
- Apache ECharts 5.4.3: `echarts.init`/`getInstanceByDom`/`dispose`/`connect`、series `sampling:"lttb"`、`endLabel`、tooltip `trigger:"axis"` + axisPointer `type:"cross"`
