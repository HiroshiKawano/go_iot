// Package timefmt は表示用の時刻整形ユーティリティを提供する。
// 外部依存を持たず（stdlib のみ）、基準時刻を引数で受け取る純粋関数として
// 実装することで決定的なテストを可能にする。
package timefmt

import (
	"fmt"
	"time"
)

// RelativeJP は now を基準に t の相対時刻を日本語で返す。
// 粒度: 1分未満「たった今」/ 1時間未満「N分前」/ 24時間未満「N時間前」/ それ以上「N日前」。
// 未来時刻（t が now より後）は「たった今」とする。
func RelativeJP(t, now time.Time) string {
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		// 1分未満（未来時刻による負の差分も含む）
		return "たった今"
	case diff < time.Hour:
		return fmt.Sprintf("%d分前", int(diff/time.Minute))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%d時間前", int(diff/time.Hour))
	default:
		return fmt.Sprintf("%d日前", int(diff/(24*time.Hour)))
	}
}

// DateTimeJP は t を「YYYY-MM-DD HH:MM:SS」形式の絶対表記で返す（最終通信日時用）。
// t の所在地（time.Location）をそのまま用いる純粋関数であり、TZ 変換は行わない。
// 表示したいタイムゾーンへの変換は呼び出し側（handler 等）の責務とする。
func DateTimeJP(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// DateTimeMinuteJP は t を「YYYY-MM-DD HH:MM」形式（分まで）の絶対表記で返す
// （最新計測テーブルの計測日時用）。秒は表示せず切り捨てる。TZ 変換は行わない。
func DateTimeMinuteJP(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}
