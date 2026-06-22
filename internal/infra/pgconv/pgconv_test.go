package pgconv

import (
	"math"
	"testing"
	"time"
)

func TestNumeric2_RoundTrip(t *testing.T) {
	tests := []struct {
		in   float64
		want float64
	}{
		{28.50, 28.50},
		{0.00, 0.00},
		{-40.00, -40.00},
		{125.00, 125.00},
		{65.30, 65.30},
		// 丸めの確認: 第3位を四捨五入
		{28.505, 28.51},
		{28.504, 28.50},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			n := Numeric2(tt.in)
			got := NumericToFloat(n)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("Numeric2(%v) → float %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestTimestamptz_RoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	ts := Timestamptz(now)
	got := TimestamptzToTime(ts)
	if !got.Equal(now) {
		t.Errorf("round trip failed: got %v, want %v", got, now)
	}
}
