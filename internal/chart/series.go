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

	// 日スケール移動平均（sma-window-select・末尾非破壊追加）。
	// nil/空のときは完全に従来挙動（後方互換の不変条件＝P2/P5 と同方式）であり、
	// 設定時のみ既存系列の後ろへ各日スケール SMA を凡例トグル（既定オフ）の細線として重ねる。
	// 既存の単数 SMA（点数窓・約1日まで）は温存し、本フィールドは「数日〜2週間」スケールを追加で張る。
	DaySMAs []DaySMASeries
}

// DaySMASeries は 1 本の日スケール単純移動平均（SMA）追加系列を表す。
// 既存の生実測線・点数窓 SMA・正常帯・乖離率の上へ重ねる凡例トグル系列で、既定オフ・細線・
// データ点マーカーなし・端ラベルなしで描画される（生実測線が主役・誤用防止に SMA のみ）。
//
// Values は handler 側で 2桁丸め済み・可視窓長（Labels と同じ並び・同じ長さ）で渡す。
// 不変条件: len(Values) == len(ChartSpec.Labels)（applyGapGrid 拡張後も維持）。
type DaySMASeries struct {
	Label  string    // 凡例ラベル・系列名（中立な「移動平均 N日」）。凡例トグルと legend.selected のキー
	Values []float64 // 平滑値（2桁丸め済み・Labels と同長・同並び）
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

// GDDChartSpec は GDD 累積曲線専用 ECharts option ビルダー（GDDChartOptionJSON）への入力契約。
// 温湿度/VPD/露点の各 Spec とは別型に隔離し、それらの無回帰を守る（P2/P3/P6 無回帰）。
//
// 他パネルとの決定的な違い: x 軸が時刻 category でなく**経過日数の value 軸**である。
// 予測到達日（データ範囲外の未来）を markLine の `xAxis: 数値` で表現するため value 軸が必須で、
// series[0] のデータは [経過日数, 累積GDD] の座標ペアで与える（Labels を持たない）。
// 期間セレクタ非連動ゆえ温湿度等の時刻ラベルとは無関係（定植日→現在の全期間）。
type GDDChartSpec struct {
	ElapsedDays []float64 // x: 各 present 日の経過日数（0 起点・gap は不連続）。Cumulative と同長
	Cumulative  []float64 // y: 累積 GDD（series[0]・単調非減少）。ElapsedDays と同長
	Color       string    // 累積線の基準色（--color-gdd・暖色）
	TargetGDD   float64   // 収穫目標 GDD（水平 markLine の y）
	ForecastDay float64   // 予測到達経過日（垂直 markLine／markPoint の x）。HasForecast=true のとき使用
	HasForecast bool      // false なら予測 markLine/markPoint を出さない（予測不能・到達済み）
}

// TrendChartSpec は統計分析ページ（長期トレンド・季節サマリ）専用 ECharts option ビルダー
// （TrendChartOptionJSON）への入力契約。温湿度/VPD/露点/GDD の各 Spec とは別型に隔離し、
// それらの無回帰を守る（P2/P3/P6/P7 無回帰・design D の別型隔離）。
//
// 系列構成（クラッタ回避のため主役はロールアップ平均線・オーバーレイは凡例で既定オフ）:
//   - 主役: ロールアップ平均（RollupAvg・series[0]・必須）。Sen トレンド線 markLine・有意区間 markArea を注入。
//   - min/max 帯: BandLower/BandUpper 指定時のみ（積み上げ area：透明ベース＋帯幅）。
//   - ブートストラップ CI 帯: CILower/CIUpper 指定時のみ（積み上げ area：透明ベース＋CI幅）。
//   - 平年比: Climatology 指定時のみ（独立線・破線）。年数不足時は nil。
//
// 各スライスは Labels と同じ並び・同じ長さを前提とする（handler が JST 整形・pgconv で渡す）。
type TrendChartSpec struct {
	Labels      []string  // X 軸カテゴリ（月次/年次ラベル "2024-01" 等・handler が JST 整形）
	Color       string    // 主役線・Sen 線・CI 帯の基準色（--color-trend）
	RollupAvg   []float64 // 主役: ロールアップ平均（series[0]・必須）
	BandLower   []float64 // min 帯の下限（任意・BandUpper と対で積み上げ area）
	BandUpper   []float64 // max 帯の上限（任意・BandLower と対）
	SenLine     []float64 // Sen トレンド線の値列（2点 or 全点・markLine の両端点に first/last を採用）
	CILower     []float64 // ブートストラップ CI 下限（任意・CIUpper と対で積み上げ area）
	CIUpper     []float64 // CI 上限（任意・CILower と対）
	Climatology []float64 // 平年比系列（任意・年数不足時 nil）
	Unit        string    // 主役系列の凡例名・単位（"℃"/"%"）

	// 有意区間（xAxis 範囲の markArea）。Mann-Kendall が有意な区間を強調する。
	// 非有意/未算出時は空（markArea を出さない・末尾非破壊追加で Run 型を VPD/露点と共有）。
	Significant []Run
}
