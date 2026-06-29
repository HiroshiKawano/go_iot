package chart

// GDD（Growing Degree Days・積算温度）の純関数層。日次の最高/最低気温と作物別 Tbase から
// 日次 GDD・累積 GDD・残り積算温度・到達日外挿・生育ステージ判定を、すべて []float64/スカラ
// 入出力の純関数として提供する（stats.go・vpd.go・dewpoint.go と同じ最下流の純粋層）。
//
// 本ファイルは gin/DB/templ/pgtype/time に依存しない（math すら使わず標準演算のみ）。
// 日次気温の JST 暦日バケット・定植日からの経過日数換算は handler 境界に留め、ここには持ち込まない
// （純粋層へは []float64/スカラのみ渡す）。入力スライスは破壊しない（イミュータブル）。
//
// 到達日外挿 ForecastDaysToTarget は stats.go::LinearFit（最小二乗）を内部利用する。

// DailyGDD は各暦日の日次 GDD = max((tMax+tMin)/2 − tBase, 0) を返す（要件 1.1）。
// 日平均気温が tBase 未満の日は 0 にクランプし、負の日次 GDD を作らない（氷点下・生育せず＝0）。
// 事前条件: len(tMax)==len(tMin)（handler が日次集計で保証）。
// 長さ不一致は短い方に合わせて防御的に扱う（欠測由来・DewPointSeries 同型・要件 1.7）。
// 事後条件: len(out)==min(len(tMax),len(tMin))、各要素 >= 0。入力スライスは破壊しない。
func DailyGDD(tMax, tMin []float64, tBase float64) []float64 {
	n := len(tMax)
	if len(tMin) < n {
		n = len(tMin)
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		g := (tMax[i]+tMin[i])/2 - tBase
		if g < 0 {
			g = 0
		}
		out[i] = g
	}
	return out
}

// CumulativeGDD は日次 GDD を定植日から前方累積した、単調非減少の累積 GDD 系列を返す（要件 1.2）。
// daily は非負（DailyGDD のゼロクランプ）ゆえ累積は単調非減少。
// 事後条件: len(out)==len(daily)、out[i]>=out[i-1]。空入力は空。入力スライスは破壊しない。
func CumulativeGDD(daily []float64) []float64 {
	out := make([]float64, len(daily))
	var sum float64
	for i, d := range daily {
		sum += d
		out[i] = sum
	}
	return out
}

// RemainingGDD は収穫目標 targetGDD までの残り積算温度 = max(targetGDD − 最新累積, 0) を返す（要件 1.3）。
// 既に到達済み（最新累積 >= targetGDD）は 0。空入力は targetGDD を返す（累積未開始＝まだ何も積んでいない）。
func RemainingGDD(cumulative []float64, targetGDD float64) float64 {
	if len(cumulative) == 0 {
		return targetGDD
	}
	r := targetGDD - cumulative[len(cumulative)-1]
	if r < 0 {
		r = 0
	}
	return r
}

// ForecastDaysToTarget は経過日数 xs と累積 GDD cumulative を LinearFit し、傾きから
// 収穫目標 targetGDD への到達経過日を外挿する（要件 1.4）。days は定植日からの経過日数。
//
// ok=false（予測不能・確定値を返さない・要件 1.5）:
//   - LinearFit が ok=false（点が足りない len<2／x 分散0＝実質1点）
//   - 傾き <= 0（生育せず・右肩下がり＝到達見込みなし）
//   - 既に最新累積 >= targetGDD（到達済み）
//   - 外挿日が直近経過日より過去（回帰直線が末尾で target を追い越す減速ケース＝過去には外挿しない）
//
// 事後条件: ok のとき days >= 直近経過日（xs の最終要素）。入力スライスは破壊しない。
func ForecastDaysToTarget(xs, cumulative []float64, targetGDD float64) (days float64, ok bool) {
	if len(cumulative) == 0 {
		return 0, false
	}
	// 到達済みは予測しない（残りは RemainingGDD=0 が示す）。
	if cumulative[len(cumulative)-1] >= targetGDD {
		return 0, false
	}
	slope, intercept, fitOK := LinearFit(xs, cumulative)
	if !fitOK || slope <= 0 {
		return 0, false
	}
	days = (targetGDD - intercept) / slope
	// 過去には外挿しない（捏造回避・到達済みの精神の防御的拡張）。
	lastX := xs[len(xs)-1]
	if days < lastX {
		return 0, false
	}
	return days, true
}

// GrowthStageIndex は累積 GDD cumulative が昇順しきい値列 stageGDD のどの段に達したかの index を返す（要件 4.2）。
// cumulative >= stageGDD[i] を満たす最大の i（昇順ゆえ末尾側が上位ステージ）。境界（==）は到達扱い。
// どのしきい値にも未到達（cumulative < stageGDD[0]）は -1（最初のステージ未満）。stageGDD 空も -1。
func GrowthStageIndex(cumulative float64, stageGDD []float64) int {
	idx := -1
	for i, threshold := range stageGDD {
		if cumulative >= threshold {
			idx = i
		} else {
			break // 昇順ゆえ以降は全て未到達
		}
	}
	return idx
}
