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

	// 欠測ギャップ可視化（data-quality-meta・末尾非破壊追加）。
	// いずれも nil/空のときは完全に従来挙動（後方互換の不変条件）であり、
	// 設定時のみ series[0] を欠測スロット nil で分断し連続欠測区間を markArea でハイライトする。
	RawNullable []*float64 // 欠測スロット nil を含む series[0] 拡張データ（非 nil 時は Raw に優先・nil 要素は ECharts null）
	GapBands    []GapBand  // 連続欠測区間（xAxis インデックス範囲）。markArea ハイライト対象
}

// GapBand は連続欠測区間を xAxis（カテゴリ）インデックスの範囲 [StartIdx, EndIdx] で表す。
// 欠測ギャップ markArea の帯1本に対応する（handler が拡張グリッドのインデックス空間で組む）。
type GapBand struct {
	StartIdx int // 欠測帯の開始 xAxis インデックス
	EndIdx   int // 欠測帯の終了 xAxis インデックス
}

// VPDChartSpec は VPD 専用 ECharts option ビルダー（VPDChartOptionJSON）への入力契約。
// 温湿度用 ChartSpec とは別型に隔離し、line option（ChartOptionJSON）の無回帰を守る（P2 無回帰）。
//
// 各スライスは Labels と同じ並び・同じ長さを前提とする（handler が pgconv/stats で整形して渡す）。
type VPDChartSpec struct {
	Labels []string  // X 軸カテゴリ（温湿度グラフと共通の時刻ラベル列）
	Color  string    // VPD 実測線・SMA の基準色（--color-vpd）
	VPD    []float64 // VPD 実測系列（series[0]・必須）
	SMA    []float64 // VPD 移動平均（nil/空なら出さない＝既定オフ）
	Lower  float64   // 適正帯下限 [kPa]
	Upper  float64   // 適正帯上限 [kPa]
	YMax   float64   // y 軸上限（高VPD側の乾きすぎゾーン可視化のため handler が算出）
}

// DewpointChartSpec は露点専用 ECharts option ビルダー（DewpointChartOptionJSON）への入力契約。
// 温湿度用 ChartSpec・VPD 用 VPDChartSpec とは別型に隔離し、両者の無回帰を守る（P2/P3 無回帰）。
//
// series[0]=露点 Td 線（主役・寒色 DewColor）、series[1]=気温 T 重ね線（気温が露点に接近=結露しやすさを見せる）。
// Condensation は結露帯（spread ≤ しきい値の連続区間・湿り側）で、xAxis 範囲の寒色 markArea でハイライトする。
// y 軸は気温と露点が同℃レンジゆえ auto 範囲（VPD の YMax 算出は不要）。
// 各スライスは Labels と同じ並び・同じ長さを前提とする（handler が pgconv で整形して渡す）。
type DewpointChartSpec struct {
	Labels       []string  // X 軸カテゴリ（温湿度/VPD と共通の時刻ラベル列）
	DewColor     string    // 露点 Td 線の基準色（--color-dewpoint・寒色＝湿り側）
	Dewpoint     []float64 // 露点 Td 系列（series[0]・必須）
	Temperature  []float64 // 気温 T 重ね線（series[1]）
	Condensation []Run     // 結露帯（xAxis 範囲の寒色 markArea）。空なら帯なし
}
