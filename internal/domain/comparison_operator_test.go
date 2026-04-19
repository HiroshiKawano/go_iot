package domain

import "testing"

func TestComparisonOperator_Label(t *testing.T) {
	tests := []struct {
		op   ComparisonOperator
		want string
	}{
		{OpGreaterThan, "より大きい"},
		{OpLessThan, "より小さい"},
		{OpGreaterThanOrEqual, "以上"},
		{OpLessThanOrEqual, "以下"},
	}
	for _, tt := range tests {
		t.Run(string(tt.op), func(t *testing.T) {
			if got := tt.op.Label(); got != tt.want {
				t.Errorf("Label() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestComparisonOperator_Evaluate_Boundary(t *testing.T) {
	// 境界値テスト: threshold = 35.00 における ±0.01 の挙動を検証
	tests := []struct {
		name      string
		op        ComparisonOperator
		actual    float64
		threshold float64
		want      bool
	}{
		// GreaterThan (>)
		{"GreaterThan: 35.01 > 35.00 → true", OpGreaterThan, 35.01, 35.00, true},
		{"GreaterThan: 35.00 > 35.00 → false (等値は含まない)", OpGreaterThan, 35.00, 35.00, false},
		{"GreaterThan: 34.99 > 35.00 → false", OpGreaterThan, 34.99, 35.00, false},

		// LessThan (<)
		{"LessThan: 29.99 < 30.00 → true", OpLessThan, 29.99, 30.00, true},
		{"LessThan: 30.00 < 30.00 → false (等値は含まない)", OpLessThan, 30.00, 30.00, false},
		{"LessThan: 30.01 < 30.00 → false", OpLessThan, 30.01, 30.00, false},

		// GreaterThanOrEqual (>=)
		{"GreaterThanOrEqual: 35.01 >= 35.00 → true", OpGreaterThanOrEqual, 35.01, 35.00, true},
		{"GreaterThanOrEqual: 35.00 >= 35.00 → true (等値を含む)", OpGreaterThanOrEqual, 35.00, 35.00, true},
		{"GreaterThanOrEqual: 34.99 >= 35.00 → false", OpGreaterThanOrEqual, 34.99, 35.00, false},

		// LessThanOrEqual (<=)
		{"LessThanOrEqual: 29.99 <= 30.00 → true", OpLessThanOrEqual, 29.99, 30.00, true},
		{"LessThanOrEqual: 30.00 <= 30.00 → true (等値を含む)", OpLessThanOrEqual, 30.00, 30.00, true},
		{"LessThanOrEqual: 30.01 <= 30.00 → false", OpLessThanOrEqual, 30.01, 30.00, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.op.Evaluate(tt.actual, tt.threshold)
			if got != tt.want {
				t.Errorf("Evaluate(%.2f, %.2f) = %v, want %v",
					tt.actual, tt.threshold, got, tt.want)
			}
		})
	}
}

func TestComparisonOperator_Valid(t *testing.T) {
	tests := []struct {
		input ComparisonOperator
		want  bool
	}{
		{OpGreaterThan, true},
		{OpLessThan, true},
		{OpGreaterThanOrEqual, true},
		{OpLessThanOrEqual, true},
		{ComparisonOperator("=="), false},
		{ComparisonOperator(""), false},
		{ComparisonOperator("GT"), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := tt.input.Valid(); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseComparisonOperator(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		op, err := ParseComparisonOperator(">=")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if op != OpGreaterThanOrEqual {
			t.Errorf("got %q, want %q", op, OpGreaterThanOrEqual)
		}
	})

	t.Run("invalid returns error", func(t *testing.T) {
		_, err := ParseComparisonOperator("==")
		if err == nil {
			t.Fatal("expected error for invalid operator")
		}
	})
}
