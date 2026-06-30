package chart

import (
	"math"
	"reflect"
	"testing"
)

// 高温ストレス純関数（THI・絶対湿度 AH・熱帯夜マスク・連続ラン）の単体テスト。
// 純粋層ゆえ DB/time/gin 非依存・table-driven・既知の手計算値一致で検証する（要件 1, 2）。
//
// 参照実装 es（vpd_test.go・es(T)[kPa]）と許容差 vpdTol は同パッケージから再利用する
// （Tetens 定数を本テストでも重複定義しない）。

// thiTol は THI（多項式・厳密）の手計算一致を検証する許容差。
const thiTol = 1e-6

// ahTol は AH[g/m³] の手計算一致を検証する許容差。
const ahTol = 0.05

// ahRef は AH[g/m³] の参照実装（実装から独立・付録A D④）。
// ea は hPa（es[kPa]×10×RH/100）、AH = 217·ea[hPa]/(T+273.15)。
func ahRef(tempC, rh float64) float64 {
	if rh < 0 {
		rh = 0
	} else if rh > 100 {
		rh = 100
	}
	eaHPa := es(tempC) * 10 * (rh / 100)
	return 217 * eaHPa / (tempC + 273.15)
}

// ---- THI -------------------------------------------------------------------

func TestTHI_KnownValues(t *testing.T) {
	// THI = 0.8·T + (RH/100)·(T−14.4) + 46.4（付録A D⑥）。
	tests := []struct {
		name string
		temp float64
		rh   float64
		want float64
	}{
		{"25℃/60%", 25, 60, 72.76},   // 20 + 0.6*10.6 + 46.4
		{"30℃/70%", 30, 70, 81.32},   // 24 + 0.7*15.6 + 46.4
		{"28℃/50%", 28, 50, 75.6},    // 22.4 + 0.5*13.6 + 46.4
		{"14.4℃/任意は気温項で相殺", 14.4, 80, 0.8*14.4 + 46.4}, // (T−14.4)=0 ゆえ RH 項ゼロ
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := THI(tt.temp, tt.rh)
			if math.Abs(got-tt.want) > thiTol {
				t.Errorf("THI(%v,%v) = %v, want %v (±%v)", tt.temp, tt.rh, got, tt.want, thiTol)
			}
		})
	}
}

// RH は [0,100] にクランプする（要件 1.3）。
func TestTHI_ClampsRH(t *testing.T) {
	// RH=150 は 100 に丸め: 0.8*25 + 1.0*(25−14.4) + 46.4 = 77.0
	if got := THI(25, 150); math.Abs(got-77.0) > thiTol {
		t.Errorf("THI(25,150) = %v, want 77.0 (RH→100 にクランプ)", got)
	}
	// RH=-10 は 0 に丸め: 0.8*25 + 0 + 46.4 = 66.4
	if got := THI(25, -10); math.Abs(got-66.4) > thiTol {
		t.Errorf("THI(25,-10) = %v, want 66.4 (RH→0 にクランプ)", got)
	}
	// 境界 0/100 はそのまま反映される。
	if got := THI(25, 0); math.Abs(got-66.4) > thiTol {
		t.Errorf("THI(25,0) = %v, want 66.4", got)
	}
	if got := THI(25, 100); math.Abs(got-77.0) > thiTol {
		t.Errorf("THI(25,100) = %v, want 77.0", got)
	}
}

// 氷点下（−40℃）でも NaN/Inf を出さない（要件 1.4）。
func TestTHI_SubzeroIsFinite(t *testing.T) {
	for _, temp := range []float64{-40, -20, -5, 0} {
		got := THI(temp, 50)
		if math.IsNaN(got) || math.IsInf(got, 0) {
			t.Errorf("THI(%v,50) = %v, NaN/Inf であってはならない", temp, got)
		}
	}
}

func TestTHISeries(t *testing.T) {
	temps := []float64{25, 30, 28}
	hums := []float64{60, 70, 50}
	got := THISeries(temps, hums)
	want := []float64{72.76, 81.32, 75.6}
	if len(got) != len(want) {
		t.Fatalf("len(THISeries) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > thiTol {
			t.Errorf("THISeries[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

// 長さ不一致は短い方に合わせる（要件: len = min(len(temps),len(hums))）。
func TestTHISeries_MinLength(t *testing.T) {
	got := THISeries([]float64{25, 30, 28}, []float64{60, 70})
	if len(got) != 2 {
		t.Errorf("len(THISeries(3,2)) = %d, want 2", len(got))
	}
}

func TestTHISeries_DoesNotMutateInput(t *testing.T) {
	temps := []float64{25, 30}
	hums := []float64{60, 70}
	tCopy := append([]float64(nil), temps...)
	hCopy := append([]float64(nil), hums...)
	_ = THISeries(temps, hums)
	if !reflect.DeepEqual(temps, tCopy) || !reflect.DeepEqual(hums, hCopy) {
		t.Errorf("THISeries が入力スライスを破壊した: temps=%v hums=%v", temps, hums)
	}
}

func TestTHISeries_Empty(t *testing.T) {
	if got := THISeries(nil, nil); len(got) != 0 {
		t.Errorf("THISeries(nil,nil) = %v, want 空", got)
	}
}

// ---- 絶対湿度 AH -----------------------------------------------------------

func TestAbsoluteHumidity_KnownValues(t *testing.T) {
	// 既知の手計算（物理アンカー）: 20℃飽和は約17.3 g/m³（教科書値）。
	tests := []struct {
		name string
		temp float64
		rh   float64
		want float64
	}{
		{"20℃/100%≈17.3g/m³(飽和の教科書値)", 20, 100, 17.31},
		{"25℃/60%≈13.84g/m³", 25, 60, 13.84},
		{"30℃/70%≈21.27g/m³", 30, 70, 21.27},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AbsoluteHumidity(tt.temp, tt.rh)
			if math.Abs(got-tt.want) > ahTol {
				t.Errorf("AbsoluteHumidity(%v,%v) = %v, want %v (±%v)", tt.temp, tt.rh, got, tt.want, ahTol)
			}
		})
	}
}

// AbsoluteHumidity は saturationVaporPressure（vpd.go）を再利用し、参照式と一致する（要件 1.2）。
func TestAbsoluteHumidity_MatchesReference(t *testing.T) {
	for _, tc := range []struct{ temp, rh float64 }{
		{15, 40}, {22, 55}, {35, 90}, {5, 30},
	} {
		got := AbsoluteHumidity(tc.temp, tc.rh)
		want := ahRef(tc.temp, tc.rh)
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("AbsoluteHumidity(%v,%v) = %v, 参照 %v と一致しない（es 再利用の不一致）", tc.temp, tc.rh, got, want)
		}
	}
}

// RH=0 → AH=0（厳密に 0・要件 1.5）。
func TestAbsoluteHumidity_ZeroRHIsExactlyZero(t *testing.T) {
	for _, temp := range []float64{-40, 0, 25, 40} {
		if got := AbsoluteHumidity(temp, 0); got != 0 {
			t.Errorf("AbsoluteHumidity(%v,0) = %v, want 厳密に 0", temp, got)
		}
	}
}

// RH は [0,100] にクランプする（要件 1.3）。負の RH は 0 扱い→AH=0。
func TestAbsoluteHumidity_ClampsRH(t *testing.T) {
	if got := AbsoluteHumidity(25, -10); got != 0 {
		t.Errorf("AbsoluteHumidity(25,-10) = %v, want 0（RH→0 クランプ）", got)
	}
	// RH=150 は 100 にクランプ → RH=100 と同値。
	if got, want := AbsoluteHumidity(25, 150), AbsoluteHumidity(25, 100); math.Abs(got-want) > 1e-9 {
		t.Errorf("AbsoluteHumidity(25,150) = %v, want %v（RH→100 クランプ）", got, want)
	}
}

// 氷点下（−40℃）でも NaN/Inf/ゼロ割を出さず、AH は非負（要件 1.4）。
func TestAbsoluteHumidity_SubzeroIsFiniteNonNegative(t *testing.T) {
	for _, temp := range []float64{-40, -20, -5, 0} {
		got := AbsoluteHumidity(temp, 50)
		if math.IsNaN(got) || math.IsInf(got, 0) {
			t.Errorf("AbsoluteHumidity(%v,50) = %v, NaN/Inf であってはならない", temp, got)
		}
		if got < 0 {
			t.Errorf("AbsoluteHumidity(%v,50) = %v, 非負であるべき", temp, got)
		}
	}
}

func TestAbsoluteHumiditySeries(t *testing.T) {
	temps := []float64{20, 25, 30}
	hums := []float64{100, 60, 70}
	got := AbsoluteHumiditySeries(temps, hums)
	if len(got) != 3 {
		t.Fatalf("len(AbsoluteHumiditySeries) = %d, want 3", len(got))
	}
	for i := range temps {
		if want := ahRef(temps[i], hums[i]); math.Abs(got[i]-want) > 1e-9 {
			t.Errorf("AbsoluteHumiditySeries[%d] = %v, want %v", i, got[i], want)
		}
	}
}

func TestAbsoluteHumiditySeries_MinLength(t *testing.T) {
	got := AbsoluteHumiditySeries([]float64{20, 25, 30}, []float64{100, 60})
	if len(got) != 2 {
		t.Errorf("len(AbsoluteHumiditySeries(3,2)) = %d, want 2", len(got))
	}
}

func TestAbsoluteHumiditySeries_DoesNotMutateInput(t *testing.T) {
	temps := []float64{20, 25}
	hums := []float64{100, 60}
	tCopy := append([]float64(nil), temps...)
	hCopy := append([]float64(nil), hums...)
	_ = AbsoluteHumiditySeries(temps, hums)
	if !reflect.DeepEqual(temps, tCopy) || !reflect.DeepEqual(hums, hCopy) {
		t.Errorf("AbsoluteHumiditySeries が入力スライスを破壊した")
	}
}

// ---- 熱帯夜マスク ----------------------------------------------------------

func TestTropicalNightMask(t *testing.T) {
	tests := []struct {
		name      string
		temps     []float64
		threshold float64
		want      []bool
	}{
		{
			name:      "閾値ちょうどは該当・直下は非該当・NaN(欠測)は非該当",
			temps:     []float64{24.9, 25.0, 25.1, math.NaN(), 26.0},
			threshold: 25,
			want:      []bool{false, true, true, false, true},
		},
		{"全日該当", []float64{26, 27, 28}, 25, []bool{true, true, true}},
		{"全日非該当", []float64{20, 21, 22}, 25, []bool{false, false, false}},
		{"全 NaN は全非該当", []float64{math.NaN(), math.NaN()}, 25, []bool{false, false}},
		{"空入力は空", nil, 25, []bool{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TropicalNightMask(tt.temps, tt.threshold)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("mask[%d] = %v, want %v（temps=%v）", i, got[i], tt.want[i], tt.temps)
				}
			}
		})
	}
}

// 入力スライスを破壊しない。
func TestTropicalNightMask_DoesNotMutateInput(t *testing.T) {
	temps := []float64{24, 25, 26}
	tCopy := append([]float64(nil), temps...)
	_ = TropicalNightMask(temps, 25)
	if !reflect.DeepEqual(temps, tCopy) {
		t.Errorf("TropicalNightMask が入力を破壊した: %v", temps)
	}
}

// ---- 連続ラン（最長/現在） -------------------------------------------------

func TestRunStats(t *testing.T) {
	tests := []struct {
		name        string
		mask        []bool
		wantLongest int
		wantCurrent int
	}{
		{"空", nil, 0, 0},
		{"単発 true", []bool{true}, 1, 1},
		{"単発 false", []bool{false}, 0, 0},
		{"全 true", []bool{true, true, true}, 3, 3},
		{"全 false", []bool{false, false}, 0, 0},
		{"末尾で途切れる(現在=0)", []bool{true, true, true, false}, 3, 0},
		{"末尾が連続中(現在>0・最長は前半)", []bool{true, true, false, true}, 2, 1},
		{"末尾が最長", []bool{true, false, true, true, true}, 3, 3},
		{"複数ラン・現在は末尾ラン", []bool{false, true, true, false, true, true, true}, 3, 3},
		{"先頭最長・末尾短い", []bool{true, true, true, true, false, true}, 4, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotL, gotC := RunStats(tt.mask)
			if gotL != tt.wantLongest || gotC != tt.wantCurrent {
				t.Errorf("RunStats(%v) = (longest=%d,current=%d), want (%d,%d)",
					tt.mask, gotL, gotC, tt.wantLongest, tt.wantCurrent)
			}
		})
	}
}
