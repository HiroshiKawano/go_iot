package domain

import "testing"

// 沖縄県の市町村数（research.md 確定データ: 11市 + 11町 + 19村 = 41）。
const wantMunicipalityCount = 41

func TestAllMunicipalities_Count(t *testing.T) {
	if got := len(AllMunicipalities()); got != wantMunicipalityCount {
		t.Errorf("AllMunicipalities() の件数 = %d, want %d", got, wantMunicipalityCount)
	}
}

func TestAllMunicipalities_NoDuplicates(t *testing.T) {
	seen := make(map[Municipality]bool)
	for _, m := range AllMunicipalities() {
		if seen[m] {
			t.Errorf("AllMunicipalities() に重複: %q", m)
		}
		seen[m] = true
	}
}

func TestAllMunicipalities_AllValid(t *testing.T) {
	for _, m := range AllMunicipalities() {
		if !m.Valid() {
			t.Errorf("AllMunicipalities() の要素 %q が Valid()==false", m)
		}
	}
}

func TestMunicipality_Valid(t *testing.T) {
	tests := []struct {
		input Municipality
		want  bool
	}{
		{MunicipalityNaha, true},     // 市
		{MunicipalityKin, true},      // 町
		{MunicipalityKunigami, true}, // 村
		{MunicipalityUruma, true},    // 合併市
		{Municipality("具志川市"), false}, // 旧市町村名は市町村ではない（Locality 側）
		{Municipality("沖縄県"), false},
		{Municipality("invalid"), false},
		{Municipality(""), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := tt.input.Valid(); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMunicipality_Label(t *testing.T) {
	// 集計軸の表示ラベルは市町村名そのもの。
	tests := []struct {
		input Municipality
		want  string
	}{
		{MunicipalityNaha, "那覇市"},
		{MunicipalityUruma, "うるま市"},
		{MunicipalityHigashi, "東村"},
	}
	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := tt.input.Label(); got != tt.want {
				t.Errorf("Label() = %q, want %q", got, tt.want)
			}
		})
	}
}
