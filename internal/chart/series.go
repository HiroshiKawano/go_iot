package chart

// ChartSpec は複数系列 ECharts option ビルダー（ChartOptionJSON）への型安全な入力契約。
// 生実測線（Raw）を主役の series[0] とし、SMA・正常帯（BandLower/BandWidth）・乖離率（Deviation）を
// 任意のオーバーレイ系列として重ねる。nil/空のオーバーレイは当該系列を出さない（防御的）。
//
// 各スライスは Labels と同じ並び・同じ長さを前提とする（handler が pgconv/stats で整形して渡す）。
type ChartSpec struct {
	Labels    []string   // X 軸カテゴリ（時刻ラベル列）
	Unit      string     // 生実測系列の凡例名・単位（"℃"/"%"）
	Color     string     // 生実測・SMA・乖離率の基準色
	Raw       []float64  // 生実測値（series[0]・必須）
	SMA       []float64  // 移動平均（nil/空なら系列を出さない）
	BandLower []float64  // 正常帯の下限 SMA-kσ（nil/空なら帯を出さない）
	BandWidth []float64  // 正常帯の帯幅 2kσ（BandLower と対で使う）
	Deviation []*float64 // 乖離率%（nil/空なら出さない・nil 要素は欠落点）
}
