package chart

import (
	"math"
	"math/rand/v2"
	"sort"
)

// トレンド検定の純粋層（統計分析ページ／長期トレンド・季節サマリ）。
//
// 自己相関が極めて強い温湿度（ラグ1≈0.9+）で偽トレンドを避けるため、自己相関補正つき
// Mann-Kendall ＋ Sen の傾きを主役に、厳密判定（Hamed-Rao 補正・ブロックブートストラップ
// 信頼区間・多重比較補正）まで Go 単体で算出する（外部プロセス R/Python に依存しない）。
//
// 本ファイルは最下流の純粋層であり gin/DB/templ/pgtype/time に依存しない（math と math/rand/v2 のみ）。
// 月次バケット・JST 境界・単位/年への換算は handler 境界に置き、ここには等間隔系列 []float64 が渡る。
// 入力スライスは破壊しない（イミュータブル）。O(N²) 検定はロールアップ後系列（N≦数百）前提。
//
// 数値正確性は外部参照実装（pyMannKendall 1.4.3 / scipy / statsmodels）の golden test で担保する。

// MKResult は Mann-Kendall 検定の結果。Z は連続性補正済み、PValue は両側。
type MKResult struct {
	S      int     // Σ sign(x_l - x_k), k<l
	VarS   float64 // タイ補正済み分散（Hamed-Rao では自己相関補正後）
	Z      float64 // 連続性補正済み標準化統計量
	PValue float64 // 両側 p 値（normalCDF 由来）
	N      int
}

// SenResult は Sen の傾き（変化の大きさ）。CI はオプション（未算出時 HasCI=false）。
type SenResult struct {
	Slope     float64 // 全ペア (x_l-x_k)/(l-k) の中央値（℃/年・%/年へは handler が換算）
	Intercept float64 // median(x) - median(0..n-1)*slope（Conover 法・pyMannKendall 準拠）
	Lower     float64
	Upper     float64
	HasCI     bool
}

// z975 は標準正規の 0.975 分位（両側 α=0.05 の臨界値）。
// scipy.stats.norm.ppf(0.975) と同値を埋め込み、Hamed-Rao の有意ラグ判定を外部実装と一致させる
// （gonum 非導入＝逆 CDF を持たないため固定 α の定数を使う・design D3）。
const z975 = 1.959963984540054

// normalCDF は標準正規 CDF Φ(z)=0.5*Erfc(-z/√2)（gonum 非依存・math.Erfc）。
// p 値計算は pyMannKendall と同じ式 2*(1-Φ(|z|)) を用い、両側 p を数値一致させる。
func normalCDF(z float64) float64 {
	return 0.5 * math.Erfc(-z/math.Sqrt2)
}

// MannKendall は S・タイ補正 Var(S)・連続性補正 Z・両側 p を算出する。
// 要素2未満は検定不能としてトレンドを断定しない（S=0, PValue=1）。
func MannKendall(xs []float64) MKResult {
	n := len(xs)
	if n < 2 {
		return MKResult{N: n, PValue: 1}
	}
	s := mkScore(xs)
	varS := varianceS(xs)
	z := zScore(s, varS)
	return MKResult{S: s, VarS: varS, Z: z, PValue: twoSidedP(z), N: n}
}

// SeasonalMannKendall は季節キーごとに S/Var(S) を算出して合算する（Hirsch-Slack）。
// seasons[i] は xs[i] の季節キー（例 月 0..11）。月別符号反転を年集約で打ち消さず、
// 各季節内のトレンドを分離して検出する（R4）。seasons は xs と同順・同長を前提。
func SeasonalMannKendall(xs []float64, seasons []int) MKResult {
	n := len(xs)
	if n == 0 || len(seasons) != n {
		return MKResult{N: n, PValue: 1}
	}
	// 季節キーごとに出現順を保って値を集める。
	groups := make(map[int][]float64)
	order := make([]int, 0)
	for i, v := range xs {
		k := seasons[i]
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], v)
	}
	totalS := 0
	totalVar := 0.0
	for _, k := range order {
		g := groups[k]
		if len(g) < 2 {
			continue // 1点の季節は S=0・Var=0（寄与なし）
		}
		totalS += mkScore(g)
		totalVar += varianceS(g)
	}
	z := zScore(totalS, totalVar)
	return MKResult{S: totalS, VarS: totalVar, Z: z, PValue: twoSidedP(z), N: n}
}

// SensSlope は全ペア (x_l-x_k)/(l-k)（k<l）の中央値で単調トレンドの大きさを推定する。
// 外れ値（台風スパイク等）に頑健。切片は Conover 法 median(x)-median(0..n-1)*slope。
// 等間隔系列前提（handler が月次/年次の等間隔系列を渡す）。CI は BlockBootstrapSenCI が別途付与。
func SensSlope(xs []float64) SenResult {
	n := len(xs)
	if n < 2 {
		return SenResult{}
	}
	slopes := make([]float64, 0, n*(n-1)/2)
	for k := 0; k < n-1; k++ {
		for l := k + 1; l < n; l++ {
			slopes = append(slopes, (xs[l]-xs[k])/float64(l-k))
		}
	}
	slope := Median(slopes)
	// median(arange(n)) = (n-1)/2（奇偶共通）。
	intercept := Median(xs) - float64(n-1)/2.0*slope
	return SenResult{Slope: slope, Intercept: intercept}
}

// Lag1Autocorr はラグ1自己相関 r1 = Σ(x_t-x̄)(x_{t+1}-x̄) / Σ(x_t-x̄)² を返す（numpy __acf 定義）。
// 要素2未満・分散0は 0 を返す。
func Lag1Autocorr(xs []float64) float64 {
	n := len(xs)
	if n < 2 {
		return 0
	}
	m := mean(xs)
	var num, den float64
	for t := 0; t < n; t++ {
		d := xs[t] - m
		den += d * d
		if t < n-1 {
			num += d * (xs[t+1] - m)
		}
	}
	if den == 0 {
		return 0
	}
	return num / den
}

// EffectiveSampleSize は有効標本サイズ N_eff≈N(1-r1)/(1+r1) を返す（下限1・上限なし）。
// 強自己相関で N_eff は大幅縮小し、design D6 の留保しきい値（N_eff<10）に使う。
// 1+r1<=0（r1<=-1 の退化）は情報量上限として N を返す。
func EffectiveSampleSize(n int, r1 float64) int {
	denom := 1 + r1
	if denom <= 0 {
		return n
	}
	raw := float64(n) * (1 - r1) / denom
	neff := int(math.Round(raw))
	if neff < 1 {
		return 1
	}
	return neff
}

// HamedRaoModifiedMK はランクの有意自己相関で Var(S) を補正した Mann-Kendall を返す
// （pyMannKendall hamed_rao_modification_test 準拠・既定 α=0.05・lag=n）。
// 要素3未満は補正不能（(n-2) 除算）のため素の MannKendall を返す。
func HamedRaoModifiedMK(xs []float64) MKResult {
	n := len(xs)
	if n < 3 {
		return MannKendall(xs)
	}
	s := mkScore(xs)
	varS := varianceS(xs)

	// Sen 傾きで除去してランク化（x_detrend[t] = x[t] - (t+1)*slope）。
	slope := SensSlope(xs).Slope
	detrend := make([]float64, n)
	for t := 0; t < n; t++ {
		detrend[t] = xs[t] - float64(t+1)*slope
	}
	ranks := rankdataAverage(detrend)
	acf := autocorr(ranks, n-1) // acf[0..n-1]

	interval := z975 / math.Sqrt(float64(n))
	sni := 0.0
	for i := 1; i < n; i++ {
		if acf[i] <= interval && acf[i] >= -interval {
			continue // 非有意ラグは寄与しない
		}
		ni := float64(n - i)
		sni += ni * (ni - 1) * (ni - 2) * acf[i]
	}
	nNS := 1 + (2/(float64(n)*float64(n-1)*float64(n-2)))*sni
	varCorrected := varS * nNS

	z := zScore(s, varCorrected)
	return MKResult{S: s, VarS: varCorrected, Z: z, PValue: twoSidedP(z), N: n}
}

// BlockBootstrapSenCI は移動ブロックブートストラップで Sen 傾きの経験信頼区間を返す。
// 自己相関を保つため Sen 傾きで detrend した残差をブロック再標本化し、決定的トレンド成分を
// 足し戻してから Sen 傾きを再算出する（生系列のブロック並べ替えはトレンド自体を壊すため不可）。
// これにより自己相関ノイズだけを再標本化し、CI は点推定の周りに正しく分布する。
// seed 固定で決定的（R5.5 再現性）。blockLen<=0 は ≈round(n^(1/3))、b=反復数、alpha=有意水準。
// 要素2未満・b<1 は (0,0)。
func BlockBootstrapSenCI(xs []float64, blockLen, b int, alpha float64, seed uint64) (lower, upper float64) {
	n := len(xs)
	if n < 2 || b < 1 {
		return 0, 0
	}
	if blockLen <= 0 {
		blockLen = int(math.Round(math.Cbrt(float64(n))))
		if blockLen < 1 {
			blockLen = 1
		}
	}
	if blockLen > n {
		blockLen = n
	}
	numBlocks := (n + blockLen - 1) / blockLen // ceil(n/blockLen)
	maxStart := n - blockLen                   // 開始位置の上限（含む）

	// Sen 傾きで detrend した残差を再標本化対象にする（トレンド成分は決定的に保持）。
	slope0 := SensSlope(xs).Slope
	residual := make([]float64, n)
	for t := 0; t < n; t++ {
		residual[t] = xs[t] - slope0*float64(t)
	}

	// seed から決定的な PCG 乱数源を作る（global state を使わず再現性を担保）。
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))

	slopes := make([]float64, b)
	resampled := make([]float64, 0, numBlocks*blockLen)
	recon := make([]float64, n)
	for j := 0; j < b; j++ {
		resampled = resampled[:0]
		for blk := 0; blk < numBlocks; blk++ {
			start := rng.IntN(maxStart + 1)
			resampled = append(resampled, residual[start:start+blockLen]...)
		}
		// トレンド成分を足し戻して再構成し、Sen 傾きを算出する。
		for t := 0; t < n; t++ {
			recon[t] = slope0*float64(t) + resampled[t]
		}
		slopes[j] = SensSlope(recon).Slope
	}
	sort.Float64s(slopes)
	lower = quantile(slopes, alpha/2)
	upper = quantile(slopes, 1-alpha/2)
	return lower, upper
}

// BenjaminiHochberg は FDR 制御（BH 法）で各仮説の棄却可否を返す（statsmodels fdr_bh 準拠）。
// 昇順 p_(i) について p_(i) <= (i/m)*alpha を満たす最大 i までを棄却し、元の並び順で返す。
func BenjaminiHochberg(pvalues []float64, alpha float64) []bool {
	m := len(pvalues)
	reject := make([]bool, m)
	if m == 0 {
		return reject
	}
	idx := sortedIndexByP(pvalues)
	kMax := -1
	for rank := 1; rank <= m; rank++ {
		p := pvalues[idx[rank-1]]
		if p <= float64(rank)/float64(m)*alpha {
			kMax = rank
		}
	}
	for rank := 1; rank <= kMax; rank++ {
		reject[idx[rank-1]] = true
	}
	return reject
}

// Bonferroni は p_i <= alpha/m を棄却とする（statsmodels bonferroni 準拠・FWER 制御）。
func Bonferroni(pvalues []float64, alpha float64) []bool {
	m := len(pvalues)
	reject := make([]bool, m)
	if m == 0 {
		return reject
	}
	thresh := alpha / float64(m)
	for i, p := range pvalues {
		reject[i] = p <= thresh
	}
	return reject
}

// --- 内部ヘルパ（パッケージ非公開・純粋。median/quantile は quality.go を流用） ---

// mkScore は S = Σ_{k<l} sign(x_l - x_k)。
func mkScore(xs []float64) int {
	s := 0
	n := len(xs)
	for k := 0; k < n-1; k++ {
		for l := k + 1; l < n; l++ {
			switch {
			case xs[l] > xs[k]:
				s++
			case xs[l] < xs[k]:
				s--
			}
		}
	}
	return s
}

// varianceS はタイ補正済み Var(S)=[N(N-1)(2N+5)-Σ t_j(t_j-1)(2t_j+5)]/18。
func varianceS(xs []float64) float64 {
	n := len(xs)
	base := float64(n) * float64(n-1) * float64(2*n+5)
	// タイ群の度数を数える（同値ごと）。
	counts := make(map[float64]int)
	for _, v := range xs {
		counts[v]++
	}
	var tieSum float64
	for _, tp := range counts {
		if tp > 1 {
			t := float64(tp)
			tieSum += t * (t - 1) * (2*t + 5)
		}
	}
	return (base - tieSum) / 18
}

// zScore は連続性補正済み標準化統計量。Var(S)<=0 は 0（検定不能）。
func zScore(s int, varS float64) float64 {
	if varS <= 0 {
		return 0
	}
	switch {
	case s > 0:
		return (float64(s) - 1) / math.Sqrt(varS)
	case s < 0:
		return (float64(s) + 1) / math.Sqrt(varS)
	default:
		return 0
	}
}

// twoSidedP は両側 p 値 2*(1-Φ(|z|))。pyMannKendall と同式で数値一致させる。
func twoSidedP(z float64) float64 {
	p := 2 * (1 - normalCDF(math.Abs(z)))
	if p > 1 {
		return 1
	}
	if p < 0 {
		return 0
	}
	return p
}

// rankdataAverage は同値に平均順位を与える1始まりランクを返す（scipy rankdata 'average' 準拠）。
func rankdataAverage(xs []float64) []float64 {
	n := len(xs)
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return xs[idx[a]] < xs[idx[b]] })
	ranks := make([]float64, n)
	i := 0
	for i < n {
		j := i
		for j+1 < n && xs[idx[j+1]] == xs[idx[i]] {
			j++
		}
		avg := float64((i+1)+(j+1)) / 2.0 // 1始まり順位の平均
		for k := i; k <= j; k++ {
			ranks[idx[k]] = avg
		}
		i = j + 1
	}
	return ranks
}

// autocorr はラグ0..maxLag の自己相関を返す（numpy __acf 定義）。
// acf[k] = Σ_{t=0}^{n-1-k} y_t y_{t+k} / Σ_{t} y_t²（y=x-x̄）。分散0は acf[0]=1・以降0。
func autocorr(xs []float64, maxLag int) []float64 {
	n := len(xs)
	m := mean(xs)
	y := make([]float64, n)
	var den float64
	for t := 0; t < n; t++ {
		y[t] = xs[t] - m
		den += y[t] * y[t]
	}
	acf := make([]float64, maxLag+1)
	if den == 0 {
		acf[0] = 1
		return acf
	}
	for k := 0; k <= maxLag; k++ {
		var num float64
		for t := 0; t+k < n; t++ {
			num += y[t] * y[t+k]
		}
		acf[k] = num / den
	}
	return acf
}

// sortedIndexByP は p 値の昇順インデックス列を返す（多重比較補正の順位付け用・安定ソート）。
func sortedIndexByP(pvalues []float64) []int {
	idx := make([]int, len(pvalues))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return pvalues[idx[a]] < pvalues[idx[b]] })
	return idx
}
