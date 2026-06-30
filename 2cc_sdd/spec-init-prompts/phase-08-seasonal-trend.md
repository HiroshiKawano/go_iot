# フェーズ8（分析ロードマップ）spec-init プロンプト: 長期トレンド・季節サマリ（新規「統計分析」ページ／月次ロールアップ／自己相関補正つき Mann-Kendall ＋ Sen's slope ／Hamed-Rao・ブロックブートストラップCI・多重比較を Go で計算し go-echarts で表示／検出力の留保）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: seasonal-trend
> 位置づけ: [分析アイデアメモ.md](../分析アイデアメモ.md) 第1章「実装ロードマップ」の**フェーズ8**〔明示／強く示唆〕。**P1〜P7 と決定的に異なり、device-show（デバイス詳細）へのパネル上載せではなく、左サイドメニューに新設する独立した「統計分析」ページ**に置く（**横断⑬**＝2026-06-29 ユーザー決定）。device-show は単一デバイスの直近監視（24h〜30d のリアルタイム可視化）に特化し、**多年・多シーズン横断の長期トレンド・季節サマリは時間軸・集計軸（横断①）が根本的に違うため分離する**。本フェーズは **P9（相関）・P10（多地点）・P11（台風）・P13（ベンチマーク）・P15（予測）も受ける「統計分析」ページ系列の初号**（横断⑬＝数年に及ぶ or 複数センサーをまたがる分析）で、ナビ・ルート・所有者認可(BOLA)・集計軸セレクタ（デバイス/地点/作物/期間）の器を用意する。`nishizawa.md`（西澤誠也氏のトレンド検出手法）由来の**付録G（トレンド検定の統計的健全性・G-1〜G-11）が本フェーズの中核**。
> 確度: 〔明示／強く示唆〕（早見表 confidence「明示」。データ主権＝Ambient 脱却の本領で、研究者・査読品質の顧客像〔付録B-1〕に直結）。
> 前提セッション: **論理依存＝長期蓄積（横断⑦）・集計軸（横断①＝P1 locality／P3 crop）**。**実装は P1〜P7 マージ済みコードの「流用」が前提だが、新ページゆえ handler/templ 構造は device-show と別系統で起こす**:
>  - **P2（temp-humidity-chart-stats）** … `internal/chart/stats.go`＝`Mean`/`MinMax`/`StdDev`/`CV`/`DiurnalRange`・**`LinearFit(xs, ys)→(slope, intercept, ok)`**（P7 で新設・最小二乗・コメントに**「将来の trend 検出が再利用可」と明記**）。→ **トレンド統計の純粋層 `internal/chart/trend.go` を新設する直接の土台**（回帰トレンド線・Sen の中央値計算に `stats.go` の分位/`quality.go` の `IQRBounds`/分位関数を流用）。`dailyStatRows` の JST 暦日バケット作法＝**月次ロールアップの土台**。
>  - **P4（sensor-data-export）** … `db/queries/sensor_readings.sql` の `ListSensorReadingsInRange`〔BETWEEN・ORDER ASC・LIMIT なし＝期間内全行〕／日次集計 `ListDailySensorAggregates`〔`GROUP BY DATE(recorded_at)`・avg/max/min/count〕。→ **月次ロールアップ（日次→月次）SQL の新設 or 日次集計の Go 二段集計の土台**（いずれも SELECT のみ・DDL なし）。CSV 本体は P4 所有（本フェーズはデータ主権の CSV を P4 へ委ねるか最小併設するか design 論点）。
>  - **P5（data-quality-meta）** … `internal/chart/quality.go`〔`RollingOutliers`/`IQRBounds`/`ZScores`/`StuckRuns`/分位関数・**純粋・time 非依存**〕・`gap_echarts.go`〔`injectGapMarkArea` 小文字 `xAxis` 範囲注入・`nullableLineData` 欠測 nil〕。**器差・センサ交換（ギャップδ・Ng）は P5 が位置軸前提で P10 へ defer 済み**（requirements Req7「器差スコープ境界」）＝**本フェーズも器差の構造化はしない**（付録G-4。δ をトレンドへ織り込むのは将来 P10 と連携）。
>  - **P3/P6/P7（vpd/dewpoint/gdd）** … `vpd_echarts.go`/`gdd_echarts.go` の **markLine/markArea 自前注入パターン**（go-echarts の JSON タグ不具合を回避し option を JSON 化→マップへ戻し series へ小文字 ECharts 準拠キーで注入）＝**Sen's slope トレンド線 markLine・正常帯/有意区間 markArea・信頼区間帯（積み上げ area）の自前注入の手本**。`series.go` の `VPDChartSpec`/`GDDChartSpec`〔別型隔離〕＝**`TrendChartSpec` を別型で起こす作法**。`--color-vpd`/`--color-gdd`〔新色トークン〕。
>  - **P1（device-location-select）** … `internal/domain/locality.go`＝**地点を集計・比較可能なキー**（横断①の地点軸＝統計分析ページのデバイス/地点セレクタの土台）。**P3** `internal/domain/crop.go`＝作物軸。
>  - **S1（web-foundation-auth）** … RequireAuth・**所有者認可 `internal/authz`（`RequireDeviceOwner`）**・`go:embed` 静的配信（echarts.min.js）。**S3（dashboard）** … 左サイドメニュー/ナビの構造＝**「統計分析」メニュー項目を足す拡張点**。
>  - **E1（device-chart-echarts）** … `App.templ` の `EChartsInitializer`〔`[data-echarts]` 走査→init/dispose・`echarts.connect`〕＝**統計分析ページのグラフ初期化の流用元**（方式B＝サーバで option JSON 生成→クライアント描画）。
> **ライブラリ判断（最重要・このフェーズの核・P1〜P7 と決定的に異なる点その2）**: P1〜P7 は**統計ライブラリ依存ゼロ**（付録G-10 で実証）で通したが、本フェーズは **Mann-Kendall の p値化（正規近似 z→両側p値）に正規分布の CDF/Quantile が要る**ため、`gonum.org/v1/gonum/stat/distuv`（`Normal.CDF`/`Quantile`）と `gonum/stat`（記述統計・分位・相関）を**初の数値計算ライブラリとして導入する**。**gonum コア v0.16.0 は既に go.sum / module graph に間接依存で存在**（grpc 経由）するため、`go.mod` の **direct require へ昇格するだけ**で追加コストはほぼゼロ・純Go・CGO 不要・BSD-3-Clause（CLAUDE.md「プロジェクトローカル完結」と整合）。**検定本体（MK 統計量 S・Var(S) タイ補正・Z 連続性補正、Sen's slope、自己相関 λ̂・N_eff、Hamed-Rao 補正、ブロックブートストラップ、FDR/Bonferroni 多重比較）は gonum に無い＝自前の純Go関数**として `internal/chart/trend.go` に実装する（gonum は p値化・分位の土台のみ）。**描画は go-echarts 一本に統一し、ユーザー提案の `gonum/plot`（静的画像生成）は画面描画に採用しない**（横断⑭＝統計プロット〔信頼区間帯/ヒストグラム/箱ひげ/ACF/Q-Q/トレンド線/MKバッジ〕は全て ECharts 標準で描け、gonum/plot 追加は CSS 単一ソース運用・モック運用・templ/HTMX 動的性・テストの全層で二重化コスト大・UX 断裂を招く）。
> **スキーマ判断**: **原則スキーマ非変更**（goose 最新は **00010**＝P7 planting_date のまま・`make db-snapshot` 不要）。トレンド統計はすべて既存 `sensor_readings`（temperature/humidity＋recorded_at）から**月次ロールアップ後に読み取り時計算**で足り、新規 SQL は**月次集計の SELECT のみ**（DDL なし）。平年値も自社データから計算 or 外部基準（気象庁長期平年値）を借りる（永続テーブルは作らない＝既定）。
> **設計フェーズで参照**:
> - 上位ロードマップ: 分析アイデアメモ.md フェーズ8（**作るもの＝日次→週次→月次→年次ロールアップ統計サマリ／日較差ΔT 日次推移／トレンド検定セット〔自己相関補正つき MK＋Sen〕／月別・季節別トレンド〔年集約は月別の符号反転を潰す＝G-5 警告〕／平年値との比較〔不確実性留意・G-9〕／検出力の留保表示〔G-8〕。▶ ◎ 平年比・ロールアップ・MK＋Sen トレンド（アプリ内Go・markLine＋有意なら markArea）／○ 回帰トレンド線（探索用）／△ STL/ACF/Edgeworth/ブロックブートストラップ/SARIMA**）
> - **統計の権威（中核）**: 分析アイデアメモ.md **付録G（トレンド検定の統計的健全性）全節** ＝ G-1 自己相関の罠〔`Var(â) ∝ (1+λ)/(1-λ)`・`N_eff ≈ N(1-λ)/(1+λ)`・温湿度は λ≈0.9+ で素の回帰/MK は偽トレンドを量産〕／ G-2 Mann-Kendall〔`S=Σsign`・タイ補正 `Var(S)`・連続性補正 `Z`〕／ G-3 Sen's slope〔全ペア傾き中央値・MK と必ずペア〕／ G-4 ギャップδ〔器差は P10/P8 役務・本フェーズは織り込みの素地まで〕／ G-5 レアイベント二分法〔ルートA 年間日数→MK/Sen／ルートB 希少バイナリ S 統計量は P11/P12〕／ G-6 ブロックブートストラップ・Edgeworth／ G-7 多重比較（FDR/Bonferroni）／ G-8 検出力・N年数の留保〔「非有意≠トレンド無し」〕／ G-9 平年値の不確実性・基準期間依存／ **G-10 Go実装 vs CSV外の線引き〔MK/Sen/Hamed-Rao/ブロックブートストラップ/多重比較は全て Go アプリ内・p値化は gonum/stat/distuv〕／ G-11 システム内完結〔一次判定〔軽量 MK/Sen/N_eff〕＋厳密判定〔Hamed-Rao/ブロックブートCI/多重比較・非同期+キャッシュ〕をどちらもアプリ内表示・CSV はデータ主権用に併設〕**
> - 数式（権威）: 付録A —「A 基本統計量」（平均/σ/CV/日較差ΔT・ロールアップの素）／「B 時系列・移動平均系」（線形回帰トレンド・SMA・**トレンド検出**）。**MK/Sen/自己相関の正式な数式は付録G**（A-B の一行「線形回帰の傾き or MK」は付録G-11 で健全版へ書き換え済み）。
> - 現スキーマ（権威）: docs/database_snapshot/table_definitions.md「sensor_readings」（**temperature/humidity numeric(5,2)・recorded_at timestamptz〔計測〕・created_at〔受信〕**・(device_id, recorded_at DESC) 部分索引）／「devices」（locality VARCHAR(20)=P1/00008・crop VARCHAR(20)=P3/00009・planting_date DATE=P7/00010）。**goose 連番の最新は 00010**。**本フェーズは原則 DDL なし**（月次集計は SELECT のみ）。
> - 既存の流用元コード（実コード確認済み・P7 マージ後の現状）: `internal/chart/stats.go`〔`LinearFit`・`Mean`/`StdDev`/`CV`/`DiurnalRange`〕／`internal/chart/quality.go`〔`IQRBounds`/分位/`RollingOutliers`〕／`internal/chart/{vpd,gdd}_echarts.go`〔markLine/markArea 自前注入〕／`internal/chart/series.go`〔別型隔離〕／`db/queries/sensor_readings.sql`〔`ListSensorReadingsInRange`/`ListDailySensorAggregates`〕／`internal/handler/device_show*.go`〔buildPanel 作法・JST 境界〕／`internal/view/component/App.templ`〔`EChartsInitializer`〕／`internal/authz`〔所有者認可〕／`go.mod`〔gonum を direct 昇格〕。
> - ビュー/モック（単一ソース運用の境界）: **新規「統計分析」ページの templ（`internal/view/`）と新規モック HTML（`mocks/html/` に統計分析ページを新設）＋`style.css`（正本・`--color-trend` 等の新色トークン）**。**ページ器・集計軸セレクタ・MK/Sen 判定バッジ枠・検出力留保注記・サマリ表は静的な器＝モック反映必須**（feedback_mock_reflects_impl_visual・project_css_single_source）。**グラフ内部のトレンド線/CI 帯/markLine/markArea 描画は動的描画ゆえモック反映の例外**（feedback_mock_graph_rendering_exception）。左サイドメニューへの「統計分析」項目追加も器＝モック反映。
> - 命名・依存規約: .kiro/steering/structure.md（依存方向＝下向き一方向・`internal/chart` 最下流純粋層〔gonum/stat・distuv も math 同様の純粋計算ゆえ chart 層から import 可・time 非依存〕・view→domain 表示メソッドのみ・所有者認可は `internal/authz` 集約・§100 マスタは Go 定数）／ tech.md（sqlc・データアクセス方針）／ CLAUDE.md（DDL を足すなら expand-contract＋`make db-snapshot`。**本フェーズは原則 DDL なし**）。
> - 数値正確性の規約（最重要・教訓）: **MK 分散のタイ補正・Hamed-Rao の有効標本サイズ式・Sen の中央値は実装を誤りやすい**。`project_vpd_physics_convention` の教訓（VPD 符号反転がテスト全緑のまま実機まで残った）と同様、**ドメイン/数式の取り違えはテストが仕様前提を符号化すると捕捉できない**。よって **pyMannKendall / R modifiedmk の既知データセットを golden test の期待値**にして数値照合する（TDD・外部参照実装との一致が安全網）。

--- spec-init 本文 ここから ---

## 機能概要

蓄積した温湿度計測データから**長期トレンドと季節サマリ**を、研究者が**多年・多シーズンを横断して論じられる**よう解析・可視化する。**P1〜P7（device-show へのパネル上載せ）とは決定的に異なり、左サイドメニューに新設する独立した「統計分析」ページ**に置く（横断⑬）。device-show は単一デバイスの直近監視（24h〜30d）に特化し、**長期トレンドは時間軸・集計軸が根本的に違う**ため分離する。本ページは **P9 相関・P10 多地点・P11 台風・P13 ベンチマーク・P15 予測も受ける「統計分析」ページ系列の初号**となる（横断⑬の振り分け基準＝(a) 数年に及ぶ分析がメイン／(b) 複数センサーをまたがる。P12 はメイン1年以内ゆえデバイス詳細・P14 は新センサー前提で保留）。本フェーズの統計的中核は分析アイデアメモ **付録G**（トレンド検定の統計的健全性）であり、次を作る。

1. **ロールアップ統計サマリ** — 日次→週次→月次→年次へ集計した 平均/最高/最低/日較差ΔT/σ/CV のサマリと、日較差ΔT の日次推移。
2. **トレンド検定セット（◎ 主役）** — **自己相関補正つき Mann-Kendall（付録G-2・タイ補正 Var(S)・連続性補正 Z）＋ Sen's slope（付録G-3・傾きの頑健推定）**。**MK＝有無の検定／Sen＝大きさ（℃/年・%/年）の点推定**をペアで提示。素の MK は独立を仮定するため、**月次ロールアップ＋ Seasonal MK（月別）or prewhitening（TFPW）で自己相関を前置き処理**してから検定する（付録G-1＝温湿度は λ≈0.9+ で素の回帰/MK は p値を桁で過大評価し偽トレンドを量産する）。
3. **月別・季節別トレンド** — 年集約は月別の符号反転（増減が月で逆）を潰すため（付録G-5 の警告）、**月別/季節別に分けてトレンド**を取る。集計軸（横断①）に月別軸を確保。
4. **平年値との比較** — 今季 vs 平年。ただし**平年値の不確実性・基準期間依存に留意**し（付録G-9）、年数不足時は気象庁長期平年値を外部基準に借りるか平年比を出さない。
5. **厳密判定（システム内・段階表示）** — **Hamed-Rao 補正MK・ブロックブートストラップ信頼区間（area 帯）・多重比較補正（FDR/Bonferroni・付録G-7）も全て Go で計算**し go-echarts でアプリ内表示する（重い計算は非同期＋結果キャッシュ）。**外部アプリ（R/Python）には依存しない＝システム内完結**（付録G-11）。
6. **検出力の留保表示（必須）** — 数年蓄積では MK の検出力が低く「非有意」が出やすい（付録G-8）。**「非有意≠トレンド無し」を明示**し、N（年数）不足時は「検定」を断定せず **Sen 傾き＋符号の記述統計**に留める。バッジに「一次判定（多重比較未補正）」等のラベルを付し、素の自己相関補正なし回帰トレンド線は探索用に留める。

**設計の核その1＝新規「統計分析」ページ（P1〜P7 と決定的に異なる）**: P1〜P7 は device-show へのパネル上載せだったが、本フェーズは**左サイドメニュー新項目→新ルート→新 templ ページ**を起こす（横断⑬）。ナビ・ルート・**所有者認可(BOLA)**・**集計軸セレクタ（デバイス/地点/作物/期間）**の器を用意し、P9/P10/P11/P13/P15 がこの器に乗る前提で設計する。**device-show・dashboard・readings 等の既存画面は無回帰で維持**する。

**設計の核その2＝システム内完結（gonum/stat・distuv 導入・gonum/plot 不採用）**: トレンド検定の p値化に `gonum/stat/distuv`（正規分布 CDF/Quantile）を**初の数値計算ライブラリ**として導入する（gonum コアは既にビルドグラフ内＝`go.mod` の direct 昇格のみ・純Go・BSD-3）。**MK/Sen/Hamed-Rao/ブロックブートストラップ/多重比較は全て自前の純Go＋gonum で計算**でき、**go-echarts でアプリ内表示できる＝外部アプリ不要**（付録G-10/G-11）。**`gonum/plot`〔ユーザー提案の描画ライブラリ〕は画面描画に採用せず go-echarts 一本に統一**（横断⑭＝統計プロットは全て ECharts で描け、二系統化は二重化コスト大）。

> **「自己相関補正なしの素の MK/回帰は使わない」（最重要・誤用防止）**: 温湿度は隣接時刻・日周で自己相関が極めて強く（λ≈0.9+）、素の回帰/MK は分散を `(1+λ)/(1-λ)` 倍に過小評価して「有意な温暖化」を偽検出する（付録G-1）。**必ず ①日次/月次ロールアップで日周相関を弱め ②Seasonal MK or STL デシーズン化で季節成分を除き ③残差の λ̂ で N_eff 補正 or Hamed-Rao 補正MK を施す**。客は研究者・査読品質（付録B-1）ゆえ、この前置きなしのトレンド主張は成立しない。

> **「検出力の留保」（重要・誤読防止）**: 本案件は蓄積が数年で N（年数）が小さく、MK は検出力が低い（付録G-8）。「有意でない＝トレンドが無い」ではない。当面は **Sen 傾き＋符号＋記述統計（平年比・年次推移）**を主に見せ、「検定」の有意性断定は長期蓄積後に限定する旨を UI/仕様に明記する。

## 背景・現状

P7（gdd-forecast）マージ後の現状は以下（実コード確認済み）。

- **統計純粋層と回帰**: `internal/chart/stats.go` が `Mean`/`MinMax`/`StdDev`/`CV`/`DiurnalRange` と **`LinearFit(xs, ys)→(slope, intercept, ok)`**（P7 で新設・最小二乗・**コメントに「将来の trend 検出が再利用可」と明記**）を `math` のみ依存・time 非依存で提供。`internal/chart/quality.go` に `IQRBounds`/分位関数（Sen の中央値計算に再利用可）。**MK/Sen/自己相関補正/Hamed-Rao/ブロックブートストラップ/多重比較はまだ無い**＝新設 `internal/chart/trend.go` に実装する。
- **依存ゼロの現状とライブラリ判断**: `go.mod` は統計ライブラリを direct require していないが、**gonum コア v0.16.0 は go.sum / module graph に間接依存で既存**（grpc 経由）。本フェーズは p値化に `gonum/stat/distuv` を **direct 昇格**して使う（純Go・CGO 不要・BSD-3）。これが本リポジトリで初の数値計算ライブラリ直接利用になる。
- **ロールアップの土台**: `db/queries/sensor_readings.sql` の `ListDailySensorAggregates`（`GROUP BY DATE(recorded_at)`・avg/max/min/count）が日次集計を確立済み。**月次/年次ロールアップは未実装**＝月次集計 SQL の新設 or 日次→月次の Go 二段集計が要る（いずれも SELECT のみ・DDL なし）。`dailyStatRows`（P2）の JST 暦日バケット作法が土台。
- **markArea/markLine 自前注入と別型隔離**: `vpd_echarts.go`/`gdd_echarts.go` が go-echarts の JSON タグ不具合を回避して series へ小文字 ECharts キーで markLine/markArea を自前注入する確立パターン。`series.go` の `VPDChartSpec`/`GDDChartSpec` が別型隔離の手本。**Sen トレンド線 markLine・有意区間/正常帯 markArea・信頼区間帯（積み上げ area）はこの方式の流用で描ける**。
- **可視化基盤（方式B）**: E1 で go-echarts へ移行済み。`App.templ` の `EChartsInitializer` が `[data-echarts]` 走査→init/dispose・`echarts.connect`。統計分析ページもこの方式で option JSON をサーバ生成しクライアント描画する。
- **集計軸の素地**: `internal/domain/locality.go`（P1・地点）・`crop.go`（P3・作物）・`devices.planting_date`（P7）が集計・比較キーを提供。統計分析ページの**デバイス/地点/作物/期間セレクタ**の土台。
- **認可・ナビ**: `internal/authz`（`RequireDeviceOwner`・閲覧系の非所有/不在は 404 で列挙防止）。S3 dashboard の左サイドメニュー構造。
- **統計分析ページ・MK/Sen・月次ロールアップ・トレンドバッジは存在しない**（現状は dashboard / device-show / readings / alert-* の画面群）。device-show の長期トレンド系（STL/ACF/MK）はメモ上「CSV外」だったが、付録G-10/G-11 で「アプリ内 Go＋go-echarts」へ方針転換済み（本フェーズで実装）。

## このセッションのスコープ（実装対象）

### 1. 統計分析ページの器（横断⑬の初号・新ナビ/ルート/認可/セレクタ）

- 左サイドメニューに**「統計分析」項目**を追加（S3 ナビ構造の拡張・モック反映必須）。新ルート（例 `GET /analysis/trend` 等）と新 templ ページ。
- **集計軸セレクタ**（デバイス/地点/作物/期間〔月次・年次〕）。**所有者認可(BOLA)** を `internal/authz` で適用（自分の device/圃場のみ・非所有は 404）。
- **粒度は design の最初の論点**（単一デバイス選択で長期トレンド／地点・作物横断ダッシュボード）。本フェーズは少なくとも単一デバイスの長期トレンドを成立させ、横断は P9/P10/P11/P13/P15 へ拡張できる器にする（未確定事項①）。

### 2. 月次ロールアップ集計層

- 日次→**月次→年次**のロールアップ（平均/最高/最低/日較差ΔT/σ/CV/サンプル数）。**月次集計 SQL の新設**（`GROUP BY date_trunc('month', recorded_at)` 等・SELECT のみ）か `ListDailySensorAggregates`→Go 二段集計か（未確定事項②・いずれも DDL なし）。
- **O(N²) の MK/Sen は必ずロールアップ後に適用**（月次なら N≦数百で軽い・付録G-10）。生の分/秒粒度に直接かけない。

### 3. トレンド統計の純粋層（`internal/chart/trend.go`・time 非依存）

- **Mann-Kendall**（付録G-2）: `S=Σ_{n<m} sign(Xm−Xn)`／タイ補正 `Var(S)=(1/18)[N(N-1)(2N+5)−Σ ti(ti-1)(2ti+5)]`／連続性補正 `Z`。**p値化は `gonum/stat/distuv.Normal` の CDF**。
- **Sen's slope**（付録G-3）: 全ペア `(Xn−Xm)/(n−m)` の中央値（外れ値・台風スパイクに頑健）。
- **自己相関補正**（付録G-1）: ラグ1自己相関 `λ̂`・有効標本 `N_eff≈N(1-λ)/(1+λ)`・prewhitening（TFPW）・Seasonal MK（月別 S 合算）。
- **Hamed-Rao 補正MK**（付録G-1/G-10）: ランクのラグ自己相関で `Var(S)` を補正（pyMannKendall/R modifiedmk 準拠で移植）。
- **ブロックブートストラップCI**（付録G-6）: 連続ブロック単位の復元抽出で自己相関を保つ・B 反復で経験 CI。**重い（B×O(N²)）ので非同期＋キャッシュ前提**・乱数 seed 固定で再現性。
- **多重比較補正**（付録G-7）: FDR（Benjamini-Hochberg）/Bonferroni（数十行）。
- すべて `[]float64`/スカラ入出力・**time 非依存**（recorded_at の月次バケットは handler 境界）。`gonum/stat`（分位・記述統計）と `distuv`（p値化）を土台に、検定本体は自前。

### 4. 描画層（`internal/chart/trend_echarts.go`・go-echarts）

- **月次/年次ロールアップ折れ線**（主役）＋ **Sen's slope トレンド線 markLine**＋ **MK 判定の信号バッジ**（有意↑/有意↓/非有意・visualMap or カード）＋ **有意区間/正常帯 markArea**＋ **ブロックブートストラップ信頼区間帯（2系列積み上げ area）**＋ **平年比**。markLine/markArea/area は P3/P6/P7 の自前注入パターン流用。
- **別型 `TrendChartSpec`** で隔離（温湿度 `ChartSpec`・VPD・露点・GDD の無回帰を守る）。長期は **dataZoom** で閲覧。

### 5. handler（`internal/handler/seasonal_trend_handler.go`）

- ページ組立（セレクタ→月次ロールアップ取得→`trend.go` 呼び出し→`component.*View` 詰め）。time/JST は handler 境界。
- **検出力の留保表示**（N年数・「非有意≠トレンド無し」注記）。**厳密判定（Hamed-Rao/ブロックブート/多重比較）は非同期＋キャッシュ**で返す段階表示（未確定事項③）。所有者認可。

### 6. View / templ / モック反映

- **新規「統計分析」ページ templ**（`internal/view/`）＋ DTO（`TrendPanelView`/`TrendBadgeView`/サマリ行 等・イミュータブル）。左サイドメニュー項目。
- **新規モック HTML**（`mocks/html/` に統計分析ページ）＋`style.css`（正本・`--color-trend` 等の新色トークン）。**ページ器・セレクタ・MK/Sen バッジ枠・検出力留保注記・サマリ表は静的な器＝モック反映必須**／**グラフ内部のトレンド線/CI帯/markLine 描画は反映例外**（feedback_mock_graph_rendering_exception）。`make sync-css`。

### 7. ライブラリ追加＋数値正確性の golden test

- `gonum.org/v1/gonum/stat`・`gonum/stat/distuv` を `go.mod` の **direct require へ昇格**（既にビルドグラフ内・追加コストほぼゼロ）。
- **golden test**: pyMannKendall / R modifiedmk の**既知データセットの期待値**（S・Z・p値・Sen's slope・Hamed-Rao 補正後 p値）と数値一致を検証（TDD・VPD 符号反転の教訓＝外部参照実装との照合が安全網）。

## スコープ外（このセッションでやらないこと）

- **`gonum/plot`（静的画像描画）の画面導入**＝横断⑭で不採用。**描画は go-echarts 一本**。統計プロット（CI帯/ヒストグラム/箱ひげ/ACF/Q-Q/トレンド線/バッジ）は全て ECharts で描く。※論文・報告書用の静的高品質図（SVG/PDF/EPS）エクスポートが要るかは将来（横断⑭の例外候補・本フェーズ対象外）。
- **レアイベントのルートB（希少バイナリのイベント位置総和 S 統計量）**＝**P11（台風）/P12（熱帯夜）**。本フェーズは年間日数化できる**ルートA（年間日数→MK/Sen）まで**触れるか、それも P11/P12 へ委ねるかは design（未確定事項⑨）。**結露/熱帯夜の年間日数トレンドの本体は P11/P12**。
- **器差・センサ交換（ギャップδ・Ng）の構造化**＝位置軸前提で **P10（multipoint-compare）**（付録G-4）。本フェーズはトレンドへのδ織り込みを**しない**（P5 も P10 へ defer 済み）。
- **STL/ACF/Edgeworth/SARIMA/変化点 Ng 推定**＝当面アプリ内不要（重い・オーバースペック・CSV外 or 将来）。本フェーズの検定は **MK/Sen/自己相関補正/Hamed-Rao/ブロックブートストラップ/多重比較**に限る。
- **相関・環境応答**＝P9（temp-humidity-correlation）。**多地点比較**＝P10。**圃場ベンチマーク・農家共有**＝P13。**本格時系列予測（Holt-Winters/SARIMA/ML）**＝P15（forecast-timeseries）。本フェーズの「予測」は線形回帰トレンドの記述まで（外挿の収穫予測は P7 所有）。
- **農家向け平易表示・共有 URL**＝P13（本フェーズは研究用詳細表示・ガードレール④）。
- **平年値の永続テーブル新設**＝既定スコープ外（自社データ計算 or 外部基準を借りる・年数不足時は出さない）。
- **データ主権 CSV の本体**＝P4（sensor-data-export）所有。本フェーズで統計分析結果の CSV を最小併設するかは design（未確定事項⑧・P4 と重複させない）。
- **能動通知（メール/LINE）**＝対象外（本フェーズは表示まで）。
- **device-show / dashboard / readings / P2〜P7 の既存機能の仕様変更**（無回帰維持・消費のみ）。認証・所有者認可・CSRF・期間バリデーション本体（S1/S5 所有）。

## 技術制約・準拠事項

- **付録G が中核（G-1〜G-11 全節）**: 自己相関補正必須（G-1）・MK＋Sen ペア（G-3）・タイ補正/連続性補正（G-2）・多重比較（G-7）・検出力留保「非有意≠トレンド無し」（G-8）・平年値の不確実性（G-9）・**システム内完結〔全統計 Go 計算＋go-echarts 表示・CSV はデータ主権用に併設〕（G-11）**。横断 ①集計軸・⑬統計分析ページ分離・⑭go-echarts 一本/gonum 計算・⑨自己相関補正・⑩MK+Sen ペア・⑪多重比較・⑫検出力留保。
- **ライブラリ**: `gonum/stat`・`distuv` を direct 昇格（p値化・分位の土台）。**検定本体は自前純Go**（gonum に MK/Sen/Hamed-Rao は無い）。**gonum/plot は画面に使わない**（go-echarts 一本）。
- **計算層と描画層の分離（ガードレール⑧）**: MK/Sen/補正/ブートストラップは純粋 Go（`internal/chart` の `[]float64` 入出力・**time 非依存**・gonum/stat・distuv は純粋計算ゆえ chart 層から利用可）。recorded_at の月次バケット・経過時間換算は **handler 境界**。**重い計算（ブロックブートストラップ B×O(N²)）は非同期＋キャッシュ**（ページロードをブロックしない）。
- **数値正確性（最重要）**: タイ補正・Hamed-Rao の有効標本サイズ式・Sen の中央値は誤りやすい。**pyMannKendall/R modifiedmk の既知データで golden test**（数式の取り違えはテストが仕様前提を符号化すると捕捉できない＝VPD 符号反転の教訓・project_vpd_physics_convention）。
- **ロールアップ前提**: O(N²) 検定は**必ず日次/月次ロールアップ後**に適用（生粒度に直接かけない・横断⑦保持期間設計が前提）。Seasonal MK or STL デシーズン化を**前置き**（生月次に素の MK を当てると季節が偽トレンド化）。
- **別型隔離**: トレンド option は `TrendChartSpec`（別型）で受け、温湿度 `ChartSpec`・VPD・露点・GDD の無回帰を守る。markLine/markArea/積み上げ area は go-echarts JSON タグ不具合を避け小文字キー自前注入（P3/P6/P7 同型）。
- **依存方向**（structure.md）: 下向き一方向。`internal/chart` 最下流純粋性。view は repository/service を import せず domain 表示メソッドのみ。**所有者認可は `internal/authz`**（閲覧系の非所有/不在は 404・列挙防止）。イミュータブル（sqlc 構造体は読取専用・純関数は入力スライス非破壊）。
- **スキーマ**: 原則 DDL なし（月次集計は SELECT のみ・goose 最新 00010 のまま・`make db-snapshot` 不要）。永続テーブル（平年値・トレンド結果キャッシュ）を起こすなら design で根拠明示（既定は非永続・メモリ/一時キャッシュ）。
- **言語**: 日本語コメント・エラー・コミット・UI ラベル（「長期トレンド」「Mann-Kendall 検定」「Sen の傾き」「自己相関補正」「信頼区間」「平年比」「検出力」「一次判定」等）。コード識別子は英語。
- **TDD**: 80% 以上。MK（S・タイ補正 Var(S)・連続性補正 Z・既知データ一致）・Sen's slope（全ペア中央値・外れ値頑健・既知値）・自己相関 λ̂/N_eff（λ=0.9 で分散膨張）・Hamed-Rao（pyMannKendall 期待値一致）・ブロックブートストラップ（seed 固定で再現・CI 幅）・FDR/Bonferroni（p 配列の補正）・p値化（distuv.Normal で両側 p）・月次ロールアップ（JST 月境界・空月）・検出力留保表示（N 小で「非有意≠無し」注記）・所有者認可（非所有→404）・**無回帰**（device-show・dashboard・P2〜P7 が従来どおり）。

## 受け入れ基準（概略）

1. **統計分析ページの新設**: 左サイドメニュー「統計分析」から新ページが開け、デバイス（自分の所有のみ・非所有は 404）と期間（月次/年次）を選んで長期トレンドが表示される。device-show とは別ページ・別ルートで、既存画面は無回帰。
2. **トレンド検定の正当性（◎）**: 月次ロールアップ系列に対し **自己相関補正つき Mann-Kendall ＋ Sen's slope** が**アプリ内 Go で計算**され、**MK=有無（Z/p値バッジ）／Sen=傾き（℃/年・%/年）** がペアで表示される。**素の MK でなく Seasonal MK or prewhitening/N_eff 補正**が施される（自己相関で偽トレンドを出さない）。
3. **システム内完結（外部アプリ不要）**: **Hamed-Rao 補正・ブロックブートストラップ信頼区間・多重比較補正も Go で計算**され go-echarts でアプリ内表示される（重い計算は非同期＋キャッシュ）。R/Python 等の外部プロセスに依存しない。p値化に `gonum/stat/distuv` を用い、`gonum/plot` は画面に使わない。
4. **数値正確性（golden test）**: MK の S・Z・p値・Sen's slope・Hamed-Rao 補正後 p値が **pyMannKendall/R modifiedmk の既知データセットと数値一致**する。
5. **検出力の留保**: N（年数）不足時に「**非有意≠トレンド無し**」が明示され、断定でなく Sen 傾き＋符号＋記述統計（平年比・年次推移）に留まる。バッジに一次判定（多重比較未補正）等のラベルが付く。
6. **月別・季節別とロールアップ**: 年集約だけでなく**月別/季節別トレンド**が取れ（符号反転を潰さない）、日次→週次→月次→年次のロールアップサマリ（平均/最高/最低/日較差/σ/CV）と日較差ΔT 推移、平年比が表示される。
7. **可視化**: 月次/年次折れ線＋ Sen トレンド線 markLine＋有意区間/CI 帯 markArea/積み上げ area＋ MK バッジが go-echarts（方式B）で描かれ、長期は dataZoom で閲覧できる（`TrendChartSpec` 別型・自前注入）。
8. **スキーマ**: 追加 DDL なし（月次集計は SELECT のみ・goose 最新 00010 のまま）。永続テーブルを足す場合のみ design で根拠明示。
9. **モック整合**: 統計分析ページ器・集計軸セレクタ・MK/Sen バッジ枠・検出力留保注記・サマリ表・左サイドメニュー項目がモック（新規 `mocks/html/` ページ＋`style.css` 正本・`--color-trend` 追加）に反映（グラフ内部の線/帯/markLine 描画は反映例外）。
10. **テスト 80% 以上**。

## 未確定事項・要確認（設計フェーズで決定）

- **① 統計分析ページの粒度・ナビ構造（design の最初の論点）**: 単一デバイス選択で長期トレンドを見る形から始めるか、最初から地点/作物横断のダッシュボードにするか。集計軸（横断①）のセレクタ UI・ルート設計・device-show からの導線（「このデバイスの長期分析へ」）・所有者認可の適用単位。P9/P10/P11/P13/P15 がこの器に乗る前提を崩さないこと。
- **② 月次/年次ロールアップの実装方式**: 月次集計 SQL 新設（`date_trunc('month', ...)`・長期×多行に有利）か `ListDailySensorAggregates`→Go 二段集計か。週次/年次の段、JST 月/年境界の扱い（いずれも SELECT のみ・DDL なし）。
- **③ 厳密判定の段階実装と非同期/キャッシュ**: Hamed-Rao/ブロックブートストラップCI/多重比較を初版に含めるか段階実装か。重い計算（B×O(N²)）の非同期化（goroutine＋進捗 or バックグラウンドジョブ＋HTMX ポーリング）・結果キャッシュ（device×期間×指標キー）・乱数 seed の固定/記録（再現性）。一次判定（即時）と厳密判定（遅延）の UI 段階表示。
- **④ gonum の最小依存範囲**: `stat`＋`stat/distuv` のみで足りるか（`stat/sampleuv` をブロックブートストラップに使うか・乱数は `math/rand` か gonum か）。direct 昇格する import の最小化。
- **⑤ 平年値の基準と外部データ**: 自社データから平年を作る（年数不足で不安定）か、気象庁アメダス長期平年値を外部基準に借りるか（取得方法・更新頻度・要否）。年数不足時は平年比を出さない方針の確定。
- **⑥ 自己相関の前置き方式**: Seasonal Mann-Kendall（月別に S を合算）か prewhitening（TFPW: Trend-Free Pre-Whitening）か STL デシーズン化か。λ̂ の推定方法（ラグ1/複数ラグ）・反復の要否・実データでの λ 実測（横断・第4章未確定）。
- **⑦ 検出力留保の見せ方**: 「非有意≠トレンド無し」「一次判定（多重比較未補正）」のラベル文言・バッジの信号色しきい値（有意水準）・トレンドを「検定」表示してよい最低データ年数の閾値。
- **⑧ データ主権 CSV の扱い**: 統計分析結果（月次サマリ・MK/Sen 結果）の CSV エクスポートを本フェーズに足すか、P4（sensor-data-export）へ委ねるか（重複回避）。CSV はあくまでデータ主権用で「表示は外部不要」を崩さない位置づけ。
- **⑨ レアイベントのルートA（年間日数トレンド）の所在**: 真夏日/熱帯夜/結露/VPD 逸脱の年間日数を MK/Sen でトレンド化する「ルートA」を本フェーズ（季節サマリの一部）に含めるか、P11（台風）/P12（熱帯夜）/P6（結露）へ委ねるか。`trend.go` の MK/Sen は共通基盤として P11/P12 が再利用する前提（重複実装回避）。
- **⑩ ギャップδ織り込みの将来連携**: 器差・センサ交換（Ng・δ）は P10 で構造化される前提（本フェーズ対象外）。将来 P10 完了後に不連続点付き回帰でδを織り込む拡張点を `trend.go` の API 設計で塞がないようにするか（前方互換の設計配慮）。

--- spec-init 本文 ここまで ---
