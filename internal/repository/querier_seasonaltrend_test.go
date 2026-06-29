package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// seasonal-trend（統計分析ページ／長期トレンド）が依存する Querier メソッドの
// 存在をコンパイル時に保証するガード。
//
// JST 暦日でバケットした日次集計（ListDailySensorAggregatesJST）が db/queries に
// 追加され `make sqlc` で再生成されていないと、この構造的インターフェースへの代入が
// コンパイルエラーになる（TDD の RED）。生成後は型整合してコンパイルが通る（GREEN）。
//
// 引数形（deviceID + 取得下限の 2 引数）は既存 ListDailySensorAggregates と同形を意図する
// （JST 版は GROUP BY のタイムゾーン換算だけが異なる SELECT のみのクエリ）。
var _ interface {
	ListDailySensorAggregatesJST(ctx context.Context, arg ListDailySensorAggregatesJSTParams) ([]ListDailySensorAggregatesJSTRow, error)
} = Querier(nil)

// jstDailyAggregateStub は ListDailySensorAggregatesJST のみを実装する DB 非依存の
// 手書きモック（テストガイダンス集: Querier 手書きモックで DB 非依存検証）。
// device_id と取得下限（recorded_at >= $2）を尊重し、JST 暦日昇順の行を返す契約を表現する。
type jstDailyAggregateStub struct {
	rows []ListDailySensorAggregatesJSTRow
}

func (s jstDailyAggregateStub) ListDailySensorAggregatesJST(_ context.Context, arg ListDailySensorAggregatesJSTParams) ([]ListDailySensorAggregatesJSTRow, error) {
	// 下限（取得開始日時）以降のみを返す。device_id は呼び出し前に所有検証済みである前提。
	var out []ListDailySensorAggregatesJSTRow
	for _, r := range s.rows {
		if arg.RecordedAt.Valid && r.ReadingDate.Time.Before(arg.RecordedAt.Time) {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func mustDate(t *testing.T, y int, m time.Month, d int) pgtype.Date {
	t.Helper()
	return pgtype.Date{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC), Valid: true}
}

// JST 日次集計が「device_id + 下限」で取得でき、JST 暦日昇順で返ることを確認する。
func TestListDailySensorAggregatesJST_AscendingFromMock(t *testing.T) {
	stub := jstDailyAggregateStub{
		rows: []ListDailySensorAggregatesJSTRow{
			{ReadingDate: mustDate(t, 2026, time.January, 10), SampleCount: 144},
			{ReadingDate: mustDate(t, 2026, time.January, 11), SampleCount: 144},
			{ReadingDate: mustDate(t, 2026, time.January, 12), SampleCount: 120},
		},
	}

	got, err := stub.ListDailySensorAggregatesJST(context.Background(), ListDailySensorAggregatesJSTParams{
		DeviceID:   1,
		RecordedAt: pgtype.Timestamptz{Time: time.Date(2026, time.January, 11, 0, 0, 0, 0, time.UTC), Valid: true},
	})
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	// 下限 2026-01-11 以降のみ＝2 行に絞られる。
	if len(got) != 2 {
		t.Fatalf("取得行数 = %d, 期待 2（下限フィルタ後）", len(got))
	}
	// JST 暦日昇順を確認。
	for i := 1; i < len(got); i++ {
		if !got[i-1].ReadingDate.Time.Before(got[i].ReadingDate.Time) {
			t.Errorf("行 %d→%d が昇順でない: %v, %v",
				i-1, i, got[i-1].ReadingDate.Time, got[i].ReadingDate.Time)
		}
	}
}
