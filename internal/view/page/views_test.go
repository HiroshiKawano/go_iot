package page

import (
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/view/component"
	"github.com/HiroshiKawano/go_iot/internal/view/layout"
)

// TestDashboardView_表示用フィールドを保持する は、ダッシュボード View-model が
// 整形済み primitive（文字列・bool・ID）のみで描画データを保持できることを検証する。
func TestDashboardView_表示用フィールドを保持する(t *testing.T) {
	v := DashboardView{
		Layout: layout.AppLayoutData{Title: "ダッシュボード", UserName: "山田太郎"},
		Devices: []component.DashboardDevice{
			{
				ID:           1,
				Name:         "ハウスA温湿度計",
				Location:     "第1ハウス",
				IsActive:     true,
				TempText:     "28.50℃",
				HumidityText: "65.30%",
				LastCommText: "2分前",
			},
		},
		Alerts: []component.DashboardAlert{
			{Message: "ハウスA温湿度計: 温度が35℃を超えました（38.50℃）"},
		},
	}

	if v.Layout.UserName != "山田太郎" {
		t.Errorf("Layout.UserName = %q, want 山田太郎", v.Layout.UserName)
	}

	d := v.Devices[0]
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"Name", d.Name, "ハウスA温湿度計"},
		{"Location", d.Location, "第1ハウス"},
		{"TempText", d.TempText, "28.50℃"},
		{"HumidityText", d.HumidityText, "65.30%"},
		{"LastCommText", d.LastCommText, "2分前"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("Device.%s = %q, want %q", c.name, c.got, c.want)
		}
	}
	if d.ID != 1 {
		t.Errorf("Device.ID = %d, want 1", d.ID)
	}
	if !d.IsActive {
		t.Error("Device.IsActive = false, want true")
	}

	if got := v.Alerts[0].Message; got != "ハウスA温湿度計: 温度が35℃を超えました（38.50℃）" {
		t.Errorf("Alert.Message = %q", got)
	}
}

// TestReadingsView_表示用フィールドを保持する は、センサーデータ履歴フルページの
// View-model が レイアウト・デバイスID・デバイス名・フォーム echo 値(from/to)・
// 結果領域 component DTO を整形済み primitive のみで保持できることを検証する。
func TestReadingsView_表示用フィールドを保持する(t *testing.T) {
	v := ReadingsView{
		Layout:     layout.AppLayoutData{Title: "センサーデータ履歴", UserName: "山田太郎"},
		DeviceID:   42,
		DeviceName: "ハウスA温湿度計",
		From:       "2026-04-01",
		To:         "2026-04-20",
		List: component.DeviceReadingsListView{
			Summary: component.SummaryView{AvgTemp: "28.30℃", AvgHum: "65.30%"},
			Rows: []component.ReadingHistoryRow{
				{RecordedAt: "2026-04-20 14:30", Temp: "28.50", Humidity: "65.30", Delay: "2秒"},
			},
			HasData:    true,
			Pagination: component.PaginationView{Current: 1, Last: 3, HasNext: true},
			Errors:     map[string]string{},
		},
	}

	if v.Layout.UserName != "山田太郎" {
		t.Errorf("Layout.UserName = %q, want 山田太郎", v.Layout.UserName)
	}
	if v.DeviceID != 42 {
		t.Errorf("DeviceID = %d, want 42", v.DeviceID)
	}
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"DeviceName", v.DeviceName, "ハウスA温湿度計"},
		{"From", v.From, "2026-04-01"},
		{"To", v.To, "2026-04-20"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}

	// 結果領域 DTO を内包し、行・集計・ページャへ辿れる。
	if !v.List.HasData {
		t.Error("List.HasData = false, want true")
	}
	if got := v.List.Rows[0].Delay; got != "2秒" {
		t.Errorf("List.Rows[0].Delay = %q, want 2秒", got)
	}
	if got := v.List.Summary.AvgTemp; got != "28.30℃" {
		t.Errorf("List.Summary.AvgTemp = %q, want 28.30℃", got)
	}
	if v.List.Pagination.Last != 3 {
		t.Errorf("List.Pagination.Last = %d, want 3", v.List.Pagination.Last)
	}
}

// TestPageパッケージはrepositoryとpgtypeをimportしない は view 純粋性
// （依存方向ルール: view は repository/pgtype を import しない）を AST 走査で守る。
// テストファイルを除く page パッケージの全 .go ファイルの直接 import を検査する。
func TestPageパッケージはrepositoryとpgtypeをimportしない(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}

	forbidden := []string{"internal/repository", "pgtype", "jackc/pgx"}
	for _, pkg := range pkgs {
		for fname, file := range pkg.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				for _, f := range forbidden {
					if strings.Contains(path, f) {
						t.Errorf("%s が禁止 import %q を含む (view 純粋性違反)", fname, path)
					}
				}
			}
		}
	}
}
