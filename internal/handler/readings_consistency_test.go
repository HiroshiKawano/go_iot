package handler

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// readings_consistency_test.go は CSV エクスポート・集計帳票・既存一覧/集計ボックスが
// 同一の期間フィルタ (from/to) を共有し、同一区間に基づく値を返すことを検証する (R7.1/7.2/7.3)。
// 期間境界写像の単一源 (parseDateBounds) が画面経路 (Index) と CSV 経路 (Export) で一致することを、
// 代表的な from/to (期間指定・未指定の全期間・終了日を当日いっぱい含む JST 暦日基準) で確認する。

// consistencyRows は同一 JST 日 (2026-04-20) の 3 計測行を返す (CSV 行数=帳票バケット入力=同一)。
func consistencyRows() []repository.SensorReading {
	return []repository.SensorReading{
		sensorRow(1, time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC), 28.50, 65.30),
		sensorRow(1, time.Date(2026, 4, 20, 3, 5, 0, 0, time.UTC), 28.60, 65.10),
		sensorRow(1, time.Date(2026, 4, 20, 3, 10, 0, 0, time.UTC), 28.40, 64.90),
	}
}

// assertIndexIntervalShared は Index 内の Count/Paginated/Summary/InRange が同一区間を使うことを検証する。
func assertIndexIntervalShared(t *testing.T, repo *fakeReadingsRepo) {
	t.Helper()
	from := repo.lastRange.RecordedAt.Time
	to := repo.lastRange.RecordedAt_2.Time
	if !repo.lastCount.RecordedAt.Time.Equal(from) || !repo.lastCount.RecordedAt_2.Time.Equal(to) {
		t.Errorf("Count 区間が全行区間と不一致: count=(%v,%v) range=(%v,%v)",
			repo.lastCount.RecordedAt.Time, repo.lastCount.RecordedAt_2.Time, from, to)
	}
	if !repo.lastList.RecordedAt.Time.Equal(from) || !repo.lastList.RecordedAt_2.Time.Equal(to) {
		t.Errorf("Paginated 区間が全行区間と不一致")
	}
	if !repo.lastSummary.RecordedAt.Time.Equal(from) || !repo.lastSummary.RecordedAt_2.Time.Equal(to) {
		t.Errorf("Summary 区間が全行区間と不一致")
	}
}

func TestConsistency_CSVと帳票と一覧が同一区間に基づく(t *testing.T) {
	cases := []struct {
		name     string
		from, to string
		wantFrom time.Time // 期待する区間下限 (JST)
		// 上限は end-of-day / センチネルを個別に検証する
		checkTo func(t *testing.T, to time.Time)
	}{
		{
			name: "期間指定は当日始端〜終了日いっぱい",
			from: "2026-04-13", to: "2026-04-20",
			wantFrom: time.Date(2026, 4, 13, 0, 0, 0, 0, jst),
			checkTo: func(t *testing.T, to time.Time) {
				// 終了日を当日いっぱい含む (2026-04-20 23:59 以降・翌日未満)。
				if to.Before(time.Date(2026, 4, 20, 23, 59, 0, 0, jst)) || !to.Before(time.Date(2026, 4, 21, 0, 0, 0, 0, jst)) {
					t.Errorf("to=%v, want 2026-04-20 end-of-day", to)
				}
			},
		},
		{
			name: "未指定は全期間センチネル",
			from: "", to: "",
			wantFrom: time.Date(1970, 1, 1, 0, 0, 0, 0, jst),
			checkTo: func(t *testing.T, to time.Time) {
				if to.Year() != 9999 {
					t.Errorf("to センチネル year=%d, want 9999", to.Year())
				}
			},
		},
		{
			name: "終了日のみは下限センチネル＋当日いっぱい",
			from: "", to: "2026-04-20",
			wantFrom: time.Date(1970, 1, 1, 0, 0, 0, 0, jst),
			checkTo: func(t *testing.T, to time.Time) {
				if to.Before(time.Date(2026, 4, 20, 23, 59, 0, 0, jst)) || !to.Before(time.Date(2026, 4, 21, 0, 0, 0, 0, jst)) {
					t.Errorf("to=%v, want 2026-04-20 end-of-day", to)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			query := "?from=" + tc.from + "&to=" + tc.to

			// 画面経路 (Index): 一覧/集計/件数/帳票・CSV リンクを同一区間で組む。
			idxRepo := goyaDeviceReadingsRepo()
			idxRepo.countVal = int64(len(consistencyRows()))
			idxRepo.summaryRow = fullSummaryRow()
			idxRepo.listRows = consistencyRows()
			idxRepo.rangeRows = consistencyRows()
			ir := newReadingsRouterWithUser(&ReadingsHandler{Repo: idxRepo}, 7)
			iw := getPath(ir, "/devices/1/readings"+query)
			if iw.Code != http.StatusOK {
				t.Fatalf("Index status=%d, want 200", iw.Code)
			}

			// CSV 経路 (Export): 同一 from/to。
			expRepo := goyaDeviceReadingsRepo()
			expRepo.rangeRows = consistencyRows()
			er := newExportRouterWithUser(&ReadingsHandler{Repo: expRepo}, 7)
			ew := getPath(er, "/devices/1/readings.csv"+query)
			if ew.Code != http.StatusOK {
				t.Fatalf("Export status=%d, want 200", ew.Code)
			}

			// 1. Index 内の全クエリが同一区間 (既存の単一区間源不変条件)。
			assertIndexIntervalShared(t, idxRepo)

			// 2. 区間境界が期待どおり (parseDateBounds 写像: 始端/センチネル/end-of-day)。
			if !idxRepo.lastRange.RecordedAt.Time.Equal(tc.wantFrom) {
				t.Errorf("from=%v, want %v", idxRepo.lastRange.RecordedAt.Time, tc.wantFrom)
			}
			tc.checkTo(t, idxRepo.lastRange.RecordedAt_2.Time)

			// 3. 画面経路と CSV 経路が同一区間 (parseDateBounds 単一源・R7.3)。
			if !idxRepo.lastRange.RecordedAt.Time.Equal(expRepo.lastRange.RecordedAt.Time) ||
				!idxRepo.lastRange.RecordedAt_2.Time.Equal(expRepo.lastRange.RecordedAt_2.Time) {
				t.Errorf("画面区間=(%v,%v) と CSV 区間=(%v,%v) が不一致 (R7.3 境界共通)",
					idxRepo.lastRange.RecordedAt.Time, idxRepo.lastRange.RecordedAt_2.Time,
					expRepo.lastRange.RecordedAt.Time, expRepo.lastRange.RecordedAt_2.Time)
			}

			// 4. CSV 行集合と帳票バケットが同一データに基づく (R7.2)。
			//    CSV のデータ行数 = 全行件数、帳票は同じ全行から 2026-04-20 の日次バケットを持つ。
			recs, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(ew.Body.Bytes(), []byte{0xEF, 0xBB, 0xBF}))).ReadAll()
			if err != nil {
				t.Fatalf("CSV パース失敗: %v", err)
			}
			if got := len(recs) - 1; got != len(consistencyRows()) { // ヘッダ除く
				t.Errorf("CSV データ行数=%d, want %d (全行出力)", got, len(consistencyRows()))
			}
			// 画面の帳票に同一日のバケットが描画される (CSV と同じ全行入力)。
			body := iw.Body.String()
			if !strings.Contains(body, "集計帳票（日次）") || !strings.Contains(body, "2026-04-20") {
				t.Errorf("帳票に 2026-04-20 の日次バケットが無い (CSV と同一全行のはず)")
			}
		})
	}
}
