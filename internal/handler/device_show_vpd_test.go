package handler

import (
	"bytes"
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/domain"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/view/component"
)

// --- 4.4 DeviceChartArea templ への VPD パネル描画 ---

// vpdSampleAreaView は VPD パネルを内包した DeviceChartAreaView (描画テスト用の代表値)。
func vpdSampleAreaView() component.DeviceChartAreaView {
	return component.DeviceChartAreaView{
		DeviceID: 1, Period: "24h", HasData: true,
		TemperatureOptionJSON: "{}", HumidityOptionJSON: "{}",
		TemperatureUnit: "℃", HumidityUnit: "%",
		TemperatureColor: tempLineColor, HumidityColor: humidityLineColor,
		TemperatureCard: component.StatCardView{Current: "28.50℃", Max: "35.20℃", Min: "18.50℃", Diurnal: "16.70℃"},
		HumidityCard:    component.StatCardView{Current: "65.30%", Max: "85.00%", Min: "30.20%", Diurnal: "54.80%"},
		VPD: component.VPDPanelView{
			OptionJSON: `{"series":[{"markArea":{}}]}`,
			Color:      vpdLineColor,
			CropLabel:  "ゴーヤ",
			LowerLabel: "0.40 kPa",
			UpperLabel: "1.20 kPa",
			Card: component.VPDCardView{
				CurrentVPD: "0.90 kPa", AverageVPD: "1.05 kPa",
				TimeInRange: "72%", MaxDeviation: "+0.40 kPa（乾き）",
			},
			InRangeRatio: 0.72,
			Hourly: []component.VPDHourlyRow{
				{Hour: "06:00", AvgVPD: "0.35", InRangePercent: "40%", Direction: "湿り"},
				{Hour: "12:00", AvgVPD: "1.45", InRangePercent: "55%", Direction: "乾き"},
			},
		},
	}
}

func renderChartArea(t *testing.T, v component.DeviceChartAreaView) string {
	t.Helper()
	var buf bytes.Buffer
	if err := component.DeviceChartArea(v).Render(context.Background(), &buf); err != nil {
		t.Fatalf("DeviceChartArea.Render() でエラー: %v", err)
	}
	return buf.String()
}

func TestDeviceChartArea_VPDパネルを描画する(t *testing.T) {
	html := renderChartArea(t, vpdSampleAreaView())
	for _, want := range []string{
		`id="vpd-chart"`,           // VPD チャートマウント先
		`data-unit="kPa"`,          // kPa 単位指定
		`data-color="#0ca678"`,     // VPD 基準色
		`id="vpd-chart-option"`,    // VPD option script
		"ゴーヤ",                      // 作物名
		"0.40 kPa", "1.20 kPa",     // 適正帯上下限
		"現在VPD", "適正帯滞在率", "最大逸脱", // VPD 数値カード
		"0.90 kPa", "72%",          // カード値
		`class="vpd-bar"`,          // 滞在率バー枠
		"vpd-bar-fill",             // 滞在率バー塗り
		"width:72%",                // 滞在率バーの動的幅 (InRangeRatio 0.72)
		"時間帯別 VPD 逸脱",            // 逸脱表見出し
		"06:00", "湿り", "12:00", "乾き", // 逸脱行 (低VPD=湿りすぎ / 高VPD=乾きすぎ)
	} {
		if !strings.Contains(html, want) {
			t.Errorf("VPD パネル描画に %q が含まれていない", want)
		}
	}
	// option script は温/湿/VPD の 3 本。
	if got := strings.Count(html, `type="application/json"`); got != 3 {
		t.Errorf("option script 数 = %d, want 3 (温度/湿度/VPD)", got)
	}
}

// 研究用スコープ: 農家向け平易表示 (信号色での良否ラベル・圃場共有 URL) を含めない (R9.3)。
func TestDeviceChartArea_農家向け表示を含めない(t *testing.T) {
	html := renderChartArea(t, vpdSampleAreaView())
	for _, ng := range []string{"圃場共有", "良好", "要注意", "シェア"} {
		if strings.Contains(html, ng) {
			t.Errorf("研究用スコープ外の農家向け表示 %q が混入している", ng)
		}
	}
}

// 空データ (HasData=false) では VPD パネルを描画しない (R4.5)。
func TestDeviceChartArea_空データでVPDパネル非表示(t *testing.T) {
	v := component.DeviceChartAreaView{
		DeviceID: 1, Period: "24h", HasData: false,
		TemperatureCard: component.StatCardView{Current: "—"},
		HumidityCard:    component.StatCardView{Current: "—"},
	}
	html := renderChartArea(t, v)
	if strings.Contains(html, `id="vpd-chart"`) {
		t.Errorf("空データで VPD パネルが描画されている")
	}
	if !strings.Contains(html, "データはまだありません") {
		t.Errorf("空データプレースホルダが無い")
	}
}

// --- 4.3 buildChartArea への VPD パネル統合 (device 受け渡し・無回帰) ---

// vpdShowRepoWithCrop は所有者(7)・デバイス1(crop 指定)・2点の生データを備えた fake を返す。
func vpdShowRepoWithCrop(crop *string) *fakeDeviceRepo {
	repo := showDeviceRepo()
	d := repo.devices[1]
	d.Crop = crop
	repo.devices[1] = d
	repo.recentReadings = []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 25, 50),
		sensorRow(1, time.Date(2026, 4, 20, 3, 5, 0, 0, time.UTC), 20, 70),
	}
	return repo
}

func TestBuildChartArea_VPDパネルを内包し作物名を適用(t *testing.T) {
	cropStr := "goya"
	repo := vpdShowRepoWithCrop(&cropStr)
	h := &DeviceHandler{Repo: repo}

	area, err := h.buildChartArea(context.Background(), repo.devices[1], "24h", time.Now())
	if err != nil {
		t.Fatalf("buildChartArea() でエラー: %v", err)
	}
	if !area.HasData {
		t.Fatal("HasData=true 想定 (生データ有り)")
	}
	// VPD パネルが内包され、作物名と option(markArea)・カードが組まれている。
	if area.VPD.OptionJSON == "" || !strings.Contains(area.VPD.OptionJSON, "markArea") {
		t.Errorf("VPD.OptionJSON に markArea が無い: %q", area.VPD.OptionJSON)
	}
	if area.VPD.CropLabel != "ゴーヤ" {
		t.Errorf("VPD.CropLabel = %q, want ゴーヤ", area.VPD.CropLabel)
	}
	if area.VPD.LowerLabel != "0.40 kPa" || area.VPD.UpperLabel != "1.20 kPa" {
		t.Errorf("VPD 適正帯 = %q〜%q, want 0.40〜1.20 kPa", area.VPD.LowerLabel, area.VPD.UpperLabel)
	}
	if area.VPD.Card.CurrentVPD == "" || area.VPD.Card.CurrentVPD == statEmptyMark {
		t.Errorf("VPD.Card.CurrentVPD が未設定: %q", area.VPD.Card.CurrentVPD)
	}
	// 無回帰: 温湿度 option は従来の line option (markArea を持たない・基準色を保持)。
	if !strings.Contains(area.TemperatureOptionJSON, tempLineColor) {
		t.Errorf("温度 option に基準色 %s が無い (無回帰崩れ)", tempLineColor)
	}
	if strings.Contains(area.TemperatureOptionJSON, "markArea") || strings.Contains(area.HumidityOptionJSON, "markArea") {
		t.Error("温湿度 option に markArea が混入している (VPD 専用のはず・無回帰崩れ)")
	}
}

func TestBuildChartArea_作物未設定は既定帯(t *testing.T) {
	repo := vpdShowRepoWithCrop(nil) // crop 未設定 (NULL)
	h := &DeviceHandler{Repo: repo}

	area, err := h.buildChartArea(context.Background(), repo.devices[1], "24h", time.Now())
	if err != nil {
		t.Fatalf("buildChartArea() でエラー: %v", err)
	}
	if area.VPD.CropLabel != "既定" {
		t.Errorf("VPD.CropLabel = %q, want 既定", area.VPD.CropLabel)
	}
	if area.VPD.LowerLabel != "0.30 kPa" || area.VPD.UpperLabel != "1.50 kPa" {
		t.Errorf("既定適正帯 = %q〜%q, want 0.30〜1.50 kPa", area.VPD.LowerLabel, area.VPD.UpperLabel)
	}
}

func TestBuildChartArea_空データはVPDパネルを組まない(t *testing.T) {
	repo := showDeviceRepo()
	repo.recentReadings = nil // 0 件
	h := &DeviceHandler{Repo: repo}

	area, err := h.buildChartArea(context.Background(), repo.devices[1], "24h", time.Now())
	if err != nil {
		t.Fatalf("buildChartArea() でエラー: %v", err)
	}
	if area.HasData {
		t.Fatal("空データで HasData=false 想定")
	}
	if area.VPD.OptionJSON != "" {
		t.Errorf("空データで VPD.OptionJSON は空のはず: %q", area.VPD.OptionJSON)
	}
}

// vpdPanelLabels は len(rows) と同長のダミーラベル列を作る（buildVPDPanel は labels をそのまま option へ渡す）。
func vpdPanelLabels(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "lbl"
	}
	return out
}

// buildVPDPanelFromRows は temps/hums を rows から取り出して buildVPDPanel を呼ぶテストヘルパ。
func buildVPDPanelFromRows(t *testing.T, rows []repository.SensorReading, crop domain.Crop, period string) component.VPDPanelView {
	t.Helper()
	temps := make([]float64, len(rows))
	hums := make([]float64, len(rows))
	for i, r := range rows {
		temps[i] = pgconv.NumericToFloat(r.Temperature)
		hums[i] = pgconv.NumericToFloat(r.Humidity)
	}
	panel, err := buildVPDPanel(vpdPanelLabels(len(rows)), temps, hums, rows, crop, period)
	if err != nil {
		t.Fatalf("buildVPDPanel() でエラー: %v", err)
	}
	return panel
}

// --- 4.2 滞在率・最大逸脱・カード ---

func TestBuildVPDPanel_カードに現在_平均_滞在率_最大逸脱(t *testing.T) {
	// 25/50→1.58kPa(高VPD=乾きすぎ超過), 10/100→0kPa(低VPD=湿りすぎ逸脱), 20/70→0.70kPa(適正帯内)。
	// 作物=ゴーヤ(0.4-1.2)。現在=最後(0.70)、滞在率=1/3、最大逸脱=湿り0.40(下限割れが最大)。
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 25, 50),
		sensorRow(1, time.Date(2026, 4, 20, 3, 5, 0, 0, time.UTC), 10, 100),
		sensorRow(1, time.Date(2026, 4, 20, 3, 10, 0, 0, time.UTC), 20, 70),
	}
	panel := buildVPDPanelFromRows(t, rows, domain.CropGoya, "24h")

	if panel.Card.CurrentVPD != "0.70 kPa" {
		t.Errorf("CurrentVPD = %q, want 0.70 kPa", panel.Card.CurrentVPD)
	}
	if panel.Card.AverageVPD != "0.76 kPa" {
		t.Errorf("AverageVPD = %q, want 0.76 kPa", panel.Card.AverageVPD)
	}
	if panel.Card.TimeInRange != "33%" {
		t.Errorf("TimeInRange = %q, want 33%%", panel.Card.TimeInRange)
	}
	// 最大逸脱は湿り 0.40kPa（B 点の下限割れ=低VPD=湿りすぎ が最大）。
	if !strings.Contains(panel.Card.MaxDeviation, "0.40") || !strings.Contains(panel.Card.MaxDeviation, "湿り") {
		t.Errorf("MaxDeviation = %q, want 0.40 と 湿り を含む", panel.Card.MaxDeviation)
	}
	// 滞在率バー比率は 1/3。
	if math.Abs(panel.InRangeRatio-1.0/3.0) > 1e-9 {
		t.Errorf("InRangeRatio = %v, want 1/3", panel.InRangeRatio)
	}
}

// --- 4.2 作物別しきい値の適用 ---

func TestBuildVPDPanel_作物別しきい値が反映される(t *testing.T) {
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 20, 70),
	}
	t.Run("ゴーヤは0.4-1.2と作物名", func(t *testing.T) {
		panel := buildVPDPanelFromRows(t, rows, domain.CropGoya, "24h")
		if panel.CropLabel != "ゴーヤ" {
			t.Errorf("CropLabel = %q, want ゴーヤ", panel.CropLabel)
		}
		if panel.LowerLabel != "0.40 kPa" || panel.UpperLabel != "1.20 kPa" {
			t.Errorf("適正帯 = %q〜%q, want 0.40 kPa〜1.20 kPa", panel.LowerLabel, panel.UpperLabel)
		}
	})
	t.Run("未設定は既定0.3-1.5と既定ラベル", func(t *testing.T) {
		panel := buildVPDPanelFromRows(t, rows, domain.Crop(""), "24h")
		if panel.CropLabel != "既定" {
			t.Errorf("CropLabel = %q, want 既定", panel.CropLabel)
		}
		if panel.LowerLabel != "0.30 kPa" || panel.UpperLabel != "1.50 kPa" {
			t.Errorf("適正帯 = %q〜%q, want 0.30 kPa〜1.50 kPa", panel.LowerLabel, panel.UpperLabel)
		}
	})
}

// --- 4.2 時間帯別逸脱バケット（在帯率・逸脱方向） ---

func TestBuildVPDPanel_時間帯別バケットと逸脱方向(t *testing.T) {
	// JST 12時に2点(高VPD=乾きすぎ超過 25/50 と適正 20/70)、JST 6時に1点(低VPD=湿りすぎ 10/100)。
	// 03:00/03:05 UTC = 12:00 JST、21:00 UTC = 翌 06:00 JST。
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 25, 50),   // 12時・高VPD=乾き
		sensorRow(1, time.Date(2026, 4, 20, 3, 5, 0, 0, time.UTC), 20, 70),   // 12時・適正
		sensorRow(1, time.Date(2026, 4, 20, 21, 0, 0, 0, time.UTC), 10, 100), // 翌6時・低VPD=湿り
	}
	panel := buildVPDPanelFromRows(t, rows, domain.CropGoya, "24h")

	if len(panel.Hourly) != 2 {
		t.Fatalf("Hourly 行数 = %d, want 2（6時/12時）: %+v", len(panel.Hourly), panel.Hourly)
	}
	// 昇順（6時 → 12時）。
	h6, h12 := panel.Hourly[0], panel.Hourly[1]
	if h6.Hour != "06:00" {
		t.Errorf("Hourly[0].Hour = %q, want 06:00", h6.Hour)
	}
	// 6時は VPD=0（下限0.4未満=低VPD）→ 湿りすぎ。
	if h6.InRangePercent != "0%" || h6.Direction != "湿り" {
		t.Errorf("6時 = 在帯%q/逸脱%q, want 0%%/湿り", h6.InRangePercent, h6.Direction)
	}
	if h12.Hour != "12:00" {
		t.Errorf("Hourly[1].Hour = %q, want 12:00", h12.Hour)
	}
	// 12時は2点中1点在帯=50%、逸脱方向は乾き(1.58=高VPD で上限超)が多数。
	if h12.InRangePercent != "50%" || h12.Direction != "乾き" {
		t.Errorf("12時 = 在帯%q/逸脱%q, want 50%%/乾き", h12.InRangePercent, h12.Direction)
	}
}

// 全点が適正帯内のバケットは逸脱方向「—」。
func TestBuildVPDPanel_無逸脱バケットはダッシュ(t *testing.T) {
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 20, 70), // 0.70kPa 適正
	}
	panel := buildVPDPanelFromRows(t, rows, domain.CropGoya, "24h")
	if len(panel.Hourly) != 1 {
		t.Fatalf("Hourly 行数 = %d, want 1", len(panel.Hourly))
	}
	if panel.Hourly[0].Direction != "—" {
		t.Errorf("無逸脱の Direction = %q, want —", panel.Hourly[0].Direction)
	}
	if panel.Hourly[0].InRangePercent != "100%" {
		t.Errorf("全在帯の InRangePercent = %q, want 100%%", panel.Hourly[0].InRangePercent)
	}
}

// --- 4.2 空行で空パネル ---

func TestBuildVPDPanel_空行で空パネル(t *testing.T) {
	panel, err := buildVPDPanel(nil, nil, nil, nil, domain.CropGoya, "24h")
	if err != nil {
		t.Fatalf("buildVPDPanel() でエラー: %v", err)
	}
	if len(panel.Hourly) != 0 {
		t.Errorf("空行で Hourly 非空: %+v", panel.Hourly)
	}
	if panel.Card.CurrentVPD != "—" {
		t.Errorf("空行で CurrentVPD = %q, want —", panel.Card.CurrentVPD)
	}
}

// --- 4.2 VPD option が markArea を内包する ---

func TestBuildVPDPanel_optionにmarkAreaと作物しきい値(t *testing.T) {
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 25, 50),
		sensorRow(1, time.Date(2026, 4, 20, 3, 5, 0, 0, time.UTC), 20, 70),
	}
	panel := buildVPDPanelFromRows(t, rows, domain.CropGoya, "24h")
	if !strings.Contains(panel.OptionJSON, "markArea") {
		t.Errorf("OptionJSON に markArea が無い: %s", panel.OptionJSON)
	}
	if strings.Contains(panel.OptionJSON, "</script>") {
		t.Errorf("OptionJSON に </script> が混入: %s", panel.OptionJSON)
	}
	if panel.Color == "" {
		t.Errorf("Color が空（--color-vpd 相当の基準色が必要）")
	}
}
