package component

import (
	"strings"
	"testing"
)

// readings_report_view_test.go は結果領域 fragment (DeviceReadingsList) へ追加した
// 集計帳票表 (日次/時間別) と CSV ダウンロードボタンの描画を Render 結果で検証する
// (テストガイダンス集 §4 templ・mocks/html/readings.html を写経した正本との一致確認)。

// reportListView は帳票・CSV リンク付きの結果領域 View を組む (帳票描画テスト用)。
func reportListView() DeviceReadingsListView {
	v := fullListView()
	v.CSVURL = "/devices/1/readings.csv?from=2026-04-13&to=2026-04-20&items=temperature"
	v.Report = ReadingsReportView{
		CropLabel:  "ゴーヤ",
		RangeLabel: "0.40〜1.20 kPa",
		HasData:    true,
		Daily: []ReadingsReportRow{
			{
				Bucket: "2026-04-20",
				TempAvg: "28.50", TempMax: "35.20", TempMin: "18.50",
				TempDiurnal: "16.70", TempSigma: "4.32", TempCV: "0.15",
				HumAvg: "62.50", HumMax: "85.00", HumMin: "30.20",
				HumDiurnal: "54.80", HumSigma: "13.50", HumCV: "0.22",
				InRange: "72%",
			},
			{
				Bucket: "2026-04-19",
				TempAvg: "—", TempMax: "—", TempMin: "—", TempDiurnal: "—", TempSigma: "—", TempCV: "—",
				HumAvg: "—", HumMax: "—", HumMin: "—", HumDiurnal: "—", HumSigma: "—", HumCV: "—",
				InRange: "—",
			},
		},
		Hourly: []ReadingsReportRow{
			{
				Bucket: "06時",
				TempAvg: "22.40", TempMax: "24.10", TempMin: "19.40",
				TempDiurnal: "4.70", TempSigma: "1.30", TempCV: "0.06",
				HumAvg: "78.20", HumMax: "85.00", HumMin: "68.00",
				HumDiurnal: "17.00", HumSigma: "4.20", HumCV: "0.05",
				InRange: "40%",
			},
		},
	}
	return v
}

// TestDeviceReadingsList_帳票表とCSVボタンを描画する は、Report.HasData かつ CSVURL 有時に
// 日次/時間別の集計帳票表 (.data-table) と CSV ダウンロードボタン (HTMX 非対象) を描画することを検証する。
func TestDeviceReadingsList_帳票表とCSVボタンを描画する(t *testing.T) {
	html := render(t, DeviceReadingsList(reportListView()))

	// CSV ダウンロードボタン: ファイルDL ゆえ HTMX 非対象 (hx-boost="false" + download)・既存 .btn 流用。
	assertContains(t, html, "CSV ダウンロード")
	assertContains(t, html, `hx-boost="false"`)
	assertContains(t, html, "download")
	assertContains(t, html, "btn")
	// href は適用済み from を保持 (& は属性で &amp; にエスケープされるため & 前まで検証)。
	assertContains(t, html, "/devices/1/readings.csv?from=2026-04-13")

	// 帳票の見出しと作物/適正帯ラベル。
	assertContains(t, html, "集計帳票（日次）")
	assertContains(t, html, "集計帳票（時間別）")
	assertContains(t, html, "ゴーヤ")
	assertContains(t, html, "0.40〜1.20 kPa")

	// 帳票表 (温度/湿度のグルーピング見出し + 適正帯滞在率列)。
	assertContains(t, html, "data-table")
	assertContains(t, html, "適正帯滞在率")
	assertContains(t, html, "日較差(℃)")
	assertContains(t, html, "日付")
	assertContains(t, html, "時間帯")

	// 日次バケットの値 (整形済み・欠測 "—" 含む) と時間別バケットの値。
	for _, cell := range []string{"2026-04-20", "28.50", "72%", "2026-04-19", "06時", "40%"} {
		assertContains(t, html, cell)
	}
}

// TestDeviceReadingsList_帳票なしCSVなしは描画しない は、Report.HasData=false かつ CSVURL 空時に
// 帳票表・CSV ボタンを描画しないことを検証する (handler 未結線時=既存 S6 表示の無回帰)。
func TestDeviceReadingsList_帳票なしCSVなしは描画しない(t *testing.T) {
	v := fullListView() // Report ゼロ値・CSVURL ""
	html := render(t, DeviceReadingsList(v))

	if strings.Contains(html, "集計帳票") {
		t.Errorf("帳票なしなのに集計帳票が描画されている:\n%s", html)
	}
	if strings.Contains(html, "CSV ダウンロード") {
		t.Errorf("CSVURL 空なのに CSV ボタンが描画されている:\n%s", html)
	}
	// 既存の集計6項目・データ一覧は従来どおり描画 (無回帰)。
	assertContains(t, html, "summary-grid")
	assertContains(t, html, "通信遅延")
}
