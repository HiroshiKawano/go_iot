package chart

import "math"

// 統計純関数層。device-show の温湿度グラフへ上載せする派生指標（移動平均・移動標準偏差・
// 正常帯・乖離率）と、数値カード／日次集計表のスカラ集計（平均・最大最小・日較差・σ・CV）を
// すべて []float64 入出力の純関数として提供する（統計の単一源）。
//
// 本ファイルは最下流の純粋層であり gin/DB/templ/pgtype/time に依存しない。
// 時刻・タイムゾーン・pgtype 変換は handler 境界に留め、ここには持ち込まない。

// SMA は窓幅 window の単純移動平均を返す。
// 先頭 window-1 点は「その時点までの可用点の expanding window」で部分平均を取り、欠落を作らない。
// 事前条件: window >= 1（満たさない場合は 1 として扱う）。事後条件: len(out)==len(values)。
func SMA(values []float64, window int) []float64 {
	if window < 1 {
		window = 1
	}
	out := make([]float64, len(values))
	for i := range values {
		out[i] = mean(windowSlice(values, i, window))
	}
	return out
}

// MovingStdDev は窓幅 window の母標準偏差（N 除算）を返す。
// 先頭は SMA と同じ expanding window の部分σ。単一点は 0。事後条件: len(out)==len(values)。
func MovingStdDev(values []float64, window int) []float64 {
	if window < 1 {
		window = 1
	}
	out := make([]float64, len(values))
	for i := range values {
		out[i] = popStdDev(windowSlice(values, i, window))
	}
	return out
}

// Band は正常帯（SMA±kσ）の下限(sma-k*sigma)と帯幅(2*k*sigma)を返す（積み上げ area 用の2系列）。
// 事前条件: len(sma)==len(sigma)、k>=0。sigma が短い場合も panic せず不足分を 0 とみなす。
func Band(sma, sigma []float64, k float64) (lower, width []float64) {
	lower = make([]float64, len(sma))
	width = make([]float64, len(sma))
	for i := range sma {
		s := 0.0
		if i < len(sigma) {
			s = sigma[i]
		}
		lower[i] = sma[i] - k*s
		width[i] = 2 * k * s
	}
	return lower, width
}

// Deviation は各点の移動平均からの乖離率(%) = (実測値-SMA)/SMA*100 を返す。
// |sma|<epsilon の点はゼロ除算を避けて nil（未定義）とする。事後条件: len(out)==len(values)。
func Deviation(values, sma []float64, epsilon float64) []*float64 {
	out := make([]*float64, len(values))
	for i := range values {
		if i >= len(sma) || math.Abs(sma[i]) < epsilon {
			out[i] = nil
			continue
		}
		d := (values[i] - sma[i]) / sma[i] * 100
		out[i] = &d
	}
	return out
}

// Mean は算術平均を返す。空入力は 0。
func Mean(values []float64) float64 {
	return mean(values)
}

// MinMax は最小値・最大値を返す。空入力は (0, 0)。
func MinMax(values []float64) (min, max float64) {
	if len(values) == 0 {
		return 0, 0
	}
	min, max = values[0], values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}

// DiurnalRange は日較差（最大-最小）を返す。空入力は 0。
func DiurnalRange(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min, max := MinMax(values)
	return max - min
}

// StdDev は系列全体の母標準偏差（N 除算）を返す。空入力・単一点は 0。
func StdDev(values []float64) float64 {
	return popStdDev(values)
}

// CV は変動係数 σ/μ を返す。|mean|<epsilon は未定義として (0, false) を返す。
func CV(values []float64, epsilon float64) (cv float64, ok bool) {
	m := Mean(values)
	if math.Abs(m) < epsilon {
		return 0, false
	}
	return StdDev(values) / m, true
}

// LinearFit は (xs, ys) に最小二乗で直線 y = slope·x + intercept を当てはめ、傾き・切片を返す。
// GDD の到達日外挿（gdd.go::ForecastDaysToTarget）専用でなく回帰の汎用純関数として置く（将来の trend 検出が再利用可）。
//
//	slope     = Σ(xᵢ−x̄)(yᵢ−ȳ) / Σ(xᵢ−x̄)²
//	intercept = ȳ − slope·x̄
//
// ok=false: len<2（点が足りない）／ Σ(xᵢ−x̄)²=0（x の分散0＝全点同一 x で傾き定義不能）。
// このとき slope/intercept は 0 を返す（呼び出し側は ok で分岐する）。
// 長さ不一致は短い方に合わせて防御的に扱う（純粋層の入力非破壊・math のみ依存）。
func LinearFit(xs, ys []float64) (slope, intercept float64, ok bool) {
	n := len(xs)
	if len(ys) < n {
		n = len(ys)
	}
	if n < 2 {
		return 0, 0, false
	}
	var sx, sy float64
	for i := 0; i < n; i++ {
		sx += xs[i]
		sy += ys[i]
	}
	mx := sx / float64(n)
	my := sy / float64(n)

	var sxx, sxy float64
	for i := 0; i < n; i++ {
		dx := xs[i] - mx
		sxx += dx * dx
		sxy += dx * (ys[i] - my)
	}
	if sxx == 0 {
		return 0, 0, false // x の分散0（全点同一 x）
	}
	slope = sxy / sxx
	intercept = my - slope*mx
	return slope, intercept, true
}

// --- 内部ヘルパ（パッケージ非公開・純粋） ---

// windowSlice は index i を末尾とする窓幅 window の部分スライスを返す。
// 先頭側は start を 0 でクランプするため、先頭区間は自然に expanding window となる。
func windowSlice(values []float64, i, window int) []float64 {
	start := i - window + 1
	if start < 0 {
		start = 0
	}
	return values[start : i+1]
}

// mean は算術平均。空は 0。
func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// popStdDev は母標準偏差（N 除算）。空・単一点は 0。
func popStdDev(xs []float64) float64 {
	n := len(xs)
	if n == 0 {
		return 0
	}
	m := mean(xs)
	var sq float64
	for _, x := range xs {
		d := x - m
		sq += d * d
	}
	return math.Sqrt(sq / float64(n))
}
