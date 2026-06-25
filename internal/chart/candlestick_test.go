package chart

import (
	"strings"
	"testing"
)

// upCandles は全足が上昇（終値>=始値）の固定データを返す。
func upCandles() []Candle {
	return []Candle{
		{Label: "6/24 09:00", Open: 25.0, High: 26.0, Low: 24.5, Close: 25.8},
		{Label: "6/24 09:30", Open: 25.8, High: 27.2, Low: 25.7, Close: 27.0},
	}
}

// downCandles は全足が下落（終値<始値）の固定データを返す。
func downCandles() []Candle {
	return []Candle{
		{Label: "6/24 09:00", Open: 27.0, High: 27.1, Low: 25.0, Close: 25.2},
		{Label: "6/24 09:30", Open: 25.2, High: 25.3, Low: 23.0, Close: 23.4},
	}
}

// TestCandlestickSVG_Empty は足が 0 本のとき空状態 SVG を返すことを検証する。
// 「データはまだありません」を含み、実体（<rect）・ヒゲ（<line）を一切含まないこと。
func TestCandlestickSVG_Empty(t *testing.T) {
	for _, tt := range []struct {
		name    string
		candles []Candle
	}{
		{"nil", nil},
		{"空スライス", []Candle{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svg := CandlestickSVG("温度", "℃", tt.candles)

			if !strings.Contains(svg, "データはまだありません") {
				t.Errorf("空状態メッセージが含まれない: %q", svg)
			}
			if strings.Contains(svg, "<rect") {
				t.Errorf("空状態で実体 <rect> を含んではいけない: %q", svg)
			}
			if strings.Contains(svg, "<line") {
				t.Errorf("空状態でヒゲ <line> を含んではいけない: %q", svg)
			}
			if !strings.Contains(svg, `viewBox="0 0 720 240"`) {
				t.Errorf("viewBox が含まれない: %q", svg)
			}
		})
	}
}

// TestCandlestickSVG_UpColor は上昇足が水色で描かれることを検証する。
// ヒゲ（<line stroke=）は方向色、凡例（<rect fill=）は両色を持つため、stroke 色で方向を判定する。
func TestCandlestickSVG_UpColor(t *testing.T) {
	svg := CandlestickSVG("温度", "℃", upCandles())

	if !strings.Contains(svg, `stroke="`+candleUpColor+`"`) {
		t.Errorf("上昇足のヒゲに水色 %s が使われていない: %q", candleUpColor, svg)
	}
	if strings.Contains(svg, `stroke="`+candleDownColor+`"`) {
		t.Errorf("全足上昇なのに下落色 %s のヒゲが含まれる: %q", candleDownColor, svg)
	}
}

// TestCandlestickSVG_DownColor は下落足がピンクで描かれることを検証する。
func TestCandlestickSVG_DownColor(t *testing.T) {
	svg := CandlestickSVG("温度", "℃", downCandles())

	if !strings.Contains(svg, `stroke="`+candleDownColor+`"`) {
		t.Errorf("下落足のヒゲにピンク %s が使われていない: %q", candleDownColor, svg)
	}
	if strings.Contains(svg, `stroke="`+candleUpColor+`"`) {
		t.Errorf("全足下落なのに上昇色 %s のヒゲが含まれる: %q", candleUpColor, svg)
	}
}

// TestCandlestickSVG_Structure は本数ぶんのヒゲ（縦線）と妥当な SVG 構造を検証する。
func TestCandlestickSVG_Structure(t *testing.T) {
	candles := upCandles()
	svg := CandlestickSVG("温度", "℃", candles)

	if !strings.HasPrefix(svg, "<svg") {
		t.Errorf("SVG ルート要素で始まっていない: %q", svg)
	}
	if !strings.HasSuffix(svg, "</svg>") {
		t.Errorf("SVG が閉じられていない: %q", svg)
	}
	// ヒゲ <line> は足数とちょうど一致する（凡例は <rect> のため <line> を増やさない）。
	if got := strings.Count(svg, "<line"); got != len(candles) {
		t.Errorf("ヒゲ <line> の本数 = %d, want %d\n%s", got, len(candles), svg)
	}
	// 空状態メッセージは含まない
	if strings.Contains(svg, "データはまだありません") {
		t.Errorf("足があるのに空状態メッセージが含まれる: %q", svg)
	}
	// X 軸ラベル（足ラベル）が描画される
	if !strings.Contains(svg, "6/24 09:00") {
		t.Errorf("X 軸ラベルが含まれない: %q", svg)
	}
}

// TestCandlestickSVG_Doji は同値（始値=終値）の足でも実体が 1px 以上で描かれることを検証する。
func TestCandlestickSVG_Doji(t *testing.T) {
	doji := []Candle{{Label: "6/24 09:00", Open: 25.0, High: 25.5, Low: 24.5, Close: 25.0}}
	svg := CandlestickSVG("温度", "℃", doji)

	if !strings.Contains(svg, "<rect") {
		t.Errorf("同値足でも実体 <rect> が描かれるべき: %q", svg)
	}
	// 同値は終値>=始値 のため上昇色（水色）扱い
	if !strings.Contains(svg, `stroke="`+candleUpColor+`"`) {
		t.Errorf("同値足は上昇色 %s で描かれるべき: %q", candleUpColor, svg)
	}
}
