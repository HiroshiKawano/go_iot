# Implementation Plan — dewpoint-disease-risk（露点・病害リスク）

> 前提（design.md / research.md）: P3 vpd-dashboard の構造的双子。**スキーマ非変更**（goose 00009 のまま・migration / sqlc / `make db-snapshot` 不要）。`internal/chart` 純粋層（`math` のみ・time 非依存）→ 描画層（go-echarts + 自前 markArea 注入）→ handler（時刻バケットは境界）→ view/templ → モック（CSS 単一ソース）の順で写経主体に積み上げる。逐次（上から1行ずつ `/tdd`）。テストは `2cc_sdd/テストガイダンス集.md` の定石（純関数 table-driven・`httptest`+gin・templ `Render`→`strings.Contains`・Querier モック・列挙防止）。
>
> **物理規約（最重要・project_vpd_physics_convention）**: 結露・葉面湿潤・高湿度は VPD の「湿り側＝寒色」と同じ向き。色・符号・ラベルを乾き側と取り違えない。自動テストは仕様前提を符号化するため向きの誤りを捕捉できない → **タスク 4.4 の実機スモーク目視確認を必須**とする（P3 VPD で全緑のまま実機まで誤りが残った前例）。

## 1. Foundation: 純粋計算層と作物メタ

- [x] 1.1 露点 Td・スプレッドの純関数を実装
  - `internal/chart/dewpoint.go` に `DewPoint(tempC, rh)` / `DewPointSeries(temps, hums)` / `DewPointSpread(temps, dewpoints)` を実装する。`γ = ln(RHc/100) + tetensB·T/(T+tetensC)`、`Td = tetensC·γ/(tetensB − γ)`。定数 `tetensB`/`tetensC` は `vpd.go` から再利用（重複定義しない・`tetensA` は不使用）
  - RH は `[rhFloor, 100]`（`rhFloor = 1.0`）に床上げして `ln(0)=−Inf` を回避する。スプレッドは `T − Td` を 0 下限クランプ（常に ≥0）
  - `math` のみ依存・time 非依存を厳守し、入力スライスを破壊しない
  - 観測可能完了: `dewpoint_test.go` が table-driven で緑（既知手計算一致・**RH=100→Td=T 恒等**・RH=0/微小で NaN/Inf なし・**氷点下 −40℃ で NaN/Inf なし**・スプレッド全要素 ≥0・`DewPointSeries` 長さ=min）
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 2.1_

- [x] 1.2 結露帯・葉面湿潤・高湿度イベントの連続区間純関数を実装
  - `dewpoint.go` に共有内部 `runsFromMask(mask []bool, minRun int) []Run`（`Run{StartIdx,EndIdx}`）と、`CondensationRuns(spread, maxSpread)`（spread ≤ maxSpread の連続・minRun=1＝**湿り側**）・`WetnessMask(hums, rhThreshold)`・`HighHumidityRuns(hums, rhThreshold, minRun)`（RH ≥ threshold が minRun 点以上連続）を実装する
  - 単発（minRun 未満）は高湿度イベントから除外し、隣接する連続は1区間に結合する。`quality.go`（P5 所有）は改変しない
  - 観測可能完了: テストが緑（しきい値境界＝境界値は湿潤/結露扱い・最小継続ちょうど/未満で除外・先頭/末尾/全域結露/無結露・空入力で空スライス・`Run.End≥Run.Start`）
  - _Requirements: 2.2, 3.1, 4.1, 4.2_
  - _Boundary: chart/dewpoint.go_

- [x] 1.3 病害スコア下地の純関数を実装
  - `dewpoint.go` に `DiseaseScore(temps []float64, wet []bool, tempLow, tempHigh float64) float64` を実装する。「発病好適温度帯 `[tempLow,tempHigh]` 内かつ葉面湿潤」を合成した下地スコア（0..1）を返す（最小合成）。確定予察モデル・特定病害予察は含めない
  - 観測可能完了: テストが緑（温度帯内×湿潤のみ寄与・温度帯外/非湿潤は寄与なし・空入力 0・`0≤score≤1`）
  - _Requirements: 5.1, 5.3_
  - _Boundary: chart/dewpoint.go_

- [x] 1.4 作物別の病害モデルしきい値を作物マスタへ非破壊追加
  - `internal/domain/crop.go` に `DiseaseModel` 型（結露スプレッド上限・葉面湿潤 RH 閾値・最小継続時間・発病好適温度帯下限/上限）と `(c Crop) DiseaseModel()`・`DefaultDiseaseModel` を**非破壊追加**する。既存 `VPDRange`/`Label`/`Valid`/`AllCrops`/`ParseCrop` と9作物集合は不変。**DB 列は追加しない**（`fmt` のみ依存・§100）
  - 施設果菜（goya/ingen/uri/mango）等に暫定値、露地・未設定・未定義作物は `DefaultDiseaseModel` にフォールバックする
  - 観測可能完了: `crop_test.go` 追加分が緑（施設果菜=暫定値・露地/未設定/不正→既定）。既存 `VPDRange`/`AllCrops` テストは不変（無回帰）。`go build ./...` が通る
  - _Requirements: 5.2, 5.4, 6.1, 6.2, 6.3, 6.4, 6.5_
  - _Boundary: domain/crop.go_

## 2. Core: 描画層と露点パネル組立

- [x] 2.1 露点 option ビルダーと結露帯 markArea を実装
  - `internal/chart/series.go` に `DewpointChartSpec`（露点 Td・気温・結露帯 `[]Run`・色 等）を**別型で隔離**追加（`ChartSpec`/`VPDChartSpec` は不変）。`internal/chart/dewpoint_echarts.go` に `DewpointChartOptionJSON(spec)` を実装し、**series[0]=露点 Td 線（寒色 `--color-dewpoint`）＋series[1]=気温 T 重ね線**を構築する
  - 結露帯（`CondensationRuns` の各区間）を **xAxis 範囲指定の markArea** で帯ハイライトする。P5 `injectGapMarkArea` と同型の小文字 `xAxis` キー自前注入を専用関数で起こす（P5 `gapZone` は改変しない・**結露帯は寒色＝湿り側**）。y 軸は共通℃の auto 範囲（YMax 算出は不要）
  - 観測可能完了: `dewpoint_echarts_test.go` が緑（option に小文字 `"xAxis"` の markArea が区間数ぶん含まれる・series 2本・結露帯空で markArea なし・`</script>` 不混入＝HTML 安全・結露帯色が寒色トークン）
  - _Requirements: 2.1, 2.2, 2.4_
  - _Depends: 1.1, 1.2_
  - _Boundary: chart/series.go, chart/dewpoint_echarts.go_

- [x] 2.2 露点パネル handler の骨格と露点カードを実装
  - `internal/handler/device_show_dewpoint.go` に `buildDewpointPanel(labels, temps, hums, rows, crop, period, now)` を起こし、`crop.DiseaseModel()` でしきい値を解決（未設定→既定）。露点系列・スプレッド・結露帯区間を組み `DewpointChartOptionJSON` を呼んで option を作る
  - 露点カード（現在露点・現在スプレッド T−Td・直近の結露帯有無）を整形（`formatStat`/`statEmptyMark="—"` 流用・℃ 小数1桁）。葉面温度を気温で近似した代理判定である旨をパネルに載せる注記文言を View へ渡す
  - 観測可能完了: `device_show_dewpoint_test.go` が緑（純データ・Querier 不要。露点カードの整形値・option JSON 非空・近似注記文言が含まれる）
  - _Requirements: 2.1, 2.3, 2.5_
  - _Depends: 2.1, 1.4_
  - _Boundary: handler/device_show_dewpoint.go_

- [x] 2.3 葉面湿潤時間の日次積算をパネルへ追加
  - `buildDewpointPanel` に、`WetnessMask` と連続する `recorded_at` 間隔から JST 暦日ごとの葉面湿潤時間（時間/日）を積算する処理を handler 境界で加える（`jstDay`/`jst` 流用）。異常に長い間隔（欠測/停波）は上限 `maxWetGap` でキャップして水増しを防ぐ
  - period 内の暦日を日次行にする（3d/7d/30d は複数行・24h は当日）。日次行は病害スコア下地（`DiseaseScore`）の列も併せ持つ
  - 観測可能完了: テストが緑（日跨ぎで正しい暦日へ計上・単一点/空で破綻なし・負の時間を生じない・`maxWetGap` キャップが効く・期間で暦日数が変わる）
  - _Requirements: 3.1, 3.2, 3.3, 3.4_
  - _Depends: 2.2_
  - _Boundary: handler/device_show_dewpoint.go_

- [x] 2.4 高湿度継続イベント一覧をパネルへ追加
  - `buildDewpointPanel` に、`HighHumidityRuns` の各区間を rows の `recorded_at` で時刻化（開始・終了・継続時間・区間内の最小スプレッドまたは最大 RH）してイベント行にする処理を加える。該当なしは空一覧
  - 観測可能完了: テストが緑（最小継続以上の区間のみ抽出・単発除外・開始/終了/継続が時刻整形される・該当なしで空）
  - _Requirements: 4.1, 4.2, 4.3, 4.4_
  - _Depends: 2.2_
  - _Boundary: handler/device_show_dewpoint.go_

- [x] 2.5 病害スコア下地をパネルへ統合
  - `buildDewpointPanel` に、`crop.DiseaseModel()` の発病好適温度帯と `WetnessMask` から `DiseaseScore` を日次（または期間）で算出し、病害スコア欄へ整形して載せる。**作物未設定/露地でも既定モデルで具体値が出る**ようにし、欄が常に空表示にならないことを保証する
  - 観測可能完了: テストが緑（少なくとも1作物＝施設果菜で具体スコアが描画される・未設定でも既定モデルで非空・下地に限定＝発病記録突合や確定予察を含まない）
  - _Requirements: 5.1, 5.2, 5.4_
  - _Depends: 2.2, 1.3, 1.4_
  - _Boundary: handler/device_show_dewpoint.go_

## 3. Integration: View/templ・モック反映・buildChartArea 結線

- [x] 3.1 露点パネルの DTO と templ 描画を実装
  - `internal/view/component/views.go` の `DeviceChartAreaView` 末尾に `Dewpoint DewpointPanelView` を非破壊追加し、`DewpointPanelView`/`DewpointCardView`/`DewpointDailyRow`/`HighHumidityEventRow` をイミュータブル DTO で定義する
  - `DeviceChartArea.templ` の `if v.HasData` 内・VPD パネルの下に露点パネルを描画する: `#dewpoint-chart`（`data-echarts data-unit="℃" data-color`）＋兄弟 option script、露点カード（`.summary-grid-4` 流用）、葉面湿潤日次表＋高湿度イベント表（`.data-table` 流用）、近似注記。独自クラス新設は最小
  - 観測可能完了: `templ generate` 後、templ Render 検証が緑（`id="dewpoint-chart"`・`data-unit="℃"`・露点カード・葉面湿潤日次表・高湿度イベント表・近似注記文言が HTML に出る）
  - _Requirements: 2.3, 3.1, 4.1, 5.2_
  - _Depends: 2.2, 2.3, 2.4, 2.5_
  - _Boundary: view/component/views.go, DeviceChartArea.templ_

- [x] 3.2 露点パネルの器とCSSトークンをモック正本へ反映
  - `mocks/html/style.css`（CSS 単一ソース正本）に `--color-dewpoint`（寒色＝湿り側）トークンを `--color-vpd` の隣に追加する。`mocks/html/device-show.html` の VPD パネルの下に露点パネルの器（チャート枠・露点カード・葉面湿潤/病害スコア枠・高湿度イベント表・近似注記）を追加し、templ と構造・クラス・ラベルを一致させる
  - グラフ内部描画（露点線/気温重ね/結露帯 markArea）はモック反映の例外。器のみ反映。`make sync-css` で本番配信 CSS を再生成する
  - 観測可能完了: `make sync-css` が成功し `internal/view/public/css/style.css` に `--color-dewpoint` が含まれる。モックをブラウザで開くと露点パネルの器が VPD パネルの下に表示される（feedback_mock_reflects_impl_visual）
  - _Requirements: 2.4_
  - _Depends: 3.1_
  - _Boundary: mocks/html/style.css, mocks/html/device-show.html_

- [x] 3.3 buildChartArea へ露点パネルを結線
  - `internal/handler/device_show.go` の `buildChartArea` の HasData 分岐で、VPD パネル組込の直後に `buildDewpointPanel(...)` を呼び `DeviceChartAreaView.Dewpoint` へ格納する（`deviceCrop(device)`・`now` は既存・signature 変更なし）。温湿度/VPD/日次/ギャップの出力は不変
  - HasData=false（計測0件）では露点パネルを組まず、templ 側で非表示（既存分岐に相乗り）
  - 観測可能完了: `Show`/`Chart` の統合テスト（httptest＋Querier モック）が緑で、所有デバイスの GET に露点パネルが含まれ、0件時は非表示＋「データはまだありません」
  - _Requirements: 2.6, 8.1_
  - _Depends: 3.1, 2.5_
  - _Boundary: handler/device_show.go_

## 4. Validation: 統合・無回帰・認可・物理規約スモーク

- [x] 4.1 露点パネルの統合テスト（描画・期間切替）
  - `Show`/`Chart` が露点パネルを描画することを `strings.Contains` で検証する（露点カード・葉面湿潤日次表・高湿度イベント表・近似注記）。期間切替フラグメント（`Chart` 24h/3d/7d/30d）に露点パネルが含まれ、日次表が期間の暦日数に追従し、不正 period→400 になる
  - 観測可能完了: 統合テストが緑（4期間で露点パネル描画・近似注記の明示・不正 period 400）
  - _Requirements: 2.5, 8.1_
  - _Depends: 3.3_

- [x] 4.2 既存可視化の無回帰テスト
  - 露点追加後も温湿度2グラフ（`#temperature-chart`/`#humidity-chart`）・統計オーバーレイ・数値カード・日次集計表・**VPD パネル（`#vpd-chart`）**・欠測ギャップ・品質メタ・期間ボタン active 往復・URL 同期・connect・空データ表示が従来同等であることを検証する（温湿度/VPD の option 文字列が不変）
  - 観測可能完了: 無回帰テストが緑（既存 `device_show_vpd_regression_test.go` 等と合わせ、温湿度・VPD・ギャップ・品質の描画が露点追加前と一致）
  - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5_
  - _Depends: 3.3_

- [x] 4.3 認可と研究用スコープ境界のテスト
  - 非所有デバイスの `Show`/`Chart` が 404（列挙防止・既存認可フロー）になることを検証する。露点パネルが研究用に限定され、農家向け平易表示（信号色の病害警告）・圃場共有 URL・病害アラート/通知の発火・THI/絶対湿度/GDD/多地点比較を描画しないことを不在アサートで確認する
  - 観測可能完了: テストが緑（非所有→404・農家平易表示/共有URL/病害アラート/他指標の不在）
  - _Requirements: 8.2, 8.3, 8.4, 8.5_
  - _Depends: 3.3_

- [x] 4.4 物理規約の向き符号化と実機スモーク目視確認（**符号化テスト緑・実機スモーク目視 合格 2026-06-29**）
  - 結露帯・葉面湿潤・高湿度を「湿り側＝寒色」に揃える向きを、色トークン（`--color-dewpoint`）・結露帯 markArea 色・カード/イベントの符号・「結露/乾燥」ラベルの向きとしてテストで符号化する（乾き側と取り違えていないこと）→ **完了**（`device_show_dewpoint_validation_test.go` の `TestValidation_物理規約_*` 3本＋`dewpoint_echarts_test.go` の `TestDewpointChartOptionJSON_CondensationBandIsColdColor`・`_SeriesItemStyleMatchesLine` が緑）
  - 実機スモーク（chrome-devtools・ローカル :8090 にフレッシュバイナリ起動・device 1 へ湿潤データ注入）で目視確認 → **合格**: 結露帯 markArea が寒色（`rgba(66,99,235,0.12)` 青）で湿潤区間のみを覆い、乾き区間には帯なし。露点線=青(寒色)/気温線=橙(暖色)で接近関係が読める。カード「直近の結露帯＝結露中」が湿り側で正しい。**スモークで凡例マーカー色の不一致（気温の凡例が既定パレットの緑・線は橙）を捕捉 → 両系列に `itemStyle.color` を追加して線色と一致させ修正（テスト追加・再スモーク合格）**
  - 観測可能完了: 向きを符号化したテストが緑（達成）、かつ実機スモークで結露帯が寒色・「結露」ラベルが湿り側で表示されることを目視確認済み（**達成**・project_vpd_physics_convention）
  - _Requirements: 2.4_
  - _Depends: 4.1, 4.2, 4.3_
