# Gap 分析: seasonal-trend（長期トレンド・季節サマリ／統計分析ページ）

> 目的: requirements（WHAT）と既存コードベースの差分を洗い出し、design フェーズの実装戦略を情報提供する。決定はしない（選択肢と根拠の提示に留める）。
> 調査日: 2026-06-29（P7 gdd-forecast マージ後の HEAD・実コード確認済み）

## 1. 要約（3-5点）

- **スコープ**: 既存画面（device-show 等）と別系統の独立「統計分析」ページ新設＋月次/年次ロールアップ＋自己相関補正つき Mann-Kendall/Sen＋厳密判定（Hamed-Rao/ブートCI/多重比較）。純粋統計層・描画方式B・認可・ナビは流用基盤が揃う。
- **最大の課題（最重要・Research Needed）**: **本リポジトリに非同期/goroutine/キャッシュ/バックグラウンドジョブ/HTMX ポーリングの既存実装が皆無**（DB ping の `context.WithTimeout` 1 箇所のみ）。Req5 の「厳密判定を非同期＋段階表示」はプロジェクト初の純新規パターン。**ただしデータ規模（月次 N≦数十〜数百）では検定計算が極めて軽く、非同期インフラ新設はオーバースペックの可能性が高い**。design で実測し「同期で足りるか／本当に非同期が要るか」を最初に判断すべき。
- **検定本体は全て新規**: 純粋層 `stats.go`/`quality.go` は流用良好（`LinearFit`・`median`・`quantile` が Sen の中央値/MK のソート分位に流用可）だが、MK 統計量 S・タイ補正 Var(S)・連続性補正 Z・Sen の傾き・λ̂/N_eff・Hamed-Rao・ブロックブートストラップ・FDR/Bonferroni は未実装＝新規 `internal/chart/trend.go`。p 値化に正規分布 CDF が要り、`gonum/stat/distuv` 導入が選択肢。
- **gonum の実態（spec-init 本文の訂正）**: gonum v0.16.0 は **go.sum（モジュールグラフ）に存在するが go.mod の require には不在**＝現在の実ビルドに未コンパイル。direct 昇格＝実 import するとビルドへ新規にコンパイル取り込み（バージョンは確定済みなので解決コストはゼロだが「追加コストほぼゼロ」は不正確）。自前純Go 実装で正規 CDF を持てば gonum 完全回避も選択肢。
- **推奨方針**: 描画（TrendChartSpec 別型＋小文字キー自前注入）・ロールアップ（date_trunc 新クエリ・SELECT のみ・DDL なし）・認可（RequireDeviceOwner 流用）は確立パターンの踏襲で低リスク。リスクは「非同期判断」と「検定の数値正確性」に集中。後者は外部参照実装の golden test で抑える。

## 2. Requirement → 資産マップ（gap タグ: ✅流用可 / ⚠️一部新規 / ❌新規 / 🔍Research）

| Req | 必要能力 | 既存資産 | gap |
|---|---|---|---|
| **R1 ページ新設/ナビ/セレクタ** | 新ルート・新 templ ページ・左メニュー項目・集計軸/期間セレクタ・モック | `cmd/server/main.go`(web グループ+`RequireAuth()`)・`sidebar.go`(NavPage const)・`Sidebar.templ`(DeviceID>0 文脈リンク)・Tom Select セレクタ作法(alert-rules)・モック単一ソース運用 | ⚠️ ルート/templ/モックは新規だが作法は確立。🔍 **ナビ構造（トップ階層 vs デバイス文脈）と粒度は design 最初の論点（未確定①）** |
| **R2 ロールアップサマリ** | 日/週/月/年集計・JST 境界・空月欠測・ΔT 推移 | `ListDailySensorAggregates`(DATE()基準・avg/max/min/count・device_id+下限)・handler の JST 変換(`time.FixedZone`)・`DiurnalRange`/`Mean`/`StdDev`/`CV` | ⚠️ 月次/年次は `date_trunc` 新クエリ（SELECT のみ）or 日次→Go 二段集計（未確定②）。🔍 **JST 月/年境界**: 既存 `DATE()` は TZ 非明示→`date_trunc('month', recorded_at AT TIME ZONE 'Asia/Tokyo')` 等の確定が要る |
| **R3 MK＋Sen＋自己相関補正** | MK(S/Var(S)タイ補正/Z)・Sen 傾き・λ̂/N_eff・Seasonal MK/prewhitening・p 値化 | `stats.go`(`LinearFit`/`Mean`/`StdDev`・math のみ・time 非依存)・`quality.go`(`median` 線形補間/`quantile`) | ❌ **検定本体すべて新規** `internal/chart/trend.go`。`median`/`quantile` は Sen 中央値/順位に流用可。🔍 p 値化＝gonum/stat/distuv 導入 or 自前正規 CDF（選択肢）。🔍 自己相関前置き方式（Seasonal MK / TFPW / STL）の確定（未確定⑥） |
| **R4 月別・季節別トレンド** | 月別/季節別に分けた MK/Sen・月別軸 | R3 の trend.go・集計軸セレクタ | ⚠️ R3 基盤の上に月別グルーピング。新規だが追加コスト小 |
| **R5 厳密判定/システム内完結/段階表示** | Hamed-Rao・ブロックブートCI・FDR/Bonferroni・非同期+キャッシュ・処理中表示・再現性 | **なし**（goroutine/cache/HTMX polling 皆無） | ❌ 検定本体新規＋🔍 **非同期/キャッシュ基盤が皆無＝最大リスク**。🔍 **実測で同期可否を先に判定**（月次 N 小ならブートストラップも同期で足りる公算大）。乱数 seed 固定で再現性。外部プロセス非依存は Go 完結で自然に満たす |
| **R6 検出力の留保** | 「非有意≠トレンド無し」・N 不足時 Sen+記述統計・一次判定ラベル・信号色 | バッジ/注記は templ+CSS 新規（モック反映） | ⚠️ ロジック単純（N しきい値で表示分岐）。🔍 「検定」表示の最低年数しきい値・有意水準（未確定⑦） |
| **R7 平年値比較** | 今季 vs 平年・不確実性注記・年数不足時の外部基準/非表示 | ロールアップ集計（平年=複数年平均） | ⚠️ 自社データ平年は集計の再利用。🔍 外部基準（気象庁長期平年値）の要否・取得（未確定⑤）。永続テーブルは既定スコープ外 |
| **R8 無回帰維持** | device-show/dashboard/readings/P2-P7 不変 | 既存全テスト（chart 層 10 本・handler/view テスト） | ✅ 別型隔離（TrendChartSpec）・別 handler・別 templ で消費のみ。既存テスト緑維持で担保 |
| **R9 所有者認可** | 自己所有のみ・非所有/不在は 404 相当・未認証は認証要求 | `authz.RequireDeviceOwner(ctx,q,deviceID,userID)`・sentinel(`ErrNotOwner`/`ErrUnauthenticated`/`pgx.ErrNoRows`)・閲覧系は 404（列挙防止）の確立方針・`renderDeviceReadError` 実例 | ✅ 流用のみ。新 handler で同パターン適用 |

## 3. 実装アプローチ選択肢

### 描画・ハンドラ・ロールアップ（R1/R2/R3/R4）→ Option B（新規コンポーネント）推奨

- **根拠**: device-show は period 連動パネル（温湿度/VPD/露点）と非連動（GDD）を `buildXxxPanel` チェーンで組む既存構造。長期トレンドは時間軸・集計軸が根本的に異なり（横断⑬）、device-show handler への上載せは肥大化を招く。`series.go` は既に 4 ChartSpec を別型隔離しており、`TrendChartSpec` 追加＝確立パターンの踏襲。
- **構成**: 新 `internal/handler/seasonal_trend_handler.go`／新 `internal/chart/trend.go`（純粋検定層）＋`trend_echarts.go`（`TrendChartOptionJSON(TrendChartSpec)→(string,error)`・Sen 線 markLine／CI 帯 stacked area／有意区間 markArea を小文字キー自前注入）／新 `internal/view/page/SeasonalTrend.templ`＋component／新モック `mocks/html/`。
- **トレードオフ**: ✅ 既存無回帰を構造的に保証・分離テスト容易 ✅ P9/P10/P13 が乗る器になる ❌ ファイル数増（許容＝structure.md「多数の小ファイル」と整合）。

### 厳密判定の非同期化（R5）→ Option C（段階的・実測駆動）推奨

- **段階1（初版・同期）**: 月次/年次ロールアップで N が小さい（数年×月次=N≦数十、最大でも数百）。MK/Sen は O(N²) でも実時間は無視可能。ブロックブートストラップ（B×O(N²)、例 B=1000-10000）も N=60 なら ~数千万 sign 演算＝数十 ms 級。**まず同期実装で実測し、ページロード許容内なら非同期基盤を作らない**（YAGNI・既存に非同期パターンが無い＝新設は保守コスト大）。
- **段階2（必要時のみ・非同期）**: 実測で許容を超える場合のみ goroutine＋インメモリキャッシュ（`sync.Map`、キー=device×期間×指標）＋ HTMX ポーリング（`hx-trigger="load delay:Xs"` or `every Ns`・index 既存例なし＝新規）。乱数 seed 固定で再現性・結果キャッシュ。
- **トレードオフ**: ✅ 過剰インフラ新設を回避・初版を早く出せる ✅ 「外部プロセス非依存」は Go 完結で常に満たす ❌ 段階2 へ移る場合 HTMX ポーリング/キャッシュが純新規（要設計研究）。🔍 **design で「同期前提の初版＋実測ベンチ」を必須タスク化**。

### gonum 依存（R3/R5 の p 値化）→ 2 択（design 決定）

- **Option α: gonum/stat・distuv を direct 昇格**: 正規 CDF/Quantile・記述統計・分位を借りる。go.sum に v0.16.0 既存＝バージョン解決コストなし。純Go・CGO 不要・BSD-3（CLAUDE.md「プロジェクトローカル完結」と整合）。ただし実ビルドへ新規コンパイル取り込み（依存表面が増える）。検定本体は依然自前。
- **Option β: gonum 完全回避（自前正規 CDF）**: 両側 p 値に必要な標準正規 CDF は `math.Erfc` で数行実装可。MK/Sen/補正は元々自前。**新規 direct 依存ゼロ**を維持（P1-P7 の「統計ライブラリ依存ゼロ」路線の継続）。分位は既存 `quantile` 流用。
- **判断材料**: gonum は分位・記述統計・将来 P9 相関でも有用だが、本フェーズだけなら β（自前 CDF）で依存を増やさない選択も合理的。🔍 design で「将来 P9/P10/P15 を含めた依存方針」として決める。

## 4. 工数・リスク（S/M/L/XL・High/Medium/Low）

| 領域 | 工数 | リスク | 根拠 |
|---|---|---|---|
| ページ器/ナビ/セレクタ/モック (R1) | **M** | Low | 確立パターン（Tom Select セレクタ・モック単一ソース・RequireAuth）。粒度決定が要 |
| 月次/年次ロールアップ (R2) | **S-M** | Low | 既存集計クエリの粒度替え（SELECT のみ・DDL なし）。JST 境界の確定のみ注意 |
| MK/Sen/自己相関補正の純粋層 (R3) | **L** | **Medium** | 検定本体全新規。タイ補正 Var(S)・Hamed-Rao 有効標本式・Sen 中央値は誤りやすい→**外部参照実装 golden test 必須**（VPD 符号反転の教訓）。`median`/`quantile` 流用で土台あり |
| 月別・季節別 (R4) | **S** | Low | R3 の上に薄く乗る |
| 厳密判定＋（必要なら）非同期 (R5) | **M（同期）/ L-XL（非同期化する場合）** | **High** | 非同期/キャッシュ/HTMX polling が皆無＝新設は High。**実測で同期回避できれば Medium に低下**。段階実装で初版リスクを限定 |
| 検出力留保・平年比 (R6/R7) | **S-M** | Low-Medium | 表示ロジック単純。平年の外部基準採否のみ Research |
| 描画 trend_echarts (R3-R7 可視化) | **M** | Low | markLine/markArea/stacked area 自前注入は確立（gdd/vpd/gap で実証）。TrendChartSpec 別型 |
| 無回帰・認可 (R8/R9) | **S** | Low | 流用のみ。既存テスト緑維持 |

**総合**: 全体 **L（1-2週）**、リスクは R5 非同期判断と R3 数値正確性の 2 点に集中。

## 5. design フェーズへの申し送り

### 推奨アプローチ
- **Option B（新規系統）＋ R5 のみ Option C（実測駆動の段階化）**。描画/ロールアップ/認可は既存パターン踏襲で低リスク化。

### 鍵となる決定（design で確定すべき）
1. **ナビ構造・粒度・ルート設計（未確定①・最初の論点）**: トップ階層の独立ページ（例 `/analysis/trend`）か、当面はデバイス文脈（`/devices/:device/seasonal-trend`・Sidebar の DeviceID>0 リンクに `NavSeasonalTrend` 追加）か。P9/P10/P13 が乗る器を崩さないこと。spec-init は前者寄りだが、初版「単一デバイス長期トレンド」は後者で早く出せる。
2. **非同期の要否（R5・最重要）**: **まず同期実装＋ベンチ**を必須タスクにし、実測許容内なら非同期基盤を作らない。超える場合のみ goroutine＋`sync.Map` キャッシュ＋HTMX ポーリング（純新規）。
3. **gonum 採否（Option α/β）**: distuv 導入 vs 自前 `math.Erfc` 正規 CDF。将来 P9 相関まで見据えた依存方針として決める。
4. **月次/年次集計方式（未確定②）**: `date_trunc(... AT TIME ZONE 'Asia/Tokyo')` 新クエリ（SELECT のみ）か日次→Go 二段集計か。JST 月/年境界を明示。**DDL なし（goose 最新 00010 のまま・`make db-snapshot` 不要）が既定**。
5. **自己相関前置き方式（未確定⑥）**: Seasonal MK（月別 S 合算）／prewhitening(TFPW)／STL のいずれか。λ̂ 推定（ラグ1/複数）。
6. **検出力留保の UI（未確定⑦）**: 「検定」表示の最低年数しきい値・有意水準・一次判定ラベル文言・信号色。
7. **平年値の基準（未確定⑤）**: 自社データ平年 vs 外部基準（気象庁長期平年値）。年数不足時は非表示。永続テーブルは既定スコープ外。
8. **CSV の扱い（未確定⑧）**: 統計分析結果 CSV を本フェーズに足すか P4 へ委ねるか（重複回避）。

### Research Needed（design で深掘り・本 gap では未決）
- **R5 ベンチマーク**: 実データ規模（数年×月次、年間日数系列）での MK/Sen/ブロックブートストラップの実測時間。これが非同期要否を分ける一次情報。
- **数値正確性 golden データ**: pyMannKendall / R modifiedmk の既知データセット（S・Z・p・Sen 傾き・Hamed-Rao 補正後 p）をテスト fixture 化する具体値の選定。chart 層は `*_test.go` が各モジュールに揃っており配置先は確立。
- **方式B の templ 組込み**: `@templ.Raw(optionScript(...))` で option JSON を inline 埋込・`[data-echarts]`＋兄弟 `-option` script を EChartsInitializer が走査する既存作法に TrendChartSpec を載せる詳細（複数チャート＝サマリ/トレンド/CI のコンテナ id 設計）。
- **HTMX ポーリング（段階2 採用時のみ）**: `hx-trigger="load delay"` / `every` の本プロジェクト初導入。HTMX実装ガイド §4/§14（ネットワークエラー/タイムアウト）参照が要。

### 既存パターン流用の確証（低リスク根拠）
- 純粋層: `internal/chart/stats.go`（math のみ・time 非依存）＋`quality.go` の `median`/`quantile`。
- 描画: 4 ChartSpec 別型隔離＋小文字キー markLine/markArea 自前注入（gdd/vpd/gap で実証）。
- 認可: `authz.RequireDeviceOwner` 一元化＋sentinel→HTTP マップ（閲覧系 404・列挙防止）。
- ナビ: `NavPage` const＋`Sidebar.templ` 条件リンク。ルートは web グループ＋`RequireAuth()`。
- モック: `mocks/html/style.css` 単一ソース＋`make sync-css`。器は反映必須・グラフ動的描画は反映例外。

---

# Design Synthesis（design フェーズ・3レンズ）

> design.md 確定前に discovery 所見へ適用した一般化／Build-vs-Adopt／簡素化の結論。

## 1. Generalization（一般化）

- **トレンド検定の共通基盤化**: R3（MK+Sen）・R4（月別・季節別）・R5（Hamed-Rao/ブート/多重比較）・将来 P11/P12（年間日数のルートA トレンド）は「自己相関補正つき単調トレンド検定」という同一問題の変種。→ `internal/chart/trend.go` を `[]float64`/`[]int` 入出力の汎用 API（`MannKendall`/`SeasonalMannKendall`/`SensSlope`/`HamedRaoModifiedMK`/`BlockBootstrapSenCI`/多重比較）として設計し、月次/季節/年間日数のどの系列でも再利用可能にする。実装スコープは本フェーズ要求（温湿度月次）に限るが、**interface は P11/P12 が乗れる形**にする（Revalidation Triggers に明記）。
- **`/analysis/*` 名前空間**: R1 の統計分析ページは P9 相関・P10 多地点・P13 ベンチマークと「研究用分析画面系列」という同一カテゴリ。→ ルートを `/analysis/trend` とし、ページ内デバイス/集計軸セレクタを共通の器にする。device-context ネスト（`/devices/:device/...`）にしない方が後続を素直に追加できる。

## 2. Build vs. Adopt

- **p 値化（正規 CDF）= Build**: gonum/stat/distuv を Adopt する選択肢（α）もあるが、必要なのは標準正規 CDF（`math.Erfc` で数行）のみ。CI は経験ブートストラップで正規逆分位を回避。→ **Build（自前・gonum 非導入）**。P1〜P7 の依存ゼロ路線を継続し direct 依存を増やさない。将来 P9 相関で gonum の記述統計/相関が広く要るなら、その時点で再評価（gonum は go.sum に v0.16.0 既存ゆえ昇格は容易）。
- **検定本体（MK/Sen/Hamed-Rao/ブート/FDR）= Build**: gonum に無く、外部プロセス（R/Python）依存は R5.2（システム内完結）に反する。→ 自前。正確性リスクは **Adopt した参照値**（pyMannKendall/R modifiedmk の既知データ）を golden test の oracle にして抑える（テスト期待値は Adopt・実装は Build）。
- **描画・セレクタ・認可・ナビ = Adopt（既存パターン流用）**: 方式B option JSON・markLine/markArea 自前注入・Tom Select セレクタ＋HX-Request 分岐・`RequireDeviceOwner`・`NavPage`/Sidebar・CSS 単一ソース。新規発明なし。

## 3. Simplification（簡素化）

- **非同期/キャッシュ基盤を作らない（最大の簡素化）**: 既存に非同期は皆無。ロールアップ後 N が小さく同期で十分（Performance 参照）。→ goroutine/`sync.Map` キャッシュ/HTMX ポーリングを**初版から排除**し、規模超過時の拡張点としてのみ文書化。R5.3/5.4 は「同期が予算内なら条件非発火で充足」と整理。
- **平年値の永続テーブルを作らない**: 自社データの暦月平均を読み取り時計算（年≥3 のみ）。外部取得・新テーブルを排除。
- **CSV を足さない**: P4 と重複させない。本ページは表示に専念。
- **月次集計の新クエリは最小限**: 月次/年次専用クエリを増やさず、**JST 日次集計 1 本＋Go 二段集約**で週/月/年を賄う（ΔT 正確性も同時に満たす）。既存 `ListDailySensorAggregates` は無改変（無回帰）。
- **別型隔離で無回帰を構造保証**: `TrendChartSpec` を別型にし既存 ChartSpec 群・device-show に一切触れない。

## 結論

最小構成: 純粋層 `trend.go`（検定）＋`trend_echarts.go`（描画）＋handler 1 本＋templ/DTO＋JST 日次集計 SQL 1 本＋モック。新規 direct 依存ゼロ・DDL ゼロ・非同期基盤ゼロ。リスクは「数値正確性（golden test で抑制）」に一点集中させ、それ以外は確立パターンの流用で低リスク化した。
