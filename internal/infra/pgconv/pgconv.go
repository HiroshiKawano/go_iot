// Package pgconv は pgx (pgtype) と Go 標準型の相互変換ヘルパーを提供する。
// NUMERIC / TIMESTAMPTZ を float64 / time.Time と行き来する際に使用する。
package pgconv

import (
	"math"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Numeric2 は float64 を NUMERIC(5,2) 相当の pgtype.Numeric に変換する。
// 小数第3位以下は四捨五入される。
func Numeric2(f float64) pgtype.Numeric {
	return pgtype.Numeric{
		Int:   big.NewInt(int64(math.Round(f * 100))),
		Exp:   -2,
		Valid: true,
	}
}

// NumericToFloat は pgtype.Numeric を float64 に変換する。
// Valid でない、または変換失敗時は 0 を返す。
func NumericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid || n.Int == nil {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

// Timestamptz は time.Time を pgtype.Timestamptz に変換する。
func Timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// TimestamptzToTime は pgtype.Timestamptz を time.Time に変換する。
// Valid でない場合はゼロ値を返す。
func TimestamptzToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}
