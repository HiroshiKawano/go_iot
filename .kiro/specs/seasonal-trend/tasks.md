# 実装計画（seasonal-trend / 長期トレンド・季節サマリ）

> 逐次実装（上から1行ずつ `/tdd` で RED→GREEN→REFACTOR）。`(P)` 並列マーカーは付けない。
> 生成順序: sqlc/templ などの生成物は消費側より前に置く。CSS は Foundation。DDL なし（goose 最新 00010 のまま）。

- [x] 1. Foundation: モック器・CSS・データ取得の整備
- [x] 1.1 統計分析ページのモック器と新色トークン・判定バッジ CSS を正本に追加
  - `mocks/html/analysis-trend.html` を新規作成し、ページ器（集計軸セレクタ枠・サマリ表・MK/Sen 判定バッジ枠・検出力留保注記領域・トレンド/日較差チャート枠）を静的 HTML で配置する
  - 正本 `mocks/html/style.css` の `@layer base` に `--color-trend` 等の新色トークン、`@layer components` に `.badge` 信号色 variant（有意↑=暖色 / 有意↓=寒色 / 非有意=グレー）を既存トークン流用で追記する（§31.2 手順）
  - `make sync-css` を実行し本番配信 `internal/view/public/css/style.css`（生成物）へ反映する。既存の go:embed / StaticFS / `CSSURL()` 配信は流用（新規配線なし）
  - 観測可能完了: ブラウザで `analysis-trend.html` を直接開くと器・バッジ枠・留保注記が表示され、`make sync-css` 後に本番 CSS にも新トークン/バッジが含まれる
  - _Requirements: 1.5, 6.4_
- [x] 1.2 JST 暦日基準の日次集計クエリを追加し sqlc を再生成する
  - `db/queries/sensor_readings.sql` に `ListDailySensorAggregatesJST`（`DATE(recorded_at AT TIME ZONE 'Asia/Tokyo')` で GROUP BY・avg/max/min/count・SELECT のみ）を追加する。既存 `ListDailySensorAggregates` は無改変
  - `make sqlc` を実行し `repository.Querier` に新メソッドを生成する（DDL なし・`make db-snapshot` 不要）
  - 観測可能完了: `repository.Querier` に JST 日次集計メソッドが生成され、Querier モックから device_id+下限で JST 暦日昇順の日次集計が取得できる
  - _Requirements: 2.1, 2.2_

- [x] 2. トレンド統計の純粋層（trend.go・外部参照実装 golden TDD）
- [x] 2.1 Mann-Kendall 検定と正規 CDF による両側 p 値
  - S=Σsign(x_l−x_k)、タイ補正済み Var(S)=[N(N−1)(2N+5)−Σ t_j(t_j−1)(2t_j+5)]/18、連続性補正済み Z を算出する
  - 標準正規 CDF を `math.Erfc` で自前実装（gonum 非導入）し両側 p 値を返す
  - 観測可能完了: pyMannKendall original_test の既知データセットに対し S・Z・p が数値一致する golden テストが緑（タイあり/なし両方）
  - _Requirements: 3.1, 3.4_
  - _Boundary: chart.trend_
- [x] 2.2 Sen の傾き（全ペア中央値・外れ値頑健）
  - 全ペア (x_l−x_k)/(l−k) の中央値を `quality.go` の median/quantile を流用して算出する
  - 台風スパイク等の外れ値に頑健であることを検証する
  - 観測可能完了: 既知データで Sen 傾きが pyMannKendall の期待値と一致し、外れ値混入時も中央値が安定する golden テストが緑
  - _Requirements: 3.1, 3.4_
  - _Boundary: chart.trend_
- [x] 2.3 ラグ1自己相関 λ̂・有効標本サイズ N_eff・Seasonal Mann-Kendall
  - ラグ1自己相関 r1、N_eff≈N(1−r1)/(1+r1)（下限1）を算出する
  - Seasonal MK（Hirsch-Slack）を季節キー（月）ごとに S/Var を合算して算出する
  - 観測可能完了: λ=0.9 相当データで N_eff が大幅縮小し、月別符号反転データで年集約 MK は非有意でも Seasonal MK が検出する golden テストが緑
  - _Requirements: 3.2, 4.1_
  - _Boundary: chart.trend_
- [x] 2.4 Hamed-Rao 補正 Mann-Kendall
  - ランクの有意自己相関で Var(S) を補正する（pyMannKendall hamed_rao_modification_test 準拠）
  - 観測可能完了: pyMannKendall の hamed_rao 既知データに対し補正後 Var(S)・Z・p が数値一致する golden テストが緑
  - _Requirements: 5.6_
  - _Boundary: chart.trend_
- [x] 2.5 ブロックブートストラップ信頼区間と多重比較補正
  - 移動ブロックブートストラップ（ブロック長≈round(n^(1/3))・B 反復・seed 固定）で Sen 傾きの経験 CI を算出する
  - Benjamini-Hochberg（FDR）と Bonferroni で p 配列の reject を算出する
  - 観測可能完了: 同一 seed で CI が決定的に再現し、既知 p 配列に対し BH/Bonferroni の reject 判定が一致するテストが緑
  - _Requirements: 5.1, 5.5_
  - _Boundary: chart.trend_

- [x] 3. 描画層（trend_echarts・別型 TrendChartSpec）
- [x] 3.1 トレンド option JSON 生成（Sen 線・有意区間・CI 帯・日較差推移）
  - `series.go` に別型 `TrendChartSpec` を追加（既存 ChartSpec 群は不変）
  - ロールアップ折れ線を主役に、Sen トレンド線=markLine・有意区間=markArea・ブートストラップ CI 帯=積み上げ area・平年比系列を ECharts 準拠の小文字キーで自前注入し、長期閲覧用 dataZoom を含める
  - 日較差ΔT 推移の line option も生成する
  - 観測可能完了: `TrendChartOptionJSON` が markLine/markArea/CI area/dataZoom を含む有効な option 文字列を返し、既存 chart テストが無回帰で緑
  - _Requirements: 3.1, 5.3, 6.4, 7.1_
  - _Boundary: chart.trend_echarts, series.go_

- [x] 4. ロールアップ集約・平年比（handler 境界の集約ロジック）
- [x] 4.1 JST 月次/年次ロールアップ集約
  - 日次集計(JST)を月/年バケットに集約し、平均/最高/最低/日較差ΔT（=日次ΔTの平均）/σ/CV/サンプル数を算出する。空バケットは欠測としてスキップ（0 補完しない）
  - O(N²) 検定はこのロールアップ後系列にのみ適用する前提を満たす（生粒度を検定に渡さない）
  - 観測可能完了: Querier モックの日次集計から、月初/月末/空月を含む期間で月次サマリ行（ΔT=日次ΔT平均）が正しく生成されるテストが緑
  - _Requirements: 2.1, 2.3, 2.4, 3.3_
  - _Depends: 1.2_
  - _Boundary: SeasonalTrendHandler_
- [x] 4.2 平年比（暦月平均・年数不足時非表示）
  - 自社データの暦月平均を平年値として算出し、利用可能年が3年以上のときのみ平年比を提示する。未満は非表示とし不確実性注記を付す
  - 観測可能完了: 年≥3 で平年比系列が生成され、年<3 では平年比非表示＋「基準期間依存・不確実」注記が付くテストが緑
  - _Requirements: 7.1, 7.2, 7.3_
  - _Boundary: SeasonalTrendHandler_

- [x] 5. View 層（templ・DTO・ナビ）
- [x] 5.1 ページ・部分更新・判定バッジの templ と DTO
  - イミュータブル DTO（ページ/セクション/バッジ/サマリ行/平年比）を定義し、`SeasonalTrend`（フルページ・App レイアウト）/`TrendSection`（#trend-section 部分）/`TrendBadge`（信号バッジ＋一次/補正済みラベル＋検出力留保注記）を実装する。モック `analysis-trend.html` を写経し独自クラスを新設しない
  - サマリ表・バッジ・留保注記・チャートコンテナ（`data-echarts`＋兄弟 option script）を描画する。`make templ` で生成
  - 観測可能完了: `Render`→`strings.Contains` で各 DTO 値（サマリ行・バッジラベル・留保注記・`data-echarts` コンテナ）が HTML に出力されることをアサートするテストが緑
  - _Requirements: 1.5, 4.2, 5.3, 6.1, 6.3_
  - _Depends: 1.1_
  - _Boundary: SeasonalTrend.templ, TrendSection, TrendBadge_
- [x] 5.2 左サイドメニュー「統計分析」項目
  - `sidebar.go` に `NavAnalysisTrend` を追加し、`Sidebar.templ` にトップ階層リンク（常時表示・ダッシュボード等と同列）として「統計分析」を追加する。モックにも反映
  - 観測可能完了: サイドバーが `NavAnalysisTrend` を active 制御込みで描画し、`/analysis/trend` への常時表示リンクが HTML に含まれるテストが緑
  - _Requirements: 1.1_
  - _Boundary: sidebar.go, Sidebar.templ_

- [x] 6. ハンドラ統合（ルート配線・認可・判定組立・HX 分岐）
- [x] 6.1 ルート登録・所有者認可・一次判定のフルページ描画
  - `cmd/server/main.go` に `web.GET("/analysis/trend", RequireAuth(), …)` を登録し、`device_id`/`granularity` を binding 検証する
  - `RequireDeviceOwner` で所有検証（非所有/不在→404・列挙防止）、JST ロールアップ取得→Seasonal MK/Sen/N_eff の一次判定を組み立て、デバイス未選択/データ無は断定せず案内表示する。集計軸（デバイス、利用可能なら地点/作物・月別軸）を選べる
  - 観測可能完了: 自己所有 device 選択でフルページにトレンドが表示され、非所有→404・未認証→認証要求、未選択は空セクションになる httptest が緑
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 3.1, 3.2, 4.2, 4.3, 9.1, 9.2, 9.3_
  - _Depends: 2.1, 2.2, 2.3, 3.1, 4.1, 5.1, 5.2_
  - _Boundary: SeasonalTrendHandler_
- [x] 6.2 厳密判定・検出力留保バッジ・平年比・HX 部分返却
  - Hamed-Rao 補正・ブートストラップ CI・多重比較補正を同期で組み立て、一次判定/補正済みをラベル区別する。`N_eff<10 または span<3年` は検定を断定せず Sen 傾き＋符号＋記述統計に留め「非有意≠トレンド無し」を表示する。平年比を組み込み、回帰トレンド線は探索用として区別する
  - `HX-Request` 時は `TrendSection` を部分返却、通常はフルページ（alert_rule と同型）。デバイス/期間セレクタは swap 対象外で Tom Select 状態を保持
  - 観測可能完了: `HX-Request` でデバイス/期間切替が `#trend-section` に差し替わり、年数不足時に留保注記＋記述統計フォールバック、厳密判定バッジが補正済みラベルで表示される httptest が緑
  - _Requirements: 1.2, 3.5, 5.1, 5.2, 5.3, 5.4, 6.1, 6.2, 6.3, 7.1_
  - _Depends: 2.4, 2.5, 3.1, 4.2, 6.1_
  - _Boundary: SeasonalTrendHandler_

- [x] 7. Validation: 無回帰・クリティカルパス・カバレッジ
- [x] 7.1 無回帰確認と全体カバレッジ
  - 別型 TrendChartSpec・新クエリ追加・既存クエリ無改変により、既存の温湿度グラフ・統計・VPD・露点・GDD・品質メタ・期間操作・CSV エクスポート、および device-show/dashboard/readings/P2〜P7 が無回帰であることを既存テスト緑で確認する。認証・所有者認可・CSRF・期間バリデーションの既存仕様も無変更
  - クリティカルパス（デバイス選択→トレンド表示、非所有→404、HX 部分更新差し替え、空月、年数不足フォールバック）の統合テストとカバレッジ80%以上を確認する
  - 観測可能完了: `go test ./...` が全緑、新規パッケージのカバレッジが80%以上、既存テストに回帰がない
  - _Requirements: 8.1, 8.2, 8.3_
