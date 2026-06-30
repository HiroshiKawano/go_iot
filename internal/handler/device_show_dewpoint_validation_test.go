package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/domain"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// device_show_dewpoint_validation_test.go は露点・病害リスクパネル追加後の詳細画面/期間切替フラグメントを
// (1) 露点パネル描画・期間切替、(2) 既存可視化 (温湿度/VPD/ギャップ/品質) の無回帰、(3) 認可と研究用スコープ境界、
// (4) 物理規約の向き (湿り側=寒色) の符号化、として統合的に固定する (タスク 4.1〜4.4)。
// Querier 手書きモック (fakeDeviceRepo) で DB 非依存・httptest+gin・templ レンダリング文字列アサート。
// なお物理規約の向きの最終確認は実機スモーク目視 (タスク 4.4・手動) が別途必須 (project_vpd_physics_convention)。

// dewpointMultiDayRepo は所有者(7)・デバイス1(crop=goya)・2 暦日(JST)にまたがる高湿度データを備えた fake を返す。
// 露点日次表が複数行になり、結露帯 (spread 小) が発生する湿りデータ。
func dewpointMultiDayRepo() *fakeDeviceRepo {
	repo := showDeviceRepo()
	cropStr := "goya"
	d := repo.devices[1]
	d.Crop = &cropStr
	repo.devices[1] = d
	// 7d/30d は可視窓スライスのため now 基準の直近2暦日(JST)に置く (固定過去日だと可視窓0件になる)。
	// 2日前・1日前は必ず別 JST 暦日 (24時間差) ゆえ露点日次表は2行になる。全点 wet (spread 小=結露)。
	now := time.Now()
	day1 := now.AddDate(0, 0, -2)
	day2 := now.AddDate(0, 0, -1)
	repo.recentReadings = []repository.SensorReading{
		sensorRow(1, day1, 20, 98),
		sensorRow(1, day1.Add(5*time.Minute), 20, 98),
		sensorRow(1, day2, 20, 98),
		sensorRow(1, day2.Add(5*time.Minute), 20, 98),
	}
	return repo
}

// ===== 4.1 露点パネルの統合テスト（描画・期間切替） =====

func TestValidation_Chart_4期間で露点パネルが描画される(t *testing.T) {
	repo := vpdRegressionRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	for _, period := range []string{"24h", "3d", "7d", "30d"} {
		w := hxGet(r, "/devices/1/chart?period="+period)
		if w.Code != http.StatusOK {
			t.Fatalf("period=%s status=%d, want 200", period, w.Code)
		}
		body := w.Body.String()
		for _, want := range []string{
			`id="dewpoint-chart"`, `id="dewpoint-chart-option"`,
			"露点・病害リスク", "現在露点",
			"葉面湿潤時間・病害スコア", "高湿度継続イベント",
			"近似", // 近似注記の明示
		} {
			if !strings.Contains(body, want) {
				t.Errorf("period=%s のフラグメントに %q が含まれていない", period, want)
			}
		}
	}
}

func TestValidation_露点日次表が暦日数に追従(t *testing.T) {
	// 2 暦日(JST)にまたがるデータ → 露点日次表は 2 行(両日付が出る)。
	// 期待日付は本番フォーマッタ(jstDay)で行から動的計算する(now 基準フィクスチャに追従)。
	repo := dewpointMultiDayRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := hxGet(r, "/devices/1/chart?period=7d")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	wantDates := map[string]bool{}
	for _, rr := range repo.recentReadings {
		wantDates[jstDay(rr.RecordedAt).Format("2006-01-02")] = true
	}
	if len(wantDates) != 2 {
		t.Fatalf("フィクスチャが2暦日(JST)になっていない: %d 日", len(wantDates))
	}
	for date := range wantDates {
		if !strings.Contains(body, date) {
			t.Errorf("露点日次表に %q が含まれていない（暦日追従）", date)
		}
	}
}

func TestValidation_不正periodは400で露点パネルなし(t *testing.T) {
	repo := vpdRegressionRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	for _, path := range []string{"/devices/1?period=99h", "/devices/1/chart?period=bad"} {
		w := getPath(r, path)
		if w.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want 400（不正 period）", path, w.Code)
		}
		if strings.Contains(w.Body.String(), `id="dewpoint-chart"`) {
			t.Errorf("不正 period 応答に露点パネルが漏れている: %s", path)
		}
	}
}

// ===== 4.2 既存可視化の無回帰テスト =====

// expectedVPDOption は buildChartArea が VPD option を組むのと同一経路で期待 option を再計算する。
// 露点追加で VPD option が変わっていないことのバイト一致検証に使う (VPD は gap グリッドを通さない)。
func expectedVPDOption(t *testing.T, rows []repository.SensorReading, crop domain.Crop, period string) string {
	t.Helper()
	label := rawLabelFor(period)
	labels := make([]string, len(rows))
	temps := make([]float64, len(rows))
	hums := make([]float64, len(rows))
	for i, rr := range rows {
		labels[i] = label(rr.RecordedAt)
		temps[i] = pgconv.NumericToFloat(rr.Temperature)
		hums[i] = pgconv.NumericToFloat(rr.Humidity)
	}
	panel, err := buildVPDPanel(labels, temps, hums, rows, crop, period)
	if err != nil {
		t.Fatalf("期待 VPD option の構築に失敗: %v", err)
	}
	return panel.OptionJSON
}

func TestValidation_露点追加後も温湿度VPD可視化が無回帰(t *testing.T) {
	repo := vpdRegressionRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()

	// 温湿度2グラフ・統計カード・VPD パネル・期間ボタンが従来同等。
	for _, want := range []string{
		`id="temperature-chart"`, `id="humidity-chart"`,
		`id="temperature-chart-option"`, `id="humidity-chart-option"`,
		"温度サマリ", "湿度サマリ", "data-echarts",
		`id="vpd-chart"`, `id="vpd-chart-option"`, "時間帯別 VPD 逸脱", `class="vpd-bar"`,
		"24時間", "3日間", "7日間", "30日間", // 期間ボタン active 往復の素
	} {
		if !strings.Contains(body, want) {
			t.Errorf("既存可視化に %q が含まれていない（無回帰崩れ）", want)
		}
	}

	// 温湿度 option・VPD option が露点追加前とバイト一致する（露点は別 option ゆえ不変）。
	tempOpt, humOpt := expectedTempHumOptions(t, repo.recentReadings, "24h")
	if !strings.Contains(body, tempOpt) {
		t.Error("温度 option が従来と一致しない（露点追加で変化した疑い）")
	}
	if !strings.Contains(body, humOpt) {
		t.Error("湿度 option が従来と一致しない")
	}
	vpdOpt := expectedVPDOption(t, repo.recentReadings, domain.CropGoya, "24h")
	if !strings.Contains(body, vpdOpt) {
		t.Error("VPD option が従来と一致しない（露点追加で変化した疑い）")
	}
	// 温湿度 option に markArea は混入しない（露点/VPD 専用）。
	if c := strings.Count(body, "markArea"); c < 1 {
		t.Errorf("VPD/露点の markArea が消えている（描画崩れ）: count=%d", c)
	}
}

func TestValidation_空データは温湿度VPD露点とも非表示(t *testing.T) {
	repo := showDeviceRepo()
	repo.recentReadings = nil // 0 件
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	for _, ng := range []string{`id="vpd-chart"`, `id="dewpoint-chart"`, `id="temperature-chart"`} {
		if strings.Contains(body, ng) {
			t.Errorf("空データで %q が描画されている", ng)
		}
	}
	if !strings.Contains(body, "データはまだありません") {
		t.Error("空データプレースホルダが無い")
	}
	if got := strings.Count(body, `type="application/json"`); got != 0 {
		t.Errorf("空データで option script 数 = %d, want 0", got)
	}
}

// ===== 4.3 認可と研究用スコープ境界 =====

func TestValidation_非所有は露点含め404列挙防止(t *testing.T) {
	repo := vpdRegressionRepo()                                 // device1 の所有者は 7
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 999) // 別ユーザー

	for _, path := range []string{"/devices/1", "/devices/1/chart?period=24h"} {
		w := getPath(r, path)
		if w.Code != http.StatusNotFound {
			t.Errorf("非所有 GET %s = %d, want 404（列挙防止）", path, w.Code)
		}
		if strings.Contains(w.Body.String(), `id="dewpoint-chart"`) {
			t.Errorf("非所有応答に露点パネルが漏れている: %s", path)
		}
	}
}

func TestValidation_露点パネルは研究用スコープに限定(t *testing.T) {
	repo := vpdRegressionRepo()
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	// 農家向け平易表示・圃場共有 URL・病害アラート/通知・他指標(THI/絶対湿度/多地点)を描画しない。
	// 注: GDD/積算温度 は gdd-forecast spec が device-show に正式追加した兄弟パネル（App 初期化の
	// data-no-connect 制御や GDD パネル本体に現れる）ゆえ、本テストの除外対象から外す（スコープ境界の移動）。
	// 露点パネル自体が他指標へ越境しないことは依然として担保する。
	for _, ng := range []string{
		"圃場共有", "病害警告", "シェア", "信号", // 農家向け平易表示・共有
		"アラートを発火", "通知を送信", // 病害アラート/通知の発火
		"THI", "不快指数", "絶対湿度", "多地点", "圃場間", // 他指標（GDD は別 spec の正式パネルゆえ除外語から外す）
	} {
		if strings.Contains(body, ng) {
			t.Errorf("研究用スコープ外の表示 %q が混入している", ng)
		}
	}
}

// ===== 4.4 物理規約の向き（湿り側=寒色）の符号化 =====
// 注意: 自動テストは仕様前提を符号化するため向きの誤りを捕捉できない。最終確認は
// /kiro-validate-impl GO 後の実機スモーク目視（手動・別途必須）で行う（project_vpd_physics_convention）。

func TestValidation_物理規約_結露帯は寒色で湿り側(t *testing.T) {
	// wet データ (spread 小 → 結露帯発生)。結露帯 markArea が寒色（青系 66,99,235）で塗られる。
	repo := dewpointMultiDayRepo()
	h := &DeviceHandler{Repo: repo}
	area, err := h.buildChartArea(context.Background(), repo.devices[1], "7d", time.Now())
	if err != nil {
		t.Fatalf("buildChartArea() でエラー: %v", err)
	}
	opt := area.Dewpoint.OptionJSON
	if !strings.Contains(opt, "markArea") {
		t.Fatal("wet データなのに結露帯 markArea が無い")
	}
	// 湿り側=寒色（青 66,99,235）。
	if !strings.Contains(opt, "66,99,235") {
		t.Errorf("結露帯が寒色（青 66,99,235）でない＝湿り側の向き違反\noption=%s", opt)
	}
	// 暖色（VPD 乾き側の橙 230,126,34）と取り違えていない（向き反転の検出）。
	if strings.Contains(opt, "230,126,34") {
		t.Error("結露帯に暖色（乾き側の橙）が混入＝向き反転")
	}
}

func TestValidation_物理規約_結露ラベルは湿り側で立つ(t *testing.T) {
	now := time.Date(2026, 4, 20, 3, 30, 0, 0, time.UTC)

	// 湿り側（最新点 spread 小）→ 直近の結露帯=結露中。
	wet := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 20, 98),
		sensorRow(1, time.Date(2026, 4, 20, 3, 5, 0, 0, time.UTC), 20, 98),
	}
	wetPanel := buildDewpointPanelFromRows(t, wet, domain.CropGoya, "24h", now)
	if wetPanel.Card.RecentCondensation != dewCondensationPresent {
		t.Errorf("湿り側で RecentCondensation = %q, want %q（向き違反）", wetPanel.Card.RecentCondensation, dewCondensationPresent)
	}

	// 乾き側（spread 大）→ 結露帯なし。
	dry := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 32, 35),
		sensorRow(1, time.Date(2026, 4, 20, 3, 5, 0, 0, time.UTC), 32, 35),
	}
	dryPanel := buildDewpointPanelFromRows(t, dry, domain.CropGoya, "24h", now)
	if dryPanel.Card.RecentCondensation != dewCondensationAbsent {
		t.Errorf("乾き側で RecentCondensation = %q, want %q（向き違反）", dryPanel.Card.RecentCondensation, dewCondensationAbsent)
	}
}

func TestValidation_物理規約_CSSトークンとライン色は寒色(t *testing.T) {
	// 露点線・結露帯の基準色 --color-dewpoint は寒色（青 #4263eb）で、温度橙/VPD 暖色と区別する。
	// dewpointLineColor は handler 定数、--color-dewpoint はモック正本由来の本番配信 CSS。
	if dewpointLineColor != "#4263eb" {
		t.Errorf("dewpointLineColor = %q, want #4263eb（寒色＝湿り側）", dewpointLineColor)
	}
	// 暖色（温度橙）と一致しないこと（向き反転の検出）。
	if dewpointLineColor == tempLineColor {
		t.Error("露点線色が温度（暖色）と同一＝湿り側の向き違反")
	}
}
