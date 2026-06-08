package handler

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// --- 3.1 parseDeviceID: device_id クエリ解釈 ---

// TestParseDeviceID は、空→全デバイス(nil,ok)・数値→ポインタ(ok)・非数値→不正フラグを検証する。
func TestParseDeviceID(t *testing.T) {
	t.Run("空は全デバイス(nil,ok)", func(t *testing.T) {
		got, ok := parseDeviceID("")
		if !ok {
			t.Fatalf("ok=false, want true (空は全デバイスで正常)")
		}
		if got != nil {
			t.Errorf("got=%v, want nil (全デバイス)", got)
		}
	})
	t.Run("数値はポインタ(ok)", func(t *testing.T) {
		got, ok := parseDeviceID("2")
		if !ok {
			t.Fatalf("ok=false, want true")
		}
		if got == nil || *got != 2 {
			t.Errorf("got=%v, want *2", got)
		}
	})
	t.Run("非数値は不正フラグ", func(t *testing.T) {
		got, ok := parseDeviceID("abc")
		if ok {
			t.Errorf("ok=true, want false (非数値)")
		}
		if got != nil {
			t.Errorf("got=%v, want nil", got)
		}
	})
}

// --- 3.1 dateRangeError: from<=to のローカル範囲検証 ---

// TestDateRangeError は、両指定かつ from>to のみエラー・from==to は許容・
// 片方のみ/未指定は範囲検証スキップを検証する (R7.1/7.2)。
func TestDateRangeError(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{"両指定 from>to はエラー", "2026-04-20", "2026-04-01", true},
		{"両指定 from==to は許容", "2026-04-01", "2026-04-01", false},
		{"両指定 from<to は許容", "2026-04-01", "2026-04-20", false},
		{"from のみは範囲検証スキップ", "2026-04-20", "", false},
		{"to のみは範囲検証スキップ", "", "2026-04-01", false},
		{"未指定はスキップ", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fromTS, toTS, errs := parseDateBounds(tc.from, tc.to)
			if len(errs) != 0 {
				t.Fatalf("形式エラーが出た(本テストは形式妥当前提): %v", errs)
			}
			got := dateRangeError(tc.from, tc.to, fromTS, toTS)
			if (got != "") != tc.wantErr {
				t.Errorf("dateRangeError(%q,%q)=%q, wantErr=%v", tc.from, tc.to, got, tc.wantErr)
			}
		})
	}
}

// --- 3.1 buildAlertHistoryRows: 履歴行 → 表示 DTO ---

// TestBuildAlertHistoryRows は、発火日時(JST分まで)・指標ラベル・条件(演算子+閾値2桁+単位)・
// 実測値(2桁+単位)・通知(済/未)へ整形することを検証する (R4.1〜4.7)。
func TestBuildAlertHistoryRows(t *testing.T) {
	rows := []repository.ListAlertHistoriesPaginatedRow{
		{
			Metric: "temperature", Operator: ">",
			Threshold: 35.00, ActualValue: 38.50,
			IsNotified:  true,
			TriggeredAt: time.Date(2026, 4, 20, 5, 30, 0, 0, time.UTC), // UTC5:30→JST14:30
			DeviceName:  "ハウスA温湿度計",
		},
		{
			Metric: "humidity", Operator: "<",
			Threshold: 30.00, ActualValue: 25.00,
			IsNotified:  false,
			TriggeredAt: time.Date(2026, 4, 20, 4, 15, 0, 0, time.UTC), // UTC4:15→JST13:15
			DeviceName:  "ハウスB温湿度計",
		},
	}
	got := buildAlertHistoryRows(rows)
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2", len(got))
	}

	r0 := got[0]
	checks0 := map[string][2]string{
		"TriggeredAt": {r0.TriggeredAt, "2026-04-20 14:30"},
		"DeviceName":  {r0.DeviceName, "ハウスA温湿度計"},
		"MetricLabel": {r0.MetricLabel, "温度"},
		"Condition":   {r0.Condition, "> 35.00℃"},
		"ActualValue": {r0.ActualValue, "38.50℃"},
		"Notified":    {r0.Notified, "済"},
	}
	for name, gw := range checks0 {
		if gw[0] != gw[1] {
			t.Errorf("行0 %s=%q, want %q", name, gw[0], gw[1])
		}
	}

	r1 := got[1]
	checks1 := map[string][2]string{
		"TriggeredAt": {r1.TriggeredAt, "2026-04-20 13:15"},
		"MetricLabel": {r1.MetricLabel, "湿度"},
		"Condition":   {r1.Condition, "< 30.00%"},
		"ActualValue": {r1.ActualValue, "25.00%"},
		"Notified":    {r1.Notified, "未"},
	}
	for name, gw := range checks1 {
		if gw[0] != gw[1] {
			t.Errorf("行1 %s=%q, want %q", name, gw[0], gw[1])
		}
	}
}

// --- 3.1 buildAlertHistoryDeviceOptions: デバイス → select 選択肢 ---

// TestBuildAlertHistoryDeviceOptions は、selectedID と一致するデバイスにのみ Selected を立て、
// 全デバイス("")時はどれも Selected でないことを検証する (R5.1)。
func TestBuildAlertHistoryDeviceOptions(t *testing.T) {
	devices := []repository.Device{
		{ID: 1, Name: "ハウスA温湿度計"},
		{ID: 2, Name: "ハウスB温湿度計"},
	}

	got := buildAlertHistoryDeviceOptions(devices, "2")
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2", len(got))
	}
	if got[0].Selected {
		t.Errorf("device1 が Selected (selectedID=2)")
	}
	if !got[1].Selected {
		t.Errorf("device2 が Selected でない (selectedID=2)")
	}
	if got[1].Name != "ハウスB温湿度計" || got[1].ID != 2 {
		t.Errorf("device2 の写像が不正: %+v", got[1])
	}

	none := buildAlertHistoryDeviceOptions(devices, "")
	if none[0].Selected || none[1].Selected {
		t.Errorf("全デバイス選択中なのに Selected が立っている: %+v", none)
	}
}

// --- 3.1 alertHistoryURL / buildAlertHistoryPagination ---

// TestAlertHistoryURL_条件保持しpage差替 は、device_id/from/to を保持し page のみ差し替えた
// 相対 URL を生成することを検証する (R3.2)。
func TestAlertHistoryURL_条件保持しpage差替(t *testing.T) {
	got := alertHistoryURL("2", "2026-04-01", "2026-04-20", 2)

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("生成 URL のパース失敗: %v (%q)", err, got)
	}
	if u.Path != "/alerts/history" {
		t.Errorf("path=%q, want /alerts/history", u.Path)
	}
	q := u.Query()
	if q.Get("device_id") != "2" {
		t.Errorf("device_id=%q, want 2", q.Get("device_id"))
	}
	if q.Get("from") != "2026-04-01" {
		t.Errorf("from=%q, want 2026-04-01", q.Get("from"))
	}
	if q.Get("to") != "2026-04-20" {
		t.Errorf("to=%q, want 2026-04-20", q.Get("to"))
	}
	if q.Get("page") != "2" {
		t.Errorf("page=%q, want 2", q.Get("page"))
	}
}

// TestAlertHistoryURL_空条件はpageのみ は、device_id/from/to 未指定時に page のみを持つことを検証する。
func TestAlertHistoryURL_空条件はpageのみ(t *testing.T) {
	got := alertHistoryURL("", "", "", 1)

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("生成 URL のパース失敗: %v (%q)", err, got)
	}
	q := u.Query()
	if q.Get("page") != "1" {
		t.Errorf("page=%q, want 1", q.Get("page"))
	}
	for _, key := range []string{"device_id", "from", "to"} {
		if _, ok := q[key]; ok {
			t.Errorf("%s が空指定で URL に含まれている: %q", key, got)
		}
	}
}

// TestBuildAlertHistoryPagination_番号と現在と条件保持 は、1..last の番号リンク・現在ページ判定・
// 前後 URL の条件保持を検証する (R3.1/3.2)。
func TestBuildAlertHistoryPagination_番号と現在と条件保持(t *testing.T) {
	v := buildAlertHistoryPagination("2", "2026-04-01", "2026-04-20", 2, 3)

	if !v.HasPrev || !v.HasNext {
		t.Errorf("中間ページ(2/3)は前後とも有効: HasPrev=%v HasNext=%v", v.HasPrev, v.HasNext)
	}
	if len(v.Pages) != 3 {
		t.Fatalf("Pages len=%d, want 3", len(v.Pages))
	}
	if v.Pages[0].Current || !v.Pages[1].Current || v.Pages[2].Current {
		t.Errorf("Current 判定が不正: %+v", v.Pages)
	}
	if v.Pages[0].Num != 1 || v.Pages[2].Num != 3 {
		t.Errorf("Num が不正: %+v", v.Pages)
	}
	// 各リンクが検索条件を保持。
	if !strings.Contains(v.Pages[0].URL, "device_id=2") || !strings.Contains(v.Pages[0].URL, "from=2026-04-01") {
		t.Errorf("番号リンクが条件保持していない: %q", v.Pages[0].URL)
	}
	if !strings.Contains(v.PrevURL, "page=1") {
		t.Errorf("PrevURL=%q, want page=1 を含む", v.PrevURL)
	}
	if !strings.Contains(v.NextURL, "page=3") {
		t.Errorf("NextURL=%q, want page=3 を含む", v.NextURL)
	}
}

// TestBuildAlertHistoryPagination_端ページで前後無効 は、先頭で前へ無効・最終で次へ無効を検証する
// (R3.5/3.6)。
func TestBuildAlertHistoryPagination_端ページで前後無効(t *testing.T) {
	first := buildAlertHistoryPagination("", "", "", 1, 3)
	if first.HasPrev {
		t.Errorf("先頭ページで HasPrev=true")
	}
	if !first.HasNext {
		t.Errorf("先頭ページで HasNext=false")
	}

	last := buildAlertHistoryPagination("", "", "", 3, 3)
	if !last.HasPrev {
		t.Errorf("最終ページで HasPrev=false")
	}
	if last.HasNext {
		t.Errorf("最終ページで HasNext=true")
	}
}
