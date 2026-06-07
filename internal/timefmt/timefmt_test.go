package timefmt

import (
	"testing"
	"time"
)

// TestRelativeJP は now を基準にした相対時刻の日本語表現を検証する。
// 決定的テストのため now は固定値を注入し、t は now からの差分で構成する。
func TestRelativeJP(t *testing.T) {
	// 基準時刻（固定）。タイムゾーン非依存に検証するため UTC で固定する。
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		t    time.Time // 評価対象の時刻
		want string
	}{
		// --- 1分未満は「たった今」 ---
		{"now と同時刻", now, "たった今"},
		{"30秒前", now.Add(-30 * time.Second), "たった今"},
		{"59秒前（1分境界の直前）", now.Add(-59 * time.Second), "たった今"},
		// --- 1時間未満は「N分前」 ---
		{"ちょうど1分前（分境界）", now.Add(-1 * time.Minute), "1分前"},
		{"2分前", now.Add(-2 * time.Minute), "2分前"},
		{"59分前（時間境界の直前）", now.Add(-59 * time.Minute), "59分前"},
		// --- 24時間未満は「N時間前」 ---
		{"ちょうど1時間前（時間境界）", now.Add(-1 * time.Hour), "1時間前"},
		{"3時間前", now.Add(-3 * time.Hour), "3時間前"},
		{"23時間前（日境界の直前）", now.Add(-23 * time.Hour), "23時間前"},
		// --- 24時間以上は「N日前」 ---
		{"ちょうど24時間前（日境界）", now.Add(-24 * time.Hour), "1日前"},
		{"2日前", now.Add(-48 * time.Hour), "2日前"},
		{"10日前", now.Add(-240 * time.Hour), "10日前"},
		// --- 未来時刻は「たった今」 ---
		{"5分後（未来）", now.Add(5 * time.Minute), "たった今"},
		{"1日後（未来）", now.Add(24 * time.Hour), "たった今"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RelativeJP(tt.t, now); got != tt.want {
				t.Errorf("RelativeJP() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDateTimeJP は最終通信日時用の絶対整形「YYYY-MM-DD HH:MM:SS」を検証する。
// 引数の時刻をそのまま整形する純粋関数（TZ 変換は行わない=呼び出し側責務）であり、
// 機種 TZ に依存せず決定的に検証するため入力を UTC 固定で構成する。
func TestDateTimeJP(t *testing.T) {
	tests := []struct {
		name string
		in   time.Time
		want string
	}{
		{"通常の日時", time.Date(2026, 4, 20, 14, 30, 5, 0, time.UTC), "2026-04-20 14:30:05"},
		{"月日時分秒すべて1桁（ゼロ埋め確認）", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), "2026-01-02 03:04:05"},
		{"真夜中 00:00:00", time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), "2026-12-31 00:00:00"},
		{"ナノ秒は切り捨て（秒までで止まる）", time.Date(2026, 6, 8, 23, 59, 59, 999_999_999, time.UTC), "2026-06-08 23:59:59"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DateTimeJP(tt.in); got != tt.want {
				t.Errorf("DateTimeJP() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDateTimeMinuteJP は計測テーブル用の絶対整形「YYYY-MM-DD HH:MM」を検証する。
// 秒を含めず分までで切る点が DateTimeJP との差分。入力は同様に UTC 固定で決定的に検証する。
func TestDateTimeMinuteJP(t *testing.T) {
	tests := []struct {
		name string
		in   time.Time
		want string
	}{
		{"通常の日時（秒は表示しない）", time.Date(2026, 4, 20, 14, 30, 5, 0, time.UTC), "2026-04-20 14:30"},
		{"月日時分すべて1桁（ゼロ埋め確認）", time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC), "2026-01-02 03:04"},
		{"真夜中 00:00", time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), "2026-12-31 00:00"},
		{"秒が大きくても分は繰り上げない（切り捨て）", time.Date(2026, 6, 8, 23, 59, 59, 0, time.UTC), "2026-06-08 23:59"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DateTimeMinuteJP(tt.in); got != tt.want {
				t.Errorf("DateTimeMinuteJP() = %q, want %q", got, tt.want)
			}
		})
	}
}
