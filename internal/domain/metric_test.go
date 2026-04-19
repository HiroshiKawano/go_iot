package domain

import "testing"

func TestMetric_Label(t *testing.T) {
	tests := []struct {
		metric Metric
		want   string
	}{
		{MetricTemperature, "温度"},
		{MetricHumidity, "湿度"},
	}
	for _, tt := range tests {
		t.Run(string(tt.metric), func(t *testing.T) {
			if got := tt.metric.Label(); got != tt.want {
				t.Errorf("Label() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMetric_Unit(t *testing.T) {
	tests := []struct {
		metric Metric
		want   string
	}{
		{MetricTemperature, "℃"},
		{MetricHumidity, "%"},
	}
	for _, tt := range tests {
		t.Run(string(tt.metric), func(t *testing.T) {
			if got := tt.metric.Unit(); got != tt.want {
				t.Errorf("Unit() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMetric_Valid(t *testing.T) {
	tests := []struct {
		input Metric
		want  bool
	}{
		{MetricTemperature, true},
		{MetricHumidity, true},
		{Metric("invalid"), false},
		{Metric(""), false},
		{Metric("TEMPERATURE"), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := tt.input.Valid(); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseMetric(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		m, err := ParseMetric("temperature")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m != MetricTemperature {
			t.Errorf("got %q, want %q", m, MetricTemperature)
		}
	})

	t.Run("invalid returns error", func(t *testing.T) {
		_, err := ParseMetric("pressure")
		if err == nil {
			t.Fatal("expected error for invalid metric")
		}
	})
}
