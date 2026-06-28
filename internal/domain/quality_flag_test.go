package domain

import "testing"

// ---- QualityFlag ------------------------------------------------------------

func TestQualityFlag_Label(t *testing.T) {
	tests := []struct {
		flag QualityFlag
		want string
	}{
		{QualityFlagMissing, "欠測直後"},
		{QualityFlagStuck, "固着"},
		{QualityFlagPhysical, "物理異常"},
		{QualityFlagOutlier, "外れ値"},
	}
	for _, tt := range tests {
		t.Run(string(tt.flag), func(t *testing.T) {
			if got := tt.flag.Label(); got != tt.want {
				t.Errorf("Label() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestQualityFlag_Valid(t *testing.T) {
	tests := []struct {
		input QualityFlag
		want  bool
	}{
		{QualityFlagMissing, true},
		{QualityFlagStuck, true},
		{QualityFlagPhysical, true},
		{QualityFlagOutlier, true},
		{QualityFlag("invalid"), false},
		{QualityFlag(""), false},
		{QualityFlag("MISSING"), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := tt.input.Valid(); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQualityFlag_Label_UnknownFallback(t *testing.T) {
	// 未知値は生文字列をそのまま返す（防御的フォールバック）。
	if got := QualityFlag("xxx").Label(); got != "xxx" {
		t.Errorf("Label() = %q, want %q", got, "xxx")
	}
}

func TestParseQualityFlag(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		f, err := ParseQualityFlag("stuck")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f != QualityFlagStuck {
			t.Errorf("got %q, want %q", f, QualityFlagStuck)
		}
	})
	t.Run("invalid returns error", func(t *testing.T) {
		if _, err := ParseQualityFlag("noise"); err == nil {
			t.Fatal("expected error for invalid flag")
		}
	})
}

func TestAllQualityFlags(t *testing.T) {
	got := AllQualityFlags()
	want := []QualityFlag{QualityFlagMissing, QualityFlagStuck, QualityFlagPhysical, QualityFlagOutlier}
	if len(got) != len(want) {
		t.Fatalf("len=%d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AllQualityFlags()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// 全列挙が Valid であること（網羅の不変条件）。
	for _, f := range got {
		if !f.Valid() {
			t.Errorf("%q は Valid であるべき", f)
		}
	}
}

// ---- QualityLevel -----------------------------------------------------------

func TestQualityLevel_Label(t *testing.T) {
	tests := []struct {
		level QualityLevel
		want  string
	}{
		{QualityLevelGood, "信頼"},
		{QualityLevelCaution, "注意"},
		{QualityLevelBad, "不良"},
	}
	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			if got := tt.level.Label(); got != tt.want {
				t.Errorf("Label() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestQualityLevel_Valid(t *testing.T) {
	tests := []struct {
		input QualityLevel
		want  bool
	}{
		{QualityLevelGood, true},
		{QualityLevelCaution, true},
		{QualityLevelBad, true},
		{QualityLevel("invalid"), false},
		{QualityLevel(""), false},
		{QualityLevel("GOOD"), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := tt.input.Valid(); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQualityLevel_BadgeClass(t *testing.T) {
	// @layer components の .badge-good/.badge-caution/.badge-bad と 1:1 対応（dot なし・全相異）。
	tests := []struct {
		level QualityLevel
		want  string
	}{
		{QualityLevelGood, "badge-good"},
		{QualityLevelCaution, "badge-caution"},
		{QualityLevelBad, "badge-bad"},
	}
	seen := map[string]bool{}
	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			got := tt.level.BadgeClass()
			if got != tt.want {
				t.Errorf("BadgeClass() = %q, want %q", got, tt.want)
			}
			if seen[got] {
				t.Errorf("BadgeClass() = %q が重複（信号色は 1:1 対応であるべき）", got)
			}
			seen[got] = true
		})
	}
}

func TestQualityLevel_UnknownFallback(t *testing.T) {
	// 未知値は Label が生文字列・BadgeClass が空文字を返す（防御的フォールバック）。
	if got := QualityLevel("xxx").Label(); got != "xxx" {
		t.Errorf("Label() = %q, want %q", got, "xxx")
	}
	if got := QualityLevel("xxx").BadgeClass(); got != "" {
		t.Errorf("BadgeClass() = %q, want \"\"", got)
	}
}

func TestParseQualityLevel(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		l, err := ParseQualityLevel("caution")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if l != QualityLevelCaution {
			t.Errorf("got %q, want %q", l, QualityLevelCaution)
		}
	})
	t.Run("invalid returns error", func(t *testing.T) {
		if _, err := ParseQualityLevel("yellow"); err == nil {
			t.Fatal("expected error for invalid level")
		}
	})
}

func TestAllQualityLevels(t *testing.T) {
	got := AllQualityLevels()
	want := []QualityLevel{QualityLevelGood, QualityLevelCaution, QualityLevelBad}
	if len(got) != len(want) {
		t.Fatalf("len=%d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AllQualityLevels()[%d] = %q, want %q", i, got[i], want[i])
		}
		if !got[i].Valid() {
			t.Errorf("%q は Valid であるべき", got[i])
		}
	}
}
