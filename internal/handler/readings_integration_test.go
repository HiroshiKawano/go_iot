package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/gin-gonic/gin"
)

// readings_integration_test.go は readings 画面 (Index) への集計帳票・CSV リンク結線 (4.1) と
// 項目フィルタ UI の echo (4.2)、CSV 経路の共存 (4.3) を httptest+gin で検証する。
// 既存 S6 表示 (集計ボックス/一覧/ページャ) の無回帰は既存テスト群が担保する。

// goyaDeviceReadingsRepo は作物=ゴーヤのデバイス1を所有する fake を返す (帳票の作物ラベル検証用)。
func goyaDeviceReadingsRepo() *fakeReadingsRepo {
	repo := ownerReadingsRepo()
	crop := "goya"
	d := repo.devices[1]
	d.Crop = &crop
	repo.devices[1] = d
	return repo
}

// --- 4.1 帳票と CSV リンクを画面ハンドラへ結線 (R4.1/R5.1/R7) ---

func TestReadingsIndex_帳票とCSVリンクをフラグメントへ結線する(t *testing.T) {
	repo := goyaDeviceReadingsRepo()
	repo.countVal = 2
	repo.summaryRow = fullSummaryRow()
	repo.listRows = []repository.SensorReading{
		historyRow(time.Date(2026, 4, 20, 5, 30, 0, 0, time.UTC), time.Date(2026, 4, 20, 5, 30, 2, 0, time.UTC), 28.50, 65.30),
	}
	// 全行取得 (帳票・CSV の共通入力)。同一 JST 日に2点。
	repo.rangeRows = []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 28.50, 65.30),
		sensorRow(1, time.Date(2026, 4, 20, 3, 5, 0, 0, time.UTC), 28.60, 65.10),
	}
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1/readings?from=2026-04-13&to=2026-04-20")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()

	// 集計帳票 (日次/時間別) が結果領域に描画される (作物=ゴーヤの適正帯)。
	assertHistoryBodyHas(t, body,
		"集計帳票（日次）", "集計帳票（時間別）", "適正帯滞在率", "ゴーヤ", "0.40〜1.20 kPa",
	)
	// CSV ダウンロードボタンと href (適用済み from/to を保持・HTMX 非対象)。
	assertHistoryBodyHas(t, body, "CSV ダウンロード", "readings.csv?from=2026-04-13")
	if !strings.Contains(body, `hx-boost="false"`) {
		t.Errorf("CSV ボタンが HTMX 非対象 (hx-boost=false) でない:\n%s", body)
	}

	// 全行取得が呼ばれ、区間が一覧/集計/件数と一致する (R7・単一区間源)。
	if !repo.rangeCalled {
		t.Fatal("ListSensorReadingsInRange が呼ばれていない (帳票/CSV 未結線)")
	}
	if !repo.lastRange.RecordedAt.Time.Equal(repo.lastList.RecordedAt.Time) ||
		!repo.lastRange.RecordedAt_2.Time.Equal(repo.lastList.RecordedAt_2.Time) {
		t.Errorf("全行取得の区間=(%v,%v) が一覧区間=(%v,%v) と不一致 (R7)",
			repo.lastRange.RecordedAt.Time, repo.lastRange.RecordedAt_2.Time,
			repo.lastList.RecordedAt.Time, repo.lastList.RecordedAt_2.Time)
	}
	if !repo.lastRange.RecordedAt.Time.Equal(repo.lastSummary.RecordedAt.Time) {
		t.Errorf("全行取得の from=%v が集計 from=%v と不一致", repo.lastRange.RecordedAt.Time, repo.lastSummary.RecordedAt.Time)
	}
}

func TestReadingsIndex_HTMX経路でも帳票とCSVリンクを返す(t *testing.T) {
	repo := goyaDeviceReadingsRepo()
	repo.countVal = 2
	repo.summaryRow = fullSummaryRow()
	repo.rangeRows = []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 28.50, 65.30),
	}
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)

	w := requestWithHeaders(r, http.MethodGet, "/devices/1/readings?from=2026-04-13&to=2026-04-20",
		map[string]string{"HX-Request": "true"})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	// フラグメントのみ (レイアウトなし) に帳票表と CSV ボタンが含まれる。
	if strings.Contains(body, "<html") {
		t.Errorf("HTMX 応答にレイアウトが含まれている")
	}
	assertHistoryBodyHas(t, body, "集計帳票（日次）", "CSV ダウンロード", "readings.csv?from=2026-04-13")
}

// 既存 S6 表示 (集計ボックス/一覧/ページャ) が帳票結線後も無回帰で維持される。
func TestReadingsIndex_帳票結線後もS6表示が無回帰(t *testing.T) {
	repo := goyaDeviceReadingsRepo()
	repo.countVal = 5
	repo.summaryRow = fullSummaryRow()
	repo.listRows = []repository.SensorReading{
		historyRow(time.Date(2026, 4, 20, 5, 30, 0, 0, time.UTC), time.Date(2026, 4, 20, 5, 30, 2, 0, time.UTC), 28.50, 65.30),
	}
	repo.rangeRows = []repository.SensorReading{sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 28.50, 65.30)}
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)

	body := getPath(r, "/devices/1/readings").Body.String()
	// S6: 集計ボックス・一覧・通信遅延が従来どおり。
	assertHistoryBodyHas(t, body, "summary-grid", "28.30℃", "62.50%", "通信遅延", "2026-04-20 14:30", "2秒")
}

// --- 4.2 項目フィルタ UI の結線 (R1.4/R1.5) ---

func TestReadingsIndex_項目フィルタの選択をフォームとCSVリンクへ反映(t *testing.T) {
	t.Run("温度のみ選択は温度checkedと items=temperature", func(t *testing.T) {
		repo := goyaDeviceReadingsRepo()
		repo.rangeRows = []repository.SensorReading{sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 28.5, 65.3)}
		r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)

		body := getPath(r, "/devices/1/readings?items=temperature").Body.String()
		// フォームの温度 checkbox は checked、湿度は未 checked (適用状態を echo)。
		if !strings.Contains(body, `value="temperature" checked`) {
			t.Errorf("温度 checkbox が checked でない:\n%s", body)
		}
		if strings.Contains(body, `value="humidity" checked`) {
			t.Errorf("湿度のみ未選択なのに湿度 checkbox が checked")
		}
		// CSV リンクの items に温度のみが反映される。
		assertHistoryBodyHas(t, body, "items=temperature")
		if strings.Contains(body, "items=humidity") {
			t.Errorf("温度のみ選択なのに CSV リンクに items=humidity がある")
		}
	})

	t.Run("未選択は両方checkedと items両方 (既定)", func(t *testing.T) {
		repo := goyaDeviceReadingsRepo()
		repo.rangeRows = []repository.SensorReading{sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 28.5, 65.3)}
		r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)

		body := getPath(r, "/devices/1/readings").Body.String()
		if !strings.Contains(body, `value="temperature" checked`) || !strings.Contains(body, `value="humidity" checked`) {
			t.Errorf("未選択 (既定) で両方 checked でない:\n%s", body)
		}
		assertHistoryBodyHas(t, body, "items=temperature", "items=humidity")
	})
}

// --- 4.3 CSV 経路のルート登録 (既存 readings と共存・GET 子経路) ---

// readings と readings.csv は :device 配下の静的兄弟セグメントとして共存できる
// (gin の route tree で競合しない・design Open Question の検証)。
func TestRoutes_readingsとreadingsCSVが共存する(t *testing.T) {
	repo := ownerReadingsRepo()
	repo.summaryRow = fullSummaryRow()
	h := &ReadingsHandler{Repo: repo}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	withUser := func(c *gin.Context) { auth.SetUserID(c, 7); c.Next() }
	// main.go と同じ並びで両経路を登録 (登録時に panic しないこと自体が共存の証左)。
	r.GET("/devices/:device/readings", withUser, h.Index)
	r.GET("/devices/:device/readings.csv", withUser, h.Export)

	// 画面経路は HTML、CSV 経路は text/csv を返し、別ハンドラへ解決される。
	wPage := getPath(r, "/devices/1/readings")
	if wPage.Code != http.StatusOK || !strings.Contains(wPage.Body.String(), "<html") {
		t.Errorf("画面経路 = %d (HTML?), want 200 HTML", wPage.Code)
	}
	wCSV := getPath(r, "/devices/1/readings.csv")
	if wCSV.Code != http.StatusOK {
		t.Fatalf("CSV 経路 = %d, want 200", wCSV.Code)
	}
	if ct := wCSV.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Errorf("CSV 経路 Content-Type = %q, want text/csv; charset=utf-8", ct)
	}
}
