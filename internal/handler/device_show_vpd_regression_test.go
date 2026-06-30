package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/chart"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// device_show_vpd_regression_test.go は VPD 追加後の詳細画面/期間切替フラグメントについて
// (1) VPD パネル描画、(2) 温湿度可視化の無回帰 (option 文字列のバイト一致を含む)、(3) 空データ分岐、
// (4) 所有者認可 (列挙防止) を統合的に固定する (タスク 6.1)。
// Querier 手書きモック (fakeDeviceRepo) で DB 非依存・httptest+gin・templ レンダリング文字列アサート。

// vpdRegressionRepo は所有者(7)・デバイス1(crop=goya)・複数点の生データを備えた fake を返す。
func vpdRegressionRepo() *fakeDeviceRepo {
	repo := showDeviceRepo()
	cropStr := "goya"
	d := repo.devices[1]
	d.Crop = &cropStr
	repo.devices[1] = d
	// 7d/30d は可視窓スライスのため now 基準の直近行を使う (固定過去日だと可視窓0件になる)。
	// 相対スパン (0/+30分/+18時間) を保持し、従来同様 18時間ジャンプの欠測ギャップを発生させる。
	base := time.Now().Add(-19 * time.Hour) // 最古点でも 24h ビュー内に収める
	repo.recentReadings = []repository.SensorReading{
		sensorRow(1, base, 25, 50),                      // 高VPD=乾きすぎ超過
		sensorRow(1, base.Add(30*time.Minute), 20, 70),  // 適正
		sensorRow(1, base.Add(18*time.Hour), 10, 100),   // 低VPD=湿りすぎ (18時間ギャップ)
	}
	return repo
}

// expectedTempHumOptions は buildChartArea が温湿度 option を組むのと同一経路で期待 option を再計算する
// (overlaySpec→ChartOptionJSON)。VPD 追加で温湿度 option が変わっていないことのバイト一致検証に使う。
func expectedTempHumOptions(t *testing.T, rows []repository.SensorReading, period string) (tempOpt, humOpt string) {
	t.Helper()
	label := rawLabelFor(period)
	labels := make([]string, len(rows))
	temps := make([]float64, len(rows))
	hums := make([]float64, len(rows))
	for i, r := range rows {
		labels[i] = label(r.RecordedAt)
		temps[i] = pgconv.NumericToFloat(r.Temperature)
		hums[i] = pgconv.NumericToFloat(r.Humidity)
	}
	window := smaWindowFor(period)
	tempSpec := overlaySpec(labels, temps, tempChartUnit, tempLineColor, window)
	humSpec := overlaySpec(labels, hums, humidityChartUnit, humidityLineColor, window)
	// buildChartArea と同一に日スケール SMA を付与する (sma-window-select 追加後の期待値)。
	// 本ヘルパの rows は全可視窓 (ルックバックなし=visibleStart 0) を前提とするため、
	// fullValues=temps/hums・visibleStart=0 で buildChartArea を忠実にミラーする。
	if windows := dayScaleWindowsFor(period); len(windows) > 0 {
		ppd := estimatePointsPerDay(rows)
		tempSpec.DaySMAs = daySMASeriesFor(windows, temps, ppd, 0)
		humSpec.DaySMAs = daySMASeriesFor(windows, hums, ppd, 0)
	}
	// buildChartArea と同一の欠測ギャップ配線をミラーする (data-quality-meta 追加後の期待値)。
	// この回帰テストの意図は「VPD 追加が温湿度 option を変えない」= 温湿度パイプライン独立性であり、
	// 欠測ありデータでは温湿度 option も gap グリッドを通す (それが正しい温湿度パイプライン出力)。
	if _, _, gaps, ok := chart.MissingStats(intervalSeconds(rows)); ok && len(gaps) > 0 {
		slotsAfter := make([]int, len(rows))
		for _, g := range gaps {
			slotsAfter[g.StartIdx] = g.MissingSlots
		}
		tempSpec = applyGapGrid(tempSpec, slotsAfter)
		humSpec = applyGapGrid(humSpec, slotsAfter)
	}
	tempOpt, err := chart.ChartOptionJSON(tempSpec)
	if err != nil {
		t.Fatalf("期待温度 option の構築に失敗: %v", err)
	}
	humOpt, err = chart.ChartOptionJSON(humSpec)
	if err != nil {
		t.Fatalf("期待湿度 option の構築に失敗: %v", err)
	}
	return tempOpt, humOpt
}

// --- 6.1 詳細表示: VPD パネル描画 + 温湿度無回帰 (option バイト一致) ---

func TestRegression_Show_VPDパネルと温湿度可視化が共存(t *testing.T) {
	repo := vpdRegressionRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()

	// VPD パネル (kPa チャート・作物名/適正帯・滞在率バー・時間帯別逸脱)。
	for _, want := range []string{
		`id="vpd-chart"`, `data-unit="kPa"`, `id="vpd-chart-option"`,
		"ゴーヤ", "適正帯", "適正帯滞在率", `class="vpd-bar"`, "時間帯別 VPD 逸脱",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("VPD パネルに %q が含まれていない", want)
		}
	}

	// 温湿度2グラフ・数値カード・ECharts 配線が従来同等 (無回帰)。
	for _, want := range []string{
		`id="temperature-chart"`, `id="humidity-chart"`,
		`id="temperature-chart-option"`, `id="humidity-chart-option"`,
		"温度サマリ", "湿度サマリ", "data-echarts",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("温湿度可視化に %q が含まれていない (無回帰崩れ)", want)
		}
	}

	// 温湿度 option 文字列が VPD 追加前と完全一致する (バイト一致の無回帰ガード)。
	tempOpt, humOpt := expectedTempHumOptions(t, repo.recentReadings, "24h")
	if !strings.Contains(body, tempOpt) {
		t.Error("温度 option が従来と一致しない (VPD 追加で温湿度 option が変化した疑い)")
	}
	if !strings.Contains(body, humOpt) {
		t.Error("湿度 option が従来と一致しない")
	}

	// option script は温/湿/VPD/露点 + 高温ストレス(THI/熱帯夜calendar/夜温ΔT/AH) の 8 本
	// (heat-stress-thi パネル追加・温湿度/VPD option は不変)。
	if got := strings.Count(body, `type="application/json"`); got != 8 {
		t.Errorf("option script 数 = %d, want 8 (温度/湿度/VPD/露点/THI/熱帯夜/夜温ΔT/AH)", got)
	}
}

// --- 6.1 期間切替フラグメント: VPD と温湿度が当該期間で更新・日次表・active 往復 ---

func TestRegression_Chart_期間切替でVPDと温湿度が更新(t *testing.T) {
	repo := vpdRegressionRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := requestWithHeaders(r, http.MethodGet, "/devices/1/chart?period=7d", map[string]string{"HX-Request": "true"})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()

	// フラグメントに VPD パネルと温湿度グラフの双方が含まれ、7日間が active。
	for _, want := range []string{`id="vpd-chart"`, `id="temperature-chart"`, `id="humidity-chart"`, "時間帯別 VPD 逸脱"} {
		if !strings.Contains(body, want) {
			t.Errorf("期間切替フラグメントに %q が含まれていない", want)
		}
	}
	if !activeButtonHas(body, "7日間") {
		t.Error("7日間 がアクティブでない")
	}
	// 複数日 (7d) は日次集計表を出す (24h は出さない・無回帰)。
	if !strings.Contains(body, "日次集計（温度）") {
		t.Error("複数日で日次集計表が描画されていない")
	}
	// 温湿度 option は 7d 経路でも従来と一致 (バイト一致)。
	tempOpt, _ := expectedTempHumOptions(t, repo.recentReadings, "7d")
	if !strings.Contains(body, tempOpt) {
		t.Error("7d の温度 option が従来と一致しない")
	}
	// フラグメントはレイアウト非包含。
	if strings.Contains(body, "<html") || strings.Contains(body, "site-header") {
		t.Error("フラグメントにレイアウト要素が含まれている")
	}
}

// --- 6.1 空データ: VPD 非表示 + プレースホルダ + カードはダッシュ (無回帰) ---

func TestRegression_空データでVPD非表示とプレースホルダ(t *testing.T) {
	repo := showDeviceRepo()
	repo.recentReadings = nil // 0 件
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, `id="vpd-chart"`) {
		t.Error("空データで VPD パネルが描画されている")
	}
	if !strings.Contains(body, "データはまだありません") {
		t.Error("空データプレースホルダが無い")
	}
	// 温湿度カードは "—" を保ちレイアウトを崩さない (無回帰)。
	if !strings.Contains(body, statEmptyMark) {
		t.Error("空データで数値カードのダッシュが無い")
	}
	// option script は 0 本 (温湿度も VPD も出さない)。
	if got := strings.Count(body, `type="application/json"`); got != 0 {
		t.Errorf("option script 数 = %d, want 0 (空データ)", got)
	}
}

// --- 6.1 認可: 非所有デバイスは VPD 含め 404 (列挙防止・無回帰) ---

func TestRegression_非所有はVPD含め404列挙防止(t *testing.T) {
	repo := vpdRegressionRepo()                                 // device1 の所有者は 7
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 999) // 別ユーザー

	for _, path := range []string{"/devices/1", "/devices/1/chart?period=24h"} {
		w := getPath(r, path)
		if w.Code != http.StatusNotFound {
			t.Errorf("非所有 GET %s = %d, want 404 (列挙防止)", path, w.Code)
		}
		if strings.Contains(w.Body.String(), `id="vpd-chart"`) {
			t.Errorf("非所有応答に VPD パネルが漏れている: %s", path)
		}
	}
}
