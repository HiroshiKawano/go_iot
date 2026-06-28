# フェーズ5（分析ロードマップ）spec-init プロンプト: データ品質メタ層（レコード単位の品質フラグ・欠測率/サンプリング間隔一貫性/通信遅延の監視・欠測ギャップ可視化・品質バッジ）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: data-quality-meta
> 位置づけ: [分析アイデアメモ.md](../分析アイデアメモ.md) 第1章「実装ロードマップ」の**フェーズ5**〔明示〕。新規画面ではなく、実装済み **device-show（S5/E1/P2）グラフ**と **readings（S6/P4）画面**に、蓄積した計測データの**信頼性メタ情報**を上載せする層＝(1) レコード単位の**品質フラグ**（欠測/stuck/flatline/物理異常/外れ値）、(2) **欠測率・サンプリング間隔一貫性・通信遅延の監視**、(3) **品質バッジ表示**、(4) チャートの**欠測ギャップ可視化**（`connectNulls:false`＋markArea）。研究＝再現性・査読品質が前提で「どの期間が信頼できるか」を示せないデータは論文・報告書に使えない、という顧客像（付録B-1 研究者・公務員）に直結する。塩害・多台数運用の沖縄で特に効く（メモ フェーズ5 の狙い）。
> 確度: 〔明示〕（分析アイデアメモ フェーズ5・ガードレール③「データ品質メタ層を最初から想定」の実装本体。提案型案件ゆえ積極採用＝付録C-3 の方針）。
> 前提セッション: **ロードマップ上の論理依存は無し**（メモ早見表＝スキーマ affordance はガードレール③で最初から想定済みのため、新たな前提フェーズを作らない）。**ただし実装は P1〜P4 マージ済みコードへの「上載せ・流用」が前提**（既存の device-show グラフ・readings 画面・統計純関数の上に品質メタを足す）: **S6（sensor-readings-history。`internal/handler/readings.go` の `parseDateBounds`〔from/to → BETWEEN 区間〕・`formatDelay`〔通信遅延 N秒＝recorded_at と created_at の差・既実装〕・一覧表〔通信遅延列は既存〕）／ P4（sensor-data-export。`ListSensorReadingsInRange`〔BETWEEN・ORDER ASC・LIMIT なし＝期間内全行スキャン〕・`readings_report.go` の欠測日 "—" 補完パターン）／ P2（temp-humidity-chart-stats。純粋統計層 `internal/chart/stats.go`＝`Mean`/`StdDev`/`MinMax`/`CV` を Zスコア・IQR・分散の素に流用）／ S5/E1（device-detail・device-chart-echarts。`internal/chart/echarts.go` の `ChartOptionJSON(ChartSpec)`＝欠測ギャップ可視化を載せる対象）／ P1（device-location-select。`devices.locality`＝器差を「同一地域の複数センサ」で粗く突き合わせる場合のグルーピングキー〔限定スコープ・後述〕）／ S1（web-foundation-auth。RequireAuth・所有者認可）**
> **スキーマ判断（最重要・このフェーズの設計の核）**: 品質シグナルは**ほぼ全て既存 `sensor_readings`（temperature/humidity の2列＋recorded_at〔計測時刻〕＋created_at〔受信時刻〕）から読み取り時に計算できる**（欠測=recorded_at の連続性・stuck/flatline=同値連続・物理異常=値域・外れ値=Zスコア/IQR・欠測率/間隔一貫性=recorded_at 差分・通信遅延=recorded_at↔created_at〔既に `formatDelay` で算出〕）。よって **P2/P3/P4 と同じく「読み取り時計算・スキーマ非変更」を既定方針とする**（ガードレール②＝派生列/品質フラグ列は必要になったフェーズで足す YAGNI／⑧＝軽い計算はアプリ内）。**ガードレール③「品質フラグ列を schema に確保」は「将来 affordance（非破壊追加できる設計）を保つ」意であって本フェーズで必ず DDL する意ではない**＝**永続フラグ列の追加は「自動導出できない人手のキュレーション判定」または「受信 API 側でのタグ付け」が要るときに限り design で判断**（既定はスキーマ非変更＝goose 最新 **00009** のまま・`make db-snapshot` 不要・新規 SQL は集計/全行取得の SELECT のみ＝DDL なし）。この方針確定が design の最初の論点（未確定事項①）。
> 設計フェーズで参照:
> - 上位ロードマップ: 分析アイデアメモ.md フェーズ5（**レコード単位の品質フラグ〔欠測/stuck/flatline/物理異常/外れ値〕／欠測率・サンプリング間隔一貫性・通信遅延の監視／同一ハウス複数センサ間の差分＝器差〔キャリブレーションずれ〕検証／品質バッジ表示／▶ ◎ 欠測ギャップ可視化〔`connectNulls:false`＋markArea〕・品質バッジ**）／ 第3章ガードレール ③データ品質メタ層を最初から想定（**本フェーズが③の実装本体**・品質フラグ列を schema に「後付け可能に」確保＝affordance）・②スキーマに品質フラグ列を後付け可能に（実列追加は必要なフェーズで＝YAGNI・集計軸を壊さない移行）・⑧計算層と描画層の分離（軽い異常検知はアプリ内・**重い統計＝STL/Mann-Kendall は CSV 外出し**）・①集計軸固定（品質メトリクスも期間×地点×作物で横並びできること）・⑤時間割合の発想（欠測「率」・在帯「率」と同型で品質も率で語る）／ 付録A「E. 異常検知」（**Zスコア `z=(x−μ)/σ`〔|z|>3〕・ローリングσ法〔移動窓 μ±kσ〕・IQR法〔x<Q1−1.5·IQR または x>Q3+1.5·IQR〕・急変検出〔変化率（微分）閾値超え〕・センサー故障〔同値 N 回連続 stuck/flatline／物理的にあり得ない値〕**）・「A. 基本統計量」（**欠測率・分散σ²・σ・CV・通信遅延**＝間隔一貫性/監視の素）
> - 現スキーマ（権威）: docs/database_snapshot/table_definitions.md「sensor_readings」（**temperature/humidity numeric(5,2)・recorded_at timestamptz〔計測〕・created_at timestamptz〔受信＝通信遅延算出に使用〕・updated_at・deleted_at**・(device_id, recorded_at DESC) 部分索引 WHERE deleted_at IS NULL／**CHECK 制約 temperature −40〜125・humidity 0〜100 が受信時に既に強制**＝範囲外の値は DB に保存され得ない。**ゆえに本フェーズの「物理異常」検知は CHECK の更に内側＝農学的にあり得ない値・据置故障の疑い値・急変**を対象とする〔単純な値域超過は受信時に弾かれ蓄積データには現れない〕）／「devices」（**locality VARCHAR(20)＝P1/00008・crop VARCHAR(20)＝P3/00009 とも追加済**・last_communicated_at timestamptz）。goose 連番の最新は **00009**。品質メタは2列＋2時刻から導出でき**スキーマ非変更で足る**（既定方針）。
> - 移行元・拡張対象コード（実コード確認済み・P4 マージ後の現状）:
>   - `internal/chart/stats.go` … 純粋統計層（`math` のみ依存・`[]float64` 入出力・gin/DB/templ/**time** 非依存）。**既存** `Mean`/`StdDev`〔母σ・N除算〕/`MinMax`/`DiurnalRange`/`CV(values,eps)→(float64,bool)`/`SMA`/`MovingStdDev`/`Band`/`Deviation`。→ **Zスコア=`(x−Mean)/StdDev`・ローリングσ法=`SMA`＋`MovingStdDev`＝既存流用**、**IQR〔四分位〕・stuck/flatline〔同値連続ラン〕・物理範囲〔農学的下限上限〕・欠測率・間隔一貫性は新規純関数**を本ファイルか **新設 `internal/chart/quality.go`** へ追加（純粋・`[]float64` 入出力・time 非依存を厳守）。
>   - `internal/chart/echarts.go` … `ChartOptionJSON(spec ChartSpec)`＝device-show 温湿度グラフの唯一の option 関数。series[0] に markPoint max/min を付与。**`connectNulls` は現状未使用**＝欠測ギャップで線を切るには `ShowSymbol`/`ConnectNulls` 相当の指定追加が要る（欠測点を nil で出し線を分断）。`ChartSpec` 拡張点（`series.go`）。
>   - `internal/chart/vpd_echarts.go` … `injectVPDMarkArea`＝go-echarts の `MarkAreaData.YAxis` が JSON タグ非準拠（大文字 `YAxis`）のため、**option マップを一旦 JSON 化→マップへ戻し series[0] へ ECharts 準拠の小文字 `yAxis` キーで markArea を自前注入する確立パターン**。→ **欠測ギャップの markArea〔xAxis 範囲の縦帯ハイライト〕は、この yAxis 注入の前例を踏襲した新規実装**（注入の枠組みは同方式・キーを `yAxis`→`xAxis` 範囲指定に置換する。既存は yAxis キーの注入実績のみで xAxis キー注入は前例が無いため、design で実装を起こす）。
>   - `internal/handler/readings.go` … S6 の中心。`parseDateBounds(from,to)→(fromTS,toTS,errs)`〔YYYY-MM-DD→BETWEEN・未指定はセンチネル・JST 暦日〕・`fetchResults`・`buildReadingHistoryRows`・**`formatDelay(recordedAt,createdAt)`＝通信遅延「N秒」整形（負値は0クランプ・既実装）**。一覧表示は既に通信遅延列を持つ。→ **品質メタ（バッジ・欠測率・間隔一貫性）はこのフィルタ区間（parseDateBounds）と同一期間で算出**し、一覧/集計と値が一致する。通信遅延は `formatDelay` を流用（再実装しない）。
>   - `internal/handler/readings_report.go`（P4）… 期間内を JST 暦日でバケット化し**最古日〜最新日の計測の無い日を欠測行 "—" で補完**するパターン（`emptyReadingsReportRow`）。→ **欠測「率」や欠測ギャップ抽出はこの暦日連続走査の作法を一般化**（連続する欠測区間＝ギャップ）。
>   - `internal/handler/device_show.go`（S5/P2）… `dailyStatRows`〔JST 暦日バケットの平均/最高/最低/日較差/σ/CV〕・`statEmptyMark="—"`・欠測日 "—" 補完・`jst`。→ **device-show に品質バッジ・欠測ギャップ可視化を載せる主対象**。日次品質メトリクスは dailyStatRows と同じバケット作法。
>   - `internal/handler/readings_export.go`（P4）… 期間内全行 CSV。→ **品質フラグ列を CSV に足す余地**（行ごとに欠測/stuck/外れ値フラグを添える）は P4 の出力に非破壊追加できる（design 判断・スコープに含めるか要検討）。
>   - `db/queries/sensor_readings.sql` … **既存** `ListSensorReadingsInRange`〔device＋BETWEEN・ORDER BY recorded_at ASC・LIMIT なし＝**期間内全行**〕（P4 で追加済）・`ListSensorReadingsPaginated`・`GetSensorReadingsSummary`・`ListDailySensorAggregates`〔`recorded_at>=$2` 単一境界・グラフ用〕・`CountSensorReadingsInRange`。→ **品質スキャンは `ListSensorReadingsInRange` の全行を Go（quality 純関数）で走査**すれば足り、**新規 SQL は原則不要**（欠測率/stuck/外れ値はクエリでなく走査で出す＝計算層分離）。永続フラグを採らない限り DDL なし。
>   - `cmd/server/main.go` … `/devices/:device`・`/chart`・`/readings`・CSV 経路の登録（GET は CSRF 対象外・所有者認可）。→ 品質メタは既存 device-show/readings ハンドラ内で組み立て（新経路は原則不要。品質専用の部分更新エンドポイントを足すかは design）。
> - ビュー/モック（単一ソース運用の境界）: `internal/view/page/DeviceShow.templ`・`internal/view/page/Readings.templ`・`internal/view/component/DeviceReadingsList.templ`／ `mocks/html/device_show.html`・`mocks/html/readings.html`＋`mocks/html/style.css`（正本）。**品質バッジ・品質フラグ列・欠測ギャップの凡例/注記は静的な「器」＝HTML/CSS ゆえモック反映必須**（feedback_mock_reflects_impl_visual・project_css_single_source）。**グラフ内部の `connectNulls`/欠測 markArea 描画は動的描画ゆえモック反映の例外**（feedback_mock_graph_rendering_exception＝器〔凡例・注記・バッジ〕は反映対象、線/帯の描画は対象外）。バッジ色（信頼=緑/注意=黄/不良=赤の信号色）は `@layer components` に追記し `make sync-css`。
> - 命名・依存規約: .kiro/steering/structure.md（依存方向＝下向き一方向・`internal/chart` は最下流純粋層〔math のみ・time 非依存〕・view→domain 表示メソッドのみ・§98 外部キー張らない・§99 論理削除・**§100 マスタ/列挙は Go 定数+VARCHAR+CHECK**・所有者認可は `internal/authz` 集約）／ tech.md（sqlc・データアクセス方針）／ CLAUDE.md（**既定はマイグレーション無し**＝`make db-snapshot` 不要。万一スキーマ追加を design で採るなら expand-contract の後方互換移行＋スナップショット再生成必須）。

--- spec-init 本文 ここから ---

## 機能概要

蓄積した温湿度計測データに**信頼性のメタ情報（データ品質メタ層）**を上載せし、「どの期間・どのレコードが信頼できるか」を可視化する。研究・普及（付録B-1 ＝再現性/査読品質が前提・CSV を外部へ出して再解析する顧客像）にとって、欠測率・器差・品質フラグを示せないデータは論文・報告書に使えない、という要請に応える層であり、**ガードレール③「データ品質メタ層を最初から想定」の実装本体**にあたる。具体的には、実装済みの **device-show（S5/E1/P2 のグラフ）**と **readings（S6/P4 の履歴画面）**に次を追加する。

1. **レコード単位の品質フラグ** — 各計測行に「欠測（前後で記録が飛んでいる）／stuck・flatline（同値が N 回連続＝センサー固着の疑い）／物理異常（農学的にあり得ない値・据置故障の疑い）／外れ値（Zスコア `|z|>3` または IQR 法）」の判定を付与する（付録A E）。
2. **期間品質メトリクスの監視** — 表示期間について **欠測率（%）・サンプリング間隔の一貫性（間隔のばらつき＝σ/CV）・通信遅延**（recorded_at と created_at の差。既存 `formatDelay` を流用）を集計表示する（付録A A／E）。
3. **品質バッジ** — 期間/デバイスの総合品質を**信号色のバッジ**（信頼＝緑／注意＝黄／不良＝赤、判定基準は design）で一目に示す。研究用の詳細表示と、農家共有の平易表示（→ P13）は分離する（ガードレール④）。
4. **欠測ギャップ可視化** — device-show のグラフで**欠測区間は線を分断**（`connectNulls:false`：欠測点を nil で出し折れ線を繋がない）し、**連続欠測区間を markArea でハイライト**する（メモ フェーズ5 の ◎）。

**設計の核＝スキーマ非変更を既定とする**: 上記の品質シグナルは**ほぼ全て既存 `sensor_readings`（temperature/humidity の2列＋recorded_at〔計測時刻〕＋created_at〔受信時刻〕）から読み取り時に計算できる**（欠測＝recorded_at 連続性・stuck/flatline＝同値連続・物理異常＝値域・外れ値＝Zスコア/IQR・欠測率/間隔一貫性＝recorded_at 差分・通信遅延＝既存 `formatDelay`）。よって **P2/P3/P4 と同じく「読み取り時計算・マイグレーションなし」を既定方針**とする（ガードレール②/⑧）。**ガードレール③「品質フラグ列を schema に確保」は将来の affordance（非破壊で列を足せる設計を保つ）であって本フェーズで必ず DDL する意ではない**。永続フラグ列の追加は「自動導出できない人手のキュレーション判定」または「受信 API 側でのタグ付け」が必要なときに限り design で判断する（未確定事項①）。既定では goose 最新は **00009** のまま・`make db-snapshot` 不要・新規 SQL は走査用 SELECT のみ（多くはクエリ追加すら不要で `ListSensorReadingsInRange` の全行を Go で走査）。

S5/E1/P2 のグラフ・統計オーバーレイ、S6/P4 の期間フィルタ・集計・一覧・ページネーション・CSV は**無回帰で維持**する。

> **「物理異常」の意味（重要・誤解防止）**: `sensor_readings` には受信時 CHECK 制約（temperature −40〜125・humidity 0〜100）が既に効いており、**範囲外の値はそもそも DB に保存されない**。したがって本フェーズの「物理異常」検知は CHECK の単純な追認ではなく、**CHECK の内側にある「農学的にあり得ない値・据置故障の疑い値・急変（変化率閾値超え）」**を対象とする（例: 湿度が物理上限 100% に張り付き続ける／温度が短時間で非現実的に跳ねる）。判定の具体しきい値は research で確定（未確定事項）。

> **「器差（複数センサ間差分）」の扱い（重要・スコープ境界）**: メモは「同一ハウス複数センサ間の差分＝器差（キャリブレーションずれ）検証」を挙げるが、**「同一ハウス」を表す構造（ハウス/圃場グルーピング・内外/東西の位置軸）は本リポジトリに未整備**（位置軸＝内外/東西は**フェーズ10 multipoint-compare** の領域）。本フェーズで持つグルーピングキーは **`devices.locality`（地域＝旧町村レベル・P1）**のみで、これは「同一ハウス」ではなく「同一地域の別圃場」を含む粗い単位である。よって器差は **(a) P10 で位置軸が入るまで defer する／(b) 同一 locality の複数 device を粗く突き合わせる限定版（「同一地域≠同一ハウス」の注記つき）に留める**のどちらかを design で確定する（未確定事項）。**本フェーズの確定スコープは単一 device の品質メタ（フラグ・期間メトリクス・バッジ・欠測ギャップ）**とし、器差は限定スコープ/defer 扱いにする。

## 背景・現状

S6/P4 マージ後・P1/P2/P3 マージ後の現状は以下（実コード確認済み）。

- **device-show（S5/E1/P2）**: `internal/handler/device_show.go`＋`internal/chart/echarts.go`。温湿度の生実測線（ECharts・`ChartOptionJSON(ChartSpec)`）に SMA/正常帯/乖離率を凡例トグルで上載せ（P2）。`dailyStatRows`〔JST 暦日バケットの平均/最高/最低/日較差/σ/CV〕・`statEmptyMark="—"`・欠測日 "—" 補完が既存。**markPoint（max/min）は付与済みだが `connectNulls` は未使用**（欠測があっても線は繋がる）。
- **readings（S6/P4）**: `internal/handler/readings.go` の `ReadingsHandler.Index`。`parseDateBounds(from,to)` が YYYY-MM-DD を BETWEEN 区間へ写す。一覧表は**通信遅延列を既に持つ**（`formatDelay(recordedAt,createdAt)`＝「N秒」・負値0クランプ）。P4 で CSV エクスポート（`readings_export.go`・`ListSensorReadingsInRange`＝期間内全行 ASC）・日次/時間別集計帳票（`readings_report.go`・欠測日 "—" 補完）が追加済み。**品質フラグ・欠測率・間隔一貫性・品質バッジ・欠測ギャップ可視化はまだ無い**。
- **純粋統計層**: `internal/chart/stats.go`（`math` のみ・`[]float64` 入出力・time 非依存）に `Mean`/`StdDev`〔母σ〕/`MinMax`/`DiurnalRange`/`CV`/`SMA`/`MovingStdDev`/`Band`/`Deviation`。**Zスコア・ローリングσ法はこれらで組める**が、**IQR・stuck/flatline・物理範囲・欠測率・間隔一貫性の純関数は未実装**。
- **markArea 注入パターン**: `internal/chart/vpd_echarts.go` の `injectVPDMarkArea` が、go-echarts の JSON タグ不具合を回避して option マップへ小文字 `yAxis` キーの markArea を自前注入する確立手法を持つ。**欠測ギャップの markArea（`xAxis` 範囲指定）も同方式で注入できる**。
- **スキーマ**: `sensor_readings` は temperature/humidity（numeric(5,2)・CHECK で −40〜125 / 0〜100 を受信時強制）＋recorded_at（計測）＋created_at（受信）＋deleted_at。`devices` は locality（P1）/crop（P3）追加済み。**goose 最新は 00009**。品質メタは2列＋2時刻から導出でき、**読み取り側のみで足りる**（既定方針）。
- **時刻・タイムゾーン**: JST 固定（`jst`）。time は handler 境界に留め純粋層（chart）へ持ち込まない規約（structure.md）。

## このセッションのスコープ（実装対象）

### 1. 品質判定の純粋層（`internal/chart`・time 非依存）

- **新設 `internal/chart/quality.go`**（または `stats.go` 追記）に、**`[]float64` 入出力・`math` のみ依存・time 非依存**の純関数群を追加する。`internal/chart` 最下流純粋層の規約を厳守（gin/DB/templ/pgtype/time を import しない）。
  - **外れ値（Zスコア）**: `z=(x−μ)/σ`、`|z|>k`（既定 k=3）で外れ値。`Mean`/`StdDev` を流用。σ≈0（定常）はゼロ除算回避で「外れ値なし」とする。
  - **外れ値（IQR 法・新規）**: 四分位 Q1/Q3 を求め `x<Q1−1.5·IQR` または `x>Q3+1.5·IQR`。Zスコアと併用/択一かは research（裾の重い分布で IQR が頑健）。
  - **stuck/flatline（新規）**: 同一値が N 回以上連続する区間を検出（センサー固着の疑い）。連続ラン長を返し、しきい値 N は design。
  - **物理範囲（農学的・新規）**: CHECK 内側の「あり得ない値」帯を判定（受信時 CHECK の追認ではない）。下限/上限・急変（変化率＝隣接差の閾値超え）の判定。具体値は research。
  - **欠測率・間隔一貫性（新規）**: 連続する recorded_at の**間隔列**（秒）に対し、期待間隔からの欠測本数・欠測率（%）と、間隔の σ/CV（一貫性）を出す。**間隔列の算出（recorded_at 差分）は time を伴うため handler 境界で行い、純粋層には `[]float64`（秒）を渡す**（`dailyStatRows`/`vpdHourlyRows` と同じ「time は境界・純関数は []float64」作法）。
- ローリングσ法（移動窓 μ±kσ 外れ警告）は既存 `SMA`＋`MovingStdDev`＋`Band` で表現できる（再実装しない）。
- **重い統計はやらない**: STL・Mann-Kendall・ACF・回帰検定等はアプリ内で計算せず CSV 外出し（ガードレール⑧・フェーズ8）。本フェーズは「軽い異常検知＋率の集計」に集中する。

### 2. 品質メタの表示（バッジ・品質フラグ・期間メトリクス）

- **品質バッジ**: 表示期間（parseDateBounds の区間）/デバイスの総合品質を**信号色のバッジ**で示す（信頼＝緑/注意＝黄/不良＝赤）。判定は欠測率・外れ値件数・stuck 検出有無・間隔一貫性の合成（しきい値・合成式は design/research）。device-show と readings の双方に出すか片方かは design。**研究用の詳細表示**であり、農家向け平易表示（→ P13）はここでは作らない（ガードレール④）。
- **レコード品質フラグ列**: readings 一覧（`DeviceReadingsList`）に各行の品質フラグ（欠測直後/stuck/外れ値/物理異常）を**バッジ or アイコン列**で添える（既存の通信遅延列の隣など）。フラグの**表示ラベル**は `internal/domain` に品質フラグの列挙（`type QualityFlag string`＋`Label()`、§100 の Go 定数/列挙パターン・純粋 domain）を置くか、handler/component で整形するかは design（domain 列挙にすると view→domain 表示メソッドで描ける）。
- **期間品質メトリクス**: 欠測率（%）・サンプリング間隔（中央値/一貫性）・通信遅延（既存 `formatDelay`・期間の平均/最大）を**集計ボックス/小表**で見せる。S6 の既存 summary-box の作法・"—"（0件）扱いに合わせる。
- **計算は読み取り時**: `ListSensorReadingsInRange`（期間内全行）を取得し、品質純関数で走査して上記を組み立てる（handler で time 境界処理→純関数）。**新規 SQL は原則不要**（必要なら走査用 SELECT のみ・DDL なし）。

### 3. 欠測ギャップ可視化（device-show グラフ・`connectNulls:false`＋markArea）

- **欠測点で線を分断**: 期待サンプリング間隔に対し欠測している区間は**データ点を nil として出し、`connectNulls:false`（既定）で折れ線を繋がない**（`echarts.go` の `ChartOptionJSON`/`ChartSpec` を拡張）。これにより「データがある区間」と「無い区間」が一目で分かる。
- **欠測区間の markArea**: 連続欠測区間を**markArea（xAxis 範囲指定）でハイライト**する。go-echarts の JSON タグ不具合を避けるため、**`vpd_echarts.go` の `injectVPDMarkArea`（yAxis 範囲注入）の枠組みを踏襲した新規実装**として、注入キーを `yAxis`→`xAxis` 範囲指定に置換して注入する（xAxis キー注入は既存実績が無いため design で起こす）。
- **クラッタ回避**: 欠測ギャップは主役の生実測線の上に控えめに重ねる（薄い灰系の帯）。凡例/注記は器ゆえモック反映、線/帯の描画はモック例外（feedback_mock_graph_rendering_exception）。
- 既存の markPoint（max/min）・P2 のオーバーレイ（SMA/正常帯/乖離率）・期間切替・温湿度2グラフ連動は**無回帰維持**。

### 4. 器差（複数センサ間差分）— 限定スコープ/defer

- 「同一ハウス複数センサの器差」は**位置軸（内外/東西）＝フェーズ10 が前提**で、本フェーズには「同一ハウス」を表す構造が無い。よって **(a) P10 へ defer／(b) 同一 `locality` の複数 device を粗く突き合わせる限定版（「同一地域≠同一ハウス」を UI で明示）**のどちらかを design で確定する（未確定事項）。**確定スコープからは外し**、本フェーズは単一 device の品質メタを完成させることを優先する。

### 5. View / templ / モック反映

- `DeviceShow.templ`／`Readings.templ`／`DeviceReadingsList.templ`（または専用 component）に **品質バッジ・品質フラグ列・期間品質メトリクスボックス・欠測ギャップの凡例/注記**を追加（handler で View を組み立てるイミュータブル方式）。既存クラス（`.summary-box`・`.data-table`・`.badge` 等）を流用し、独自クラス新設は最小（バッジ信号色のみ `@layer components` 追記）。
- **モック反映**: 品質バッジ・フラグ列・メトリクスボックス・欠測ギャップ凡例は `mocks/html/device_show.html`・`mocks/html/readings.html`＋`mocks/html/style.css`（正本）へ反映し `make sync-css`（feedback_mock_reflects_impl_visual）。**グラフ内部の `connectNulls`/欠測 markArea 描画は動的描画ゆえモック反映の例外**（器＝凡例/注記/バッジは反映対象）。

## スコープ外（このセッションでやらないこと）

- **品質フラグ列・派生指標列の DB 追加・マイグレーション（既定）**: 品質シグナルは読み取り時計算で足りる（ガードレール②/⑧）。永続フラグ列は「人手キュレーション判定」or「受信時タグ付け」が要るときのみ design で例外採用（既定はスキーマ非変更＝goose 00009 のまま・`make db-snapshot` 不要・新規 SQL は走査 SELECT のみ＝DDL なし）。`sensor_readings`/`devices` スキーマ・受信 API（`sensor_api.go`）の変更はしない。
- **器差の本実装（同一ハウス複数センサの突き合わせ）＝フェーズ10（multipoint-compare）**。本フェーズは単一 device 中心、器差は限定版/defer（locality 粗突合せに留めるか P10 送り）。内外/東西の位置軸は作らない。
- **本格統計のアプリ内計算/描画**（STL・ACF・Mann-Kendall・回帰・分散分析・SARIMA）＝CSV 外出し（ガードレール⑧・フェーズ8/15）。本フェーズの異常検知は Zスコア/IQR/stuck/物理範囲/急変＝軽量に限る。
- **アラート/通知の発火**（品質劣化を検知してメール/LINE 等で能動通知する）＝本フェーズは**表示（バッジ/フラグ/メトリクス/ギャップ）まで**。既存のアラート判定（S2 `alert_evaluator`・しきい値超過の履歴化）は別系統で、品質メタを判定条件に組み込むことはしない（将来検討）。
- **農家向け平易表示・共有 URL**（信号色の平易ダッシュボード）＝フェーズ13（farm-benchmark-share）。本フェーズは研究用詳細表示（ガードレール④）。
- **欠測の補間/穴埋め（補完値の生成）**: 欠測は「無い」と示す（ギャップ可視化・"—"）。線形補間や予測での穴埋めはしない（データ主権・査読品質の観点で**生データを生のまま示す**＝補完は分析を歪める）。
- **VPD/露点/GDD/THI 等の派生指標の品質**（P3/P6/P7/P12 所有）。本フェーズは生の温湿度の品質に集中（派生指標は元データが健全なら従属）。
- **P2/P3/P4/E1 の既存機能の仕様変更**（統計オーバーレイ・VPD パネル・CSV・グラフ移行は無回帰維持・消費のみ）。認証・所有者認可・MethodOverride・CSRF・期間バリデーション本体（S1/S6 所有・消費のみ）。

## 技術制約・準拠事項

- **読み取り側のみ・スキーマ非変更（既定）**: マイグレーションを行わない（goose 最新 00009 のまま・`make db-snapshot` 不要）。品質メタは `ListSensorReadingsInRange`（期間内全行）を Go の品質純関数で走査して算出。新規 SQL を足す場合も走査/集計の SELECT に限り `make sqlc`（DDL なし）。
- **計算層と描画層の分離（ガードレール⑧）**: 異常検知/率集計は純粋 Go（`internal/chart` の `[]float64` 入出力・**time 非依存**）。recorded_at 差分（間隔列）の算出は **handler 境界**で行い純関数には `[]float64`（秒）を渡す（`dailyStatRows`/`vpdHourlyRows` 作法）。重い統計はアプリで計算せず CSV へ出す。
- **「物理異常」は受信 CHECK の内側**: temperature −40〜125・humidity 0〜100 は受信時に既に強制済み。本フェーズは農学的にあり得ない値・据置故障・急変を対象（CHECK の追認ではない）。判定しきい値は research で確定し、ハードコードせず定数化。
- **欠測ギャップ可視化**: `connectNulls:false`（欠測点 nil で線分断）＋欠測区間 markArea。markArea は go-echarts JSON タグ不具合を避け `vpd_echarts.go` の自前注入方式（小文字 `yAxis`/`xAxis` キー）を流用。返り値 JSON は `encoding/json`（SetEscapeHTML=true）で HTML 安全化（既存 `ChartOptionJSON` の不変条件を維持）。
- **通信遅延は既存流用**: `formatDelay(recordedAt,createdAt)`（負値0クランプ）を再実装しない。期間集計（平均/最大遅延）が要るなら同関数の素（差分秒）を使う。
- **依存方向**（structure.md）: 下向き一方向。`internal/chart` 最下流純粋性を維持。`internal/domain` に品質フラグ列挙を置く場合は純粋（fmt のみ・§100 Go 定数+列挙）。view は repository/service を import せず、品質フラグの `Label()` 等 domain 表示メソッドのみ参照可。所有者認可は `internal/authz`（`RequireDeviceOwner`）集約・非所有は 404。
- **イミュータブル**: sqlc 生成構造体は読取専用。handler で View / 品質判定結果を組み立てる。`internal/chart` 純関数は入力スライスを破壊しない。
- **マスタ/列挙＝Go 定数**（structure.md §100）: 品質フラグを列挙化するなら `domain.QualityFlag`（VARCHAR 化はしない＝DB に持たないため CHECK 不要・純粋 Go 列挙）。
- **言語**: 日本語コメント・エラー・コミット・UI ラベル・バッジ文言。コード識別子は英語。
- **TDD**: 80% 以上。Zスコア（σ≈0 のゼロ除算回避・|z|>k 境界）・IQR（Q1/Q3・1.5·IQR 境界・小標本）・stuck/flatline（連続ラン長・N 境界・全同値/全相異）・物理範囲/急変（境界値・空入力）・欠測率/間隔一貫性（等間隔=率0・抜け区間・単一点）・品質バッジ合成（緑/黄/赤しきい値）・欠測ギャップ（連続欠測区間抽出・先頭/末尾欠測・全欠測）・`connectNulls:false` の nil 出力・markArea 注入（小文字キー・空ギャップ）・所有者認可（非所有→404）・**無回帰**（P2 オーバーレイ・S6 フィルタ/一覧/通信遅延・P4 CSV/帳票が従来どおり）。

## 受け入れ基準（概略）

1. **レコード品質フラグ**: 各計測行に欠測（直後）/stuck・flatline/物理異常/外れ値（Zスコア or IQR）の判定が付与され、readings 一覧でフラグ列（バッジ/アイコン）として確認できる。
2. **期間品質メトリクス**: 表示期間の**欠測率（%）・サンプリング間隔の一貫性（σ/CV）・通信遅延（既存 `formatDelay` 流用）**が集計表示され、空期間/単一点が安全に扱われる。
3. **品質バッジ**: 期間/デバイスの総合品質が信号色バッジ（緑/黄/赤）で示され、判定基準（欠測率・外れ値・stuck・一貫性の合成）が明確。研究用詳細表示で農家向け平易表示は含まない（P13 分離）。
4. **欠測ギャップ可視化**: device-show グラフで欠測区間は線が分断（`connectNulls:false`）され、連続欠測区間が markArea でハイライトされる。go-echarts JSON タグ不具合を避けた自前注入で正しく描画される。
5. **物理異常の定義**: 「物理異常」が受信 CHECK（−40〜125 / 0〜100）の追認ではなく、その内側の農学的にあり得ない値・据置故障・急変を対象としていることがテスト/仕様で明確。
6. **スキーマ非変更（既定）**: マイグレーションを行わない（goose 00009 のまま・`make db-snapshot` 不要）。品質メタはすべて読み取り時計算（`ListSensorReadingsInRange` の全行を Go 純関数で走査）。永続フラグ列を採る場合のみ design で根拠（人手判定/受信タグ）を明示し expand-contract 移行＋スナップショット再生成。
7. **計算層分離（ガードレール⑧）**: 異常検知/率集計は `internal/chart` の純粋 Go（time 非依存）に集約。重い統計（STL/Mann-Kendall 等）はアプリ内で計算・描画しない。
8. **器差の線引き**: 器差は限定版（同一 locality 粗突合せ・「同一地域≠同一ハウス」明示）か P10 defer のいずれかで、本フェーズの確定スコープ（単一 device 品質メタ）が崩れていない。
9. **無回帰**: P2 統計オーバーレイ・S6 期間フィルタ/一覧/通信遅延/ページネーション・P4 CSV/集計帳票・E1 グラフ移行が従来どおり動作。所有者でない device への品質メタ表示は 404。
10. **モック整合**: 品質バッジ・フラグ列・メトリクスボックス・欠測ギャップ凡例がモック（device_show.html・readings.html＋style.css 正本）に反映されている（グラフ内部の connectNulls/markArea 描画は反映例外）。
11. **テスト 80% 以上**。

## 未確定事項・要確認（設計フェーズで決定）

- **① スキーマ：永続品質フラグ列を足すか（最重要・design の最初の論点）**: 既定は**読み取り時計算・スキーマ非変更**（P2/P3/P4 踏襲・ガードレール②/⑧）。永続列（例 `sensor_readings.quality_flags` bitmask/VARCHAR）を足すのは **(a) 自動導出できない人手のキュレーション判定を保存したい／(b) 受信 API でタグ付けしたい／(c) 毎読の再走査が重い**ときに限る。本フェーズの規模（同一区間を一度走査）では再計算は軽く、(a)(b) の確定要件が無ければ**スキーマ非変更を強く推奨**。ガードレール③の「最初から想定」は affordance（後付け可能性）であって DDL 必須ではない点を確認。
- **② 期待サンプリング間隔の決め方（欠測率の分母）**: 欠測率は「期待間隔」が要る。firmware/seed は約5分間隔。**(a) 観測間隔の中央値から推定／(b) 既定定数（5分）／(c) device 設定列（スキーマ＝避けたい）**のいずれか。間隔が運用で変わる可能性も含め research/引継ぎメモで確認。
- **③ 外れ値の手法（Zスコア vs IQR vs 併用）**: 温湿度の昼夜変動が大きい系列で大域 Zスコア/IQR は昼夜差を外れ値と誤検出しうる。**日内/移動窓ベース（ローリングσ法＝既存 SMA±kσ）**にするか、大域にするか。しきい値 k・IQR 係数は research。
- **④ stuck/flatline・物理異常・急変のしきい値**: 連続同値 N 回（センサー粒度・5分間隔で何回連続を異常とするか）・農学的下限上限・変化率閾値。沖縄の実環境（高湿度が長く続くのは正常＝湿度100%張り付きを安易に異常としない）をユーザーの実地知見で確定。
- **⑤ 品質バッジの合成式と置き場所**: 緑/黄/赤の境界（欠測率○%・外れ値○件・stuck 有無・一貫性 CV○）の合成ルール。device-show と readings の双方に出すか片方か。dashboard のデバイスカード（S3）に小バッジを出すかは将来。
- **⑥ 器差スコープ**: P10 defer か、同一 locality 粗突合せの限定版か。限定版なら「同一地域≠同一ハウス」をどう UI で明示するか。
- **⑦ 品質フラグの CSV 連携（P4 連携）**: P4 の CSV 出力に品質フラグ列（行ごとの欠測直後/stuck/外れ値）を非破壊追加するか。研究者が外部ツールで品質込みに解析できる利点があるが、本フェーズ必須か P4 への後追いかを判断。
- **⑧ 品質フラグの表示単位**: レコード単位の細かいフラグを一覧で全行に出すと煩雑。バッジ集約（期間サマリ）を主にし、行フラグは「異常行のみ強調」等の出し方にするか。
- **⑨ domain 列挙化の要否**: 品質フラグを `domain.QualityFlag`（§100 純粋列挙・`Label()`）にして view→domain 表示メソッドで描くか、handler/component 整形に留めるか。

--- spec-init 本文 ここまで ---
