package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
)

// device_show_gdd_integration_test.go は GDD 機能の重要フローを HTTP フルスタック (Show→page 描画) で
// 統合検証する (タスク 7.1) ＋ 既存可視化の無回帰と GDD の connect 非参加を検証する (タスク 7.2)。
// Querier 手書きモック (fakeDeviceRepo) で DB 非依存・httptest+gin・templ レンダリング文字列アサート。

// fullPanelsRepo は5チャート全て (温度/湿度/VPD/露点 + GDD) が描画される所有者(7)・デバイス1 を備えた fake。
// crop=rice・定植日2026-04-01・日次気温 (GDD)・期間内の生データ (温湿度/VPD/露点) を全て持つ。
func fullPanelsRepo() *fakeDeviceRepo {
	repo := gddRepo() // crop=rice・定植日・dailyAggs (GDD パネル)
	// 温湿度2グラフ・VPD・露点パネル用の期間内生データ (HasData=true にする)。
	repo.recentReadings = []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 25, 50),
		sensorRow(1, time.Date(2026, 4, 20, 3, 30, 0, 0, time.UTC), 20, 70),
		sensorRow(1, time.Date(2026, 4, 20, 21, 0, 0, 0, time.UTC), 10, 100),
	}
	return repo
}

// ===== 7.1 GDD 統合テスト（重要フロー） =====

// 定植日設定済み米のフルページに、GDD 累積曲線・予測到達 markLine・各カード・生育ステージ表が描画される。
func TestGDDIntegration_米フルページにGDD要素群が描画される(t *testing.T) {
	r := newShowRouterWithUser(&DeviceHandler{Repo: gddRepo()}, 7)
	w := getPath(r, "/devices/1")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()

	// 累積曲線の器 (connect 除外マーカー付き)。
	for _, marker := range []string{
		`id="gdd-chart"`,
		"data-no-connect",
		`data-unit="℃·日"`,
		`<script type="application/json" id="gdd-chart-option">`,
	} {
		if !strings.Contains(body, marker) {
			t.Errorf("GDD 器 %q が描画されていない", marker)
		}
	}

	// option JSON に目標 markLine (yAxis) と予測到達 markLine (xAxis)・予測点 markPoint が含まれる。
	// gddRepo は傾き>0・未到達ゆえ HasForecast=true で予測マークが出る。
	for _, marker := range []string{`"markLine"`, `"yAxis"`, `"xAxis"`, `"markPoint"`} {
		if !strings.Contains(body, marker) {
			t.Errorf("GDD option に %q が含まれない (予測到達マークが出ていない)", marker)
		}
	}

	// 数値カードのラベルと具体値 (累積60・残り1340・予測収穫日・経過2日・現在ステージ発芽)。
	for _, marker := range []string{
		"現在累積GDD", "残り積算温度", "予測収穫日", "現在ステージ",
		"60 ℃·日", "1340 ℃·日", "2026-06-09", "発芽",
	} {
		if !strings.Contains(body, marker) {
			t.Errorf("GDD カード %q が描画されていない", marker)
		}
	}

	// 生育ステージ表 (5段の段名としきい値)。
	for _, marker := range []string{"生育ステージ（GDD 対応）", "分げつ", "出穂", "登熟", "収穫", "1400 ℃·日"} {
		if !strings.Contains(body, marker) {
			t.Errorf("生育ステージ表の %q が描画されていない", marker)
		}
	}

	// 近似注記 (予測が線形外挿の目安である旨・R3.4)。
	if !strings.Contains(body, "線形外挿による目安") {
		t.Errorf("近似注記が描画されていない")
	}
}

// 期間切替 (24h↔30d) で GDD パネル部分の HTML が不変 (period 非連動・R6.2)。
func TestGDDIntegration_期間切替でGDDパネル不変(t *testing.T) {
	r1 := newShowRouterWithUser(&DeviceHandler{Repo: fullPanelsRepo()}, 7)
	r2 := newShowRouterWithUser(&DeviceHandler{Repo: fullPanelsRepo()}, 7)

	w24 := getPath(r1, "/devices/1?period=24h")
	w30 := getPath(r2, "/devices/1?period=30d")
	if w24.Code != http.StatusOK || w30.Code != http.StatusOK {
		t.Fatalf("status 24h=%d 30d=%d, want 200", w24.Code, w30.Code)
	}

	gdd24 := gddSectionOf(t, w24.Body.String())
	gdd30 := gddSectionOf(t, w30.Body.String())
	if gdd24 != gdd30 {
		t.Errorf("GDD パネル部分が period で変化 (非連動に反する)\n24h:\n%s\n30d:\n%s", gdd24, gdd30)
	}
}

// 前提欠落 (作物に GDD モデルなし or 定植日 NULL) は導線注記へ縮退し、#gdd-chart を出さない (R6.3)。
func TestGDDIntegration_前提欠落は導線注記(t *testing.T) {
	repo := fullPanelsRepo()
	d := repo.devices[1]
	d.PlantingDate = pgtype.Date{Valid: false} // 定植日 NULL
	repo.devices[1] = d
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "作物と定植日を設定すると") {
		t.Errorf("前提欠落の導線注記が描画されていない")
	}
	if strings.Contains(body, `id="gdd-chart"`) {
		t.Errorf("前提欠落で #gdd-chart が描画されている (縮退すべき)")
	}
}

// 定植日以降データ0件はデータ未到着へ縮退する (R6.4)。
func TestGDDIntegration_空データは未到着(t *testing.T) {
	repo := fullPanelsRepo()
	repo.dailyAggs = nil // 定植日以降の日次集計が空
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "計測データがまだありません") {
		t.Errorf("データ未到着の注記が描画されていない")
	}
}

// ===== 7.2 既存可視化の無回帰 + GDD の connect 非参加 =====

// GDD チャートのみ data-no-connect を持ち、既存4チャート (温度/湿度/VPD/露点) は持たない。
// → GDD は echarts.connect グループに含まれない (R7.4)。既存4チャートは従来どおり連動する。
func TestGDDIntegration_GDDのみconnect非参加で既存4チャート維持(t *testing.T) {
	r := newShowRouterWithUser(&DeviceHandler{Repo: fullPanelsRepo()}, 7)
	w := getPath(r, "/devices/1")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()

	// 既存4チャートの器が無回帰で存在する。
	for _, id := range []string{
		`id="temperature-chart"`, `id="humidity-chart"`, `id="vpd-chart"`, `id="dewpoint-chart"`,
	} {
		if !strings.Contains(body, id) {
			t.Errorf("既存チャート %q が描画されていない (無回帰崩れ)", id)
		}
	}
	// GDD チャートも存在する。
	if !strings.Contains(body, `id="gdd-chart"`) {
		t.Errorf("GDD チャートが描画されていない")
	}

	// data-no-connect は gdd-chart の器にのみ付く (既存チャートには付かない)。
	// ※ ページ全体の文字列カウントは App.templ の initScope JS (コメント＋hasAttribute 判定) が
	//   "data-no-connect" を含むため使えない。器 (<div>) 単位で属性の有無を検査する。
	gddDiv := chartDivOf(t, body, "gdd-chart")
	if !strings.Contains(gddDiv, "data-no-connect") {
		t.Errorf("gdd-chart に data-no-connect が付いていない: %q", gddDiv)
	}
	for _, id := range []string{"temperature-chart", "humidity-chart", "vpd-chart", "dewpoint-chart"} {
		if div := chartDivOf(t, body, id); strings.Contains(div, "data-no-connect") {
			t.Errorf("%s に data-no-connect が付いている (connect 連動から外れてしまう): %q", id, div)
		}
	}
}

// chartDivOf は id="<chartID>" を含む <div ...> 開始タグ部分を返す (data-no-connect の有無検査用)。
func chartDivOf(t *testing.T, html, chartID string) string {
	t.Helper()
	anchor := `id="` + chartID + `"`
	idx := strings.Index(html, anchor)
	if idx < 0 {
		t.Fatalf("%q が見つからない", anchor)
	}
	// 直前の "<div" から直後の ">" までを開始タグとして切り出す。
	start := strings.LastIndex(html[:idx], "<div")
	end := strings.Index(html[idx:], ">")
	if start < 0 || end < 0 {
		t.Fatalf("%q を囲む <div ...> が見つからない", anchor)
	}
	return html[start : idx+end+1]
}
