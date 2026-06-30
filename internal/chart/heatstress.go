package chart

import "math"

// 高温ストレス（暑熱）純関数層。温湿度から温湿度指数 THI・絶対湿度 AH・熱帯夜マスク・
// 連続ラン（最長/現在）を、すべて []float64/[]bool/スカラ入出力の純関数として提供する
// （vpd.go の延長・派生指標パネルの第4弾）。
//
// 本ファイルは最下流の純粋層であり gin/DB/templ/pgtype/time に依存しない（math のみ）。
// 時刻・タイムゾーン・暦日/夜間/年バケット・作物別しきい値の解決は handler 境界に留める。

// AH 換算定数（要件 1.2・付録A D④）。
//   AH[g/m³] = 217 · ea[hPa] / T[K]
// 物理式 AH = 2.1668·e[Pa]/T[K] の e を hPa 表記にした近似定数（216.68≈217）。
// 飽和水蒸気圧 saturationVaporPressure は kPa を返すため、ea[hPa] = es[kPa]·10·RH/100 で換算する。
const ahConstantHPa = 217.0

// clampRH は相対湿度を [0,100] にクランプする（要件 1.3・CHECK 保証だが防御的）。
func clampRH(rh float64) float64 {
	if rh < 0 {
		return 0
	}
	if rh > 100 {
		return 100
	}
	return rh
}

// THI は気温 tempC[℃] と相対湿度 rh[%] から温湿度指数 THI を返す（付録A D⑥）。
//   THI = 0.8·T + (RH/100)·(T − 14.4) + 46.4
// RH は [0,100] にクランプする。氷点下でも多項式ゆえ NaN/Inf を出さない（要件 1.4）。
func THI(tempC, rh float64) float64 {
	rh = clampRH(rh)
	return 0.8*tempC + (rh/100)*(tempC-14.4) + 46.4
}

// THISeries は温湿度の同長スライスから THI 系列を返す（len = min(len(temps), len(hums))）。
// 欠測由来の長さ不一致は短い方に合わせる。入力スライスは破壊しない。
func THISeries(temps, hums []float64) []float64 {
	n := len(temps)
	if len(hums) < n {
		n = len(hums)
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = THI(temps[i], hums[i])
	}
	return out
}

// AbsoluteHumidity は気温 tempC[℃] と相対湿度 rh[%] から容積絶対湿度 AH[g/m³] を返す（要件 1.2）。
//   ea[hPa] = saturationVaporPressure(T)[kPa] · 10 · RH/100      （実水蒸気圧）
//   AH      = 217 · ea[hPa] / (T + 273.15)                        （付録A D④）
// saturationVaporPressure（vpd.go）を再利用し Tetens 定数を重複定義しない。
//   - RH=0%   → AH=0（厳密・要件 1.5）
//   - RH 範囲外 → [0,100] にクランプ（要件 1.3）
//   - 氷点下（−40℃まで）→ NaN/Inf/ゼロ割なし（T+273.15 ≥ 233.15 > 0・要件 1.4）。AH は非負。
func AbsoluteHumidity(tempC, rh float64) float64 {
	rh = clampRH(rh)
	if rh == 0 {
		return 0
	}
	eaHPa := saturationVaporPressure(tempC) * 10 * (rh / 100)
	return ahConstantHPa * eaHPa / (tempC + 273.15)
}

// AbsoluteHumiditySeries は温湿度の同長スライスから AH 系列を返す（len = min(len(temps), len(hums))）。
// 入力スライスは破壊しない。
func AbsoluteHumiditySeries(temps, hums []float64) []float64 {
	n := len(temps)
	if len(hums) < n {
		n = len(hums)
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = AbsoluteHumidity(temps[i], hums[i])
	}
	return out
}

// TropicalNightMask は各暦日の夜温（日次代表気温）が threshold 以上の日を true とするマスクを返す。
// 閾値ちょうど（==threshold）は熱帯夜＝true（境界を含む・要件 2.1, 2.2）。
// NaN（夜間欠測＝判定不能）は熱帯夜0と誤計上せず false とする（要件 2.5）。
// 事後条件: len(out)==len(dailyNightTemps)。空入力は空。入力スライスは破壊しない。
func TropicalNightMask(dailyNightTemps []float64, threshold float64) []bool {
	out := make([]bool, len(dailyNightTemps))
	for i, t := range dailyNightTemps {
		if math.IsNaN(t) {
			continue // 欠測（NaN）は非該当（false 既定値のまま）
		}
		out[i] = t >= threshold
	}
	return out
}

// RunStats は真偽マスクから true の最長連続日数と末尾（現在）連続日数を返す（要件 2.3）。
// current は末尾から遡って連続する true の長さ（末尾が false なら 0）。
// 全 true・全 false・単発・空を破綻なく処理する（要件 2.4）。
func RunStats(mask []bool) (longest, current int) {
	run := 0
	for _, v := range mask {
		if v {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	return longest, run
}
