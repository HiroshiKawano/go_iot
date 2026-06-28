# Implementation Plan

> 逐次実装（上から1行ずつ `/tdd` で RED→GREEN→REFACTOR）。スキーマ非変更（goose 00009 のまま・migration / `make db-snapshot` / 新規 SQL なし）。品質メタは既存 `ListSensorReadingsInRange` の全行走査で読み取り時計算。
> テスト定石は `2cc_sdd/テストガイダンス集.md`（純関数=表駆動 / handler=Querier モックで DB 非依存 + `httptest` / templ=`Render`→`strings.Contains`）。

- [x] 1. 品質判定の純粋関数層（internal/chart・math のみ・time 非依存）

- [x] 1.1 欠測率・欠測ギャップ区間・サンプリング間隔一貫性の純関数
  - 間隔秒列（`[]float64`）から期待間隔＝中央値を求め、欠測本数 Σmax(0,round(間隔/中央値)−1)・欠測率(%)・連続欠測区間（開始/終了インデックス・欠測スロット数）を返す
  - 間隔秒列の変動係数（σ/μ）を既存 `CV` 流用で返し、一貫性指標とする
  - 境界: 要素2未満は ok=false（率/CV 未定義）、等間隔列は欠測率0・CV0、先頭/末尾/全欠測を破綻なく扱う
  - 観測可能完了: 表駆動テストで「等間隔=率0」「抜け1区間でギャップ1件＋欠測本数一致」「len<2 で ok=false」が緑
  - _Requirements: 2.1, 3.1, 3.2, 6.1, 6.2_
  - _Boundary: internal/chart quality.go_

- [x] 1.2 外れ値判定の純関数（ローリングσ法を主・Zスコア/IQR を補助）
  - 移動窓 μ±kσ（既存 `SMA`/`MovingStdDev`/`Band` 流用）で各点の外れ値真偽を返す。warm-up（index<window−1）と σ≤ε は「外れ値なし」
  - 補助として Zスコア列・IQR 境界（四分位）の純関数も用意（主経路外・将来調整用）
  - 境界: 散らばり0（定常列）でゼロ除算せず全 false、空/単一点を安全に扱う
  - 観測可能完了: 表駆動テストで「昼夜変動を模した列で誤検出ゼロ」「σ≈0 で全 false」「|z|>k 境界」が緑
  - _Requirements: 2.4, 2.5, 6.4_
  - _Boundary: internal/chart quality.go_

- [x] 1.3 stuck/flatline・物理範囲・急変の純関数（しきい値は定数化）
  - 同値が minRun 回以上連続する区間（固着の疑い）を真偽列で返す
  - 農学的下限上限（CHECK の内側・既定 温度[−10,60]℃）外を真とする物理範囲判定と、隣接差 |Δ| が上限超（既定 温度10℃・湿度40%RH）を真とする急変判定
  - しきい値はコメント根拠付き定数（research/沖縄実環境で調整可・湿度の長時間高止まりは正常ゆえ固着のみで検出）
  - 観測可能完了: 表駆動テストで「全同値で stuck 検出・N 境界」「範囲境界値」「急変境界」「空入力で空結果」が緑
  - _Requirements: 2.2, 2.3, 2.6, 6.3_
  - _Boundary: internal/chart quality.go_

- [x] 2. 品質フラグ/品質レベルのドメイン列挙（internal/domain・純粋 fmt のみ・DB 非永続）

- [x] 2.1 QualityFlag・QualityLevel 列挙と表示メソッド
  - `QualityFlag`（欠測直後/固着/物理異常/外れ値）と総合 `QualityLevel`（信頼/注意/不良）を既存 `Metric`/`Crop` の書式（type string + 定数 + `Label()`/`Valid()`/`Parse`/`All`）で定義
  - `QualityLevel` に信号色 CSS クラス名を返すメソッドを持たせ、`@layer components` の variant と1:1対応させる
  - DB に持たないため §100 の VARCHAR+CHECK は対象外（純 Go 列挙）
  - 観測可能完了: 表駆動テストで全列挙の `Label()`・`Valid()`・未知値 false・CSS クラス対応が緑
  - _Requirements: 1.1, 1.2, 4.1, 4.2_
  - _Boundary: internal/domain quality_flag.go_

- [x] 3. 欠測ギャップ可視化のグラフ層拡張（internal/chart・後方互換厳守）

- [x] 3.1 ChartSpec の欠測スロット対応と線分断
  - `ChartSpec` に欠測スロット nil を持つ series データと連続欠測区間（xAxis インデックス範囲）を末尾非破壊追加し、`echarts.go` の series[0] 構築で nil→ECharts null へ写す
  - `connectNulls:false`（既定）を明示し、欠測スロットで折れ線を繋がない
  - 後方互換: 新フィールド未設定時は既存出力と完全一致（既存 echarts 4 テストが無改変で緑）を不変条件にする
  - 観測可能完了: テストで「新フィールド設定時に series データへ null が出る」「未設定時に既存テスト不変」が緑
  - _Requirements: 5.1, 5.4, 9.1_
  - _Boundary: internal/chart series.go, echarts.go_

- [x] 3.2 連続欠測区間の markArea 自前注入（xAxis 範囲）
  - 既存 VPD markArea 注入（option を JSON 化→マップ→series[0] へ小文字キー注入→再 Marshal で HTML 安全化）を踏襲し、キーを xAxis 範囲指定にした欠測ハイライト注入を起こす
  - 欠測区間が空のときは原文をそのまま返す。薄い灰系の控えめな帯にする
  - 観測可能完了: テストで「小文字 xAxis キーの markArea が注入される」「空区間で原文不変」「< > & が \uXXXX 化」が緑
  - _Requirements: 5.2, 5.3_
  - _Depends: 3.1_
  - _Boundary: internal/chart gap_echarts.go, echarts.go_

- [x] 4. readings への品質メタ組み立て（handler・time 境界処理）

- [x] 4.1 期間品質メトリクスと総合バッジ合成
  - 期間内全行（`ListSensorReadingsInRange`）から温湿度値列と間隔秒列（recorded_at 差分＝handler 境界で生成）を作り、欠測率・間隔一貫性・通信遅延代表値（平均/最大・`formatDelay` の素を流用）を算出
  - 総合品質レベルを合成（赤=欠測率>30% or 固着検出 or 物理異常>0／黄=欠測率>5% or 外れ値率>5% or 間隔CV>0.5／緑=その他・境界は定数）
  - 境界: 0件/単一点は ok=false → 既存 `statEmptyMark`（"—"）作法で表示
  - 観測可能完了: Querier モックで DB 非依存テスト「欠測率/CV/遅延の算出」「緑/黄/赤の各境界でレベル切替」「空期間で—」が緑
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 4.1, 4.2, 4.3, 6.1, 6.2_
  - _Depends: 1.1, 1.2, 1.3, 2.1_
  - _Boundary: internal/handler readings_quality.go_

- [x] 4.2 レコード単位の品質フラグ算出（異常行のみ非空）
  - 各行について温度・湿度双方の判定（欠測直後/固着/物理異常/外れ値）を OR し、その行に該当する品質フラグ集合を返す。正常行は空（バッジを付けない）
  - 観測可能完了: テストで「外れ値行のみフラグ非空」「正常行は空」「複数該当行で複数フラグ」が緑
  - _Requirements: 1.1, 1.2, 1.3_
  - _Depends: 1.2, 1.3, 2.1_
  - _Boundary: internal/handler readings_quality.go_

- [x] 4.3 readings ハンドラへの配線とViewModel拡張（統合）
  - `ReadingHistoryRow` に品質フラグ集合、`DeviceReadingsListView` に品質メトリクスビュー（欠測率/間隔CV/通信遅延代表値/総合レベル/HasData）を末尾追加し、`fetchResults`/`buildReadingHistoryRows` から 4.1・4.2 を呼んで載せる
  - 期間メトリクスは parseDateBounds の BETWEEN 区間（一覧/集計/CSV と同一）で算出し整合させる。所有者認可（既存 `RequireDeviceOwner`・非所有→404）を継承
  - 形式不正フィルタ（既存経路）では品質メタを算出せず空状態にする
  - 観測可能完了: ハンドラテストで `#device-readings-list` 相当の ViewModel に品質メトリクス/バッジ/行フラグが載ること、非所有で 404 が緑
  - _Requirements: 1.4, 3.4, 8.1, 8.2, 8.3_
  - _Depends: 4.1, 4.2_
  - _Boundary: internal/handler readings.go, internal/view/component views.go_

- [x] 5. device-show への欠測ギャップ配線（handler）

- [x] 5.1 グラフ欠測検出と拡張グリッド組み立て
  - `buildChartArea` で間隔中央値から欠測区間を検出し、欠測スロットを nil で埋めた拡張グリッド（Labels と各系列を同一インデックス空間に）を組んで ChartSpec の欠測対応フィールドへ設定（device-show は開区間窓・視覚表現のみで率の数値は出さない）
  - 欠測ギャップ凡例表示用のフラグを ViewModel へ渡す。所有者認可（既存）を継承。欠測なし時は従来描画（無回帰）
  - 観測可能完了: ハンドラ/option テストで「欠測ありデータで null と markArea が出力に載る」「欠測なしで従来出力不変」が緑
  - _Requirements: 5.1, 5.2, 5.4, 7.1, 9.1_
  - _Depends: 3.1, 3.2_
  - _Boundary: internal/handler device_show.go_

- [x] 6. View / templ / モック反映

- [x] 6.1 モック正本とCSSに品質の器を定義
  - `mocks/html/readings.html` に品質フラグ列・品質メトリクスボックス・総合バッジの見本、`mocks/html/device_show.html` に欠測ギャップ凡例/注記を反映
  - `mocks/html/style.css`（正本）の `@layer components` に `.badge` と信号色 variant（既存 `--color-primary/warning/danger` トークン流用）を追加し `make sync-css` で本番 CSS を再生成
  - グラフ内部の線分断/markArea 描画はモック反映の対象外（器のみ反映）
  - 観測可能完了: モックをブラウザ表示で品質バッジ/フラグ列/メトリクスボックス/凡例が見え、`make sync-css` 後に本番 CSS と一致
  - _Requirements: 4.1, 5.2, 10.1, 10.2_
  - _Boundary: mocks/html, style.css_

- [x] 6.2 履歴一覧templへの品質フラグ列・メトリクスボックス・バッジ描画
  - `DeviceReadingsList.templ` の `.data-table` に品質フラグ列（異常行のみ domain `Label()` でバッジ表示・正常行は空）、`.summary-grid` に品質メトリクスボックスと総合品質バッジ（domain のCSSクラス）をモック写経で追加
  - 既存クラス流用・独自クラス新設はバッジ信号色のみ。フィルタ/ページ送りで `#device-readings-list` ごと再描画される既存挙動に相乗り
  - 観測可能完了: templ `Render`→`strings.Contains` テストで品質メトリクス文言・バッジクラス・異常行フラグが出力に含まれることが緑
  - _Requirements: 1.1, 1.4, 4.1, 4.4, 10.1_
  - _Depends: 2.1, 4.3, 6.1_
  - _Boundary: internal/view/component DeviceReadingsList.templ_

- [x] 6.3 デバイス詳細templへの欠測ギャップ凡例/注記描画
  - `DeviceChartArea.templ` に欠測ギャップの凡例/注記（静的な器）を追加し、期間切替で `#device-chart-area` ごと再描画される既存挙動に相乗り
  - 観測可能完了: templ `Render`→`strings.Contains` テストで欠測ギャップ凡例の文言が出力に含まれることが緑
  - _Requirements: 5.2, 10.1_
  - _Depends: 5.1, 6.1_
  - _Boundary: internal/view/component DeviceChartArea.templ_

- [x] 7. 統合・無回帰・境界検証

- [x] 7.1 readings 画面の統合テスト（httptest）
  - 期間フィルタ結果のフラグメント HTML に品質メトリクスボックス・総合バッジ・行フラグ列が含まれること、非所有 device で 404、`from>to` で品質メタも空（既存0件経路と区別）を固定
  - 観測可能完了: `httptest` 統合テストが上記3経路で緑
  - _Requirements: 1.4, 3.4, 4.1, 8.2_
  - _Depends: 6.2_
  - _Boundary: internal/handler readings.go_

- [x] 7.2 device-show グラフの統合・無回帰テスト
  - 欠測を含むデータで `connectNulls:false` による線分断（null 出力）と欠測区間 markArea が載ること、欠測なしデータで従来のグラフ出力が不変（P2 オーバーレイ・markPoint・期間切替・温湿度2グラフ連動が無回帰）を固定
  - 観測可能完了: 統合/option テストが「欠測あり=ギャップ描画」「欠測なし=既存不変」で緑
  - _Requirements: 5.1, 5.2, 5.3, 9.1_
  - _Depends: 6.3_
  - _Boundary: internal/handler device_show.go, internal/chart echarts.go_

- [x] 7.3 全体無回帰と境界ガードの確認
  - 既存テスト一式（chart/stats・readings・readings_report・P2 オーバーレイ・S6 フィルタ/一覧/通信遅延/ページネーション・P4 CSV/帳票）が無改変で緑であることを確認し、スキーマ非変更（goose 00009・`make db-snapshot` 差分なし）を確認
  - 器差/位置軸の新規構造を作っていないこと（単一 device に確定・器差は P10 へ defer・限定版を作らない）を境界ガードとして確認
  - 観測可能完了: 全テストスイートが緑、`git diff` に migration / 新規 SQL / 器差・位置軸コードが無い
  - _Requirements: 7.1, 7.2, 7.3, 9.1, 9.2, 9.3, 9.4_
  - _Depends: 7.1, 7.2_
  - _Boundary: 全体（無回帰・境界検証）_
