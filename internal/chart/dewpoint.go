package chart

import "math"

// 露点・結露・葉面湿潤・病害下地の純関数層。温湿度から露点温度 Td[℃]・スプレッド T−Td・
// 結露帯/高湿度の連続区間・病害スコア下地を、すべて []float64/[]bool/スカラ入出力の
// 純関数として提供する（vpd.go・quality.go と同じ最下流の純粋層）。
//
// 本ファイルは gin/DB/templ/pgtype/time に依存しない（math のみ）。
// 時刻・タイムゾーン・recorded_at 差分の暦日積算・作物別しきい値の解決は handler 境界に留める。
// 入力スライスは破壊しない（イミュータブル）。Tetens 定数 tetensB/tetensC は vpd.go から再利用する。

// rhFloor は露点算出時の相対湿度の下限[%]。RH=0% の ln(0)=−Inf を避けるため床上げする。
// CHECK 制約は RH=0 を許容するため防御的に必須（design 確定値・要件 1.4）。
const rhFloor = 1.0

// DewPoint は気温 tempC[℃] と相対湿度 rh[%] から露点温度 Td[℃] を返す（Magnus/Tetens）。
//
//	γ  = ln(RHc/100) + tetensB·T/(T+tetensC)
//	Td = tetensC·γ/(tetensB − γ)
//
// RHc は [rhFloor, 100] に床上げ/上限クランプする（要件 1.4）。
//   - RH=100% → Td=T（恒等・代数的に厳密／要件 1.3）
//   - RH=0%/微小 → 床上げで NaN/Inf を出さない（要件 1.4）
//   - 氷点下（−40℃）でも分母 T+tetensC≈197.3>0、物理域で γ<tetensB ゆえ tetensB−γ>0 → NaN/Inf なし（要件 1.5）
func DewPoint(tempC, rh float64) float64 {
	rhc := rh
	if rhc < rhFloor {
		rhc = rhFloor
	} else if rhc > 100 {
		rhc = 100
	}
	gamma := math.Log(rhc/100) + tetensB*tempC/(tempC+tetensC)
	return tetensC * gamma / (tetensB - gamma)
}

// DewPointSeries は温湿度の同長スライスから露点系列を返す（len = min(len(temps), len(hums))）。
// 欠測由来の長さ不一致は短い方に合わせて防御的に扱う（要件 1.6・VPDSeries 同型）。
func DewPointSeries(temps, hums []float64) []float64 {
	n := len(temps)
	if len(hums) < n {
		n = len(hums)
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = DewPoint(temps[i], hums[i])
	}
	return out
}

// DewPointSpread は気温と露点のスプレッド T−Td を返す（len = min(len(temps), len(dewpoints))）。
// スプレッドは結露しやすさの代理（小さいほど結露しやすい）。負値は 0 にクランプし常に ≥0（要件 2.1）。
func DewPointSpread(temps, dewpoints []float64) []float64 {
	n := len(temps)
	if len(dewpoints) < n {
		n = len(dewpoints)
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		s := temps[i] - dewpoints[i]
		if s < 0 {
			s = 0
		}
		out[i] = s
	}
	return out
}

// Run は xAxis インデックスの連続区間 [StartIdx, EndIdx]（両端含む）を表す。
// 結露帯・高湿度イベント・（将来の）結露時間で共有する（不変条件 EndIdx ≥ StartIdx）。
type Run struct {
	StartIdx int
	EndIdx   int
}

// runsFromMask は真偽マスク上で true が minRun 点以上連続する区間を Run スライスで返す。
// 結露帯（minRun=1）と高湿度イベント（minRun=N）の共有内部関数（synthesis 一般化）。
// minRun 未満の連続（単発含む）は除外し、隣接する連続は1区間に結合する。空マスクは空スライス。
// quality.go の StuckRuns（同値ラン）と同型だが述語が呼び出し側のマスク生成に委ねられている点が異なる。
func runsFromMask(mask []bool, minRun int) []Run {
	if minRun < 1 {
		minRun = 1
	}
	var runs []Run
	start := -1 // -1 = ラン外
	for i, m := range mask {
		switch {
		case m && start < 0:
			start = i // ラン開始
		case !m && start >= 0:
			if i-start >= minRun {
				runs = append(runs, Run{StartIdx: start, EndIdx: i - 1})
			}
			start = -1 // ラン終了
		}
	}
	if start >= 0 && len(mask)-start >= minRun {
		runs = append(runs, Run{StartIdx: start, EndIdx: len(mask) - 1})
	}
	return runs
}

// CondensationRuns はスプレッド spread[i] ≤ maxSpread の連続区間（結露帯＝湿り側）を返す。
// 結露しやすい時間区間ゆえ minRun=1（単発も結露扱い）。境界値（== maxSpread）は結露に含める。
// 事前条件: maxSpread ≥ 0（handler が DiseaseModel で保証）。入力スライスは破壊しない（要件 2.2）。
func CondensationRuns(spread []float64, maxSpread float64) []Run {
	mask := make([]bool, len(spread))
	for i, s := range spread {
		mask[i] = s <= maxSpread
	}
	return runsFromMask(mask, 1)
}

// WetnessMask は各点が葉面湿潤（RH ≥ rhThreshold）かを真偽列で返す（要件 3.1）。
// 境界値（== rhThreshold）は湿潤扱い。日次積算の時間換算は handler 境界（純粋層は time 非依存）。
// 事後条件: len(out) == len(hums)。入力スライスは破壊しない。
func WetnessMask(hums []float64, rhThreshold float64) []bool {
	out := make([]bool, len(hums))
	for i, h := range hums {
		out[i] = h >= rhThreshold
	}
	return out
}

// HighHumidityRuns は RH ≥ rhThreshold が minRun 点以上連続した区間（高湿度イベント）を返す。
// 単発（minRun 未満）は除外する（要件 4.1, 4.2）。WetnessMask と runsFromMask を共有する。
// 事前条件: minRun ≥ 1（handler が点数換算で保証・0 以下は 1 に正規化）。入力スライスは破壊しない。
func HighHumidityRuns(hums []float64, rhThreshold float64, minRun int) []Run {
	return runsFromMask(WetnessMask(hums, rhThreshold), minRun)
}

// DiseaseScore は「発病好適温度帯 [tempLow,tempHigh] 内かつ葉面湿潤」の点割合を 0..1 で返す病害下地。
// 温度帯×葉面湿潤の最小合成（下地）であり、確定予察モデル・特定病害固有予察・発病記録突合は含めない（要件 5.1, 5.3）。
// 温度帯境界（== tempLow / == tempHigh）は帯内扱い。空入力は 0。
// 事前条件: tempLow ≤ tempHigh（handler が DiseaseModel で保証）。事後条件: 0 ≤ score ≤ 1。
// temps/wet の長さ不一致は短い方に合わせる（欠測由来・防御的）。入力スライスは破壊しない。
func DiseaseScore(temps []float64, wet []bool, tempLow, tempHigh float64) float64 {
	n := len(temps)
	if len(wet) < n {
		n = len(wet)
	}
	if n == 0 {
		return 0
	}
	hit := 0
	for i := 0; i < n; i++ {
		if wet[i] && temps[i] >= tempLow && temps[i] <= tempHigh {
			hit++
		}
	}
	return float64(hit) / float64(n)
}
