# フェーズ6（分析ロードマップ）spec-init プロンプト: 露点・病害リスク（露点Td時系列・結露帯markArea・葉面湿潤時間の日次積算・高湿度継続イベント抽出/一覧・病害スコア下地）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: dewpoint-disease-risk
> 位置づけ: [分析アイデアメモ.md](../分析アイデアメモ.md) 第1章「実装ロードマップ」の**フェーズ6**〔強く示唆〕。新規画面ではなく、実装済み **device-show（S5/E1/P2/P3/P5）グラフ**へ、温湿度から導出する**露点 Td（付録A D③）と病害リスクの蓄積解析層**を上載せする層＝(1) **露点 Td 時系列**（気温との接近＝**結露帯**を markArea でハイライト）、(2) **葉面湿潤時間（高湿度継続）の日次積算**→病害スコアの下地、(3) **高湿度継続イベントの抽出・一覧**、(4) 発病記録と環境の事後突合に使えるデータ提示。梅雨・台風・スコールで病害圧が高い沖縄で、病害予察（研究センターの本務・付録B-1 研究者/公務員の顧客像）に直結する。VPD（P3）で築いた**派生指標基盤**（純粋層 `internal/chart/vpd.go`・markArea 注入方式・VPD パネル組立の作法・作物マスタ `domain.Crop`）の直接の延長として、別パネルで露点・病害を足す。
> 確度: 〔強く示唆〕（引継ぎメモの確定依頼ではなくスタンプ側の仮説だが、**提案型案件ゆえ積極採用**＝付録C-3 の方針）。ゴールが「リアルタイム警告（結露・病害）」なら付録B-3(a) により前倒しされる位置づけ。
> 前提セッション: **ロードマップ上の論理依存＝P3（vpd-dashboard。派生指標の純粋層＋作物マスタ＝本フェーズの直接の土台）・蓄積（病害解析は要データ）**。**実装は P3/P5 マージ済みコードへの「上載せ・流用」が前提**: **P3（vpd-dashboard。`internal/chart/vpd.go`＝Tetens 定数 `tetensB=17.27`/`tetensC=237.3`〔露点 γ 式で再利用〕・`saturationVaporPressure`・`VPDSeries`・`TimeInRange`／`internal/chart/vpd_echarts.go`＝`injectVPDMarkArea`〔小文字 `yAxis` キー自前注入の確立パターン〕／`internal/handler/device_show_vpd.go`＝`buildVPDPanel`〔別パネルを読み取り時に組む作法・`vpdHourlyRows` の hour-of-day バケット・`formatVPD`/`formatPercent`〕／`internal/domain/crop.go`＝`type Crop`＋9作物＋`VPDRange()`〔**病害モデル属性をこの型へ非破壊追加する明示済みフック**〕／`devices.crop` 列＝00009）／ P5（data-quality-meta。`internal/chart/gap_echarts.go`＝`injectGapMarkArea`〔**小文字 `xAxis` 範囲キーの markArea 自前注入＝結露帯〔時間区間ハイライト〕の直接の前例**〕・`nullableLineData`〔欠測 nil 点〕・`internal/chart/quality.go`＝`StuckRuns`/`RapidChanges`〔連続ラン/急変＝イベント検出の素〕）／ P2（temp-humidity-chart-stats。`internal/chart/stats.go`＝`Mean`/`MinMax`/`SMA`/`StdDev`・`dailyStatRows` の JST 暦日バケット作法〔葉面湿潤時間の日次積算に流用〕）／ P4（sensor-data-export。読み取り時計算・期間内全行スキャンの踏襲元かつ「結露時間列」の後追い対象）／ S5/E1（device-detail・device-chart-echarts。`internal/handler/device_show.go` の `buildChartArea`＝パネルを生行から組み View へ詰める拡張点）／ S1（web-foundation-auth。RequireAuth・所有者認可）**
> **スキーマ判断（最重要・このフェーズの設計の核）**: 露点 Td・露点スプレッド（T−Td）・結露帯・葉面湿潤時間・高湿度継続イベントは**すべて既存 `sensor_readings`（temperature/humidity の2列）から読み取り時に計算できる**（露点は付録A D③の確定式・葉面湿潤/イベントは RH しきい値の連続ラン）。よって **P2/P3/P4/P5 と同じく「読み取り時計算・スキーマ非変更」を既定方針とする**（ガードレール②＝派生列は必要になったフェーズで足す YAGNI／⑧＝軽い計算はアプリ内）。**病害モデルのしきい値（作物別の温度帯×葉面湿潤時間）は `domain.Crop` へ Go 定数で非破壊追加**（`crop.go` の「GDD 基準温度・病害モデル等の他属性は別フェーズが非破壊的に追加する前提」コメントが明示する拡張点・structure.md §100＝マスタはテーブルでなく Go 定数）＝**DB 列は増やさない**（goose 最新 **00009** のまま・`make db-snapshot` 不要）。**唯一スキーマ追加を要するのは「発病記録（ユーザーが手入力する実観測）を永続化して環境と事後突合する」場合のみ**で、これは付録の ▶ で △（蓄積前提・作物モデル前提）とされる発展部分＝**本フェーズの確定スコープからは外し、表 `disease_observations` を起こすかは design の論点（既定は起こさない＝環境側の risk 提示まで）**。この方針確定が design の最初の論点（未確定事項①）。
> **物理規約（最重要・project_vpd_physics_convention を厳守）**: 結露・葉面湿潤・高湿度は VPD の「湿り側＝寒色」と同じ向きに揃える（乾き側＝暖色と取り違えない）。詳細・教訓（P3 VPD で向き逆転がテスト全緑のまま実機スモークまで残った前例）と実機スモーク目視確認の要請は機能概要内の警告（下記）に集約する。
> 設計フェーズで参照:
> - 上位ロードマップ: 分析アイデアメモ.md フェーズ6（**露点時系列〔気温との接近＝結露帯〕／葉面湿潤時間（高湿度継続）の日次積算→病害スコア／高湿度継続イベントの抽出・一覧／発病記録と環境の事後突合用データ蓄積／▶ ◎ 露点＋結露帯 markArea・△ 病害スコア〔作物モデル・蓄積前提〕**）／ 第2章 §2-2 クラッタ対策（主役1〜2系列・派生指標は**温湿度と別パネル/タブ**・線より「帯」で見せる）・§2-3 表現テク早見表（**固定しきい値/時間区間の帯＝markArea**・期間ズーム＝dataZoom）・§2-4（フェーズ6 ◎主役＝露点＋結露帯／△＝病害スコア）／ 第3章ガードレール ④研究用画面と農家共有画面の分離（農家共有＝P13・本フェーズは**研究用**）・⑥作物メタデータの保持（病害モデルは作物依存・無ければ既定挙動）・⑧計算層と描画層の分離（軽い露点/イベント検出はアプリ内・重い統計は CSV 外出し）・②スキーマに派生指標列を後付け可能に（実列追加は必要フェーズで＝YAGNI）／ 付録B-3(a)（リアルタイム警告ゴールなら前倒し）・B-1（病害予察＝研究センターの本務）
> - 数式（権威）: 分析アイデアメモ.md 付録A —「**D③ 露点温度・結露リスク**」（**γ = ln(RH/100) + 17.27·T/(T+237.3)／露点 Td = 237.3·γ/(17.27 − γ) [℃]／結露リスク: 葉面温度 ≦ Td のとき発生**。定数 17.27・237.3 は Tetens 式と共通＝`vpd.go` の `tetensB`/`tetensC` を再利用）・「**D⑤ 病害リスクモデル**」（**「温度帯 × 葉面湿潤時間」で発病確率が上がる作物が多い＝葉面湿潤時間（高湿度継続時間）を積算してリスクスコア化**）・「**E. 異常検知**」（**急変検出〔変化率（微分）閾値超え〕・センサー固着〔stuck/flatline〕**＝高湿度継続イベント抽出に隣接する手法。`quality.go` の `StuckRuns`/`RapidChanges` が同型の連続ラン/差分検出）
> - 現スキーマ（権威）: docs/database_snapshot/table_definitions.md「sensor_readings」（**temperature/humidity numeric(5,2)・CHECK 温度 −40〜125・湿度 0〜100・recorded_at timestamptz〔計測〕・created_at〔受信〕**・(device_id, recorded_at DESC) 部分索引 WHERE deleted_at IS NULL。露点は temperature＋humidity から導出でき**スキーマ非変更**で足る）／「devices」（**locality VARCHAR(20)＝P1/00008・crop VARCHAR(20)＝P3/00009 とも追加済**・CHECK `devices_crop_valid` で9作物ミラー）。goose 連番の最新は **00009**。
> - 移行元・拡張対象コード（実コード確認済み・P5 マージ後の現状）:
>   - `internal/chart/vpd.go` … 派生指標の純粋層（`math` のみ依存・`[]float64`/スカラ入出力・gin/DB/templ/**time** 非依存）。**Tetens 定数 `tetensA=0.6108`/`tetensB=17.27`/`tetensC=237.3`**・`saturationVaporPressure(tempC)`・`VPD`/`VPDSeries`・`TimeInRange(values,lower,upper)`。→ **露点 Td の純関数（`DewPoint(temp,rh)`／系列版 `DewPointSeries`）・露点スプレッド（T−Td）・結露帯判定・葉面湿潤時間積算・高湿度継続イベント抽出**を、本ファイル隣の **新設 `internal/chart/dewpoint.go`** へ追加（`tetensB`/`tetensC` 再利用・`TimeInRange`/`Mean`/`MinMax` 流用・純粋・time 非依存を厳守）。**γ の `ln(RH/100)` は RH=0 で −Inf となる**ため RH 下限クランプ（防御）必須＝CHECK で 0 は許容されるため。
>   - `internal/chart/vpd_echarts.go` … `VPDChartOptionJSON(spec VPDChartSpec)`＝VPD 折れ線＋適正帯3ゾーン markArea＋VPD移動平均。**`injectVPDMarkArea`＝go-echarts の `MarkAreaData.YAxis` が JSON タグ非準拠（大文字 `YAxis`）のため option を JSON 化→マップへ戻し series[0] へ ECharts 準拠の小文字 `yAxis` キーで markArea を自前注入する確立パターン**。→ 露点パネルの option（露点 Td 線・気温 line の重ね・**結露帯**）は本方式を踏襲した新規 `internal/chart/dewpoint_echarts.go` として起こす。
>   - `internal/chart/gap_echarts.go`（P5）… `injectGapMarkArea(optionJSON, bands)`＝**series[0] へ連続区間の `xAxis` 範囲 markArea を小文字 `xAxis` キーで自前注入する確立パターン**（VPD の yAxis 注入を xAxis へ展開済み）・`nullableLineData([]*float64)`＝nil 欠測点で線分断・`gapZone(startIdx,endIdx)`・`GapBand`。→ **結露帯（時間区間のハイライト）は xAxis 範囲指定ゆえ `injectGapMarkArea` と同型＝本フェーズはこの xAxis 注入を結露帯色で再利用/汎用化する**（gap の灰色に対し結露帯は寒色＝湿り側）。
>   - `internal/chart/quality.go`（P5）… `StuckRuns(values,minRun)`〔同値連続ラン〕・`RapidChanges(values,maxDelta)`〔隣接差の急変〕・`MissingStats`〔欠測区間 `GapSpan`〕・内部 `median`/`round2`。→ **高湿度継続イベント抽出**（RH≧しきい値の連続ラン＝開始/終了/継続長）は `StuckRuns` の連続ラン検出と同型の純関数を新設（しきい値・最小継続を引数化）。
>   - `internal/handler/device_show.go`（S5/E1/P2/P3/P5）… `buildChartArea(ctx,device,period,now)` が生行 `ListRecentSensorReadings` → 温湿度 float 列＋ラベル列に整形 → 温湿度 option（`ChartOptionJSON`）・統計カード・日次集計（`dailyStatRows`）・**`buildVPDPanel(...)` で VPD パネル**を組み `DeviceChartAreaView` を返す。欠測ギャップは `applyGapGrid`（P5）。`jst`・`smaWindowFor(period)`・`statEmptyMark="—"`・`deviceCrop(device)`。→ **露点パネルを `buildVPDPanel` と同様に `buildDewpointPanel(...)` で組み View へ詰める拡張点**（時刻が要る hour-of-day/日次バケットは handler 境界で・純粋層は `[]float64`）。
>   - `internal/handler/device_show_vpd.go`（P3）… `buildVPDPanel(labels,temps,hums,rows,crop,period)`＝**別パネルを読み取り時に組む作法の手本**。`vpdHourlyRows`〔JST hour-of-day バケット〕・`vpdMaxDeviation`〔逸脱量+方向・**VPD 物理規約の符号付け**〕・`formatVPD`/`formatPercent`/`emptyVPDCard`。→ **新設 `internal/handler/device_show_dewpoint.go` に `buildDewpointPanel` を本ファイルの写経で起こす**（露点カード・結露帯・葉面湿潤時間・高湿度イベント一覧）。
>   - `internal/chart/series.go` … `ChartSpec`（温湿度 line・末尾に P5 の `RawNullable`/`GapBands` 非破壊追加）・`VPDChartSpec`（VPD 専用に隔離）・`GapBand`。→ **露点専用 `DewpointChartSpec`（露点 Td・気温・結露帯区間 等）を別型で隔離**し温湿度/VPD の無回帰を守る（P3 が VPDChartSpec を別型にした作法）。
>   - `internal/view/component/views.go` … `DeviceChartAreaView`（温湿度 option/カード/日次・`VPD VPDPanelView`・`HasGap`）／ `VPDPanelView`（OptionJSON/Color/CropLabel/Lower-UpperLabel/Card/InRangeRatio/Hourly）／ `VPDCardView`／ `VPDHourlyRow`。→ **`DewpointPanelView`・`DewpointCardView`・葉面湿潤/イベント行 DTO をイミュータブルに追加**（`DeviceChartAreaView` 末尾へ非破壊追加）。
>   - `internal/view/component/DeviceChartArea.templ` … 期間ボタン＋温湿度2グラフ＋数値カード＋日次集計表＋**VPD パネル**（`@vpdPanel`・`#vpd-chart` `data-echarts data-unit="kPa" data-color`）。`internal/view/layout/App.templ` の `EChartsInitializer`〔`[data-echarts]` 走査→init/dispose・`echarts.connect`〕。→ **露点パネル（別ブロック・`#dewpoint-chart`）と葉面湿潤/高湿度イベント表を追加描画**。器（パネル枠/カード/表）はモック準拠・独自クラス新設は最小。
> - ビュー/モック（単一ソース運用の境界）: `internal/view/component/DeviceChartArea.templ`／ `mocks/html/device-show.html`＋`mocks/html/style.css`（正本・`--color-vpd:#0ca678` の隣に**露点用の新色トークン `--color-dewpoint` を追加**）。**露点パネルの器・露点数値カード・葉面湿潤時間/病害スコアの枠・高湿度継続イベント表は静的な「器」＝HTML/CSS ゆえモック反映必須**（feedback_mock_reflects_impl_visual・project_css_single_source）。**グラフ内部の露点線/気温重ね/結露帯 markArea 描画は動的描画ゆえモック反映の例外**（feedback_mock_graph_rendering_exception＝器は反映対象・線/帯は対象外）。色は `@layer components`・`:root` トークンへ追記し `make sync-css`。
> - 命名・依存規約: .kiro/steering/structure.md（依存方向＝下向き一方向・`internal/chart` は最下流純粋層〔math のみ・time 非依存〕・view→domain 表示メソッドのみ・§98 外部キー張らない・§99 論理削除・**§100 マスタ/列挙は Go 定数+VARCHAR+CHECK**・所有者認可は `internal/authz` 集約）／ tech.md（sqlc・データアクセス方針）／ CLAUDE.md（**既定はマイグレーション無し**＝`make db-snapshot` 不要。発病記録テーブルを design で採るなら expand-contract の後方互換移行＋スナップショット再生成必須）。

--- spec-init 本文 ここから ---

## 機能概要

蓄積した温湿度計測データから**露点 Td と病害リスクの蓄積解析層**を device-show（デバイス詳細画面）に上載せし、梅雨・台風・スコールで病害圧が高い沖縄の**病害予察**（研究センターの本務・付録B-1）を支える。生の温湿度ではなく、結露・葉面湿潤という植物病理の駆動因に変換して語ることで、VPD（P3）に続く**派生指標ダッシュボード**の第2弾を作る。具体的には、P3 の VPD パネルと並ぶ**別パネル（露点パネル）**として次を追加する。

1. **露点 Td 時系列**（付録A D③・`Td = 237.3·γ/(17.27 − γ)`, `γ = ln(RH/100) + 17.27·T/(T+237.3)`）を折れ線で描き、**気温 T を重ね描き**して「T が Td に接近するほど結露しやすい」関係を可視化する。
2. **結露帯のハイライト** — 気温が露点に十分接近した（＝結露しやすい）時間区間を **markArea（xAxis 範囲指定）で帯ハイライト**する（メモ フェーズ6 の ◎）。**葉面温度センサが無いため葉面温度は気温で近似**し、結露判定は「露点スプレッド T−Td ≦ しきい値」または「RH ≧ しきい値」で代理する（しきい値は research）。
3. **葉面湿潤時間（高湿度継続）の日次積算** — RH がしきい値以上で連続した時間を JST 暦日ごとに積算し（付録A D⑤）、**日次の葉面湿潤時間（時間/日）**を表で示す。これを病害スコアの下地とする。
4. **高湿度継続イベントの抽出・一覧** — RH がしきい値以上で一定時間以上続いた区間を**イベントとして抽出**し（開始/終了/継続時間/最小スプレッド 等）一覧化する（付録A E の連続ラン検出）。発病記録との事後突合に使える環境データの提示。

**設計の核＝スキーマ非変更を既定とする**: 露点・結露帯・葉面湿潤時間・高湿度イベントは**すべて既存 `sensor_readings`（temperature/humidity の2列）から読み取り時に計算できる**。よって **P2/P3/P4/P5 と同じく「読み取り時計算・マイグレーションなし」を既定方針**とする（ガードレール②/⑧・goose 最新 **00009** のまま・`make db-snapshot` 不要・新規 SQL は不要で `ListRecentSensorReadings` の生行を Go 純関数で走査）。**病害モデルのしきい値（作物別の温度帯×葉面湿潤時間）は `domain.Crop` へ Go 定数で非破壊追加**（`crop.go` が明示する「病害モデル等の他属性は別フェーズが非破壊追加する前提」のフック・§100）＝**DB 列は増やさない**。S5/E1/P2/P3/P5 の既存機能（温湿度2グラフ・統計オーバーレイ・VPD パネル・欠測ギャップ・期間切替・connect 連動）は**無回帰で維持**する。

> **「結露帯」の物理的向き（重要・project_vpd_physics_convention を厳守）**: VPD は乾燥度指標で「高VPD=乾きすぎ=暖色／低VPD=多湿=湿りすぎ=寒色」と確定済み。**結露・葉面湿潤・高湿度は VPD の「湿り側」と同じ事象**ゆえ、**結露帯/葉面湿潤の塗り・符号・ラベルは寒色（湿り側）に揃える**（乾き側の暖色と取り違えない）。P3 VPD では spec 初稿で向きが逆になり、テストも同前提で符号化したため全緑のまま誤りが残り**実機スモークで初めて発覚**した。本フェーズは**結露=多湿側の向きを spec/テストに明記**し、色・符号・「結露/乾燥」ラベルの向きを**実機スモークで必ず目視確認**する。

> **「葉面温度」の不在（重要・スコープ境界）**: 結露の厳密判定は「葉面温度 ≦ 露点 Td」（付録A D③）だが、本リポジトリには葉面温度センサが無く `sensor_readings` も温湿度のみ。よって**葉面温度は気温で近似**し、結露帯は「露点スプレッド T−Td ≦ しきい値（例 1〜2℃）」または「RH ≧ しきい値（例 95%）」で代理する（メモ第4章「葉面温度の取得可否…無ければ気温で近似」）。近似である旨は UI 注記とし、葉面温度センサ導入時の精緻化は将来（P14 多項目統合）に委ねる。

## 背景・現状

P5（data-quality-meta）マージ後・P3（vpd-dashboard）マージ後の現状は以下（実コード確認済み）。

- **派生指標の純粋層**: `internal/chart/vpd.go` が `math` のみ依存・`[]float64`/スカラ入出力・time 非依存で **Tetens 定数 `tetensA=0.6108`/`tetensB=17.27`/`tetensC=237.3`**・`saturationVaporPressure(tempC)`・`VPD`/`VPDSeries`・`TimeInRange(values,lower,upper)` を提供。**露点 γ 式の `17.27·T/(T+237.3)` は `tetensB`/`tetensC` と同じ定数**＝再利用できる。**露点 Td・スプレッド・結露帯・葉面湿潤・高湿度イベントの純関数はまだ無い**。
- **markArea 注入パターン**: `internal/chart/vpd_echarts.go` の `injectVPDMarkArea` が go-echarts の JSON タグ不具合を回避し option マップへ小文字 `yAxis` キーの markArea を自前注入。**P5 が `internal/chart/gap_echarts.go` の `injectGapMarkArea` で同方式を `xAxis` 範囲キーへ展開済み**（`nullableLineData`・`gapZone(startIdx,endIdx)`・`GapBand`）。**結露帯（時間区間ハイライト）は xAxis 範囲ゆえ `injectGapMarkArea` と同型**。
- **イベント検出の素**: `internal/chart/quality.go` に `StuckRuns(values,minRun)`〔同値連続ラン〕・`RapidChanges(values,maxDelta)`〔急変〕・`MissingStats`〔欠測区間 `GapSpan`〕。**高湿度継続イベント（RH≧しきい値の連続ラン）は `StuckRuns` と同型**だが「同値」でなく「しきい値以上」の連続を見る新関数が要る。
- **別パネル組立の作法**: `internal/handler/device_show_vpd.go` の `buildVPDPanel(labels,temps,hums,rows,crop,period)` が、生行から VPD 系列・滞在率・hour-of-day バケット（`vpdHourlyRows`）・移動平均を組み `component.VPDPanelView` を返す（時刻は handler 境界・純粋層は `[]float64`）。`buildChartArea`（`device_show.go`）が温湿度 option/カード/日次/欠測ギャップ（`applyGapGrid`）に続けてこれを呼び `DeviceChartAreaView` へ詰める。**露点パネルはこの作法の写経で起こせる**。
- **作物マスタ**: `internal/domain/crop.go` が `type Crop`＋9作物（goya/ingen/sugarcane/mango/pineapple/uri/rice/imo/leafy_vegetable）＋`Label()`/`Valid()`/`VPDRange()`/`AllCrops()` を持ち、**「GDD 基準温度・病害モデル等の他属性は別フェーズが非破壊的に追加する前提」とコメントで明示**。`devices.crop`（00009・CHECK ミラー）。**病害モデルのしきい値はこの `Crop` 型へ Go 定数で足す**（DB 列は増やさない）。
- **スキーマ**: `sensor_readings` は temperature/humidity（numeric(5,2)・CHECK −40〜125 / 0〜100）＋recorded_at（計測）＋created_at（受信）。`devices` は locality（P1）/crop（P3）追加済み。**goose 最新は 00009**。露点・病害は2列から導出でき**読み取り側のみで足りる**（既定方針）。
- **時刻・タイムゾーン**: JST 固定（`jst`）。time は handler 境界に留め純粋層（chart）へ持ち込まない規約（structure.md）。
- **露点パネル・結露帯・葉面湿潤時間・高湿度イベントは device-show に存在しない**（現状は温湿度グラフ＋統計＋VPD パネル＋欠測ギャップ＋最新計測テーブル）。

## このセッションのスコープ（実装対象）

### 1. 露点・病害の純粋層（`internal/chart`・time 非依存）

- **新設 `internal/chart/dewpoint.go`**（`vpd.go` の隣・同じ純粋性）に、**`[]float64`/スカラ入出力・`math` のみ依存・time 非依存**の純関数群を追加する。`internal/chart` 最下流純粋層の規約を厳守（gin/DB/templ/pgtype/time を import しない）。
  - **露点 Td**: `DewPoint(tempC, rh float64) float64`（付録A D③・`γ = ln(RH/100) + tetensB·T/(T+tetensC)`、`Td = tetensC·γ/(tetensB − γ)`、**`tetensB`/`tetensC` を `vpd.go` から再利用**＝`tetensA=0.6108` は飽和水蒸気圧専用で露点では未使用ゆえ `DewPoint` は `saturationVaporPressure` を経由せず γ 式を直接組む）。系列版 `DewPointSeries(temps, hums []float64) []float64`（`VPDSeries` と同型）。**RH=0 で `ln(0)=−Inf` となる**ため RH を下限クランプ必須（CHECK は湿度0を許容するため実害あり。床値は research＝例「RH<1% は Td 算出対象外＝欠測扱い」か「RH を微小正値へ床上げ」かを確定）。RH=100 で Td=T（恒等）を満たすこと。**氷点下（CHECK 下限 −40℃ でも分母 T+237.3≈197.3 が正）でも NaN/Inf を出さない**。なお分母 `tetensB−γ` は物理域（RH≤100%）では γ<17.27 が厳密成立しゼロに近づかないため**特別なガードは不要**（到達不能ケースへの過剰ガードを設計しない）。
  - **露点スプレッド**: `DewPointSpread`（T − Td・常に ≥0）の系列。結露しやすさの連続量。
  - **結露帯判定**: スプレッド ≦ しきい値（または RH ≧ しきい値）の点/連続区間を返す純関数（しきい値は引数・既定は定数）。**結露=多湿側**（project_vpd_physics_convention）。
  - **葉面湿潤時間の素**: RH ≧ しきい値の連続を測る純関数（点数→handler が間隔秒で時間換算、または in-range 割合）。`TimeInRange`/連続ラン検出を流用。日次積算は handler が JST 暦日バケットで（`dailyStatRows` 作法）。
  - **高湿度継続イベント抽出**: RH ≧ しきい値が「最小継続スロット以上」続いた区間を `[]EventSpan`（開始/終了 index・継続スロット数・区間内の最小スプレッド/最大RH 等）で返す（`StuckRuns` の連続ラン検出と同型・「同値」でなく「しきい値以上」）。しきい値・最小継続は引数。
  - **病害スコア（△・下地）**: 「温度帯 × 葉面湿潤時間」（付録A D⑤）で日次リスクスコアを出す純関数の**骨格**（作物別しきい値は `domain.Crop` から handler が渡す）。作物モデル・蓄積が要る本格スコアは△ゆえ、**最小の合成（例: 葉面湿潤時間×温度帯該当の重み）に留め、確定モデルは research/未確定**とする。ただし下地として意味を持たせるため、**最低 1 作物（例: ゴーヤ等の施設果菜）で温度帯×湿潤の最小合成が device-show 上に具体値として描画されること**を下限とし、病害スコア欄が常に空表示にならないようにする（他作物への拡張・特定病害予察の精緻化は未確定）。
- **重い統計はやらない**: 病害の統計モデル（ロジスティック回帰・各種病害予察式の本格実装）はアプリ内で計算せず、必要なら CSV 外出し（ガードレール⑧・P4/P8）。本フェーズは「露点の確定式＋しきい値ベースの軽いイベント検出/積算」に集中する。

### 2. 描画層（`internal/chart/dewpoint_echarts.go`・結露帯 markArea）

- **新設 `internal/chart/dewpoint_echarts.go`** に `DewpointChartOptionJSON(spec DewpointChartSpec)` を起こす（`vpd_echarts.go` の写経）。**series[0]=露点 Td 線（主役）**＋**気温 T の重ね線**（接近を見せる）。HTML 安全 JSON（`encoding/json`・SetEscapeHTML=true・`</script>` 不混入）を維持。
- **結露帯 markArea**: 結露しやすい連続区間を **xAxis 範囲指定の markArea で帯ハイライト**する（メモ §2-3 早見表は結露帯を yAxis 帯の一般例として挙げるが、本フェーズの結露帯は「条件が成立する*時間区間*」ゆえ xAxis 範囲指定が物理的に正しく、P5 の xAxis 注入と同型＝意図的にメモ表と異なる）。go-echarts の JSON タグ不具合を避け、**P5 の `injectGapMarkArea`（小文字 `xAxis` キー注入）と同型の自前注入**で実装する（gap の灰色に対し**結露帯は寒色＝湿り側**・project_vpd_physics_convention）。既存 `injectGapMarkArea`/`gapZone` を汎用化して色引数を取れるようにするか、露点用に別関数を起こすかは design。
- **入力契約**: `internal/chart/series.go` に **`DewpointChartSpec` を別型で隔離**（露点 Td・気温・結露帯区間 等）。温湿度 `ChartSpec`・`VPDChartSpec` の無回帰を守る（P3 が VPDChartSpec を別型にした作法）。
- **クラッタ回避**: 露点パネルは温湿度・VPD と**別パネル**（§2-2）。主役は露点 Td＋気温の2本＋結露帯（線を増やしすぎない）。

### 3. handler（`internal/handler/device_show_dewpoint.go`・露点パネル組立）

- **新設 `internal/handler/device_show_dewpoint.go`** に `buildDewpointPanel(labels, temps, hums, rows, crop, period)` を起こす（`device_show_vpd.go` の `buildVPDPanel` の写経）。生行から露点系列・スプレッド・結露帯区間・葉面湿潤時間（日次）・高湿度イベントを組み `component.DewpointPanelView` を返す。**時刻が要る hour-of-day/JST 暦日バケットは handler 境界**（`vpdHourlyRows`/`dailyStatRows` 作法）・純粋層は `[]float64`。
- **`device_show.go` の `buildChartArea` から呼び出し**、`DeviceChartAreaView` 末尾へ `Dewpoint DewpointPanelView` を非破壊追加（VPD パネルと並列・`HasData=false`〔計測0件〕では空パネルで templ 側非表示）。
- **作物別病害しきい値**: `deviceCrop(device)`（既存）→ `domain.Crop` の病害モデル属性（新設メソッド）で温度帯/湿潤しきい値を解決。未設定・露地作物は既定挙動にフォールバック（ガードレール⑥）。
- **露点カード**: 現在露点・現在スプレッド（T−Td）・本日の葉面湿潤時間・直近の結露帯有無 等を整形カードで（`VPDCardView` と同型・`formatStat`/`statEmptyMark` 流用）。

### 4. View / templ / モック反映

- `internal/view/component/views.go` に **`DewpointPanelView`・`DewpointCardView`・葉面湿潤日次行・高湿度イベント行 DTO**をイミュータブルに追加（`DeviceChartAreaView` 末尾へ非破壊追加）。
- `DeviceChartArea.templ` に **露点パネル（別ブロック・`#dewpoint-chart` `data-echarts data-unit="℃" data-color`）＋露点カード＋葉面湿潤時間（日次表）＋高湿度継続イベント表**を VPD パネルの下に追加描画。既存クラス（`.summary-grid-4`・`.data-table` 等）を流用し独自クラス新設は最小。
- `App.templ` の `EChartsInitializer` は `[data-echarts]` 走査で露点 line も init される（series[0] のみ endLabel/sampling は無回帰）。露点パネルの connect 参加（温湿度との時間軸 axisPointer 共有・y は℃で温度と同系）は design 判断。
- **モック反映**: 露点パネルの器・露点カード・葉面湿潤/病害スコア枠・高湿度イベント表は `mocks/html/device-show.html`＋`mocks/html/style.css`（正本・**`--color-dewpoint` 新色トークン追加**）へ反映し `make sync-css`（feedback_mock_reflects_impl_visual）。**グラフ内部の露点線/気温重ね/結露帯 markArea 描画はモック反映の例外**（feedback_mock_graph_rendering_exception）。

### 5. 病害スコア・発病記録突合 — △/限定スコープ/未確定

- **病害スコア**はメモで △（作物モデル・蓄積前提）。本フェーズは**葉面湿潤時間×温度帯の最小合成（下地）まで**とし、確定した病害予察モデルは research/未確定に送る。
- **発病記録（ユーザー手入力の実観測）の永続化＋環境突合**は、唯一スキーマ追加（`disease_observations` テーブル）を要する発展部分。**既定スコープからは外し**、表を起こすかは design の論点（既定は起こさず環境側の risk 提示まで）。本フェーズは単一 device の露点・結露・葉面湿潤・高湿度イベントの可視化を完成させることを優先する。

## スコープ外（このセッションでやらないこと）

- **派生指標列の DB 追加・マイグレーション（既定）**: 露点・結露帯・葉面湿潤・高湿度イベントは読み取り時計算で足りる（ガードレール②/⑧）。**病害モデルしきい値は `domain.Crop` へ Go 定数で非破壊追加**（DB 列なし・§100）。`sensor_readings`/`devices` スキーマ・受信 API（`sensor_api.go`）の変更はしない（goose 00009 のまま・`make db-snapshot` 不要）。**発病記録テーブル（`disease_observations`）の新設は既定スコープ外**（採るなら design で根拠を明示し expand-contract＋スナップショット再生成）。
- **葉面温度センサの統合・CO2/照度/土壌水分 等の新規計測項目**＝フェーズ14（multimetric-integration）。本フェーズは温湿度のみから露点を導く（葉面温度は気温で近似）。
- **本格的な病害予察統計モデル**（作物別の発病ロジスティックモデル・各病害固有の予察式の確定実装・機械学習）＝研究の本務だが**アプリ内では計算しない**（ガードレール⑧で CSV 外出し・P8/P15）。本フェーズは露点の確定式＋しきい値ベースの軽いイベント検出/積算＋スコア下地まで。
- **病害アラート/通知の発火**（結露帯/高湿度継続を検知してメール/LINE 通知）＝本フェーズは**表示（露点線/結露帯/葉面湿潤時間/イベント一覧）まで**。既存のアラート判定（S2 `alert_evaluator`・しきい値超過の履歴化）に病害条件を組み込むことはしない（将来検討）。
- **農家向け平易表示・共有 URL**（信号色の平易な病害警告ダッシュボード）＝フェーズ13（farm-benchmark-share）。本フェーズは研究用詳細表示（ガードレール④）。
- **CSV エクスポートへの結露時間列の追加**: P4（sensor-data-export）の集計帳票は「結露時間列は露点 P6 依存で本フェーズ対象外」として明示的に保留している。**P6 で露点を実装後、P4 の帳票へ結露時間列を非破壊追加するかは design の論点**（本フェーズ必須か P4 への後追いかを判断）。CSV 機能本体は P4 所有（消費のみ）。
- **多地点の露点/結露比較（内外/東西）**＝フェーズ10（multipoint-compare。複数センサ採用が前提）。本フェーズは単一 device。
- **THI・熱帯夜・絶対湿度 AH**＝フェーズ12（heat-stress-thi）。**GDD・収穫予測**＝フェーズ7。本フェーズの派生指標は**露点 Td・葉面湿潤（付録A D③/D⑤）に限る**。
- **P2/P3/P4/P5/E1 の既存機能の仕様変更**（統計オーバーレイ・VPD パネル・CSV・品質メタ・グラフ移行は無回帰維持・消費のみ）。認証・所有者認可・MethodOverride・CSRF・期間バリデーション本体（S1/S5 所有・消費のみ）。device-show 以外の画面（dashboard・readings 等）への露点展開。

## 技術制約・準拠事項

- **読み取り側のみ・スキーマ非変更（既定）**: マイグレーションを行わない（goose 最新 00009 のまま・`make db-snapshot` 不要）。露点・病害メタは `ListRecentSensorReadings`（既存・期間生行）を Go の純関数で走査して算出。病害モデルしきい値は `domain.Crop` の Go 定数（DB 列を増やさない）。発病記録テーブルを採る場合のみ DDL（design 判断）。
- **計算層と描画層の分離（ガードレール⑧）**: 露点/スプレッド/結露帯/葉面湿潤/イベント検出は純粋 Go（`internal/chart` の `[]float64` 入出力・**time 非依存**）。recorded_at の時間換算（葉面湿潤の「時間」・hour-of-day/日次バケット）は **handler 境界**で行い純関数には `[]float64` を渡す（`vpdHourlyRows`/`dailyStatRows` 作法）。重い病害統計はアプリで計算しない。
- **数式の確定値（付録A D③）**: 露点 `γ = ln(RH/100) + 17.27·T/(T+237.3)`／`Td = 237.3·γ/(17.27 − γ)`。定数 17.27・237.3 は Tetens 式と共通＝**`vpd.go` の `tetensB`/`tetensC` を再利用**（重複定義しない）。**RH=0 のクランプ・RH=100 で Td=T・氷点下（−40℃）**を数値安全に扱う（分母 `tetensB−γ` は物理域で γ<17.27 ゆえゼロに近づかず特別なガード不要＝到達不能ケースへの過剰ガードを設計しない）。
- **物理規約（project_vpd_physics_convention）**: **結露・葉面湿潤・高湿度は VPD の「湿り側（低VPD=多湿=寒色）」と同じ向き**。結露帯の塗り色は寒色（`--color-dewpoint` または湿り側トークン）、スコア/逸脱の符号・「結露/乾燥」ラベルの向きを取り違えない。**spec/テストに向きを明記し、実機スモークで目視確認**（テストは仕様前提を符号化するため向きの誤りを捕捉できない）。
- **結露帯 markArea**: 時間区間ハイライトゆえ xAxis 範囲指定。go-echarts JSON タグ不具合を避け **P5 `injectGapMarkArea` の小文字 `xAxis` キー自前注入を踏襲/再利用**。返り値 JSON は `encoding/json`（SetEscapeHTML=true）で HTML 安全化（既存 option 関数の不変条件を維持）。
- **別型隔離**: 露点 option は `DewpointChartSpec`（別型）で受け、温湿度 `ChartSpec`・`VPDChartSpec` の無回帰を守る（P3 の作法）。
- **依存方向**（structure.md）: 下向き一方向。`internal/chart` 最下流純粋性を維持。`internal/domain.Crop` の病害モデル属性は純粋（fmt のみ・§100 Go 定数）。view は repository/service を import せず、domain 表示メソッドのみ参照可。所有者認可は `internal/authz`（`RequireDeviceOwner`）集約・閲覧系の非所有/不在は 404（列挙防止）。
- **イミュータブル**: sqlc 生成構造体は読取専用。handler で View / 露点・病害結果を組み立てる。`internal/chart` 純関数は入力スライスを破壊しない。
- **マスタ/列挙＝Go 定数**（structure.md §100）: 病害モデルしきい値は `domain.Crop` のメソッド/定数で持つ（DB に持たないため CHECK 不要・純粋 Go）。9作物の集合・並びは P3 の `crop.go`・`devices.crop` CHECK と一致を維持。
- **言語**: 日本語コメント・エラー・コミット・UI ラベル（「露点」「結露帯」「葉面湿潤時間」等）。コード識別子は英語。
- **TDD**: 80% 以上。露点 Td（RH=100→Td=T 恒等・RH=0 下限クランプ・氷点下 −40℃ で NaN/Inf なし・既知の手計算値一致）・スプレッド（T−Td≥0）・結露帯判定（しきい値境界・連続区間抽出・先頭/末尾・全区間結露/無結露）・葉面湿潤時間（連続ラン・しきい値境界・日跨ぎバケット）・高湿度イベント抽出（最小継続境界・単発除外・連続結合）・病害スコア下地（温度帯×湿潤の合成・空入力）・結露帯 markArea 注入（小文字 `xAxis` キー・空区間）・**物理規約の向き（結露=湿り側の色/符号/ラベル）**・所有者認可（非所有→404）・**無回帰**（P3 VPD パネル・P5 欠測ギャップ/品質・P2 統計オーバーレイ・温湿度2グラフ・期間切替が従来どおり）。

## 受け入れ基準（概略）

1. **露点 Td 計算正当性**: 任意の (温度, 湿度) で Td が付録A D③（`γ = ln(RH/100)+17.27T/(T+237.3)`・`Td = 237.3γ/(17.27−γ)`）どおり算出され、**RH=100 で Td=T・RH=0 で数値破綻しない**（下限クランプ）・氷点下（−40℃）で NaN/Inf を出さない（既知の手計算ケースと一致）。定数は `vpd.go` の `tetensB`/`tetensC` を再利用している（`tetensA` は露点では未使用）。
2. **露点＋気温＋結露帯（◎）**: device-show の露点パネルに露点 Td 線＋気温 T の重ね線が描かれ、**結露しやすい時間区間が markArea（xAxis 範囲）でハイライト**される。go-echarts JSON タグ不具合を避けた xAxis 自前注入（P5 流用）で正しく描画される。結露帯の判定が葉面温度を気温で近似した代理（スプレッド T−Td ≦ しきい値 or RH ≧ しきい値・厳密な「葉面温度 ≦ Td」ではない）である旨が UI/仕様に明示される（Td の算出は基準1 の D③ 厳密式・結露帯の判定はこの代理式、と切り分ける）。
3. **物理規約の向き**: 結露帯/葉面湿潤の**塗り色・符号・ラベルが「湿り側（寒色）」**で一貫している（project_vpd_physics_convention）。乾き側（暖色）と取り違えていないことが spec/テストに明記され、実機スモーク確認の手順が残る。
4. **葉面湿潤時間の日次積算**: RH ≧ しきい値の連続を JST 暦日ごとに積算した**葉面湿潤時間（時間/日）**が表で確認でき、空期間/単一点/日跨ぎが安全に扱われる。
5. **高湿度継続イベント一覧**: RH ≧ しきい値が最小継続以上続いた区間がイベント（開始/終了/継続/最小スプレッド 等）として抽出・一覧化され、単発（最小継続未満）が除外される。
6. **葉面温度の近似が明示**: 葉面温度センサが無く**気温で近似**している旨と、結露判定が「スプレッド ≦ しきい値 or RH ≧ しきい値」の代理である旨が UI 注記/仕様で明確。
7. **病害スコア（△）の線引き**: 病害スコアは葉面湿潤時間×温度帯の最小合成（下地）に留まり、確定した病害予察モデル・発病記録突合は research/将来として切り分けられている。本フェーズの確定スコープ（単一 device の露点・結露・葉面湿潤・高湿度イベント）が崩れていない。
8. **スキーマ非変更（既定）**: マイグレーションを行わない（goose 00009 のまま・`make db-snapshot` 不要）。露点・病害メタはすべて読み取り時計算。病害モデルしきい値は `domain.Crop` の Go 定数。発病記録テーブルを採る場合のみ design で根拠明示＋expand-contract 移行＋スナップショット再生成。
9. **無回帰**: P3 VPD パネル・P5 欠測ギャップ/品質メタ・P2 統計オーバーレイ・温湿度2グラフ・期間切替（24h/3d/7d/30d・URL 同期）・connect 連動・空データ表示が従来どおり動作。所有者でない device への露点パネル表示は 404。
10. **モック整合**: 露点パネル器・露点カード・葉面湿潤/病害スコア枠・高湿度イベント表がモック（device-show.html＋style.css 正本・`--color-dewpoint` 追加）に反映されている（グラフ内部の露点線/気温重ね/結露帯 markArea 描画は反映例外）。
11. **テスト 80% 以上**。

## 未確定事項・要確認（設計フェーズで決定）

- **① 発病記録の永続化を採るか（最重要・design の最初の論点）**: 既定は**読み取り時計算・スキーマ非変更**（P2〜P5 踏襲）。発病記録（ユーザー手入力の実観測）を保存して環境と事後突合するには `disease_observations` テーブル（device_id・観測日・病害名・程度 等＋00010 マイグレーション＋CRUD UI）が要る。本フェーズの規模・確度（病害スコアは △）では**既定はスキーマ非変更で環境側 risk 提示まで**を強く推奨し、発病記録突合は別 spec/将来へ。採るなら expand-contract＋`make db-snapshot`。
- **② 結露判定の代理しきい値（葉面温度の不在）**: 葉面温度が無いため「スプレッド T−Td ≦ しきい値（例 1〜2℃）」か「RH ≧ しきい値（例 95%）」のどちらを結露帯の判定にするか・しきい値の値。**沖縄の実環境（夜間放射冷却で葉面温度が気温より下がる・高湿度が長く続くのは正常）をユーザーの実地知見（権威）で確定**。
- **③ 葉面湿潤時間のしきい値・最小継続**: RH 何%以上を「湿潤」とみなすか（例 90%）・連続何分以上を1イベントとするか（サンプリング間隔＝約5分前提）。病害予察文献と沖縄実環境で research。
- **④ 病害スコアのモデル**: 「温度帯 × 葉面湿潤時間」（付録A D⑤）の具体式。作物別（灰色かび病・うどんこ病 等の発病好適温度帯）の重み付けをどこまで `domain.Crop` に持たせるか。本フェーズは下地（最小合成）に留めるか、特定病害1つの簡易予察まで踏み込むか。**作物別の発病好適条件はユーザー（研究者本人）/文献で確定**。
- **⑤ 病害モデル属性の `domain.Crop` 設計**: `crop.go` が明示する非破壊追加フックに、病害モデルの温度帯/湿潤しきい値をどう持たせるか（`DiseaseModel()` メソッド／定数表）。9作物のうち施設果菜（ゴーヤ/インゲン 等）と露地（サトウキビ/米 等）で持つ属性が割れる前提。
- **⑥ 露点パネルの見せ方**: 露点 Td 単独か気温 T 重ねか（接近を見せるなら重ねが有効）。結露帯 markArea の判定（スプレッド or RH）。露点グラフの connect 参加（温度と同じ℃軸ゆえ温度グラフと axisPointer 共有が自然か）。期間別（24h はカード/瞬間・複数日は日次葉面湿潤表）で粒度を変えるか（P2/P3 の ShowDaily と整合）。
- **⑦ 露点グラフの y 軸スケール**: 気温と露点を重ねるため共通℃軸。固定 or auto。結露帯（スプレッド小）が見やすいスケール。
- **⑧ P4 集計帳票への結露時間列追加**: P4 が保留した「結露時間列」を本フェーズで P4 帳票へ非破壊追加するか、P6 では device-show の可視化までとし P4 への後追いにするか。**責務境界の確定**: 結露時間の*算出*（連続結露区間の時間積算）は葉面湿潤/結露帯の純関数と同根ゆえ**本フェーズの `dewpoint.go` で提供**し（P4 後追い時の再実装を避ける）、P4 帳票への*列追加（消費）*のみを design 判断とする。
- **⑨ 物理規約の向き（実機スモーク必須）**: 結露/葉面湿潤=湿り側=寒色の向きを spec・テスト・色トークン・ラベルに一貫実装し、**GO 判定後に実機スモークで目視確認**する手順を tasks に含める（P3 VPD で向き逆転がテスト全緑のまま実機まで残った前例＝project_vpd_physics_convention）。

--- spec-init 本文 ここまで ---
