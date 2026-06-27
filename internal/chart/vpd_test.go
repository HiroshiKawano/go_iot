package chart

import (
	"math"
	"testing"
)

// vpdTol は VPD(kPa) の手計算一致を検証する許容差。
const vpdTol = 0.01

// es は Tetens 飽和水蒸気圧 es(T)[kPa]（テスト独立の参照実装）。
func es(t float64) float64 {
	return 0.6108 * math.Exp(17.27*t/(t+237.3))
}

func TestVPD_KnownValues(t *testing.T) {
	tests := []struct {
		name string
		temp float64
		rh   float64
		want float64
	}{
		{"25℃/50%≈1.58kPa", 25, 50, 1.58},
		{"30℃/80%≈0.85kPa", 30, 80, 0.85},
		{"10℃/100%→0kPa(飽差ゼロ)", 10, 100, 0},
		{"20℃/0%→es(20)(最大飽差)", 20, 0, es(20)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VPD(tt.temp, tt.rh)
			if math.Abs(got-tt.want) > vpdTol {
				t.Errorf("VPD(%v,%v) = %v, want %v (±%v)", tt.temp, tt.rh, got, tt.want, vpdTol)
			}
		})
	}
}

// RH=100% は飽差ゼロ（厳密に 0）であること（要件 1.3）。
func TestVPD_FullHumidityIsExactlyZero(t *testing.T) {
	for _, temp := range []float64{-5, 0, 10, 25, 40} {
		if got := VPD(temp, 100); got != 0 {
			t.Errorf("VPD(%v,100) = %v, want 厳密に 0", temp, got)
		}
	}
}

// RH=0% は es(T)（その温度での最大飽差）に一致すること（要件 1.4）。
func TestVPD_ZeroHumidityIsSaturation(t *testing.T) {
	for _, temp := range []float64{0, 10, 25, 40} {
		got := VPD(temp, 0)
		if math.Abs(got-es(temp)) > vpdTol {
			t.Errorf("VPD(%v,0) = %v, want es(%v)=%v", temp, got, temp, es(temp))
		}
	}
}

// 氷点下でも NaN/Inf を出さず正の飽差を返すこと（要件 1.5）。
func TestVPD_BelowFreezingNoNaNInf(t *testing.T) {
	for _, temp := range []float64{-40, -10, -5, -0.5} {
		got := VPD(temp, 50)
		if math.IsNaN(got) || math.IsInf(got, 0) {
			t.Errorf("VPD(%v,50) = %v（NaN/Inf を出してはいけない）", temp, got)
		}
		if got <= 0 {
			t.Errorf("VPD(%v,50) = %v, want 正の飽差", temp, got)
		}
	}
}

// RH は [0,100] にクランプされること（CHECK 保証だが防御的・design）。
func TestVPD_ClampsHumidity(t *testing.T) {
	// RH>100 は 100 として VPD=0。
	if got := VPD(25, 150); got != 0 {
		t.Errorf("VPD(25,150) = %v, want 0（RH>100 を 100 にクランプ）", got)
	}
	// RH<0 は 0 として VPD=es(T)。
	got := VPD(25, -20)
	if math.Abs(got-es(25)) > vpdTol {
		t.Errorf("VPD(25,-20) = %v, want es(25)=%v（RH<0 を 0 にクランプ）", got, es(25))
	}
}

func TestVPDSeries(t *testing.T) {
	t.Run("同長スライスから各点 VPD を返す", func(t *testing.T) {
		temps := []float64{25, 30, 10}
		hums := []float64{50, 80, 100}
		got := VPDSeries(temps, hums)
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
		want := []float64{VPD(25, 50), VPD(30, 80), VPD(10, 100)}
		for i := range want {
			if math.Abs(got[i]-want[i]) > 1e-9 {
				t.Errorf("VPDSeries[%d] = %v, want %v", i, got[i], want[i])
			}
		}
	})

	t.Run("長さ不一致は短い方に合わせる(欠測・防御的)", func(t *testing.T) {
		got := VPDSeries([]float64{25, 30, 10}, []float64{50, 80})
		if len(got) != 2 {
			t.Errorf("len = %d, want 2（min(3,2)）", len(got))
		}
	})

	t.Run("空入力は空系列", func(t *testing.T) {
		if got := VPDSeries(nil, nil); len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})
}

func TestTimeInRange(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		lower  float64
		upper  float64
		want   float64
	}{
		{"空入力は0", nil, 0.3, 1.5, 0},
		{"全て在帯→1", []float64{0.4, 0.8, 1.2}, 0.3, 1.5, 1},
		{"全て逸脱→0", []float64{0.1, 0.2, 2.0}, 0.3, 1.5, 0},
		{"半分在帯→0.5", []float64{0.1, 0.4, 2.0, 0.8}, 0.3, 1.5, 0.5},
		{"両端含む(下限ちょうど在帯)", []float64{0.3}, 0.3, 1.5, 1},
		{"両端含む(上限ちょうど在帯)", []float64{1.5}, 0.3, 1.5, 1},
		{"下限直下は逸脱", []float64{0.29}, 0.3, 1.5, 0},
		{"上限直上は逸脱", []float64{1.51}, 0.3, 1.5, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TimeInRange(tt.values, tt.lower, tt.upper)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("TimeInRange(%v,%v,%v) = %v, want %v", tt.values, tt.lower, tt.upper, got, tt.want)
			}
			if got < 0 || got > 1 {
				t.Errorf("TimeInRange の事後条件 0<=r<=1 違反: %v", got)
			}
		})
	}
}
