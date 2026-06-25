package chart

import "math"

// xAtIndex は i 番目の点 (全 n 点) の X 座標を返す。
// LineChartSVG の折れ線と LineChartHoverPoints のホバー座標で共有し、両者を一致させる
// (n<=1 はプロット中央寄せ)。
func xAtIndex(i, n int) float64 {
	if n <= 1 {
		return plotLeft + plotWidth/2.0
	}
	return plotLeft + float64(i)/float64(n-1)*plotWidth
}

// HoverPoint は折れ線1点のホバー用メタデータ。X,Y は SVG (viewBox 720x240) 座標系の頂点位置、
// Label は X 軸の時刻文字列 ("HH:MM"/"M/D HH:MM")、Value は実測値 (温度℃ または 湿度%)。
// JSON タグは DOM 埋め込み payload を小さくするため短縮名。
type HoverPoint struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Label string  `json:"t"`
	Value float64 `json:"v"`
}

// LineChartHoverPoints は単一系列 (生データ折れ線) の各点について、SVG 上の頂点座標
// (LineChartSVG と同一スケール計算)・時刻ラベル・実測値を返す。クライアント側のホバー
// (十字ポインター + 値/時刻ツールチップ) が SVG の折れ線頂点と一致した位置を出すために使う。
// 系列が1本でない/点が無い場合は nil を返す (ホバー無効。30d の日次2系列や空データが該当)。
func LineChartHoverPoints(series []Series) []HoverPoint {
	if len(series) != 1 || len(series[0].Points) == 0 {
		return nil
	}
	pts := series[0].Points
	yMin, yMax := dataRange(series)
	scaleMin, scaleMax := paddedScale(yMin, yMax)
	n := len(pts)

	out := make([]HoverPoint, 0, n)
	for i, p := range pts {
		y := plotTop + (scaleMax-p.Y)/(scaleMax-scaleMin)*plotHeight
		out = append(out, HoverPoint{
			X:     round1(xAtIndex(i, n)),
			Y:     round1(y),
			Label: p.Label,
			Value: p.Y,
		})
	}
	return out
}

// round1 は座標を小数1桁へ丸める (DOM 埋め込み JSON の冗長な桁を抑える)。
func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
