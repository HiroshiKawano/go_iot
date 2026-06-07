// Package chart は計測点列から温度/湿度の線グラフ SVG 文字列を生成する純粋ユーティリティ。
//
// stdlib（strings.Builder / fmt / strconv）のみに依存し、gin・DB・templ・pgtype を
// import しない（structure.md 最下流ユーティリティ）。入力は整形済みの float 点列で、
// pgtype 変換（pgconv.NumericToFloat 等）は handler 側の責務とする。出力は handler が
// templ の @templ.Raw で埋め込むことを前提とした安全な自前生成文字列。
package chart

import (
	"fmt"
	"strconv"
	"strings"
)

// Point は 1 データ点。Label は X 軸ラベル用文字列（24h: "14:30" / 7d・30d: "06-08"）、
// Y は数値（温度℃ または 湿度%）。
type Point struct {
	Label string
	Y     float64
}

// Series は 1 本の折れ線。Name は凡例名（単一系列は "" で凡例省略）、
// Dashed は破線指定（日次の最小系列など）、Points は点列。
type Series struct {
	Name   string
	Dashed bool
	Points []Point
}

// SVG 寸法と視覚仕様（design「SVG 視覚仕様（確定）」に準拠）。
const (
	svgWidth     = 720
	svgHeight    = 240
	marginLeft   = 48 // 左 Y 軸ラベル用余白
	marginRight  = 16
	marginTop    = 16
	marginBottom = 32 // 下 X 軸ラベル用余白

	tempColor     = "#e8590c" // 温度=暖色
	humidityColor = "#1971c2" // 湿度=寒色
	axisColor     = "#868e96" // 軸ラベル・空状態文言
	fontFamily    = "system-ui, sans-serif"

	emptyMessage = "データはまだありません"
	maxXLabels   = 6 // X 軸ラベルの最大表示数（過密回避）
)

// プロット領域（軸ラベル用の余白を除いた描画矩形）。
const (
	plotLeft   = marginLeft               // 48
	plotRight  = svgWidth - marginRight   // 704
	plotTop    = marginTop                // 16
	plotBottom = svgHeight - marginBottom // 208
	plotWidth  = plotRight - plotLeft     // 656
	plotHeight = plotBottom - plotTop     // 192
)

// LineChartSVG は系列群から線グラフ SVG を生成する。
// 事前条件: series は 1〜2 本。unit は "℃"/"%"。
// 事後条件: 有効点が 0 のとき空状態 SVG（emptyMessage 埋め込み）を返す。
// 不変条件: 返り値は常に整形済み <svg …>…</svg> 文字列（外部入力を含まない安全な自前生成）。
func LineChartSVG(title, unit string, series []Series) string {
	var b strings.Builder
	writeSVGOpen(&b, title)

	if totalPoints(series) == 0 {
		writeEmptyState(&b)
		b.WriteString("</svg>")
		return b.String()
	}

	yMin, yMax := dataRange(series)
	scaleMin, scaleMax := paddedScale(yMin, yMax)
	n := maxLen(series)

	yToPixel := func(y float64) float64 {
		return plotTop + (scaleMax-y)/(scaleMax-scaleMin)*plotHeight
	}
	xAt := func(i int) float64 {
		if n <= 1 {
			return plotLeft + plotWidth/2.0
		}
		return plotLeft + float64(i)/float64(n-1)*plotWidth
	}

	writeYAxisLabels(&b, unit, yMin, yMax, yToPixel)
	writeXAxisLabels(&b, labelSource(series), xAt)
	writePolylines(&b, unit, series, xAt, yToPixel)
	writeLegend(&b, unit, series)

	b.WriteString("</svg>")
	return b.String()
}

// writeSVGOpen は SVG ルート要素と a11y 用 <title> を書き出す。
func writeSVGOpen(b *strings.Builder, title string) {
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" role="img" font-family="%s">`,
		svgWidth, svgHeight, fontFamily)
	fmt.Fprintf(b, `<title>%s</title>`, esc(title))
}

// writeEmptyState は中央に空状態メッセージを書き出す（折れ線は描かない）。
func writeEmptyState(b *strings.Builder) {
	fmt.Fprintf(b,
		`<text x="%d" y="%d" text-anchor="middle" dominant-baseline="middle" fill="%s" font-size="14">%s</text>`,
		svgWidth/2, svgHeight/2, axisColor, emptyMessage)
}

// writeYAxisLabels は左 Y 軸に最大値（上）・最小値（下）のラベルを unit 付きで書き出す。
func writeYAxisLabels(b *strings.Builder, unit string, yMin, yMax float64, yToPixel func(float64) float64) {
	writeAxisText(b, plotLeft-6, yToPixel(yMax), "end", formatNum(yMax)+unit)
	writeAxisText(b, plotLeft-6, yToPixel(yMin), "end", formatNum(yMin)+unit)
}

// writeXAxisLabels は下 X 軸に間引いた点ラベルを書き出す。
func writeXAxisLabels(b *strings.Builder, points []Point, xAt func(int) float64) {
	for _, i := range labelIndices(len(points)) {
		writeAxisText(b, xAt(i), float64(svgHeight-10), "middle", points[i].Label)
	}
}

// writeAxisText は軸ラベル 1 個分の <text> を書き出す（色・フォントサイズは軸共通）。
func writeAxisText(b *strings.Builder, x, y float64, anchor, text string) {
	fmt.Fprintf(b,
		`<text x="%s" y="%s" text-anchor="%s" fill="%s" font-size="12">%s</text>`,
		coord(x), coord(y), anchor, axisColor, esc(text))
}

// writePolylines は各系列の折れ線を書き出す。
// 同一 unit の色を共有し、Dashed 系列（日次の最小など）は破線（stroke-dasharray）にする。
func writePolylines(b *strings.Builder, unit string, series []Series, xAt func(int) float64, yToPixel func(float64) float64) {
	color := lineColor(unit)
	for _, s := range series {
		if len(s.Points) == 0 {
			continue
		}
		var pts strings.Builder
		for i, p := range s.Points {
			if i > 0 {
				pts.WriteByte(' ')
			}
			fmt.Fprintf(&pts, "%s,%s", coord(xAt(i)), coord(yToPixel(p.Y)))
		}
		fmt.Fprintf(b, `<polyline fill="none" stroke="%s" stroke-width="2"%s points="%s" />`,
			color, dashAttr(s.Dashed), pts.String())
	}
}

// writeLegend は名前付き系列の凡例（線サンプル + 系列名）を右上に書き出す。
// 系列名が空（単一系列）の場合は凡例を省略する。
func writeLegend(b *strings.Builder, unit string, series []Series) {
	color := lineColor(unit)
	row := 0
	for _, s := range series {
		if s.Name == "" || len(s.Points) == 0 {
			continue
		}
		y := plotTop + 6 + row*16
		x1 := plotRight - 88
		x2 := x1 + 22
		fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="2"%s />`,
			x1, y, x2, y, color, dashAttr(s.Dashed))
		fmt.Fprintf(b, `<text x="%d" y="%d" fill="%s" font-size="12" dominant-baseline="middle">%s</text>`,
			x2+6, y, axisColor, esc(s.Name))
		row++
	}
}

// dashAttr は破線指定時に stroke-dasharray 属性（前置スペース付き）を返す。
func dashAttr(dashed bool) string {
	if dashed {
		return ` stroke-dasharray="6 4"`
	}
	return ""
}

// lineColor は unit から線色を決める（℃=温度暖色 / それ以外=湿度寒色）。
func lineColor(unit string) string {
	if unit == "℃" {
		return tempColor
	}
	return humidityColor
}

// totalPoints は全系列の点数の合計を返す（0 なら空状態）。
func totalPoints(series []Series) int {
	n := 0
	for _, s := range series {
		n += len(s.Points)
	}
	return n
}

// dataRange は全系列の Y の最小・最大を返す（呼び出し側で点数>0 を保証）。
func dataRange(series []Series) (min, max float64) {
	first := true
	for _, s := range series {
		for _, p := range s.Points {
			if first {
				min, max, first = p.Y, p.Y, false
				continue
			}
			if p.Y < min {
				min = p.Y
			}
			if p.Y > max {
				max = p.Y
			}
		}
	}
	return min, max
}

// paddedScale は描画用に上下へ余白を付けたスケール範囲を返す。
// 平坦（min==max）な系列でもゼロ除算を避けるため最低でも ±1 の幅を確保する。
func paddedScale(yMin, yMax float64) (scaleMin, scaleMax float64) {
	pad := (yMax - yMin) * 0.1
	if pad == 0 {
		pad = 1
	}
	return yMin - pad, yMax + pad
}

// maxLen は系列中で最も点数が多い系列の点数を返す（X 座標の分母に使う）。
func maxLen(series []Series) int {
	m := 0
	for _, s := range series {
		if len(s.Points) > m {
			m = len(s.Points)
		}
	}
	return m
}

// labelSource は X 軸ラベルの取得元として最も点数が多い系列の点列を返す。
func labelSource(series []Series) []Point {
	var src []Point
	for _, s := range series {
		if len(s.Points) > len(src) {
			src = s.Points
		}
	}
	return src
}

// labelIndices は n 点のうち X 軸に表示するインデックスを間引いて返す
// （maxXLabels 以下ならすべて、超える場合は等間隔 + 末尾を必ず含める）。
func labelIndices(n int) []int {
	if n <= 0 {
		return nil
	}
	if n <= maxXLabels {
		idx := make([]int, n)
		for i := range idx {
			idx[i] = i
		}
		return idx
	}
	step := (n + maxXLabels - 1) / maxXLabels
	var idx []int
	for i := 0; i < n; i += step {
		idx = append(idx, i)
	}
	if idx[len(idx)-1] != n-1 {
		idx = append(idx, n-1)
	}
	return idx
}

// formatNum は軸ラベル用に float を小数 1 桁の文字列へ整形する。
func formatNum(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// coord は座標値を小数 1 桁へ整形する（SVG 属性の冗長な桁を抑える）。
func coord(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// esc は XML テキストノード用の最小エスケープ（外部入力は想定しないが防御的に実施）。
func esc(s string) string {
	return xmlEscaper.Replace(s)
}

var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
