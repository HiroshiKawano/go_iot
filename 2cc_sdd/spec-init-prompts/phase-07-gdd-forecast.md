# フェーズ7（分析ロードマップ）spec-init プロンプト: GDD 積算・収穫予測（作物別 Tbase・定植/播種日からの GDD 累積曲線・収穫適期までの残り積算温度・線形回帰による到達日予測・生育ステージ⇔GDD 対応表）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: gdd-forecast
> 位置づけ: [分析アイデアメモ.md](../分析アイデアメモ.md) 第1章「実装ロードマップ」の**フェーズ7**〔明示〕（引継ぎメモではなくスタンプ側だが、付録C-3 の方針で積極採用。GDD は付録A の本命派生指標の一つ）。新規画面ではなく、実装済み **device-show（S5/E1/P2/P3/P5/P6）グラフ**へ、温湿度（日次の最高・最低気温）から導出する**積算温度 GDD（付録A D②）と収穫予測の解析層**を上載せする層＝(1) **作物別 Tbase**（基準温度）、(2) **定植/播種日からの GDD 累積曲線**、(3) **収穫適期までの残り積算温度**、(4) **線形回帰による到達日（収穫適期）予測の外挿**、(5) **作型比較**、(6) **生育ステージ⇔GDD 対応表**。沖縄は冬も温暖で多期作・周年栽培ゆえ GDD の実利が大きく（米の二期作＝GDD の教科書作物・サトウキビ＝前職の糖業課に直結・付録B-4）、「あと何℃で収穫適期か」を予測できる。VPD（P3）／露点（P6）で築いた**派生指標基盤**（作物マスタ `domain.Crop`・別パネル組立の作法・自前 markArea/markLine 注入方式）の直接の延長として、別パネルで GDD・収穫予測を足す。
> 確度: 〔明示〕（分析アイデアメモ 早見表で confidence「明示」＝引継ぎメモに記述あり。ゴールが「生育/収穫予測」なら付録B-3(b) により前倒しされる位置づけ）。
> 前提セッション: **ロードマップ上の論理依存＝作物マスタ（`domain.Crop`＝Tbase・生育ステージのしきい値を持たせる＝横断⑥）・蓄積（収穫予測は要データ＝横断⑦）**。**実装は P3/P5/P6 マージ済みコードへの「上載せ・流用」が前提**:
>  - **P3（vpd-dashboard）** … `internal/domain/crop.go`＝`type Crop`＋9作物＋`VPDRange()`／**P6 が `DiseaseModel()` を非破壊追加済み**。コメントで**「GDD 基準温度・病害モデル等の他属性は別フェーズが非破壊的に追加する前提」**と GDD 用フックを明示している（**本フェーズが Tbase/生育ステージをこの型へ足す＝コメントの GDD 部分を解消する当事者**）。`devices.crop` 列＝00009（CHECK ミラー）。`internal/handler/device_show_vpd.go`＝`buildVPDPanel`〔別パネルを読み取り時に組む作法・`formatVPD`/`formatPercent`/`emptyVPDCard`／時刻が要るバケットは handler 境界〕。`internal/chart/vpd_echarts.go`＝`injectVPDMarkArea`〔小文字 `yAxis` キー自前注入の確立パターン〕。
>  - **P5（data-quality-meta）** … `internal/chart/gap_echarts.go`＝`injectGapMarkArea`〔小文字 `xAxis` 範囲キーの markArea 自前注入〕・`nullableLineData`〔欠測 nil 点で線分断〕。→ **GDD 累積曲線の収穫予測（外挿）は markLine/markPoint の自前注入が要る**（go-echarts の JSON タグ不具合回避＝VPD/gap と同型の「option を JSON 化→マップへ戻し series へ ECharts 準拠キーで注入」）。
>  - **P6（dewpoint-disease-risk）** … `internal/chart/dewpoint.go`〔純粋層を新設する直近の手本〕・`internal/chart/dewpoint_echarts.go`・`internal/handler/device_show_dewpoint.go`＝`buildDewpointPanel`〔別パネル handler の最新の写経元〕・`internal/chart/series.go` の `DewpointChartSpec`〔別型隔離の最新例〕・`--color-dewpoint`〔新色トークン追加の手本〕。
>  - **P2（temp-humidity-chart-stats）** … `internal/chart/stats.go`＝`Mean`/`MinMax`/`SMA`/`StdDev`/`DiurnalRange`・**`dailyStatRows` の JST 暦日バケット作法**〔GDD は日次の最高/最低気温から積算するため、日次バケットが土台〕。**線形回帰（最小二乗）はまだ無い**＝収穫到達日予測のために `stats.go` へ追加する（付録A B 回帰）。
>  - **P4（sensor-data-export）** … `db/queries/sensor_readings.sql` の `ListSensorReadingsInRange`〔BETWEEN・ORDER ASC・LIMIT なし＝期間内全行〕・読み取り時計算・期間内全行スキャンの踏襲元。GDD 累積は**定植日→現在の全期間**（24h-30d の期間セレクタを超える）を走るため、本クエリの全行取得 or **日次集計クエリの新設**が論点。
>  - **S5/E1（device-detail・device-chart-echarts）** … `internal/handler/device_show.go` の `buildChartArea`＝パネルを生行から組み View へ詰める拡張点。`jst`・`statEmptyMark="—"`・`deviceCrop(device)`。
>  - **S4（device-create-edit）** … `internal/view/component/DeviceForm.templ`＝作物 select（`v.Crops`）の隣に**定植/播種日 input を足す拡張点**（P1 locality・P3 crop と同じフォーム拡張作法）。
>  - **S1（web-foundation-auth）** … RequireAuth・所有者認可（`internal/authz`）。
> **スキーマ判断（最重要・このフェーズの設計の核・P6 と決定的に異なる点）**: 露点（P6）や VPD（P3）は2列から「読み取り時計算・スキーマ非変更」で完結したが、**GDD は「定植/播種日」という時間原点（アンカー）を必要とする**。これは温湿度ログから導出できず、ユーザー入力＝**永続化が必要な唯一の新データ**である。`sensor_readings` には無く `devices` にも無い。よって本フェーズは **P1（locality・00008）/P3（crop・00009）と同じ「devices へ単一 nullable 列を非破壊追加」パターンで `devices.planting_date`（DATE・nullable・00010）を足すのが既定の推奨**（マイグレーションを行う＝P2〜P6 の「スキーマ非変更」とは異なる）。一方、**Tbase・生育ステージ⇔GDD のしきい値表は `domain.Crop` へ Go 定数で非破壊追加**（`crop.go` が明示する GDD フック・structure.md §100＝マスタはテーブルでなく Go 定数）＝**DB 列は増やさない**。**作型比較（同一圃場で複数の播種〜収穫サイクルを履歴として並べる）は、本格的には `crop_cycles` テーブル（device_id・作物・定植日・収穫日 等）を要する重い発展部分＝既定スコープ外**（単一 `devices.planting_date` で「現在進行中の1サイクル」に限定し、複数サイクル比較は別 spec/将来へ defer）。この「定植日をどう持つか（単一列 00010／作型テーブル／非永続クエリパラメータ）」が design の最初の論点（未確定事項①）。
> 設計フェーズで参照:
> - 上位ロードマップ: 分析アイデアメモ.md フェーズ7（**作物別 Tbase／定植・播種日からの GDD 累積曲線／収穫適期までの残り積算温度／線形回帰による到達日予測の外挿／作型比較／生育ステージ⇔GDD 対応表／▶ ◎ GDD 累積曲線＋収穫予測（外挿 markLine）**）／ 第2章 §2-2 クラッタ対策（主役1〜2系列・派生指標は**温湿度と別パネル/タブ**）・§2-3 表現テク早見表（**最高/最低/平均/しきい値の印＝markPoint/markLine**・期間ズーム＝dataZoom・**日単位の連続性〔熱帯夜・GDD〕＝calendar 座標＋heatmap も候補**）・§2-4（フェーズ7 ◎主役＝GDD 累積＋収穫予測）／ 第3章ガードレール ⑥作物メタデータの保持（GDD の Tbase は作物依存・無ければ既定挙動）・⑦長期蓄積・保持期間を自前制御（積算温度は長期データ前提）・⑧計算層と描画層の分離（GDD/回帰は軽くアプリ内・重い時系列予測は CSV 外出し or P15）・②スキーマに派生指標列を後付け可能に（ただし**定植日はユーザー入力ゆえ実列追加が要る**＝必要フェーズで足す YAGNI に合致）・④研究用画面と農家共有画面の分離（農家共有＝P13・本フェーズは研究用）／ 付録B-3(b)（生育/収穫予測ゴールなら前倒し）・B-4（**米＝二期作で GDD の教科書作物・出穂予測／サトウキビ＝露地で GDD・日較差・前職の糖業課に直結／施設果菜より露地作物で GDD 本命**）
> - 数式（権威）: 分析アイデアメモ.md 付録A —「**D② 積算温度 GDD**」（**GDD = Σ max( (T_max + T_min)/2 − T_base , 0 )／T_base は作物固有（例: 多くの作物で 10℃）**。収穫日予測・生育ステージ判定に直結）・「**B. 時系列・移動平均系**」の**トレンド検出＝線形回帰の傾き**（GDD 累積 vs 経過日数の回帰で到達日を外挿）・「**A. 基本統計量**」の日較差 ΔT・最高/最低 `T_max`/`T_min`（GDD の日次入力）
> - 現スキーマ（権威）: docs/database_snapshot/table_definitions.md「sensor_readings」（**temperature numeric(5,2)・CHECK −40〜125・recorded_at timestamptz〔計測〕・created_at〔受信〕**・(device_id, recorded_at DESC) 部分索引 WHERE deleted_at IS NULL。日次の最高/最低気温は recorded_at を JST 暦日でバケットして算出）／「devices」（**locality VARCHAR(20)＝P1/00008・crop VARCHAR(20)＝P3/00009 とも追加済**・CHECK `devices_crop_valid` で9作物ミラー。**planting_date 列はまだ無い**＝本フェーズで 00010 追加が既定推奨）。goose 連番の最新は **00009**。
> - 移行元・拡張対象コード（実コード確認済み・P6 マージ後の現状）:
>   - `internal/domain/crop.go` … `type Crop`＋9作物＋`Label`/`Valid`/`VPDRange()`/`DiseaseModel()`/`AllCrops()`。**型コメントが「GDD 基準温度・病害モデル等の他属性は別フェーズが非破壊的に追加する前提」と GDD フックを明示**（P6 が DiseaseModel で病害部分を解消済み・GDD 部分は未解消）。→ **`GDDModel`（または `Tbase()`＋生育ステージ表）を本型へ Go 定数で非破壊追加**（`DiseaseModel` と同じグルーピング作法・`math`/`fmt` のみ依存・DB 列なし）。**米/サトウキビ等の露地作物の Tbase・収穫目標 GDD・生育ステージ閾値**を持たせ、未設定・未定義作物は既定 Tbase（例 10℃）にフォールバック（ガードレール⑥）。値は暫定でユーザー（沖縄実地知見＝権威）/文献で research 確定（`VPDRange`/`DiseaseModel` と同じ「1メソッド更新で確定」作法）。
>   - `internal/chart/stats.go`（P2）… `Mean`/`MinMax`/`SMA`/`StdDev`/`DiurnalRange`/`CV`・内部 `mean`/`popStdDev`/`windowSlice`（純粋・`math` のみ・time 非依存）。**線形回帰（最小二乗の傾き/切片）はまだ無い**。→ **`LinearFit(xs, ys []float64) (slope, intercept float64, ok bool)`（最小二乗）を新設**（付録A B・到達日外挿に使用・分母ゼロ〔x の分散0〕で ok=false）。GDD の日次/累積の純関数は**新設 `internal/chart/gdd.go`** へ（`vpd.go`/`dewpoint.go` の隣・同じ純粋性）。
>   - `internal/chart/vpd_echarts.go`（P3）… `injectVPDMarkArea`＝go-echarts の `MarkAreaData.YAxis` が JSON タグ非準拠（大文字）のため option を JSON 化→マップへ戻し series へ小文字キーで自前注入する確立パターン。→ **GDD 累積＋収穫予測 option は markLine/markPoint の自前注入が要る**（目標 GDD の水平 markLine・予測到達日の垂直 markLine/markPoint）ため、本方式を踏襲した新規 `internal/chart/gdd_echarts.go` を起こす。
>   - `internal/chart/gap_echarts.go`（P5）… `injectGapMarkArea(optionJSON, bands)`＝series[0] へ `xAxis` 範囲 markArea を小文字キーで自前注入。`nullableLineData`〔nil 欠測点〕。→ markLine の自前注入も同型（series へ ECharts 準拠キーで足す）の参考。
>   - `internal/handler/device_show_vpd.go`（P3）／`internal/handler/device_show_dewpoint.go`（P6）… `buildVPDPanel`/`buildDewpointPanel`＝**別パネルを読み取り時に組む作法の手本**（生行→純粋層→`component.*PanelView`・時刻は handler 境界・`formatVPD`/`formatPercent`/`emptyVPDCard` 等の整形ヘルパ）。→ **新設 `internal/handler/device_show_gdd.go` に `buildGDDPanel` を本ファイルの写経で起こす**（GDD 累積カード・残り積算温度・予測到達日・生育ステージ表）。GDD は**定植日＋作物 Tbase の両方が要る**ため、どちらか欠落時は空パネル＋導線注記（「定植日と作物を設定すると GDD を表示」）を返す。
>   - `internal/handler/device_show.go`（S5/E1/P2/P3/P5/P6）… `buildChartArea(ctx,device,period,now)` が生行 → 温湿度 option・統計カード・日次集計・VPD パネル・露点パネル・欠測ギャップを組み `DeviceChartAreaView` を返す。`jst`・`statEmptyMark`・`deviceCrop(device)`。→ **GDD パネルを `buildGDDPanel(...)` で組み View 末尾へ非破壊追加**。**ただし GDD 累積は定植日→現在の全期間（24h-30d 期間セレクタを超え得る）を走る**ため、`buildChartArea` の期間限定生行とは別に**定植日以降の日次データ取得**が要る（クエリ新設 or 全行取得＝design 判断・未確定事項③）。
>   - `internal/chart/series.go` … `ChartSpec`（温湿度）・`VPDChartSpec`・`DewpointChartSpec`（別型隔離の作法）。→ **GDD 専用 `GDDChartSpec`（経過日ラベル・累積 GDD・予測外挿点・目標 GDD・予測到達日 等）を別型で隔離**し温湿度/VPD/露点の無回帰を守る。
>   - `internal/view/component/views.go` … `DeviceChartAreaView`（`VPD VPDPanelView`・`Dewpoint DewpointPanelView`・`HasGap` 等）／各 `*PanelView`/`*CardView`/`*Row`。→ **`GDDPanelView`・`GDDCardView`・生育ステージ行 DTO をイミュータブルに追加**（`DeviceChartAreaView` 末尾へ非破壊追加）。
>   - `internal/view/component/DeviceChartArea.templ` … 期間ボタン＋温湿度2グラフ＋数値カード＋日次集計表＋VPD パネル（`@vpdPanel`・`#vpd-chart`）＋露点パネル（`#dewpoint-chart`）。`App.templ` の `EChartsInitializer`〔`[data-echarts]` 走査→init/dispose・`echarts.connect`〕。→ **GDD パネル（別ブロック・`#gdd-chart` `data-echarts data-unit="℃·日"` 相当・`data-color`）＋GDD カード＋生育ステージ表を追加描画**。
>   - `internal/view/component/DeviceForm.templ`（S4）… `<select name="crop" class="js-tom-select">`（`v.Crops`）。→ **定植/播種日 `<input type="date" name="planting_date">` を作物 select の隣へ追加**（device 登録/編集フォーム・任意・空可）。handler（`internal/handler/device.go` 系）の作成/更新で planting_date を非破壊バインド。
> - ビュー/モック（単一ソース運用の境界）: `internal/view/component/DeviceChartArea.templ`／`DeviceForm.templ`／ `mocks/html/device-show.html`・`mocks/html/device-create.html`・`mocks/html/device-edit.html`＋`mocks/html/style.css`（正本・`--color-vpd`/`--color-dewpoint` の隣に**GDD 用の新色トークン `--color-gdd` を追加**）。**GDD パネルの器・GDD/残り積算温度/予測到達日カード・生育ステージ表・定植日フォーム項目は静的な「器」＝HTML/CSS ゆえモック反映必須**（feedback_mock_reflects_impl_visual・project_css_single_source）。**グラフ内部の GDD 累積曲線/外挿線/markLine 描画は動的描画ゆえモック反映の例外**（feedback_mock_graph_rendering_exception＝器は反映対象・曲線/外挿は対象外）。色は `@layer components`・`:root` トークンへ追記し `make sync-css`。
> - 命名・依存規約: .kiro/steering/structure.md（依存方向＝下向き一方向・`internal/chart` は最下流純粋層〔math のみ・time 非依存〕・view→domain 表示メソッドのみ・§98 外部キー張らない・§99 論理削除・**§100 マスタ/列挙は Go 定数+VARCHAR+CHECK**・所有者認可は `internal/authz` 集約）／ tech.md（sqlc・データアクセス方針）／ CLAUDE.md（**`devices.planting_date` を足すなら expand-contract の後方互換移行（nullable 追加・DROP は down）＋`make db-snapshot` 再生成必須**＝P1/P3 と同じ手順）。
> - 物理規約の補足: GDD は**熱量蓄積指標**（多いほど生育が早い・暑い側＝暖色）で、VPD/露点のような「乾き⇔湿り」の向き取り違え問題（project_vpd_physics_convention）は直接は無い。ただし GDD 累積色・収穫予測 markLine は温度系（暖色）に揃え、温湿度/VPD/露点の既存色と区別する（`--color-gdd`）。

--- spec-init 本文 ここから ---

## 機能概要

蓄積した温湿度計測データ（日次の最高・最低気温）から**積算温度 GDD（Growing Degree Days・付録A D②）と収穫予測の解析層**を device-show（デバイス詳細画面）に上載せし、沖縄の多期作・周年栽培（米の二期作・サトウキビ・露地作物。付録B-4）で「**あと何℃で収穫適期か／いつ収穫適期に達するか**」を予測できるようにする。生の温湿度ではなく、作物の生育を駆動する**積算温度**に変換して語ることで、VPD（P3）・露点（P6）に続く**派生指標ダッシュボード**の第3弾を作る。具体的には、VPD パネル・露点パネルと並ぶ**別パネル（GDD パネル）**として次を追加する。

1. **作物別 Tbase（基準温度）** — 作物ごとの生育下限温度を `domain.Crop` の Go 定数で持ち、日次 GDD 算出に使う（米・サトウキビ等の露地作物が GDD 本命・付録B-4）。未設定・未定義作物は既定 Tbase（例 10℃）にフォールバック（ガードレール⑥）。
2. **定植/播種日からの GDD 累積曲線** — 付録A D②（`GDD = Σ max((T_max + T_min)/2 − T_base, 0)`）で**日次 GDD を定植/播種日から現在まで積算**し、単調増加の累積曲線を描く（◎ 主役）。
3. **収穫適期までの残り積算温度** — 作物の**収穫目標 GDD**（生育ステージ表の最終）から現在の累積 GDD を引いた**残り積算温度**を数値で示す。
4. **線形回帰による到達日（収穫適期）予測の外挿** — 累積 GDD vs 経過日数を**線形回帰**（付録A B・最小二乗）し、傾き（≒1日あたり平均 GDD）から目標 GDD への**到達日を外挿**して予測（◎ 収穫予測＝外挿 markLine）。
5. **生育ステージ⇔GDD 対応表** — 作物の生育ステージ（発芽/開花/収穫 等）と累積 GDD の対応を表で示し、現在どのステージかを示す。
6. **作型比較（限定/△）** — 同一作物の作型（播種時期）違いを比較する素地。**本格的な複数サイクル履歴比較は重い（`crop_cycles` テーブル前提）ため既定スコープ外**とし、本フェーズは「現在進行中の1サイクル（`devices.planting_date`）」に限定する。

**設計の核＝定植日アンカーの永続化（P2〜P6 と決定的に異なる点）**: 露点・VPD は温湿度2列から「読み取り時計算・スキーマ非変更」で完結したが、**GDD は「定植/播種日」という時間原点を必要とし、これは温湿度ログから導出できないユーザー入力＝永続化が要る唯一の新データ**である。よって本フェーズは **P1（locality・00008）/P3（crop・00009）と同じく「devices へ単一 nullable 列を非破壊追加」＝`devices.planting_date`（DATE・nullable・00010）を足すのが既定推奨**（expand-contract＋`make db-snapshot`）。一方、**Tbase・収穫目標 GDD・生育ステージ閾値は `domain.Crop` へ Go 定数で非破壊追加**（`crop.go` が明示する GDD フック・§100）＝**DB 列は増やさない**。S5/E1/P2/P3/P5/P6 の既存機能（温湿度2グラフ・統計オーバーレイ・VPD パネル・露点パネル・欠測ギャップ・期間切替・connect 連動）は**無回帰で維持**する。

> **「GDD 累積は期間セレクタを超える」（重要・スコープ境界）**: device-show の期間切替（24h/3d/7d/30d）は温湿度/VPD/露点グラフの表示窓だが、**GDD 累積は定植日→現在の全期間**を走る（米・サトウキビは生育に数十〜百数十日かかり 30d を容易に超える）。よって GDD パネルは期間セレクタに連動せず、**定植日以降の日次データを独立に取得**する（日次集計クエリの新設 or 全行取得は design 判断）。長期の累積曲線は **dataZoom** で閲覧しやすくする（§2-3）。

> **「線形回帰の外挿は近似」（重要・誤読防止）**: 到達日予測は「累積 GDD が経過日数に対し概ね線形に増える」近似（季節内では妥当だが、季節をまたぐ気温変化で傾きは変動する）。**予測日は確定でなく目安**である旨を UI 注記とし、本格的な季節性込みの時系列予測（Holt-Winters/SARIMA）は P15（forecast-timeseries）へ委ねる（ガードレール⑧＝重い予測は外出し）。

## 背景・現状

P6（dewpoint-disease-risk）マージ後・P3（vpd-dashboard）マージ後の現状は以下（実コード確認済み）。

- **作物マスタと GDD フック**: `internal/domain/crop.go` が `type Crop`＋9作物（goya/ingen/sugarcane/mango/pineapple/uri/rice/imo/leafy_vegetable）＋`Label()`/`Valid()`/`VPDRange()`/`AllCrops()` を持ち、**P6 が `DiseaseModel()` を非破壊追加済み**。型コメントが**「GDD 基準温度・病害モデル等の他属性は別フェーズが非破壊的に追加する前提」**と GDD 用フックを明示しており（病害部分は P6 で解消・**GDD 部分は未解消**）、`VPDRange`/`DiseaseModel` と同じ「作物グルーピング switch＋既定フォールバック＋1メソッド更新で確定」作法で Tbase/生育ステージを足せる。`devices.crop`（00009・CHECK ミラー）。
- **統計純粋層と回帰の不在**: `internal/chart/stats.go` が `Mean`/`MinMax`/`SMA`/`StdDev`/`DiurnalRange`/`CV` を `math` のみ依存・time 非依存で提供。**線形回帰（最小二乗）はまだ無い**＝収穫到達日の外挿のために新設が要る。
- **日次バケットの作法**: P2 の `dailyStatRows`（`device_show.go` 系）が生行を JST 暦日でバケットして日次の最高/最低/平均/日較差を出す作法を確立済み。**GDD は日次の最高/最低気温が入力**ゆえこの日次バケットが土台になる。
- **markArea/markLine 自前注入パターン**: `internal/chart/vpd_echarts.go` の `injectVPDMarkArea`（小文字 `yAxis` キー）・P5 の `injectGapMarkArea`（小文字 `xAxis` キー）が go-echarts の JSON タグ不具合を回避して series へ ECharts 準拠キーで注入する確立パターン。**GDD の目標 GDD 水平 markLine・予測到達日 markLine/markPoint も同型の自前注入が要る**。
- **別パネル組立の作法**: P3 `buildVPDPanel`・P6 `buildDewpointPanel` が、生行から系列・カード・表を組み `component.*PanelView` を返し、`buildChartArea`（`device_show.go`）が温湿度 option/カード/日次/VPD/露点/欠測ギャップに続けて呼び `DeviceChartAreaView` へ詰める。**GDD パネルはこの作法の写経で起こせる**。
- **フォーム拡張の作法**: `DeviceForm.templ` が作物 select（`v.Crops`）を持ち、P1 locality・P3 crop をフォームへ非破壊追加した実績がある。**定植/播種日 input も同作法で足せる**。
- **スキーマ**: `sensor_readings` は temperature/humidity（numeric(5,2)・CHECK −40〜125 / 0〜100）＋recorded_at（計測）＋created_at（受信）。`devices` は locality（P1）/crop（P3）追加済み・**planting_date は無い**。**goose 最新は 00009**。**GDD は日次の最高/最低気温（temperature を JST 暦日でバケット）から導出できるが、定植日アンカーだけは新規永続データが要る**。
- **時刻・タイムゾーン**: JST 固定（`jst`）。time は handler 境界に留め純粋層（chart）へ持ち込まない規約（structure.md）。
- **GDD パネル・累積曲線・収穫予測・生育ステージ表・定植日フォーム項目は device-show / DeviceForm に存在しない**（現状は温湿度グラフ＋統計＋VPD パネル＋露点パネル＋欠測ギャップ＋最新計測テーブル）。

## このセッションのスコープ（実装対象）

### 1. GDD・回帰の純粋層（`internal/chart`・time 非依存）

- **新設 `internal/chart/gdd.go`**（`vpd.go`/`dewpoint.go` の隣・同じ純粋性）に、**`[]float64`/スカラ入出力・`math` のみ依存・time 非依存**の純関数群を追加する。`internal/chart` 最下流純粋層の規約を厳守（gin/DB/templ/pgtype/time を import しない）。
  - **日次 GDD**: `DailyGDD(tMax, tMin []float64, tBase float64) []float64`（付録A D②・各日 `max((tMax+tMin)/2 − tBase, 0)`・負はゼロクランプ）。tMax/tMin は handler が JST 暦日バケットで算出して渡す（純粋層は日付を知らない・`[]float64` のみ）。
  - **累積 GDD**: `CumulativeGDD(daily []float64) []float64`（前方累積和・単調増加・事後条件 `len(out)==len(daily)`）。
  - **残り積算温度**: `RemainingGDD(cumulative []float64, targetGDD float64) float64`（`max(targetGDD − 最新累積, 0)`）。
  - **到達日予測（線形回帰の外挿）**: `internal/chart/stats.go` へ **`LinearFit(xs, ys []float64) (slope, intercept float64, ok bool)`**（最小二乗・付録A B）を新設し、GDD 累積 vs 経過日数（0,1,2,…）で回帰→傾き（≒日あたり平均 GDD）から `targetGDD` への到達日数を外挿する純関数（`ForecastDaysToTarget` 等）。**傾き ≤ 0（生育が進まない）・x の分散 0・既に到達済みは「予測不能/到達済み」を表す形（ok=false や負値）で返す**（呼出側で UI 文言化）。
  - **生育ステージ判定**: 累積 GDD と作物のステージ閾値列（昇順）から現在ステージ index を返す純関数（しきい値は handler が `domain.Crop` から渡す）。
- **重い予測はやらない**: 季節性込みの時系列予測（Holt-Winters/SARIMA/ML）はアプリ内で計算せず P15 へ（ガードレール⑧）。本フェーズは「GDD の確定式＋単純な線形外挿」に集中する。

### 2. 描画層（`internal/chart/gdd_echarts.go`・累積曲線＋収穫予測 markLine）

- **新設 `internal/chart/gdd_echarts.go`** に `GDDChartOptionJSON(spec GDDChartSpec)` を起こす（`vpd_echarts.go`/`dewpoint_echarts.go` の写経）。**series[0]=GDD 累積曲線（主役・単調増加）**＋**収穫予測の外挿線**（破線の第2系列 or markLine）。HTML 安全 JSON（`encoding/json`・SetEscapeHTML=true・`</script>` 不混入）を維持。
- **目標 GDD・予測到達日のマーク**: **目標 GDD の水平 markLine（yAxis）**＋**予測到達日の垂直 markLine/markPoint（xAxis）**を、go-echarts の JSON タグ不具合を避け **P3/P5 の自前注入と同型（option を JSON 化→マップへ戻し series へ ECharts 準拠の小文字キーで `markLine`/`markPoint` を注入）**で実装する。生育ステージ閾値も水平 markLine 群として描けるが、クラッタ回避（§2-2）で主役は累積＋目標＋予測の最小本数に絞る（ステージは表で見せる）。
- **入力契約**: `internal/chart/series.go` に **`GDDChartSpec` を別型で隔離**（経過日ラベル・累積 GDD・予測外挿点・目標 GDD・予測到達日 index 等）。温湿度 `ChartSpec`・`VPDChartSpec`・`DewpointChartSpec` の無回帰を守る。
- **長期データの dataZoom**: GDD 累積は長期（数十〜百数十日）ゆえ dataZoom（inside/slider）で閲覧性を確保（§2-3・design 判断）。

### 3. domain（`internal/domain/crop.go`・GDD モデルの Go 定数追加）

- **`domain.Crop` へ GDD 属性を Go 定数で非破壊追加**（`DiseaseModel` と同じ作法）。`crop.go` の型コメントが明示する GDD フックを解消する。
  - **Tbase**: 作物別の基準温度（米・サトウキビ等の露地作物が本命）。既定 Tbase（例 10℃）にフォールバック。
  - **収穫目標 GDD・生育ステージ閾値**: 生育ステージ（発芽/開花/収穫 等）⇔累積 GDD の対応（昇順 `[]GrowthStage{Name string; GDD float64}` 等）。最終ステージの GDD が収穫目標。
  - 例えば `GDDModel struct { Tbase float64; Stages []GrowthStage }` と `func (c Crop) GDDModel() GDDModel`（未設定・未定義は `DefaultGDDModel` フォールバック・`VPDRange`/`DiseaseModel` と同型）。**値は暫定**でユーザー（沖縄実地知見＝権威）/文献で research 確定（1メソッド更新で確定する作法）。
  - **9作物すべてが GDD 本命ではない**（施設果菜より露地が本命・付録B-4）。**最低 1 作物（例 米：二期作の出穂予測で GDD の教科書作物）で Tbase・生育ステージ・収穫目標が具体値として埋まり、GDD パネルが具体値で描画されること**を下限とする（他作物は既定 or 段階拡張・research）。

### 4. スキーマ（`devices.planting_date`・00010・既定推奨）／クエリ

- **`devices.planting_date`（DATE・nullable）を 00010 マイグレーションで非破壊追加**（P1 locality・P3 crop と同じ expand-contract・down は DROP COLUMN・`make db-snapshot` 再生成）。COMMENT で用途（GDD 積算の起点・NULL=未設定）を明示。**sqlc・`devices` クエリ・DeviceForm バインドを同期**。
- **GDD 用の日次データ取得**: GDD 累積は定植日→現在の全期間ゆえ、`db/queries/sensor_readings.sql` に**定植日以降の日次最高/最低気温集計クエリを新設**（`GROUP BY (recorded_at AT TIME ZONE 'Asia/Tokyo')::date` 等の SELECT・DDL なし）するか、`ListSensorReadingsInRange`（P4）の全行取得を Go で日次集計するかは design（長期×多数行のため SQL 集計が有利か）。**いずれも SELECT のみ＝planting_date 列追加以外の DDL は無い**。

### 5. handler（`internal/handler/device_show_gdd.go`・GDD パネル組立）

- **新設 `internal/handler/device_show_gdd.go`** に `buildGDDPanel(...)` を起こす（`buildVPDPanel`/`buildDewpointPanel` の写経）。定植日以降の日次最高/最低気温＋作物 Tbase/生育ステージから、日次 GDD→累積→残り積算→到達日予測→現在ステージを組み `component.GDDPanelView` を返す。**時刻が要る JST 暦日バケット・経過日数換算は handler 境界**・純粋層は `[]float64`。
- **前提欠落時の扱い**: GDD は**定植日＋作物（Tbase を持つ作物）の両方が要る**。どちらか欠落なら**空パネル＋導線注記**（「作物と定植日を設定すると GDD を表示します」等）を返し、`HasData=false` で templ 側非表示 or 注記表示。計測 0 件も同様。
- **`device_show.go` の `buildChartArea` から呼び出し**、`DeviceChartAreaView` 末尾へ `GDD GDDPanelView` を非破壊追加（VPD/露点パネルと並列）。**GDD は期間セレクタ非連動**（定植日→現在）ゆえ、buildChartArea の period 引数とは独立に定植日アンカーで集計する。
- **GDD カード**: 現在累積 GDD・残り積算温度・予測収穫日（外挿）・現在の生育ステージ・経過日数 等を整形カードで（`VPDCardView`/`DewpointCardView` と同型・`formatStat`/`statEmptyMark` 流用）。予測不能（傾き≤0 等）は "—"＋注記。

### 6. View / templ / モック反映

- `internal/view/component/views.go` に **`GDDPanelView`・`GDDCardView`・生育ステージ行 DTO**をイミュータブルに追加（`DeviceChartAreaView` 末尾へ非破壊追加）。
- `DeviceChartArea.templ` に **GDD パネル（別ブロック・`#gdd-chart` `data-echarts data-color`）＋GDD カード＋生育ステージ⇔GDD 対応表**を露点パネルの下に追加描画。既存クラス（`.summary-grid-*`・`.data-table` 等）を流用し独自クラス新設は最小。
- `DeviceForm.templ` に **定植/播種日 `<input type="date" name="planting_date">`**（任意・空可）を作物 select の隣へ追加。device 登録/編集 handler でバインド・入力値復元（バリデーションエラー時）・編集時の既存値プリセット。
- `App.templ` の `EChartsInitializer` は `[data-echarts]` 走査で GDD line も init される。GDD パネルの connect 参加（GDD は累積℃·日で温度/露点の℃軸とは別スケール・x も経過日でラベルが異なるため**connect は外す**のが自然か＝design 判断）。
- **モック反映**: GDD パネルの器・GDD/残り積算温度/予測収穫日カード・生育ステージ表は `mocks/html/device-show.html`＋`mocks/html/style.css`（正本・**`--color-gdd` 新色トークン追加**）へ、定植日フォーム項目は `mocks/html/device-create.html`/`device-edit.html` へ反映し `make sync-css`（feedback_mock_reflects_impl_visual）。**グラフ内部の GDD 累積曲線/外挿線/markLine 描画はモック反映の例外**（feedback_mock_graph_rendering_exception）。

### 7. 作型比較・収穫実績突合 — △/限定スコープ/未確定

- **作型比較**はメモで挙がるが、同一圃場の複数サイクル（播種〜収穫を履歴で並べる）には `crop_cycles` テーブル（device_id・作物・定植日・収穫日 等）が要る重い発展部分。**既定スコープからは外し**、本フェーズは `devices.planting_date` の「現在進行中の1サイクル」に限定する。作型テーブルを起こすかは design の論点（既定は起こさず単一サイクル）。
- **収穫実績の記録・予測精度の事後検証**（実収穫日を入力し予測と突合）も永続化を要する発展部分で**既定スコープ外**（P6 の発病記録と同じ位置づけ・将来）。

## スコープ外（このセッションでやらないこと）

- **定植日アンカー以外の DB スキーマ拡張**: GDD・累積・残り積算・到達日予測・生育ステージは日次気温＋Go 定数（Tbase/ステージ）から読み取り時計算で足りる。**新規永続データは `devices.planting_date`（00010）の単一列のみ**（P1/P3 と同型）。`sensor_readings`/受信 API（`sensor_api.go`）・CHECK の変更はしない。**作型テーブル（`crop_cycles`）・収穫実績テーブルの新設は既定スコープ外**（採るなら design で根拠を明示し expand-contract＋スナップショット再生成）。
- **本格的な時系列予測**（Holt-Winters/SARIMA/ML による季節性込みの収穫予測）＝フェーズ15（forecast-timeseries）。本フェーズは**単純な線形回帰の外挿**（付録A B）まで＝目安としての到達日予測。
- **気温の将来補完なしの外挿**: 予測は「過去の累積トレンドの線形外挿」であり、**未来の気温予報を取り込んだ精緻な GDD 予測はしない**（外部気象データ統合は将来）。
- **作物別 Tbase・生育ステージ・収穫目標 GDD の全作物確定**: 9作物すべての確定値を埋めるのは research の領域。本フェーズは**最低 1 作物（米等の露地 GDD 本命）で具体値が描画されること**を下限とし、他作物は既定 Tbase フォールバック or 段階拡張（値の確定はユーザー＝研究者本人/文献）。
- **農家向け平易表示・共有 URL**（信号色の収穫カウントダウン等）＝フェーズ13（farm-benchmark-share）。本フェーズは研究用詳細表示（ガードレール④）。
- **THI・熱帯夜・絶対湿度 AH**＝フェーズ12（heat-stress-thi）。**露点・病害**＝フェーズ6。**VPD**＝フェーズ3。本フェーズの派生指標は**積算温度 GDD（付録A D②）と到達日予測（付録A B 回帰）に限る**。
- **多地点の GDD 比較（内外/東西・圃場間）**＝フェーズ10（multipoint-compare。複数センサ採用が前提）。本フェーズは単一 device。
- **収穫適期アラート/通知**（到達予測日が近づいたらメール/LINE）＝本フェーズは**表示（累積曲線/残り積算/予測日/ステージ）まで**。既存のアラート判定（S2）への組み込みはしない（将来検討）。
- **P2/P3/P4/P5/P6/E1 の既存機能の仕様変更**（統計オーバーレイ・VPD/露点パネル・CSV・品質メタ・グラフ移行は無回帰維持・消費のみ）。認証・所有者認可・MethodOverride・CSRF・期間バリデーション本体（S1/S5 所有・消費のみ）。device-show 以外の画面（dashboard・readings 等）への GDD 展開（ただし DeviceForm への定植日項目追加は device 登録/編集に必要）。

## 技術制約・準拠事項

- **スキーマ＝planting_date 単一列のみ追加（既定）**: `devices.planting_date`（DATE・nullable・00010）を expand-contract で非破壊追加（P1/P3 と同手順・down は DROP・`make db-snapshot` 再生成）。それ以外は読み取り時計算（GDD・累積・回帰・ステージ）と Go 定数（Tbase/ステージ＝`domain.Crop`）で足す。日次気温は `recorded_at` を JST 暦日でバケット（クエリ集計 or Go 集計）。
- **計算層と描画層の分離（ガードレール⑧）**: GDD/累積/残り積算/回帰/ステージ判定は純粋 Go（`internal/chart` の `[]float64` 入出力・**time 非依存**）。recorded_at の日次バケット・経過日数換算は **handler 境界**で行い純関数には `[]float64`/スカラを渡す（`dailyStatRows` 作法）。重い時系列予測はアプリで計算しない（P15）。
- **数式の確定値（付録A D②/B）**: 日次 GDD `max((T_max + T_min)/2 − T_base, 0)`（負はゼロクランプ）／累積＝前方和／到達日＝累積 vs 経過日数の最小二乗回帰の傾きから外挿。**Tbase は作物固有・既定 10℃**（例）。氷点下・全ゼロ（生育せず）・単一日・傾き≤0・既に到達済みを数値安全に扱う。
- **markLine/markPoint の自前注入**: 目標 GDD（水平）・予測到達日（垂直）は go-echarts の JSON タグ不具合を避け **P3/P5 の小文字キー自前注入と同型**で実装。返り値 JSON は `encoding/json`（SetEscapeHTML=true）で HTML 安全化（既存 option 関数の不変条件を維持）。
- **別型隔離**: GDD option は `GDDChartSpec`（別型）で受け、温湿度 `ChartSpec`・`VPDChartSpec`・`DewpointChartSpec` の無回帰を守る（P3/P6 の作法）。
- **依存方向**（structure.md）: 下向き一方向。`internal/chart` 最下流純粋性を維持。`internal/domain.Crop` の GDD モデルは純粋（math/fmt のみ・§100 Go 定数）。view は repository/service を import せず、domain 表示メソッドのみ参照可。所有者認可は `internal/authz`（`RequireDeviceOwner`）集約・閲覧系の非所有/不在は 404（列挙防止）。
- **イミュータブル**: sqlc 生成構造体は読取専用。handler で View / GDD 結果を組み立てる。`internal/chart` 純関数は入力スライスを破壊しない。
- **マスタ/列挙＝Go 定数**（structure.md §100）: Tbase・生育ステージ・収穫目標 GDD は `domain.Crop` のメソッド/定数で持つ（DB に持たないため CHECK 不要・純粋 Go）。9作物の集合・並びは P3 の `crop.go`・`devices.crop` CHECK と一致を維持。
- **言語**: 日本語コメント・エラー・コミット・UI ラベル（「積算温度」「GDD」「残り積算温度」「収穫予測日」「生育ステージ」等）。コード識別子は英語。
- **TDD**: 80% 以上。日次 GDD（`(Tmax+Tmin)/2−Tbase` の正負・ゼロクランプ・Tbase 境界）・累積（単調増加・前方和・空入力）・残り積算（到達前/到達済み=0）・線形回帰 `LinearFit`（既知の手計算値一致・x 分散0で ok=false・傾き0/負）・到達日外挿（傾き≤0/到達済み/通常）・生育ステージ判定（閾値境界・最終ステージ超え・空ステージ）・GDD markLine 注入（小文字キー・目標/予測の本数）・`domain.Crop.GDDModel`（米等で具体値・未定義で既定フォールバック・Tbase 既定）・planting_date バインド（空可・不正日付）・所有者認可（非所有→404）・**無回帰**（P3 VPD・P6 露点・P5 欠測ギャップ・P2 統計オーバーレイ・温湿度2グラフ・期間切替が従来どおり）。

## 受け入れ基準（概略）

1. **GDD 計算正当性**: 任意の日次 (Tmax, Tmin, Tbase) で日次 GDD が付録A D②（`max((Tmax+Tmin)/2 − Tbase, 0)`・負はゼロ）どおり算出され、累積が単調増加の前方和になる（既知の手計算ケースと一致・全ゼロ/単一日/氷点下が安全）。
2. **GDD 累積曲線＋収穫予測（◎）**: device-show の GDD パネルに定植/播種日からの**累積 GDD 曲線**が描かれ、**目標 GDD の水平 markLine**と**線形外挿による予測到達日の markLine/markPoint**が表示される。go-echarts JSON タグ不具合を避けた自前注入で正しく描画される。**予測が近似（線形外挿・季節性なし）で目安である旨が UI/仕様に明示**される。
3. **残り積算温度・予測収穫日**: 収穫目標 GDD から現在累積を引いた**残り積算温度**と、累積 vs 経過日数の回帰傾きから外挿した**予測収穫日**がカードで確認でき、傾き≤0/到達済み等の予測不能ケースは "—"＋注記で安全に扱われる。
4. **作物別 Tbase・生育ステージ**: `domain.Crop` の GDD モデルから Tbase・生育ステージ・収穫目標 GDD が解決され、**未設定・未定義作物は既定 Tbase にフォールバック**。**最低 1 作物（米等の露地 GDD 本命）で具体値が描画**され、生育ステージ⇔GDD 対応表に現在ステージが示される。
5. **定植日アンカーの永続化**: `devices.planting_date`（00010・nullable）が device 登録/編集フォームで入力・保存・復元でき、GDD 累積の起点に使われる。**定植日 or 作物（Tbase 保有）が欠落のときは空パネル＋導線注記**（設定を促す）になる。
6. **期間セレクタ非連動**: GDD 累積が device-show の期間切替（24h/3d/7d/30d）に縛られず**定植日→現在の全期間**を描き、長期データを dataZoom で閲覧できる。
7. **スキーマは planting_date 単一列のみ**: 追加 DDL は `devices.planting_date`（00010・expand-contract・`make db-snapshot` 再生成）のみ。GDD/累積/回帰/ステージ/Tbase は読み取り時計算＋Go 定数。作型/収穫実績テーブルは採らない（採る場合のみ design で根拠明示）。
8. **無回帰**: P3 VPD パネル・P6 露点パネル・P5 欠測ギャップ/品質メタ・P2 統計オーバーレイ・温湿度2グラフ・期間切替（URL 同期）・connect 連動・空データ表示が従来どおり動作。所有者でない device への GDD パネル表示は 404。
9. **モック整合**: GDD パネル器・GDD/残り積算温度/予測収穫日カード・生育ステージ表・定植日フォーム項目がモック（device-show.html/device-create.html/device-edit.html＋style.css 正本・`--color-gdd` 追加）に反映されている（グラフ内部の累積曲線/外挿/markLine 描画は反映例外）。
10. **テスト 80% 以上**。

## 未確定事項・要確認（設計フェーズで決定）

- **① 定植/播種日の持ち方（最重要・design の最初の論点）**: 既定推奨は **`devices.planting_date` 単一 nullable 列（00010・P1/P3 と同型）＝現在進行中の1サイクル**。代替は (a) **非永続のクエリパラメータ/フォーム（期間切替のように URL 同期するが保存しない）**＝マイグレーション不要だがリロードで消え累積/残り積算/到達日が安定しない（**非推奨**＝GDD は安定アンカーが要る）／(b) **`crop_cycles` テーブル（device_id・作物・定植日・収穫日 等）＝複数サイクルの作型比較が可能だが重い（既定スコープ外・将来）**。**(a) より単一列を強く推奨**し、作型比較の本格対応は別 spec へ defer する方針を確定する。
- **② Tbase・収穫目標 GDD・生育ステージ閾値の値**: 作物別の Tbase（米・サトウキビ等の露地が本命・付録B-4）・生育ステージ（発芽/出穂/開花/収穫 等）⇔累積 GDD の対応・収穫目標 GDD。**沖縄の作物の実値はユーザー（研究者本人＝権威・前職の糖業課でサトウキビ実務）/文献で確定**。本フェーズで埋める作物の優先（米＝二期作の出穂予測で GDD の教科書作物を第一候補）。
- **③ GDD 用日次データの取得方式**: 定植日→現在の日次最高/最低気温を、**(a) 日次集計クエリ新設（`GROUP BY ::date`・長期×多数行に有利）** か **(b) `ListSensorReadingsInRange` 全行取得＋Go 日次集計（P4 流用）** か。長期生育（数十〜百数十日×5分間隔）の行数を踏まえて design 判断（いずれも SELECT のみ・DDL は planting_date のみ）。
- **④ GDD の上限カットオフ（modified GDD）**: 日次平均が作物の生育上限温度を超える分を頭打ちにする「modified GDD（上限キャップ）」を入れるか、単純 GDD（下限クランプのみ）に留めるか。沖縄の高温（夏季）で上限超過が起き得るため要検討（既定は単純 GDD・上限は research/将来）。
- **⑤ 線形回帰の窓**: 到達日外挿の回帰を**全累積データ**で取るか**直近 N 日**で取るか（季節変化で傾きが変わるため直近の方が当たる場合がある）。外挿の妥当性・予測のブレ幅（信頼区間まで出すか）。本格化は P15 へ。
- **⑥ 予測不能/異常ケースの UI 文言**: 傾き≤0（寒くて GDD が増えない）・データ不足（経過日数が少ない）・既に目標到達済み・定植日が未来日 等のときのカード表示と注記（"—"＋理由）。定植日が未来/将来日付のバリデーション。
- **⑦ GDD パネルの見せ方と connect**: GDD は累積℃·日で温度/露点の℃軸・x の経過日ラベルとも異なるため、**温湿度グラフとの `echarts.connect` は外す**のが自然か（軸が違うと axisPointer 連動が無意味）。生育ステージ閾値を markLine 群で描くか表のみに留めるか（クラッタ回避・§2-2）。期間別（24h は GDD 不向き）で GDD パネルの粒度/表示要否を変えるか。
- **⑧ 作型比較の限定範囲**: 単一 `planting_date` で「今期の累積」に限定する前提で、作型比較の最小の見せ方（過去サイクルなしでも作物別の標準曲線/目安を重ねるか）。本格比較（複数サイクル履歴）は ① の `crop_cycles` 採否と連動＝既定は defer。

--- spec-init 本文 ここまで ---
