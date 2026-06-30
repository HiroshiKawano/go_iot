package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// device_show_sma_window_integration_test.go は sma-window-select の handler→chart→templ
// 結合・無回帰を HTTP 層 (Show/Chart 経由・httptest) で固定する (タスク5.1)。
// 採用案(a)=新規静的 UI なし・templ 無改修ゆえ、日スケール系列は option JSON 文字列の一部として
// HTML に流れることを end-to-end で検証する (グラフ内部の系列描画は 8.2 のモック反映例外)。

// smaWindowRows は now 基準で過去 days 日分を 12 時間間隔(ASC)で生成する。
// 7d/30d の可視窓に日スケール SMA 系列が出る密度を確保する (固定過去日だと可視窓0件になる)。
func smaWindowRows(days int) []repository.SensorReading {
	now := time.Now()
	var rows []repository.SensorReading
	for h := days * 24; h >= 0; h -= 12 { // 古い順 (クエリは recorded_at ASC)
		rows = append(rows, sensorRow(1, now.Add(-time.Duration(h)*time.Hour), 25.0, 60.0))
	}
	return rows
}

// --- 5.1 日スケール系列の有無 (Show 初期表示・全期間) -------------------------

// 初期表示 (Show フルページ) で、7d/30d は option script に「移動平均 N日」を含み、
// 24h/3d は含まないこと (buildChartArea 共有・短期ビューは窓なし)。
func TestShow_日スケールSMA系列は7d_30dのみoption_scriptに出る(t *testing.T) {
	tests := []struct {
		period  string
		wantDay bool
		want14d bool // 30d のみ 14日窓
	}{
		{"24h", false, false},
		{"3d", false, false},
		{"7d", true, false},
		{"30d", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			repo := showDeviceRepo()
			repo.recentReadings = smaWindowRows(32)
			r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

			body := getPath(r, "/devices/1?period="+tt.period).Body.String()

			// 「移動平均 7日」は 7d/30d の双方に出る日スケールラベル。24h/3d には出ない。
			if got := strings.Contains(body, "移動平均 7日"); got != tt.wantDay {
				t.Errorf("%s: 「移動平均 7日」含有=%v, want %v", tt.period, got, tt.wantDay)
			}
			// 「移動平均 14日」は 30d のみ。
			if got := strings.Contains(body, "移動平均 14日"); got != tt.want14d {
				t.Errorf("%s: 「移動平均 14日」含有=%v, want %v", tt.period, got, tt.want14d)
			}
		})
	}
}

// --- 5.1 期間切替フラグメント (Chart・#device-chart-area の中身) ---------------

// 期間切替フラグメント (7d) が #device-chart-area へ swap される中身を返し、日スケール系列を含み、
// 採用案(a)につき新規静的 UI (窓セレクタ等) を追加しない (期間ボタンは従来どおり4本・<select> なし)。
func TestChart_7dフラグメントは日スケール系列を含み器は無改修(t *testing.T) {
	repo := showDeviceRepo()
	repo.recentReadings = smaWindowRows(16)
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := hxGet(r, "/devices/1/chart?period=7d")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()

	// 日スケール系列 (7d は 3日/7日) が option script に含まれる。
	for _, label := range []string{"移動平均 3日", "移動平均 7日"} {
		if !strings.Contains(body, label) {
			t.Errorf("7d フラグメントに %q が無い", label)
		}
	}
	// #device-chart-area へ swap される中身 = フラグメント (レイアウト非包含・7日間 active)。
	if strings.Contains(body, "<html") || strings.Contains(body, "site-header") {
		t.Error("フラグメントにレイアウト要素が含まれている (#device-chart-area の中身でない)")
	}
	if !activeButtonHas(body, "7日間") {
		t.Error("7日間 がアクティブでない")
	}
	// 採用案(a): 新規静的 UI なし。期間ボタンは従来どおり4本、窓セレクタ <select> を追加しない。
	if got := strings.Count(body, "period-btn"); got != 4 {
		t.Errorf("期間ボタン数 (period-btn) = %d, want 4 (採用案(a)=新規 UI なし)", got)
	}
	if strings.Contains(body, "<select") {
		t.Error("窓セレクタ <select> が追加されている (採用案(a)=凡例トグルのみ・新規 UI なし)")
	}
}

// 30d で既存の点数窓 SMA (P2「移動平均」) と日スケール SMA (「移動平均 N日」) が共存し、
// いずれも既定オフ (selected:false) であること (R6.2 無回帰・R2.1 既定オフ)。
func TestChart_30dはP2移動平均と日スケール移動平均が共存し既定オフ(t *testing.T) {
	repo := showDeviceRepo()
	repo.recentReadings = smaWindowRows(40)
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	body := hxGet(r, "/devices/1/chart?period=30d").Body.String()

	// 既存オーバーレイ (P2 SMA/正常帯/乖離率) と日スケール3本がいずれも既定オフで legend.selected に並ぶ。
	for _, want := range []string{
		`"移動平均":false`, `"正常帯":false`, `"乖離率(%)":false`, // P2 (無回帰)
		`"移動平均 3日":false`, `"移動平均 7日":false`, `"移動平均 14日":false`, // 日スケール (既定オフ)
	} {
		if !strings.Contains(body, want) {
			t.Errorf("legend.selected に %q が無い (共存/既定オフ崩れ):\n%s", want, body)
		}
	}
	// 生実測線の markPoint(最高/最低) は据え置き (主役温存・無回帰)。
	for _, want := range []string{`"type":"max"`, `"type":"min"`} {
		if !strings.Contains(body, want) {
			t.Errorf("生実測線の markPoint %q が無い (無回帰崩れ)", want)
		}
	}
}

// 計測0件 (可視窓も空) のとき、7d でも「データはまだありません」を返し option script を出さない (6.4)。
func TestChart_7d計測0件はデータなしメッセージ(t *testing.T) {
	repo := showDeviceRepo()
	repo.recentReadings = nil // 0 件
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	body := hxGet(r, "/devices/1/chart?period=7d").Body.String()
	if !strings.Contains(body, "データはまだありません") {
		t.Error("計測0件の未到着メッセージが無い (6.4)")
	}
	if strings.Contains(body, `type="application/json"`) {
		t.Error("計測0件なのに option script が出ている")
	}
	if strings.Contains(body, "移動平均 7日") {
		t.Error("計測0件なのに日スケール系列が出ている")
	}
}
