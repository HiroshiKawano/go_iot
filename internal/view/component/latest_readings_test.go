package component

import (
	"strings"
	"testing"
)

func TestLatestReadingsTable_最新行と列とtbodyId(t *testing.T) {
	v := LatestReadingsView{
		DeviceID: 1,
		Rows: []ReadingRow{
			{RecordedAt: "2026-04-20 14:30", Temp: "28.50", Humidity: "65.30"},
			{RecordedAt: "2026-04-20 14:25", Temp: "28.30", Humidity: "65.50"},
			{RecordedAt: "2026-04-20 14:20", Temp: "28.10", Humidity: "65.80"},
		},
	}
	html := render(t, LatestReadingsTable(v))

	// HTMX 差し替え専用 id は tbody に付与（スタイリングには使わない）
	assertContains(t, html, `id="latest-readings-table"`)
	assertContains(t, html, "data-table")

	// 列見出し（計測日時・温度・湿度）
	assertContains(t, html, "計測日時")
	assertContains(t, html, "温度(℃)")
	assertContains(t, html, "湿度(%)")

	// 各行の値（日時 YYYY-MM-DD HH:MM / 温湿度 小数2桁）
	for _, want := range []string{"2026-04-20 14:30", "28.50", "65.30", "2026-04-20 14:20", "28.10"} {
		assertContains(t, html, want)
	}
	// 3 行 = 9 セル
	if got := strings.Count(html, "<td>"); got != 9 {
		t.Errorf("<td> の数 = %d, want 9\n%s", got, html)
	}

	// もっと見る導線（S6 の履歴一覧へ）
	assertContains(t, html, `href="/devices/1/readings"`)

	// データありなので空メッセージは出さない
	if strings.Contains(html, "計測データはまだありません。") {
		t.Errorf("データありなのに空メッセージが描画されている:\n%s", html)
	}
}

func TestLatestReadingsTable_0件で空メッセージとtbody常設(t *testing.T) {
	html := render(t, LatestReadingsTable(LatestReadingsView{DeviceID: 5}))

	// tbody（id）は 0 件でも常設（HTMX ターゲットを失わない）
	assertContains(t, html, `id="latest-readings-table"`)
	assertContains(t, html, "計測データはまだありません。")
	if strings.Contains(html, "<td>") {
		t.Errorf("0 件なのにデータ行が描画されている:\n%s", html)
	}
}
