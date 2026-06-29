package chart

import (
	"math"
	"testing"
)

// trend.go（統計分析ページの純粋検定層）の golden テスト。
//
// 期待値はすべて外部参照実装を oracle として生成した（自前実装の自己採点を避ける）:
//   - MK / Sen / Seasonal MK / Hamed-Rao : pyMannKendall 1.4.3
//   - 標準正規 CDF                        : scipy.stats.norm.cdf
//   - 多重比較補正（BH/Bonferroni）        : statsmodels multipletests
//   - ラグ1自己相関                        : numpy（__acf 定義）
// 生成スクリプト: scratchpad/gen_golden.py（pymannkendall/scipy/statsmodels/numpy）。
//
// 誤りやすい箇所（タイ補正 Var(S)・Hamed-Rao 有効標本式・Sen 中央値）を数値一致で固定する。

// closeEnough は言語間の浮動小数点実装差を吸収する近接判定（abs+rel）。
// 同一閉形式（VarS 等）は本許容より遥かに近いが、Hamed-Rao の acf 合算など
// 演算順序が numpy と異なる箇所もあるため緩めに取る（アルゴリズム誤りは桁で外れる）。
func closeEnough(got, want, tol float64) bool {
	return math.Abs(got-want) <= tol+tol*math.Abs(want)
}

func assertClose(t *testing.T, label string, got, want, tol float64) {
	t.Helper()
	if !closeEnough(got, want, tol) {
		t.Errorf("%s = %v, want %v (tol=%v)", label, got, want, tol)
	}
}

// ---- 2.1 MannKendall（タイ補正 VarS・連続性補正 Z・両側 p）----------------------

func TestMannKendall_Golden(t *testing.T) {
	cases := []struct {
		name string
		xs   []float64
		s    int
		varS float64
		z    float64
		p    float64
	}{
		{
			// pyMannKendall original_test（タイなし・n=12）
			name: "no_ties",
			xs:   []float64{4.5, 5.1, 3.9, 6.2, 5.8, 7.1, 6.5, 8.0, 7.4, 9.2, 8.6, 10.1},
			s:    54,
			varS: 212.66666666666666,
			z:    3.634345051015831,
			p:    0.0002786876842340025,
		},
		{
			// pyMannKendall original_test（タイあり・n=10）
			name: "with_ties",
			xs:   []float64{5, 5, 6, 4, 6, 7, 5, 8, 7, 9},
			s:    26,
			varS: 119.33333333333333,
			z:    2.2885432413650753,
			p:    0.022105904942060883,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := MannKendall(tc.xs)
			if r.S != tc.s {
				t.Errorf("S = %d, want %d", r.S, tc.s)
			}
			if r.N != len(tc.xs) {
				t.Errorf("N = %d, want %d", r.N, len(tc.xs))
			}
			assertClose(t, "VarS", r.VarS, tc.varS, 1e-9)
			assertClose(t, "Z", r.Z, tc.z, 1e-9)
			assertClose(t, "PValue", r.PValue, tc.p, 1e-9)
		})
	}
}

func TestMannKendall_EmptyAndSingle(t *testing.T) {
	for _, xs := range [][]float64{nil, {}, {42.0}} {
		r := MannKendall(xs)
		if r.S != 0 || r.VarS != 0 || r.Z != 0 {
			t.Errorf("退化入力 %v: S/VarS/Z = %d/%v/%v, want 0/0/0", xs, r.S, r.VarS, r.Z)
		}
		if r.PValue != 1 {
			t.Errorf("退化入力 %v: PValue = %v, want 1（トレンド断定しない）", xs, r.PValue)
		}
	}
}

// 全同値（固着センサ等）は Var(S)=0・S=0 で Z=0・PValue=1（トレンド断定しない）。
func TestMannKendall_AllEqual(t *testing.T) {
	r := MannKendall([]float64{7, 7, 7, 7, 7})
	if r.S != 0 {
		t.Errorf("S = %d, want 0", r.S)
	}
	if r.VarS != 0 {
		t.Errorf("VarS = %v, want 0", r.VarS)
	}
	if r.Z != 0 || r.PValue != 1 {
		t.Errorf("Z/PValue = %v/%v, want 0/1", r.Z, r.PValue)
	}
}

// ---- 2.2 SensSlope（全ペア中央値・外れ値頑健）-----------------------------------

func TestSensSlope_Golden(t *testing.T) {
	cases := []struct {
		name      string
		xs        []float64
		slope     float64
		intercept float64
	}{
		// pyMannKendall sens_slope（original_test の slope/intercept と一致）
		{"no_ties", []float64{4.5, 5.1, 3.9, 6.2, 5.8, 7.1, 6.5, 8.0, 7.4, 9.2, 8.6, 10.1}, 0.5, 4.05},
		{"with_ties", []float64{5, 5, 6, 4, 6, 7, 5, 8, 7, 9}, 0.42857142857142855, 4.071428571428571},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := SensSlope(tc.xs)
			assertClose(t, "Slope", r.Slope, tc.slope, 1e-9)
			assertClose(t, "Intercept", r.Intercept, tc.intercept, 1e-9)
		})
	}
}

// 台風スパイク等の外れ値が混入しても Sen 傾き（全ペア中央値）は安定する。
// クリーン系列・スパイク系列とも Sen slope=2.0（OLS はスパイクで 2.54 に歪む）。
func TestSensSlope_OutlierRobust(t *testing.T) {
	clean := []float64{1, 3, 5, 7, 9, 11, 13, 15, 17, 19}
	spiked := []float64{1, 3, 5, 7, 9, 100, 13, 15, 17, 19} // 中央に外れ値

	rc := SensSlope(clean)
	rs := SensSlope(spiked)
	assertClose(t, "clean Slope", rc.Slope, 2.0, 1e-9)
	assertClose(t, "spiked Slope", rs.Slope, 2.0, 1e-9) // 外れ値混入でも中央値は不変

	// 参考対比: OLS はスパイクで明確に歪む（Sen の頑健性の裏付け）。
	if olsSlope, _, ok := LinearFit(indexAxis(len(spiked)), spiked); ok {
		if closeEnough(olsSlope, 2.0, 0.05) {
			t.Errorf("対比のはずの OLS slope=%v がスパイクで歪んでいない（テスト前提崩れ）", olsSlope)
		}
	}
}

func indexAxis(n int) []float64 {
	xs := make([]float64, n)
	for i := range xs {
		xs[i] = float64(i)
	}
	return xs
}

// ---- 2.3a Lag1Autocorr / EffectiveSampleSize ----------------------------------

func TestLag1Autocorr_Golden(t *testing.T) {
	// AR(1) phi=0.9 の決定的実現値（numpy 生成・6桁丸め）。r1 は __acf 定義。
	r1 := Lag1Autocorr(ar1Series)
	assertClose(t, "r1(AR1)", r1, 0.857625594070134, 1e-9)

	// 近似 i.i.d. 系列は r1≈0（負の弱相関）。
	r1iid := Lag1Autocorr(nearIIDSeries)
	assertClose(t, "r1(iid)", r1iid, -0.15742724907510483, 1e-9)
}

func TestEffectiveSampleSize_Golden(t *testing.T) {
	// 強自己相関 → N_eff が大幅縮小（60→5）。
	if got := EffectiveSampleSize(60, 0.857625594070134); got != 5 {
		t.Errorf("N_eff(AR1) = %d, want 5", got)
	}
	// 負相関 → N_eff は N を上回りうる（下限1のみ・上限なし＝design D6 留保しきい値用途）。
	if got := EffectiveSampleSize(30, -0.15742724907510483); got != 41 {
		t.Errorf("N_eff(iid) = %d, want 41", got)
	}
	// 完全相関（r1>=1）は下限1にクランプ。
	if got := EffectiveSampleSize(50, 1.0); got != 1 {
		t.Errorf("N_eff(r1=1) = %d, want 1", got)
	}
}

// ---- 2.3b SeasonalMannKendall（月別符号反転・季節サイクル）----------------------

// 季節サイクルが強く各月は年0.5の一定増。素の MK は季節振動に希釈され非有意だが、
// 季節 MK（Hirsch-Slack・月別 S 合算）は月内トレンドを分離して検出する。
func TestSeasonalMannKendall_Golden(t *testing.T) {
	// 素の MannKendall は非有意（pyMannKendall original_test: Z=0.9577, P=0.3382）。
	plain := MannKendall(seasonalFlat)
	if plain.S != 198 {
		t.Errorf("plain S = %d, want 198", plain.S)
	}
	assertClose(t, "plain Z", plain.Z, 0.9576656758227381, 1e-9)
	assertClose(t, "plain P", plain.PValue, 0.338231370166147, 1e-9)
	if plain.PValue <= 0.05 {
		t.Errorf("素の MK は非有意であるべき: P=%v", plain.PValue)
	}

	// 季節 MK は有意（pyMannKendall seasonal_test: S=180, VarS=340, Z=9.708, P≈0）。
	seas := SeasonalMannKendall(seasonalFlat, seasonalKeys)
	if seas.S != 180 {
		t.Errorf("seasonal S = %d, want 180", seas.S)
	}
	assertClose(t, "seasonal VarS", seas.VarS, 340.0, 1e-9)
	assertClose(t, "seasonal Z", seas.Z, 9.707637987384864, 1e-9)
	if seas.PValue >= 0.05 {
		t.Errorf("季節 MK は有意であるべき: P=%v", seas.PValue)
	}
}

// 退化入力（空・season 長不一致・各季節1点・全同値）でも破綻せず断定しない。
func TestSeasonalMannKendall_Degenerate(t *testing.T) {
	if r := SeasonalMannKendall(nil, nil); r.PValue != 1 || r.S != 0 {
		t.Errorf("空: S/PValue=%d/%v, want 0/1", r.S, r.PValue)
	}
	// season 長が xs と不一致 → 断定しない。
	if r := SeasonalMannKendall([]float64{1, 2, 3}, []int{0, 1}); r.PValue != 1 {
		t.Errorf("長さ不一致: PValue=%v, want 1", r.PValue)
	}
	// 各季節が1点のみ → 寄与なしで S=0・Var=0。
	if r := SeasonalMannKendall([]float64{1, 2, 3}, []int{0, 1, 2}); r.S != 0 || r.VarS != 0 {
		t.Errorf("各季節1点: S/VarS=%d/%v, want 0/0", r.S, r.VarS)
	}
}

func TestLag1Autocorr_Degenerate(t *testing.T) {
	if got := Lag1Autocorr([]float64{5}); got != 0 {
		t.Errorf("単一点: r1=%v, want 0", got)
	}
	if got := Lag1Autocorr([]float64{3, 3, 3, 3}); got != 0 {
		t.Errorf("全同値: r1=%v, want 0", got)
	}
}

func TestSensSlope_Degenerate(t *testing.T) {
	if r := SensSlope([]float64{9}); r.Slope != 0 || r.HasCI {
		t.Errorf("単一点: Slope/HasCI=%v/%v, want 0/false", r.Slope, r.HasCI)
	}
}

// ---- 2.4 HamedRaoModifiedMK（自己相関で VarS 補正）-----------------------------

func TestHamedRaoModifiedMK_Golden(t *testing.T) {
	// AR(1) 系列。原 MK は有意（VarS=24583, Z=-3.744, P=0.000181）だが、
	// Hamed-Rao 補正で VarS が膨張し（106086.6）有意性が低下する（Z=-1.802, P=0.0715）。
	hr := HamedRaoModifiedMK(ar1Series)
	if hr.S != -588 {
		t.Errorf("AR1 S = %d, want -588", hr.S)
	}
	assertClose(t, "AR1 corrected VarS", hr.VarS, 106086.60313928193, 1e-6)
	assertClose(t, "AR1 corrected Z", hr.Z, -1.802220101623875, 1e-6)
	assertClose(t, "AR1 corrected P", hr.PValue, 0.07151078327201699, 1e-6)

	// 明瞭トレンド + 弱自己相関の短系列（pyMannKendall hamed_rao: VarS=67.045, Z=9.526）。
	hr2 := HamedRaoModifiedMK([]float64{1, 2, 1.5, 3, 2.5, 4, 3.5, 5, 4.5, 6, 5.5, 7, 6.5, 8})
	if hr2.S != 79 {
		t.Errorf("hr2 S = %d, want 79", hr2.S)
	}
	assertClose(t, "hr2 corrected VarS", hr2.VarS, 67.04510073260073, 1e-6)
	assertClose(t, "hr2 corrected Z", hr2.Z, 9.526011004578889, 1e-6)
}

// ---- 2.5a normalCDF（scipy norm.cdf 一致・gonum 非依存検証）----------------------

func TestNormalCDF_Golden(t *testing.T) {
	cases := []struct {
		z, want float64
	}{
		{0.0, 0.5},
		{1.0, 0.8413447460685429},
		{1.959963984540054, 0.975},
		{-1.959963984540054, 0.025},
		{2.5758293035489004, 0.995},
		{-3.0, 0.0013498980316300933},
		{3.0, 0.9986501019683699},
	}
	for _, tc := range cases {
		got := normalCDF(tc.z)
		assertClose(t, "Phi", got, tc.want, 1e-12)
	}
}

// ---- 2.5b BlockBootstrapSenCI（seed 固定で決定的・R5.5）------------------------

func TestBlockBootstrapSenCI_Deterministic(t *testing.T) {
	// 明瞭な正トレンド＋交互ノイズ（残差が非ゼロ＝CI が実区間になる現実的データ）。
	xs := []float64{2, 1, 4, 3, 6, 5, 8, 7, 10, 9, 12, 11, 14, 13, 16, 15}
	lo1, up1 := BlockBootstrapSenCI(xs, 0, 2000, 0.05, 42)
	lo2, up2 := BlockBootstrapSenCI(xs, 0, 2000, 0.05, 42)

	// 同一 seed で完全再現（決定性が要件 R5.5）。
	if lo1 != lo2 || up1 != up2 {
		t.Errorf("同一 seed で非決定的: (%v,%v) != (%v,%v)", lo1, up1, lo2, up2)
	}
	// 区間の整合性。
	if lo1 > up1 {
		t.Errorf("lower(%v) > upper(%v)", lo1, up1)
	}
	// 明瞭な正トレンドゆえ CI は正側に寄る（下限 > 0）。
	if lo1 <= 0 {
		t.Errorf("正トレンドで lower=%v, want > 0", lo1)
	}
	// 点推定（Sen=2）が CI に含まれる。
	point := SensSlope(xs).Slope
	if point < lo1 || point > up1 {
		t.Errorf("点推定 %v が CI [%v, %v] の外", point, lo1, up1)
	}
}

func TestBlockBootstrapSenCI_DegenerateInput(t *testing.T) {
	// 要素不足は (0,0) を返し panic しない。
	if lo, up := BlockBootstrapSenCI([]float64{5}, 0, 100, 0.05, 1); lo != 0 || up != 0 {
		t.Errorf("単一点: (%v,%v), want (0,0)", lo, up)
	}
}

// ---- 2.5c 多重比較補正（statsmodels multipletests 一致）------------------------

func TestBenjaminiHochberg_Golden(t *testing.T) {
	// 古典的 BH(1995) 例（n=15）。FDR 0.05 で先頭4件棄却。
	pvals := []float64{0.0001, 0.0004, 0.0019, 0.0095, 0.0201, 0.0278, 0.0298, 0.0344, 0.0459, 0.324, 0.4262, 0.5719, 0.6528, 0.759, 1.0}
	want := []bool{true, true, true, true, false, false, false, false, false, false, false, false, false, false, false}
	assertBoolSlice(t, "BH", BenjaminiHochberg(pvals, 0.05), want)

	// 小規模対照（[0.01..0.05] は BH で全棄却）。
	assertBoolSlice(t, "BH2",
		BenjaminiHochberg([]float64{0.01, 0.02, 0.03, 0.04, 0.05}, 0.05),
		[]bool{true, true, true, true, true})
}

func TestBonferroni_Golden(t *testing.T) {
	pvals := []float64{0.0001, 0.0004, 0.0019, 0.0095, 0.0201, 0.0278, 0.0298, 0.0344, 0.0459, 0.324, 0.4262, 0.5719, 0.6528, 0.759, 1.0}
	want := []bool{true, true, true, false, false, false, false, false, false, false, false, false, false, false, false}
	assertBoolSlice(t, "Bonferroni", Bonferroni(pvals, 0.05), want)

	// 小規模対照（α/n=0.01 を満たすのは先頭のみ）。
	assertBoolSlice(t, "Bonferroni2",
		Bonferroni([]float64{0.01, 0.02, 0.03, 0.04, 0.05}, 0.05),
		[]bool{true, false, false, false, false})
}

func assertBoolSlice(t *testing.T, label string, got, want []bool) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len=%d, want %d", label, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d] = %v, want %v", label, i, got[i], want[i])
		}
	}
}
