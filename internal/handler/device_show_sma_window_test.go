package handler

import (
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// --- 2.1 日スケール窓集合の決定（dayScaleWindowsFor） -------------------------

// 表示期間ごとに、最大3本・可視スパン以下の日スケール窓集合とラベルを返すこと
// （24h/3d→空・7d→2本{3,7}・30d→3本{3,7,14}・各ラベル「移動平均 N日」）（R1.1, 2.4, 3.1, 5.2）。
func TestDayScaleWindowsFor(t *testing.T) {
	tests := []struct {
		period     string
		wantDays   []int
		wantLabels []string
	}{
		{"24h", nil, nil},
		{"3d", nil, nil},
		{"7d", []int{3, 7}, []string{"移動平均 3日", "移動平均 7日"}},
		{"30d", []int{3, 7, 14}, []string{"移動平均 3日", "移動平均 7日", "移動平均 14日"}},
		{"unknown", nil, nil}, // 想定外 period は窓なし（防御）
	}
	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			got := dayScaleWindowsFor(tt.period)

			if len(got) != len(tt.wantDays) {
				t.Fatalf("dayScaleWindowsFor(%q) 本数 = %d, want %d (%v)", tt.period, len(got), len(tt.wantDays), got)
			}
			// 最大3本（R2.4）。
			if len(got) > 3 {
				t.Errorf("窓は最大3本想定だが %d 本", len(got))
			}
			for i := range got {
				if got[i].Days != tt.wantDays[i] {
					t.Errorf("窓[%d].Days = %d, want %d", i, got[i].Days, tt.wantDays[i])
				}
				if got[i].Label != tt.wantLabels[i] {
					t.Errorf("窓[%d].Label = %q, want %q", i, got[i].Label, tt.wantLabels[i])
				}
			}
		})
	}
}

// 各窓は当該ビューの可視スパン（7d=7日・30d=30日）以下であること（長すぎる窓を出さない・5.2）。
func TestDayScaleWindowsFor_窓は可視スパン以下(t *testing.T) {
	spanDays := map[string]int{"7d": 7, "30d": 30}
	for period, span := range spanDays {
		for _, w := range dayScaleWindowsFor(period) {
			if w.Days > span {
				t.Errorf("%s: 窓 %d日 が可視スパン %d日 を超える", period, w.Days, span)
			}
		}
	}
}

// --- 2.1 点数/日の推定（estimatePointsPerDay） -------------------------------

// intervalRows は固定間隔 interval で n 行の計測を作る（決定的テスト用）。
func intervalRows(n int, interval time.Duration) []repository.SensorReading {
	base := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	rows := make([]repository.SensorReading, n)
	for i := 0; i < n; i++ {
		rows[i] = sensorRow(1, base.Add(time.Duration(i)*interval), 25.0, 60.0)
	}
	return rows
}

// 間隔中央値から 86400/中央値 を返し、行不足や算出不能では 288 へフォールバックすること
// （5分→288・10分→144・0/1行→288・外れ値混在でも中央値が安定）（R1.1, 5.2）。
func TestEstimatePointsPerDay(t *testing.T) {
	const eps = 1e-9
	tests := []struct {
		name string
		rows []repository.SensorReading
		want float64
	}{
		{"5分間隔は288点/日", intervalRows(12, 5*time.Minute), 288},
		{"10分間隔は144点/日", intervalRows(12, 10*time.Minute), 144},
		{"0行は288フォールバック", nil, defaultPointsPerDay},
		{"1行は288フォールバック", intervalRows(1, 5*time.Minute), defaultPointsPerDay},
		{"2行(5分)は288", intervalRows(2, 5*time.Minute), 288},
		{"間隔0(同時刻)は288フォールバック", intervalRows(5, 0), defaultPointsPerDay}, // 中央値<=0 ガード
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimatePointsPerDay(tt.rows)
			if diff := got - tt.want; diff < -eps || diff > eps {
				t.Errorf("estimatePointsPerDay() = %v, want %v", got, tt.want)
			}
		})
	}
}

// 大きな欠測ギャップ（外れ値間隔）が混ざっても、中央値採用ゆえ点数/日が安定すること（⑤）。
func TestEstimatePointsPerDay_外れ値間隔に頑健(t *testing.T) {
	const eps = 1e-9
	base := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	// 5分間隔の点列に、途中 2時間の欠測ギャップ（外れ値間隔）を 1 つ挿入する。
	times := []time.Time{
		base,
		base.Add(5 * time.Minute),
		base.Add(10 * time.Minute),
		base.Add(15 * time.Minute),
		base.Add(15*time.Minute + 2*time.Hour), // 2時間ギャップ（外れ値）
		base.Add(20*time.Minute + 2*time.Hour),
		base.Add(25*time.Minute + 2*time.Hour),
	}
	rows := make([]repository.SensorReading, len(times))
	for i, ts := range times {
		rows[i] = sensorRow(1, ts, 25.0, 60.0)
	}

	// 間隔列 = [300,300,300,7200,300,300]（6個）。中央値=300 → 288。外れ値 7200 に引きずられない。
	got := estimatePointsPerDay(rows)
	if diff := got - 288.0; diff < -eps || diff > eps {
		t.Errorf("外れ値混在で estimatePointsPerDay() = %v, want 288（中央値が安定）", got)
	}
}
