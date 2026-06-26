package chart

// Point は 1 データ点。Label は X 軸ラベル用文字列（24h: "14:30" / 3d・7d・30d: "06-08"）、
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
