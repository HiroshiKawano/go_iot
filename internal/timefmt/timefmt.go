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
