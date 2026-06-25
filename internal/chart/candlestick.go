package chart

import (
	"fmt"
	"math"
	"strings"
)

// Candle は 1 本のローソク足（OHLC）。Label は X 軸ラベル用文字列（"6/24 09:00"）、
// Open/High/Low/Close はその足の始値/高値/安値/終値（温度℃ または 湿度%）。
type Candle struct {
	Label string
	Open  float64
	High  float64
	Low   float64
	Close float64
}

// ローソク足の配色（投資チャート慣習に合わせ、上昇=水色 / 下落=ピンク）。
// 終値 >= 始値 を上昇（水色）、終値 < 始値 を下落（ピンク）とする。
const (
	candleUpColor   = "#4dabf7" // 上昇: 水色
	candleDownColor = "#f783ac" // 下落: ピンク
)

// CandlestickSVG はローソク足群から OHLC チャート SVG を生成する。
// 事前条件: unit は "℃"/"%"。事後条件: candles が空のとき空状態 SVG（emptyMessage 埋め込み）を返す。
// 各足は高値-安値のヒゲ（細線）と始値-終値の実体（矩形）で描き、終値>=始値 を水色、終値<始値 をピンクで塗る。
// 不変条件: 返り値は常に整形済み <svg …>…</svg> 文字列（外部入力を含まない安全な自前生成）。
func CandlestickSVG(title, unit string, candles []Candle) string {
	var b strings.Builder
	writeSVGOpen(&b, title)

	if len(candles) == 0 {
		writeEmptyState(&b)
		b.WriteString("</svg>")
		return b.String()
	}

	yMin, yMax := candleRange(candles)
	scaleMin, scaleMax := paddedScale(yMin, yMax)
	n := len(candles)

	yToPixel := func(y float64) float64 {
		return plotTop + (scaleMax-y)/(scaleMax-scaleMin)*plotHeight
	}
	// 足は等間隔のスロット中央に配置する（線グラフの端寄せと異なり、実体に幅があるため）。
	slot := plotWidth / float64(n)
	xCenter := func(i int) float64 {
		return plotLeft + slot*(float64(i)+0.5)
	}
	bodyW := candleBodyWidth(slot)

	writeYAxisLabels(&b, unit, yMin, yMax, yToPixel)
	writeCandleXAxisLabels(&b, candles, xCenter)
	writeCandles(&b, candles, xCenter, yToPixel, bodyW)
	writeCandleLegend(&b)

	b.WriteString("</svg>")
	return b.String()
}

// candleRange は全足の安値の最小・高値の最大を返す（Y スケールの素の範囲）。
func candleRange(candles []Candle) (min, max float64) {
	min, max = candles[0].Low, candles[0].High
	for _, c := range candles {
		if c.Low < min {
			min = c.Low
		}
		if c.High > max {
			max = c.High
		}
	}
	return min, max
}

// candleBodyWidth は足の間隔（slot）から実体の幅を決める（間隔の 6 割・1〜10px に制限）。
// 本数が多い（48時間=96本）ほど細くなり、潰れないよう下限 1px を設ける。
func candleBodyWidth(slot float64) float64 {
	w := slot * 0.6
	if w < 1 {
		w = 1
	}
	if w > 10 {
		w = 10
	}
	return w
}

// candleColor は足の方向から色を決める（終値>=始値=水色 / 終値<始値=ピンク）。
func candleColor(c Candle) string {
	if c.Close >= c.Open {
		return candleUpColor
	}
	return candleDownColor
}

// writeCandles は各足のヒゲ（高値-安値の縦線）と実体（始値-終値の矩形）を書き出す。
// 実体の高さは始値=終値（同値）でも視認できるよう最低 1px を確保する。
func writeCandles(b *strings.Builder, candles []Candle, xCenter func(int) float64, yToPixel func(float64) float64, bodyW float64) {
	half := bodyW / 2
	for i, c := range candles {
		x := xCenter(i)
		color := candleColor(c)

		// ヒゲ: 高値 → 安値の縦線
		fmt.Fprintf(b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1" />`,
			coord(x), coord(yToPixel(c.High)), coord(x), coord(yToPixel(c.Low)), color)

		// 実体: 始値-終値の矩形（上端=高い方・下端=低い方、高さ最低 1px）
		top := yToPixel(math.Max(c.Open, c.Close))
		bottom := yToPixel(math.Min(c.Open, c.Close))
		h := bottom - top
		if h < 1 {
			h = 1
		}
		fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s" />`,
			coord(x-half), coord(top), coord(bodyW), coord(h), color)
	}
}

// writeCandleXAxisLabels は下 X 軸に間引いた足ラベルを書き出す（線グラフと同じ間引き規則）。
func writeCandleXAxisLabels(b *strings.Builder, candles []Candle, xCenter func(int) float64) {
	for _, i := range labelIndices(len(candles)) {
		writeAxisText(b, xCenter(i), float64(svgHeight-10), "middle", candles[i].Label)
	}
}

// writeCandleLegend は配色の凡例（上昇=水色 / 下落=ピンク）を右上に書き出す。
func writeCandleLegend(b *strings.Builder) {
	items := []struct {
		color string
		label string
	}{
		{candleUpColor, "上昇"},
		{candleDownColor, "下落"},
	}
	for row, it := range items {
		y := plotTop + 6 + row*16
		x := plotRight - 72
		fmt.Fprintf(b, `<rect x="%d" y="%d" width="12" height="10" fill="%s" />`, x, y-5, it.color)
		fmt.Fprintf(b, `<text x="%d" y="%d" fill="%s" font-size="12" dominant-baseline="middle">%s</text>`,
			x+18, y, axisColor, esc(it.label))
	}
}
