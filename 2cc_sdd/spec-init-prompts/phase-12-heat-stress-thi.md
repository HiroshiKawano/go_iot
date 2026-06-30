# フェーズ12（分析ロードマップ）spec-init プロンプト: THI・熱帯夜・高温ストレス（THI 温湿度指数の hour×day heatmap／熱帯夜〔夜温≥25℃〕の calendar ヒートマップ・連続日数・夜温推移／絶対湿度 AH による除湿負荷／日較差ΔT と品質の関係づけ／熱帯夜年間日数の経年トレンドは P8 `trend.go` の MK＋Sen を再利用・検出力の留保つき）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: heat-stress-thi
> 位置づけ: [分析アイデアメモ.md](../分析アイデアメモ.md) 第1章「実装ロードマップ」の**フェーズ12**〔推測〕。**P8 とは決定的に異なり、device-show（デバイス詳細）へのパネル上載せ**（**横断⑬**＝2026-06-29 ユーザー決定。「P12（THI・熱帯夜）はメインが1年以内（季節内）ゆえデバイス詳細」と明記）。沖縄は熱帯夜が長く日較差が稼ぎにくい＝品質低下要因（ユーザーは沖縄在住10年以上の実地知見＝権威。主要作物 ゴーヤ〔施設〕/サトウキビ〔露地〕/インゲン〔冬作〕/米〔離島二期作〕。`user_okinawa_local_knowledge`）。狙いは品種選定・遮光指導のエビデンス化。VPD（P3）・露点（P6）・GDD（P7）に続く **device-show 派生指標パネルの第4弾**で、**暑熱（高温多湿）側のストレスを語る**層。**唯一の例外＝「熱帯夜の年間日数の経年トレンド」（○）は多年集計ゆえ、P8（seasonal-trend）が新設した共通トレンド基盤 `internal/chart/trend.go`（`MannKendall`/`SensSlope`）を再利用**し（**G-5 ルートA＝閾値超えの年間日数化→従来 MK/Sen**・重複実装しない）、**数年では「検定」を断定せず Sen 傾き＋符号に留める（G-8）**。この多年トレンドを device-show のミニパネルに置くか「統計分析」ページ（P8 系列＝横断⑬の多年・横断軸）へ寄せるかは design 論点（下記 未確定事項）。
> 確度: 〔推測〕（早見表 confidence「推測」＝引継ぎメモの確定依頼でなくスタンプ側の仮説だが、**提案型案件ゆえ積極採用**＝付録C-3 の方針。`project_analysis_ideas_temp_humidity`「移動平均/VPD/多地点/GDD を仮説でも積極採用し開発進行・本人確認はデモ並行で着手ブロッカーにしない」）。ゴールが「リアルタイム警告（高温ストレス）」なら付録B-3(a) により前倒しされる位置づけ。
> 前提セッション: **論理依存＝蓄積（横断⑦＝熱帯夜の経年トレンドは要データ・年内の calendar/heatmap も季節を通した蓄積が前提）・集計軸（横断①＝P1 locality／P3 crop）**。**実装は P2〜P8 マージ済みコードへの「上載せ・流用」が前提**:
>  - **P3（vpd-dashboard）** … `internal/chart/vpd.go`＝**Tetens 定数 `tetensA=0.6108`/`tetensB=17.27`/`tetensC=237.3`・`saturationVaporPressure(tempC)→es(T)[kPa]`**（**絶対湿度 AH の実水蒸気圧 ea＝es(T)·RH/100 にこの飽和水蒸気圧をそのまま再利用＝重複定義しない**・付録A D④）・`VPD`/`VPDSeries`/`TimeInRange`／`internal/chart/vpd_echarts.go`＝`injectVPDMarkArea`〔go-echarts の `MarkAreaData.YAxis` が JSON タグ非準拠ゆえ option を JSON 化→マップへ戻し series へ ECharts 準拠の小文字キーで markArea を自前注入する確立パターン〕／`internal/handler/device_show_vpd.go`＝`buildVPDPanel`〔**別パネルを読み取り時に組む作法の手本**・`vpdHourlyRows`〔JST hour-of-day バケット＝THI hour×day heatmap の土台〕・`formatVPD`/`formatPercent`〕／`internal/domain/crop.go`＝`type Crop`＋9作物＋`VPDRange()`/`DiseaseModel()`/`GDDModel()`〔**作物別の派生指標属性を Go 定数で非破壊追加する確立パターン**＝高温ストレスしきい値が作物別に要るなら同型で `HeatStressModel()` を足す拡張点〕／`devices.crop`＝00009）
>  - **P8（seasonal-trend）** … `internal/chart/trend.go`＝`MannKendall(xs)→MKResult`・`SensSlope(xs)→SenResult`・`SeasonalMannKendall`・`HamedRaoModifiedMK`・`Lag1Autocorr`・`EffectiveSampleSize`・`BlockBootstrapSenCI`・`BenjaminiHochberg`/`Bonferroni`（**タイ補正 Var(S)・連続性補正 Z は実装済み・p値化は gonum/stat/distuv**）。→ **熱帯夜年間日数（各年に1値が立つ＝G-5 ルートA）のトレンドは `MannKendall`＋`SensSlope` を呼ぶだけ**で足る（**`trend.go` を P12 が再利用する前提は P8 が明示済み＝重複実装禁止**・付録G-5/G-10）。数年では `SensSlope` の傾き点推定＋符号に留め MK の有意性は断定しない（G-8）。`gonum/stat/distuv` は P8 で direct 昇格済み。
>  - **P2（temp-humidity-chart-stats）** … `internal/chart/stats.go`＝`Mean`/`MinMax`/`StdDev`/`CV`・**`DiurnalRange(values)→日較差ΔT`**（**日較差ΔT 推移はこの既存関数を再利用**）・`SMA`/`MovingStdDev`/`Band`。`dailyStatRows` の JST 暦日バケット作法〔熱帯夜日数・夜温の日次集約／calendar 1セル=1日 の土台〕。
>  - **P5（data-quality-meta）** … `internal/chart/quality.go`＝`StuckRuns`/`RapidChanges`〔連続ラン検出〕・欠測区間。**熱帯夜の「連続日数」（夜温≥25℃が続いた日数ラン）は `StuckRuns` と同型の連続ラン検出**（「同値」でなく「閾値超え日が連続」）。`gap_echarts.go`＝`injectGapMarkArea`〔小文字 `xAxis` 範囲 markArea 自前注入〕・`nullableLineData`〔欠測 nil〕。
>  - **P6/P7（dewpoint/gdd）** … `internal/handler/device_show_dewpoint.go` `buildDewpointPanel`・`device_show_gdd.go` `buildGDDPanel`＝**`buildVPDPanel` を写経して別パネルを起こす作法の最新前例**（本フェーズの `buildHeatStressPanel` はこれらの写経）。`dewpoint_echarts.go`/`gdd_echarts.go`＝markLine/markArea 自前注入。`series.go` の `VPDChartSpec`/`DewpointChartSpec`/`GDDChartSpec`/`TrendChartSpec`〔**別型隔離**〕。
>  - **S5/E1（device-detail・device-chart-echarts）** … `internal/handler/device_show.go` の `buildChartArea(ctx,device,period,now)`＝生行 `ListRecentSensorReadings` → 温湿度 float＋ラベル → 温湿度 option・統計カード・日次集計・**`buildVPDPanel`/`buildDewpointPanel`/`buildGDDPanel`** を組み `DeviceChartAreaView` を返す拡張点。`jst`・`statEmptyMark="—"`・`deviceCrop(device)`・欠測ギャップ `applyGapGrid`。`App.templ` の `EChartsInitializer`〔`[data-echarts]` 走査→init/dispose・`echarts.connect`〕。
>  - **S1（web-foundation-auth）** … RequireAuth・**所有者認可 `internal/authz`（`RequireDeviceOwner`・閲覧系の非所有/不在は 404＝列挙防止）**。
> **可視化の核＝新 ECharts 系列型の初導入（最重要・このフェーズの設計の核その1）**: P1〜P8 は line／積み上げ area／markLine／markArea／gauge で通したが、**本フェーズの ◎ 主役は「熱帯夜 calendar ヒートマップ」（calendar 座標＋heatmap）・○「THI hour×day heatmap」（heatmap＋visualMap）で、いずれも本リポジトリで未使用の ECharts 系列型**（`grep` 確認済み＝`internal/chart` に calendar/heatmap/visualMap の利用ゼロ）。go-echarts v2.7.2 が calendar 座標系・heatmap 系列・visualMap component をどこまで型で表現できるか（表現しきれない属性は P3/P5 と同型の **JSON 化→マップ注入の自前パターン**で補う）を design で実証することが第一の論点。`EChartsInitializer` の `[data-echarts]` 走査・`echarts.connect` が heatmap/calendar でも破綻しないこと（連動の要否含む）も確認する。**visualMap は §2-3 早見表「値で線色を変える（THI ストレス度）」「時間帯×日の濃淡（heatmap）」「日単位の連続性（熱帯夜＝calendar 座標＋heatmap）」に対応**（◎/○ は実現可能と早見表が評価）。
> **スキーマ判断（最重要・このフェーズの設計の核その2）**: THI・絶対湿度 AH・熱帯夜（夜温≥25℃）・夜温（夜間最低/平均）・連続日数・日較差ΔT・熱帯夜年間日数は**すべて既存 `sensor_readings`（temperature/humidity の2列＋recorded_at）から読み取り時に計算できる**（THI＝付録A D⑥ の確定式・AH＝D④・熱帯夜/夜温は気温の夜間バケット集約・ΔT＝`DiurnalRange`・年間日数→MK/Sen は `trend.go`）。よって **P2〜P8 と同じく「読み取り時計算・スキーマ非変更」を既定方針**とする（ガードレール②＝派生列は必要フェーズで足す YAGNI／⑧＝軽い計算はアプリ内・重い統計は CSV 外出し）。**goose 最新は 00010（P7 planting_date）のまま・`make db-snapshot` 不要・新規 SQL は熱帯夜/夜温の年間〜日次集約 SELECT のみで DDL なし**。高温ストレスしきい値（熱帯夜の夜温 25℃・THI のストレス帯境界）は**まず作物非依存の物理/生理定数**として持ち、**作物別に分ける必要が判明したら `domain.Crop` へ Go 定数で非破壊追加**（`VPDRange()`/`DiseaseModel()`/`GDDModel()` と同型の `HeatStressModel()`・§100・DB 列を増やさない）＝**DB 列は増やさない**。
> 設計フェーズで参照:
> - 上位ロードマップ: 分析アイデアメモ.md フェーズ12（**THI 高温ストレス帯の時間帯別集計／熱帯夜〔夜温25℃以上〕の連続日数・夜温推移／絶対湿度 AH による除湿負荷／日較差ΔT〔糖度・着果〕と品質の関係づけ／使う手法 D⑥〔THI〕・D④〔AH〕・A・G-5 ルートA〔熱帯夜の年間日数を MK/Sen でトレンド化＝タイ補正必須・G-2〕・G-8〔検出力の留保〕／▶ ◎ 熱帯夜 calendar ヒートマップ・○ THI hour×day heatmap・○ 熱帯夜日数の経年トレンド〔年集計＝アプリ内 MK/Sen・数年では Sen傾き＋符号に留める G-8〕**）／ **【2026-06-30 追記】「夜温の高止まり/朝の立ち上がり」を見たい隠れたニーズ（UECS-GEAR のローソク足が狙う情報）は本フェーズの時間帯別集計・夜温（夜間最低/平均）系列がより直接的な器**（ローソク足は日周性で陽陰線が無意味化＝不採用・フェーズ2/P2b 追記と整合）／ 第2章 §2-2 クラッタ対策（主役1〜2＋派生は別パネル・線より「帯/濃淡」で見せる）・§2-3 表現テク早見表（**THI ストレス度＝visualMap・時間帯×日＝heatmap・日単位の連続性〔熱帯夜〕＝calendar 座標＋heatmap・固定しきい値帯＝markArea**）・§2-4（フェーズ12 ◎主役＝熱帯夜 calendar）／ 第3章ガードレール ①集計軸（月別/季節別・年別を壊さない）・②スキーマに派生指標列を後付け可能に（実列追加は必要フェーズで＝YAGNI）・④研究用画面と農家共有画面の分離（農家共有＝P13・本フェーズは研究用）・⑥作物メタデータの保持（高温ストレスしきい値が作物別に要るなら `domain.Crop`・無ければ既定）・⑧計算層と描画層の分離（軽い THI/AH/熱帯夜判定はアプリ内・重い統計は CSV 外出し）・⑬統計分析ページ vs device-show の振り分け（**P12 はメイン1年以内ゆえ device-show・多年トレンドのみ P8 系列と連携**）／ 付録B-4（米/サトウキビ＝露地で GDD・日較差・THI が前面・VPD は効きにくい）・B-3(a)（リアルタイム警告ゴールなら前倒し）
> - 数式（権威）: 分析アイデアメモ.md 付録A —「**D⑥ THI（温湿度指数/高温ストレス）**」（**THI = 0.8·T + (RH/100)·(T − 14.4) + 46.4**・高温多湿の複合ストレスを1指標化）・「**D④ 絶対湿度（容積絶対湿度）AH**」（**AH ≈ 217·ea/(T+273.15) [g/m³]**・ea は実水蒸気圧 [kPa]＝`saturationVaporPressure(T)·RH/100` を Pa 換算して使う・温度に依存しない実際の水分量＝換気・除湿判断に有用）・「**A 基本統計量**」（平均/σ/CV/**日較差ΔT**＝`DiurnalRange`）／ 付録G —「**G-5 レアイベントのトレンド検定 ルートA**」（**真夏日・熱帯夜・無降水日のように閾値超えを年間日数へ集計すれば各年に1値が立つ→G-2/G-3〔MK/Sen〕をそのまま適用・年集計→MK/Sen は軽く Go 実装可**・本体は `trend.go` 再利用）・「**G-2 Mann-Kendall**」（タイ補正必須＝温湿度はセンサ分解能や日数系列〔熱帯夜0日の年が複数〕で同値が頻発し Var(S) が歪む・既に `trend.go` で実装済み）・「**G-8 検出力・必要データ年数の留保**」（**数年蓄積では N が小さく「トレンドあり」断定は時期尚早・Sen 傾き点推定＋符号と記述統計に留め、有意性結論は長期蓄積後・「非有意≠トレンド無し」を明示**）
> - 現スキーマ（権威）: docs/database_snapshot/table_definitions.md「sensor_readings」（**temperature/humidity numeric(5,2)・CHECK 温度 −40〜125・湿度 0〜100・recorded_at timestamptz〔計測〕・created_at〔受信〕**・(device_id, recorded_at DESC) 部分索引 WHERE deleted_at IS NULL。THI/AH/熱帯夜/夜温/ΔT は temperature＋humidity から導出でき**スキーマ非変更**で足る）／「devices」（**locality VARCHAR(20)＝P1/00008・crop VARCHAR(20)＝P3/00009・planting_date DATE＝P7/00010**・CHECK `devices_crop_valid` で9作物ミラー）。**goose 連番の最新は 00010**。
> - 移行元・拡張対象コード（実コード確認済み・P8 マージ後の現状）:
>   - `internal/chart/vpd.go` … `saturationVaporPressure(tempC)→es(T)[kPa]`（**AH の実水蒸気圧 ea＝es(T)·RH/100 にそのまま再利用**・氷点下でも NaN/Inf を出さない）・Tetens 定数。→ **THI・AH の純関数は新設 `internal/chart/heatstress.go`**（`vpd.go`/`dewpoint.go`/`gdd.go` の隣・同じ純粋性＝`math` のみ依存・time 非依存）に追加し、`saturationVaporPressure` を再利用（重複定義しない）。
>   - `internal/chart/stats.go` … `DiurnalRange(values)→日較差ΔT`・`Mean`/`MinMax`/`StdDev`/`CV`・JST 暦日バケット `dailyStatRows` 作法。→ **日較差ΔT 推移は `DiurnalRange` を再利用**・夜温/熱帯夜の日次集約は `MinMax`/`Mean` を夜間バケットへ。
>   - `internal/chart/trend.go`（P8）… `MannKendall(xs)→MKResult`・`SensSlope(xs)→SenResult`（タイ補正/連続性補正/p値化 実装済み）。→ **熱帯夜年間日数（年ごとに1値＝G-5 ルートA）に `MannKendall`＋`SensSlope` をそのまま適用**（重複実装禁止・数年では Sen 傾き＋符号に留める G-8）。
>   - `internal/chart/quality.go`（P5）… `StuckRuns(values,minRun)`〔連続ラン〕。→ **熱帯夜「連続日数」（夜温≥25℃ が続いた日のラン＝最長連続・現在連続）は連続ラン検出と同型**の純関数（「同値」でなく「閾値超え日が連続」）。
>   - `internal/chart/vpd_echarts.go`/`gap_echarts.go` … `injectVPDMarkArea`（小文字 `yAxis`）・`injectGapMarkArea`（小文字 `xAxis`）＝**go-echarts の JSON タグ不具合を避け option マップへ ECharts 準拠キーを自前注入する確立パターン**。→ **calendar/heatmap/visualMap で go-echarts の型が表現しきれない属性（visualMap の piecewise 区分・calendar の range/cellSize 等）は同型の自前注入で補う**（新設 `internal/chart/heatstress_echarts.go`）。
>   - `internal/handler/device_show_vpd.go`（P3）/`device_show_dewpoint.go`（P6）/`device_show_gdd.go`（P7）… `buildVPDPanel`/`buildDewpointPanel`/`buildGDDPanel`＝**別パネルを読み取り時に組む作法の手本**（`vpdHourlyRows`〔JST hour-of-day バケット〕・`formatVPD`/`formatPercent`・空データの空カード）。→ **新設 `internal/handler/device_show_heatstress.go` に `buildHeatStressPanel(...)` を写経で起こす**（THI hour×day・熱帯夜 calendar・夜温系列・AH・ΔT・熱帯夜連続日数カード・〔多年があれば〕年間日数 Sen トレンド）。
>   - `internal/handler/device_show.go`（S5/E1/P2〜P8）… `buildChartArea` が温湿度 option/カード/日次（`applyGapGrid`）に続けて `buildVPDPanel`/`buildDewpointPanel`/`buildGDDPanel` を呼び `DeviceChartAreaView` へ詰める。`jst`・`deviceCrop(device)`・`statEmptyMark="—"`。→ **`buildHeatStressPanel(...)` を追加呼び出し**し `DeviceChartAreaView` 末尾へ `HeatStress HeatStressPanelView` を非破壊追加。**THI hour×day/熱帯夜 calendar は「夜間」「暦日」「年」境界が要るため時刻バケットは handler 境界**（純粋層は `[]float64`＋index）。
>   - `internal/chart/series.go` … `ChartSpec`／`VPDChartSpec`／`DewpointChartSpec`／`GDDChartSpec`／`TrendChartSpec`〔別型隔離〕。→ **`HeatStressChartSpec`（THI heatmap・熱帯夜 calendar・夜温 line・AH 等）を別型で隔離**し温湿度/VPD/露点/GDD/トレンドの無回帰を守る。
>   - `internal/view/component/views.go` … `DeviceChartAreaView`（温湿度 option/カード/日次・`VPDPanelView`/`DewpointPanelView`/`GDDPanelView`・`HasGap`）。→ **`HeatStressPanelView`・`HeatStressCardView`・夜温/熱帯夜日次行・年間日数行 DTO をイミュータブルに追加**（`DeviceChartAreaView` 末尾へ非破壊追加）。
>   - `internal/view/component/DeviceChartArea.templ` … 期間ボタン＋温湿度2グラフ＋数値カード＋日次集計表＋VPD/露点/GDD パネル。`App.templ` の `EChartsInitializer`。→ **THI heatmap（`#thi-heatmap`）＋熱帯夜 calendar（`#tropical-night-calendar`）＋夜温系列＋AH＋ΔT カード・熱帯夜連続日数カードを別ブロックで追加描画**（`data-echarts`）。器（パネル枠/カード/表/凡例）はモック準拠・独自クラス新設は最小。
> - ビュー/モック（単一ソース運用の境界）: `internal/view/component/DeviceChartArea.templ`／ `mocks/html/device-show.html`＋`mocks/html/style.css`（正本・`--color-vpd`/`--color-dewpoint`/`--color-gdd` の隣に**高温ストレス用の新色トークン `--color-heat`〔暑熱＝暖色側〕を追加**）。**THI/熱帯夜パネルの器・夜温/AH/ΔT/連続日数カード・熱帯夜日数表は静的な「器」＝HTML/CSS ゆえモック反映必須**（feedback_mock_reflects_impl_visual・project_css_single_source）。**heatmap/calendar/visualMap のセル濃淡・トレンド線描画は動的描画ゆえモック反映の例外**（feedback_mock_graph_rendering_exception＝器は反映対象・濃淡/線/凡例は対象外。ただし**凡例/カラースケールの「枠とラベル」は器**）。色は `@layer components`・`:root` トークンへ追記し `make sync-css`。
> - 命名・依存規約: .kiro/steering/structure.md（依存方向＝下向き一方向・`internal/chart` は最下流純粋層〔math のみ・time 非依存〕・view→domain 表示メソッドのみ・§98 外部キー張らない・§99 論理削除・**§100 マスタ/列挙は Go 定数+VARCHAR+CHECK**・所有者認可は `internal/authz` 集約）／ tech.md（sqlc・データアクセス方針）／ CLAUDE.md（**既定はマイグレーション無し**＝`make db-snapshot` 不要。新規 SQL を起こす場合も SELECT のみで DDL なし）。
> - 物理規約（最重要・project_vpd_physics_convention を順守）: VPD は乾燥度指標で「高VPD=乾きすぎ=暖色／低VPD=多湿=寒色」と確定済みで、**後続フェーズ（露点 P6／THI P12 等）も同向き**と明記されている。**THI/熱帯夜/高温ストレスは「暑熱側」＝高 THI=高温多湿ストレス=暑い=暖色**（`--color-heat`）で揃える。熱帯夜 calendar の濃淡・THI heatmap の visualMap・ストレス帯の色/符号/「安全〜危険」ラベルの向きを取り違えない。**spec/テストに向きを明記し、GO 判定後の実機スモークで目視確認**（P3 VPD で向き逆転がテスト全緑のまま実機スモークまで残った前例＝テストは仕様前提を符号化すると向きの誤りを捕捉できない）。

--- spec-init 本文 ここから ---

## 機能概要

蓄積した温湿度計測データから**暑熱（高温多湿）ストレスの蓄積解析層**を device-show（デバイス詳細画面）に上載せし、**熱帯夜が長く日較差が稼ぎにくい沖縄固有の品質低下要因**（ユーザーは沖縄在住10年以上の実地知見＝権威）を可視化して、品種選定・遮光指導のエビデンスにする。生の気温・湿度ではなく、**THI（温湿度指数）・絶対湿度 AH・熱帯夜（夜温≥25℃）・夜温推移・日較差ΔT**という暑熱の駆動因に変換して語ることで、VPD（P3）・露点（P6）・GDD（P7）に続く**派生指標ダッシュボードの第4弾**を作る。**横断⑬により本フェーズは device-show のパネル**（メインが1年以内＝季節内の暑熱ストレスゆえ。多年・横断は「統計分析」ページ＝P8 系列）。具体的には、既存の VPD/露点/GDD パネルと並ぶ**別パネル（高温ストレスパネル）**として次を追加する。

1. **THI（温湿度指数）の hour×day heatmap**（付録A D⑥・`THI = 0.8·T + (RH/100)·(T − 14.4) + 46.4`）を、**時間帯（hour-of-day）× 日 の heatmap＋visualMap（ストレス度で濃淡）**で描く（メモ §2-3「時間帯×日の濃淡＝heatmap」「THI ストレス度＝visualMap」）。高温多湿が昼夜どの時間帯に集中するかを一目で示す。
2. **熱帯夜（夜温≥25℃）の calendar ヒートマップ（◎ 主役）** — 暦日 × 月の **calendar 座標＋heatmap** で、各日が熱帯夜か（夜温≥25℃）・夜温の高さを濃淡表示する（メモ §2-3「日単位の連続性〔熱帯夜〕＝calendar 座標＋heatmap」・§2-4 ◎ 主役）。**熱帯夜の連続日数**（最長連続・現在連続）と**夜温（夜間最低/平均）の推移**も併せて示す。
3. **絶対湿度 AH（除湿負荷）**（付録A D④・`AH ≈ 217·ea/(T+273.15) [g/m³]`, `ea = es(T)·RH/100`＝`saturationVaporPressure` 再利用）— 温度に依存しない実際の水分量を時系列/カードで示し、換気・除湿の負荷判断に使う。
4. **日較差ΔT（糖度・着果）と品質の関係づけ** — 既存 `DiurnalRange` で日較差ΔT を日次推移として示し（昼夜の温度差が稼げているか）、暑熱で日較差が縮む沖縄の品質低下を可視化する。
5. **熱帯夜年間日数の経年トレンド（○・多年・G-5 ルートA）** — 各年の熱帯夜日数を集計すれば**各年に1値が立つ**（付録G-5 ルートA）ので、**P8 が新設した共通基盤 `internal/chart/trend.go` の `MannKendall`＋`SensSlope` をそのまま再利用**してトレンド化する（**重複実装しない**）。ただし**蓄積が数年では N（年数）が小さく検出力が低い**（付録G-8）ため、**「検定」の有意性は断定せず Sen 傾きの点推定＋符号＋記述統計に留め、「非有意≠トレンド無し」を明示**する。この多年パートを device-show のミニパネルに置くか「統計分析」ページ（P8 系列＝横断⑬の多年・横断軸）へ寄せるかは design 論点。

**設計の核その1＝新 ECharts 系列型（calendar/heatmap/visualMap）の初導入**: P1〜P8 は line／積み上げ area／markLine／markArea／gauge で通したが、**本フェーズの ◎/○ 主役は calendar 座標＋heatmap・heatmap＋visualMap で、いずれも本リポジトリ未使用の系列型**。go-echarts v2.7.2 がこれらをどこまで型で表現できるか、表現しきれない属性は **P3/P5 と同型の JSON 化→マップ自前注入**で補えるか、`EChartsInitializer`（`[data-echarts]` 走査・`echarts.connect`）が heatmap/calendar で破綻しないかを design で実証する（第一の論点）。

**設計の核その2＝スキーマ非変更を既定とする**: THI・AH・熱帯夜・夜温・連続日数・ΔT・熱帯夜年間日数は**すべて既存 `sensor_readings`（temperature/humidity の2列＋recorded_at）から読み取り時に計算できる**。よって **P2〜P8 と同じく「読み取り時計算・マイグレーションなし」を既定方針**とする（ガードレール②/⑧・goose 最新 **00010** のまま・`make db-snapshot` 不要・新規 SQL は熱帯夜/夜温の年間〜日次集約 SELECT のみで DDL なし）。高温ストレスしきい値（夜温 25℃・THI ストレス帯境界）はまず**作物非依存の物理/生理定数**として持ち、作物別に分ける必要が判明したら `domain.Crop` へ Go 定数で非破壊追加（`VPDRange()`/`DiseaseModel()`/`GDDModel()` と同型の `HeatStressModel()`・§100）＝**DB 列は増やさない**。S5/E1/P2〜P8 の既存機能（温湿度2グラフ・統計オーバーレイ・VPD/露点/GDD パネル・欠測ギャップ・期間切替・connect 連動）は**無回帰で維持**する。

> **物理規約の向き（重要・project_vpd_physics_convention を順守）**: VPD は「高=乾きすぎ=暖色／低=多湿=寒色」で確定し**後続（露点 P6／THI P12）も同向き**と明記。**THI/熱帯夜/高温ストレスは暑熱側＝高 THI=高温多湿ストレス=暑い=暖色（`--color-heat`）**で揃える。熱帯夜 calendar の濃淡・THI heatmap の visualMap・ストレス帯の色/符号/「安全〜危険」ラベルの向きを取り違えない。P3 VPD では spec 初稿で向きが逆になりテストも同前提で符号化したため全緑のまま誤りが残り**実機スモークで初めて発覚**した。本フェーズは**暑熱＝暖色の向きを spec/テストに明記し、色・濃淡・ラベルの向きを実機スモークで必ず目視確認**する。

> **「夜温」の定義（重要・未確定／ユーザー権威で確定）**: 「熱帯夜＝夜温≥25℃」の「夜温」をどう取るか（**日最低気温≥25℃**〔気象庁の熱帯夜の公式定義に近い〕か、**夜間時間帯〔例 18:00〜翌06:00〕の最低/平均≥25℃**か）と、夜間時間帯の窓は **JST で日跨ぎ**する（夕方〜翌朝）ため handler 境界のバケット設計が要る。**沖縄は熱帯夜が非常に多い**（ユーザーの実地知見＝権威）ため、定義次第で熱帯夜日数が大きく変わる。定義・夜間窓・閾値は**ユーザーの沖縄実地知見（権威）で確定**する（design 未確定事項）。

## 背景・現状

P8（seasonal-trend）マージ後の現状は以下（実コード確認済み）。

- **派生指標の純粋層**: `internal/chart/vpd.go` が `saturationVaporPressure(tempC)→es(T)[kPa]`・Tetens 定数 `tetensA/B/C` を `math` のみ依存・time 非依存で提供。**AH の実水蒸気圧 ea＝es(T)·RH/100 にこの `saturationVaporPressure` をそのまま再利用できる**（付録A D④）。`dewpoint.go`（P6・露点 Td）・`gdd.go`（P7・GDD）も同じ純粋層に並ぶ。**THI・AH・熱帯夜判定・夜温集約・連続日数の純関数はまだ無い**。
- **共通トレンド基盤（P8）**: `internal/chart/trend.go` が `MannKendall(xs)→MKResult`・`SensSlope(xs)→SenResult`・`SeasonalMannKendall`・`HamedRaoModifiedMK`・`Lag1Autocorr`・`EffectiveSampleSize`・`BlockBootstrapSenCI`・`BenjaminiHochberg`/`Bonferroni` を実装済み（**タイ補正 Var(S)・連続性補正 Z つき・p値化は gonum/stat/distuv**）。**熱帯夜年間日数（G-5 ルートA＝各年に1値）のトレンドは `MannKendall`＋`SensSlope` を呼ぶだけ**で足り、**P8 が「`trend.go` を P11/P12 が再利用する前提（重複実装回避）」を明示済み**。
- **日較差・基本統計**: `internal/chart/stats.go` に **`DiurnalRange(values)→日較差ΔT`**・`Mean`/`MinMax`/`StdDev`/`CV` と JST 暦日バケット `dailyStatRows` 作法。**日較差ΔT 推移はこの既存関数を再利用**。
- **連続ラン検出**: `internal/chart/quality.go`（P5）に `StuckRuns(values,minRun)`〔同値連続ラン〕。**熱帯夜の連続日数（夜温≥25℃ が続いた日のラン）は同型**だが「同値」でなく「閾値超え日が連続」を見る新関数が要る。
- **markArea/markLine 自前注入と別型隔離**: `vpd_echarts.go`/`gap_echarts.go`/`dewpoint_echarts.go`/`gdd_echarts.go`/`trend_echarts.go` が go-echarts の JSON タグ不具合を回避して option マップへ ECharts 準拠の小文字キーを自前注入する確立パターン。`series.go` の `VPDChartSpec`/`DewpointChartSpec`/`GDDChartSpec`/`TrendChartSpec` が別型隔離の手本。**ただし calendar/heatmap/visualMap は本リポジトリ未使用**（`grep` 確認＝`internal/chart` に利用ゼロ）＝新系列型の option 構築は本フェーズが初。
- **別パネル組立の作法**: `internal/handler/device_show_vpd.go` `buildVPDPanel`・`device_show_dewpoint.go` `buildDewpointPanel`・`device_show_gdd.go` `buildGDDPanel` が、生行から系列・hour-of-day/暦日バケット・カードを組み `component.*PanelView` を返す（時刻は handler 境界・純粋層は `[]float64`）。`buildChartArea`（`device_show.go`）がこれらを順に呼び `DeviceChartAreaView` へ詰める。**高温ストレスパネルはこの作法の写経で起こせる**。
- **作物マスタ**: `internal/domain/crop.go` が `type Crop`＋9作物＋`Label()`/`Valid()`/`VPDRange()`/**`DiseaseModel()`**/**`GDDModel()`**/`AllCrops()` を持ち、**「作物別の派生指標属性を Go 定数で非破壊追加する」確立パターン**（VPD 適正帯・病害モデル・GDD モデルが既に同型で同居）。`devices.crop`（00009・CHECK ミラー）。**高温ストレスしきい値が作物別に要れば同型の `HeatStressModel()` を足す**（DB 列は増やさない）。
- **スキーマ**: `sensor_readings` は temperature/humidity（numeric(5,2)・CHECK −40〜125 / 0〜100）＋recorded_at（計測）＋created_at（受信）。`devices` は locality（P1/00008）/crop（P3/00009）/planting_date（P7/00010）追加済み。**goose 最新は 00010**。THI/AH/熱帯夜/夜温/ΔT/年間日数は2列から導出でき**読み取り側のみで足りる**（既定方針）。
- **時刻・タイムゾーン**: JST 固定（`jst`）。time は handler 境界に留め純粋層（chart）へ持ち込まない規約（structure.md）。**熱帯夜の夜間窓は JST で日跨ぎ**するため handler のバケット設計が要る。
- **高温ストレスパネル（THI heatmap・熱帯夜 calendar・夜温・AH・ΔT トレンド）は device-show に存在しない**（現状は温湿度グラフ＋統計＋VPD/露点/GDD パネル＋欠測ギャップ＋最新計測テーブル）。

## このセッションのスコープ（実装対象）

### 1. 暑熱ストレスの純粋層（`internal/chart/heatstress.go`・time 非依存）

- **新設 `internal/chart/heatstress.go`**（`vpd.go`/`dewpoint.go`/`gdd.go` の隣・同じ純粋性）に、**`[]float64`/スカラ入出力・`math` のみ依存・time 非依存**の純関数群を追加する。`internal/chart` 最下流純粋層の規約を厳守（gin/DB/templ/pgtype/time を import しない）。
  - **THI**: `THI(tempC, rh float64) float64`（付録A D⑥・`THI = 0.8·T + (RH/100)·(T − 14.4) + 46.4`）。系列版 `THISeries(temps, hums []float64) []float64`（`VPDSeries` と同型）。RH を [0,100] にクランプ（防御）。氷点下でも NaN/Inf を出さない。
  - **絶対湿度 AH**: `AbsoluteHumidity(tempC, rh float64) float64`（付録A D④・`ea = es(T)·RH/100`〔kPa〕→Pa 換算→`AH ≈ 217·ea/(T+273.15)` [g/m³]・**`saturationVaporPressure` を再利用＝重複定義しない**）。系列版。`T+273.15`（絶対温度）は CHECK 下限 −40℃ でも 233.15 で正＝ゼロ割なし。
  - **熱帯夜判定・夜温**: 「夜温（夜間最低 or 平均）≥ 閾値（既定 25℃）」を判定する純関数（夜温の系列を受けて bool/連続区間を返す。閾値は引数）。**夜間バケット（夜間窓・暦日）は handler 境界**で組み、純粋層には夜温の `[]float64`（日次）を渡す。
  - **熱帯夜の連続日数**: 夜温≥閾値が続いた日のラン（最長連続・末尾の現在連続）を返す純関数（`StuckRuns` の連続ラン検出と同型・「同値」でなく「閾値超え」）。
  - **日較差ΔT**: `stats.go` の **`DiurnalRange` を再利用**（新規実装しない）。日次の昼夜温度差。
  - すべて純粋・入力スライス非破壊。
- **熱帯夜年間日数のトレンドは `trend.go` を再利用**: 各年の熱帯夜日数を `[]float64`（年ごとに1値）にして **`MannKendall`＋`SensSlope` を呼ぶだけ**（**新規 MK/Sen を実装しない**・G-5 ルートA・G-2 タイ補正は既に `trend.go` 実装済み）。数年では `SensSlope` の傾き＋符号に留め MK の有意性は断定しない（G-8）。
- **重い統計はやらない**: STL/SARIMA/Edgeworth・レアイベントのルートB（希少バイナリのイベント位置総和 S 統計量＝台風の P11 領域）は本フェーズでアプリ内計算せず、必要なら CSV 外出し（ガードレール⑧）。本フェーズは「THI/AH の確定式＋熱帯夜の閾値判定・連続ラン＋既存 `trend.go` 再利用」に集中する。

### 2. 描画層（`internal/chart/heatstress_echarts.go`・heatmap/calendar/visualMap）

- **新設 `internal/chart/heatstress_echarts.go`** に option 構築関数を起こす。**(a) THI hour×day heatmap**（heatmap 系列＋visualMap でストレス度の濃淡）・**(b) 熱帯夜 calendar ヒートマップ**（calendar 座標系＋heatmap で 暦日×月のセル濃淡）・**(c) 夜温推移 line**・**(d) AH 時系列 line**・**(e)〔多年があれば〕熱帯夜年間日数の棒/折れ線＋Sen トレンド markLine**。HTML 安全 JSON（`encoding/json`・SetEscapeHTML=true・`</script>` 不混入）を維持。
- **新系列型の自前注入**: go-echarts v2.7.2 の型で calendar 座標系・visualMap・heatmap が表現しきれない属性は、**P3 `injectVPDMarkArea`／P5 `injectGapMarkArea` と同型の「option を JSON 化→マップへ戻し ECharts 準拠キーを自前注入」**で補う（visualMap の `pieces`/`inRange.color`・calendar の `range`/`cellSize`/`orient` 等）。**暑熱＝暖色**の visualMap カラースケール（`--color-heat` 系）。
- **入力契約**: `internal/chart/series.go` に **`HeatStressChartSpec` を別型で隔離**（THI heatmap データ・熱帯夜 calendar データ・夜温系列・AH 系列・年間日数 等）。温湿度 `ChartSpec`・VPD/露点/GDD/トレンドの各 Spec の無回帰を守る。
- **クラッタ回避**: 高温ストレスパネルは温湿度・VPD・露点・GDD と**別パネル**（§2-2）。主役は熱帯夜 calendar（◎）＋THI heatmap（○）。線を増やしすぎない。

### 3. handler（`internal/handler/device_show_heatstress.go`・パネル組立）

- **新設 `internal/handler/device_show_heatstress.go`** に `buildHeatStressPanel(...)` を起こす（`buildVPDPanel`/`buildDewpointPanel`/`buildGDDPanel` の写経）。生行から THI（hour×day バケット）・熱帯夜（夜間窓→暦日バケット→年バケット）・夜温系列・AH 系列・日較差ΔT を組み `component.HeatStressPanelView` を返す。**夜間窓〔日跨ぎ〕・hour-of-day・JST 暦日・年バケットは handler 境界**（`vpdHourlyRows`/`dailyStatRows` 作法の拡張）・純粋層は `[]float64`。
- **`device_show.go` の `buildChartArea` から呼び出し**、`DeviceChartAreaView` 末尾へ `HeatStress HeatStressPanelView` を非破壊追加（VPD/露点/GDD パネルと並列・`HasData=false`〔計測0件〕では空パネルで templ 側非表示）。
- **熱帯夜 calendar の期間**: calendar は本質的に年スケール（暦日×月）。device-show の期間セレクタ（24h/3d/7d/30d）とは別に、**選択中の年（または直近1年）の calendar を描く**（GDD パネルが期間セレクタ非連動で定植日→現在を描く前例と同型・design 判断）。
- **高温ストレスしきい値**: まず作物非依存の既定（夜温 25℃・THI ストレス帯境界）。作物別に分ける必要が判明したら `deviceCrop(device)`→`domain.Crop` の `HeatStressModel()`（新設・design 判断）で解決。
- **カード**: 現在 THI・現在 AH・本日/直近の夜温・熱帯夜の連続日数（最長/現在）・〔多年あれば〕熱帯夜年間日数の Sen 傾き＋符号 等を整形カードで（`VPDCardView` と同型・`formatStat`/`statEmptyMark` 流用）。

### 4. View / templ / モック反映

- `internal/view/component/views.go` に **`HeatStressPanelView`・`HeatStressCardView`・THI heatmap セル DTO・熱帯夜 calendar セル DTO・夜温/熱帯夜日次行・年間日数行 DTO**をイミュータブルに追加（`DeviceChartAreaView` 末尾へ非破壊追加）。
- `DeviceChartArea.templ` に **THI heatmap（`#thi-heatmap`）＋熱帯夜 calendar（`#tropical-night-calendar`）＋夜温推移＋AH 時系列＋日較差ΔT＋熱帯夜連続日数カード**を VPD/露点/GDD パネルの下に追加描画（`data-echarts data-color`）。既存クラス（`.summary-grid-4`・`.data-table` 等）を流用し独自クラス新設は最小。
- `App.templ` の `EChartsInitializer` は `[data-echarts]` 走査で heatmap/calendar も init される想定。**heatmap/calendar/visualMap が走査・dispose・connect で破綻しないこと**を確認（連動の要否＝温湿度との axisPointer 共有は heatmap/calendar では不自然な可能性＝design 判断）。
- **モック反映**: 高温ストレスパネルの器・夜温/AH/ΔT/連続日数カード・熱帯夜日数表・**visualMap カラースケールの枠とラベル**は `mocks/html/device-show.html`＋`mocks/html/style.css`（正本・**`--color-heat` 新色トークン追加**）へ反映し `make sync-css`（feedback_mock_reflects_impl_visual）。**heatmap/calendar のセル濃淡・トレンド線描画はモック反映の例外**（feedback_mock_graph_rendering_exception＝器・凡例枠は反映対象・濃淡/線は対象外）。

### 5. 熱帯夜年間日数の経年トレンド — ○/多年/配置は design 論点

- 各年の熱帯夜日数を `trend.go` の `MannKendall`＋`SensSlope` でトレンド化（**再利用・新規実装しない**・G-5 ルートA）。**数年では Sen 傾き＋符号＋記述統計に留め「非有意≠トレンド無し」を明示**（G-8）。
- **配置**: 多年・横断は本来「統計分析」ページ（P8 系列＝横断⑬）の領分。本フェーズの device-show ミニパネルに収めるか、P8 系列へ「熱帯夜トレンド」として寄せるかは **design の論点**（既定は device-show に「数年あれば Sen 傾き＋符号のミニ表示」・本格の多年横断は P8 系列へ）。トレンド計算本体は `trend.go` で共通ゆえどちらでも重複は出ない。

## スコープ外（このセッションでやらないこと）

- **派生指標列の DB 追加・マイグレーション（既定）**: THI・AH・熱帯夜・夜温・連続日数・ΔT・年間日数は読み取り時計算で足りる（ガードレール②/⑧）。**高温ストレスしきい値は `domain.Crop` へ Go 定数で非破壊追加**（DB 列なし・§100・作物別が要る場合のみ）。`sensor_readings`/`devices` スキーマ・受信 API（`sensor_api.go`）の変更はしない（goose 00010 のまま・`make db-snapshot` 不要・新規 SQL は SELECT のみで DDL なし）。
- **レアイベントのルートB（希少バイナリのイベント位置総和 S 統計量）＝台風の P11（typhoon-event）**。本フェーズの熱帯夜は**年間日数化できるルートA（年間日数→既存 `trend.go` の MK/Sen）まで**。
- **新規 MK/Sen/Hamed-Rao/ブートストラップ/多重比較の実装**＝**P8 の `trend.go` を再利用**（重複実装禁止）。本フェーズはトレンド統計を新規に書かず既存基盤を呼ぶだけ。
- **新規センサ項目（CO2/照度/土壌水分・体感温度の追加計測）＝フェーズ14（multimetric-integration）**。本フェーズは温湿度のみから THI/AH/熱帯夜を導く。
- **本格的な高温障害の作物別予察モデル**（出穂期高温の不稔率モデル・品質低下の定量モデル・機械学習）＝研究の本務だが**アプリ内では計算しない**（ガードレール⑧で CSV 外出し・P8/P15）。本フェーズは THI/AH の確定式＋熱帯夜の閾値判定・連続ラン＋既存トレンド再利用まで。
- **高温ストレスアラート/通知の発火**（熱帯夜連続/THI 危険帯を検知してメール/LINE 通知）＝本フェーズは**表示（THI heatmap/熱帯夜 calendar/夜温/AH/ΔT/連続日数）まで**。既存のアラート判定（S2 `alert_evaluator`）に高温条件を組み込むことはしない（将来検討）。
- **農家向け平易表示・共有 URL**（信号色の平易な高温警告ダッシュボード）＝フェーズ13（farm-benchmark-share）。本フェーズは研究用詳細表示（ガードレール④）。
- **多地点の熱帯夜/THI 比較（内外/東西・地点横断）＝フェーズ10（multipoint-compare。複数センサ採用が前提）**。本フェーズは単一 device。
- **VPD＝P3／露点・病害＝P6／GDD・収穫予測＝P7／長期トレンド全般・相関＝P8/P9**。本フェーズの派生指標は**THI・AH・熱帯夜・夜温・日較差ΔT（付録A D⑥/D④/A）に限る**。
- **P2〜P8/E1 の既存機能の仕様変更**（統計オーバーレイ・VPD/露点/GDD パネル・品質メタ・トレンドページ・グラフ移行は無回帰維持・消費のみ）。認証・所有者認可・MethodOverride・CSRF・期間バリデーション本体（S1/S5 所有・消費のみ）。device-show 以外の画面（dashboard・readings 等）への高温ストレス展開。

## 技術制約・準拠事項

- **読み取り側のみ・スキーマ非変更（既定）**: マイグレーションを行わない（goose 最新 00010 のまま・`make db-snapshot` 不要）。THI/AH/熱帯夜/夜温/ΔT/年間日数は `ListRecentSensorReadings`（既存・期間生行）や熱帯夜/夜温の年間〜日次集約 SELECT（DDL なし）を Go の純関数で走査して算出。高温ストレスしきい値は作物非依存の既定 or `domain.Crop` の Go 定数（DB 列を増やさない）。
- **計算層と描画層の分離（ガードレール⑧）**: THI/AH/熱帯夜判定/連続ランは純粋 Go（`internal/chart` の `[]float64` 入出力・**time 非依存**）。recorded_at の夜間窓〔日跨ぎ〕・hour-of-day/暦日/年バケットは **handler 境界**で行い純関数には `[]float64` を渡す（`vpdHourlyRows`/`dailyStatRows` 作法の拡張）。重い高温障害統計はアプリで計算しない。
- **数式の確定値（付録A D⑥/D④）**: THI `= 0.8·T + (RH/100)·(T − 14.4) + 46.4`／AH `≈ 217·ea/(T+273.15)`, `ea = es(T)·RH/100`〔kPa→Pa 換算〕。**es(T) は `vpd.go` の `saturationVaporPressure` を再利用**（Tetens 定数を重複定義しない）。RH の [0,100] クランプ・氷点下（−40℃）の数値安全（`T+273.15`≥233.15＞0 でゼロ割なし）。
- **熱帯夜年間日数のトレンドは `trend.go` 再利用（G-5 ルートA・G-2・G-8）**: 各年に1値→`MannKendall`＋`SensSlope`（タイ補正/連続性補正は実装済み）。**新規実装しない**。**数年では Sen 傾き＋符号に留め MK の有意性を断定せず「非有意≠トレンド無し」を明示**（G-8）。
- **物理規約（project_vpd_physics_convention）**: **高温ストレスは暑熱側＝高 THI=暑い=暖色（`--color-heat`）**。熱帯夜 calendar 濃淡・THI visualMap・ストレス帯の符号・「安全〜危険」ラベルの向きを取り違えない。**spec/テストに向きを明記し、実機スモークで目視確認**（テストは仕様前提を符号化するため向きの誤りを捕捉できない）。
- **新 ECharts 系列型（calendar/heatmap/visualMap）**: 本リポジトリ初導入。go-echarts v2.7.2 の型で表現しきれない属性は **P3 `injectVPDMarkArea`／P5 `injectGapMarkArea` の小文字キー自前注入を踏襲**。返り値 JSON は `encoding/json`（SetEscapeHTML=true）で HTML 安全化（既存 option 関数の不変条件を維持）。`EChartsInitializer` の走査/dispose/connect が heatmap/calendar で破綻しないこと。
- **別型隔離**: 高温ストレス option は `HeatStressChartSpec`（別型）で受け、温湿度 `ChartSpec`・VPD/露点/GDD/トレンドの各 Spec の無回帰を守る（P3〜P8 の作法）。
- **依存方向**（structure.md）: 下向き一方向。`internal/chart` 最下流純粋性を維持（gonum/stat・distuv は P8 で導入済み・トレンド再利用時のみ間接利用）。`internal/domain.Crop` の高温ストレス属性は純粋（fmt のみ・§100 Go 定数）。view は repository/service を import せず、domain 表示メソッドのみ参照可。所有者認可は `internal/authz`（`RequireDeviceOwner`）集約・閲覧系の非所有/不在は 404（列挙防止）。
- **イミュータブル**: sqlc 生成構造体は読取専用。handler で View / 暑熱指標結果を組み立てる。`internal/chart` 純関数は入力スライスを破壊しない。
- **マスタ/列挙＝Go 定数**（structure.md §100）: 高温ストレスしきい値を作物別にするなら `domain.Crop` のメソッド/定数（DB に持たないため CHECK 不要・純粋 Go）。9作物の集合・並びは `crop.go`・`devices.crop` CHECK と一致を維持。
- **言語**: 日本語コメント・エラー・コミット・UI ラベル（「THI（温湿度指数）」「熱帯夜」「夜温」「絶対湿度」「除湿負荷」「日較差」「高温ストレス」「連続日数」等）。コード識別子は英語。
- **TDD**: 80% 以上。THI（D⑥ 既知の手計算値一致・RH クランプ・氷点下 NaN/Inf なし）・AH（D④ 既知値・`saturationVaporPressure` 再利用・氷点下ゼロ割なし・RH=0 で AH=0）・熱帯夜判定（夜温=閾値の境界・連続日数ラン＝最長/現在・全日熱帯夜/全日非熱帯夜・単発）・日較差ΔT（`DiurnalRange` 再利用の確認）・年間日数トレンド（`trend.go` 再利用＝MK/Sen を年系列へ・N 小での Sen 傾き＋符号・タイ〔熱帯夜0日の年複数〕）・heatmap/calendar/visualMap option 注入（小文字キー・空データ・暑熱＝暖色の向き）・**物理規約の向き（高温=暖色の濃淡/符号/ラベル）**・所有者認可（非所有→404）・**無回帰**（P3 VPD・P6 露点・P7 GDD パネル・P5 欠測ギャップ・P2 統計オーバーレイ・温湿度2グラフ・期間切替が従来どおり）。

## 受け入れ基準（概略）

1. **THI 計算正当性**: 任意の (温度, 湿度) で THI が付録A D⑥（`0.8·T + (RH/100)·(T − 14.4) + 46.4`）どおり算出され、RH の [0,100] クランプ・氷点下（−40℃）で NaN/Inf を出さない（既知の手計算ケースと一致）。
2. **AH 計算正当性**: AH が付録A D④（`ea = es(T)·RH/100`〔kPa→Pa〕・`AH ≈ 217·ea/(T+273.15)`）どおり算出され、**es(T) は `vpd.go` の `saturationVaporPressure` を再利用**（Tetens 定数を重複定義しない）・RH=0 で AH=0・氷点下でゼロ割しない（既知値一致）。
3. **熱帯夜 calendar ヒートマップ（◎）**: device-show の高温ストレスパネルに、暦日×月の **calendar 座標＋heatmap** で各日が熱帯夜か（夜温≥閾値）・夜温の高さが**暑熱＝暖色の濃淡**で描かれる。go-echarts の型で足りない属性は P3/P5 同型の自前注入で正しく描画される。
4. **THI hour×day heatmap（○）**: 時間帯×日の **heatmap＋visualMap** で THI ストレス度の濃淡が描かれ、高温多湿が集中する時間帯が読める（暑熱＝暖色）。
5. **熱帯夜の連続日数・夜温推移**: 夜温≥閾値が続いた**最長連続・現在連続**日数がカードで示され、夜温（夜間最低/平均）の推移が確認できる。単発・全日熱帯夜・全日非熱帯夜が安全に扱われる。
6. **絶対湿度 AH（除湿負荷）・日較差ΔT**: AH の時系列/カードが描かれ、日較差ΔT が既存 `DiurnalRange` 再利用で日次推移として示される。
7. **熱帯夜年間日数の経年トレンド（○・再利用・留保つき）**: 多年（≥2年）があれば各年の熱帯夜日数を **`trend.go` の `MannKendall`＋`SensSlope` で再利用してトレンド化**（新規実装なし）。**数年では Sen 傾き＋符号＋記述統計に留まり「非有意≠トレンド無し」が明示**される（G-8）。配置（device-show ミニ vs P8 系列）が design で切り分けられている。
8. **物理規約の向き**: 熱帯夜 calendar 濃淡・THI visualMap・ストレス帯の**色・符号・ラベルが「暑熱（暖色）」**で一貫している（project_vpd_physics_convention）。spec/テストに向きが明記され、実機スモーク確認の手順が残る。
9. **スキーマ非変更（既定）**: マイグレーションを行わない（goose 00010 のまま・`make db-snapshot` 不要）。THI/AH/熱帯夜/夜温/ΔT/年間日数はすべて読み取り時計算。高温ストレスしきい値は作物非依存の既定 or `domain.Crop` の Go 定数。
10. **新 ECharts 系列型の動作**: calendar/heatmap/visualMap が `EChartsInitializer`（`[data-echarts]` 走査・dispose・connect）で破綻なく描画・破棄される（連動の要否が design で判断されている）。
11. **無回帰**: P3 VPD・P6 露点・P7 GDD パネル・P5 欠測ギャップ・P2 統計オーバーレイ・温湿度2グラフ・期間切替（24h/3d/7d/30d・URL 同期）・connect 連動・空データ表示が従来どおり動作。所有者でない device への高温ストレスパネル表示は 404。
12. **モック整合**: 高温ストレスパネル器・夜温/AH/ΔT/連続日数カード・熱帯夜日数表・visualMap カラースケールの枠とラベルがモック（device-show.html＋style.css 正本・`--color-heat` 追加）に反映されている（heatmap/calendar のセル濃淡・トレンド線描画は反映例外）。
13. **テスト 80% 以上**。

## 未確定事項・要確認（設計フェーズで決定）

- **① 新 ECharts 系列型（calendar/heatmap/visualMap）の実現方式（最重要・design の最初の論点）**: go-echarts v2.7.2 が calendar 座標系・heatmap 系列・visualMap component をどこまで型で表現できるか。表現しきれない属性（visualMap の `pieces`/`inRange.color`・calendar の `range`/`cellSize`/`orient`）を P3 `injectVPDMarkArea`／P5 `injectGapMarkArea` 同型の自前注入で補う設計。`EChartsInitializer` の `[data-echarts]` 走査・dispose・`echarts.connect` が heatmap/calendar で破綻しないか（連動の要否含む）。
- **② 「夜温」と熱帯夜の定義（ユーザー権威で確定）**: 「夜温≥25℃」の夜温を **日最低気温≥25℃**（気象庁の公式定義に近い）とするか、**夜間時間帯〔例 18:00〜翌06:00〕の最低/平均≥25℃**とするか。夜間窓は JST で日跨ぎ（夕方〜翌朝）するため handler のバケット設計が要る。**沖縄は熱帯夜が非常に多い**（ユーザーの実地知見＝権威）ため定義で日数が大きく変わる。閾値（25℃・真夏日30℃・猛暑日等の追加帯の要否）も含めユーザーに確認。
- **③ THI のストレス帯境界（visualMap の区分）**: THI のストレス度をどの値で区切るか（家畜由来の THI 帯〔例 68/72/80〕は人/施設果菜にそのまま当たらない＝作物・文脈依存で暫定）。visualMap を piecewise（段階色）にするか continuous（連続色）にするか。**作物別の高温好適/障害温度帯はユーザー（研究者本人・前職の糖業課）/文献で確定**。
- **④ 高温ストレスしきい値の `domain.Crop` 設計**: 熱帯夜閾値・THI 帯・高温障害温度帯を作物別にする必要があるか。要るなら `VPDRange()`/`DiseaseModel()`/`GDDModel()` と同型の `HeatStressModel()` を非破壊追加（施設果菜と露地〔サトウキビ/米〕で持つ属性が割れる前提）。当面は作物非依存の既定で足りるか。
- **⑤ 熱帯夜年間日数トレンドの配置（device-show vs 統計分析ページ）**: 多年・横断は本来 P8 系列（横断⑬）。本フェーズの device-show ミニパネル（数年あれば Sen 傾き＋符号）に収めるか、P8 系列へ「熱帯夜トレンド」として寄せるか。トレンド計算本体は `trend.go` で共通ゆえ重複は出ない。**既定は device-show ミニ表示＋本格多年横断は P8 系列**を推奨。
- **⑥ 熱帯夜 calendar の対象期間と粒度**: calendar は年スケール（暦日×月）ゆえ device-show の期間セレクタ（24h/3d/7d/30d）と非連動（GDD パネルが定植日→現在で非連動の前例と同型）。選択中の年/直近1年のどちらを描くか・年の切替 UI の要否。短期（24h/3d）ビューで calendar をどう縮退表示するか（空 or 直近年に固定）。
- **⑦ THI/AH/夜温パネルの見せ方とクラッタ**: 主役は熱帯夜 calendar（◎）＋THI heatmap（○）。夜温推移・AH 時系列・ΔT を同パネルに併置するか legend トグルで畳むか（§2-2 主役1〜2）。AH を line とカードのどちらで見せるか。温湿度グラフとの connect 連動が heatmap/calendar で不自然にならないか。
- **⑧ サンプリング間隔と夜温/THI の集約（5分前提）**: 現行サンプリング5分（`project_sampling_interval_5min_vs_1min`）で夜間窓の最低/平均・hour-of-day の THI 平均が安定して取れるか。欠測日（夜間データ無し）の熱帯夜判定を "—"/非該当のどちらにするか（査読品質＝欠測を熱帯夜0と誤計上しない）。
- **⑨ 物理規約の向き（実機スモーク必須）**: 高温=暑熱=暖色の向きを spec・テスト・色トークン（`--color-heat`）・visualMap カラースケール・「安全〜危険」ラベルに一貫実装し、**GO 判定後に実機スモークで目視確認**する手順を tasks に含める（P3 VPD で向き逆転がテスト全緑のまま実機まで残った前例＝project_vpd_physics_convention）。

--- spec-init 本文 ここまで ---
