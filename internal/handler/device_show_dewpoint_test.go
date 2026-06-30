package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/chart"
	"github.com/HiroshiKawano/go_iot/internal/domain"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/view/component"
)

// dewpointPanelLabels は len(rows) と同長のダミーラベル列を作る（buildDewpointPanel は labels をそのまま option へ渡す）。
func dewpointPanelLabels(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "lbl"
	}
	return out
}

// buildDewpointPanelFromRows は temps/hums を rows から取り出して buildDewpointPanel を呼ぶテストヘルパ。
func buildDewpointPanelFromRows(t *testing.T, rows []repository.SensorReading, crop domain.Crop, period string, now time.Time) component.DewpointPanelView {
	t.Helper()
	temps := make([]float64, len(rows))
	hums := make([]float64, len(rows))
	for i, r := range rows {
		temps[i] = pgconv.NumericToFloat(r.Temperature)
		hums[i] = pgconv.NumericToFloat(r.Humidity)
	}
	panel, err := buildDewpointPanel(dewpointPanelLabels(len(rows)), temps, hums, rows, crop, period, now)
	if err != nil {
		t.Fatalf("buildDewpointPanel() でエラー: %v", err)
	}
	return panel
}

// --- 2.2 露点カード（現在露点・現在スプレッド・直近の結露帯）と option/近似注記 ---

func TestBuildDewpointPanel_露点カードの現在値(t *testing.T) {
	// 25/50→Td≈13.86・spread≈11.14、最後の点 20/95→Td≈19.06・spread≈0.94（結露帯しきい値2.0以下=結露中）。
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 25, 50),
		sensorRow(1, time.Date(2026, 4, 20, 3, 5, 0, 0, time.UTC), 20, 95),
	}
	now := time.Date(2026, 4, 20, 3, 10, 0, 0, time.UTC)
	panel := buildDewpointPanelFromRows(t, rows, domain.CropGoya, "24h", now)

	// 現在露点=最新点 Td（20/95→約19.0℃）。
	wantTd := chart.DewPoint(20, 95)
	if panel.Card.CurrentDewpoint != formatDewpointStat(wantTd) {
		t.Errorf("CurrentDewpoint = %q, want %q", panel.Card.CurrentDewpoint, formatDewpointStat(wantTd))
	}
	// 現在スプレッド=最新点 T−Td（約0.9℃）。
	wantSpread := 20 - wantTd
	if panel.Card.CurrentSpread != formatDewpointStat(wantSpread) {
		t.Errorf("CurrentSpread = %q, want %q", panel.Card.CurrentSpread, formatDewpointStat(wantSpread))
	}
	// 最新点 spread≈0.94 ≤ 2.0（ゴーヤ既定）ゆえ直近の結露帯=結露中。
	if panel.Card.RecentCondensation != dewCondensationPresent {
		t.Errorf("RecentCondensation = %q, want %q（最新点が結露帯内）", panel.Card.RecentCondensation, dewCondensationPresent)
	}
}

func TestBuildDewpointPanel_直近結露帯なし(t *testing.T) {
	// 全点で乾燥（spread 大）→ 結露帯なし。
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 30, 40),
		sensorRow(1, time.Date(2026, 4, 20, 3, 5, 0, 0, time.UTC), 30, 45),
	}
	now := time.Date(2026, 4, 20, 3, 10, 0, 0, time.UTC)
	panel := buildDewpointPanelFromRows(t, rows, domain.CropGoya, "24h", now)
	if panel.Card.RecentCondensation != dewCondensationAbsent {
		t.Errorf("RecentCondensation = %q, want %q（結露帯なし）", panel.Card.RecentCondensation, dewCondensationAbsent)
	}
}

func TestBuildDewpointPanel_optionと近似注記(t *testing.T) {
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 25, 50),
		sensorRow(1, time.Date(2026, 4, 20, 3, 5, 0, 0, time.UTC), 20, 95),
	}
	now := time.Date(2026, 4, 20, 3, 10, 0, 0, time.UTC)
	panel := buildDewpointPanelFromRows(t, rows, domain.CropGoya, "24h", now)

	if panel.OptionJSON == "" {
		t.Errorf("OptionJSON が空（露点 option が必要）")
	}
	if strings.Contains(panel.OptionJSON, "</script>") {
		t.Errorf("OptionJSON に </script> が混入: %s", panel.OptionJSON)
	}
	if panel.DewColor == "" {
		t.Errorf("DewColor が空（--color-dewpoint 相当の寒色基準色が必要）")
	}
	// 近似注記（葉面温度を気温で近似した代理判定である旨）を View へ渡す（要件 2.3）。
	if !strings.Contains(panel.Note, "近似") {
		t.Errorf("Note に近似注記が無い: %q", panel.Note)
	}
}

// --- 2.3 葉面湿潤時間の日次積算（日跨ぎ・DiseaseScore 列・maxWetGap キャップ） ---

func TestBuildDewpointPanel_葉面湿潤日次積算と日跨ぎ(t *testing.T) {
	// JST 暦日 = UTC+9。JST 2026-04-20 10:00 = UTC 01:00、JST 2026-04-21 09:00 = UTC 04-21 00:00。
	// Day1(04-20): 10:00 wet, 10:05 wet, 10:10 dry → wet 区間は1つ(300秒=0.1時間)。temps=20(帯内)。
	// Day2(04-21): 09:00 dry, 09:05 wet, 09:10 wet → wet 区間は1つ(300秒=0.1時間)。temps=20(帯内)。
	// 日跨ぎの dry→dry 間隔は計上しない（正しい暦日へ計上）。
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 1, 0, 0, 0, time.UTC), 20, 95),  // 04-20 10:00 JST wet
		sensorRow(1, time.Date(2026, 4, 20, 1, 5, 0, 0, time.UTC), 20, 95),  // 04-20 10:05 JST wet
		sensorRow(1, time.Date(2026, 4, 20, 1, 10, 0, 0, time.UTC), 20, 50), // 04-20 10:10 JST dry
		sensorRow(1, time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC), 20, 50),  // 04-21 09:00 JST dry
		sensorRow(1, time.Date(2026, 4, 21, 0, 5, 0, 0, time.UTC), 20, 95),  // 04-21 09:05 JST wet
		sensorRow(1, time.Date(2026, 4, 21, 0, 10, 0, 0, time.UTC), 20, 95), // 04-21 09:10 JST wet
	}
	now := time.Date(2026, 4, 21, 1, 0, 0, 0, time.UTC) // 04-21 10:00 JST
	panel := buildDewpointPanelFromRows(t, rows, domain.CropGoya, "3d", now)

	if len(panel.Daily) != 2 {
		t.Fatalf("Daily 行数 = %d, want 2（04-20/04-21）: %+v", len(panel.Daily), panel.Daily)
	}
	d20, d21 := panel.Daily[0], panel.Daily[1]
	if d20.Date != "2026-04-20" || d20.WetHours != "0.1 時間" {
		t.Errorf("Day1 = %q/%q, want 2026-04-20/0.1 時間", d20.Date, d20.WetHours)
	}
	if d21.Date != "2026-04-21" || d21.WetHours != "0.1 時間" {
		t.Errorf("Day2 = %q/%q, want 2026-04-21/0.1 時間", d21.Date, d21.WetHours)
	}
	// DiseaseScore 列: 各日 wet 2点/全3点 かつ temp=20(帯内) → 2/3=67%。
	if d20.DiseaseScore != "67%" {
		t.Errorf("Day1 DiseaseScore = %q, want 67%%", d20.DiseaseScore)
	}
	// 負の時間を生じない（"-" 始まりでない）。
	for _, r := range panel.Daily {
		if strings.HasPrefix(r.WetHours, "-") {
			t.Errorf("WetHours が負: %q", r.WetHours)
		}
	}
	// 本日(04-21)の葉面湿潤時間がカードに載る。
	if panel.Card.TodayWetHours != "0.1 時間" {
		t.Errorf("TodayWetHours = %q, want 0.1 時間（本日04-21）", panel.Card.TodayWetHours)
	}
}

func TestBuildDewpointPanel_maxWetGapキャップ(t *testing.T) {
	// 同一日内に wet→wet が3時間空き（10:00→13:00 JST）。両端 wet ゆえ間隔を計上するが maxWetGap(1時間)で
	// キャップされ 1.0 時間に留まる（欠測の水増し防止）。キャップなしなら 3.0 時間。
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 1, 0, 0, 0, time.UTC), 20, 95), // 10:00 JST wet
		sensorRow(1, time.Date(2026, 4, 20, 4, 0, 0, 0, time.UTC), 20, 95), // 13:00 JST wet
	}
	now := time.Date(2026, 4, 20, 5, 0, 0, 0, time.UTC)
	panel := buildDewpointPanelFromRows(t, rows, domain.CropGoya, "24h", now)
	if len(panel.Daily) != 1 {
		t.Fatalf("Daily 行数 = %d, want 1: %+v", len(panel.Daily), panel.Daily)
	}
	if panel.Daily[0].WetHours != "1.0 時間" {
		t.Errorf("WetHours = %q, want 1.0 時間（maxWetGap=1時間でキャップ）", panel.Daily[0].WetHours)
	}
}

func TestBuildDewpointPanel_単一点で破綻しない(t *testing.T) {
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 1, 0, 0, 0, time.UTC), 20, 95),
	}
	now := time.Date(2026, 4, 20, 2, 0, 0, 0, time.UTC)
	panel := buildDewpointPanelFromRows(t, rows, domain.CropGoya, "24h", now)
	if len(panel.Daily) != 1 {
		t.Fatalf("Daily 行数 = %d, want 1", len(panel.Daily))
	}
	// 単一点は wet 区間を作れない → 0.0 時間（破綻なし・負でない）。
	if panel.Daily[0].WetHours != "0.0 時間" {
		t.Errorf("単一点 WetHours = %q, want 0.0 時間", panel.Daily[0].WetHours)
	}
}

// --- 2.5 病害スコア下地の統合（施設果菜=具体値・未設定でも既定モデルで非空） ---

// hasConcreteScore は日次行のいずれかに具体的な病害スコア（"NN%"・"—" でない）があるか判定する。
func hasConcreteScore(daily []component.DewpointDailyRow) bool {
	for _, r := range daily {
		if r.DiseaseScore != "" && r.DiseaseScore != statEmptyMark && strings.HasSuffix(r.DiseaseScore, "%") {
			return true
		}
	}
	return false
}

func TestBuildDewpointPanel_病害スコア施設果菜で具体値(t *testing.T) {
	// ゴーヤ(15-25℃/RH90)。temp=20(帯内)・RH=95(湿潤) → 病害スコア下地が具体値(100%)。
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 1, 0, 0, 0, time.UTC), 20, 95),
		sensorRow(1, time.Date(2026, 4, 20, 1, 5, 0, 0, time.UTC), 20, 95),
		sensorRow(1, time.Date(2026, 4, 20, 1, 10, 0, 0, time.UTC), 20, 95),
	}
	now := time.Date(2026, 4, 20, 2, 0, 0, 0, time.UTC)
	panel := buildDewpointPanelFromRows(t, rows, domain.CropGoya, "24h", now)
	if !hasConcreteScore(panel.Daily) {
		t.Errorf("施設果菜で病害スコアが具体値にならない: %+v", panel.Daily)
	}
	// 全点 帯内×湿潤 → 100%。
	if panel.Daily[0].DiseaseScore != "100%" {
		t.Errorf("DiseaseScore = %q, want 100%%", panel.Daily[0].DiseaseScore)
	}
}

func TestBuildDewpointPanel_病害スコア未設定でも既定モデルで非空(t *testing.T) {
	// 作物未設定(Crop(""))→DefaultDiseaseModel(15-25℃/RH90)。同データで非空・具体値（要件 5.2/5.4）。
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 1, 0, 0, 0, time.UTC), 20, 95),
		sensorRow(1, time.Date(2026, 4, 20, 1, 5, 0, 0, time.UTC), 20, 95),
	}
	now := time.Date(2026, 4, 20, 2, 0, 0, 0, time.UTC)
	panel := buildDewpointPanelFromRows(t, rows, domain.Crop(""), "24h", now)
	if !hasConcreteScore(panel.Daily) {
		t.Errorf("作物未設定でも病害スコアが具体値にならない（既定モデルで非空のはず）: %+v", panel.Daily)
	}
}

// 病害スコアは下地（最小合成・温度帯外/非湿潤は寄与なし）に限定され、確定予察を含まない。
// 温度帯外（temp=35）かつ湿潤でも 0%（下地ゆえ発病記録突合や予察モデルの加点をしない）。
func TestBuildDewpointPanel_病害スコアは下地に限定(t *testing.T) {
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 1, 0, 0, 0, time.UTC), 35, 95), // 帯外(35>25)・湿潤
		sensorRow(1, time.Date(2026, 4, 20, 1, 5, 0, 0, time.UTC), 35, 95),
	}
	now := time.Date(2026, 4, 20, 2, 0, 0, 0, time.UTC)
	panel := buildDewpointPanelFromRows(t, rows, domain.CropGoya, "24h", now)
	if panel.Daily[0].DiseaseScore != "0%" {
		t.Errorf("温度帯外の病害スコア = %q, want 0%%（下地限定・寄与なし）", panel.Daily[0].DiseaseScore)
	}
}

// --- 2.4 高湿度継続イベント一覧（最小継続以上のみ・単発除外・時刻整形） ---

func TestMinRunFromHours(t *testing.T) {
	cases := []struct {
		hours float64
		want  int
	}{
		{1.0, 12}, // 約5分間隔=12点/時
		{0.5, 6},
		{2.0, 24},
		{0.0, 1},  // 0 以下は 1 に正規化
		{0.01, 1}, // round(0.12)=0 → 1 にクランプ
	}
	for _, c := range cases {
		if got := minRunFromHours(c.hours); got != c.want {
			t.Errorf("minRunFromHours(%v) = %d, want %d", c.hours, got, c.want)
		}
	}
}

func TestBuildDewpointPanel_高湿度イベント抽出と単発除外(t *testing.T) {
	// ゴーヤ HighHumidityMinHours=1.0 → minRun=12点。
	// 13点連続高湿度(≥12=抽出) → dry1点 → 5点高湿度(<12=単発除外)。約5分間隔。
	base := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC) // JST 09:00
	var rows []repository.SensorReading
	for i := 0; i < 13; i++ {
		rows = append(rows, sensorRow(1, base.Add(time.Duration(i)*5*time.Minute), 20, 95)) // wet
	}
	rows = append(rows, sensorRow(1, base.Add(13*5*time.Minute), 20, 50)) // dry で区切る
	for i := 14; i < 19; i++ {
		rows = append(rows, sensorRow(1, base.Add(time.Duration(i)*5*time.Minute), 20, 95)) // 5点(<12)
	}
	now := base.Add(2 * time.Hour)
	panel := buildDewpointPanelFromRows(t, rows, domain.CropGoya, "24h", now)

	if len(panel.Events) != 1 {
		t.Fatalf("Events 行数 = %d, want 1（13点連続のみ・5点単発は除外）: %+v", len(panel.Events), panel.Events)
	}
	e := panel.Events[0]
	// index0..12（13点）= 12間隔 = 60分 = 1.0時間。
	if e.Duration != "1.0 時間" {
		t.Errorf("Duration = %q, want 1.0 時間", e.Duration)
	}
	// 開始/終了が時刻整形される（JST 09:00 → 10:00）。
	if e.Start != "04/20 09:00" {
		t.Errorf("Start = %q, want 04/20 09:00", e.Start)
	}
	if e.End != "04/20 10:00" {
		t.Errorf("End = %q, want 04/20 10:00", e.End)
	}
	// 区間内最小スプレッド（20/95→spread≈0.9）が℃整形される。
	if !strings.Contains(e.MinSpread, "℃") {
		t.Errorf("MinSpread = %q, want ℃ 付き", e.MinSpread)
	}
}

func TestBuildDewpointPanel_高湿度イベント該当なしで空(t *testing.T) {
	rows := []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), 30, 40),
		sensorRow(1, time.Date(2026, 4, 20, 0, 5, 0, 0, time.UTC), 30, 45),
	}
	now := time.Date(2026, 4, 20, 1, 0, 0, 0, time.UTC)
	panel := buildDewpointPanelFromRows(t, rows, domain.CropGoya, "24h", now)
	if len(panel.Events) != 0 {
		t.Errorf("高湿度なしで Events 非空: %+v", panel.Events)
	}
}

func TestBuildDewpointPanel_空行で空パネル(t *testing.T) {
	now := time.Date(2026, 4, 20, 3, 10, 0, 0, time.UTC)
	panel, err := buildDewpointPanel(nil, nil, nil, nil, domain.CropGoya, "24h", now)
	if err != nil {
		t.Fatalf("buildDewpointPanel() でエラー: %v", err)
	}
	if panel.Card.CurrentDewpoint != statEmptyMark {
		t.Errorf("空行で CurrentDewpoint = %q, want —", panel.Card.CurrentDewpoint)
	}
	if panel.OptionJSON != "" {
		t.Errorf("空行で OptionJSON は空のはず: %q", panel.OptionJSON)
	}
	if len(panel.Daily) != 0 || len(panel.Events) != 0 {
		t.Errorf("空行で Daily/Events 非空: Daily=%+v Events=%+v", panel.Daily, panel.Events)
	}
}

// --- 3.3 buildChartArea への露点パネル統合（device 受け渡し・無回帰） ---

func TestBuildChartArea_露点パネルを内包(t *testing.T) {
	cropStr := "goya"
	repo := vpdShowRepoWithCrop(&cropStr)
	h := &DeviceHandler{Repo: repo}

	area, err := h.buildChartArea(context.Background(), repo.devices[1], "24h", time.Now())
	if err != nil {
		t.Fatalf("buildChartArea() でエラー: %v", err)
	}
	if !area.HasData {
		t.Fatal("HasData=true 想定（生データ有り）")
	}
	// 露点パネルが組まれている（option・寒色・近似注記）。
	if area.Dewpoint.OptionJSON == "" {
		t.Errorf("Dewpoint.OptionJSON が空（露点パネルが組まれていない）")
	}
	if area.Dewpoint.DewColor != dewpointLineColor {
		t.Errorf("Dewpoint.DewColor = %q, want %q（寒色）", area.Dewpoint.DewColor, dewpointLineColor)
	}
	if !strings.Contains(area.Dewpoint.Note, "近似") {
		t.Errorf("Dewpoint.Note に近似注記が無い: %q", area.Dewpoint.Note)
	}
	// 無回帰: 温湿度 option に markArea は混入しない（露点は別 option）。VPD パネルは従来どおり組まれる。
	if strings.Contains(area.TemperatureOptionJSON, "markArea") || strings.Contains(area.HumidityOptionJSON, "markArea") {
		t.Error("温湿度 option に markArea が混入（露点専用のはず・無回帰崩れ）")
	}
	if area.VPD.OptionJSON == "" {
		t.Error("VPD パネルが組まれていない（無回帰崩れ）")
	}
}

func TestBuildChartArea_空データは露点パネルを組まない(t *testing.T) {
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
	if area.Dewpoint.OptionJSON != "" {
		t.Errorf("空データで露点パネルが組まれている: %q", area.Dewpoint.OptionJSON)
	}
}

func TestShow_露点パネルを描画する(t *testing.T) {
	cropStr := "goya"
	repo := vpdShowRepoWithCrop(&cropStr)
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		`id="dewpoint-chart"`, `id="dewpoint-chart-option"`,
		"露点・病害リスク", "現在露点",
		"葉面湿潤時間・病害スコア", "高湿度継続イベント",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("詳細ページに %q が含まれていない（露点パネル）", want)
		}
	}
	// option script は温/湿/VPD/露点 + 高温ストレス(THI/熱帯夜calendar/夜温ΔT/AH) の 8 本
	// (heat-stress-thi パネル追加・温湿度/VPD/露点 option は不変)。
	if got := strings.Count(body, `type="application/json"`); got != 8 {
		t.Errorf("option script 数 = %d, want 8（温度/湿度/VPD/露点/THI/熱帯夜/夜温ΔT/AH）", got)
	}
}
