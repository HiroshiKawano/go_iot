# フェーズ4（分析ロードマップ）spec-init プロンプト: センサーデータ CSV エクスポート＋集計表（任意期間×項目フィルタの CSV ダウンロード・日次/時間別の集計帳票）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: sensor-data-export
> 位置づけ: [分析アイデアメモ.md](../分析アイデアメモ.md) 第1章「実装ロードマップ」の**フェーズ4**〔明示〕（引継ぎメモ §11 の確認待ち候補にも挙がる）。新規画面ではなく、実装済み **readings（S6 ＝センサーデータ履歴）** 画面の**データ主権機能の拡張**＝(1) 任意条件で計測データを外部へ出す **CSV ダウンロード**と、(2) 画面で見せる**日次・時間別の集計帳票**。Ambient 脱却（自前保存したデータを自由に持ち出して再解析できる）の核。本格統計（回帰/分散分析/SARIMA）はアプリ内で描かず CSV で外部ツールへ出す前提（ガードレール⑧）。
> 確度: 〔明示〕（分析アイデアメモ フェーズ4・引継ぎメモ §11 の確認待ち候補。データ主権はメモ最小ゴール＝Ambient 代替/自前保存の延長線で、提案型案件ゆえ積極採用＝付録C-3 の方針）。
> 前提セッション: **S6（sensor-readings-history。期間フィルタ from/to → BETWEEN 区間写像・集計ボックス・ページネーション・通信遅延。本フェーズはこの画面とフィルタ基盤の直接の拡張）**／ **P2（temp-humidity-chart-stats。純粋統計層 `internal/chart/stats.go`＝`Mean`/`MinMax`/`DiurnalRange`/`StdDev`/`CV`。集計帳票の列はこれを流用。device-show の日次集計表 dailyStatRows パターンも参照）**／ **P3（vpd-dashboard。`internal/chart`＝`VPDSeries`/`TimeInRange`・`internal/domain/crop.go`＝`Crop.VPDRange()`。「適正帯滞在率」列はこれを流用。`device_show_vpd.go` の時間帯バケット vpdHourlyRows パターンも参照）**／ **P1（device-location-select。`devices.locality`＝地点軸・CSV のメタ列／集計軸キー）**／ S5（device-detail。readings への「もっと見る」導線）／ S1（web-foundation-auth。RequireAuth・所有者認可・ルーターグループ）
> **スキーマ変更なし（本フェーズは読み取り側のみ）**: CSV・日次/時間別集計・適正帯滞在率はすべて既存 `sensor_readings`（temperature＋humidity）＋ `devices`（locality/crop は P1/P3 で追加済）から**読み取り時に計算/整形**する。**マイグレーションは行わない**（goose 連番は **00009** のまま）。派生指標列・品質フラグ列の DB 追加はしない（ガードレール②＝必要になったフェーズで足す YAGNI）。**新規 SQL は集計/全行取得の SELECT のみ**（DDL なし）。
> 設計フェーズで参照:
> - 上位ロードマップ: 分析アイデアメモ.md フェーズ4（**任意期間×地点×項目フィルタの CSV ダウンロード**〔将来は Parquet 等〕／圃場/地点/期間別の**日次・時間別集計表**〔平均/最高最低/日較差/適正帯滞在率/結露時間〕を帳票化／▶ 主に**表・ダウンロード（非チャート）**・欠測は line `connectNulls:false` で可視化）／ 第3章ガードレール ①集計軸を最初に固定（圃場×地点×作物×期間×指標。どの軸でも横並び集計・CSV化できること。**本フェーズが「CSV化できること」の実体**）・⑧計算層と描画層の分離（**重い統計＝STL/Mann-Kendall/SARIMA は CSV 外出し**、アプリ内は軽い集計に集中）・②スキーマに派生指標/品質フラグを後付け可能に（実列追加は必要なフェーズで）・⑥作物メタデータ（適正帯滞在率は作物依存・無ければ既定しきい値）／ 付録A「A. 基本統計量」（平均μ・最高/最低・日較差ΔT・σ・CV）・「D①飽差VPD」（適正帯滞在率の素＝在帯割合）・「D③露点Td」（**結露時間の素・本フェーズは未実装＝P6 依存**）
> - 現スキーマ（権威）: docs/database_snapshot/table_definitions.md「sensor_readings」（temperature/humidity numeric(5,2)・recorded_at timestamptz・created_at＝受信時刻〔通信遅延算出に既用〕・(device_id, recorded_at DESC) 部分索引 WHERE deleted_at IS NULL）／「devices」（**locality VARCHAR(20)＝P1/00008・crop VARCHAR＝P3/00009 とも追加済**）。goose 連番の最新は **00009**。CSV/集計は2列＋メタ列から導出でき**スキーマ非変更**で足る。
> - 移行元・拡張対象コード（実コード確認済み・P3 マージ後の現状）:
>   - `internal/handler/readings.go` … **S6 の中心・本フェーズの主拡張点**。`ReadingsHandler{Repo ReadingsRepo}`・`Index`（GET /devices/:device/readings）。`parseDateBounds(from,to)→(fromTS,toTS,errs)`（YYYY-MM-DD → BETWEEN 区間・未指定は `distantPast`(1970)/`distantFuture`(9999) センチネル・to は end-of-day 拡張・JST 暦日）／`fetchResults`（件数→ページクランプ→`ListSensorReadingsPaginated`＋`GetSensorReadingsSummary` を同一区間で）／`buildSummary`（平均/最高/最低の整形・0件は "—"）／`buildReadingHistoryRows`／`formatDelay`（通信遅延）。**CSV・集計帳票はこのフィルタ写像（parseDateBounds）と区間取得を共有する**（同じ from/to で CSV・帳票・一覧が一致）。
>   - `internal/chart/stats.go` … 純粋統計層（`math` のみ依存・`[]float64` 入出力・gin/DB/templ/time 非依存）。**既存** `Mean`/`MinMax`/`DiurnalRange`/`StdDev`/`CV(values,eps)→(float64,bool)`。→ **日次/時間別集計表の 平均・最高・最低・日較差・σ・CV 列はこれを流用**（新規計算関数は最小）。
>   - `internal/chart/`（P3）… `VPDSeries(temps,hums []float64) []float64`（Tetens 式の VPD 系列）・`TimeInRange(values []float64, lower, upper float64) float64`（適正帯在帯割合 0..1）。→ **「適正帯滞在率」列はこれ＋`domain.Crop.VPDRange()` を流用**（device の作物で適正帯を引き、未設定は既定 0.3-1.5 kPa）。
>   - `internal/handler/device_show_vpd.go`（P3）… `vpdHourlyRows(rows, vpd, lower, upper)`＝生行を JST 時刻（hour-of-day 0-23）でバケット化し時間帯別に集計するパターン。**時刻が要る「時間別集計」はこの handler 境界バケット作法を温湿度へ一般化**（純粋層 stats に time を持ち込まない）。`device_show.go` の `dailyStatRows`（JST 暦日バケットの 平均/最高/最低/日較差/σ/CV）も日次帳票の手本。`jst`・`statEmptyMark="—"` を共用。
>   - `internal/domain/crop.go`（P3）… `Crop.VPDRange()→(lower,upper)`・`Label()`・未設定/不正は既定帯。→ 適正帯滞在率の作物別しきい値解決にそのまま使用（本フェーズで作物マスタの変更はしない）。
>   - `internal/domain/locality.go`（P1）… 地点キー。→ CSV のメタ列・集計軸（地点別の横断は外部ツール想定＝後述スコープ）。
>   - `db/queries/sensor_readings.sql` … `ListSensorReadingsPaginated`（BETWEEN＋LIMIT/OFFSET・一覧用）／`GetSensorReadingsSummary`（BETWEEN・期間集計）／`ListDailySensorAggregates`（**`recorded_at >= $2` の単一境界＝24h以外グラフ用で BETWEEN ではない**・DATE バケットの avg/max/min/count）。→ **CSV は「期間内全行（ページングなし）」が要る**ため `ListSensorReadingsInRange`（BETWEEN・ORDER ASC・LIMIT なし＝Paginated の非ページ版）を新設するか、CSV/帳票とも**取得した全行を Go 側（stats）で集計**するか設計判断（日次は `ListDailySensorAggregates` が BETWEEN でないため、BETWEEN 版を起こすか Go 集計に寄せる）。
>   - `cmd/server/main.go` L156-165 … `/devices/:device`・`/chart`・`/readings` の登録（`:device` パラメータ node 共有・GET は CSRF 対象外）。→ **CSV ダウンロード経路を同 node に追加**（GET・所有者認可・HTML でなくファイル応答）。
> - ビュー/モック（単一ソース運用の境界）: `internal/view/page/Readings.templ`・`internal/view/component/DeviceReadingsList.templ`／ `mocks/html/readings.html`（フィルタ `.filter-form`〔from/to date〕・集計 `.summary-grid`/`.summary-box`・一覧 `.data-table`・`.pagination`）＋`mocks/html/style.css`（正本）。**項目フィルタの拡張・CSV ダウンロードボタン・日次/時間別の集計帳票表は静的な「器」＝HTML/CSS ゆえモック反映必須**（feedback_mock_reflects_impl_visual・project_css_single_source）。CSV はファイル出力でグラフ描画を伴わないため**グラフ描画例外の対象外**（帳票表・ボタンはあくまで器＝反映対象）。
> - 命名・依存規約: .kiro/steering/structure.md（依存方向＝下向き一方向・`internal/chart` は最下流純粋・view→domain 表示メソッドのみ・§100 マスタは Go 定数）／ tech.md（sqlc・データアクセス方針）／ CLAUDE.md（**本フェーズはマイグレーション無し**＝`make db-snapshot` 不要。新規 SQL を足したら `make sqlc`）。

--- spec-init 本文 ここから ---

## 機能概要

センサーデータ履歴（readings／S6）画面に、蓄積した計測データを**外部へ持ち出す CSV エクスポート**と、**画面上の集計帳票（日次・時間別）**を追加する。狙いはデータ主権＝Ambient 脱却の核：自前保存した温湿度データを任意条件で CSV ダウンロードし、本格統計（回帰・分散分析・STL・SARIMA 等）は外部ツール（Excel / R / Python）で再解析できるようにする（アプリ内では描かない＝ガードレール⑧）。具体的には (1) **任意期間×項目フィルタの CSV ダウンロード**（既存 from/to 期間フィルタと同じ区間で、期間内の全計測行を生データとして出力。各行に device 名・**地点（locality）・作物（crop）のメタ列**を添え、外部ツールが地点別/作物別に横断・pivot できるようにする＝集計軸の CSV 化〔ガードレール①〕）、(2) **日次・時間別の集計帳票**（期間内を JST 暦日／時間帯でバケット化し、**平均・最高・最低・日較差・標準偏差・CV・適正帯滞在率**を表で見せる。適正帯滞在率は device の作物の VPD 適正帯〔`domain.Crop.VPDRange()`・未設定は既定 0.3〜1.5 kPa〕に対する在帯割合）。CSV・集計帳票・既存の一覧/集計ボックスは**同一の期間フィルタ（from/to → BETWEEN）を共有**し値が一致する。すべて既存 `sensor_readings`（temperature＋humidity）＋ `devices`（locality/crop）から**読み取り時に計算**し、**スキーマ変更・マイグレーションは行わない**（読み取り側のみ・ガードレール②/⑧）。S6 の既存機能（期間フィルタ・集計ボックス・一覧・ページネーション・通信遅延）は**無回帰で維持**する。

> **「結露時間」列について（重要・スコープ判断）**: 分析アイデアメモ フェーズ4 は集計帳票の列に「結露時間」を挙げるが、結露時間は**露点 Td**（付録A D③）に基づき、露点・病害リスクは**フェーズ6（dewpoint-disease-risk）**の領域で**本リポジトリに未実装**（葉面温度センサも無く気温近似の限界がある）。本フェーズの集計帳票は **平均/最高/最低/日較差/σ/CV/適正帯滞在率**までを確定スコープとし、**結露時間列は P6 で露点 Td を実装した後に同じ帳票へ非破壊追加する**（または最小の露点近似を本フェーズに含めるかを design で判断＝未確定事項）。

## 背景・現状

S6（sensor-readings-history）マージ後・P1/P2/P3 マージ後の現状は以下（実コード確認済み）。

- **readings 画面（S6）**: `internal/handler/readings.go` の `ReadingsHandler.Index`（GET /devices/:device/readings）。`parseDateBounds(from,to)` が YYYY-MM-DD を BETWEEN 用区間へ写す（未指定は `distantPast`(1970)/`distantFuture`(9999) センチネル・to は end-of-day 拡張・JST 暦日・形式不正は日本語エラーで検索スキップ）。`fetchResults` が同一区間で件数→ページクランプ→`ListSensorReadingsPaginated`＋`GetSensorReadingsSummary` を取得。`buildSummary`（平均/最高/最低・0件は "—"）・`buildReadingHistoryRows`・`formatDelay`（通信遅延 N秒）。**CSV・集計帳票はまだ無い**。
- **集計クエリ**: `db/queries/sensor_readings.sql` に `GetSensorReadingsSummary`（BETWEEN・期間の avg/max/min/count）・`ListSensorReadingsPaginated`（BETWEEN＋LIMIT/OFFSET）・`ListDailySensorAggregates`（**`recorded_at >= $2` の単一境界**＝グラフ用で BETWEEN ではない・DATE バケット）・`CountSensorReadingsInRange`。**「期間内全行（ページングなし）」を返すクエリと、BETWEEN 境界の日次集計クエリは無い**。
- **純粋統計層**: `internal/chart/stats.go`（`math` のみ依存・`[]float64` 入出力）に `Mean`/`MinMax`/`DiurnalRange`/`StdDev`/`CV` がある（集計帳票の列にそのまま使える）。`VPDSeries`/`TimeInRange`（P3）で適正帯滞在率が出せる。
- **時刻バケット作法**: `device_show_vpd.go` の `vpdHourlyRows` が生行を JST hour-of-day でバケット化（純粋層に time を持ち込まず handler 境界で）。`device_show.go` の `dailyStatRows` が JST 暦日バケットの 平均/最高/最低/日較差/σ/CV を出す。**温湿度の時間別集計はこれらを一般化すれば足りる**。
- **メタ**: `devices.locality`（P1・地点キー）・`devices.crop`（P3・作物キー）とも追加済み。`domain.Locality`・`domain.Crop.VPDRange()` で表示名・適正帯を引ける。
- **ビュー/モック**: `Readings.templ`・`DeviceReadingsList.templ`／`mocks/html/readings.html`（`.filter-form`・`.summary-grid`/`.summary-box`・`.data-table`・`.pagination`）。**項目フィルタ・CSV ボタン・集計帳票表は存在しない**。
- **DB は読み取り側のみで足りる**: CSV/集計/滞在率はすべて2列＋メタ列から導出でき、スキーマ非変更で計算可能。

## このセッションのスコープ（実装対象）

### CSV エクスポート（ファイル応答・新規 GET 経路）

- **エンドポイント**: device 配下の GET 経路を新設（例 `GET /devices/:device/readings.csv` または `/devices/:device/readings/export.csv`・GET ゆえ CSRF 対象外・**所有者認可必須**＝`authz.RequireDeviceOwner` を readings と同様に通す）。HTML でなく**ファイル応答**（`renderPage` を経由しない）。経路の正確な形（拡張子 path か `?format=csv` か）は design 判断（未確定事項）。
- **フィルタ**: 既存 `parseDateBounds`（from/to）を**共有**（CSV と画面一覧・帳票が同一区間で一致）。加えて**項目フィルタ**（出力する列＝温度/湿度。将来 VPD/露点を増やす余地を残す）。
- **出力内容**: 期間内の**全計測行（ページングなし・recorded_at 昇順）**。列は最低 `recorded_at`（JST・ISO 等）・`temperature`・`humidity`、加えて**メタ列**（device_id / device 名 / 地点 locality / 作物 crop）を行または先頭メタ行に持たせ、**外部ツールが地点別/作物別に横断・pivot できる**ようにする（集計軸の CSV 化＝ガードレール①）。欠測・通信遅延列を含めるかは design。
- **実装**: 標準 `encoding/csv` を使用。**大期間でもメモリを撹乱しないストリーミング**（`c.Writer` へ逐次 `Write`＋`Flush`／必要なら全行クエリをカーソル/バッチ取得）。`Content-Type: text/csv`＋**文字コード**（Excel で文字化けしない UTF-8 BOM 付与 or Shift_JIS＝要判断・未確定事項）＋`Content-Disposition: attachment; filename=...`（**日本語ファイル名は RFC 5987 `filename*` エンコードの罠**に注意＝device 名＋期間を入れるなら ASCII フォールバック＋`filename*`）。
- **全行取得クエリ**: `ListSensorReadingsInRange`（BETWEEN・ORDER BY recorded_at ASC・LIMIT なし）を新設するか、既存取得経路を非ページ化するかは design（`make sqlc`）。**DDL は伴わない**。

### 集計帳票（画面・日次/時間別）

- **日次集計**: 期間内を JST 暦日でバケット化し、各日の **平均/最高/最低/日較差/σ/CV/適正帯滞在率** を表で見せる（device-show の `dailyStatRows`＝P2 と列が重なる部分は流用・**重複実装を避ける**）。日次クエリは `ListDailySensorAggregates` が BETWEEN でないため、**BETWEEN 版を起こすか、CSV 用に取得した全行を Go（stats）で集計**するかを design 判断（後者なら新 SQL 不要）。
- **時間別集計**: 期間内を JST 時間帯（hour-of-day 0-23 など）でバケット化し時間帯別に集計（`vpdHourlyRows`＝P3 の作法を温湿度へ一般化・時刻は handler 境界）。バケット幅（1時間 or 昼夜区分）は design。
- **適正帯滞在率列**: device の作物の `Crop.VPDRange()`（未設定は既定 0.3-1.5 kPa）に対し、`chart.VPDSeries`＋`chart.TimeInRange` で各バケットの在帯割合を算出（瞬間値/平均でなく「その期間の何%が適正帯か」＝ガードレール⑤・P3 と一貫）。
- **結露時間列**: **本フェーズはスコープ外**（露点 Td＝P6 依存）。帳票の列構造は P6 で露点列を非破壊追加できる形にしておく。
- **計算層分離**: 集計は純粋 Go（stats）に集約、**本格統計（STL/ACF/Mann-Kendall/回帰/分散分析/SARIMA）はアプリ内で計算・描画しない**＝CSV で外部ツールへ出す（ガードレール⑧）。

### View / templ / モック反映

- `Readings.templ`／`DeviceReadingsList.templ`（または専用 component）に **CSV ダウンロードボタン**（フィルタ条件を引き継ぐリンク／フォーム）・**項目フィルタ UI**・**日次/時間別集計帳票表**を追加（イミュータブル＝handler で View を組み立て）。集計表は `.data-table` 等モック既存クラスを流用、独自クラス新設は最小。
- **モック反映**: 項目フィルタ拡張・CSV ボタン・集計帳票表は `mocks/html/readings.html`＋`mocks/html/style.css`（正本）へ反映し `make sync-css`（feedback_mock_reflects_impl_visual）。CSV のファイル出力自体は描画を伴わずモック対象外（画面に出る器＝ボタン/表は反映対象）。

## スコープ外（このセッションでやらないこと）

- **露点 Td・結露帯・結露時間・病害リスク**（フェーズ6 ＝ dewpoint-disease-risk）。本フェーズの集計帳票は 平均/最高/最低/日較差/σ/CV/適正帯滞在率 まで。結露時間列は P6 で露点を実装後に非破壊追加（最小露点近似を含めるかは未確定事項）。
- **派生指標列・品質フラグ列の DB 追加・マイグレーション**（読み取り時計算で足りる＝ガードレール②）。`sensor_readings`・`devices` スキーマ・受信 API・既存クエリ本体の変更。**本フェーズの新規 SQL は集計/全行取得の SELECT のみ**（DDL なし＝`make db-snapshot` 不要）。
- **多地点の横断集計 UI**（複数 device を地点/作物で串刺し集計してアプリ内表示する）＝フェーズ10（multipoint-compare）・フェーズ13（farm-benchmark-share）。本フェーズは**単一 device の CSV/帳票**を基本とし、**横断は CSV にメタ列〔locality/crop〕を持たせて外部ツールで行う**（ガードレール①・⑧）。device をまたぐ集約ダッシュボードは作らない。
- **本格統計のアプリ内計算/描画**（STL・ACF・Mann-Kendall・回帰・分散分析・Holt-Winters・SARIMA）＝CSV 外出し（ガードレール⑧・フェーズ8/15）。**Parquet・その他フォーマット**（将来拡張・本フェーズは CSV のみ）。
- **VPD 適正帯ダッシュボード本体・作物マスタの変更**（フェーズ3 ＝ vpd-dashboard 所有・本フェーズは `Crop.VPDRange()`/`VPDSeries`/`TimeInRange` を**消費するのみ**）。**温湿度グラフ・統計オーバーレイの変更**（P2/E1 所有・無回帰維持）。
- **CSV インポート・API での一括取得・定期バッチ出力・スケジュール配信**。**農家向け共有 URL・帳票の PDF 化**（フェーズ13）。
- 認証・所有者認可・MethodOverride・CSRF・期間バリデーション本体（S1/S6 所有・消費のみ）。device-show 以外の他画面（dashboard・alert 系）への展開。

## 技術制約・準拠事項

- **読み取り側のみ・スキーマ非変更**: マイグレーションを行わない（goose 最新は 00009 のまま）。新規 SQL は SELECT（集計/全行取得）に限り `make sqlc` のみ。CSV/集計/滞在率はすべて読み取り時計算（ガードレール②/⑧）。
- **集計軸の固定（ガードレール①）**: 期間×項目を確定軸とし、**locality/crop を CSV のメタ列**として常に出す（後から地点別/作物別に外部で横並びできる）。後付けで軸を足してもスキーマを壊さない設計。
- **計算層と描画層の分離（ガードレール⑧）**: 集計は純粋 Go（`internal/chart` の `[]float64` 入出力・gin/DB/templ/**time** 非依存）。時刻が要る日/時間バケットは **handler 境界**で（`dailyStatRows`/`vpdHourlyRows` 作法）。重い統計はアプリで計算せず CSV へ出す。
- **CSV 仕様**: 標準 `encoding/csv`。**ストリーミング**で大期間に耐える（`c.Writer` 逐次 Flush）。**文字コードは Excel 互換**（UTF-8 BOM 付与 or Shift_JIS＝要判断）で日本語ヘッダが文字化けしないこと。`Content-Disposition: attachment` ＋ 日本語ファイル名は `filename*`（RFC 5987）＋ASCII フォールバック。改行・カンマ・ダブルクォートのエスケープは `encoding/csv` に委ねる。
- **依存方向**（structure.md）: 下向き一方向。`internal/chart` の最下流純粋性を維持。View は repository/service を import しない。pgtype→float 変換は handler 境界（`pgconv`）。
- **フィルタ共有**: CSV・集計帳票・既存一覧/集計ボックスは `parseDateBounds` の同一区間を使い、同じ from/to で**値が一致**すること（実装の単一源）。
- **イミュータブル**: sqlc 生成構造体は読取専用。handler で View / CSV 行を組み立てる。
- **マスタ＝Go 定数**（structure.md §100）: 作物/地点は `domain.Crop`/`domain.Locality` を消費（テーブル化しない・本フェーズで変更なし）。
- **言語**: 日本語コメント・エラー・コミット・CSV ヘッダ。コード識別子は英語。
- **TDD**: 80% 以上。parseDateBounds 共有（CSV と画面の区間一致）・全行取得（ページなし・昇順・空期間）・CSV 整形（ヘッダ/メタ列/エスケープ/文字コード/Content-Disposition/filename・大量行ストリーミング）・日次/時間別バケット（JST 境界・空バケット・日跨ぎ）・集計列（Mean/MinMax/DiurnalRange/σ/CV の流用）・適正帯滞在率（crop 別しきい値・未設定既定・空データ）・所有者認可（非所有→404）・S6 無回帰（既存フィルタ/集計/ページネーション）。

## 受け入れ基準（概略）

1. **CSV ダウンロード**: 既存 from/to 期間フィルタ（＋項目フィルタ）と**同一区間**で、期間内の全計測行（ページングなし・recorded_at 昇順）が CSV で取得でき、各行に device 名・地点（locality）・作物（crop）のメタ情報が付く。`Content-Disposition: attachment` でファイルとして保存される。
2. **文字コード/エスケープ**: 日本語ヘッダ・値が Excel で文字化けせず（UTF-8 BOM or Shift_JIS）、カンマ/改行/引用符を含む値が `encoding/csv` で正しくエスケープされる。
3. **集計帳票（日次/時間別）**: 期間内を JST 暦日／時間帯でバケット化した **平均/最高/最低/日較差/σ/CV/適正帯滞在率** が画面の表で確認でき、空期間・空バケットが安全に扱われる。
4. **適正帯滞在率**: device の作物の `Crop.VPDRange()`（未設定は既定 0.3-1.5 kPa）に対する在帯割合が `VPDSeries`/`TimeInRange` で算出され、「その期間の何%が適正帯か」を表す（瞬間値/平均でない）。
5. **スキーマ非変更**: マイグレーションを行わない（goose 最新 00009 のまま・`make db-snapshot` 不要）。新規 SQL は SELECT のみ（DDL なし）。CSV/集計はすべて読み取り時計算。
6. **大期間で安定**: 長期間の CSV でもストリーミングでメモリを撹乱せず完了する。
7. **無回帰**: S6（期間フィルタ・集計ボックス・一覧・ページネーション・通信遅延・形式エラー時の挙動）が従来どおり動作する。所有者でない device への CSV/帳票アクセスは 404（列挙防止）。
8. **集計軸の CSV 化（ガードレール①）**: CSV のメタ列（locality/crop）により、外部ツールで地点別/作物別に横断・pivot できる。
9. **計算層分離（ガードレール⑧）**: 本格統計（STL/回帰/SARIMA 等）はアプリ内で計算・描画せず CSV で外部へ出す方針が守られている。
10. **モック整合**: 項目フィルタ・CSV ボタン・集計帳票表がモック（readings.html＋style.css 正本）に反映されている。
11. **テスト 80% 以上**。

## 未確定事項・要確認（設計フェーズで決定）

- **結露時間列（最重要・スコープ判断）**: 露点 Td（付録A D③・P6 dewpoint-disease-risk）に依存し本リポジトリ未実装。**(a) P6 へ defer して列構造だけ非破壊追加可能にする／(b) 葉面温度が無い前提で最小の露点近似（気温≦Td の高湿度継続時間）を本フェーズに含める**のどちらかを design で確定。葉面温度センサが無く気温近似の限界がある旨を注記。
- **CSV 文字コード（重要）**: UTF-8 BOM 付与か Shift_JIS か。眞境名さん（客）が Excel で開くか R/Python で読むかで最適が変わる（Excel 直開きなら BOM 付き UTF-8 か Shift_JIS、R/Python なら素の UTF-8）。引継ぎメモ・並行デモで確認。
- **CSV エンドポイント形**: 拡張子 path（`/devices/:device/readings.csv`）か `?format=csv` か別経路か。`:device` node の既存子経路（/edit・/chart・/readings）と競合しない形。
- **CSV ファイル名**: device 名＋期間を入れるか固定か。日本語を含めるなら `filename*`（RFC 5987）＋ASCII フォールバック。
- **メタ列の持たせ方**: 各行に locality/crop を反復付与するか、先頭にメタ情報ブロック（device 名/期間/地点/作物）を1行で出してからデータ部にするか。外部ツールでの pivot しやすさを優先。
- **項目フィルタの範囲**: 温度/湿度のみか、VPD・絶対湿度などの派生指標列も選べるようにするか（VPD は P3 の `VPDSeries` で算出可・露点は P6 まで不可）。
- **集計帳票の置き場所・日次クエリ方針**: readings 画面の延長で出すか（device-show の `dailyStatRows`＝P2 と列が重なるため重複を避ける）。日次集計を BETWEEN 版 SQL で起こすか、CSV 用に取得した全行を Go（stats）で集計して SQL を増やさないか。
- **時間別バケット幅**: 1時間（hour-of-day）か昼夜区分か。期間が長いときの粒度（時間別は短期間のみ・日次は長期間）出し分け。
- **多地点横断の境界**: 本フェーズは単一 device＋CSV メタ列で外部横断（P10/P13 で UI 横断）。この線引きで良いか（客が「複数圃場を1ファイルに」を望む場合の出し方）。
- **欠測の扱い**: CSV で欠測行をどう表現するか（行を出さない／空セル）。集計帳票での欠測バケットの表示（"—"）。

--- spec-init 本文 ここまで ---
