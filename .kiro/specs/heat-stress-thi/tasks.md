# Implementation Plan — heat-stress-thi（Phase 12 高温ストレス）

> 逐次（sequential）。上から1行ずつ `/tdd` で実装。`(P)` なし。スキーマ変更・新規クエリ・マイグレーションなし（Decision D2＝既存 `ListDailySensorAggregatesJST`＋`ListRecentSensorReadings` 再利用・goose 00010 据置・`make db-snapshot` 不要）。

- [x] 1. Foundation: 高温ストレスパネルの器とカラートークンをモック正本へ追加し本番CSSへ同期
  - CSS/HTML の正本である `mocks/html/device-show.html` の GDD パネルの下に高温ストレスパネルの器を追加（THI ヒートマップ・熱帯夜カレンダー・夜温/ΔT・AH の各プレースホルダ枠、現在THI/現在AH/直近夜温/連続日数のカード=`summary-grid summary-grid-4`、visualMap カラースケールの枠とラベル）。本番 templ の描画実装は Task 6 で行う（本タスクはモック正本のみ）
  - `mocks/html/style.css` の `:root` へ `--color-heat`（暑熱=暖色・温度橙/GDD赤 #e03131 と判別する hue・既定 #d6336c）を追加し、器スタイルは `@layer components` の既存クラス流用で独自クラス新設は最小
  - `make sync-css` で本番配信 CSS（生成物 `internal/view/public/css/style.css`・手編集しない）を再生成
  - 観測可能完了: ブラウザでモック `device-show.html` を開くと高温ストレスパネルの器・カード・カラースケール枠が表示され、`mocks/html/style.css` と同期後の本番 CSS の双方に `--color-heat` が含まれる
  - _Requirements: 3.1, 4.3, 7.1_
  - _Boundary: mocks/html（CSS 単一ソース正本）_

- [x] 2. 純粋計算層: 暑熱指標の純関数
- [x] 2.1 THI と絶対湿度 AH の純関数を実装
  - THI = `0.8·T + (RH/100)·(T−14.4) + 46.4` をスカラ/系列で算出、RH を [0,100] にクランプ
  - AH を実水蒸気圧 `ea`（既存の飽和水蒸気圧算出を再利用・Tetens 定数を重複定義しない）から算出し、RH=0 で AH=0
  - 氷点下（−40℃）で NaN/無限大/ゼロ割が出ないこと、入力スライス非破壊
  - 観測可能完了: 既知の手計算ケース（特定 T,RH）に対し THI/AH が一致し、RH クランプ・氷点下・RH=0 を検証する table-driven テストが緑
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7_
  - _Boundary: internal/chart（純粋層）_

- [x] 2.2 熱帯夜マスクと連続日数（最長/現在）の純関数を実装
  - 夜温（日次代表気温）が閾値以上の日を true とするマスク（NaN/欠測は false）
  - マスクから最長連続・末尾（現在）連続の日数を算出（既存の同値連続ラン検出と同型）
  - 全日該当/全日非該当/単発/空を破綻なく処理、夜温=閾値ちょうどは該当
  - 観測可能完了: 既知系列に対し最長/現在連続が一致し、境界（=閾値・空・単発・全日）を検証するテストが緑
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.6_
  - _Boundary: internal/chart（純粋層）_

- [x] 3. 描画層: 入力契約と option 構築（heatmap/calendar/visualMap/line）
- [x] 3.1 入力契約（別型）と THI 時間帯×日ヒートマップ option を実装
  - 高温ストレス用の入力契約を別型で定義し既存 Spec 群へ非破壊追加（温湿度/VPD/露点/GDD/トレンドの無回帰を守る）
  - 時間帯×日の heatmap + visualMap（連続色・暑熱=暖色＝高THIほど濃い暖色）の option JSON を構築、HTML 安全（`SetEscapeHTML`・`</script>` 不混入）、空データで空 option
  - 観測可能完了: option JSON に小文字キー（series type=heatmap・visualMap・暖色の inRange.color 並び）が含まれ、空データでも破綻しない `strings.Contains` テストが緑
  - _Requirements: 4.1, 4.2, 4.4, 7.1_
  - _Boundary: internal/chart（heatstress_echarts, series）_
  - _Depends: 2.1_

- [x] 3.2 熱帯夜カレンダーヒートマップ option を実装
  - calendar 座標系 + heatmap + visualMap（夜温の高さ・暑熱=暖色）の option JSON、直近1年レンジ
  - go-echarts ネイティブ型を本リポジトリの直列化経路に通し `calendar`/`visualMap` キーが camelCase で残ることを実証（残らない属性のみ小文字キー自前注入＝Q1）
  - 観測可能完了: option JSON に `calendar`・`visualMap`・heatmap series が camelCase で含まれ、暖色スケール・空データを検証するテストが緑
  - _Requirements: 3.1, 3.2, 3.5, 7.1_
  - _Boundary: internal/chart（heatstress_echarts）_
  - _Depends: 3.1_

- [x] 3.3 夜温推移・日較差ΔT・絶対湿度 AH の line option を実装
  - 夜温推移 + 日較差ΔT の2 line option と、AH（除湿負荷）line option を構築、空データで破綻しない
  - 観測可能完了: 各 option JSON が line series を含み、空データで空 option を返すテストが緑
  - _Requirements: 3.3, 5.1, 5.2, 5.3, 5.4_
  - _Boundary: internal/chart（heatstress_echarts）_
  - _Depends: 3.1_

- [x] 3.4 熱帯夜年間日数トレンドの option を実装
  - 年間日数の棒 + Sen 傾き線（markLine）の option JSON、トレンド非表示時は描かない
  - 観測可能完了: 年系列と Sen 線を含む option が生成され、トレンド無し時に空 option を返すテストが緑
  - _Requirements: 6.1_
  - _Boundary: internal/chart（heatstress_echarts）_
  - _Depends: 3.1_

- [x] 4. View DTO: 高温ストレスパネルの表示モデルを定義し device チャート領域 ViewModel へ末尾非破壊追加
  - 高温ストレスパネルの表示モデル（option JSON 群・カード文言・連続日数・トレンドノート・Guidance・HasData/HasTrend）をイミュータブルに定義
  - 既存の device チャート領域 ViewModel の末尾へ高温ストレスフィールドを追加（既存フィールド順を変えない）
  - 観測可能完了: view component パッケージがコンパイルでき、チャート領域 ViewModel 末尾に高温ストレスが追加され、既存 templ/handler テストが緑のまま
  - _Requirements: 9.2, 10.1_
  - _Boundary: internal/view/component（views.go）_

- [x] 5. handler: 高温ストレスパネルの組立（データアクセス・バケット境界）
- [x] 5.1 既存クエリでデータ取得し空データ時に空パネルを返す
  - 既存の JST 日次集約クエリと直近生行クエリを呼ぶ（新規クエリ・マイグレーションなし）、日次集約の `interface{}` の最高/最低気温を float 化
  - 両方空なら HasData=false ＋ Guidance 注記（error を返さない・レイアウト非破壊）
  - 観測可能完了: Querier モックで空入力時に HasData=false の高温ストレス ViewModel が返り、データ有り時に系列が満たされるテストが緑
  - _Requirements: 9.1, 9.2, 9.4_
  - _Boundary: internal/handler（device_show_heatstress）, repository.Querier_
  - _Depends: 4_

- [x] 5.2 熱帯夜判定・連続日数・夜温/ΔT 日次系列を組み立てる
  - 夜温＝JST 暦日の最低気温、作物非依存の既定閾値定数 25℃で熱帯夜判定し、純粋層の連続ランで最長/現在連続を算出
  - 直近1年の夜温推移と日較差ΔT（日次 最高−最低）を組む、欠測日（行なし）は熱帯夜0と誤計上せず非該当
  - 観測可能完了: モック日次から熱帯夜カレンダーセル・夜温/ΔT 系列・連続日数が組まれ、欠測日が非該当になるテストが緑
  - _Requirements: 2.5, 2.7, 3.3, 3.4, 5.2, 8.1, 8.4_
  - _Boundary: internal/handler（device_show_heatstress）_
  - _Depends: 2.2, 3.1, 3.2, 3.3, 5.1_

- [x] 5.3 THI 時間帯×日バケットと AH 系列・現在値カードを組み立てる
  - 直近生行を JST の（暦日×時間帯）でバケットし平均 THI をヒートマップセルに、AH 系列を line に、最新計測から現在 THI/現在 AH/直近夜温カードを整形
  - 観測可能完了: モック生行から時間帯×日セル・AH 系列・各カード文言が組まれるテストが緑
  - _Requirements: 1.7, 4.1, 5.1, 5.3_
  - _Boundary: internal/handler（device_show_heatstress）_
  - _Depends: 2.1, 3.1, 3.3, 5.1_

- [x] 5.4 熱帯夜年間日数の経年トレンドを既存基盤の再利用で算出
  - JST 年ごとに夜温≥閾値の日数を計数し、2年以上のとき既存の Mann-Kendall/Sen 傾きを再利用（新規統計を実装しない）、数年では Sen 傾き＋符号＋記述統計に留め有意性を断定しない
  - 「非有意≠トレンド無し・複数年必要」のノートを付け、年系列が1点以下は HasTrend=false（device-show のミニ表示・本格多年横断は P8 系列へ繰り延べ＝D7）
  - 観測可能完了: 多年モックで Sen 傾き＋符号が出て、1年以下で HasTrend=false＋注記になり、熱帯夜0年が複数（タイ）でも破綻しないテストが緑
  - _Requirements: 6.1, 6.2, 6.3, 6.4_
  - _Boundary: internal/handler（device_show_heatstress）, internal/chart（trend 再利用）_
  - _Depends: 3.4, 5.1_

- [x] 6. 高温ストレスブロックを device-show のチャート領域へ描画（モックの器を写経して本番 templ 実装）
  - Task 1 のモック器を写経し、GDD パネルの下に THI ヒートマップ・熱帯夜カレンダー・夜温/ΔT・AH の各コンテナ（`data-echarts data-no-connect data-color`）と option script、カード（`summary-grid summary-grid-4`）、連続日数、トレンドノートを本番 templ で描画。トレンドは HasTrend 時のみ、空データ時は Guidance のみ、独自クラス新設は最小
  - 観測可能完了: 高温ストレス ViewModel を与えて templ をレンダリングすると各コンテナ id・`data-no-connect`・カード文言・カラースケール枠が HTML に現れ、空データ時は Guidance のみになる `Render`→`strings.Contains` テストが緑
  - _Requirements: 3.1, 4.3, 5.1, 6.1, 7.1, 9.3_
  - _Boundary: internal/view/component（DeviceChartArea.templ）_
  - _Depends: 1, 4_

- [x] 7. Integration: device-show のチャート領域組立に高温ストレスパネルを配線
  - チャート領域組立から高温ストレスパネル組立を呼び ViewModel 末尾へ詰める（VPD/露点/GDD の列に追加）、全体表示の経路のみ（期間切替の部分更新には追加しない＝period 非連動）、所有者認可は既存の全体表示経路に相乗り
  - 観測可能完了: 自分の device の device-show 全体 GET で高温ストレスパネルが描画され、期間切替（部分更新）では再計算されず、他者 device は既存どおり 404 になる統合テストが緑
  - _Requirements: 9.3, 10.1, 11.1, 11.2, 11.3, 11.4, 11.5_
  - _Boundary: internal/handler（device_show）_
  - _Depends: 5.2, 5.3, 5.4, 6_

- [x] 8. Validation: 無回帰と実機スモーク
- [x] 8.1 既存可視化の無回帰と calendar の period 非連動を確認
  - 温湿度2グラフ・統計オーバーレイ・VPD/露点/GDD パネル・欠測ギャップ・期間切替・URL同期・connect 連動・空データ表示の既存テストが緑のまま、高温ストレスチャートが connect 連動に混ざらない（`data-no-connect`）
  - 期間切替（24h/3d/7d/30d）で熱帯夜カレンダーが当該短期窓へ縮まず直近1年で固定されること（period 非連動）を独立した回帰テスト項目として検証
  - 観測可能完了: 既存 device-show 関連テスト一式・period 非連動テスト・新規テストが緑、全体テストカバレッジ 80% 以上
  - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5_
  - _Boundary: internal/handler, internal/view/component_
  - _Depends: 7_

- [x] 8.2 暑熱=暖色の向きと新規ヒートマップの実機スモーク
  - GO 判定後にローカル（多年は `make seed-trendsensor`）で device-show を開き、熱帯夜カレンダー濃淡・THI visualMap・カード/ラベルが暑熱=暖色で一貫し、ヒートマップ/カレンダーが初期化・破棄され温湿度 line の connect と干渉しないこと（Q2）を目視確認
  - 観測可能完了: 実機画面で暖色の向きが正しく、ページ再描画・離脱で残留要素やコンソールエラーが無いことを確認したスモーク記録
  - _Requirements: 7.1, 7.2, 7.3, 9.3_
  - _Boundary: device-show（実機スモーク）_
  - _Depends: 7_

> **意図的繰り延べ（要件 8.2, 8.3）**: 作物別の高温ストレスしきい値を作物マスタへ非破壊追加する 8.2/8.3 は Design Decision D6 により本フェーズでは実装せず、作物非依存の既定閾値定数（Task 5.2）で 8.1/8.4 を満たす。作物別が必要と判明した将来フェーズで `VPDRange()`/`GDDModel()` と同型の `HeatStressModel()` 拡張点として追加する（DB 列は増やさない）。
