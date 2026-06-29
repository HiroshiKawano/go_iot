package chart

import (
	"math"
	"testing"
)

// gdd_test.go は GDD 純関数層（gdd.go）の table-driven テスト。
// floatTol / floatSliceEqual は stats_test.go（同 package）の共有ヘルパを再利用する。

// ---- 2.2 DailyGDD (日次 GDD = max((Tmax+Tmin)/2 − Tbase, 0)) ----------------

func TestDailyGDD(t *testing.T) {
	tests := []struct {
		name       string
		tMax, tMin []float64
		tBase      float64
		want       []float64
	}{
		{
			// 設計の代表ケース: (30+20)/2−10 = 15。
			name: "代表ケース 平均25−Tbase10=15",
			tMax: []float64{30}, tMin: []float64{20}, tBase: 10,
			want: []float64{15},
		},
		{
			// 日平均がちょうど Tbase → 0（境界）。
			name: "平均=Tbase は0",
			tMax: []float64{12}, tMin: []float64{8}, tBase: 10,
			want: []float64{0},
		},
		{
			// 氷点下を含み日平均が Tbase 未満 → 負をゼロクランプ（負の累積を作らない）。
			name: "氷点下は0クランプ",
			tMax: []float64{2}, tMin: []float64{-4}, tBase: 10,
			want: []float64{0},
		},
		{
			// 複数日（生育日・停滞日・氷点下日）。
			name: "複数日 [15,0,0]",
			tMax: []float64{30, 12, 2}, tMin: []float64{20, 8, -4}, tBase: 10,
			want: []float64{15, 0, 0},
		},
		{
			// 全ゼロ（生育せず）= 全日 Tbase 未満。
			name: "全日 Tbase 未満は全0",
			tMax: []float64{5, 6}, tMin: []float64{0, 1}, tBase: 10,
			want: []float64{0, 0},
		},
		{
			name: "空入力は空",
			tMax: []float64{}, tMin: []float64{}, tBase: 10,
			want: []float64{},
		},
		{
			// 長さ不一致は短い方に合わせて防御的（DewPointSeries 同型）。
			name: "長さ不一致は短い方",
			tMax: []float64{30, 30}, tMin: []float64{20}, tBase: 10,
			want: []float64{15},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DailyGDD(tt.tMax, tt.tMin, tt.tBase)
			if !floatSliceEqual(got, tt.want) {
				t.Errorf("DailyGDD(%v,%v,%v) = %v, want %v", tt.tMax, tt.tMin, tt.tBase, got, tt.want)
			}
			// 事後条件: 各要素 >= 0。
			for i, v := range got {
				if v < 0 {
					t.Errorf("DailyGDD[%d]=%v が負（ゼロクランプ違反）", i, v)
				}
			}
		})
	}
}

// ---- 2.2 CumulativeGDD (前方累積和・単調非減少) -----------------------------

func TestCumulativeGDD(t *testing.T) {
	tests := []struct {
		name  string
		daily []float64
		want  []float64
	}{
		{
			name:  "前方累積和 [15,0,5]→[15,15,20]",
			daily: []float64{15, 0, 5},
			want:  []float64{15, 15, 20},
		},
		{
			name:  "一定増 [3,3,3]→[3,6,9]",
			daily: []float64{3, 3, 3},
			want:  []float64{3, 6, 9},
		},
		{
			name:  "単一日",
			daily: []float64{7},
			want:  []float64{7},
		},
		{
			name:  "空は空",
			daily: []float64{},
			want:  []float64{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CumulativeGDD(tt.daily)
			if !floatSliceEqual(got, tt.want) {
				t.Errorf("CumulativeGDD(%v) = %v, want %v", tt.daily, got, tt.want)
			}
			if len(got) != len(tt.daily) {
				t.Errorf("len(out)=%d, want %d", len(got), len(tt.daily))
			}
			// 事後条件: 単調非減少（daily が非負ゆえ）。
			for i := 1; i < len(got); i++ {
				if got[i] < got[i-1] {
					t.Errorf("単調非減少違反 out[%d]=%v < out[%d]=%v", i, got[i], i-1, got[i-1])
				}
			}
		})
	}
}

// ---- 2.2 RemainingGDD (max(target − 最新累積, 0)・空は target) ---------------

func TestRemainingGDD(t *testing.T) {
	tests := []struct {
		name       string
		cumulative []float64
		target     float64
		want       float64
	}{
		{
			name:       "到達前は残り target−最新",
			cumulative: []float64{15, 15, 20}, target: 100,
			want: 80,
		},
		{
			name:       "到達済みは0",
			cumulative: []float64{50, 120}, target: 100,
			want: 0,
		},
		{
			name:       "ちょうど到達は0（境界）",
			cumulative: []float64{100}, target: 100,
			want: 0,
		},
		{
			name:       "空入力は target（未開始）",
			cumulative: []float64{}, target: 500,
			want: 500,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RemainingGDD(tt.cumulative, tt.target)
			if math.Abs(got-tt.want) > floatTol {
				t.Errorf("RemainingGDD(%v,%v) = %v, want %v", tt.cumulative, tt.target, got, tt.want)
			}
		})
	}
}

// TestGDDPureFuncs_NonDestructive は DailyGDD/CumulativeGDD が入力を破壊しないことを保証する。
func TestGDDPureFuncs_NonDestructive(t *testing.T) {
	tMax := []float64{30, 12}
	tMin := []float64{20, 8}
	daily := []float64{15, 0, 5}
	tMaxCopy := append([]float64(nil), tMax...)
	tMinCopy := append([]float64(nil), tMin...)
	dailyCopy := append([]float64(nil), daily...)

	DailyGDD(tMax, tMin, 10)
	CumulativeGDD(daily)

	if !floatSliceEqual(tMax, tMaxCopy) || !floatSliceEqual(tMin, tMinCopy) {
		t.Errorf("DailyGDD が入力を破壊した tMax=%v tMin=%v", tMax, tMin)
	}
	if !floatSliceEqual(daily, dailyCopy) {
		t.Errorf("CumulativeGDD が入力を破壊した daily=%v", daily)
	}
}

// ---- 2.3 ForecastDaysToTarget (到達日外挿・LinearFit 利用) -------------------

func TestForecastDaysToTarget(t *testing.T) {
	tests := []struct {
		name           string
		xs, cumulative []float64
		target         float64
		wantDays       float64
		wantOK         bool
	}{
		{
			// 完全な直線 cum=10x（傾き10）。target=50 → 50/10=5日で到達。lastX=3 ≤ 5。
			name: "通常外挿 傾き10で5日",
			xs:   []float64{0, 1, 2, 3}, cumulative: []float64{0, 10, 20, 30}, target: 50,
			wantDays: 5, wantOK: true,
		},
		{
			// 全ゼロ（生育せず）= 傾き0 → ok=false。
			name: "傾き0は ok=false",
			xs:   []float64{0, 1, 2}, cumulative: []float64{0, 0, 0}, target: 100,
			wantOK: false,
		},
		{
			// 右肩下がり = 傾き負 → ok=false。
			name: "傾き負は ok=false",
			xs:   []float64{0, 1, 2}, cumulative: []float64{30, 20, 10}, target: 100,
			wantOK: false,
		},
		{
			// 既に最新累積 120 >= target 100（到達済み）→ ok=false。
			name: "到達済みは ok=false",
			xs:   []float64{0, 1, 2}, cumulative: []float64{0, 60, 120}, target: 100,
			wantOK: false,
		},
		{
			// 実質1点（len<2）→ LinearFit ok=false 経由で ok=false。
			name: "単一点は ok=false",
			xs:   []float64{0}, cumulative: []float64{5}, target: 100,
			wantOK: false,
		},
		{
			// 空入力（定植日以降データ0件）→ ok=false（防御）。
			name: "空入力は ok=false",
			xs:   []float64{}, cumulative: []float64{}, target: 100,
			wantOK: false,
		},
		{
			// x 分散0（同一経過日が2点）→ LinearFit ok=false 経由で ok=false。
			name: "x分散0は ok=false",
			xs:   []float64{2, 2}, cumulative: []float64{10, 20}, target: 100,
			wantOK: false,
		},
		{
			// 減速気味の累積に回帰直線が末尾で target を追い越し、外挿日が直近経過日(2)より過去(≈1.82)になる。
			// 「過去には外挿しない」事後条件ゆえ ok=false（捏造回避・到達済みの精神の防御的拡張）。
			name: "過去外挿は ok=false",
			xs:   []float64{0, 1, 2}, cumulative: []float64{0, 10, 11}, target: 11.5,
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDays, gotOK := ForecastDaysToTarget(tt.xs, tt.cumulative, tt.target)
			if gotOK != tt.wantOK {
				t.Fatalf("ForecastDaysToTarget(%v,%v,%v) ok=%v, want %v", tt.xs, tt.cumulative, tt.target, gotOK, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if math.Abs(gotDays-tt.wantDays) > floatTol {
				t.Errorf("days = %v, want %v", gotDays, tt.wantDays)
			}
			// 事後条件: ok のとき days >= 直近経過日（過去には外挿しない）。
			lastX := tt.xs[len(tt.xs)-1]
			if gotDays < lastX-floatTol {
				t.Errorf("days=%v が直近経過日 %v より過去（過去外挿違反）", gotDays, lastX)
			}
		})
	}
}

// ---- 2.3 GrowthStageIndex (昇順しきい値・cum >= stages[i] の最大 i) ----------

func TestGrowthStageIndex(t *testing.T) {
	// 生育ステージ閾値（昇順・最終段=収穫目標）の例。
	stages := []float64{0, 200, 800, 1400, 2000}
	tests := []struct {
		name   string
		cum    float64
		stages []float64
		want   int
	}{
		{"発芽境界 cum=0 は0段目", 0, stages, 0},
		{"第1段未満は0段目維持", 199, stages, 0},
		{"第1段ちょうど到達(境界)は1", 200, stages, 1},
		{"中間段", 900, stages, 2},
		{"最終段ちょうど(境界)は最終index", 2000, stages, 4},
		{"最終段超えも最終index", 2500, stages, 4},
		{"どの段にも未到達は-1", 50, []float64{100, 500}, -1},
		{"空しきい値は-1", 1234, []float64{}, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GrowthStageIndex(tt.cum, tt.stages)
			if got != tt.want {
				t.Errorf("GrowthStageIndex(%v,%v) = %d, want %d", tt.cum, tt.stages, got, tt.want)
			}
		})
	}
}
