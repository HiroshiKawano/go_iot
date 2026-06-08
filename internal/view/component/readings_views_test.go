package component

import "testing"

// TestDeviceReadingsListView_結果領域フィールドを保持する は、フィルタ結果領域 fragment の
// View-model が 集計・行スライス・データ有無フラグ・ページャ・エラーマップを
// 整形済み primitive のみで束ねられることを検証する。
func TestDeviceReadingsListView_結果領域フィールドを保持する(t *testing.T) {
	v := DeviceReadingsListView{
		Summary: SummaryView{
			AvgTemp: "28.30℃", MaxTemp: "31.20℃", MinTemp: "25.10℃",
			AvgHum: "65.30%", MaxHum: "70.00%", MinHum: "60.50%",
		},
		Rows: []ReadingHistoryRow{
			{RecordedAt: "2026-04-20 14:30", Temp: "28.50", Humidity: "65.30", Delay: "2秒"},
		},
		HasData:    true,
		Pagination: PaginationView{Current: 2, Last: 5, HasPrev: true, HasNext: true},
		Errors:     map[string]string{"from": "日付の形式が正しくありません。"},
	}

	if !v.HasData {
		t.Error("HasData = false, want true")
	}
	if len(v.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(v.Rows))
	}
	if got := v.Rows[0].Delay; got != "2秒" {
		t.Errorf("Rows[0].Delay = %q, want 2秒", got)
	}
	if got := v.Summary.MinHum; got != "60.50%" {
		t.Errorf("Summary.MinHum = %q, want 60.50%%", got)
	}
	if v.Pagination.Current != 2 {
		t.Errorf("Pagination.Current = %d, want 2", v.Pagination.Current)
	}
	if got := v.Errors["from"]; got != "日付の形式が正しくありません。" {
		t.Errorf("Errors[from] = %q", got)
	}
}

// TestSummaryView_整形済み6項目を保持する は、集計表示 View-model が
// 平均/最高/最低×温度/湿度の6項目を整形済み文字列で保持できることを検証する。
// 0件時に "—" を保持できることも併せて確認する。
func TestSummaryView_整形済み6項目を保持する(t *testing.T) {
	s := SummaryView{
		AvgTemp: "28.30℃", MaxTemp: "31.20℃", MinTemp: "25.10℃",
		AvgHum: "65.30%", MaxHum: "70.00%", MinHum: "60.50%",
	}
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"AvgTemp", s.AvgTemp, "28.30℃"},
		{"MaxTemp", s.MaxTemp, "31.20℃"},
		{"MinTemp", s.MinTemp, "25.10℃"},
		{"AvgHum", s.AvgHum, "65.30%"},
		{"MaxHum", s.MaxHum, "70.00%"},
		{"MinHum", s.MinHum, "60.50%"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}

	empty := SummaryView{
		AvgTemp: "—", MaxTemp: "—", MinTemp: "—",
		AvgHum: "—", MaxHum: "—", MinHum: "—",
	}
	if empty.AvgTemp != "—" || empty.MinHum != "—" {
		t.Errorf("0件時の項目が「—」でない: %+v", empty)
	}
}

// TestReadingHistoryRow_4項目を保持する は、履歴一覧の1行 View-model が
// 計測日時・温度・湿度・通信遅延の4項目を保持できることを検証する
// (既存の ReadingRow 3項目とは別型で、Delay を加える)。
func TestReadingHistoryRow_4項目を保持する(t *testing.T) {
	r := ReadingHistoryRow{
		RecordedAt: "2026-04-20 14:30",
		Temp:       "28.50",
		Humidity:   "65.30",
		Delay:      "2秒",
	}
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"RecordedAt", r.RecordedAt, "2026-04-20 14:30"},
		{"Temp", r.Temp, "28.50"},
		{"Humidity", r.Humidity, "65.30"},
		{"Delay", r.Delay, "2秒"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// TestPaginationView_現在総ページと前後リンクを保持する は、簡易ページャ View-model が
// 現在/総ページ・前後ページ有無・前後 URL を保持できることを検証する。
func TestPaginationView_現在総ページと前後リンクを保持する(t *testing.T) {
	p := PaginationView{
		Current: 2,
		Last:    5,
		HasPrev: true,
		HasNext: true,
		PrevURL: "/devices/42/readings?from=2026-04-01&to=2026-04-20&page=1",
		NextURL: "/devices/42/readings?from=2026-04-01&to=2026-04-20&page=3",
	}
	if p.Current != 2 || p.Last != 5 {
		t.Errorf("Current/Last = %d/%d, want 2/5", p.Current, p.Last)
	}
	if !p.HasPrev || !p.HasNext {
		t.Errorf("HasPrev/HasNext = %v/%v, want true/true", p.HasPrev, p.HasNext)
	}
	if got := p.PrevURL; got != "/devices/42/readings?from=2026-04-01&to=2026-04-20&page=1" {
		t.Errorf("PrevURL = %q", got)
	}
	if got := p.NextURL; got != "/devices/42/readings?from=2026-04-01&to=2026-04-20&page=3" {
		t.Errorf("NextURL = %q", got)
	}

	// 先頭ページでは前へ無し、最終ページでは次へ無しを表現できる。
	first := PaginationView{Current: 1, Last: 3, HasPrev: false, HasNext: true}
	if first.HasPrev {
		t.Error("先頭ページで HasPrev = true になっている")
	}
}
