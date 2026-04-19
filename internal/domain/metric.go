package domain

import "fmt"

// Metric は計測指標を表す Enum。
// DB には文字列値 (temperature / humidity) として格納され、
// alert_rules.metric / alert_histories.metric の CHECK 制約と対応する。
type Metric string

const (
	MetricTemperature Metric = "temperature"
	MetricHumidity    Metric = "humidity"
)

// Label は画面表示用の日本語ラベルを返す。
func (m Metric) Label() string {
	switch m {
	case MetricTemperature:
		return "温度"
	case MetricHumidity:
		return "湿度"
	}
	return string(m)
}

// Unit は指標の単位を返す。
func (m Metric) Unit() string {
	switch m {
	case MetricTemperature:
		return "℃"
	case MetricHumidity:
		return "%"
	}
	return ""
}

// Valid は Enum として定義された値かを判定する。
func (m Metric) Valid() bool {
	switch m {
	case MetricTemperature, MetricHumidity:
		return true
	}
	return false
}

// ParseMetric は文字列から Metric への変換を試み、不正値ならエラーを返す。
func ParseMetric(s string) (Metric, error) {
	m := Metric(s)
	if !m.Valid() {
		return "", fmt.Errorf("invalid metric: %q", s)
	}
	return m, nil
}

// AllMetrics は定義済み Metric の全列挙。フォーム選択肢の生成等に使用。
func AllMetrics() []Metric {
	return []Metric{MetricTemperature, MetricHumidity}
}
