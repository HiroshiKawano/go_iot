package handler

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// fixedNow はルックバック検証の決定的基準時刻（buildChartArea へ直接注入）。
var fixedNow = time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)

// --- 3.1 純粋ヘルパ: maxWindowDays / visibleStartIndex / daySMASeriesFor -----

func TestMaxWindowDays(t *testing.T) {
	tests := []struct {
		name    string
		windows []dayWindow
		want    int
	}{
		{"空はルックバックなし", nil, 0},
		{"7d は最長7日", dayScaleWindowsFor("7d"), 7},
		{"30d は最長14日", dayScaleWindowsFor("30d"), 14},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maxWindowDays(tt.windows); got != tt.want {
				t.Errorf("maxWindowDays() = %d, want %d", got, tt.want)
			}
		})
	}
}

// visibleStartIndex は since 以降の最初の index を返し、全行が since 未満なら len（可視窓空）。
func TestVisibleStartIndex(t *testing.T) {
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	rows := []repository.SensorReading{
		sensorRow(1, base, 1, 1),                  // index0
		sensorRow(1, base.AddDate(0, 0, 1), 2, 2), // index1
		sensorRow(1, base.AddDate(0, 0, 2), 3, 3), // index2
		sensorRow(1, base.AddDate(0, 0, 3), 4, 4), // index3
	}
	tests := []struct {
		name  string
		since time.Time
		want  int
	}{
		{"全行が since 以降は0", base.AddDate(0, 0, -1), 0},
		{"先頭2行はルックバックで index2", base.AddDate(0, 0, 2), 2},
		{"境界(等値)は含む", base.AddDate(0, 0, 1), 1},
		{"全行が since 未満は len(可視窓空)", base.AddDate(0, 0, 10), len(rows)},
		{"空行は0", time.Time{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := visibleStartIndex(rows, tt.since)
			if tt.name == "空行は0" {
				got = visibleStartIndex(nil, tt.since)
			}
			if got != tt.want {
				t.Errorf("visibleStartIndex() = %d, want %d", got, tt.want)
			}
		})
	}
}

// daySMASeriesFor は各窓の SMA を全系列で算出し2桁丸めして可視窓へスライスする。
// 窓空は nil。各 Values 長 == len(fullValues)-visibleStart（可視窓長）。
func TestDaySMASeriesFor(t *testing.T) {
	// full = 10点（前半5点はルックバック相当）。ppd=1（1点/日想定）で 3日窓=3点平均。
	full := []float64{10, 10, 10, 10, 10, 20, 20, 20, 20, 20}
	visibleStart := 5

	t.Run("窓空はnil", func(t *testing.T) {
		if got := daySMASeriesFor(nil, full, 1, visibleStart); got != nil {
			t.Errorf("窓空は nil 想定, got %v", got)
		}
	})

	t.Run("7d窓2本_可視窓長_2桁丸め", func(t *testing.T) {
		windows := dayScaleWindowsFor("7d") // {3日, 7日}, ppd=1 → pts=3,7
		got := daySMASeriesFor(windows, full, 1, visibleStart)
		if len(got) != 2 {
			t.Fatalf("系列数 = %d, want 2", len(got))
		}
		for i, s := range got {
			if s.Label != windows[i].Label {
				t.Errorf("系列[%d].Label = %q, want %q", i, s.Label, windows[i].Label)
			}
			// 可視窓長 == len(full) - visibleStart。
			if len(s.Values) != len(full)-visibleStart {
				t.Errorf("系列[%d] Values 長 = %d, want %d", i, len(s.Values), len(full)-visibleStart)
			}
			// 2桁丸め（生値も整数なので破綻しないが、丸め関数が適用されること）。
			for _, v := range s.Values {
				if r := float64(int64(v*100)) / 100; r != v {
					t.Errorf("系列[%d] の値 %v が2桁丸めされていない", i, v)
				}
			}
		}
	})

	t.Run("点数換算はmax1でゼロ窓を防ぐ", func(t *testing.T) {
		// ppd が極小でも pts>=1 で空窓を避ける（線を欠落させない）。
		got := daySMASeriesFor(dayScaleWindowsFor("7d"), full, 0.0001, visibleStart)
		for _, s := range got {
			if len(s.Values) != len(full)-visibleStart {
				t.Errorf("pts>=1 で可視窓長が保たれていない: %d", len(s.Values))
			}
		}
	})
}

// --- 3.1 buildChartArea のルックバック分岐（統合・Querier モック） -----------

// rawSeriesLen は option JSON の series[0]（生実測線）のデータ点数を返す。
func rawSeriesLen(t *testing.T, optJSON string) int {
	t.Helper()
	var doc struct {
		Series []struct {
			Data []json.RawMessage `json:"data"`
		} `json:"series"`
	}
	if err := json.Unmarshal([]byte(optJSON), &doc); err != nil {
		t.Fatalf("option JSON が妥当でない: %v", err)
	}
	if len(doc.Series) == 0 {
		t.Fatalf("series が空")
	}
	return len(doc.Series[0].Data)
}

// 24h/3d は取得起点が periodSince のまま（ルックバックなし）で option に日スケール系列を含まない。
func TestBuildChartArea_短期ビューはルックバックせず日スケール系列なし(t *testing.T) {
	for _, period := range []string{"24h", "3d"} {
		t.Run(period, func(t *testing.T) {
			repo := showDeviceRepo()
			repo.recentReadings = []repository.SensorReading{
				sensorRow(1, fixedNow.Add(-3*time.Hour), 25.0, 60.0),
				sensorRow(1, fixedNow.Add(-2*time.Hour), 26.0, 61.0),
				sensorRow(1, fixedNow.Add(-1*time.Hour), 27.0, 62.0),
			}
			h := &DeviceHandler{Repo: repo}

			area, err := h.buildChartArea(context.Background(), repo.devices[1], period, fixedNow)
			if err != nil {
				t.Fatalf("buildChartArea() でエラー: %v", err)
			}
			// 取得起点 = periodSince（手前へ広げない＝既存パス完全保持）。
			wantStart := periodSince(period, fixedNow)
			if got := repo.lastRecentParams.RecordedAt.Time; !got.Equal(wantStart) {
				t.Errorf("%s 取得起点 = %v, want %v（periodSince のまま）", period, got, wantStart)
			}
			// option に日スケール系列（「移動平均 N日」）を含まない。
			if strings.Contains(area.TemperatureOptionJSON, "移動平均 ") {
				t.Errorf("%s の温度 option に日スケール系列が混入: %s", period, area.TemperatureOptionJSON)
			}
		})
	}
}

// 7d は取得起点を periodSince−7日 へ広げ、可視窓（periodSince 以降）のみを生実測線にし、
// option に「移動平均 3日」「移動平均 7日」を含む（ルックバックは warm-up 専用で表示しない）。
func TestBuildChartArea_7dはルックバック取得し可視窓スライスと日スケール系列(t *testing.T) {
	repo := showDeviceRepo()
	visibleSince := periodSince("7d", fixedNow) // = fixedNow-7d

	var rows []repository.SensorReading
	// ルックバック 7点（visibleSince より前・日次・温度10）。
	for d := 14; d >= 8; d-- {
		rows = append(rows, sensorRow(1, fixedNow.AddDate(0, 0, -d), 10.0, 40.0))
	}
	// 可視 8点（visibleSince 以降・日次・温度25）。
	for d := 7; d >= 0; d-- {
		rows = append(rows, sensorRow(1, fixedNow.AddDate(0, 0, -d), 25.0, 70.0))
	}
	repo.recentReadings = rows
	h := &DeviceHandler{Repo: repo}

	area, err := h.buildChartArea(context.Background(), repo.devices[1], "7d", fixedNow)
	if err != nil {
		t.Fatalf("buildChartArea() でエラー: %v", err)
	}
	if !area.HasData {
		t.Fatal("HasData=true 想定（可視窓に8点）")
	}
	// 取得起点 = periodSince − maxWindow(7日)。
	wantStart := visibleSince.AddDate(0, 0, -7)
	if got := repo.lastRecentParams.RecordedAt.Time; !got.Equal(wantStart) {
		t.Errorf("7d 取得起点 = %v, want %v（periodSince − 7日）", got, wantStart)
	}
	// 生実測線は可視窓のみ（8点・ルックバック7点は表示しない）。
	if got := rawSeriesLen(t, area.TemperatureOptionJSON); got != 8 {
		t.Errorf("生実測線の点数 = %d, want 8（可視窓のみ）", got)
	}
	// option に日スケール系列を含む。
	for _, label := range []string{"移動平均 3日", "移動平均 7日"} {
		if !strings.Contains(area.TemperatureOptionJSON, label) {
			t.Errorf("温度 option に %q が無い", label)
		}
		if !strings.Contains(area.HumidityOptionJSON, label) {
			t.Errorf("湿度 option に %q が無い", label)
		}
	}
}

// 30d は取得起点を periodSince−14日 へ広げ、option に3本（移動平均 14日 を含む）を含む。
func TestBuildChartArea_30dは最長14日ルックバックと3系列(t *testing.T) {
	repo := showDeviceRepo()
	var rows []repository.SensorReading
	for d := 44; d >= 0; d-- { // periodSince(30d)-14d = fixedNow-44d 以降を網羅
		temp := 25.0
		if d > 30 {
			temp = 10.0 // ルックバック相当
		}
		rows = append(rows, sensorRow(1, fixedNow.AddDate(0, 0, -d), temp, 70.0))
	}
	repo.recentReadings = rows
	h := &DeviceHandler{Repo: repo}

	area, err := h.buildChartArea(context.Background(), repo.devices[1], "30d", fixedNow)
	if err != nil {
		t.Fatalf("buildChartArea() でエラー: %v", err)
	}
	wantStart := periodSince("30d", fixedNow).AddDate(0, 0, -14)
	if got := repo.lastRecentParams.RecordedAt.Time; !got.Equal(wantStart) {
		t.Errorf("30d 取得起点 = %v, want %v（periodSince − 14日）", got, wantStart)
	}
	for _, label := range []string{"移動平均 3日", "移動平均 7日", "移動平均 14日"} {
		if !strings.Contains(area.TemperatureOptionJSON, label) {
			t.Errorf("温度 option に %q が無い", label)
		}
	}
}

// ルックバックにデータがあっても可視窓が0件なら HasData=false（空表示・6.4）。
func TestBuildChartArea_可視窓0件はHasData_false(t *testing.T) {
	repo := showDeviceRepo()
	// すべて visibleSince(=fixedNow-7d) より前のルックバック行のみ。
	var rows []repository.SensorReading
	for d := 14; d >= 8; d-- {
		rows = append(rows, sensorRow(1, fixedNow.AddDate(0, 0, -d), 10.0, 40.0))
	}
	repo.recentReadings = rows
	h := &DeviceHandler{Repo: repo}

	area, err := h.buildChartArea(context.Background(), repo.devices[1], "7d", fixedNow)
	if err != nil {
		t.Fatalf("buildChartArea() でエラー: %v", err)
	}
	if area.HasData {
		t.Error("可視窓0件は HasData=false 想定（ルックバックにデータがあっても表示しない・6.4）")
	}
	if area.TemperatureOptionJSON != "" {
		t.Errorf("可視窓0件で option が組まれている: %s", area.TemperatureOptionJSON)
	}
}

// 日スケール系列の値長が Labels（可視窓）と同長であること（applyGapGrid 前の不変条件・chart 契約）。
func TestBuildChartArea_日スケール系列はLabelsと同長(t *testing.T) {
	repo := showDeviceRepo()
	var rows []repository.SensorReading
	for d := 14; d >= 0; d-- {
		rows = append(rows, sensorRow(1, fixedNow.AddDate(0, 0, -d), 25.0, 70.0))
	}
	repo.recentReadings = rows
	h := &DeviceHandler{Repo: repo}

	area, err := h.buildChartArea(context.Background(), repo.devices[1], "7d", fixedNow)
	if err != nil {
		t.Fatalf("buildChartArea() でエラー: %v", err)
	}
	// option をパースして series 全体の data 長が一致することを確認（生線=可視窓長と日スケール系列が同長）。
	var doc struct {
		Series []struct {
			Name string            `json:"name"`
			Data []json.RawMessage `json:"data"`
		} `json:"series"`
	}
	if err := json.Unmarshal([]byte(area.TemperatureOptionJSON), &doc); err != nil {
		t.Fatalf("option JSON が妥当でない: %v", err)
	}
	rawLen := -1
	for _, s := range doc.Series {
		if s.Name == tempChartUnit {
			rawLen = len(s.Data)
		}
	}
	if rawLen < 0 {
		t.Fatal("生実測線 series が見つからない")
	}
	for _, s := range doc.Series {
		if strings.HasPrefix(s.Name, "移動平均 ") && len(s.Data) != rawLen {
			t.Errorf("%s の長さ = %d, want %d（生線=Labels と同長）", s.Name, len(s.Data), rawLen)
		}
	}
}

// 欠測ありの 7d でも、欠測グリッド拡張後に日スケール系列が生実測線(拡張 Labels)と同長になること
// （applyGapGrid の DaySMAs carry-forward・タスク4が解消する整列ズレの統合検証・6.2）。
func TestBuildChartArea_欠測ありでも日スケール系列がLabelsと同長(t *testing.T) {
	repo := showDeviceRepo()
	// 可視窓(7d 内)に 5分間隔の点列＋1区間だけ 30分ギャップ（中央値5分に対し欠測検出）。
	base := fixedNow.Add(-1 * time.Hour) // 直近1時間内＝7d 可視窓
	repo.recentReadings = []repository.SensorReading{
		sensorRow(1, base, 20, 60),
		sensorRow(1, base.Add(5*time.Minute), 21, 61),
		sensorRow(1, base.Add(10*time.Minute), 20, 60),
		sensorRow(1, base.Add(40*time.Minute), 22, 62), // 30分ギャップ
		sensorRow(1, base.Add(45*time.Minute), 21, 61),
	}
	h := &DeviceHandler{Repo: repo}

	area, err := h.buildChartArea(context.Background(), repo.devices[1], "7d", fixedNow)
	if err != nil {
		t.Fatalf("buildChartArea() でエラー: %v", err)
	}
	if !area.HasGap {
		t.Fatal("HasGap=true 想定（欠測あり）")
	}
	var doc struct {
		Series []struct {
			Name string            `json:"name"`
			Data []json.RawMessage `json:"data"`
		} `json:"series"`
	}
	if err := json.Unmarshal([]byte(area.TemperatureOptionJSON), &doc); err != nil {
		t.Fatalf("option JSON が妥当でない: %v", err)
	}
	rawLen := -1
	for _, s := range doc.Series {
		if s.Name == tempChartUnit {
			rawLen = len(s.Data)
		}
	}
	if rawLen < 0 {
		t.Fatal("生実測線 series が見つからない")
	}
	daySeriesSeen := 0
	for _, s := range doc.Series {
		if strings.HasPrefix(s.Name, "移動平均 ") {
			daySeriesSeen++
			if len(s.Data) != rawLen {
				t.Errorf("%s の長さ = %d, want %d（欠測グリッド拡張後も生線と同長）", s.Name, len(s.Data), rawLen)
			}
		}
	}
	if daySeriesSeen == 0 {
		t.Error("日スケール系列が option に無い（7d で付与されるはず）")
	}
}
