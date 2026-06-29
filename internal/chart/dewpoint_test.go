package chart

import (
	"math"
	"testing"
)

// dewTol は露点 Td(℃) の手計算一致を検証する許容差。
const dewTol = 0.01

// TestDewPoint_KnownValues は既知の手計算ケースと一致することを固定する（要件 1.1, 1.7）。
//   - 25℃/50% → 露点 ≈ 13.86℃
//   - 30℃/80% → 露点 ≈ 26.17℃
//
// γ = ln(RH/100) + 17.27·T/(T+237.3)、Td = 237.3·γ/(17.27 − γ)。
func TestDewPoint_KnownValues(t *testing.T) {
	tests := []struct {
		name string
		temp float64
		rh   float64
		want float64
	}{
		{"25℃/50%≈13.86℃", 25, 50, 13.86},
		{"30℃/80%≈26.17℃", 30, 80, 26.17},
		{"20℃/100%→Td=T(恒等)", 20, 100, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DewPoint(tt.temp, tt.rh)
			if math.Abs(got-tt.want) > dewTol {
				t.Errorf("DewPoint(%v,%v) = %v, want %v (±%v)", tt.temp, tt.rh, got, tt.want, dewTol)
			}
		})
	}
}

// TestDewPoint_ClampsHumidity は RH を [rhFloor,100] にクランプすることを固定する（要件 1.4・防御的）。
func TestDewPoint_ClampsHumidity(t *testing.T) {
	// RH>100 は 100 として扱い Td=T（CHECK 保証だが防御的）。
	if got := DewPoint(25, 150); math.Abs(got-25) > 1e-9 {
		t.Errorf("DewPoint(25,150) = %v, want 25（RH>100 を 100 にクランプ→Td=T）", got)
	}
	// RH<rhFloor（負含む）は rhFloor へ床上げし、同じ床上げ値 0.5% と一致する。
	if got, want := DewPoint(25, -10), DewPoint(25, rhFloor); math.Abs(got-want) > 1e-9 {
		t.Errorf("DewPoint(25,-10) = %v, want %v（RH<rhFloor を床上げ）", got, want)
	}
}

// TestDewPoint_FullHumidityIsTemperature は RH=100% で Td=T となる恒等を固定する（要件 1.3）。
func TestDewPoint_FullHumidityIsTemperature(t *testing.T) {
	for _, temp := range []float64{-5, 0, 10, 25, 40} {
		got := DewPoint(temp, 100)
		if math.Abs(got-temp) > 1e-9 {
			t.Errorf("DewPoint(%v,100) = %v, want %v（RH=100→Td=T 恒等）", temp, got, temp)
		}
	}
}

// TestDewPoint_LowHumidityNoNaNInf は RH=0/微小でも床上げで NaN/Inf を出さないことを固定する（要件 1.4）。
func TestDewPoint_LowHumidityNoNaNInf(t *testing.T) {
	for _, rh := range []float64{0, 0.0001, 0.5, 0.99} {
		got := DewPoint(25, rh)
		if math.IsNaN(got) || math.IsInf(got, 0) {
			t.Errorf("DewPoint(25,%v) = %v（NaN/Inf を出してはいけない）", rh, got)
		}
		// 露点は気温以下であること（乾けば乾くほど低い）。
		if got > 25 {
			t.Errorf("DewPoint(25,%v) = %v, want ≤ 25（乾燥側は気温未満）", rh, got)
		}
	}
}

// TestDewPoint_BelowFreezingNoNaNInf は氷点下（下限 −40℃）でも NaN/Inf を出さないことを固定する（要件 1.5）。
func TestDewPoint_BelowFreezingNoNaNInf(t *testing.T) {
	for _, temp := range []float64{-40, -10, -5, -0.5} {
		for _, rh := range []float64{0, 1, 50, 100} {
			got := DewPoint(temp, rh)
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Errorf("DewPoint(%v,%v) = %v（NaN/Inf を出してはいけない）", temp, rh, got)
			}
			if got > temp+1e-9 {
				t.Errorf("DewPoint(%v,%v) = %v, want ≤ 気温（露点は気温以下）", temp, rh, got)
			}
		}
	}
}

func TestDewPointSeries(t *testing.T) {
	t.Run("各点で DewPoint と一致", func(t *testing.T) {
		temps := []float64{25, 30, 10}
		hums := []float64{50, 80, 100}
		got := DewPointSeries(temps, hums)
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
		want := []float64{DewPoint(25, 50), DewPoint(30, 80), DewPoint(10, 100)}
		for i := range want {
			if math.Abs(got[i]-want[i]) > 1e-9 {
				t.Errorf("DewPointSeries[%d] = %v, want %v", i, got[i], want[i])
			}
		}
	})

	t.Run("長さ不一致は短い方に合わせる(欠測・防御的)", func(t *testing.T) {
		got := DewPointSeries([]float64{25, 30, 10}, []float64{50, 80})
		if len(got) != 2 {
			t.Errorf("len = %d, want 2（min(3,2)）", len(got))
		}
	})

	t.Run("空入力は空系列", func(t *testing.T) {
		if got := DewPointSeries(nil, nil); len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})

	t.Run("入力スライスを破壊しない", func(t *testing.T) {
		temps := []float64{25, 30}
		hums := []float64{50, 80}
		_ = DewPointSeries(temps, hums)
		if temps[0] != 25 || temps[1] != 30 || hums[0] != 50 || hums[1] != 80 {
			t.Errorf("入力スライスが破壊された: temps=%v hums=%v", temps, hums)
		}
	})
}

func TestDewPointSpread(t *testing.T) {
	t.Run("T−Td を返す", func(t *testing.T) {
		temps := []float64{25, 30}
		dews := []float64{13.86, 26.17}
		got := DewPointSpread(temps, dews)
		want := []float64{25 - 13.86, 30 - 26.17}
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		for i := range want {
			if math.Abs(got[i]-want[i]) > 1e-9 {
				t.Errorf("DewPointSpread[%d] = %v, want %v", i, got[i], want[i])
			}
		}
	})

	t.Run("T<Td でも 0 下限クランプ（全要素 ≥0）", func(t *testing.T) {
		temps := []float64{10, 20}
		dews := []float64{12, 18} // 10-12=-2 → 0、20-18=2
		got := DewPointSpread(temps, dews)
		for i, s := range got {
			if s < 0 {
				t.Errorf("DewPointSpread[%d] = %v, want ≥0（負はクランプ）", i, s)
			}
		}
		if math.Abs(got[0]-0) > 1e-9 {
			t.Errorf("DewPointSpread[0] = %v, want 0（クランプ）", got[0])
		}
		if math.Abs(got[1]-2) > 1e-9 {
			t.Errorf("DewPointSpread[1] = %v, want 2", got[1])
		}
	})

	t.Run("長さ不一致は短い方に合わせる", func(t *testing.T) {
		got := DewPointSpread([]float64{25, 30, 10}, []float64{13, 26})
		if len(got) != 2 {
			t.Errorf("len = %d, want 2", len(got))
		}
	})

	t.Run("空入力は空系列", func(t *testing.T) {
		if got := DewPointSpread(nil, nil); len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})
}

// assertRunsEqual は Run スライスの一致を検証し、不変条件 End≥Start も併せて確認する。
func assertRunsEqual(t *testing.T, got, want []Run) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("runs len = %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("runs[%d] = %+v, want %+v", i, got[i], want[i])
		}
		if got[i].EndIdx < got[i].StartIdx {
			t.Errorf("runs[%d] 不変条件違反 End(%d) < Start(%d)", i, got[i].EndIdx, got[i].StartIdx)
		}
	}
}

func TestCondensationRuns(t *testing.T) {
	const maxSpread = 2.0
	tests := []struct {
		name   string
		spread []float64
		want   []Run
	}{
		{"無結露(全て上限超)", []float64{3, 3, 3}, nil},
		{"境界値(=上限)は結露扱い", []float64{2, 2}, []Run{{0, 1}}},
		{"上限直上は非結露", []float64{2.0001}, nil},
		{"中間の連続区間", []float64{3, 2, 1, 3, 0, 0}, []Run{{1, 2}, {4, 5}}},
		{"先頭結露", []float64{1, 1, 3}, []Run{{0, 1}}},
		{"末尾結露", []float64{3, 1, 1}, []Run{{1, 2}}},
		{"全域結露(1区間に結合)", []float64{1, 1, 1}, []Run{{0, 2}}},
		{"単発も結露扱い(minRun=1)", []float64{3, 1, 3}, []Run{{1, 1}}},
		{"空入力は空スライス", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CondensationRuns(tt.spread, maxSpread)
			assertRunsEqual(t, got, tt.want)
		})
	}
}

func TestWetnessMask(t *testing.T) {
	const rhThreshold = 90.0
	tests := []struct {
		name string
		hums []float64
		want []bool
	}{
		{"しきい値境界(=しきい値は湿潤)", []float64{89.99, 90, 90.01}, []bool{false, true, true}},
		{"混在", []float64{85, 90, 95, 89}, []bool{false, true, true, false}},
		{"空入力は空スライス", nil, []bool{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WetnessMask(tt.hums, rhThreshold)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("WetnessMask[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}

	t.Run("入力スライスを破壊しない", func(t *testing.T) {
		hums := []float64{85, 95}
		_ = WetnessMask(hums, rhThreshold)
		if hums[0] != 85 || hums[1] != 95 {
			t.Errorf("入力が破壊された: %v", hums)
		}
	})
}

func TestHighHumidityRuns(t *testing.T) {
	const rhThreshold = 90.0
	tests := []struct {
		name   string
		hums   []float64
		minRun int
		want   []Run
	}{
		{"単発(minRun未満)は除外", []float64{95, 80, 95, 80}, 2, nil},
		{"最小継続ちょうどは抽出", []float64{95, 95, 80}, 2, []Run{{0, 1}}},
		{"最小継続未満は除外", []float64{95, 80, 80}, 2, nil},
		{"長短混在(短は除外・長は抽出)", []float64{95, 95, 85, 95, 95, 95, 80}, 3, []Run{{3, 5}}},
		{"境界RH(=しきい値)も継続に算入", []float64{90, 90, 90}, 3, []Run{{0, 2}}},
		{"全域高湿度(1区間)", []float64{95, 95, 95}, 1, []Run{{0, 2}}},
		{"末尾で終わる区間も抽出", []float64{80, 95, 95}, 2, []Run{{1, 2}}},
		{"minRun=0 は 1 に正規化(単発も抽出)", []float64{95, 80}, 0, []Run{{0, 0}}},
		{"空入力は空スライス", nil, 2, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HighHumidityRuns(tt.hums, rhThreshold, tt.minRun)
			assertRunsEqual(t, got, tt.want)
		})
	}
}

func TestDiseaseScore(t *testing.T) {
	const (
		tempLow  = 15.0
		tempHigh = 25.0
	)
	tests := []struct {
		name  string
		temps []float64
		wet   []bool
		want  float64
	}{
		{"空入力は0", nil, nil, 0},
		{"温度帯内×湿潤のみ寄与(2/4=0.5)", []float64{20, 20, 20, 20}, []bool{true, true, false, false}, 0.5},
		{"温度帯外は寄与なし(湿潤でも0)", []float64{30, 30}, []bool{true, true}, 0},
		{"非湿潤は寄与なし(帯内でも0)", []float64{20, 20}, []bool{false, false}, 0},
		{"温度帯境界(下限/上限)は帯内扱い→全寄与", []float64{15, 25}, []bool{true, true}, 1},
		{"温度帯直外は寄与なし", []float64{14.99, 25.01}, []bool{true, true}, 0},
		{"全点が帯内×湿潤→1", []float64{18, 20, 22}, []bool{true, true, true}, 1},
		{"長さ不一致は短い方(min)で評価", []float64{20, 20, 20}, []bool{true}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DiseaseScore(tt.temps, tt.wet, tempLow, tempHigh)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("DiseaseScore(%v,%v) = %v, want %v", tt.temps, tt.wet, got, tt.want)
			}
			if got < 0 || got > 1 {
				t.Errorf("DiseaseScore の事後条件 0≤score≤1 違反: %v", got)
			}
		})
	}

	t.Run("入力スライスを破壊しない", func(t *testing.T) {
		temps := []float64{20, 30}
		wet := []bool{true, true}
		_ = DiseaseScore(temps, wet, tempLow, tempHigh)
		if temps[0] != 20 || temps[1] != 30 || !wet[0] || !wet[1] {
			t.Errorf("入力が破壊された: temps=%v wet=%v", temps, wet)
		}
	})
}
