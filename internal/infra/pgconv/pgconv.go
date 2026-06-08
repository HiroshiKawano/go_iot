// Package pgconv は repository の生成型(database/sql ベース)を表示用の整形済み値へ
// 変換する最小ヘルパを提供する。
//
// SQLite 移行後は NUMERIC=float64・datetime=time.Time が repository から直接得られるため、
// 旧来の pgtype 往復ヘルパ(Numeric2/NumericToFloat/Timestamptz/TimestamptzToTime)は不要となり、
// 本パッケージの責務は次の2点に痩せている:
//   - NULL 許容 datetime 列(*time.Time)のゼロ値合体
//   - NUMERIC(5,2) 相当の %.2f 整形
//
// JST 表示(相対/日時ラベル)は internal/timefmt が担う。本パッケージは型合体と数値整形に限定する。
package pgconv

import (
	"fmt"
	"math"
	"time"
)

// Format2 は数値を小数第2位までの文字列に整形する(NUMERIC(5,2) 相当の表示)。
// 温度・湿度・閾値・実測値の画面表示に用い、末尾は0補完される(例: 28.5 → "28.50")。
//
// 注意: fmt の %.2f は偶数丸め(banker's rounding)。3桁目の丸めは保存側の Quantize2 が
// 担う前提であり、保存値が既に2桁なら %.2f は無風になる。保存(Quantize2)と表示(Format2)で
// 丸めの責務を分離し、表示時に移行前との丸め差を生まないようにしている。
func Format2(f float64) string {
	return fmt.Sprintf("%.2f", f)
}

// Quantize2 は数値を小数第2位へ量子化する(math.Round による half-away-from-zero)。
// 移行前は NUMERIC(5,2) 列が保存の瞬間に2桁へ丸めていた(旧 pgconv.Numeric2 と同一の
// math.Round(f*100)/100)。SQLite の REAL は float64 を丸めず保持するため、保存側で
// この量子化を再現して「移行前と同一の値として保存・取得」(R4.1)を満たす。
//
// 用途: sensor_api / seed 等の温度・湿度・閾値・実測値の INSERT/UPDATE 直前(Task 4.1)。
func Quantize2(f float64) float64 {
	return math.Round(f*100) / 100
}

// TimeOrZero は NULL 許容 datetime 列(*time.Time)をゼロ値合体した time.Time に変換する。
// nil(SQL NULL)の場合はゼロ値の time.Time を返す。
//
// 対象列: deleted_at / last_communicated_at / expires_at / email_verified_at / last_used_at。
// 呼び出し側は表示分岐(未通信ラベル等)が必要なら nil 判定または戻り値の IsZero() を用いる。
func TimeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
