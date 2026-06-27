package handler

import (
	"bytes"
	"encoding/csv"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/gin-gonic/gin"
)

// readings_export_handler_test.go は CSV エクスポートハンドラ (ReadingsHandler.Export) を
// httptest+gin で検証する (テストガイダンス集 §33.1/§3.5/§6)。200/400/404/500・項目フィルタ・
// 空期間・大量行・非所有の DB 副作用なしを網羅する。

// newExportRouterWithUser は認証済み (uid) で CSV 経路を配線したルータを返す。
func newExportRouterWithUser(h *ReadingsHandler, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	withUser := func(c *gin.Context) { auth.SetUserID(c, userID); c.Next() }
	r.GET("/devices/:device/readings.csv", withUser, h.Export)
	return r
}

// parseCSVBody は応答ボディの BOM を除去し csv パースして全レコードを返す。
func parseCSVBody(t *testing.T, body []byte) [][]string {
	t.Helper()
	recs, err := csv.NewReader(bytes.NewReader(stripBOM(body))).ReadAll()
	if err != nil {
		t.Fatalf("CSV パース失敗: %v", err)
	}
	return recs
}

// --- 3.2 正常 (200・ヘッダ・BOM・全行) (R1.1/R1.2/R3.3) ---

func TestExport_正常は200で添付CSVと全行(t *testing.T) {
	repo := ownerReadingsRepo()
	repo.rangeRows = []repository.SensorReading{
		exportRow(time.Date(2026, 4, 20, 5, 30, 0, 0, time.UTC), 28.50, 65.30),
		exportRow(time.Date(2026, 4, 20, 5, 35, 0, 0, time.UTC), 28.60, 65.10),
	}
	h := &ReadingsHandler{Repo: repo}
	r := newExportRouterWithUser(h, 7)

	w := requestWithHeaders(r, http.MethodGet, "/devices/1/readings.csv?from=2026-04-13&to=2026-04-20", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/csv; charset=utf-8", ct)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") || !strings.Contains(cd, "filename*=UTF-8''") || !strings.Contains(cd, ".csv") {
		t.Errorf("Content-Disposition = %q, want attachment + filename* + .csv", cd)
	}
	body := w.Body.Bytes()
	if !bytes.HasPrefix(body, utf8BOM) {
		t.Error("先頭に UTF-8 BOM が無い")
	}
	recs := parseCSVBody(t, body)
	if len(recs) != 3 { // ヘッダ + 2 行
		t.Fatalf("行数 = %d, want 3 (header+2)", len(recs))
	}
	// メタ列 + 計測値が出る (デバイス名・未設定地点/作物・温湿度)。
	assertHistoryBodyHas(t, w.Body.String(), "ハウスA温湿度計", "未設定", "28.50", "65.30")
	// 区間境界は parseDateBounds 共有 (画面一覧と同一・R7)。
	if !repo.rangeCalled || repo.lastRange.DeviceID != 1 {
		t.Errorf("ListSensorReadingsInRange 未呼出 or device 不一致: called=%v arg=%+v", repo.rangeCalled, repo.lastRange)
	}
}

// --- 3.2 項目フィルタ (温度のみで湿度列なし) (R1.4) ---

func TestExport_項目フィルタ温度のみは湿度列を出さない(t *testing.T) {
	repo := ownerReadingsRepo()
	repo.rangeRows = []repository.SensorReading{exportRow(time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), 28.50, 65.30)}
	h := &ReadingsHandler{Repo: repo}
	r := newExportRouterWithUser(h, 7)

	w := requestWithHeaders(r, http.MethodGet, "/devices/1/readings.csv?items=temperature", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	recs := parseCSVBody(t, w.Body.Bytes())
	header := strings.Join(recs[0], ",")
	if !strings.Contains(header, "温度(℃)") || strings.Contains(header, "湿度(%)") {
		t.Errorf("ヘッダ = %v, want 温度ありで湿度なし", recs[0])
	}
}

// --- 3.2 空期間はヘッダのみ (R1.6) ---

func TestExport_空期間はヘッダのみで200(t *testing.T) {
	repo := ownerReadingsRepo()
	repo.rangeRows = nil // 期間内データなし
	h := &ReadingsHandler{Repo: repo}
	r := newExportRouterWithUser(h, 7)

	w := requestWithHeaders(r, http.MethodGet, "/devices/1/readings.csv", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (空でもエラーにしない)", w.Code)
	}
	recs := parseCSVBody(t, w.Body.Bytes())
	if len(recs) != 1 {
		t.Errorf("行数 = %d, want 1 (ヘッダのみ)", len(recs))
	}
}

// --- 3.2 形式不正は400・データ無出力 (R1.7) ---

func TestExport_日付形式不正は400でクエリ未実行(t *testing.T) {
	repo := ownerReadingsRepo()
	h := &ReadingsHandler{Repo: repo}
	r := newExportRouterWithUser(h, 7)

	w := requestWithHeaders(r, http.MethodGet, "/devices/1/readings.csv?from=invalid", nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if repo.rangeCalled {
		t.Error("形式不正なのに全行取得クエリが呼ばれた (データ無出力に反する)")
	}
	if bytes.HasPrefix(w.Body.Bytes(), utf8BOM) {
		t.Error("形式不正で CSV ボディを返している")
	}
}

// --- 3.2 認可 (非所有/不在→404・列挙防止・副作用なし) (R8.1) ---

func TestExport_非所有デバイスは404で副作用なし(t *testing.T) {
	repo := ownerReadingsRepo()
	repo.devices[2] = repository.Device{ID: 2, UserID: 999, Name: "他人のデバイス"} // 別ユーザー所有
	h := &ReadingsHandler{Repo: repo}
	r := newExportRouterWithUser(h, 7)

	w := requestWithHeaders(r, http.MethodGet, "/devices/2/readings.csv", nil)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (列挙防止)", w.Code)
	}
	if repo.rangeCalled {
		t.Error("非所有なのに全行取得クエリが呼ばれた (副作用)")
	}
}

func TestExport_不在デバイスは404(t *testing.T) {
	repo := ownerReadingsRepo()
	h := &ReadingsHandler{Repo: repo}
	r := newExportRouterWithUser(h, 7)

	w := requestWithHeaders(r, http.MethodGet, "/devices/999/readings.csv", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestExport_非数値IDは400(t *testing.T) {
	repo := ownerReadingsRepo()
	h := &ReadingsHandler{Repo: repo}
	r := newExportRouterWithUser(h, 7)

	w := requestWithHeaders(r, http.MethodGet, "/devices/abc/readings.csv", nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// --- 3.2 DB エラーは500 (R: System Errors) ---

func TestExport_全行取得のDBエラーは500(t *testing.T) {
	repo := ownerReadingsRepo()
	repo.rangeErr = errors.New("db down")
	h := &ReadingsHandler{Repo: repo}
	r := newExportRouterWithUser(h, 7)

	w := requestWithHeaders(r, http.MethodGet, "/devices/1/readings.csv", nil)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// --- 3.2 大量行でも全行出力 (R9) ---

func TestExport_大量行でも全行を出力する(t *testing.T) {
	repo := ownerReadingsRepo()
	const n = 500
	rows := make([]repository.SensorReading, n)
	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	for i := range rows {
		rows[i] = exportRow(base.Add(time.Duration(i)*time.Minute), 20.00, 50.00)
	}
	repo.rangeRows = rows
	h := &ReadingsHandler{Repo: repo}
	r := newExportRouterWithUser(h, 7)

	w := requestWithHeaders(r, http.MethodGet, "/devices/1/readings.csv?page=99&per_page=10", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	recs := parseCSVBody(t, w.Body.Bytes())
	if len(recs) != n+1 { // ヘッダ + n 行 (ページングに影響されない)
		t.Errorf("行数 = %d, want %d (header + %d)", len(recs), n+1, n)
	}
}

// 未認証 (302→/login) は Export ハンドラの外側 (RequireAuth ミドルウェア) の責務であり
// require_auth_test.go で担保済み・経路登録は 4.3。本ファイルは認証済み前提のハンドラ挙動に集中する。
