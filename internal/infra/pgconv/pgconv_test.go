package pgconv

import (
	"testing"
	"time"
)

// TestFormat2 は NUMERIC(5,2) 相当の小数第2位整形を検証する(table-driven)。
// 温度・湿度・閾値・実測値は2桁精度で保存され、表示も %.2f で末尾0補完する。
func TestFormat2(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want string
	}{
		{"末尾0補完", 28.5, "28.50"},
		{"2桁そのまま", 28.50, "28.50"},
		{"ゼロ", 0, "0.00"},
		{"最小温度(負)", -40, "-40.00"},
		{"最大温度", 125, "125.00"},
		{"湿度", 65.3, "65.30"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format2(tt.in); got != tt.want {
				t.Errorf("Format2(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestQuantize2 は保存時2桁量子化(移行前の NUMERIC(5,2) 保存丸めの再現)を検証する。
// math.Round による half-away-from-zero で、3桁目以下を小数第2位へ丸める。
func TestQuantize2(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{"2桁はそのまま", 28.50, 28.50},
		{"3桁目切上", 28.567, 28.57},
		{"3桁目切下", 28.564, 28.56},
		{"負の3桁目切上", -28.567, -28.57},
		{"ゼロ", 0, 0},
		{"最大温度", 125.00, 125.00},
		{"最小温度", -40.00, -40.00},
		{"湿度3桁切上", 65.308, 65.31},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Quantize2(tt.in); got != tt.want {
				t.Errorf("Quantize2(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestQuantize2とFormat2の連携は移行前と等価な表示になる は、保存時 Quantize2 → 表示 Format2
// の経路が、3桁目入力でも安定した2桁表示を返すこと(偶数丸め差を生まないこと)を固定する。
func TestQuantize2とFormat2の連携は移行前と等価な表示になる(t *testing.T) {
	// 3桁目を持つ入力でも、保存側 Quantize2 を通せば表示は量子化済み2桁になる。
	tests := []struct {
		in   float64
		want string
	}{
		{28.567, "28.57"},
		{28.564, "28.56"},
		{65.308, "65.31"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := Format2(Quantize2(tt.in)); got != tt.want {
				t.Errorf("Format2(Quantize2(%v)) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestTimeOrZero は NULL 許容 datetime 列(*time.Time)の合体を検証する。
// nil(SQL NULL)はゼロ値、非nilは指す時刻をそのまま返す。
func TestTimeOrZero(t *testing.T) {
	v := time.Date(2026, 6, 8, 12, 34, 56, 0, time.UTC)
	tests := []struct {
		name   string
		in     *time.Time
		want   time.Time
		isZero bool
	}{
		{"非nilはそのまま返す", &v, v, false},
		{"nilはゼロ値", nil, time.Time{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TimeOrZero(tt.in)
			if !got.Equal(tt.want) {
				t.Errorf("TimeOrZero(%v) = %v, want %v", tt.in, got, tt.want)
			}
			if got.IsZero() != tt.isZero {
				t.Errorf("TimeOrZero(%v).IsZero() = %v, want %v", tt.in, got.IsZero(), tt.isZero)
			}
		})
	}
}
