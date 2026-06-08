package component

import (
	"strings"
	"testing"
)

// --- 3.1 Pagination（簡易ページャ） ---

// TestPagination_前後リンクと現在総ページ は、前後ページ有時に「前へ」「次へ」リンクと
// 「現在 / 総ページ」表記を描画し、HTMX 部分更新属性 (hx-boost/target/swap) を備えることを検証する。
func TestPagination_前後リンクと現在総ページ(t *testing.T) {
	v := PaginationView{
		Current: 2, Last: 5, HasPrev: true, HasNext: true,
		PrevURL: "/devices/42/readings?page=1",
		NextURL: "/devices/42/readings?page=3",
	}
	html := render(t, Pagination(v))

	// HTMX 部分更新属性（C05 / 結果領域をターゲットに内側差し替え）。
	assertContains(t, html, `class="pagination"`)
	assertContains(t, html, `hx-boost="true"`)
	assertContains(t, html, `hx-target="#device-readings-list"`)
	assertContains(t, html, `hx-swap="innerHTML"`)

	// 現在 / 総ページ表記と前後リンク。
	assertContains(t, html, "2 / 5 ページ")
	assertContains(t, html, "前へ")
	assertContains(t, html, "次へ")
	assertContains(t, html, "/devices/42/readings?page=1")
	assertContains(t, html, "/devices/42/readings?page=3")

	// 既存クラスのみ使用（独自クラス新設禁止・§31）。
	assertContains(t, html, "btn btn-small btn-secondary")
}

// TestPagination_先頭ページは前へを出さない は、前ページ無し時に「前へ」リンクを描画しないことを検証する。
func TestPagination_先頭ページは前へを出さない(t *testing.T) {
	v := PaginationView{
		Current: 1, Last: 3, HasPrev: false, HasNext: true,
		NextURL: "/devices/42/readings?page=2",
	}
	html := render(t, Pagination(v))

	assertContains(t, html, "1 / 3 ページ")
	assertContains(t, html, "次へ")
	if strings.Contains(html, "前へ") {
		t.Errorf("先頭ページで「前へ」が描画されている:\n%s", html)
	}
}

// TestPagination_最終ページは次へを出さない は、次ページ無し時に「次へ」リンクを描画しないことを検証する。
func TestPagination_最終ページは次へを出さない(t *testing.T) {
	v := PaginationView{
		Current: 3, Last: 3, HasPrev: true, HasNext: false,
		PrevURL: "/devices/42/readings?page=2",
	}
	html := render(t, Pagination(v))

	assertContains(t, html, "3 / 3 ページ")
	assertContains(t, html, "前へ")
	if strings.Contains(html, "次へ") {
		t.Errorf("最終ページで「次へ」が描画されている:\n%s", html)
	}
}

// --- 3.2 DeviceReadingsList（フィルタ結果領域 fragment） ---

func fullListView() DeviceReadingsListView {
	return DeviceReadingsListView{
		Summary: SummaryView{
			AvgTemp: "28.30℃", MaxTemp: "35.20℃", MinTemp: "18.50℃",
			AvgHum: "62.50%", MaxHum: "85.00%", MinHum: "30.20%",
		},
		Rows: []ReadingHistoryRow{
			{RecordedAt: "2026-04-20 14:30", Temp: "28.50", Humidity: "65.30", Delay: "2秒"},
			{RecordedAt: "2026-04-20 14:25", Temp: "28.30", Humidity: "65.50", Delay: "1秒"},
		},
		HasData:    true,
		Pagination: PaginationView{Current: 1, Last: 2, HasNext: true, NextURL: "/devices/1/readings?page=2"},
		Errors:     map[string]string{},
	}
}

// TestDeviceReadingsList_行ありで集計と一覧とページャ は、データ有時にフラグメント id・
// 集計6項目・4列データ一覧・ページャを内包することを Render 結果で検証する。
func TestDeviceReadingsList_行ありで集計と一覧とページャ(t *testing.T) {
	html := render(t, DeviceReadingsList(fullListView()))

	// HTMX ターゲットのフラグメント id（ラッパー）。
	assertContains(t, html, `id="device-readings-list"`)

	// 集計6項目（ラベル + 整形済み値）。
	assertContains(t, html, "summary-grid")
	for _, label := range []string{"平均温度", "最高温度", "最低温度", "平均湿度", "最高湿度", "最低湿度"} {
		assertContains(t, html, label)
	}
	for _, val := range []string{"28.30℃", "35.20℃", "18.50℃", "62.50%", "85.00%", "30.20%"} {
		assertContains(t, html, val)
	}

	// データ一覧（4列・通信遅延を含む）。
	assertContains(t, html, "data-table")
	assertContains(t, html, "通信遅延")
	for _, cell := range []string{"2026-04-20 14:30", "28.50", "65.30", "2秒", "1秒"} {
		assertContains(t, html, cell)
	}

	// ページャを内包。
	assertContains(t, html, "1 / 2 ページ")
	assertContains(t, html, "次へ")

	// データ有なので空状態メッセージは出さない。
	if strings.Contains(html, "指定期間の計測データはありません。") {
		t.Errorf("データ有なのに空状態メッセージが描画されている:\n%s", html)
	}
}

// TestDeviceReadingsList_0件で空状態メッセージかつテーブル非表示 は、行0件時に
// データ一覧テーブルを描画せず空状態メッセージを表示すること (R7.1) を検証する。
func TestDeviceReadingsList_0件で空状態メッセージかつテーブル非表示(t *testing.T) {
	v := DeviceReadingsListView{
		Summary: SummaryView{
			AvgTemp: "—", MaxTemp: "—", MinTemp: "—",
			AvgHum: "—", MaxHum: "—", MinHum: "—",
		},
		Rows:       nil,
		HasData:    false,
		Pagination: PaginationView{Current: 1, Last: 1},
		Errors:     map[string]string{},
	}
	html := render(t, DeviceReadingsList(v))

	assertContains(t, html, `id="device-readings-list"`)
	assertContains(t, html, "指定期間の計測データはありません。")
	// 0件時はテーブル本体を描画しない（テーブル非表示）。
	if strings.Contains(html, "data-table") {
		t.Errorf("0件なのにデータ一覧テーブルが描画されている:\n%s", html)
	}
	// 集計は「—」表示。
	assertContains(t, html, "—")
}

// TestDeviceReadingsList_エラー有でインラインエラー は、エラーマップ非空時に
// フラグメント内へインラインエラーを描画すること (R6.2/6.3) を検証する。
func TestDeviceReadingsList_エラー有でインラインエラー(t *testing.T) {
	v := DeviceReadingsListView{
		Summary:    SummaryView{AvgTemp: "—", MaxTemp: "—", MinTemp: "—", AvgHum: "—", MaxHum: "—", MinHum: "—"},
		Rows:       nil,
		HasData:    false,
		Pagination: PaginationView{Current: 1, Last: 1},
		Errors:     map[string]string{"from": "開始日は YYYY-MM-DD 形式で入力してください"},
	}
	html := render(t, DeviceReadingsList(v))

	assertContains(t, html, `id="device-readings-list"`)
	assertContains(t, html, "error-message")
	assertContains(t, html, "開始日は YYYY-MM-DD 形式で入力してください")
}

// TestDeviceReadingsList_終了日のみエラーでも描画 は、終了日のみ形式不正のとき
// 終了日のエラーメッセージを描画し、開始日エラーは描画しないことを検証する
// （from キー不在=skip / to キー有=描画 の両分岐を被覆）。
func TestDeviceReadingsList_終了日のみエラーでも描画(t *testing.T) {
	v := DeviceReadingsListView{
		Summary:    SummaryView{AvgTemp: "—", MaxTemp: "—", MinTemp: "—", AvgHum: "—", MaxHum: "—", MinHum: "—"},
		Rows:       nil,
		HasData:    false,
		Pagination: PaginationView{Current: 1, Last: 1},
		Errors:     map[string]string{"to": "終了日は YYYY-MM-DD 形式で入力してください"},
	}
	html := render(t, DeviceReadingsList(v))

	assertContains(t, html, "error-message")
	assertContains(t, html, "終了日は YYYY-MM-DD 形式で入力してください")
	// 開始日キーが無いので開始日メッセージは描画しない。
	if strings.Contains(html, "開始日は") {
		t.Errorf("終了日のみエラーなのに開始日メッセージが描画されている:\n%s", html)
	}
}
