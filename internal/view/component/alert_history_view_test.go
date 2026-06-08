package component

import (
	"strings"
	"testing"
)

// --- 2.1 AlertHistoryPagination（番号付きページャ） ---

// fullPaginationView は総3ページ・現在2ページ目のページャ View（前後リンク有）。
func fullPaginationView() AlertHistoryPaginationView {
	return AlertHistoryPaginationView{
		HasPrev: true, HasNext: true,
		PrevURL: "/alerts/history?page=1",
		NextURL: "/alerts/history?page=3",
		Pages: []PageLink{
			{Num: 1, URL: "/alerts/history?page=1", Current: false},
			{Num: 2, URL: "/alerts/history?page=2", Current: true},
			{Num: 3, URL: "/alerts/history?page=3", Current: false},
		},
	}
}

// TestAlertHistoryPagination_番号と前後リンク は、総3ページ・現在2のとき「前へ」リンク・
// 番号 1/3 のリンク・2 が .current・「次へ」リンクを描画し、HTMX 部分更新属性
// (hx-boost で #alert-history-list を innerHTML swap) を備えることを検証する (R3.1/3.4)。
func TestAlertHistoryPagination_番号と前後リンク(t *testing.T) {
	html := render(t, AlertHistoryPagination(fullPaginationView()))

	// HTMX 部分更新属性（ページ送りリンクは hx-boost で #alert-history-list を対象）。
	assertContains(t, html, `class="pagination"`)
	assertContains(t, html, `hx-boost="true"`)
	assertContains(t, html, `hx-target="#alert-history-list"`)
	assertContains(t, html, `hx-swap="innerHTML"`)

	// 前へ・次へはリンク。
	assertContains(t, html, `>← 前へ</a>`)
	assertContains(t, html, `>次へ →</a>`)
	assertContains(t, html, "/alerts/history?page=1") // 前へ URL
	assertContains(t, html, "/alerts/history?page=3") // 次へ URL

	// 番号 1/3 はリンク、2 は .current（リンクにしない）。
	assertContains(t, html, `>1</a>`)
	assertContains(t, html, `>3</a>`)
	assertContains(t, html, `<span class="current">2</span>`)
	if strings.Contains(html, `>2</a>`) {
		t.Errorf("現在ページ2がリンクで描画されている:\n%s", html)
	}
}

// TestAlertHistoryPagination_先頭ページは前へ無効 は、前ページ無し時に「前へ」を
// .disabled の span（リンクでない）で描画することを検証する (R3.5)。
func TestAlertHistoryPagination_先頭ページは前へ無効(t *testing.T) {
	v := AlertHistoryPaginationView{
		HasPrev: false, HasNext: true,
		NextURL: "/alerts/history?page=2",
		Pages: []PageLink{
			{Num: 1, Current: true},
			{Num: 2, URL: "/alerts/history?page=2"},
			{Num: 3, URL: "/alerts/history?page=3"},
		},
	}
	html := render(t, AlertHistoryPagination(v))

	assertContains(t, html, `<span class="disabled">← 前へ</span>`)
	assertContains(t, html, `>次へ →</a>`) // 次へはリンク
	if strings.Contains(html, `>← 前へ</a>`) {
		t.Errorf("先頭ページで「前へ」がリンクで描画されている:\n%s", html)
	}
}

// TestAlertHistoryPagination_最終ページは次へ無効 は、次ページ無し時に「次へ」を
// .disabled の span（リンクでない）で描画することを検証する (R3.6)。
func TestAlertHistoryPagination_最終ページは次へ無効(t *testing.T) {
	v := AlertHistoryPaginationView{
		HasPrev: true, HasNext: false,
		PrevURL: "/alerts/history?page=2",
		Pages: []PageLink{
			{Num: 1, URL: "/alerts/history?page=1"},
			{Num: 2, URL: "/alerts/history?page=2"},
			{Num: 3, Current: true},
		},
	}
	html := render(t, AlertHistoryPagination(v))

	assertContains(t, html, `<span class="disabled">次へ →</span>`)
	assertContains(t, html, `>← 前へ</a>`) // 前へはリンク
	if strings.Contains(html, `>次へ →</a>`) {
		t.Errorf("最終ページで「次へ」がリンクで描画されている:\n%s", html)
	}
}

// --- 2.2 AlertHistoryList（結果領域 fragment） ---

// fullAlertHistoryListView は行2件・ページャ有の結果領域 View。
func fullAlertHistoryListView() AlertHistoryListView {
	return AlertHistoryListView{
		Rows: []AlertHistoryRow{
			{TriggeredAt: "2026-04-20 14:30", DeviceName: "ハウスA温湿度計", MetricLabel: "温度", Condition: "> 35.00℃", ActualValue: "38.50℃", Notified: "済"},
			{TriggeredAt: "2026-04-20 13:15", DeviceName: "ハウスB温湿度計", MetricLabel: "湿度", Condition: "< 30.00%", ActualValue: "25.00%", Notified: "未"},
		},
		HasData:       true,
		HasPagination: true,
		Pagination: AlertHistoryPaginationView{
			HasNext: true, NextURL: "/alerts/history?page=2",
			Pages: []PageLink{{Num: 1, Current: true}, {Num: 2, URL: "/alerts/history?page=2"}},
		},
		Errors: map[string]string{},
	}
}

// TestAlertHistoryList_行ありで6列テーブルとページャ は、データ有時にフラグメント id・
// 6列見出し・各行6セル（条件/実測値は HTML エスケープ済み）・ページャ内包を検証する
// (R4.1〜4.6, R3.1)。
func TestAlertHistoryList_行ありで6列テーブルとページャ(t *testing.T) {
	html := render(t, AlertHistoryList(fullAlertHistoryListView()))

	// HTMX ターゲットのフラグメント id（ラッパー）。
	assertContains(t, html, `id="alert-history-list"`)

	// 6列見出し（発火日時/デバイス/指標/ルール(条件/閾値)/実測値/通知）。
	assertContains(t, html, "data-table")
	for _, h := range []string{"発火日時", "デバイス", "指標", "ルール", "実測値", "通知"} {
		assertContains(t, html, h)
	}

	// 1行目の6セル（条件 ">" は templ が &gt; にエスケープ）。
	for _, cell := range []string{"2026-04-20 14:30", "ハウスA温湿度計", "温度", "&gt; 35.00℃", "38.50℃", "済"} {
		assertContains(t, html, cell)
	}
	// 2行目（湿度・"<" は &lt;・未通知）。
	for _, cell := range []string{"2026-04-20 13:15", "ハウスB温湿度計", "湿度", "&lt; 30.00%", "25.00%", "未"} {
		assertContains(t, html, cell)
	}

	// 各行が6列であること（2行 × 6セル = 12 td）。
	if got := strings.Count(html, "<td>"); got != 12 {
		t.Errorf("td 数が 12 でない: got=%d\n%s", got, html)
	}

	// ページャを内包（HasPagination=true）。
	assertContains(t, html, `class="pagination"`)
	assertContains(t, html, `<span class="current">1</span>`)

	// データ有なので空状態メッセージは出さない。
	if strings.Contains(html, "指定期間のアラート履歴はありません。") {
		t.Errorf("データ有なのに空状態メッセージが描画されている:\n%s", html)
	}
}

// TestAlertHistoryList_0件で空状態かつページャ非表示 は、行0件時にテーブルを描画せず
// 空状態メッセージを表示し、ページャを出力しないこと (R6.1/6.2) を検証する。
func TestAlertHistoryList_0件で空状態かつページャ非表示(t *testing.T) {
	v := AlertHistoryListView{
		Rows:          nil,
		HasData:       false,
		HasPagination: false,
		Pagination:    AlertHistoryPaginationView{},
		Errors:        map[string]string{},
	}
	html := render(t, AlertHistoryList(v))

	assertContains(t, html, `id="alert-history-list"`)
	assertContains(t, html, "指定期間のアラート履歴はありません。")
	if strings.Contains(html, "data-table") {
		t.Errorf("0件なのにデータ一覧テーブルが描画されている:\n%s", html)
	}
	if strings.Contains(html, `class="pagination"`) {
		t.Errorf("0件なのにページャが描画されている:\n%s", html)
	}
}

// TestAlertHistoryList_終了日エラーをインライン表示 は、Errors["to"] 指定時に
// フラグメント内へ .error-message として該当文言を描画すること (R7.1) を検証する。
func TestAlertHistoryList_終了日エラーをインライン表示(t *testing.T) {
	v := AlertHistoryListView{
		Rows:          nil,
		HasData:       false,
		HasPagination: false,
		Errors:        map[string]string{"to": "終了日は開始日以降の日付を指定してください"},
	}
	html := render(t, AlertHistoryList(v))

	assertContains(t, html, `id="alert-history-list"`)
	assertContains(t, html, "error-message")
	assertContains(t, html, "終了日は開始日以降の日付を指定してください")
	// from キー不在なので開始日メッセージは描画しない。
	if strings.Contains(html, "開始日は") {
		t.Errorf("終了日のみエラーなのに開始日メッセージが描画されている:\n%s", html)
	}
}

// TestAlertHistoryList_開始日エラーをインライン表示 は、Errors["from"] 指定時に
// 開始日メッセージを描画し、終了日メッセージは描画しないことを検証する
// （from キー有=描画 / to キー不在=skip の両分岐を被覆）。
func TestAlertHistoryList_開始日エラーをインライン表示(t *testing.T) {
	v := AlertHistoryListView{
		Rows:          nil,
		HasData:       false,
		HasPagination: false,
		Errors:        map[string]string{"from": "開始日は YYYY-MM-DD 形式で入力してください"},
	}
	html := render(t, AlertHistoryList(v))

	assertContains(t, html, "error-message")
	assertContains(t, html, "開始日は YYYY-MM-DD 形式で入力してください")
	// to キーが無いので終了日メッセージは描画しない。
	if strings.Contains(html, "終了日は") {
		t.Errorf("開始日のみエラーなのに終了日メッセージが描画されている:\n%s", html)
	}
}
