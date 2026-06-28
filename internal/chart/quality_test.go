package chart

import (
	"math"
	"testing"
)

// boolSliceEqual は2つの bool スライスが長さ・各要素ともに一致するか判定する。
func boolSliceEqual(a, b []bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---- 1.1 MissingStats -------------------------------------------------------

func TestMissingStats(t *testing.T) {
	tests := []struct {
		name         string
		intervalSecs []float64
		wantRate     float64
		wantMissing  int
		wantGaps     []GapSpan
		wantOK       bool
	}{
		{
			// 等間隔（5 readings = 4 intervals）: 中央値300・全 round1 → 欠測0・率0。
			name:         "等間隔は率0・ギャップ無し",
			intervalSecs: []float64{300, 300, 300, 300},
			wantRate:     0,
			wantMissing:  0,
			wantGaps:     nil,
			wantOK:       true,
		},
		{
			// 中央の1区間が900秒（中央値300の3倍）→ 欠測2スロット・ギャップ1件。
			// actualSamples=5, expected=5+2=7 → rate=2/7*100=28.571%。
			name:         "抜け1区間でギャップ1件・欠測本数2",
			intervalSecs: []float64{300, 300, 900, 300},
			wantRate:     2.0 / 7.0 * 100,
			wantMissing:  2,
			wantGaps:     []GapSpan{{StartIdx: 2, EndIdx: 3, MissingSlots: 2}},
			wantOK:       true,
		},
		{
			// 先頭区間が欠測（先頭ギャップを破綻なく扱う）。中央値300・round(900/300)=3 → 欠測2。
			name:         "先頭欠測",
			intervalSecs: []float64{900, 300, 300},
			wantRate:     2.0 / 6.0 * 100,
			wantMissing:  2,
			wantGaps:     []GapSpan{{StartIdx: 0, EndIdx: 1, MissingSlots: 2}},
			wantOK:       true,
		},
		{
			// 末尾区間が欠測（末尾ギャップを破綻なく扱う）。
			name:         "末尾欠測",
			intervalSecs: []float64{300, 300, 900},
			wantRate:     2.0 / 6.0 * 100,
			wantMissing:  2,
			wantGaps:     []GapSpan{{StartIdx: 2, EndIdx: 3, MissingSlots: 2}},
			wantOK:       true,
		},
		{
			// 複数ギャップ（中央値300・600=2スロット相当→欠測1, 1200=4→欠測3）。
			name:         "複数ギャップ",
			intervalSecs: []float64{300, 600, 300, 1200, 300},
			wantRate:     4.0 / 10.0 * 100, // 欠測4 / (6+4)
			wantMissing:  4,
			wantGaps: []GapSpan{
				{StartIdx: 1, EndIdx: 2, MissingSlots: 1},
				{StartIdx: 3, EndIdx: 4, MissingSlots: 3},
			},
			wantOK: true,
		},
		{
			name:         "要素1は未定義(ok=false)",
			intervalSecs: []float64{300},
			wantOK:       false,
		},
		{
			name:         "空は未定義(ok=false)",
			intervalSecs: []float64{},
			wantOK:       false,
		},
		{
			// 全間隔0（同一時刻が並ぶ）→ 中央値0 → 算出不能。
			name:         "中央値0は未定義(ok=false)",
			intervalSecs: []float64{0, 0, 0},
			wantOK:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRate, gotMissing, gotGaps, gotOK := MissingStats(tt.intervalSecs)
			if gotOK != tt.wantOK {
				t.Fatalf("ok=%v, want %v", gotOK, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if math.Abs(gotRate-tt.wantRate) > floatTol {
				t.Errorf("rate=%v, want %v", gotRate, tt.wantRate)
			}
			if gotMissing != tt.wantMissing {
				t.Errorf("missingCount=%d, want %d", gotMissing, tt.wantMissing)
			}
			if !gapSpansEqual(gotGaps, tt.wantGaps) {
				t.Errorf("gaps=%+v, want %+v", gotGaps, tt.wantGaps)
			}
		})
	}
}

func TestMissingStats_DoesNotMutateInput(t *testing.T) {
	in := []float64{300, 900, 300}
	cp := append([]float64(nil), in...)
	MissingStats(in)
	if !floatSliceEqual(in, cp) {
		t.Errorf("入力が破壊された: got %v, want %v", in, cp)
	}
}

// gapSpansEqual は GapSpan スライスの一致を判定する（空と nil は同一視）。
func gapSpansEqual(a, b []GapSpan) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---- 1.1 IntervalConsistency ------------------------------------------------

func TestIntervalConsistency(t *testing.T) {
	const eps = 1e-9
	tests := []struct {
		name         string
		intervalSecs []float64
		wantCV       float64
		wantOK       bool
	}{
		{"等間隔はCV0", []float64{300, 300, 300}, 0, true},
		{"σ/μ=2/5=0.4", []float64{2, 4, 4, 4, 5, 5, 7, 9}, 0.4, true},
		{"要素1は未定義", []float64{300}, 0, false},
		{"空は未定義", []float64{}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCV, gotOK := IntervalConsistency(tt.intervalSecs, eps)
			if gotOK != tt.wantOK {
				t.Fatalf("ok=%v, want %v", gotOK, tt.wantOK)
			}
			if tt.wantOK && math.Abs(gotCV-tt.wantCV) > floatTol {
				t.Errorf("cv=%v, want %v", gotCV, tt.wantCV)
			}
		})
	}
}

// ---- 1.2 RollingOutliers ----------------------------------------------------

func TestRollingOutliers(t *testing.T) {
	const (
		window = 12
		k      = 3.0
		eps    = 1e-9
	)
	t.Run("定常列(σ≈0)は全false", func(t *testing.T) {
		values := []float64{5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5}
		got := RollingOutliers(values, window, k, eps)
		if len(got) != len(values) {
			t.Fatalf("len=%d, want %d", len(got), len(values))
		}
		for i, b := range got {
			if b {
				t.Errorf("index %d=true, want false（σ≈0）", i)
			}
		}
	})

	t.Run("緩やかな昼夜変動は誤検出ゼロ", func(t *testing.T) {
		// 1点ずつ緩やかに上下する列。窓が追従するため外れ値は出ない。
		values := []float64{20, 21, 22, 23, 24, 25, 24, 23, 22, 21, 20, 21, 22, 23, 24}
		got := RollingOutliers(values, 5, k, eps)
		for i, b := range got {
			if b {
				t.Errorf("index %d=true, want false（緩やかな変動を誤検出）", i)
			}
		}
	})

	t.Run("末尾の明確なスパイクのみ検出・warm-upは全false", func(t *testing.T) {
		// 10 が 11 点続いた後 25 へスパイク。window=12 ゆえ index11 のみ窓が満ちる。
		values := []float64{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 25}
		got := RollingOutliers(values, window, k, eps)
		for i := 0; i < 11; i++ {
			if got[i] {
				t.Errorf("warm-up index %d=true, want false", i)
			}
		}
		if !got[11] {
			t.Errorf("index 11（スパイク）=false, want true")
		}
	})

	t.Run("空入力は空", func(t *testing.T) {
		if got := RollingOutliers([]float64{}, window, k, eps); len(got) != 0 {
			t.Errorf("len=%d, want 0", len(got))
		}
	})

	t.Run("単一点は[false]", func(t *testing.T) {
		got := RollingOutliers([]float64{42}, window, k, eps)
		if len(got) != 1 || got[0] {
			t.Errorf("got=%v, want [false]", got)
		}
	})
}

// ---- 1.2 ZScores ------------------------------------------------------------

func TestZScores(t *testing.T) {
	t.Run("μ5・σ2の標準化", func(t *testing.T) {
		// [2,4,4,4,5,5,7,9] は μ5・σ2 → z=(x-5)/2。
		got := ZScores([]float64{2, 4, 4, 4, 5, 5, 7, 9})
		want := []float64{-1.5, -0.5, -0.5, -0.5, 0, 0, 1, 2}
		if !floatSliceEqual(got, want) {
			t.Errorf("got=%v, want %v", got, want)
		}
	})
	t.Run("定常列(σ0)は全0", func(t *testing.T) {
		got := ZScores([]float64{5, 5, 5})
		if !floatSliceEqual(got, []float64{0, 0, 0}) {
			t.Errorf("got=%v, want [0 0 0]", got)
		}
	})
	t.Run("空は空", func(t *testing.T) {
		if got := ZScores([]float64{}); len(got) != 0 {
			t.Errorf("len=%d, want 0", len(got))
		}
	})
}

// ---- 1.2 IQRBounds ----------------------------------------------------------

func TestIQRBounds(t *testing.T) {
	t.Run("線形補間でQ1=2・Q3=4・coef1.5", func(t *testing.T) {
		// [1,2,3,4,5]: Q1(pos1)=2, Q3(pos3)=4, IQR=2 → lower=-1, upper=7。
		lower, upper, ok := IQRBounds([]float64{1, 2, 3, 4, 5}, 1.5)
		if !ok {
			t.Fatalf("ok=false, want true")
		}
		if math.Abs(lower-(-1)) > floatTol || math.Abs(upper-7) > floatTol {
			t.Errorf("lower=%v upper=%v, want -1, 7", lower, upper)
		}
	})
	t.Run("順不同入力でも同結果", func(t *testing.T) {
		lower, upper, ok := IQRBounds([]float64{3, 5, 1, 4, 2}, 1.5)
		if !ok || math.Abs(lower-(-1)) > floatTol || math.Abs(upper-7) > floatTol {
			t.Errorf("lower=%v upper=%v ok=%v, want -1,7,true", lower, upper, ok)
		}
	})
	t.Run("分位点が要素間に来る場合は線形補間", func(t *testing.T) {
		// [1,2,3,4]: Q1(pos0.75)=1.75, Q3(pos2.25)=3.25, IQR=1.5, coef1.5 → lower=-0.5, upper=5.5。
		lower, upper, ok := IQRBounds([]float64{1, 2, 3, 4}, 1.5)
		if !ok || math.Abs(lower-(-0.5)) > floatTol || math.Abs(upper-5.5) > floatTol {
			t.Errorf("lower=%v upper=%v ok=%v, want -0.5,5.5,true", lower, upper, ok)
		}
	})
	t.Run("要素1は未定義", func(t *testing.T) {
		if _, _, ok := IQRBounds([]float64{5}, 1.5); ok {
			t.Errorf("ok=true, want false")
		}
	})
	t.Run("空は未定義", func(t *testing.T) {
		if _, _, ok := IQRBounds([]float64{}, 1.5); ok {
			t.Errorf("ok=true, want false")
		}
	})
	t.Run("入力を破壊しない", func(t *testing.T) {
		in := []float64{3, 5, 1, 4, 2}
		cp := append([]float64(nil), in...)
		IQRBounds(in, 1.5)
		if !floatSliceEqual(in, cp) {
			t.Errorf("入力が破壊された: got %v, want %v", in, cp)
		}
	})
}

// ---- 1.3 StuckRuns ----------------------------------------------------------

func TestStuckRuns(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		minRun int
		want   []bool
	}{
		{
			name:   "全同値・run長6でminRun6は全true",
			values: []float64{5, 5, 5, 5, 5, 5},
			minRun: 6,
			want:   []bool{true, true, true, true, true, true},
		},
		{
			name:   "run長5はminRun6に満たず全false（N境界）",
			values: []float64{5, 5, 5, 5, 5},
			minRun: 6,
			want:   []bool{false, false, false, false, false},
		},
		{
			name:   "中央の3連続のみ検出",
			values: []float64{1, 2, 2, 2, 3},
			minRun: 3,
			want:   []bool{false, true, true, true, false},
		},
		{
			name:   "2小数まで完全同値で判定（微小差は同値）",
			values: []float64{1.001, 1.002, 1.004},
			minRun: 3,
			want:   []bool{true, true, true},
		},
		{
			name:   "末尾runがminRun未満なら立てない",
			values: []float64{2, 2, 2, 1},
			minRun: 3,
			want:   []bool{true, true, true, false},
		},
		{
			// minRun<1 は 1 として安全に扱う（各点が長さ1のランで全 true に退化）。
			name:   "minRun0は1扱い",
			values: []float64{1, 2, 3},
			minRun: 0,
			want:   []bool{true, true, true},
		},
		{
			name:   "空入力は空",
			values: []float64{},
			minRun: 3,
			want:   []bool{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StuckRuns(tt.values, tt.minRun)
			if len(got) != len(tt.values) {
				t.Fatalf("len=%d, want %d", len(got), len(tt.values))
			}
			if !boolSliceEqual(got, tt.want) {
				t.Errorf("got=%v, want %v", got, tt.want)
			}
		})
	}
}

// ---- 1.3 PhysicalAnomalies --------------------------------------------------

func TestPhysicalAnomalies(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		min, max float64
		want     []bool
	}{
		{
			// 境界値（==min/==max）は範囲内＝false。外側のみ true。
			name:   "温度範囲[-10,60]の境界と外側",
			values: []float64{-10, 0, 60, 61, -11},
			min:    TempPhysicalMin,
			max:    TempPhysicalMax,
			want:   []bool{false, false, false, true, true},
		},
		{
			name:   "空入力は空",
			values: []float64{},
			min:    -10,
			max:    60,
			want:   []bool{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PhysicalAnomalies(tt.values, tt.min, tt.max)
			if len(got) != len(tt.values) {
				t.Fatalf("len=%d, want %d", len(got), len(tt.values))
			}
			if !boolSliceEqual(got, tt.want) {
				t.Errorf("got=%v, want %v", got, tt.want)
			}
		})
	}
}

// ---- 1.3 RapidChanges -------------------------------------------------------

func TestRapidChanges(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		maxDelta float64
		want     []bool
	}{
		{
			// |Δ| が maxDelta を超える後側の点を立てる。先頭点は常に false。
			name:     "急変点のみ後側を立てる",
			values:   []float64{20, 21, 35, 36},
			maxDelta: 10,
			want:     []bool{false, false, true, false},
		},
		{
			// ちょうど maxDelta は超過でない（strict >）→ false。
			name:     "境界ちょうどは立てない",
			values:   []float64{20, 30},
			maxDelta: 10,
			want:     []bool{false, false},
		},
		{
			name:     "境界をわずかに超えたら立てる",
			values:   []float64{20, 30.1},
			maxDelta: 10,
			want:     []bool{false, true},
		},
		{
			name:     "単一点は[false]",
			values:   []float64{5},
			maxDelta: 10,
			want:     []bool{false},
		},
		{
			name:     "空入力は空",
			values:   []float64{},
			maxDelta: 10,
			want:     []bool{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RapidChanges(tt.values, tt.maxDelta)
			if len(got) != len(tt.values) {
				t.Fatalf("len=%d, want %d", len(got), len(tt.values))
			}
			if !boolSliceEqual(got, tt.want) {
				t.Errorf("got=%v, want %v", got, tt.want)
			}
		})
	}
}
