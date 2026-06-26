# 実装計画: device-chart-echarts

> デバイス詳細画面（device-show）のグラフ描画を自作 SVG ＋ Alpine ホバーから Apache ECharts（go-echarts による option 構築）へ移行する。逐次実装（番号順＝実装順・各タスクは `/tdd` 1サイクル）。DB スキーマ変更なし（データ源 `ListRecentSensorReadings` は不変）。

- [x] 1. Foundation: ECharts アセットの self-host 配信
  - go-echarts v2.7.2 を依存追加する（`go get github.com/go-echarts/go-echarts/v2@v2.7.2`、go.mod/go.sum 更新）
  - echarts.min.js（ECharts 5.4.3・go-echarts-assets 由来）を `internal/view/public/js/` 配下に配置する（`//go:embed all:public` は既存・再配線不要）
  - `JSURL()` を `CSSURL()` と同方式で追加する（`/static/js/echarts.min.js?v=<Version>`・package `view`）
  - 観測: `GET /static/js/echarts.min.js?v=<Version>` が 200 で配信され、`JSURL()` がバージョンクエリ付き URL を返す（static_test.go が green）
  - _Requirements: 5.1, 5.2, 5.3_

- [x] 2. サーバー側 ECharts option ビルダ
  - 折れ線データ表現（`Point`/`Series`）を専用ファイルへ移設し、`svg.go` 内の同型定義は削除する（同一 package・コンパイル緑を維持。`svg.go`/`hover.go` 本体は task 6 で撤去）
  - 単一系列の折れ線について ECharts option を HTML 安全 JSON 文字列で返すビルダを実装する（go-echarts `charts.Line` で構築）
  - option に xAxis カテゴリ（X軸ラベル列）・1 series（実測値）・markPoint（最高/最低）・tooltip(trigger axis)+axisPointer(type cross)・lineStyle.color を含める。endLabel と sampling は含めない（クライアントで付与）。返却は `encoding/json` で HTML 安全化する（§10-E）
  - table-driven テスト: series/xAxis、markPoint max-min、tooltip+cross、線色 温度#e8590c/湿度#1971c2、HTML安全エスケープ（`< > &` を含むラベルで生タグ/`</script>` が漏れない）
  - 観測: ビルダが上記キーを含む JSON を返し、chart パッケージの option テストが green（`Point`/`Series` 移設後も `go build` 緑）
  - _Requirements: 2.1, 2.2, 2.4, 2.5, 3.1, 3.2, 7.2_

- [x] 3. サーバー描画を ECharts option へ切替（統合: ViewModel + フラグメント templ + handler）
  - `DeviceChartAreaView` を SVG/HoverJSON 4 フィールドから option JSON 2本 + unit/color + `HasData` へ変更し、`hoverData` ヘルパを撤去する（`chartPeriods`・器は維持）
  - `DeviceChartArea.templ` を `@templ.Raw(SVG)`+`x-data` から、コンテナ div（`#temperature-chart`/`#humidity-chart` に `data-unit`/`data-color`/`data-echarts`）+ `<script type="application/json">` option へ書換える。`HasData=false` 時は「データはまだありません」ブロックのみ。期間セレクタ/カード/`<h2>` の器は維持する（独自クラス新設しない・グラフ本体は反映例外）
  - `buildChartArea` を option 構築呼び出しへ改修する（`rawSeries` 再利用、SVG/`hoverJSON` 呼び出し削除、0件で `HasData=false`）。`Show`/`Chart` の ID パース・認可・period 検証・最新計測・情報パネルは不変
  - `device_show_test` の SVG/role=img アサーションを option-script/コンテナ id アサーションへ置換する。period 検証400・認可404 列挙防止・active 1個・空データメッセージは維持
  - `.templ` 改修後に `templ generate` を実行し、生成物込みで `go build` が緑になることを確認する
  - 観測: httptest で Show 200 が `#device-chart-area`・`#temperature-chart`/`#humidity-chart` コンテナ（DOM 内で id 一意）・温/湿 option script を含み `period-btn active` が1個、Chart フラグメントが option script 2本のみでレイアウト非包含かつ情報パネル/最新計測テーブルを含まない（期間非連動）、空データで「データはまだありません」かつ option script 非出力、period 不正400・認可404 が維持
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 4.1, 4.2, 9.1, 9.2_

- [x] 4. 器: グラフコンテナの明示高さ（CSS 単一ソース）
  - 正本 `mocks/html/style.css` にグラフコンテナの明示高さを付与する（ECharts は明示高さ必須・0高さ回避）。器（`.chart-wrapper`/`.chart-placeholder` 高さ）と整合させ、独自クラスを新設しない（§31.1/§40-B）
  - `make sync-css` で本番 `internal/view/public/css/style.css` へ反映する（生成物・手編集禁止）
  - 観測: `#temperature-chart`/`#humidity-chart` が明示高さを持ち、ECharts 描画領域が 0 高さにならない（生成 CSS にコンテナ高さ規則が存在）
  - _Requirements: 9.1_

- [x] 5. クライアント ECharts 初期化（App.templ グローバル）
  - `App.templ` `<head>` に echarts.min.js を `JSURL()` で1回読込する（`view` を直接 import、`AppLayoutData` 非変更。Guest.templ=login/register は非対象）
  - `linkedCharts()` を撤去し、グローバル初期化スクリプトを追加する: `DOMContentLoaded` と `htmx:afterSwap` で `[data-echarts]` コンテナを `dispose()`→`init()`→option(`JSON.parse`)→endLabel(`data-unit`)/sampling("lttb") 付与→`setOption`、温/湿2インスタンスを `echarts.connect`、`htmx:beforeSwap` で `dispose()`。option script 不在のコンテナは skip
  - `.templ` 改修後に `templ generate` を実行し、生成物込みで `go build` が緑になることを確認する
  - 観測（手動/視覚）: device-show で温湿度グラフが ECharts 描画・十字ホバーで現在値(endLabel)表示・温湿度連動、期間切替の繰返しでインスタンス/リスナーが蓄積せず再描画、echarts.min.js は1回読込・期間切替で再DLされない、JS無効環境ではコンテナのみで折れ線は描画されない（許容仕様）、他認証画面は `[data-echarts]` 不在で no-op
  - _Requirements: 2.3, 3.3, 5.2, 6.1, 6.2, 7.1, 8.1, 8.2_

- [x] 6. 旧 SVG/ホバー生成コードの撤去
  - chart の `svg.go`（`LineChartSVG` ほか SVG 生成一式）・`hover.go`（`LineChartHoverPoints`/`HoverPoint`/`xAtIndex`）と `svg_test.go`・`hover_test.go` を撤去する（`Point`/`Series` 型は task 2 で移設済みのため残置）
  - `device_show.go` に残る未使用ヘルパ（`hoverJSON` 等）を撤去する（`aggregateToFloat` は `readings.go` が流用中のため残す）
  - 観測: `go build`・`go vet` 通過、chart パッケージに SVG/ホバー残骸が無く、`go test ./...` が green
  - _Requirements: 1.1, 7.2_

- [x] 7. 回帰検証
  - `go test ./...`（chart option ビルダ + device_show handler）green を確認する
  - 他認証画面（dashboard/readings/alert-*）が echarts.min.js 読込のみで JS エラー無く無回帰であることを確認する（`[data-echarts]` 不在で no-op）
  - 手動/視覚: 期間切替繰返しのリーク無し(6)、連動ホバー(3.3)、endLabel(2.3)、30日相当の滑らかさ(7.1)、echarts.min.js 1回読込・再DL無し(5)
  - 観測: 全自動テスト green、device-show の受け入れ基準（無回帰・連動・self-host・init/dispose・負荷）が手動チェックリストで確認済み
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 5.3, 6.1, 6.2, 7.1_
