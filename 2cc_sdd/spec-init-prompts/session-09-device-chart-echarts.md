# セッション9（拡張）spec-init プロンプト: デバイス詳細グラフの go-echarts 移行（自作SVG → Apache ECharts）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: device-chart-echarts
> 位置づけ: S1〜S8 完了後の**拡張スペック**。新規画面ではなく、実装済み S5（device-detail）のグラフ領域のレンダリング基盤を差し替えるリファクタ／移行。画面の業務要件・URL・期間切替の UX は維持し、描画エンジンのみ置換する。
> 命名注記: feature-name は device-chart-echarts。対象画面は S5 と同じ device-show（GET /devices/{device}、internal/view/page/DeviceShow.templ）。
> 実行例: /kiro-spec-init "（本文を貼り付け）"
> 前提セッション: S5（device-detail。グラフ領域・期間切替 HTMX・ホバー連動が実装済みであること）
> 設計フェーズで参照:
> - 置換対象の現仕様: システム構成図.md「サーバサイド SVG グラフ自作」、HTMX実装ガイド(動的).md §3.3（id一覧）・§4 device-show・§10-D フルフラグメント swap
> - 現コード（移行元）: internal/chart/svg.go（LineChartSVG・約318行）/ internal/chart/hover.go（LineChartHoverPoints）/ internal/view/component/DeviceChartArea.templ / internal/view/layout/App.templ の linkedCharts()（約50行のインライン script）/ internal/handler/device_show.go の buildChartArea・rawSeries・hoverJSON / internal/view/component/views.go の DeviceChartAreaView / internal/view/static.go（//go:embed all:public）
> - データ源（不変）: db/queries/sensor_readings.sql の ListRecentSensorReadings、docs/database_snapshot/ の sensor_readings
> - ライブラリ: go-echarts v2（github.com/go-echarts/go-echarts/v2、v2.7.2 / 内部 ECharts 5.4.3）。render パッケージ RenderSnippet / opts パッケージ（MarkPoints・Tooltip・AxisPointer・DataZoom 等）
> - モック: mocks/html/device-show.html / mocks/html/style.css（**グラフ描画本体は反映対象外＝feedback_mock_graph_rendering_exception。器＝期間ボタン・見出し・カード枠・配色のみモック準拠**）

--- spec-init 本文 ここから ---

## 機能概要

デバイス詳細画面（device-show）の温度・湿度グラフは現在、サーバーサイドで自作した SVG 文字列（internal/chart/svg.go）を @templ.Raw で埋め込み、ホバーの十字ポインター・ツールチップ・温湿度2グラフ連動を Alpine.js（App.templ の linkedCharts）で自前実装している。本セッションでは、この描画基盤を **go-echarts v2（Apache ECharts の Go バインディング）** へ移行する。グラフの見た目・期間切替（24h/3d/7d/30d）の UX・温湿度連動ホバー・最高/最低・右端現在値といった**機能要件は維持**したまま、描画と対話を ECharts 標準機能（tooltip / axisPointer cross / markPoint / connect / dataZoom）へ置き換え、自作 SVG 生成コードと Alpine ホバーコードを撤去する。これにより処理負荷（ペイロード・ホバー計算量）とグラフ拡張性を改善する。

## 背景・現状

S5 で実装済みの現状は以下の通り（実コード確認済み）。

- **サーバー側描画**: buildChartArea（internal/handler/device_show.go）が ListRecentSensorReadings で生データを取得 → rawSeries で温度・湿度それぞれ単一系列の chart.Series に写像 → LineChartSVG（internal/chart/svg.go）で SVG 文字列を2本生成 → さらに LineChartHoverPoints + hoverJSON でホバー用の点列 JSON を2本生成。全期間（24h/3d/7d/30d）とも生データ単一折れ線で、ダウンサンプリングは無い。
- **テンプレート**: DeviceChartArea.templ が @templ.Raw(SVG) で SVG を埋め込み、x-data="linkedCharts({...HoverJSON...})" でホバーを初期化。
- **クライアント対話**: App.templ のインライン script linkedCharts() が mousemove ごとに点列を線形走査（O(N)）して最寄り点を求め、両グラフの縦線・横線・ドット・値/時刻ツールチップを同一 index で更新（温湿度連動）。
- **期間切替**: 期間ボタンに hx-get="/devices/{device}/chart?period=..." + hx-target="#device-chart-area" + hx-swap="innerHTML"。Chart ハンドラがフラグメント（DeviceChartArea）を返す。hx-push-url で URL も同期。
- **アセット**: Alpine.js / HTMX は CDN 読込（App.templ <head>）。echarts は未導入。

**移行の動機（本セッションで実施した調査・負荷比較の要点）**:
- 現状は同じ座標を SVG（ピクセル）とホバー JSON（ピクセル＋値）で**二重に送出**しており、30日分（5分間隔で約8,640点）のフラグメントは概算で約1.0MB（うちホバー JSON が約778KB）。期間切替のたびに再生成・再パースしている。
- go-echarts は生値のみを option JSON で送り（ピクセル座標はクライアントで計算）、ペイロードは概算で約1/3〜1/4。ホバーは ECharts 内部の二分探索＋スロットルで O(log N) となり、現状の O(N) 線形走査（30dで毎 mousemove 8,640要素走査）より軽い。
- 代償は echarts.min.js（約1MB / gzip 約330KB）の初回読込のみ。self-host＋キャッシュで一過性コストに抑える。

## このセッションのスコープ（実装対象）

### 依存追加とアセット self-host

- go.mod に `github.com/go-echarts/go-echarts/v2` を追加（プロジェクトローカル完結。feedback_project_local_setup 準拠）。
- echarts.min.js を **self-host**（CDN 直依存にしない）。配置は go:embed 配線に合わせ internal/view/public 配下（例: internal/view/public/js/echarts.min.js。go:embed は `..`/symlink 不可のため public は internal/view/ 配下＝既存 static.go の方針と同じ）。
- App.templ の <head> で echarts.min.js を **1回だけ** 読み込む（フラグメント側には出さない＝複数チャート・期間切替で再 DL させない）。バージョン付き静的配信（CSSURL() と同様のキャッシュバスティング方針）を踏襲。
- go-echarts のアセットホストを self-host 先へ向ける（AssetsHost 指定または JSAssets 書き換え。正確な API は設計フェーズで確定）。

### サーバー側のグラフ構築（go-echarts）

- buildChartArea を、SVG 文字列生成から go-echarts のチャート構築へ置換する。
  - データ取得 ListRecentSensorReadings と pgtype→float 変換（rawSeries 相当）は**再利用**（データ源は不変）。
  - 温度・湿度それぞれ Line チャートを構築し、**RenderSnippet()** でフラグメント（ChartSnippet の Element / Script / Option）を取り出して templ へ渡す。
- 依存方向の扱い: internal/chart は現状「gin・DB・templ・pgtype を import しない最下流ユーティリティ」。go-echarts 依存（render/opts）をどの層に置くか（chart パッケージ内に echarts 用サブユニットを新設するか、service/handler 側に置くか）は structure.md の依存方向ルールに従い設計フェーズで決定。
- 機能の ECharts 標準機能への対応付け（現状の自作実装を置換）:
  - 最高/最低見出し（現 writeYAxisLabels）→ **MarkPoints**（Type "max"/"min"）または y 軸ラベル＋markLine。
  - 右端の現在値（現 writeCurrentValue）→ line series の endLabel 相当、または最終点への MarkPoint。
  - 十字ポインター（現 Alpine 縦線/横線）→ **Tooltip（trigger: axis）＋ AxisPointer（type: "cross"）**。
  - 温湿度2グラフ連動（現 linkedCharts の共有 index）→ **echarts.connect() / AxisPointerLink** で2チャートのツールチップ・軸ポインターを連動。
  - 空状態（現 emptyMessage「データはまだありません」）→ ECharts の空表示（graphic/title もしくはサーバー側で点0時に従来同等のメッセージ表示）。
  - 線色は現状を踏襲（温度 #e8590c / 湿度 #1971c2）。

### テンプレート・ViewModel の改修

- DeviceChartArea.templ: @templ.Raw(SVG) ＋ x-data="linkedCharts(...)" を、ECharts コンテナ div（#temperature-chart / #humidity-chart 相当）＋ RenderSnippet の初期化 script へ置換。期間ボタン群・カード見出し・レイアウトの**器は維持**（モック準拠）。
- views.go の DeviceChartAreaView: TemperatureSVG / HumiditySVG / TemperatureHoverJSON / HumidityHoverJSON を廃し、ECharts スニペット（Element＋Script、もしくは template.HTML 化したスニペット）を保持する型へ変更（イミュータブル方針＝Handler 側で組み立てて View に詰める）。

### HTMX フラグメント差し替え時の初期化・破棄

- 期間切替（hx-swap innerHTML）でフラグメントが入れ替わる際、**旧 ECharts インスタンスを dispose() してから新規 init** する（メモリリーク防止）。htmx:beforeSwap / htmx:afterSwap もしくは Alpine x-init/x-destroy 等での初期化・破棄パターンを確立する。
- RenderSnippet の初期化 script が swap 後に確実に実行されること（HTMX 2.x の script 評価 ＋ 対象 div が DOM 存在後に init されること）を保証する。

### 旧コードの撤去

- 移行完了後、internal/chart/svg.go の SVG 生成（LineChartSVG ほか）・internal/chart/hover.go・App.templ の linkedCharts() インライン script・device_show.go の hoverJSON / SVG 生成呼び出しを撤去する。chart.Point / chart.Series などデータ表現は再利用可能なら残す。
- 関連テスト（svg_test.go / hover_test.go）は新構築（option 構築）のテストへ置換・調整する。

### データ量対策（任意・設計判断）

- 7d/30d の点数（最大約8,640点）に対し、サーバー側ダウンサンプリング（画面幅相当の数百点へ間引き）や ECharts の dataZoom・sampling("lttb") の採用可否を設計フェーズで決定する。負荷低減に最も効くため本セッションで併せて検討するが、見た目を損なわない範囲とする。

## スコープ外（このセッションでやらないこと）

- グラフ以外の device-show 構成要素（情報パネル・最新計測テーブル・削除モーダル）の変更。
- センサーデータ履歴画面（S6）のグラフ化・他画面へのグラフ追加（本セッションは device-show のみ）。
- データ取得クエリ（ListRecentSensorReadings）やスキーマの変更。
- アラート・通知・認証など他機能。
- ダッシュボードへのグラフ展開・自動更新ポーリング。
- 複数系列の新規追加（飽差 VPD 等の派生指標。将来の分析ロードマップ＝分析アイデアメモ.md は別スペック）。

## 技術制約・準拠事項

- **go-echarts v2.7.2**（内部 ECharts 5.4.3）。RenderSnippet による**フラグメント出力**を用い、フルHTMLページ Render は使わない。
- **echarts.min.js は self-host**（go:embed 配信。internal/view/public 配下）。<head> で1回読込。CDN 直依存にしない（feedback_project_local_setup）。
- **依存方向**（structure.md）: 下向き一方向。go-echarts（描画ライブラリ＝外部）への依存を internal/chart 最下流ユーティリティの不変条件と整合させる配置を設計で決める。View は repository/service を import しない方針を維持。
- **HTMX**: 期間切替は現状の hx-get / hx-target / hx-swap(innerHTML) / hx-push-url を維持（§10-D フルフラグメント swap）。swap 時の ECharts init/dispose を追加。
- **templ** v0.3: コンテナ div ＋ 初期化 script の埋め込み。SVG の @templ.Raw 埋め込みは廃止。
- **CSS**: グラフ描画本体（canvas/SVG の中身）は **モック単一ソース運用の例外**（feedback_mock_graph_rendering_exception）。期間ボタン・カード見出し・枠・配色など静的な器は mocks/html/style.css 正本に準拠（project_css_single_source）。
- **イミュータブル方針**: sqlc 生成構造体は読取専用。Handler で View を組み立てる。
- **言語**: 日本語コメント・エラーメッセージ。コード識別子は英語。
- **TDD**: 80% 以上。グラフ option 構築（系列・markPoint・最高/最低算出）のユニットテスト、Handler Show / Chart の回帰テスト（期間バリデーション・フラグメント返却・空データ）を整備。
- セキュリティ: 既存同様、CSRF（meta + htmx:configRequest）を維持。ECharts 初期化はインライン script だが現状（linkedCharts）と同等の取り扱い。

## 受け入れ基準（概略）

1. **機能維持（無回帰）**: 初期表示（GET /devices/{device}、デフォルト24h）と期間切替（24h/3d/7d/30d、HTMX フラグメント差し替え・アクティブボタン往復・URL 同期）が現状同等に動作する。
2. **グラフ機能の再現**: 温度・湿度の2グラフ、最高/最低、右端の現在値、十字ホバー（axisPointer cross）、温湿度2グラフの連動が ECharts 標準機能で再現される。空データ時は従来同等のメッセージを表示する。
3. **self-host・単回読込**: echarts.min.js は self-host され <head> で1回のみ読み込まれ、期間切替で再 DL されない（ネットワークログで確認）。
4. **init/dispose 健全性**: 期間切替を繰り返してもグラフが正しく再描画され、旧 ECharts インスタンスが dispose され、インスタンス/リスナーのリークが無い。
5. **負荷改善**: 30日相当のデータ（約8,640点）で、フラグメントのペイロードが現状（SVG＋ホバー JSON 二重送出）より縮小し、ホバー操作が線形走査によるジャンク無く滑らかに動作する（必要に応じて間引き/dataZoom を併用）。
6. **モック整合**: グラフ描画は反映対象外として扱い、器（期間ボタン・見出し・カード・配色）はモック準拠を維持する。
7. **テストカバレッジ 80% 以上**: option 構築ユニットテストと Handler 回帰テストを完備。旧 SVG/ホバー生成コードとそのテストは撤去・置換される。

## 未確定事項・要確認（設計フェーズで決定）

- **採用方式**: 方式A（go-echarts で RenderSnippet＝サーバーで Element＋init script を生成）を本命とするが、方式B（go-echarts は option JSON 生成のみ、クライアントで echarts.init/setOption）も HTMX/Alpine 親和性の観点で比較検討する。
- **レンダラ**: canvas（既定・多点向き）か SVG レンダラか。点数とモバイル描画負荷で判断。
- **アセットホスト指定 API**: go-echarts の self-host 切替の正確な手段（opts.Initialization の AssetsHost か JSAssets の書き換えか）を実コードで確認。
- **opts での表現可否**: endLabel（右端現在値）・sampling("lttb") が go-echarts の opts 構造体で出せるか、生 option 注入が必要かを確認。
- **ダウンサンプリング方針**: 7d/30d をサーバーで間引くか、ECharts dataZoom/sampling に委ねるか、その閾値。
- **2グラフ構成 vs 統合**: 現状の温度・湿度の縦並び2チャート（connect 連動）を維持するか、1チャート2y軸へ統合するか（現状維持を既定とし、統合は別検討）。
- **no-JS フォールバック**: 現状の SVG は JS 無しでも表示されたが ECharts は JS 必須。社内/関係者向けダッシュボードとして許容するかを確認（既定: 許容）。

--- spec-init 本文 ここまで ---
