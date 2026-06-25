package chart

import "testing"

// TestLineChartHoverPoints_単一系列の頂点を返す は、生データ折れ線の各点が
// SVG 座標 (端は plotLeft/plotRight)・ラベル・値を保持し、Y が値の大小と逆向き
// (大きい値ほど上=小さい Y) になることを検証する。
func TestLineChartHoverPoints_単一系列の頂点を返す(t *testing.T) {
	series := []Series{{Points: []Point{
		{Label: "00:00", Y: 20.0},
		{Label: "12:00", Y: 25.5}, // 最大
		{Label: "23:00", Y: 18.0}, // 最小
	}}}

	pts := LineChartHoverPoints(series)
	if len(pts) != 3 {
		t.Fatalf("len=%d, want 3", len(pts))
	}
	// X 軸は端から端 (先頭=plotLeft 48 / 末尾=plotRight 704)
	if pts[0].X != plotLeft {
		t.Errorf("先頭 X=%v, want %v", pts[0].X, float64(plotLeft))
	}
	if pts[2].X != plotRight {
		t.Errorf("末尾 X=%v, want %v", pts[2].X, float64(plotRight))
	}
	// ラベル・値はそのまま保持
	if pts[1].Label != "12:00" || pts[1].Value != 25.5 {
		t.Errorf("中央点 = (%q,%v), want (12:00,25.5)", pts[1].Label, pts[1].Value)
	}
	// 最大値(25.5)の点は最小値(18.0)の点より上 (= Y が小さい)
	if !(pts[1].Y < pts[2].Y) {
		t.Errorf("最大値の点 Y=%v は最小値の点 Y=%v より小さい(上)はず", pts[1].Y, pts[2].Y)
	}
	// Y はプロット範囲 (16〜208) に収まる
	for _, p := range pts {
		if p.Y < plotTop || p.Y > plotBottom {
			t.Errorf("Y=%v がプロット範囲 [%d,%d] 外", p.Y, plotTop, plotBottom)
		}
	}
}

// TestLineChartHoverPoints_多系列や空はnil は、日次2系列・空・nil でホバー無効 (nil) を返すことを検証する。
func TestLineChartHoverPoints_多系列や空はnil(t *testing.T) {
	if LineChartHoverPoints(nil) != nil {
		t.Error("nil 系列は nil を返すべき")
	}
	if LineChartHoverPoints([]Series{}) != nil {
		t.Error("空スライスは nil を返すべき")
	}
	if got := LineChartHoverPoints([]Series{{Points: nil}}); got != nil {
		t.Errorf("点が空なら nil を返すべき: %v", got)
	}
	two := []Series{
		{Name: "最高", Points: []Point{{Label: "06-08", Y: 30}}},
		{Name: "最低", Points: []Point{{Label: "06-08", Y: 18}}},
	}
	if got := LineChartHoverPoints(two); got != nil {
		t.Errorf("2系列 (日次 max/min) はホバー無効 nil を返すべき: %v", got)
	}
}
