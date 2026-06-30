package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
)

// device_show_heatstress_test.go は高温ストレスパネル組立 (buildHeatStressPanel・period 非連動) を
// Querier 手書きモックで DB 非依存に検証する (タスク 5.1〜5.4)。
// 空データ→空パネル (5.1)、熱帯夜判定/連続日数/欠測非該当 (5.2)、THI 時間帯×日/AH/現在カード (5.3)、
// 年間日数トレンド (5.4) を網羅する。

// heatDailyJSTRow は ListDailySensorAggregatesJST の1行 (JST 暦日 + 日次 max/min 気温) を作る。
// max/min は本番 SQL で明示キャストが無く interface{} ゆえ float64 を渡す (aggregateToFloat が処理)。
func heatDailyJSTRow(date time.Time, maxT, minT float64) repository.ListDailySensorAggregatesJSTRow {
	return repository.ListDailySensorAggregatesJSTRow{
		ReadingDate:    pgtype.Date{Time: date, Valid: true},
		MaxTemperature: maxT,
		MinTemperature: minT,
	}
}

// heatRawReading は生行 (計測時刻 + 温湿度) を作る。
func heatRawReading(recordedAt time.Time, temp, hum float64) repository.SensorReading {
	return repository.SensorReading{
		RecordedAt:  pgconv.Timestamptz(recordedAt),
		Temperature: pgconv.Numeric2(temp),
		Humidity:    pgconv.Numeric2(hum),
	}
}

// --- 5.1 空データ → 空パネル + Guidance ---

func TestBuildHeatStressPanel_空データはHasDataFalseとGuidance(t *testing.T) {
	repo := showDeviceRepo() // 日次・生行とも空
	h := &DeviceHandler{Repo: repo}
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, jst)

	v, err := h.buildHeatStressPanel(context.Background(), repo.devices[1], now)
	if err != nil {
		t.Fatalf("buildHeatStressPanel() error: %v", err)
	}
	if v.HasData {
		t.Error("空データで HasData=true (false であるべき)")
	}
	if v.Guidance == "" {
		t.Error("空データで Guidance が空 (導線注記が必要)")
	}
	// カードは全項目 "—"。
	for _, got := range []string{v.Card.CurrentTHI, v.Card.CurrentAH, v.Card.LatestNightTemp, v.Card.SenSlopeSign} {
		if got != "—" {
			t.Errorf("空データでカード値が %q (— であるべき)", got)
		}
	}
}

// --- 5.2 熱帯夜判定・連続日数・欠測非該当 ---

// heatGapRepo は直近6日のうち1日 (06-27) を欠測 (行なし) にした日次を備える。
// 06-25=26, 06-26=27, [06-27 欠測], 06-28=25(閾値ちょうど), 06-29=26, 06-30=26 (すべて夜温≥25)。
// 欠測を正しく非該当 (run 切れ) 扱いすれば 最長=3 (06-28〜30)・現在=3。欠測を無視すると最長=5 になる。
func heatGapRepo() *fakeDeviceRepo {
	repo := showDeviceRepo()
	repo.dailyAggsJST = []repository.ListDailySensorAggregatesJSTRow{
		heatDailyJSTRow(dateOnlyUTC(2026, 6, 25), 32, 26),
		heatDailyJSTRow(dateOnlyUTC(2026, 6, 26), 33, 27),
		// 06-27 は欠測 (行なし)
		heatDailyJSTRow(dateOnlyUTC(2026, 6, 28), 31, 25),
		heatDailyJSTRow(dateOnlyUTC(2026, 6, 29), 32, 26),
		heatDailyJSTRow(dateOnlyUTC(2026, 6, 30), 32, 26),
	}
	return repo
}

func TestBuildHeatStressPanel_熱帯夜連続日数は欠測で切れる(t *testing.T) {
	repo := heatGapRepo()
	h := &DeviceHandler{Repo: repo}
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, jst)

	v, err := h.buildHeatStressPanel(context.Background(), repo.devices[1], now)
	if err != nil {
		t.Fatalf("buildHeatStressPanel() error: %v", err)
	}
	if !v.HasData {
		t.Fatal("日次データ有りで HasData=false")
	}
	// 欠測 (06-27) で run が切れ、最長=3 (06-28〜30)・現在=3。欠測を無視すると 5 になる。
	if v.TropicalLongest != "3 日" {
		t.Errorf("TropicalLongest = %q, want \"3 日\" (欠測で run が切れるはず)", v.TropicalLongest)
	}
	if v.TropicalCurrent != "3 日" {
		t.Errorf("TropicalCurrent = %q, want \"3 日\"", v.TropicalCurrent)
	}
	// 熱帯夜カレンダー・夜温/ΔT は系列が組まれる (空 option "{}" でない)。
	if v.CalendarJSON == "{}" || v.CalendarJSON == "" {
		t.Errorf("CalendarJSON が空: %q", v.CalendarJSON)
	}
	if !strings.Contains(v.CalendarJSON, "2026-06-30") {
		t.Errorf("CalendarJSON に直近日が無い: %s", v.CalendarJSON)
	}
	if v.NightDeltaJSON == "{}" || v.NightDeltaJSON == "" {
		t.Errorf("NightDeltaJSON が空: %q", v.NightDeltaJSON)
	}
	// 直近夜温カード = 06-30 の日最低 26 → "26.0℃"。
	if !strings.Contains(v.Card.LatestNightTemp, "26.0") {
		t.Errorf("LatestNightTemp = %q, want 26.0 を含む", v.Card.LatestNightTemp)
	}
}

// --- 5.3 THI 時間帯×日・AH・現在カード ---

func TestBuildHeatStressPanel_THI時間帯とAHと現在カード(t *testing.T) {
	repo := showDeviceRepo()
	// 直近の生行 (昇順=最後が最新)。最新 28℃/65% → THI=0.8*28+0.65*13.6+46.4=77.64 → "77.6"。
	repo.recentReadings = []repository.SensorReading{
		heatRawReading(time.Date(2026, 6, 30, 3, 0, 0, 0, jst), 26, 80),
		heatRawReading(time.Date(2026, 6, 30, 14, 0, 0, 0, jst), 28, 65),
	}
	h := &DeviceHandler{Repo: repo}
	now := time.Date(2026, 6, 30, 15, 0, 0, 0, jst)

	v, err := h.buildHeatStressPanel(context.Background(), repo.devices[1], now)
	if err != nil {
		t.Fatalf("buildHeatStressPanel() error: %v", err)
	}
	if !v.HasData {
		t.Fatal("生行有りで HasData=false")
	}
	// THI 時間帯×日ヒートマップ・AH line が組まれる。
	if !strings.Contains(v.THIHeatmapJSON, "heatmap") {
		t.Errorf("THIHeatmapJSON に heatmap が無い: %s", v.THIHeatmapJSON)
	}
	if !strings.Contains(v.AHJSON, "line") {
		t.Errorf("AHJSON に line が無い: %s", v.AHJSON)
	}
	// 現在カード: THI=最新点 77.6・AH=最新点 (非"—"・g/m³)。
	if !strings.Contains(v.Card.CurrentTHI, "77.6") {
		t.Errorf("CurrentTHI = %q, want 77.6 を含む", v.Card.CurrentTHI)
	}
	if v.Card.CurrentAH == "—" || !strings.Contains(v.Card.CurrentAH, "g/m³") {
		t.Errorf("CurrentAH = %q, want g/m³ 付き実数", v.Card.CurrentAH)
	}
}

// --- 5.4 年間日数トレンド (Sen 傾き再利用・検出力の留保) ---

// heatTrendRepo は各年に cnt 個の熱帯夜 (min26) と 1 個の冬日 (min12=年の存在保証) を備える。
func heatTrendRepo(yearCounts map[int]int) *fakeDeviceRepo {
	repo := showDeviceRepo()
	var rows []repository.ListDailySensorAggregatesJSTRow
	for y, cnt := range yearCounts {
		for i := 0; i < cnt; i++ {
			rows = append(rows, heatDailyJSTRow(dateOnlyUTC(y, 7, i+1), 32, 26))
		}
		rows = append(rows, heatDailyJSTRow(dateOnlyUTC(y, 1, 15), 18, 12)) // 冬日 (非熱帯夜・年の存在)
	}
	repo.dailyAggsJST = rows
	return repo
}

func TestBuildHeatStressPanel_多年でトレンドとSen符号(t *testing.T) {
	repo := heatTrendRepo(map[int]int{2024: 2, 2025: 3, 2026: 4}) // counts=[2,3,4] 増加
	h := &DeviceHandler{Repo: repo}
	now := time.Date(2026, 12, 31, 12, 0, 0, 0, jst)

	v, err := h.buildHeatStressPanel(context.Background(), repo.devices[1], now)
	if err != nil {
		t.Fatalf("buildHeatStressPanel() error: %v", err)
	}
	if !v.HasTrend {
		t.Fatal("多年蓄積で HasTrend=false (true であるべき)")
	}
	if !strings.Contains(v.TrendJSON, "bar") || !strings.Contains(v.TrendJSON, "markLine") {
		t.Errorf("TrendJSON に bar/markLine が無い: %s", v.TrendJSON)
	}
	// Sen 傾き＋符号: 増加ゆえ "+" を含み、参考である旨。
	if !strings.Contains(v.Card.SenSlopeSign, "+") {
		t.Errorf("SenSlopeSign = %q, want + (増加傾向)", v.Card.SenSlopeSign)
	}
	// 検出力の留保注記 (非有意≠トレンド無し・複数年必要)。
	if v.TrendNote == "" {
		t.Error("TrendNote が空 (検出力の留保注記が必要)")
	}
}

func TestBuildHeatStressPanel_減少傾向はSen符号マイナス(t *testing.T) {
	repo := heatTrendRepo(map[int]int{2024: 5, 2025: 3, 2026: 1}) // counts=[5,3,1] 減少
	h := &DeviceHandler{Repo: repo}
	now := time.Date(2026, 12, 31, 12, 0, 0, 0, jst)

	v, err := h.buildHeatStressPanel(context.Background(), repo.devices[1], now)
	if err != nil {
		t.Fatalf("buildHeatStressPanel() error: %v", err)
	}
	if !strings.Contains(v.Card.SenSlopeSign, "-") || !strings.Contains(v.Card.SenSlopeSign, "減少") {
		t.Errorf("SenSlopeSign = %q, want - と減少傾向", v.Card.SenSlopeSign)
	}
}

func TestBuildHeatStressPanel_横ばいはSen符号ゼロ(t *testing.T) {
	repo := heatTrendRepo(map[int]int{2024: 3, 2025: 3, 2026: 3}) // counts=[3,3,3] 横ばい
	h := &DeviceHandler{Repo: repo}
	now := time.Date(2026, 12, 31, 12, 0, 0, 0, jst)

	v, err := h.buildHeatStressPanel(context.Background(), repo.devices[1], now)
	if err != nil {
		t.Fatalf("buildHeatStressPanel() error: %v", err)
	}
	if !strings.Contains(v.Card.SenSlopeSign, "横ばい") {
		t.Errorf("SenSlopeSign = %q, want 横ばい", v.Card.SenSlopeSign)
	}
}

func TestBuildHeatStressPanel_1年はトレンド無し(t *testing.T) {
	repo := heatTrendRepo(map[int]int{2026: 4}) // 1年のみ
	h := &DeviceHandler{Repo: repo}
	now := time.Date(2026, 12, 31, 12, 0, 0, 0, jst)

	v, err := h.buildHeatStressPanel(context.Background(), repo.devices[1], now)
	if err != nil {
		t.Fatalf("buildHeatStressPanel() error: %v", err)
	}
	if v.HasTrend {
		t.Error("1年のみで HasTrend=true (false であるべき)")
	}
	if v.Card.SenSlopeSign != "—" {
		t.Errorf("SenSlopeSign = %q, want — (1年では傾き断定しない)", v.Card.SenSlopeSign)
	}
	if v.TrendNote == "" {
		t.Error("TrendNote が空 (複数年必要の注記が必要)")
	}
}

func TestBuildHeatStressPanel_熱帯夜0年タイで破綻しない(t *testing.T) {
	repo := heatTrendRepo(map[int]int{2024: 0, 2025: 0, 2026: 5}) // counts=[0,0,5] タイあり
	h := &DeviceHandler{Repo: repo}
	now := time.Date(2026, 12, 31, 12, 0, 0, 0, jst)

	v, err := h.buildHeatStressPanel(context.Background(), repo.devices[1], now)
	if err != nil {
		t.Fatalf("buildHeatStressPanel() error: %v", err)
	}
	if !v.HasTrend {
		t.Error("3年蓄積 (0年タイ含む) で HasTrend=false")
	}
}

// --- DB エラー伝播 ---

func TestBuildHeatStressPanel_日次クエリエラーは伝播(t *testing.T) {
	repo := showDeviceRepo()
	repo.dailyJSTErr = errors.New("db down")
	h := &DeviceHandler{Repo: repo}
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, jst)

	if _, err := h.buildHeatStressPanel(context.Background(), repo.devices[1], now); err == nil {
		t.Error("日次クエリ error が伝播していない (500 経路)")
	}
}

// --- Integration: device-show 全体描画への配線（Task 7） ---

// heatShowRepo は所有者(7)・デバイス1に、time.Now() 相対の生行＋JST日次を備える
// （Show は now=time.Now() ゆえ、直近窓に収まるよう実時刻相対で作る）。
func heatShowRepo() *fakeDeviceRepo {
	repo := showDeviceRepo()
	now := time.Now()
	repo.recentReadings = []repository.SensorReading{
		heatRawReading(now.Add(-3*time.Hour), 27, 75),
		heatRawReading(now.Add(-1*time.Hour), 29, 68),
	}
	repo.dailyAggsJST = []repository.ListDailySensorAggregatesJSTRow{
		heatDailyJSTRow(now.AddDate(0, 0, -3), 33, 27),
		heatDailyJSTRow(now.AddDate(0, 0, -2), 32, 26),
		heatDailyJSTRow(now.AddDate(0, 0, -1), 31, 25),
	}
	return repo
}

func TestShow_高温ストレスパネルがフルページに描画される(t *testing.T) {
	repo := heatShowRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := requestWithHeaders(r, http.MethodGet, "/devices/1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	// 高温ストレスパネルの器がフルページに現れる（GDD パネルの下・period 非連動の兄弟）。
	for _, marker := range []string{
		"高温ストレス（THI・熱帯夜・絶対湿度・日較差）",
		`id="thi-heatmap"`,
		`id="tropical-night-calendar"`,
		`id="ah-line"`,
		"heat-scale",
	} {
		if !strings.Contains(body, marker) {
			t.Errorf("フルページに高温ストレスの %q が無い", marker)
		}
	}
	// calendar/heatmap は connect 連動に混ざらない（data-no-connect・Q2）。
	if !strings.Contains(body, "data-no-connect") {
		t.Error("高温ストレスチャートに data-no-connect が無い（connect 連動から除外できていない）")
	}
}

func TestChart_高温ストレスパネルは期間フラグメントに含まれない(t *testing.T) {
	repo := heatShowRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	// 期間切替（部分更新）フラグメント。高温ストレスは period 非連動ゆえ再計算・再描画されない。
	w := requestWithHeaders(r, http.MethodGet, "/devices/1/chart?period=3d", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	for _, absent := range []string{
		`id="thi-heatmap"`,
		`id="tropical-night-calendar"`,
		"高温ストレス（THI・熱帯夜・絶対湿度・日較差）",
	} {
		if strings.Contains(body, absent) {
			t.Errorf("期間フラグメントに高温ストレスの %q が混入（period 非連動のはず）", absent)
		}
	}
}

func TestShow_他者deviceは高温ストレスも含め404(t *testing.T) {
	repo := heatShowRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 8) // 非所有ユーザー

	w := requestWithHeaders(r, http.MethodGet, "/devices/1", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404（他者 device は列挙防止）", w.Code)
	}
	if strings.Contains(w.Body.String(), `id="thi-heatmap"`) {
		t.Error("404 応答に高温ストレスパネルが漏れている")
	}
}
