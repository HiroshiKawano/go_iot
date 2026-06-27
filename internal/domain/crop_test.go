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
