# ECharts 系列マーカー色バグ調査

## 概要

- **発見日**: 2026-07-02〜03
- **発見経緯**: `dark-mode` スペック（`.kiro/specs/dark-mode/`）タスク4.3「実機スモーク: グラフ・意味色」実施中、グラフ検証エージェントが意味色（要件6.1/6.2）の目視確認を行った際に副次的に発見。
- **性質**: **ダークモード機能とは無関係の既存バグ**。ライト/ダーク両テーマで完全に同一に再現するため、dark-mode 実装（テーマ patch）が原因ではない。`internal/chart` 配下のチャートオプション生成コードに起因する。
- **本スペックでの扱い**: `dark-mode` の `design.md` は Out of Boundary として「サーバ側 chart option の内容（`internal/chart` は原則非変更。系列色・markArea 定数は既存所有のまま）」と明記しており、本バグの修正は `dark-mode` スペックの境界外。**別セッション・別スペック（bugfix）での対応を想定**し、本ドキュメントに調査結果を残す。

## 根本原因（共通パターン）

go-echarts の line/bar 系列は `charts.WithLineStyleOpts(opts.LineStyle{Color: ...})` で **線の色**は指定できるが、データ点のシンボルマーカー（既定 `emptyCircle`、`ShowSymbol` 未指定時は `true`）の色は **別途 `charts.WithItemStyleOpts(opts.ItemStyle{Color: ...})` を指定しない限り ECharts 既定パレット色（`#5470c6` 等の虹色パレット）で描画される**。

`internal/chart/dewpoint_echarts.go` の実装は `LineStyleOpts` と `ItemStyleOpts` の両方に同じ色を渡しており、これが正しい実装の模範例になっている。他の複数のチャートビルダーではこの `ItemStyleOpts` 指定が欠落しており、線は意図した意味色で描画されるがマーカーだけ既定パレット色になる、という不整合が生じている。

## 影響範囲・重大度別詳細

### 🔴 重大: GDD累積曲線（`internal/chart/gdd_echarts.go`）

- **箇所**: `GDDChartOptionJSON()` 内、`internal/chart/gdd_echarts.go:55-56`
  ```go
  line.AddSeries(seriesNameGDD, gddCoordData(spec.ElapsedDays, spec.Cumulative),
      charts.WithLineStyleOpts(opts.LineStyle{Color: spec.Color}),
  )
  ```
- **症状**: 線自体は `lineStyle.color=#e03131`（要件6.1どおり赤）で正しく設定されているが、`ItemStyleOpts` が無いためデータ点シンボル（`emptyCircle`、既定 `symbolSize`）が ECharts 既定パレット青 `#5470c6` で描画される。長期間表示（実機確認は30日間表示、61点）ではマーカーが密集し、線幅2pxの赤線をほぼ覆い隠す。**実際の見た目は「赤い線」ではなく「青いグラフ」に見える。**
- **要件との齟齬**: 要件6.1「GDD累積線は赤」を視覚的に満たしていない（データとしては赤だが視認できない）。
- **他系列との関係**: GDD チャートは系列がこの1本のみのため、他の帯・オーバーレイに紛れて目立たなくなる余地もなく、最も影響が大きい。

### 🟠 中: 夜温推移・日較差ΔT（`internal/chart/heatstress_echarts.go`）

- **箇所**: `NightTempDeltaLineOptionJSON()` 内、`internal/chart/heatstress_echarts.go:173-177`
  ```go
  line.AddSeries(seriesNameNightTemp, lineData(spec.NightTemps),
      charts.WithLineStyleOpts(opts.LineStyle{Color: spec.Color}),
  )
  line.AddSeries(seriesNameDeltaT, lineData(spec.DeltaT),
      charts.WithLineStyleOpts(opts.LineStyle{Color: spec.Color, Width: 1}),
      charts.WithLineChartOpts(opts.LineChart{ShowSymbol: opts.Bool(false)}),
  )
  ```
- **症状**: 「夜温推移」「日較差ΔT」の**2系列が同一の `spec.Color`**（`HeatStressChartSpec.Color`、暑熱基準色 `#d6336c` 系）を参照しており、色で区別できない。加えて `seriesNameNightTemp` 側は `ShowSymbol:false` が無いため、既定パレット色のマーカー（青 `#5470c6`）も乗り、凡例アイコンと実線色が食い違って見える。
- **備考**: `seriesNameDeltaT` 側は `ShowSymbol:false` 済みなのでマーカー色の問題は無いが、色そのものが `seriesNameNightTemp` と同一という別問題がある。

### 🟡 軽微〜中: 絶対湿度AH（`internal/chart/heatstress_echarts.go`）

- **箇所**: `AHLineOptionJSON()` 内、`internal/chart/heatstress_echarts.go:199-200`
  ```go
  line.AddSeries(seriesNameAH, lineData(spec.AH),
      charts.WithLineStyleOpts(opts.LineStyle{Color: spec.Color}),
  )
  ```
- **症状**: `ItemStyleOpts` 未設定のため、凡例の丸アイコンは ECharts 既定パレット青で表示される一方、実際の折れ線は `spec.Color`（マゼンタピンク系）で描画され、**凡例と実線の色が一致しない**（ECharts は line 系列の凡例アイコンに itemStyle 由来の色を優先して使う挙動があるため）。データ点マーカーも同様に既定青が混在する。

### 🟡 軽微: 温度・湿度・VPD折れ線（`internal/chart/echarts.go`, `internal/chart/vpd_echarts.go`）

- **箇所**:
  - `internal/chart/echarts.go:100-112`（`ChartOptionJSON` の主役系列 `spec.Unit`。温度・湿度チャートで使用）
  - `internal/chart/echarts.go:114-120`（同関数の SMA 系列 `seriesNameSMA`）
  - `internal/chart/vpd_echarts.go:73-86`（`VPDChartOptionJSON` の VPD 本線・SMA）
- **症状**: 上記いずれも `ItemStyleOpts` 未設定。データ点マーカーが既定パレット青で描画され、系列本来の意味色（温度=橙、湿度=青、VPD=緑）と食い違う。GDD ほど密ではなく視覚的な破壊度は軽微だが、特に **VPD チャート（緑ラインに青マーカー）** は意味色（要件6.1）の観点でやや目立つ。
- **影響を受けない系列**: 同ファイル内の正常帯（`seriesNameBand`/`seriesNameBandLower`）・乖離率（`seriesNameDeviation`）・日スケールSMA（`DaySMAs`）は `ShowSymbol: opts.Bool(false)` が明示されているため、この問題の対象外（マーカー自体が描画されない）。

### ✅ 模範例（バグなし）: 露点・気温重ね線（`internal/chart/dewpoint_echarts.go:67-75`）

```go
line.AddSeries(seriesNameDewpoint, lineData(spec.Dewpoint),
    charts.WithLineStyleOpts(opts.LineStyle{Color: spec.DewColor}),
    charts.WithItemStyleOpts(opts.ItemStyle{Color: spec.DewColor}),
)
line.AddSeries(seriesNameDewTemp, lineData(spec.Temperature),
    charts.WithLineStyleOpts(opts.LineStyle{Color: dewTempOverlayColor, Width: 1}),
    charts.WithItemStyleOpts(opts.ItemStyle{Color: dewTempOverlayColor}),
)
```

`LineStyleOpts` と `ItemStyleOpts` に同じ色を渡しており、線・マーカー・凡例アイコンが完全に一致する。**修正時はこのパターンを横展開すればよい。**

### 影響なし（バー系列）: 熱帯夜年間日数トレンド

- `internal/chart/heatstress_echarts.go:223-224`（`TropicalNightTrendOptionJSON`）は `ItemStyleOpts` を明示設定済みで bar 型のためシンボルマーカーの概念も無く、この問題は発生しない。

## 推奨対応

各該当箇所の `charts.WithLineStyleOpts(opts.LineStyle{Color: X})` の直後に `charts.WithItemStyleOpts(opts.ItemStyle{Color: X})` を追加する（`dewpoint_echarts.go` と同型）。夜温推移・日較差ΔTについては色重複の解消（`DeltaT` 側に別の基準色を割り当てるか、`HeatStressChartSpec` に系列別カラーフィールドを追加）も合わせて設計判断が必要。

対象ファイル・関数一覧:
1. `internal/chart/echarts.go` — `ChartOptionJSON()` の主役系列 (L100-112) と SMA系列 (L114-120)
2. `internal/chart/vpd_echarts.go` — `VPDChartOptionJSON()` の VPD本線・SMA (L73-86)
3. `internal/chart/gdd_echarts.go` — `GDDChartOptionJSON()` (L55-56)
4. `internal/chart/heatstress_echarts.go` — `NightTempDeltaLineOptionJSON()` (L173-177、色重複の解消含む)・`AHLineOptionJSON()` (L199-200)

代替案（symbol自体を消す）: マーカーを表示する意図がない系列であれば `charts.WithLineChartOpts(opts.LineChart{ShowSymbol: opts.Bool(false)})` を追加する方法もある（正常帯・乖離率・日スケールSMAで既に採用されている方式）。ただし GDD・VPD・温湿度の主役線は端点の値強調（markPoint併用）の意図があるため、ItemStyle着色の方が既存デザイン意図に近いと考えられる。

## 検証方法（次回セッション向け）

1. 修正後、`go build ./... && go test ./...` で無回帰確認。
2. 実機で `/devices/{id}` の GDD・温度・湿度・VPDチャート、`/devices/{id}` の高温ストレスパネル（夜温推移・AH）を表示し、線色とマーカー色が一致することを目視確認。
3. 可能であれば `internal/chart/*_echarts_test.go` に「主役系列の `itemStyle.color` が `lineStyle.color` と一致する」構造アサートを追加し、回帰を機械的に防止する（`echarts_neutral_test.go` の構造アサート手法が流用できる）。

## 参考

- 発見元: `dark-mode` スペック タスク4.3 実機スモークテスト（Workflow `dark-mode-smoke-test`、エージェント `dark-mode-graph-verification` の `criticalIssues`）
- 対象デバイス: id=3「長期トレンドテスト（沖縄・10年）」、期間=30日間
- 関連スペック（本バグの影響を受ける既存機能）: `gdd-forecast`, `temp-humidity-chart-stats`, `vpd-dashboard`, `heat-stress-thi`, `dewpoint-disease-risk`（このうち `dewpoint-disease-risk` のみ実装が正しい）
