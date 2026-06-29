package domain

import "testing"

// vpdEpsilon は VPD 適正帯 (kPa) の浮動小数比較許容差。
const vpdEpsilon = 1e-9

func TestCrop_Label(t *testing.T) {
	tests := []struct {
		crop Crop
		want string
	}{
		{CropGoya, "ゴーヤ"},
		{CropIngen, "インゲン"},
		{CropSugarcane, "サトウキビ"},
		{CropMango, "マンゴー"},
		{CropPineapple, "パイナップル"},
		{CropUri, "ウリ"},
		{CropRice, "米"},
		{CropImo, "いも"},
		{CropLeafyVegetable, "葉野菜"},
	}
	for _, tt := range tests {
		t.Run(string(tt.crop), func(t *testing.T) {
			if got := tt.crop.Label(); got != tt.want {
				t.Errorf("Label() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCrop_Valid(t *testing.T) {
	tests := []struct {
		input Crop
		want  bool
	}{
		{CropGoya, true},
		{CropIngen, true},
		{CropSugarcane, true},
		{CropMango, true},
		{CropPineapple, true},
		{CropUri, true},
		{CropRice, true},
		{CropImo, true},
		{CropLeafyVegetable, true},
		{Crop("invalid"), false},
		{Crop(""), false},
		{Crop("GOYA"), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := tt.input.Valid(); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCrop_VPDRange(t *testing.T) {
	tests := []struct {
		name      string
		crop      Crop
		wantLower float64
		wantUpper float64
	}{
		// 施設果菜 (VPD 本命・暫定 0.4-1.2)
		{"goya 施設果菜", CropGoya, 0.4, 1.2},
		{"ingen 施設果菜", CropIngen, 0.4, 1.2},
		{"uri 施設果菜", CropUri, 0.4, 1.2},
		{"mango 施設果菜", CropMango, 0.4, 1.2},
		// 施設葉菜 (低め・暫定 0.3-1.0)
		{"leafy_vegetable 施設葉菜", CropLeafyVegetable, 0.3, 1.0},
		// 露地 (既定踏襲 0.3-1.5)
		{"sugarcane 露地=既定", CropSugarcane, DefaultVPDLower, DefaultVPDUpper},
		{"rice 露地=既定", CropRice, DefaultVPDLower, DefaultVPDUpper},
		{"pineapple 露地=既定", CropPineapple, DefaultVPDLower, DefaultVPDUpper},
		{"imo 露地=既定", CropImo, DefaultVPDLower, DefaultVPDUpper},
		// 未設定 (空) / 不正 → 既定フォールバック
		{"未設定(空)→既定", Crop(""), DefaultVPDLower, DefaultVPDUpper},
		{"不正値→既定", Crop("invalid"), DefaultVPDLower, DefaultVPDUpper},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lower, upper := tt.crop.VPDRange()
			if diff := lower - tt.wantLower; diff < -vpdEpsilon || diff > vpdEpsilon {
				t.Errorf("VPDRange() lower = %v, want %v", lower, tt.wantLower)
			}
			if diff := upper - tt.wantUpper; diff < -vpdEpsilon || diff > vpdEpsilon {
				t.Errorf("VPDRange() upper = %v, want %v", upper, tt.wantUpper)
			}
			if lower > upper {
				t.Errorf("VPDRange() lower %v > upper %v (不変条件違反)", lower, upper)
			}
		})
	}
}

// TestCrop_DefaultVPDRange は既定適正帯が要件どおり 0.3〜1.5 kPa であることを固定する。
func TestCrop_DefaultVPDRange(t *testing.T) {
	if DefaultVPDLower != 0.3 {
		t.Errorf("DefaultVPDLower = %v, want 0.3", DefaultVPDLower)
	}
	if DefaultVPDUpper != 1.5 {
		t.Errorf("DefaultVPDUpper = %v, want 1.5", DefaultVPDUpper)
	}
}

func TestAllCrops(t *testing.T) {
	crops := AllCrops()
	if len(crops) != 9 {
		t.Fatalf("AllCrops() len = %d, want 9", len(crops))
	}
	// 列挙はすべて有効値で、重複が無いこと。
	seen := make(map[Crop]bool, len(crops))
	for _, c := range crops {
		if !c.Valid() {
			t.Errorf("AllCrops() に無効な作物 %q が含まれる", c)
		}
		if seen[c] {
			t.Errorf("AllCrops() に重複 %q がある", c)
		}
		seen[c] = true
	}
	// 要件 2.1 の9作物が漏れなく含まれること。
	want := []Crop{
		CropGoya, CropIngen, CropSugarcane, CropMango,
		CropPineapple, CropUri, CropRice, CropImo, CropLeafyVegetable,
	}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("AllCrops() に %q が欠落", w)
		}
	}
}

// TestCrop_DiseaseModel は作物別の病害モデルしきい値を固定する（要件 5.4, 6.1, 6.2, 6.3）。
// 施設果菜=暫定値・施設葉菜=温度帯狭め・露地/未設定/不正は DefaultDiseaseModel にフォールバック。
func TestCrop_DiseaseModel(t *testing.T) {
	greenhouseFruit := DiseaseModel{
		CondensationMaxSpread: 2.0,
		WetnessRHThreshold:    90,
		HighHumidityMinHours:  1.0,
		DiseaseTempLow:        15,
		DiseaseTempHigh:       25,
	}
	leafy := DiseaseModel{
		CondensationMaxSpread: 2.0,
		WetnessRHThreshold:    90,
		HighHumidityMinHours:  1.0,
		DiseaseTempLow:        15,
		DiseaseTempHigh:       22,
	}
	tests := []struct {
		name string
		crop Crop
		want DiseaseModel
	}{
		// 施設果菜（灰色かび病・うどんこ病・暫定値）
		{"goya 施設果菜", CropGoya, greenhouseFruit},
		{"ingen 施設果菜", CropIngen, greenhouseFruit},
		{"uri 施設果菜", CropUri, greenhouseFruit},
		{"mango 施設果菜", CropMango, greenhouseFruit},
		// 施設葉菜（温度帯やや狭め・暫定値）
		{"leafy_vegetable 施設葉菜", CropLeafyVegetable, leafy},
		// 露地 → 既定フォールバック
		{"sugarcane 露地=既定", CropSugarcane, DefaultDiseaseModel},
		{"rice 露地=既定", CropRice, DefaultDiseaseModel},
		{"pineapple 露地=既定", CropPineapple, DefaultDiseaseModel},
		{"imo 露地=既定", CropImo, DefaultDiseaseModel},
		// 未設定（空）/ 不正 → 既定フォールバック
		{"未設定(空)→既定", Crop(""), DefaultDiseaseModel},
		{"不正値→既定", Crop("invalid"), DefaultDiseaseModel},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.crop.DiseaseModel()
			if got != tt.want {
				t.Errorf("DiseaseModel() = %+v, want %+v", got, tt.want)
			}
			// 不変条件: 発病好適温度帯は下限 ≤ 上限（DiseaseScore の事前条件）。
			if got.DiseaseTempLow > got.DiseaseTempHigh {
				t.Errorf("DiseaseTempLow %v > DiseaseTempHigh %v（不変条件違反）", got.DiseaseTempLow, got.DiseaseTempHigh)
			}
			// しきい値は物理的に妥当な正値（結露上限 ≥0・RH 0..100・最小継続 >0）。
			if got.CondensationMaxSpread < 0 {
				t.Errorf("CondensationMaxSpread = %v, want ≥0", got.CondensationMaxSpread)
			}
			if got.WetnessRHThreshold < 0 || got.WetnessRHThreshold > 100 {
				t.Errorf("WetnessRHThreshold = %v, want 0..100", got.WetnessRHThreshold)
			}
			if got.HighHumidityMinHours <= 0 {
				t.Errorf("HighHumidityMinHours = %v, want >0", got.HighHumidityMinHours)
			}
		})
	}
}

// TestDefaultDiseaseModel は既定病害モデルの値を要件どおり固定する（5.4 フォールバックの基準値）。
func TestDefaultDiseaseModel(t *testing.T) {
	want := DiseaseModel{
		CondensationMaxSpread: 2.0,
		WetnessRHThreshold:    90,
		HighHumidityMinHours:  1.0,
		DiseaseTempLow:        15,
		DiseaseTempHigh:       25,
	}
	if DefaultDiseaseModel != want {
		t.Errorf("DefaultDiseaseModel = %+v, want %+v", DefaultDiseaseModel, want)
	}
}

// TestCrop_DiseaseModel_AllCropsNonEmpty は全9作物＋未設定/不正で病害モデルが
// 具体値（DiseaseTempLow ≤ DiseaseTempHigh の有効帯）を返し、欄が常に非空になることを保証する（要件 5.2）。
func TestCrop_DiseaseModel_AllCropsNonEmpty(t *testing.T) {
	cases := append(AllCrops(), Crop(""), Crop("invalid"))
	for _, c := range cases {
		dm := c.DiseaseModel()
		if dm.DiseaseTempLow > dm.DiseaseTempHigh {
			t.Errorf("crop %q: 無効な温度帯 %v..%v", c, dm.DiseaseTempLow, dm.DiseaseTempHigh)
		}
	}
}

// ---- 3.1 GDDModel（作物別 Tbase・生育ステージ・収穫目標 GDD） -----------------

// TestCrop_GDDModel は作物別 GDD モデルが、米で具体値・他は既定フォールバックを返し、
// Stages が GDD 昇順・名称非空・Tbase 正値の不変条件を満たすことを固定する（要件 5.1〜5.4）。
// 数値そのものの妥当性は暫定でテスト対象外（実機/文献で確定＝GO 後スモーク）。具体値の「存在」を固定する。
func TestCrop_GDDModel(t *testing.T) {
	tests := []struct {
		name         string
		crop         Crop
		wantNonEmpty bool // 具体的な Stages（非空）を期待するか
	}{
		{"rice 具体値（GDD 本命作物）", CropRice, true},
		// 未確定作物は既定フォールバック（段階拡張で後追い可）。
		{"sugarcane 未定義→既定", CropSugarcane, false},
		{"goya 未定義→既定", CropGoya, false},
		{"leafy_vegetable 未定義→既定", CropLeafyVegetable, false},
		// 未設定（空）/ 不正 → 既定フォールバック（要件 5.4）。
		{"未設定(空)→既定", Crop(""), false},
		{"不正値→既定", Crop("invalid"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.crop.GDDModel()
			// Tbase は常に具体値（既定でも正値）で、Stages が空でも日次 GDD は算出可能（要件 5.4）。
			if m.Tbase <= 0 {
				t.Errorf("Tbase = %v, want >0", m.Tbase)
			}
			if tt.wantNonEmpty {
				if len(m.Stages) == 0 {
					t.Errorf("crop %q: Stages が空、具体値を期待", tt.crop)
				}
			} else if len(m.Stages) != 0 {
				// 未定義作物は DefaultGDDModel（Stages 空）にフォールバックする。
				t.Errorf("crop %q: 既定フォールバックのはずが Stages 非空 %+v", tt.crop, m.Stages)
			}
			// 不変条件: Stages は GDD 昇順（CumulativeGDD/GrowthStageIndex の前提）。
			for i := 1; i < len(m.Stages); i++ {
				if m.Stages[i].GDD < m.Stages[i-1].GDD {
					t.Errorf("Stages が昇順でない: [%d]=%v < [%d]=%v", i, m.Stages[i].GDD, i-1, m.Stages[i-1].GDD)
				}
			}
			// 各ステージ名は表示用ゆえ非空。
			for _, s := range m.Stages {
				if s.Name == "" {
					t.Errorf("ステージ名が空 %+v", s)
				}
			}
		})
	}
}

// TestCrop_GDDModel_Rice は米の具体値（Tbase=10・非空 Stages・収穫目標>0）を固定する（要件 5.2）。
func TestCrop_GDDModel_Rice(t *testing.T) {
	m := CropRice.GDDModel()
	if m.Tbase != 10 {
		t.Errorf("Rice Tbase = %v, want 10", m.Tbase)
	}
	if len(m.Stages) == 0 {
		t.Fatal("Rice Stages が空、具体的な生育ステージを期待")
	}
	// 最終段=収穫目標 GDD（昇順の最大・>0）。
	last := m.Stages[len(m.Stages)-1]
	if last.GDD <= 0 {
		t.Errorf("収穫目標 GDD（最終段）= %v, want >0", last.GDD)
	}
}

// TestDefaultGDDModel は既定 GDD モデル（5.4 フォールバックの基準値）を固定する。
func TestDefaultGDDModel(t *testing.T) {
	if DefaultGDDModel.Tbase <= 0 {
		t.Errorf("DefaultGDDModel.Tbase = %v, want >0", DefaultGDDModel.Tbase)
	}
	if len(DefaultGDDModel.Stages) != 0 {
		t.Errorf("DefaultGDDModel.Stages = %+v, want 空（収穫目標未定義）", DefaultGDDModel.Stages)
	}
}

func TestParseCrop(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		c, err := ParseCrop("goya")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c != CropGoya {
			t.Errorf("got %q, want %q", c, CropGoya)
		}
	})

	t.Run("invalid returns error", func(t *testing.T) {
		_, err := ParseCrop("tomato")
		if err == nil {
			t.Fatal("expected error for invalid crop")
		}
	})

	t.Run("empty returns error", func(t *testing.T) {
		_, err := ParseCrop("")
		if err == nil {
			t.Fatal("expected error for empty crop")
		}
	})
}
