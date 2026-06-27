package layout

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func assertContains(t *testing.T, html, substr string) {
	t.Helper()
	if !strings.Contains(html, substr) {
		t.Errorf("出力に %q が含まれていない:\n%s", substr, html)
	}
}

func TestApp_共通要素を描画(t *testing.T) {
	data := AppLayoutData{
		Title:     "ダッシュボード - 農業IoTシステム",
		UserName:  "テストユーザー",
		CSRFToken: "tok-xyz",
		CSSURL:    "/static/css/style.css?v=dev",
	}
	html := render(t, App(data))

	assertContains(t, html, `id="main-content"`)           // HTMX 差し替えターゲット
	assertContains(t, html, `name="csrf-token"`)           // meta tag
	assertContains(t, html, "tok-xyz")                     // csrf トークン値
	assertContains(t, html, `id="flash-message"`)          // 共通通知領域
	assertContains(t, html, "/static/css/style.css?v=dev") // CSSURL
	assertContains(t, html, "テストユーザー")                     // SiteHeader 経由
	assertContains(t, html, "htmx:configRequest")          // CSRF ヘッダ自動付与
	assertContains(t, html, `x-data="{ navOpen: false }"`)
}

func TestApp_TomSelectアセットと422swap設定を加算(t *testing.T) {
	data := AppLayoutData{
		Title:     "アラートルール - 農業IoTシステム",
		UserName:  "テストユーザー",
		CSRFToken: "tok-abc",
		CSSURL:    "/static/css/style.css?v=dev",
	}
	html := render(t, App(data))

	// Tom Select アセット (CDN 2.3.1・モック準拠) を head に加算
	assertContains(t, html, "tom-select@2.3.1/dist/css/tom-select.css")
	assertContains(t, html, "tom-select@2.3.1/dist/js/tom-select.complete.min.js")

	// select.js-tom-select の一括初期化 (対象 select が無いページでは no-op)
	assertContains(t, html, "select.js-tom-select")
	assertContains(t, html, "new TomSelect")

	// 422 をスワップ対象に含める responseHandling 設定 (インライン CRUD のバリデーション部分返却用)
	assertContains(t, html, "htmx.config.responseHandling")
	assertContains(t, html, `{code: "422", swap: true}`)

	// [45].. には error: true を付与する (全置換で htmx 既定の error: true を書き落とすと
	// htmx:responseError が発火せず §14 のエラートーストが死にコード化するため)
	assertContains(t, html, `{code: "[45]..", swap: false, error: true}`)

	// §14 グローバルエラー通知トースト + htmx:responseError/sendError ハンドラ。
	// error: true とセットで機能する。401 セッションタイムアウト分岐 (/login 遷移) を含む。
	assertContains(t, html, `id="global-error-toast"`)
	assertContains(t, html, "htmx:responseError")
	assertContains(t, html, "htmx:sendError")
	assertContains(t, html, "__sessionExpiredHandled")

	// 既存の CSRF 機構 (meta + htmx:configRequest) は不変
	assertContains(t, html, "htmx:configRequest")
	assertContains(t, html, `name="csrf-token"`)
}

func TestApp_EChartsアセットをheadで1回読込(t *testing.T) {
	data := AppLayoutData{
		Title:     "デバイス詳細 - 農業IoTシステム",
		UserName:  "テストユーザー",
		CSRFToken: "tok-1",
		CSSURL:    "/static/css/style.css?v=dev",
	}
	html := render(t, App(data))

	// self-host した echarts.min.js を <head> で読み込む (外部 CDN ではなく自サーバ・R5.1)
	assertContains(t, html, "/static/js/echarts.min.js")
	// 1回だけ読み込む (R5.2・期間切替フラグメントには出さない=再 DL させない R5.3)
	if got := strings.Count(html, "/static/js/echarts.min.js"); got != 1 {
		t.Errorf("echarts.min.js の読込回数 = %d, want 1", got)
	}
	// <head> 内に置かれている (body より前)
	head := html[:strings.Index(html, "</head>")]
	if !strings.Contains(head, "/static/js/echarts.min.js") {
		t.Errorf("echarts.min.js が <head> 内に無い:\n%s", head)
	}
}

func TestApp_EChartsグローバル初期化スクリプトを同梱(t *testing.T) {
	data := AppLayoutData{
		Title:     "デバイス詳細 - 農業IoTシステム",
		UserName:  "テストユーザー",
		CSRFToken: "tok-2",
		CSSURL:    "/static/css/style.css?v=dev",
	}
	html := render(t, App(data))

	// init/dispose/connect/endLabel/sampling をグローバルに集約 (旧 linkedCharts の置換)。
	// [data-echarts] コンテナを走査して描画 (id/data-* で初期化・R2.3/3.3/6/7)。
	for _, marker := range []string{
		"[data-echarts]",   // 初期化対象セレクタ
		"echarts.init",     // 描画インスタンス生成
		"getInstanceByDom", // 既存インスタンス検出 (再描画・リーク防止 R6)
		"dispose",          // 破棄 (R6)
		"echarts.connect",  // 温湿度2インスタンス連動 (R3.3)
		"DOMContentLoaded", // 初回読込時の初期化 (R8.1)
		"htmx:afterSwap",   // 期間切替フラグメント swap 後の初期化
		"htmx:beforeSwap",  // swap 前の破棄 (リーク防止 R6)
		"endLabel",         // 右端の現在値 (R2.3)
		"data-unit",        // endLabel formatter 用の単位
		"lttb",             // 30日相当のダウンサンプリング (R7.1)
	} {
		assertContains(t, html, marker)
	}
}

// 正常帯は「帯下限(透明な積み上げ基線・凡例非表示)」+「帯幅」の2系列で描く。tooltip は
// trigger:axis のため帯下限まで拾い、既定オフでもホバー時に生値が漏れてクラッタになる。
// EChartsInitializer が tooltip.formatter で帯下限を除外し、数値を2桁へ丸めることを固定する。
func TestApp_tooltipは帯下限を除外し数値を丸める(t *testing.T) {
	data := AppLayoutData{
		Title:     "デバイス詳細 - 農業IoTシステム",
		UserName:  "テストユーザー",
		CSRFToken: "tok-tip",
		CSSURL:    "/static/css/style.css?v=dev",
	}
	html := render(t, App(data))

	for _, marker := range []string{
		"option.tooltip",       // tooltip を client 側で後加工する
		"formatter",            // tooltip.formatter を付与
		"seriesName === '帯下限'", // 透明ヘルパ系列を除外する判定
		"toFixed(2)",           // 生値の長い float を小数2桁へ丸める
	} {
		assertContains(t, html, marker)
	}
}

func TestApp_旧linkedChartsを撤去(t *testing.T) {
	data := AppLayoutData{
		Title:     "ダッシュボード - 農業IoTシステム",
		UserName:  "テストユーザー",
		CSRFToken: "tok-3",
		CSSURL:    "/static/css/style.css?v=dev",
	}
	html := render(t, App(data))

	// 旧 Alpine ベースの自作ホバー連動 (linkedCharts 関数) は ECharts へ移行済みで撤去。
	// 関数定義の不在を検証する (移行を説明するコメント中の言及は許容)。
	if strings.Contains(html, "function linkedCharts") {
		t.Errorf("旧 linkedCharts 関数が App に残存している (ECharts へ移行済み):\n%s", html)
	}
	// Alpine 自作ホバー連動の DOM フック (x-data linkedCharts 呼出) も残っていないこと
	if strings.Contains(html, "linkedCharts(") {
		t.Errorf("linkedCharts() の呼出/定義が残存している:\n%s", html)
	}
}

func TestGuest_カードでchildrenを描画(t *testing.T) {
	var buf bytes.Buffer
	ctx := templ.WithChildren(context.Background(), templ.Raw("<p>子要素</p>"))
	if err := Guest("ログイン - 農業IoTシステム", "/static/css/style.css?v=dev").Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()

	assertContains(t, html, "guest-layout")
	assertContains(t, html, "guest-card")
	assertContains(t, html, "/static/css/style.css?v=dev")
	assertContains(t, html, "<title>ログイン - 農業IoTシステム</title>")
	assertContains(t, html, "子要素")
}
