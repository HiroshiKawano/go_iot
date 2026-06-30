# 実装計画: sma-window-select（日スケール SMA 窓のユーザー選択）

> **前提（Foundation なし）**: 本 spec はスキーマ/クエリ/マイグレーション/templ/CSS/HTMX 変更なし（読み取り時計算・採用案(a) 凡例トグル併置）。よって Foundation 系タスク（goose / `make db-snapshot` / `sqlc generate` / `templ generate` / `make sync-css`）は不要。すべて Go（`internal/chart` 純粋層・`internal/handler`）の加算改修。
>
> 実装は上から順に `/tdd`（RED→GREEN→REFACTOR）で進める（逐次・`(P)` なし）。

- [x] 1. chart 層: 日スケール SMA 追加系列の入力契約と描画
- [x] 1.1 ChartSpec に日スケール SMA 系列の入力契約を追加し、ChartOptionJSON が各系列を凡例トグル（既定オフ）の細線として描画する
  - 日スケール SMA 系列（ラベル＋値列）の型を新設し、ChartSpec へ末尾非破壊追加する（nil/空で完全に従来挙動＝P2/P5 の後方互換不変条件を維持）
  - ChartOptionJSON が既存系列（生実測=series[0]／P2 SMA／正常帯／乖離率）の描画後に各日スケール SMA を追加系列として組む（基準色・細線 dashed・データ点マーカーなし・端ラベルなし）
  - 凡例 data に各ラベルを追加し既定オフ（selected:false）。SMA のみで EMA/WMA・ローソク足（OHLC）・売買シグナル/交差の UI や文言・有意判定の出力を持たない。凡例ラベルは中立な「移動平均 N日」
  - 観測可能完了: 日スケール系列を渡すと option JSON に各「移動平均 N日」系列と legend.selected[ラベル]==false・dashed/symbol なしが含まれ、加重平滑（EMA/WMA）や OHLC 系列は一切現れず、系列空では従来 JSON とバイト等価（既存 echarts テストが無改変で緑）
  - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2, 2.3, 4.1, 4.2, 4.3, 4.4_

- [x] 2. handler 層: 窓集合の決定と点数換算
- [x] 2.1 ビュー別の日スケール窓集合を返すヘルパと、サンプリング間隔から点数/日を推定するヘルパを追加する
  - 窓集合: 7d→{3日,7日}／30d→{3日,7日,14日}／24h・3d→なし（最大3本でクラッタ抑制・可視スパン以下の窓のみ）
  - 点数/日: 間隔秒列の中央値から 86400/中央値、行不足や算出不能時は 288（5分・288点/日）へフォールバック（将来の間隔変更に追従）
  - 観測可能完了: table-driven テストで 24h/3d→空・7d→2本・30d→3本・各ラベル文字列が一致し、5分間隔→≈288／10分→≈144／0-1行→288 を返す
  - _Requirements: 1.1, 2.4, 3.1, 5.2_

- [x] 3. handler 層: ルックバック取得と可視窓スライスによる日スケール SMA 組立
- [x] 3.1 buildChartArea を分岐拡張し、7d/30d でルックバック取得→可視窓スライス→日スケール SMA を全系列計算して可視窓へスライスし温度/湿度 spec に付与する
  - 窓なし（24h/3d）は取得起点 periodSince のままで既存パスを一切変えない（完全無回帰）
  - 窓あり（7d/30d）は取得起点を periodSince−maxWindowDays へ広げ、visibleStart 以降の可視行を既存パス（生実測線・overlaySpec・gap検出・日次表・VPD/露点）へ渡す（今日と同一入力＝バイト等価）
  - 日スケール SMA は全系列で算出し、2桁丸め（chartRound2）して可視窓へスライスし spec に付与する。可視窓0件は従来どおり HasData=false
  - 観測可能完了: Querier モックで 24h/3d は取得起点=periodSince かつ option に日スケール系列なし、7d/30d は取得起点=periodSince−maxWindow かつ option の生実測点数が periodSince 以降のみで option に「移動平均7日」等を含み、可視窓0件で HasData=false
  - _Requirements: 1.1, 1.4, 5.1, 5.2, 6.1, 6.2, 6.4, 7.1, 7.2_
  - _Depends: 1.1, 2.1_

- [x] 4. handler 層: 欠測グリッドの日スケール SMA carry-forward
- [x] 4.1 applyGapGrid を拡張し、欠測スロットで各日スケール SMA 系列も既存 SMA と同様に直前値で carry-forward する
  - 元 spec 非破壊（値コピー）の不変条件を維持し、拡張グリッド上で各系列を Labels と同長に揃える
  - 観測可能完了: 欠測ありフィクスチャで拡張後の各日スケール系列長 == Labels 長、P5 の欠測 markArea（GapBands）が従来どおり描画され、元 spec が破壊されない
  - _Requirements: 1.1, 6.2_
  - _Depends: 1.1, 3.1_

- [x] 5. 結合・無回帰検証: handler→chart→templ
- [x] 5.1 期間切替フラグメントと初期表示で、日スケール系列の有無・既存無回帰・空データ・モック整合を結合検証する
  - Show/Chart 両経路（buildChartArea 共有）で 24h/3d は従来同一・7d/30d は日スケール系列を含む option を返す
  - 既存オーバーレイ（P2 SMA/正常帯/乖離率・カード・日次表・VPD/露点）が可視窓で従来同一、templ DeviceChartArea が日スケール系列を含む option script を描画する
  - 採用案(a)につき新規静的 UI を追加せず DeviceChartArea.templ/モックは無改修（8.1 は該当 UI なしで充足・グラフ内部の系列描画は 8.2 のモック反映例外）
  - 観測可能完了: httptest で GET /devices/:id（7d/30d）の HTML/option script に「移動平均N日」を含み 24h/3d には含まない、period 切替フラグメントが #device-chart-area の中身を返す、計測0件で「データはまだありません」を返す
  - _Requirements: 1.3, 3.1, 3.2, 6.1, 6.2, 6.4, 8.1, 8.2_
  - _Depends: 3.1, 4.1_

- [x] 6. 検証: クライアント無回帰と実機スモーク
- [x] 6.1 凡例トグル・温湿度連動・期間切替・計算系列の表示丸めをブラウザ/実機で確認する（手動スモーク）
  - ローカル実機スモーク完了（2026-06-30・フレッシュバイナリ :8090・device3「長期トレンドテスト(沖縄・10年)」/多年データ・chrome-devtools）。本番リデプロイ後の実機スモークは次回デプロイ時に実施（`bash deploy/redeploy.sh`）。
  - 凡例から日スケール系列を表示/非表示でき、生実測線が主役（既定オフ）であること
  - 温湿度2グラフの echarts.connect() 連動・期間切替（アクティブ表示/URL同期）が従来どおりで、追加系列に端ラベルが付かず生実測線のみに付くこと
  - chartRound2 により計算系列が2桁表示で破綻せず、30d×14日窓の応答が体感上従来と同等であること
  - 観測可能完了: ローカル（`make seed-trendsensor` の多年データ）で上記が確認でき、本番リデプロイ後の実機スモークで温湿度グラフが正常表示される
  - _Requirements: 2.1, 2.3, 6.3, 6.5_
