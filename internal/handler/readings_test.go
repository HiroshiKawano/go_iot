package handler

import (
	"math"
	"net/url"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// --- 2.1 parseDateBounds: 日付フィルタ → BETWEEN 用区間 ---

// TestParseDateBounds_未指定は遠過去遠未来センチネル は、from/to 未指定で全期間検索が
// 成立する区間 (1970 下限・9999 上限) を返し、エラーマップが空であることを検証する。
func TestParseDateBounds_未指定は遠過去遠未来センチネル(t *testing.T) {
	fromTS, toTS, errs := parseDateBounds("", "")

	if len(errs) != 0 {
		t.Fatalf("errs = %v, want 空", errs)
	}
	if fromTS.Year() != 1970 || fromTS.Month() != time.January || fromTS.Day() != 1 {
		t.Errorf("fromTS = %v, want 1970-01-01 (遠過去センチネル)", fromTS)
	}
	if toTS.Year() != 9999 {
		t.Errorf("toTS = %v, want 9999 年 (遠未来センチネル)", toTS)
	}
	// 下限が上限より前 (区間として成立)。
	if !fromTS.Before(toTS) {
		t.Errorf("fromTS(%v) が toTS(%v) より後", fromTS, toTS)
	}
}

// TestParseDateBounds_両指定で暦日区間 は、両端指定時に開始日は当日始端、終了日は当日終端
// (end-of-day) となり、終了日当日 23:59 の計測が区間に含まれることを検証する。
func TestParseDateBounds_両指定で暦日区間(t *testing.T) {
	fromTS, toTS, errs := parseDateBounds("2026-04-01", "2026-04-20")

	if len(errs) != 0 {
		t.Fatalf("errs = %v, want 空", errs)
	}
	wantFrom := time.Date(2026, 4, 1, 0, 0, 0, 0, jst)
	if !fromTS.Equal(wantFrom) {
		t.Errorf("fromTS = %v, want %v (開始日始端 JST)", fromTS, wantFrom)
	}

	// 終了日当日 23:59 の計測は区間内 (end-of-day 補正で取りこぼさない)。
	reading2359 := time.Date(2026, 4, 20, 23, 59, 0, 0, jst)
	if fromTS.After(reading2359) || toTS.Before(reading2359) {
		t.Errorf("toTS = %v は 2026-04-20 23:59 を含まない (end-of-day 補正不足)", toTS)
	}
	// 翌日 00:00 は区間外 (上限を翌日へはみ出させない)。
	nextDay := time.Date(2026, 4, 21, 0, 0, 0, 0, jst)
	if !toTS.Before(nextDay) {
		t.Errorf("toTS = %v が翌日 00:00 以降を含んでいる", toTS)
	}
}

// TestParseDateBounds_片側指定 は、一方のみ指定時に他方がセンチネルになることを検証する。
func TestParseDateBounds_片側指定(t *testing.T) {
	// from のみ → to は遠未来センチネル。
	fromTS, toTS, errs := parseDateBounds("2026-04-01", "")
	if len(errs) != 0 {
		t.Fatalf("from のみ: errs = %v, want 空", errs)
	}
	if !fromTS.Equal(time.Date(2026, 4, 1, 0, 0, 0, 0, jst)) {
		t.Errorf("from のみ: fromTS = %v, want 2026-04-01 始端", fromTS)
	}
	if toTS.Year() != 9999 {
		t.Errorf("from のみ: toTS = %v, want 遠未来センチネル", toTS)
	}

	// to のみ → from は遠過去センチネル、to は end-of-day。
	fromTS, toTS, errs = parseDateBounds("", "2026-04-20")
	if len(errs) != 0 {
		t.Fatalf("to のみ: errs = %v, want 空", errs)
	}
	if fromTS.Year() != 1970 {
		t.Errorf("to のみ: fromTS = %v, want 遠過去センチネル", fromTS)
	}
	reading2359 := time.Date(2026, 4, 20, 23, 59, 0, 0, jst)
	if toTS.Before(reading2359) {
		t.Errorf("to のみ: toTS = %v は 2026-04-20 23:59 を含まない", toTS)
	}
}

// TestParseDateBounds_形式不正はエラーマップにキーを立てる は、YYYY-MM-DD として解釈
// できない入力でエラーマップに該当キーが立つことを検証する (区間は使わせない)。
func TestParseDateBounds_形式不正はエラーマップにキーを立てる(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantKey string
	}{
		{"スラッシュ区切りの開始日", "2026/04/20", "", "from"},
		{"英字の開始日", "abc", "", "from"},
		{"スラッシュ区切りの終了日", "", "2026/04/20", "to"},
		{"英字の終了日", "", "xyz", "to"},
		{"存在しない月の開始日", "2026-13-01", "", "from"},
		{"月末超過日の開始日(4月31日)", "2026-04-31", "", "from"}, // Go が正規化せず弾くことの確認
		{"月末超過日の終了日(2月30日)", "", "2026-02-30", "to"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, errs := parseDateBounds(tc.from, tc.to)
			if errs[tc.wantKey] == "" {
				t.Errorf("parseDateBounds(%q,%q): errs[%q] が空 (errs=%v)", tc.from, tc.to, tc.wantKey, errs)
			}
		})
	}
}

// TestParseDateBounds_開始日が終了日より後でも形式妥当ならエラー無し は、from>to が
// 意味検証の対象外（形式のみ検証）であり、反転区間をそのまま返すことを検証する
// （R2.4 は BETWEEN が自然に空を返す＝0件扱いで、特別なエラーにしない）。
func TestParseDateBounds_開始日が終了日より後でも形式妥当ならエラー無し(t *testing.T) {
	fromTS, toTS, errs := parseDateBounds("2026-04-20", "2026-04-01")

	if len(errs) != 0 {
		t.Fatalf("from>to で errs=%v, want 空（意味検証なし）", errs)
	}
	if !fromTS.Equal(time.Date(2026, 4, 20, 0, 0, 0, 0, jst)) {
		t.Errorf("fromTS=%v, want 2026-04-20 始端", fromTS)
	}
	// 反転区間（from > to）がそのまま返る。
	if !fromTS.After(toTS) {
		t.Errorf("fromTS(%v) は toTS(%v) より後であるべき（反転区間）", fromTS, toTS)
	}
}

// --- 2.2 parsePage / totalPagesOf ---

// TestParsePage は、ページ番号文字列を 1 以上へ正規化することを検証する。
func TestParsePage(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 1},
		{"0", 1},
		{"-1", 1},
		{"abc", 1},
		{"1", 1},
		{"3", 3},
		{"  5 ", 5},
		{"21", 21},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := parsePage(tc.in); got != tc.want {
				t.Errorf("parsePage(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// TestTotalPagesOf は、総件数から総ページ数 (1ページ20件・0件でも1ページ) を算出することを検証する。
func TestTotalPagesOf(t *testing.T) {
	tests := []struct {
		total int64
		want  int
	}{
		{0, 1},
		{1, 1},
		{19, 1},
		{20, 1},
		{21, 2},
		{40, 2},
		{41, 3},
	}
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			if got := totalPagesOf(tc.total); got != tc.want {
				t.Errorf("totalPagesOf(%d) = %d, want %d", tc.total, got, tc.want)
			}
		})
	}
}

// TestTotalPagesOf_巨大件数でも正のページ数 は、int64 上限近傍の総件数でも
// 加算オーバーフローによる負のページ数を返さないこと (防御的実装) を検証する。
func TestTotalPagesOf_巨大件数でも正のページ数(t *testing.T) {
	for _, total := range []int64{math.MaxInt64, math.MaxInt64 - 1, math.MaxInt64 - 19} {
		if got := totalPagesOf(total); got <= 0 {
			t.Errorf("totalPagesOf(%d) = %d, want 正の値 (オーバーフロー)", total, got)
		}
	}
}

// --- 2.3 formatDelay ---

// TestFormatDelay は、計測時刻とサーバ受信時刻の差を四捨五入した整数秒「N秒」へ整形し、
// 負値 (クロックずれ) を「0秒」にクランプすることを検証する。
func TestFormatDelay(t *testing.T) {
	base := time.Date(2026, 4, 20, 14, 30, 0, 0, jst)
	tests := []struct {
		name  string
		delay time.Duration
		want  string
	}{
		{"切り捨て側 0.4秒", 400 * time.Millisecond, "0秒"},
		{"切り上げ側 1.5秒", 1500 * time.Millisecond, "2秒"},
		{"半数切り上げ 2.5秒", 2500 * time.Millisecond, "3秒"},
		{"ちょうど 1秒", 1 * time.Second, "1秒"},
		{"差分0", 0, "0秒"},
		{"負値クランプ", -1 * time.Second, "0秒"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorded := base
			created := base.Add(tc.delay)
			if got := formatDelay(recorded, created); got != tc.want {
				t.Errorf("formatDelay(delay=%v) = %q, want %q", tc.delay, got, tc.want)
			}
		})
	}
}

// --- 2.4 buildSummary ---

// TestBuildSummary_データ有は小数2桁単位付き は、集計行を平均/最高/最低×温度/湿度の
// 6項目 (小数第2位+単位) へ整形することを検証する。
func TestBuildSummary_データ有は小数2桁単位付き(t *testing.T) {
	row := repository.GetSensorReadingsSummaryRow{
		AvgTemperature: 28.30,
		MaxTemperature: 31.20, // 集計列は CAST(... AS REAL) で float64 (silent 平坦化防止)
		MinTemperature: 25.10,
		AvgHumidity:    65.30,
		MaxHumidity:    70.00,
		MinHumidity:    60.50,
		SampleCount:    5,
	}
	got, err := buildSummary(row)
	if err != nil {
		t.Fatalf("buildSummary で予期しないエラー: %v", err)
	}
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"AvgTemp", got.AvgTemp, "28.30℃"},
		{"MaxTemp", got.MaxTemp, "31.20℃"},
		{"MinTemp", got.MinTemp, "25.10℃"},
		{"AvgHum", got.AvgHum, "65.30%"},
		{"MaxHum", got.MaxHum, "70.00%"},
		{"MinHum", got.MinHum, "60.50%"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// TestBuildSummary_0件は全項目ダッシュ は、サンプル数0のとき6項目すべてを「—」にして
// 0.00 と誤表示しないことを検証する。
func TestBuildSummary_0件は全項目ダッシュ(t *testing.T) {
	// 0件時は SampleCount=0 (集計クエリ空集合は handler 側で sql.ErrNoRows をゼロ値行へ写す)。
	row := repository.GetSensorReadingsSummaryRow{SampleCount: 0}
	got, err := buildSummary(row)
	if err != nil {
		t.Fatalf("buildSummary で予期しないエラー: %v", err)
	}
	fields := map[string]string{
		"AvgTemp": got.AvgTemp, "MaxTemp": got.MaxTemp, "MinTemp": got.MinTemp,
		"AvgHum": got.AvgHum, "MaxHum": got.MaxHum, "MinHum": got.MinHum,
	}
	for name, v := range fields {
		if v != "—" {
			t.Errorf("%s = %q, want 「—」 (0件)", name, v)
		}
	}
}

// --- 2.5 readingsURL ---

// TestReadingsURL_期間を保持しページを差し替える は、開始日・終了日を保持したまま
// 指定ページに差し替えた相対 URL を生成することを検証する。
func TestReadingsURL_期間を保持しページを差し替える(t *testing.T) {
	got := readingsURL(42, "2026-04-01", "2026-04-20", 2)

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("生成 URL のパース失敗: %v (%q)", err, got)
	}
	if u.Path != "/devices/42/readings" {
		t.Errorf("path = %q, want /devices/42/readings", u.Path)
	}
	q := u.Query()
	if q.Get("from") != "2026-04-01" {
		t.Errorf("from = %q, want 2026-04-01", q.Get("from"))
	}
	if q.Get("to") != "2026-04-20" {
		t.Errorf("to = %q, want 2026-04-20", q.Get("to"))
	}
	if q.Get("page") != "2" {
		t.Errorf("page = %q, want 2", q.Get("page"))
	}
}

// TestReadingsURL_期間未指定はpageのみ は、from/to 未指定時に page のみを持つ URL を返すことを検証する。
func TestReadingsURL_期間未指定はpageのみ(t *testing.T) {
	got := readingsURL(7, "", "", 1)

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("生成 URL のパース失敗: %v (%q)", err, got)
	}
	if u.Path != "/devices/7/readings" {
		t.Errorf("path = %q, want /devices/7/readings", u.Path)
	}
	q := u.Query()
	if q.Get("page") != "1" {
		t.Errorf("page = %q, want 1", q.Get("page"))
	}
	if _, ok := q["from"]; ok {
		t.Errorf("from が空指定で URL に含まれている: %q", got)
	}
	if _, ok := q["to"]; ok {
		t.Errorf("to が空指定で URL に含まれている: %q", got)
	}
}
