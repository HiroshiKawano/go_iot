package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/chart"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// seasonal_trend_handler_test.go は統計分析ページ（GET /analysis/trend）の HTTP 境界を
// Querier 手書きモック + httptest で DB 非依存に検証する（タスク 6.1/6.2）。
// 認可（自己所有=表示・非所有/不在=404 列挙防止）・未選択/データ無の縮退・一次/厳密判定ラベル・
// 検出力留保・HX 部分返却を網羅する。

// fakeTrendRepo は SeasonalTrendRepo の手書きモック（DB 非依存）。
type fakeTrendRepo struct {
	users      map[int64]repository.User
	devices    map[int64]repository.Device   // GetDevice 用（UserID で所有判定）
	byUser     map[int64][]repository.Device // ListDevicesByUser 用
	jstRows    []repository.ListDailySensorAggregatesJSTRow
	jstErr     error
	getUserErr error
	listDevErr error
}

func (f *fakeTrendRepo) GetUser(_ context.Context, id int64) (repository.User, error) {
	if f.getUserErr != nil {
		return repository.User{}, f.getUserErr
	}
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return repository.User{}, pgx.ErrNoRows
}

func (f *fakeTrendRepo) ListDevicesByUser(_ context.Context, uid int64) ([]repository.Device, error) {
	if f.listDevErr != nil {
		return nil, f.listDevErr
	}
	return f.byUser[uid], nil
}

func (f *fakeTrendRepo) GetDevice(_ context.Context, id int64) (repository.Device, error) {
	if d, ok := f.devices[id]; ok {
		return d, nil
	}
	return repository.Device{}, pgx.ErrNoRows
}

func (f *fakeTrendRepo) ListDailySensorAggregatesJST(_ context.Context, _ repository.ListDailySensorAggregatesJSTParams) ([]repository.ListDailySensorAggregatesJSTRow, error) {
	return f.jstRows, f.jstErr
}

var _ SeasonalTrendRepo = (*fakeTrendRepo)(nil)

// genTrendRows は startYear から years 年分の月次1日次行（緩やかな上昇トレンド）を生成する。
func genTrendRows(startYear, years int) []repository.ListDailySensorAggregatesJSTRow {
	var rows []repository.ListDailySensorAggregatesJSTRow
	i := 0
	for y := 0; y < years; y++ {
		for m := 1; m <= 12; m++ {
			t := 15.0 + 0.15*float64(i) // 温度: 緩やかな上昇
			h := 60.0 + 0.05*float64(i) // 湿度: 緩やかな上昇
			rows = append(rows, jstDailyRow(startYear+y, time.Month(m), 15,
				t, t+5, t-5, h, h+8, h-8, 144))
			i++
		}
	}
	return rows
}

// trendRepo は所有者 uid=7・device1(所有)・device2(他者所有=99) と日次行を備えた fake。
func trendRepo(rows []repository.ListDailySensorAggregatesJSTRow) *fakeTrendRepo {
	dev1 := repository.Device{ID: 1, UserID: 7, Name: "ハウスA温湿度計"}
	dev2 := repository.Device{ID: 2, UserID: 99, Name: "他人のデバイス"}
	return &fakeTrendRepo{
		users:   map[int64]repository.User{7: {ID: 7, Name: "テスト農場主"}},
		devices: map[int64]repository.Device{1: dev1, 2: dev2},
		byUser:  map[int64][]repository.Device{7: {dev1}},
		jstRows: rows,
	}
}

func newTrendRouterWithUser(h *SeasonalTrendHandler, uid int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	withUser := func(c *gin.Context) { auth.SetUserID(c, uid); c.Next() }
	r.GET("/analysis/trend", withUser, h.Show)
	return r
}

func doTrendGET(t *testing.T, h *SeasonalTrendHandler, uid int64, target string, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	w := httptest.NewRecorder()
	newTrendRouterWithUser(h, uid).ServeHTTP(w, req)
	return w
}

// ---- 6.1 自己所有 device でフルページにトレンド表示 ----------------------------

func TestSeasonalTrendShow_自己所有でフルページ表示(t *testing.T) {
	h := &SeasonalTrendHandler{Repo: trendRepo(genTrendRows(2024, 3))}
	w := doTrendGET(t, h, 7, "/analysis/trend?device_id=1&granularity=monthly", false)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	assertContains(t, body, "<html")              // フルページ
	assertContains(t, body, "統計分析")               // 見出し
	assertContains(t, body, `id="trend-section"`) // 部分更新ターゲット
	assertContains(t, body, "トレンド判定")             // 判定表
	assertContains(t, body, "data-echarts")       // チャート器
	assertContains(t, body, `id="trend-chart"`)
	assertContains(t, body, "ロールアップ統計サマリ（温度）") // サマリ表
	// サイドバーで統計分析が active。
	assertContains(t, body, `href="/analysis/trend" class="active"`)
}

// ---- 6.1 認可: 非所有/不在は 404（列挙防止）------------------------------------

func TestSeasonalTrendShow_非所有は404(t *testing.T) {
	h := &SeasonalTrendHandler{Repo: trendRepo(genTrendRows(2024, 3))}
	w := doTrendGET(t, h, 7, "/analysis/trend?device_id=2", false) // device2 は uid=99 所有
	if w.Code != http.StatusNotFound {
		t.Fatalf("非所有 status = %d, want 404（列挙防止）", w.Code)
	}
}

func TestSeasonalTrendShow_不在は404(t *testing.T) {
	h := &SeasonalTrendHandler{Repo: trendRepo(genTrendRows(2024, 3))}
	w := doTrendGET(t, h, 7, "/analysis/trend?device_id=999", false)
	if w.Code != http.StatusNotFound {
		t.Fatalf("不在 status = %d, want 404", w.Code)
	}
}

// ---- 6.1 未選択は空セクション案内（断定しない）--------------------------------

func TestSeasonalTrendShow_未選択は空セクション(t *testing.T) {
	h := &SeasonalTrendHandler{Repo: trendRepo(genTrendRows(2024, 3))}
	w := doTrendGET(t, h, 7, "/analysis/trend", false) // device_id 省略
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	assertContains(t, body, `id="trend-section"`)
	assertContains(t, body, "対象のデバイスを選択") // 案内
	// 未選択ではトレンドチャートを描かない。
	if strings.Contains(body, `id="trend-chart"`) {
		t.Errorf("未選択で #trend-chart が描画されている（縮退すべき）")
	}
}

// ---- 6.1 データ無は案内（断定しない）------------------------------------------

func TestSeasonalTrendShow_データ無は案内(t *testing.T) {
	h := &SeasonalTrendHandler{Repo: trendRepo(nil)} // 所有 device だが日次行なし
	w := doTrendGET(t, h, 7, "/analysis/trend?device_id=1", false)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	assertContains(t, body, "計測データがありません")
	if strings.Contains(body, `id="trend-chart"`) {
		t.Errorf("データ無で #trend-chart が描画されている（縮退すべき）")
	}
}

// ---- 6.2 一次/補正済みラベルの段階表示 ----------------------------------------

func TestSeasonalTrendShow_一次と補正済みのラベルを表示(t *testing.T) {
	h := &SeasonalTrendHandler{Repo: trendRepo(genTrendRows(2024, 3))}
	w := doTrendGET(t, h, 7, "/analysis/trend?device_id=1&granularity=monthly", false)
	body := w.Body.String()
	assertContains(t, body, "一次判定（多重比較未補正）")
	assertContains(t, body, "補正済み（Hamed-Rao）")
	// Sen の傾きは単位/年で提示。
	assertContains(t, body, "℃/年")
	// 信号色バッジが少なくとも1つ出る（up/down/flat いずれか）。
	if !strings.Contains(body, "badge-trend-up") &&
		!strings.Contains(body, "badge-trend-down") &&
		!strings.Contains(body, "badge-trend-flat") {
		t.Errorf("信号色バッジが描画されていない")
	}
}

// ---- 6.2 年数不足で検出力留保＋平年比非表示注記 ------------------------------

func TestSeasonalTrendShow_年数不足で留保注記(t *testing.T) {
	h := &SeasonalTrendHandler{Repo: trendRepo(genTrendRows(2024, 2))} // 2年=span<3
	w := doTrendGET(t, h, 7, "/analysis/trend?device_id=1&granularity=monthly", false)
	body := w.Body.String()
	// 検出力留保（非有意≠トレンド無し）。
	assertContains(t, body, "トレンド無し")
	// 平年比は年<3 で非表示＋基準期間依存・不確実の注記。
	assertContains(t, body, "3年未満")
}

// ---- 6.2 HX-Request は #trend-section を部分返却 ------------------------------

func TestSeasonalTrendShow_HX部分返却(t *testing.T) {
	h := &SeasonalTrendHandler{Repo: trendRepo(genTrendRows(2024, 3))}
	w := doTrendGET(t, h, 7, "/analysis/trend?device_id=1&granularity=monthly", true)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	// 部分返却ゆえフルページ（<html>）ではなくセクションを返す。
	if strings.Contains(body, "<html") {
		t.Errorf("HX-Request でフルページが返っている（部分返却すべき）")
	}
	assertContains(t, body, `id="trend-section"`)
	assertContains(t, body, "トレンド判定")
}

// 不正な granularity は既定 monthly にフォールバックし 200（断定しない）。
func TestSeasonalTrendShow_不正粒度は既定へフォールバック(t *testing.T) {
	h := &SeasonalTrendHandler{Repo: trendRepo(genTrendRows(2024, 3))}
	w := doTrendGET(t, h, 7, "/analysis/trend?device_id=1&granularity=weekly", false)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200（既定フォールバック）", w.Code)
	}
	assertContains(t, w.Body.String(), `id="trend-section"`)
}

// ---- 異常系: DB 失敗は 500（fail-closed・機密非漏洩）------------------------

func TestSeasonalTrendShow_デバイス一覧取得失敗は500(t *testing.T) {
	repo := trendRepo(genTrendRows(2024, 3))
	repo.listDevErr = pgx.ErrTxClosed // 任意の DB エラー
	h := &SeasonalTrendHandler{Repo: repo}
	w := doTrendGET(t, h, 7, "/analysis/trend?device_id=1", false)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestSeasonalTrendShow_JST集計取得失敗は500(t *testing.T) {
	repo := trendRepo(genTrendRows(2024, 3))
	repo.jstErr = pgx.ErrTxClosed
	h := &SeasonalTrendHandler{Repo: repo}
	w := doTrendGET(t, h, 7, "/analysis/trend?device_id=1", false)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestCVString_未定義はダッシュ(t *testing.T) {
	if got := cvString(rollupStat{HasCV: false}); got != statEmptyMark {
		t.Errorf("CV 未定義 = %q, want %q", got, statEmptyMark)
	}
	if got := cvString(rollupStat{CV: 0.16, HasCV: true}); got != "0.16" {
		t.Errorf("CV = %q, want 0.16", got)
	}
}

// ---- 判定分類・整形ヘルパの単体（検出力充足時の有意/非有意分岐を直接被覆）----------

func TestClassifyTrend(t *testing.T) {
	cases := []struct {
		name           string
		mk             chart.MKResult
		caution        bool
		wantDir, wantV string
	}{
		{"有意な上昇", chart.MKResult{Z: 3, PValue: 0.001}, false, "up", "有意な上昇"},
		{"有意な下降", chart.MKResult{Z: -3, PValue: 0.001}, false, "down", "有意な下降"},
		{"非有意", chart.MKResult{Z: 0.5, PValue: 0.4}, false, "flat", "非有意"},
		{"検出力不足は判定保留", chart.MKResult{Z: 3, PValue: 0.001}, true, "flat", "判定保留"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, v := classifyTrend(tc.mk, tc.caution)
			if dir != tc.wantDir || v != tc.wantV {
				t.Errorf("classifyTrend = (%q,%q), want (%q,%q)", dir, v, tc.wantDir, tc.wantV)
			}
		})
	}
}

func TestClassifyStrict(t *testing.T) {
	// BH 棄却＋Z>0＝有意な上昇、棄却されなければ非有意、検出力不足は判定保留。
	if dir, v := classifyStrict(chart.MKResult{Z: 3}, true, false); dir != "up" || v != "有意な上昇" {
		t.Errorf("棄却+Z>0 = (%q,%q), want (up,有意な上昇)", dir, v)
	}
	if dir, v := classifyStrict(chart.MKResult{Z: -3}, true, false); dir != "down" || v != "有意な下降" {
		t.Errorf("棄却+Z<0 = (%q,%q), want (down,有意な下降)", dir, v)
	}
	if dir, _ := classifyStrict(chart.MKResult{Z: 3}, false, false); dir != "flat" {
		t.Errorf("非棄却 = %q, want flat（非有意）", dir)
	}
	if _, v := classifyStrict(chart.MKResult{Z: 3}, true, true); v != "判定保留" {
		t.Errorf("検出力不足 = %q, want 判定保留", v)
	}
}

func TestChartRound2(t *testing.T) {
	// チャート表示系列は2桁に丸める（endLabel/markPoint が長い小数を出さない・実機スモークで発見）。
	got := chartRound2([]float64{26.793103448275862, 27.0, -2.49999, 60.526})
	want := []float64{26.79, 27.0, -2.5, 60.53}
	if len(got) != len(want) {
		t.Fatalf("len=%d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("chartRound2[%d] = %v, want %v", i, got[i], want[i])
		}
	}
	// nil/空は安全に空を返す。
	if chartRound2(nil) != nil {
		t.Errorf("nil 入力は nil を返すべき")
	}
}

// 実機スモーク回帰ガード: ロールアップ平均が長い小数のとき、チャート option に丸めた値が出て
// 生の長小数（例 26.793103）が漏れないこと。
func TestBuildTrendSection_チャート値は丸められる(t *testing.T) {
	// 平均が割り切れない長小数になるデータ（3点の平均 = 26.79310...）を1バケットに作る。
	rows := []repository.ListDailySensorAggregatesJSTRow{
		jstDailyRow(2024, time.January, 10, 26.79, 30, 23, 60, 70, 50, 100),
		jstDailyRow(2024, time.January, 11, 26.80, 30, 23, 60, 70, 50, 100),
		jstDailyRow(2024, time.January, 12, 26.79, 30, 23, 60, 70, 50, 100),
		jstDailyRow(2024, time.February, 10, 25.0, 29, 21, 58, 68, 48, 100),
	}
	section, err := buildTrendSection(rows, granularityMonthly)
	if err != nil {
		t.Fatalf("buildTrendSection: %v", err)
	}
	// 生の長小数（4桁以上）がチャート option に漏れていない（丸め済み）。
	if strings.Contains(section.TrendOptionJSON, "26.7931") || strings.Contains(section.TrendOptionJSON, ".793103") {
		t.Errorf("チャート option に生の長小数が漏れている:\n%s", section.TrendOptionJSON)
	}
}

func TestFormatSlopeAndP(t *testing.T) {
	if got := formatSlopePerYear(0.03, "℃"); got != "+0.03 ℃/年" {
		t.Errorf("formatSlopePerYear(+) = %q, want +0.03 ℃/年", got)
	}
	if got := formatSlopePerYear(-0.15, "%"); got != "-0.15 %/年" {
		t.Errorf("formatSlopePerYear(-) = %q, want -0.15 %%/年", got)
	}
	if got := formatTrendP(0.0001); got != "<0.001" {
		t.Errorf("formatTrendP(極小) = %q, want <0.001", got)
	}
	if got := formatTrendP(0.012); got != "0.012" {
		t.Errorf("formatTrendP = %q, want 0.012", got)
	}
}

// assertContains は出力に部分文字列が含まれることを検証する（handler テスト共通の最小ヘルパ）。
func assertContains(t *testing.T, body, substr string) {
	t.Helper()
	if !strings.Contains(body, substr) {
		t.Errorf("出力に %q が含まれていない:\n%s", substr, body)
	}
}
