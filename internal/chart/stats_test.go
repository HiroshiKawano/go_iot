package chart

import (
	"math"
	"testing"
)

// floatTol は float 比較の許容誤差（pgconv_test.go と同水準）。
const floatTol = 0.001

// floatSliceEqual は2つの float スライスが長さ・各要素（許容誤差内）ともに一致するか判定する。
func floatSliceEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i]-b[i]) > floatTol {
			return false
		}
	}
	return true
}

// ---- 2.1 SMA ----------------------------------------------------------------

func TestSMA(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		window int
		want   []float64
	}{
		{
			// 窓幅3。先頭2点は expanding（部分平均）、3点目以降は trailing 窓の平均。
			// i0:[1]=1, i1:[1,2]=1.5, i2:[1,2,3]=2, i3:[2,3,4]=3, i4:[3,4,5]=4
			name:   "既知系列・窓幅3（立ち上がり込み）",
			values: []float64{1, 2, 3, 4, 5},
			window: 3,
			want:   []float64{1, 1.5, 2, 3, 4},
		},
		{
			// 窓幅1は各点そのもの。
			name:   "窓幅1は恒等",
			values: []float64{5, 6, 7},
			window: 1,
			want:   []float64{5, 6, 7},
		},
		{
			// 窓 > 系列長: 全点が expanding（=累積平均）に退化し panic しない。
			// i0:[2]=2, i1:[2,4]=3
			name:   "窓>系列長は累積平均へ退化",
			values: []float64{2, 4},
			window: 5,
			want:   []float64{2, 3},
		},
		{
			name:   "単一点",
			values: []float64{7},
			window: 3,
			want:   []float64{7},
		},
		{
			name:   "空入力は空",
			values: []float64{},
			window: 3,
			want:   []float64{},
		},
		{
			// 事前条件 window>=1 を満たさない場合も 1 として安全に扱う。
			name:   "window<1 は1扱いで恒等",
			values: []float64{3, 9},
			window: 0,
			want:   []float64{3, 9},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SMA(tt.values, tt.window)
			if len(got) != len(tt.values) {
				t.Fatalf("len(out)=%d, want %d（事後条件 len(out)==len(values)）", len(got), len(tt.values))
			}
			if !floatSliceEqual(got, tt.want) {
				t.Errorf("SMA(%v, %d) = %v, want %v", tt.values, tt.window, got, tt.want)
			}
		})
	}
}

// ---- 2.1 MovingStdDev --------------------------------------------------------

func TestMovingStdDev(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		window int
		want   []float64
	}{
		{
			// 母標準偏差（N 除算）。i0:[1]=σ0, i1:[1,3]: mean2,var(1+1)/2=1,σ=1
			name:   "立ち上がり: 単一点σ=0 → 2点σ",
			values: []float64{1, 3},
			window: 2,
			want:   []float64{0, 1},
		},
		{
			// 古典例: [2,4,4,4,5,5,7,9] の母σ=2（mean5・分散4）。窓8で末尾は全体σ。
			// i0:[2]=0, i1:[2,4]mean3 var1 σ1, ...（末尾 i7 が 2 になることを主眼に）
			name:   "既知系列・窓8で末尾が母σ=2",
			values: []float64{2, 4, 4, 4, 5, 5, 7, 9},
			window: 8,
			want:   []float64{0, 1, 0.942809, 0.866025, 0.979796, 1.0, 1.399708, 2},
		},
		{
			name:   "単一点はσ=0",
			values: []float64{5},
			window: 3,
			want:   []float64{0},
		},
		{
			// 事前条件 window>=1 を満たさない場合も 1 として安全に扱う（各点が単一窓 → σ=0）。
			name:   "window<1 は1扱いで全点σ=0",
			values: []float64{1, 3},
			window: 0,
			want:   []float64{0, 0},
		},
		{
			name:   "空入力は空",
			values: []float64{},
			window: 3,
			want:   []float64{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MovingStdDev(tt.values, tt.window)
			if len(got) != len(tt.values) {
				t.Fatalf("len(out)=%d, want %d", len(got), len(tt.values))
			}
			if !floatSliceEqual(got, tt.want) {
				t.Errorf("MovingStdDev(%v, %d) = %v, want %v", tt.values, tt.window, got, tt.want)
			}
		})
	}
}

// ---- 2.2 Band ---------------------------------------------------------------

func TestBand(t *testing.T) {
	tests := []struct {
		name      string
		sma       []float64
		sigma     []float64
		k         float64
		wantLower []float64
		wantWidth []float64
	}{
		{
			// k=2: 下限=sma-2σ、帯幅=4σ
			name:      "k=2の下限と帯幅",
			sma:       []float64{5, 5},
			sigma:     []float64{1, 2},
			k:         2,
			wantLower: []float64{3, 1},
			wantWidth: []float64{4, 8},
		},
		{
			// σ=0 の点は帯幅0（帯が SMA に収束）
			name:      "σ=0は帯幅0",
			sma:       []float64{10},
			sigma:     []float64{0},
			k:         2,
			wantLower: []float64{10},
			wantWidth: []float64{0},
		},
		{
			name:      "空入力は空",
			sma:       []float64{},
			sigma:     []float64{},
			k:         2,
			wantLower: []float64{},
			wantWidth: []float64{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lower, width := Band(tt.sma, tt.sigma, tt.k)
			if !floatSliceEqual(lower, tt.wantLower) {
				t.Errorf("Band lower = %v, want %v", lower, tt.wantLower)
			}
			if !floatSliceEqual(width, tt.wantWidth) {
				t.Errorf("Band width = %v, want %v", width, tt.wantWidth)
			}
		})
	}
}

// ---- 2.2 Deviation ----------------------------------------------------------

func TestDeviation(t *testing.T) {
	const eps = 1e-9

	t.Run("乖離率の定義どおり算出", func(t *testing.T) {
		// (110-100)/100*100=10, (90-100)/100*100=-10
		got := Deviation([]float64{110, 90}, []float64{100, 100}, eps)
		if len(got) != 2 {
			t.Fatalf("len=%d, want 2", len(got))
		}
		if got[0] == nil || math.Abs(*got[0]-10) > floatTol {
			t.Errorf("got[0]=%v, want 10", deref(got[0]))
		}
		if got[1] == nil || math.Abs(*got[1]-(-10)) > floatTol {
			t.Errorf("got[1]=%v, want -10", deref(got[1]))
		}
	})

	t.Run("SMA0近傍はnil（ゼロ除算ガード・温度0℃近傍）", func(t *testing.T) {
		// sma が epsilon 未満 → 未定義(nil)。
		got := Deviation([]float64{5, 5}, []float64{0.0, 1e-12}, eps)
		if len(got) != 2 {
			t.Fatalf("len=%d, want 2", len(got))
		}
		if got[0] != nil {
			t.Errorf("got[0]=%v, want nil（|sma|<eps）", deref(got[0]))
		}
		if got[1] != nil {
			t.Errorf("got[1]=%v, want nil（|sma|<eps）", deref(got[1]))
		}
	})

	t.Run("乖離率0の点は0でnilでない", func(t *testing.T) {
		// (5-5)/5*100=0 は定義された値0（nil ではない）。
		got := Deviation([]float64{5}, []float64{5}, eps)
		if len(got) != 1 || got[0] == nil || math.Abs(*got[0]) > floatTol {
			t.Errorf("got=%v, want [0]", got)
		}
	})

	t.Run("空入力は空", func(t *testing.T) {
		got := Deviation([]float64{}, []float64{}, eps)
		if len(got) != 0 {
			t.Errorf("len=%d, want 0", len(got))
		}
	})
}

// deref はテスト出力用に *float64 を安全に文字列化するヘルパ。
func deref(p *float64) interface{} {
	if p == nil {
		return "nil"
	}
	return *p
}

// ---- 2.3 スカラ集計 ---------------------------------------------------------

func TestMean(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"通常", []float64{2, 4, 6}, 4},
		{"単一点", []float64{9}, 9},
		{"空は0", []float64{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Mean(tt.values); math.Abs(got-tt.want) > floatTol {
				t.Errorf("Mean(%v) = %v, want %v", tt.values, got, tt.want)
			}
		})
	}
}

func TestMinMax(t *testing.T) {
	tests := []struct {
		name             string
		values           []float64
		wantMin, wantMax float64
	}{
		{"通常（順不同）", []float64{3, 1, 2}, 1, 3},
		{"負値含む", []float64{-5, 0, -2}, -5, 0},
		{"単一点", []float64{7}, 7, 7},
		{"空は(0,0)", []float64{}, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax := MinMax(tt.values)
			if math.Abs(gotMin-tt.wantMin) > floatTol || math.Abs(gotMax-tt.wantMax) > floatTol {
				t.Errorf("MinMax(%v) = (%v,%v), want (%v,%v)", tt.values, gotMin, gotMax, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestDiurnalRange(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"最大-最小", []float64{3, 1, 2}, 2},
		{"単一点は0", []float64{5}, 0},
		{"空は0", []float64{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DiurnalRange(tt.values); math.Abs(got-tt.want) > floatTol {
				t.Errorf("DiurnalRange(%v) = %v, want %v", tt.values, got, tt.want)
			}
		})
	}
}

func TestStdDev(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"古典例の母σ=2", []float64{2, 4, 4, 4, 5, 5, 7, 9}, 2},
		{"単一点は0", []float64{5}, 0},
		{"空は0", []float64{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StdDev(tt.values); math.Abs(got-tt.want) > floatTol {
				t.Errorf("StdDev(%v) = %v, want %v", tt.values, got, tt.want)
			}
		})
	}
}

func TestCV(t *testing.T) {
	const eps = 1e-9
	tests := []struct {
		name   string
		values []float64
		wantCV float64
		wantOK bool
	}{
		{"σ/μ=2/5=0.4", []float64{2, 4, 4, 4, 5, 5, 7, 9}, 0.4, true},
		{"平均0近傍は未定義", []float64{-1, 1}, 0, false},
		{"空は未定義", []float64{}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCV, gotOK := CV(tt.values, eps)
			if gotOK != tt.wantOK {
				t.Fatalf("CV(%v) ok=%v, want %v", tt.values, gotOK, tt.wantOK)
			}
			if tt.wantOK && math.Abs(gotCV-tt.wantCV) > floatTol {
				t.Errorf("CV(%v) = %v, want %v", tt.values, gotCV, tt.wantCV)
			}
		})
	}
}

// ---- 2.1 LinearFit (最小二乗の線形回帰) ------------------------------------

func TestLinearFit(t *testing.T) {
	tests := []struct {
		name          string
		xs, ys        []float64
		wantSlope     float64
		wantIntercept float64
		wantOK        bool
	}{
		{
			// 完全な直線 y=2x+1 は傾き2・切片1 で厳密一致。
			name:          "完全直線 y=2x+1",
			xs:            []float64{0, 1, 2, 3},
			ys:            []float64{1, 3, 5, 7},
			wantSlope:     2,
			wantIntercept: 1,
			wantOK:        true,
		},
		{
			// 教科書の最小二乗例。x̄=3,ȳ=4・Σ(x-x̄)(y-ȳ)=6・Σ(x-x̄)²=10 → 傾き0.6・切片2.2。
			name:          "ノイズ込み既知例 傾き0.6/切片2.2",
			xs:            []float64{1, 2, 3, 4, 5},
			ys:            []float64{2, 4, 5, 4, 5},
			wantSlope:     0.6,
			wantIntercept: 2.2,
			wantOK:        true,
		},
		{
			// 右肩下がり: 傾き負も算出する（ForecastDaysToTarget 側で ok=false 判定に使う材料）。
			name:          "負の傾き y=-2x+6",
			xs:            []float64{0, 1, 2},
			ys:            []float64{6, 4, 2},
			wantSlope:     -2,
			wantIntercept: 6,
			wantOK:        true,
		},
		{
			// x が全て同値 → 分散0 で傾き定義不能 → ok=false。
			name:   "x分散0は ok=false",
			xs:     []float64{3, 3, 3},
			ys:     []float64{1, 2, 3},
			wantOK: false,
		},
		{
			name:   "単一点は ok=false",
			xs:     []float64{5},
			ys:     []float64{7},
			wantOK: false,
		},
		{
			name:   "空は ok=false",
			xs:     []float64{},
			ys:     []float64{},
			wantOK: false,
		},
		{
			// 長さ不一致は短い方(3点)に合わせて防御的に当てはめる。先頭3点は y=2x+1。
			name:          "長さ不一致は短い方で当てはめ",
			xs:            []float64{0, 1, 2, 3},
			ys:            []float64{1, 3, 5},
			wantSlope:     2,
			wantIntercept: 1,
			wantOK:        true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSlope, gotIntercept, gotOK := LinearFit(tt.xs, tt.ys)
			if gotOK != tt.wantOK {
				t.Fatalf("LinearFit(%v,%v) ok=%v, want %v", tt.xs, tt.ys, gotOK, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if math.Abs(gotSlope-tt.wantSlope) > floatTol {
				t.Errorf("slope = %v, want %v", gotSlope, tt.wantSlope)
			}
			if math.Abs(gotIntercept-tt.wantIntercept) > floatTol {
				t.Errorf("intercept = %v, want %v", gotIntercept, tt.wantIntercept)
			}
		})
	}
}

// TestLinearFit_NonDestructive は入力スライスを破壊しないことを保証する（純粋層の不変条件）。
func TestLinearFit_NonDestructive(t *testing.T) {
	xs := []float64{1, 2, 3}
	ys := []float64{2, 4, 6}
	xsCopy := append([]float64(nil), xs...)
	ysCopy := append([]float64(nil), ys...)
	LinearFit(xs, ys)
	if !floatSliceEqual(xs, xsCopy) || !floatSliceEqual(ys, ysCopy) {
		t.Errorf("LinearFit が入力を破壊した xs=%v ys=%v", xs, ys)
	}
}
