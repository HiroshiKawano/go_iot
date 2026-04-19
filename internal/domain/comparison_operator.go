package domain

import "fmt"

// ComparisonOperator は閾値比較の演算子を表す Enum。
// DB には文字列値 (> / < / >= / <=) として格納され、
// alert_rules.operator / alert_histories.operator の CHECK 制約と対応する。
type ComparisonOperator string

const (
	OpGreaterThan        ComparisonOperator = ">"
	OpLessThan           ComparisonOperator = "<"
	OpGreaterThanOrEqual ComparisonOperator = ">="
	OpLessThanOrEqual    ComparisonOperator = "<="
)

// Label は画面表示用の日本語ラベルを返す。
func (op ComparisonOperator) Label() string {
	switch op {
	case OpGreaterThan:
		return "より大きい"
	case OpLessThan:
		return "より小さい"
	case OpGreaterThanOrEqual:
		return "以上"
	case OpLessThanOrEqual:
		return "以下"
	}
	return string(op)
}

// Evaluate は actual を threshold と比較し、演算子の条件が成立するかを返す。
// アラート判定ロジックの中核。
func (op ComparisonOperator) Evaluate(actual, threshold float64) bool {
	switch op {
	case OpGreaterThan:
		return actual > threshold
	case OpLessThan:
		return actual < threshold
	case OpGreaterThanOrEqual:
		return actual >= threshold
	case OpLessThanOrEqual:
		return actual <= threshold
	}
	return false
}

// Valid は Enum として定義された値かを判定する。
func (op ComparisonOperator) Valid() bool {
	switch op {
	case OpGreaterThan, OpLessThan, OpGreaterThanOrEqual, OpLessThanOrEqual:
		return true
	}
	return false
}

// ParseComparisonOperator は文字列から演算子への変換を試み、不正値ならエラーを返す。
func ParseComparisonOperator(s string) (ComparisonOperator, error) {
	op := ComparisonOperator(s)
	if !op.Valid() {
		return "", fmt.Errorf("invalid comparison operator: %q", s)
	}
	return op, nil
}

// AllComparisonOperators は定義済み演算子の全列挙。フォーム選択肢の生成等に使用。
func AllComparisonOperators() []ComparisonOperator {
	return []ComparisonOperator{
		OpGreaterThan,
		OpLessThan,
		OpGreaterThanOrEqual,
		OpLessThanOrEqual,
	}
}
