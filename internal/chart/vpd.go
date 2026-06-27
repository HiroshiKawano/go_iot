package chart

import "math"

// VPD（飽差）純関数層。温湿度から飽差 VPD[kPa]・VPD 系列・適正帯滞在率を
// すべて []float64/スカラ入出力の純関数として提供する（stats.go の延長）。
//
// 本ファイルは最下流の純粋層であり gin/DB/templ/pgtype/time に依存しない（math のみ）。
// 時刻・タイムゾーン・pgtype 変換・作物別しきい値の解決は handler 境界に留める。

// Tetens 式の確定定数（変更禁止・要件 1.1）。
// es(T) = 0.6108 * exp(17.27*T/(T+237.3)) [kPa]。
const (
	tetensA = 0.6108 // 0℃の飽和水蒸気圧 [kPa]
	tetensB = 17.27
	tetensC = 237.3
)

// saturationVaporPressure は気温 tempC[℃] の飽和水蒸気圧 es(T)[kPa] を Tetens 式で返す。
// es は常に正で、氷点下でも NaN/Inf を出さない（指数は有限・分母は常に正：tempC > -237.3℃）。
func saturationVaporPressure(tempC float64) float64 {
	return tetensA * math.Exp(tetensB*tempC/(tempC+tetensC))
}

// VPD は気温 tempC[℃] と相対湿度 rh[%] から飽差 VPD[kPa] を返す。
// VPD = es(T) * (1 - RH/100)。RH は [0,100] にクランプする（CHECK 保証だが防御的）。
//   - RH=100% → 0kPa（飽差ゼロ・要件 1.3）
//   - RH=0%   → es(T)（その温度での最大飽差・要件 1.4）
//   - 氷点下でも数値破綻（NaN/Inf）しない（要件 1.5）
func VPD(tempC, rh float64) float64 {
	if rh < 0 {
		rh = 0
	} else if rh > 100 {
		rh = 100
	}
	return saturationVaporPressure(tempC) * (1 - rh/100)
}

// VPDSeries は温湿度の同長スライスから VPD 系列を返す（len = min(len(temps), len(hums))）。
// 欠測由来の長さ不一致は短い方に合わせて防御的に扱う（通常は同長・要件 1.6）。
func VPDSeries(temps, hums []float64) []float64 {
	n := len(temps)
	if len(hums) < n {
		n = len(hums)
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = VPD(temps[i], hums[i])
	}
	return out
}

// TimeInRange は values のうち適正帯 [lower, upper]（両端含む）に入る割合(0..1)を返す。
// 「計測のうち適正帯に入っていた時間割合」＝滞在率の素（要件 5.1, 5.2）。
// 空入力は 0。事前条件 lower<=upper（呼び出し側が crop.VPDRange で保証）。
// 事後条件 0 <= r <= 1。
func TimeInRange(values []float64, lower, upper float64) float64 {
	if len(values) == 0 {
		return 0
	}
	in := 0
	for _, v := range values {
		if v >= lower && v <= upper {
			in++
		}
	}
	return float64(in) / float64(len(values))
}
