package chart

import (
	"math"
	"sort"
)

// 品質判定の純関数層。蓄積済み温湿度データへ「どの期間・どのレコードが信頼できるか」の
// メタ情報を上載せするため、欠測率・サンプリング間隔の一貫性・外れ値・固着(stuck/flatline)・
// 物理範囲・急変といった異常検知/率集計を、すべて []float64 入出力の純関数として提供する。
//
// 本ファイルは stats.go と同じく最下流の純粋層であり gin/DB/templ/pgtype/time に依存しない。
// 時刻差分（間隔秒列の生成）は handler 境界で行い、ここには秒数 []float64 として渡る。
// 入力スライスは破壊しない（イミュータブル）。空/単一点/全同値/散らばり0 でも破綻しない。
//
// しきい値は各関数の引数で受け（テスト容易性・将来調整のため）、既定値は本ファイル下部の
// 定数群が持つ。handler 層がその定数を渡す設計とし、純関数自体はハードコードしない。

// GapSpan は連続欠測区間を表す。インデックスは間隔秒列に対応する readings 側の隣接点で表し、
// その区間に何スロット欠落しているか（MissingSlots）を併せ持つ。
type GapSpan struct {
	StartIdx     int // 欠測区間の手前の readings インデックス
	EndIdx       int // 欠測区間の直後の readings インデックス（= StartIdx+1）
	MissingSlots int // この区間で欠落している期待スロット数（round(間隔/中央値)-1）
}

// ---- 1.1 欠測率・欠測ギャップ区間・サンプリング間隔一貫性 ----------------------

// MissingStats は間隔秒列（recorded_at 差分・handler 生成）から欠測の概況を返す。
//   - rate         : 欠測率(%) = 欠測本数 / 期待総本数 * 100
//   - missingCount : 欠測本数 = Σ max(0, round(間隔/中央値) - 1)
//   - gaps         : 欠測スロットを含む各区間（GapSpan・元データのインデックス基準）
//   - ok           : 算出可否（要素2未満・中央値0以下なら false ＝率/区間 未定義）
//
// 期待間隔は観測間隔の中央値とし、firmware 変更・device 差・gap 自体に頑健にする。
func MissingStats(intervalSecs []float64) (rate float64, missingCount int, gaps []GapSpan, ok bool) {
	if len(intervalSecs) < 2 {
		return 0, 0, nil, false
	}
	med := Median(intervalSecs)
	if med <= 0 {
		return 0, 0, nil, false
	}
	for i, iv := range intervalSecs {
		slots := int(math.Round(iv/med)) - 1
		if slots <= 0 {
			continue
		}
		missingCount += slots
		gaps = append(gaps, GapSpan{StartIdx: i, EndIdx: i + 1, MissingSlots: slots})
	}
	// 期待総本数 = 実本数 + 欠測本数。実本数 = 間隔数 + 1。
	expected := len(intervalSecs) + 1 + missingCount
	rate = float64(missingCount) / float64(expected) * 100
	return rate, missingCount, gaps, true
}

// IntervalConsistency は間隔秒列のばらつき指標（変動係数 CV = σ/μ）を返す。
// 等間隔なら CV0。要素2未満・μ≈0（|μ|<eps）なら ok=false（未定義）。既存 CV を流用する。
func IntervalConsistency(intervalSecs []float64, eps float64) (cv float64, ok bool) {
	if len(intervalSecs) < 2 {
		return 0, false
	}
	return CV(intervalSecs, eps)
}

// ---- 1.2 外れ値判定（ローリングσ法を主・Zスコア/IQR を補助） -----------------

// RollingOutliers は移動窓 μ±kσ（SMA/MovingStdDev 流用）で各点が外れ値かを返す。
// 昼夜変動を窓が追従するため大域 Z/IQR より誤検出が少ない（主たる外れ値判定）。
//   - warm-up（index < window-1）は false（窓が満ちず過検出を避ける）
//   - σ <= eps（散らばり0近傍）は false（ゼロ除算を避け外れ値なしとする）
//   - それ以外は |value - SMA| > k*σ を外れ値とする
//
// 事後条件: len(out) == len(values)。空/単一点も安全に扱う。
func RollingOutliers(values []float64, window int, k, eps float64) []bool {
	out := make([]bool, len(values))
	sma := SMA(values, window)
	sigma := MovingStdDev(values, window)
	for i := range values {
		if i < window-1 {
			continue // warm-up（窓が満ちず過検出を避ける）
		}
		if sigma[i] <= eps {
			continue // 散らばり0近傍はゼロ除算を避け外れ値なし
		}
		if math.Abs(values[i]-sma[i]) > k*sigma[i] {
			out[i] = true
		}
	}
	return out
}

// ZScores は大域 Zスコア (value-μ)/σ を返す（補助・主経路外）。
// σ=0（定常列）はゼロ除算を避けて全 0 を返す。事後条件: len(out)==len(values)。
func ZScores(values []float64) []float64 {
	out := make([]float64, len(values))
	sd := StdDev(values)
	if sd == 0 {
		return out // 定常列はゼロ除算を避けて全 0
	}
	m := Mean(values)
	for i, v := range values {
		out[i] = (v - m) / sd
	}
	return out
}

// IQRBounds は四分位範囲ベースの外れ値境界（補助・主経路外）を返す。
//   - lower = Q1 - coef*IQR, upper = Q3 + coef*IQR（IQR = Q3-Q1）
//   - ok   : 要素2未満なら false
//
// 四分位は線形補間（位置 = p*(n-1)）で求める。入力スライスは破壊しない。
func IQRBounds(values []float64, coef float64) (lower, upper float64, ok bool) {
	if len(values) < 2 {
		return 0, 0, false
	}
	s := make([]float64, len(values))
	copy(s, values)
	sort.Float64s(s)
	q1 := quantile(s, 0.25)
	q3 := quantile(s, 0.75)
	iqr := q3 - q1
	return q1 - coef*iqr, q3 + coef*iqr, true
}

// ---- 1.3 stuck/flatline・物理範囲・急変 --------------------------------------

// StuckRuns は同一値（2 小数まで完全同値）が minRun 回以上連続する区間を真偽列で返す。
// センサー固着(stuck/flatline)の疑いを示す。事後条件: len(out)==len(values)。空入力は空。
func StuckRuns(values []float64, minRun int) []bool {
	if minRun < 1 {
		minRun = 1
	}
	out := make([]bool, len(values))
	runStart := 0
	for i := 1; i <= len(values); i++ {
		// 2 小数まで丸めて直前と完全同値か判定し、同値ランの末尾で長さを評価する。
		if i < len(values) && round2(values[i]) == round2(values[runStart]) {
			continue
		}
		if i-runStart >= minRun {
			for j := runStart; j < i; j++ {
				out[j] = true
			}
		}
		runStart = i
	}
	return out
}

// PhysicalAnomalies は農学的物理範囲 [min,max] の外（min 未満 or max 超）を真とする。
// 受信 CHECK の内側にありながらあり得ない値・据置故障を捉える。境界値（==min/==max）は範囲内＝偽。
// 事後条件: len(out)==len(values)。
func PhysicalAnomalies(values []float64, min, max float64) []bool {
	out := make([]bool, len(values))
	for i, v := range values {
		out[i] = v < min || v > max
	}
	return out
}

// RapidChanges は隣接サンプル間の差 |Δ| が maxDelta を超える点を真とする（後側の点を立てる）。
// 短時間の非現実的な急変を捉える。先頭点（前者なし）は偽。事後条件: len(out)==len(values)。
func RapidChanges(values []float64, maxDelta float64) []bool {
	out := make([]bool, len(values))
	for i := 1; i < len(values); i++ {
		if math.Abs(values[i]-values[i-1]) > maxDelta {
			out[i] = true
		}
	}
	return out
}

// --- 品質判定しきい値の既定定数（research/沖縄実環境で調整可・handler が純関数へ渡す） ---

const (
	// OutlierWindow はローリングσ外れ値判定の移動窓幅（点数）。
	// 既定 12 点 ≈ 1 時間（firmware の約 5 分間隔送信前提）。昼夜変動を窓が追従し誤検出を抑える。
	OutlierWindow = 12
	// OutlierK はローリングσ外れ値の許容係数（μ±kσ の外を外れ値）。経験則の 3σ。
	OutlierK = 3.0
	// OutlierEps はローリングσが実質ゼロ（定常列）か判定する下限。これ以下は外れ値なし。
	OutlierEps = 1e-9

	// StuckMinRun は固着(stuck/flatline)とみなす完全同値の最小連続回数。
	// 既定 6 回 ≈ 30 分（5 分間隔）同値が続けばセンサー固着を疑う。
	// 沖縄では湿度の長時間高止まりが正常なため、固着は exact 同値連続でのみ検出する（湿度の物理範囲は設けない）。
	StuckMinRun = 6

	// TempPhysicalMin / TempPhysicalMax は温度の農学的物理範囲 [℃]（受信 CHECK の内側）。
	// 沖縄のハウス環境で −10〜60℃ を外れる値は据置故障・誤配線を疑う。
	TempPhysicalMin = -10.0
	TempPhysicalMax = 60.0

	// TempRapidDelta / HumidityRapidDelta は隣接サンプル間の急変上限（|Δ|）。
	// 5 分間で温度 10℃ 超・湿度 40%RH 超の変化は非現実的で計測異常を疑う。
	TempRapidDelta     = 10.0
	HumidityRapidDelta = 40.0
)

// --- 内部ヘルパ（パッケージ非公開・純粋） ---

// Median は中央値を返す（入力は破壊しない）。空は 0。偶数長は中央2要素の平均。
// handler の点数換算（estimatePointsPerDay）が間隔中央値を再利用するためエクスポートする。
func Median(xs []float64) float64 {
	n := len(xs)
	if n == 0 {
		return 0
	}
	s := make([]float64, n)
	copy(s, xs)
	sort.Float64s(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// round2 は小数第2位までに丸める（固着の同値判定用）。
func round2(x float64) float64 {
	return math.Round(x*100) / 100
}

// quantile は昇順ソート済みスライス sorted の分位点 p(0..1) を線形補間で返す。
// 位置 = p*(n-1) として前後要素を内挿する。len>=1 を前提（呼出側で保証）。
func quantile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 1 {
		return sorted[0]
	}
	pos := p * float64(n-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	frac := pos - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}
