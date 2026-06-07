package chart

import (
	"strings"
	"testing"
)

// sampleSeries は単一系列テスト用の固定点列を返す（決定的テストのため値を固定）。
// 最大 25.5 / 最小 18.0 で、Y 軸ラベルの期待値が一意に定まるよう構成する。
func sampleSeries() []Series {
	return []Series{{
		Name: "",
		Points: []Point{
			{Label: "00:00", Y: 20.0},
			{Label: "12:00", Y: 25.5},
			{Label: "23:00", Y: 18.0},
		},
	}}
}

// TestLineChartSVG_Empty は有効点が 0 件のとき空状態 SVG を返すことを検証する（R4.5）。
// 「データはまだありません」を含み、折れ線（<polyline>）を一切含まないこと。
func TestLineChartSVG_Empty(t *testing.T) {
	tests := []struct {
		name   string
		series []Series
	}{
		{"nil 系列", nil},
		{"空スライス", []Series{}},
		{"系列はあるが点が空", []Series{{Name: "", Points: nil}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svg := LineChartSVG("温度", "℃", tt.series)

			if !strings.Contains(svg, "データはまだありません") {
				t.Errorf("空状態メッセージが含まれない: %q", svg)
			}
			if strings.Contains(svg, "<polyline") {
				t.Errorf("空状態で <polyline> を含んではいけない: %q", svg)
			}
			// 空でも妥当な SVG 要素であること（viewBox を持つ）
			if !strings.Contains(svg, `viewBox="0 0 720 240"`) {
				t.Errorf("viewBox が含まれない: %q", svg)
			}
		})
	}
}

// TestLineChartSVG_SingleSeries は単一系列の線グラフ生成を検証する（R4.1, R4.2, R4.4）。
func TestLineChartSVG_SingleSeries(t *testing.T) {
	svg := LineChartSVG("温度", "℃", sampleSeries())

	// SVG ルートと寸法（viewBox 720x240）
	if !strings.HasPrefix(svg, "<svg") {
		t.Errorf("SVG ルート要素で始まっていない: %q", svg)
	}
	if !strings.Contains(svg, `viewBox="0 0 720 240"`) {
		t.Errorf("viewBox が含まれない: %q", svg)
	}

	// 折れ線はちょうど 1 本
	if got := strings.Count(svg, "<polyline"); got != 1 {
		t.Errorf("<polyline> の本数 = %d, want 1\n%s", got, svg)
	}

	// 空状態メッセージは含まない
	if strings.Contains(svg, "データはまだありません") {
		t.Errorf("有効点があるのに空状態メッセージが含まれる: %q", svg)
	}

	// Y 軸 min/max ラベル（unit 付き）
	if !strings.Contains(svg, "25.5℃") {
		t.Errorf("Y 軸の最大ラベル 25.5℃ が含まれない: %q", svg)
	}
	if !strings.Contains(svg, "18.0℃") {
		t.Errorf("Y 軸の最小ラベル 18.0℃ が含まれない: %q", svg)
	}

	// X 軸ラベル（各点の Label）
	for _, want := range []string{"00:00", "12:00", "23:00"} {
		if !strings.Contains(svg, want) {
			t.Errorf("X 軸ラベル %q が含まれない: %q", want, svg)
		}
	}
}

// TestLineChartSVG_ColorByUnit は unit に応じた線色を検証する（design 視覚仕様）。
// 温度(℃)=#e8590c / 湿度(%)=#1971c2。
func TestLineChartSVG_ColorByUnit(t *testing.T) {
	tests := []struct {
		name      string
		unit      string
		wantColor string
		notColor  string
	}{
		{"温度は暖色", "℃", "#e8590c", "#1971c2"},
		{"湿度は寒色", "%", "#1971c2", "#e8590c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svg := LineChartSVG("グラフ", tt.unit, sampleSeries())
			if !strings.Contains(svg, tt.wantColor) {
				t.Errorf("線色 %q が含まれない: %q", tt.wantColor, svg)
			}
			if strings.Contains(svg, tt.notColor) {
				t.Errorf("他系統の線色 %q が含まれてはいけない: %q", tt.notColor, svg)
			}
		})
	}
}

// countDashedPolylines は <polyline> のうち stroke-dasharray を持つ本数を数える。
// 凡例の <line> 等が stroke-dasharray を持っても影響されないよう polyline タグに限定する。
func countDashedPolylines(svg string) int {
	cnt := 0
	for _, seg := range strings.Split(svg, "<polyline")[1:] {
		if end := strings.Index(seg, ">"); end >= 0 {
			seg = seg[:end]
		}
		if strings.Contains(seg, "stroke-dasharray") {
			cnt++
		}
	}
	return cnt
}

// TestLineChartSVG_TwoSeries は日次 最大/最小の 2 系列描画を検証する（R4.3）。
// 最大=実線・最小=破線（stroke-dasharray）の 2 本と、凡例（系列名）を含むこと。
func TestLineChartSVG_TwoSeries(t *testing.T) {
	series := []Series{
		{Name: "最高", Dashed: false, Points: []Point{
			{Label: "06-06", Y: 30.0}, {Label: "06-07", Y: 31.0}, {Label: "06-08", Y: 29.0},
		}},
		{Name: "最低", Dashed: true, Points: []Point{
			{Label: "06-06", Y: 18.0}, {Label: "06-07", Y: 19.0}, {Label: "06-08", Y: 17.0},
		}},
	}
	svg := LineChartSVG("温度", "℃", series)

	// 折れ線は 2 本、うち破線はちょうど 1 本
	if got := strings.Count(svg, "<polyline"); got != 2 {
		t.Errorf("<polyline> の本数 = %d, want 2\n%s", got, svg)
	}
	if got := countDashedPolylines(svg); got != 1 {
		t.Errorf("破線 <polyline> の本数 = %d, want 1\n%s", got, svg)
	}

	// 凡例に両系列名が含まれる
	for _, want := range []string{"最高", "最低"} {
		if !strings.Contains(svg, want) {
			t.Errorf("凡例 %q が含まれない: %q", want, svg)
		}
	}

	// Y 軸は全系列の最小・最大（17.0〜31.0）
	if !strings.Contains(svg, "31.0℃") {
		t.Errorf("Y 軸の最大ラベル 31.0℃ が含まれない: %q", svg)
	}
	if !strings.Contains(svg, "17.0℃") {
		t.Errorf("Y 軸の最小ラベル 17.0℃ が含まれない: %q", svg)
	}

	// X 軸ラベル（日付）
	for _, want := range []string{"06-06", "06-07", "06-08"} {
		if !strings.Contains(svg, want) {
			t.Errorf("X 軸ラベル %q が含まれない: %q", want, svg)
		}
	}
}

// TestLineChartSVG_ManyPointsDownsampleXLabels は点数が多い（30日）場合に
// X 軸ラベルが間引かれ、先頭と末尾は必ず表示されることを検証する。
func TestLineChartSVG_ManyPointsDownsampleXLabels(t *testing.T) {
	pts := make([]Point, 30)
	for i := range pts {
		// "d00".."d29" の一意ラベル（部分一致衝突を避ける）
		pts[i] = Point{Label: "d" + twoDigits(i), Y: float64(20 + i%5)}
	}
	svg := LineChartSVG("温度", "℃", []Series{{Points: pts}})

	// 先頭・末尾は必ず表示
	for _, want := range []string{"d00", "d29"} {
		if !strings.Contains(svg, want) {
			t.Errorf("端のラベル %q が含まれない", want)
		}
	}
	// 間引かれて表示されないラベルがある（全件は出さない）
	if strings.Contains(svg, "d01") {
		t.Errorf("間引き対象 d01 が表示されている（過密）: %q", svg)
	}
	// 折れ線は 1 本（点が多くても 1 系列）
	if got := strings.Count(svg, "<polyline"); got != 1 {
		t.Errorf("<polyline> の本数 = %d, want 1", got)
	}
}

// TestLineChartSVG_FlatSeries は全点が同値（平坦）でもゼロ除算せず描画できることを検証する。
func TestLineChartSVG_FlatSeries(t *testing.T) {
	svg := LineChartSVG("温度", "℃", []Series{{Points: []Point{
		{Label: "00:00", Y: 22.0}, {Label: "01:00", Y: 22.0}, {Label: "02:00", Y: 22.0},
	}}})

	if strings.Contains(svg, "データはまだありません") {
		t.Errorf("平坦でも有効点があるので空状態にしてはいけない: %q", svg)
	}
	if got := strings.Count(svg, "<polyline"); got != 1 {
		t.Errorf("<polyline> の本数 = %d, want 1", got)
	}
	if !strings.Contains(svg, "22.0℃") {
		t.Errorf("Y 軸ラベル 22.0℃ が含まれない: %q", svg)
	}
}

// TestLineChartSVG_SinglePoint は点が 1 つだけでも描画でき、折れ線が 1 本出ることを検証する。
func TestLineChartSVG_SinglePoint(t *testing.T) {
	svg := LineChartSVG("温度", "℃", []Series{{Points: []Point{{Label: "14:00", Y: 25.0}}}})

	if got := strings.Count(svg, "<polyline"); got != 1 {
		t.Errorf("<polyline> の本数 = %d, want 1", got)
	}
	if !strings.Contains(svg, "14:00") {
		t.Errorf("X 軸ラベル 14:00 が含まれない: %q", svg)
	}
	if !strings.Contains(svg, "25.0℃") {
		t.Errorf("Y 軸ラベル 25.0℃ が含まれない: %q", svg)
	}
}

// TestLineChartSVG_SkipsEmptySeries は点が空の系列を描画・凡例から除外することを検証する。
func TestLineChartSVG_SkipsEmptySeries(t *testing.T) {
	svg := LineChartSVG("温度", "℃", []Series{
		{Name: "最高", Points: nil}, // 点が無いので描画も凡例もしない
		{Name: "最低", Dashed: true, Points: []Point{
			{Label: "06-07", Y: 18.0}, {Label: "06-08", Y: 17.0},
		}},
	})

	if got := strings.Count(svg, "<polyline"); got != 1 {
		t.Errorf("<polyline> の本数 = %d, want 1", got)
	}
	if !strings.Contains(svg, "最低") {
		t.Errorf("点のある系列の凡例 最低 が含まれない: %q", svg)
	}
	if strings.Contains(svg, "最高") {
		t.Errorf("点が空の系列 最高 は凡例に出してはいけない: %q", svg)
	}
}

// twoDigits は 0〜99 を 2 桁ゼロ埋め文字列にする（テストのラベル生成用）。
func twoDigits(n int) string {
	const d = "0123456789"
	return string([]byte{d[(n/10)%10], d[n%10]})
}
