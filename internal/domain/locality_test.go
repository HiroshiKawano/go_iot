package domain

import "testing"

// 沖縄の地域数（research.md 確定データ: 未合併36市町村 + 旧町村17 = 53）。
const (
	wantLocalityCount         = 53
	wantUnmergedLocalityCount = 36
	wantMergedLocalityCount   = 17
)

func TestAllLocalities_Count(t *testing.T) {
	if got := len(AllLocalities()); got != wantLocalityCount {
		t.Errorf("AllLocalities() の件数 = %d, want %d", got, wantLocalityCount)
	}
}

func TestAllLocalities_NoDuplicates(t *testing.T) {
	seen := make(map[Locality]bool)
	for _, l := range AllLocalities() {
		if seen[l] {
			t.Errorf("AllLocalities() に重複: %q", l)
		}
		seen[l] = true
	}
}

func TestAllLocalities_AllValid(t *testing.T) {
	for _, l := range AllLocalities() {
		if !l.Valid() {
			t.Errorf("AllLocalities() の要素 %q が Valid()==false", l)
		}
	}
}

// 親市町村が常に有効な Municipality であること（不変条件）。
func TestAllLocalities_ParentIsValidMunicipality(t *testing.T) {
	for _, l := range AllLocalities() {
		if m := l.Municipality(); !m.Valid() {
			t.Errorf("Locality %q の親 %q が有効な Municipality でない", l, m)
		}
	}
}

// 未合併36（親=自身）と合併旧町村17（親≠自身）の内訳。
func TestAllLocalities_MergedVsUnmerged(t *testing.T) {
	var unmerged, merged int
	for _, l := range AllLocalities() {
		if string(l) == string(l.Municipality()) {
			unmerged++
		} else {
			merged++
		}
	}
	if unmerged != wantUnmergedLocalityCount {
		t.Errorf("未合併地域 = %d, want %d", unmerged, wantUnmergedLocalityCount)
	}
	if merged != wantMergedLocalityCount {
		t.Errorf("合併旧町村 = %d, want %d", merged, wantMergedLocalityCount)
	}
}

// 41市町村すべてが、いずれかの地域の親として現れること（集計軸の網羅）。
func TestAllLocalities_CoversAllMunicipalities(t *testing.T) {
	parents := make(map[Municipality]bool)
	for _, l := range AllLocalities() {
		parents[l.Municipality()] = true
	}
	for _, m := range AllMunicipalities() {
		if !parents[m] {
			t.Errorf("市町村 %q を親に持つ地域が存在しない", m)
		}
	}
}

func TestLocality_Municipality(t *testing.T) {
	tests := []struct {
		loc  Locality
		want Municipality
	}{
		// 未合併（親=自身）
		{LocalityNaha, MunicipalityNaha},
		{LocalityTaketomi, MunicipalityTaketomi},
		{LocalityHigashi, MunicipalityHigashi},
		// 合併（旧町村→現市町村）
		{LocalityGushikawaShi, MunicipalityUruma},  // 旧具志川市 → うるま市
		{LocalityGushikawaSon, MunicipalityKumejima}, // 旧具志川村 → 久米島町
		{LocalityIshikawaShi, MunicipalityUruma},   // 旧石川市 → うるま市
		{LocalitySashikiCho, MunicipalityNanjo},    // 旧佐敷町 → 南城市
		{LocalityHiraraShi, MunicipalityMiyakojima}, // 旧平良市 → 宮古島市
		{LocalityKochindaCho, MunicipalityYaese},   // 旧東風平町 → 八重瀬町
		{LocalityNakazatoSon, MunicipalityKumejima}, // 旧仲里村 → 久米島町
	}
	for _, tt := range tests {
		t.Run(string(tt.loc), func(t *testing.T) {
			if got := tt.loc.Municipality(); got != tt.want {
				t.Errorf("Municipality() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLocality_Label(t *testing.T) {
	tests := []struct {
		loc  Locality
		want string
	}{
		// 未合併＝市町村名そのもの
		{LocalityNaha, "那覇市"},
		{LocalityTaketomi, "竹富町"},
		{LocalityHigashi, "東村"},
		// 合併＝「短縮名（現市町村）」
		{LocalitySashikiCho, "佐敷（南城市）"},
		{LocalityIshikawaShi, "石川（うるま市）"},
		{LocalityHiraraShi, "平良（宮古島市）"},
		{LocalityKochindaCho, "東風平（八重瀬町）"},
	}
	for _, tt := range tests {
		t.Run(string(tt.loc), func(t *testing.T) {
			if got := tt.loc.Label(); got != tt.want {
				t.Errorf("Label() = %q, want %q", got, tt.want)
			}
		})
	}
}

// 同名（具志川）が別 Locality として区別され、Label に現市町村が併記されること。
func TestLocality_SameNameDistinction(t *testing.T) {
	if LocalityGushikawaShi == LocalityGushikawaSon {
		t.Fatal("旧具志川市と旧具志川村が同一 Locality 値になっている（区別不能）")
	}
	if got := LocalityGushikawaShi.Label(); got != "具志川（うるま市）" {
		t.Errorf("旧具志川市 Label() = %q, want %q", got, "具志川（うるま市）")
	}
	if got := LocalityGushikawaSon.Label(); got != "具志川（久米島町）" {
		t.Errorf("旧具志川村 Label() = %q, want %q", got, "具志川（久米島町）")
	}
}

func TestLocality_Valid(t *testing.T) {
	tests := []struct {
		input Locality
		want  bool
	}{
		{LocalityNaha, true},
		{LocalityGushikawaShi, true},
		{LocalityGushikawaSon, true},
		{Locality("うるま市"), false}, // 合併後の市は地域(認識名)ではない＝旧町村を選ぶ
		{Locality("宮古島市"), false},
		{Locality("具志川"), false},   // 短縮名は Locality 値ではない
		{Locality("沖縄県"), false},
		{Locality(""), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := tt.input.Valid(); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLocality(t *testing.T) {
	okTests := []struct {
		in   string
		want Locality
	}{
		{"具志川市", LocalityGushikawaShi}, // 正式名
		{"佐敷町", LocalitySashikiCho},     // 正式名
		{"佐敷", LocalitySashikiCho},       // 短縮名（一意）
		{"石川", LocalityIshikawaShi},      // 短縮名（一意）
		{"那覇市", LocalityNaha},            // 未合併＝正式名=現市町村名
		{"那覇", LocalityNaha},             // 未合併の短縮名
	}
	for _, tt := range okTests {
		t.Run("ok_"+tt.in, func(t *testing.T) {
			got, err := ParseLocality(tt.in)
			if err != nil {
				t.Fatalf("ParseLocality(%q) 予期せぬエラー: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseLocality(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}

	ngTests := []string{
		"具志川",   // 同名で曖昧（具志川市/具志川村）→ 解決不能
		"うるま市", // 合併後市町村名は単一地域に解決できない（旧町村が複数）
		"存在しない地名",
		"",
	}
	for _, in := range ngTests {
		t.Run("ng_"+in, func(t *testing.T) {
			if _, err := ParseLocality(in); err == nil {
				t.Errorf("ParseLocality(%q) はエラーを期待したが nil", in)
			}
		})
	}
}
