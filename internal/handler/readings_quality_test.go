package handler

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/domain"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// readings_quality_test.go は品質メタ組み立て (期間メトリクス/バッジ合成/行フラグ) を
// DB 非依存で検証する (テストガイダンス集 §4・純粋層は表駆動)。

// qualityBase は決定的テスト用の基準時刻 (JST 換算は handler 内で行う)。
var qualityBase = time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)

// qRow は ID・recorded_at(基準+offsetSec)・created_at(recorded+delaySec)・温湿度を持つ計測行を作る。
func qRow(id int64, offsetSec, delaySec, temp, hum float64) repository.SensorReading {
	rec := qualityBase.Add(time.Duration(offsetSec) * time.Second)
	return repository.SensorReading{
		ID:          id,
		DeviceID:    1,
		RecordedAt:  pgconv.Timestamptz(rec),
		CreatedAt:   pgconv.Timestamptz(rec.Add(time.Duration(delaySec) * time.Second)),
		Temperature: pgconv.Numeric2(temp),
		Humidity:    pgconv.Numeric2(hum),
	}
}

// hasFlag はフラグ集合に f が含まれるか。
func hasFlag(fs []domain.QualityFlag, f domain.QualityFlag) bool {
	for _, x := range fs {
		if x == f {
			return true
		}
	}
	return false
}

// ---- 4.1 buildQualityMetrics ------------------------------------------------

func TestBuildQualityMetrics_正常系の欠測率CV遅延(t *testing.T) {
	// 等間隔300秒・温湿度は微変動(正常)・遅延 [2,4,2,4,2]。
	rows := []repository.SensorReading{
		qRow(1, 0, 2, 20, 60),
		qRow(2, 300, 4, 21, 61),
		qRow(3, 600, 2, 20, 60),
		qRow(4, 900, 4, 22, 62),
		qRow(5, 1200, 2, 21, 61),
	}
	m := buildQualityMetrics(rows)

	if !m.HasData {
		t.Fatal("HasData=false, want true")
	}
	if math.Abs(m.MissingRate-0) > 0.001 {
		t.Errorf("MissingRate=%v, want 0", m.MissingRate)
	}
	if math.Abs(m.IntervalCV-0) > 0.001 {
		t.Errorf("IntervalCV=%v, want 0", m.IntervalCV)
	}
	if math.Abs(m.DelayAvg-2.8) > 0.001 {
		t.Errorf("DelayAvg=%v, want 2.8", m.DelayAvg)
	}
	if math.Abs(m.DelayMax-4) > 0.001 {
		t.Errorf("DelayMax=%v, want 4", m.DelayMax)
	}
	// 正常系ゆえ外れ値0・固着なし・物理異常0。
	if m.OutlierRate != 0 || m.StuckDetected || m.PhysicalCount != 0 {
		t.Errorf("正常系で異常が検出された: outlier=%v stuck=%v phys=%d", m.OutlierRate, m.StuckDetected, m.PhysicalCount)
	}
}

func TestBuildQualityMetrics_欠測区間で欠測率が上がる(t *testing.T) {
	// 中央値300秒に対し2区間目が900秒(欠測2スロット)。missingRate=2/(4+2)*100=33.33%。
	rows := []repository.SensorReading{
		qRow(1, 0, 1, 20, 60),
		qRow(2, 300, 1, 21, 61),
		qRow(3, 1200, 1, 20, 60), // 900秒ギャップ
		qRow(4, 1500, 1, 22, 62),
	}
	m := buildQualityMetrics(rows)
	if !m.HasData {
		t.Fatal("HasData=false, want true")
	}
	if math.Abs(m.MissingRate-(2.0/6.0*100)) > 0.01 {
		t.Errorf("MissingRate=%v, want 33.33", m.MissingRate)
	}
}

func TestBuildQualityMetrics_空と単一点はHasDatafalse(t *testing.T) {
	if m := buildQualityMetrics(nil); m.HasData {
		t.Error("0件で HasData=true")
	}
	if m := buildQualityMetrics([]repository.SensorReading{qRow(1, 0, 1, 20, 60)}); m.HasData {
		t.Error("単一点で HasData=true")
	}
	// 2点(間隔1本)は中央値を確立できず欠測率/CVが未定義 → HasData=false。
	twoRows := []repository.SensorReading{qRow(1, 0, 1, 20, 60), qRow(2, 300, 1, 21, 61)}
	if m := buildQualityMetrics(twoRows); m.HasData {
		t.Error("2点で HasData=true（間隔1本では算出不能のはず）")
	}
}

func TestBuildQualityMetrics_異常系のカウント(t *testing.T) {
	t.Run("固着+物理異常の件数を集計", func(t *testing.T) {
		// 70℃が6行連続 → 全行が固着かつ物理範囲外。
		rows := make([]repository.SensorReading, 6)
		for i := range rows {
			rows[i] = qRow(int64(i+1), float64(i*300), 1, 70.0, 50.0)
		}
		m := buildQualityMetrics(rows)
		if !m.StuckDetected {
			t.Error("StuckDetected=false, want true")
		}
		if m.PhysicalCount != 6 {
			t.Errorf("PhysicalCount=%d, want 6", m.PhysicalCount)
		}
	})
	t.Run("外れ値率を集計", func(t *testing.T) {
		// 微変動の末尾にスパイク1点 → 外れ値率 1/13。
		temps := []float64{20.0, 20.1, 20.0, 20.1, 20.0, 20.1, 20.0, 20.1, 20.0, 20.1, 20.0, 20.1, 30.0}
		hums := []float64{60.0, 60.1, 60.0, 60.1, 60.0, 60.1, 60.0, 60.1, 60.0, 60.1, 60.0, 60.1, 60.0}
		rows := make([]repository.SensorReading, len(temps))
		for i := range temps {
			rows[i] = qRow(int64(i+1), float64(i*300), 1, temps[i], hums[i])
		}
		m := buildQualityMetrics(rows)
		if m.OutlierRate <= 0 {
			t.Errorf("OutlierRate=%v, want >0", m.OutlierRate)
		}
	})
}

// ---- 4.1 badgeLevel ---------------------------------------------------------

func TestBadgeLevel(t *testing.T) {
	tests := []struct {
		name string
		m    QualityMetrics
		want domain.QualityLevel
	}{
		{"全て低水準は信頼(緑)", QualityMetrics{MissingRate: 0, OutlierRate: 0, IntervalCV: 0}, domain.QualityLevelGood},
		{"欠測率5ちょうどは緑(>5でない)", QualityMetrics{MissingRate: 5}, domain.QualityLevelGood},
		{"欠測率5超は注意(黄)", QualityMetrics{MissingRate: 5.1}, domain.QualityLevelCaution},
		{"外れ値率5超は注意", QualityMetrics{OutlierRate: 6}, domain.QualityLevelCaution},
		{"間隔CV0.5超は注意", QualityMetrics{IntervalCV: 0.6}, domain.QualityLevelCaution},
		{"欠測率30ちょうどは注意(>30でない)", QualityMetrics{MissingRate: 30}, domain.QualityLevelCaution},
		{"欠測率30超は不良(赤)", QualityMetrics{MissingRate: 30.1}, domain.QualityLevelBad},
		{"固着検出は不良", QualityMetrics{StuckDetected: true}, domain.QualityLevelBad},
		{"物理異常>0は不良", QualityMetrics{PhysicalCount: 1}, domain.QualityLevelBad},
		{"赤が黄より優先", QualityMetrics{MissingRate: 6, PhysicalCount: 1}, domain.QualityLevelBad},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := badgeLevel(tt.m); got != tt.want {
				t.Errorf("badgeLevel(%+v) = %q, want %q", tt.m, got, tt.want)
			}
		})
	}
}

// ---- 4.2 rowQualityFlags ----------------------------------------------------

func TestRowQualityFlags_正常行は空(t *testing.T) {
	// 等間隔・微変動・範囲内・固着なし → どの行もフラグ無し。
	rows := []repository.SensorReading{
		qRow(1, 0, 1, 20, 60),
		qRow(2, 300, 1, 21, 61),
		qRow(3, 600, 1, 20, 60),
		qRow(4, 900, 1, 22, 62),
		qRow(5, 1200, 1, 21, 61),
	}
	flags := rowQualityFlags(rows)
	if len(flags) != len(rows) {
		t.Fatalf("len=%d, want %d", len(flags), len(rows))
	}
	for i, fs := range flags {
		if len(fs) != 0 {
			t.Errorf("正常行 %d にフラグが付いた: %v", i, fs)
		}
	}
}

func TestRowQualityFlags_外れ値行のみ非空(t *testing.T) {
	// 微変動(20.0/20.1)で σ を低く保ち、末尾(index12)に 30.0 スパイク。
	// 窓12点が満ちる index12 のみ外れ値。固着回避(交互で同値連続なし)・急変回避(9.9<10)。
	temps := []float64{20.0, 20.1, 20.0, 20.1, 20.0, 20.1, 20.0, 20.1, 20.0, 20.1, 20.0, 20.1, 30.0}
	hums := []float64{60.0, 60.1, 60.0, 60.1, 60.0, 60.1, 60.0, 60.1, 60.0, 60.1, 60.0, 60.1, 60.0}
	rows := make([]repository.SensorReading, len(temps))
	for i := range temps {
		rows[i] = qRow(int64(i+1), float64(i*300), 1, temps[i], hums[i])
	}
	flags := rowQualityFlags(rows)

	for i := 0; i < len(rows)-1; i++ {
		if len(flags[i]) != 0 {
			t.Errorf("行 %d は正常のはずがフラグ付き: %v", i, flags[i])
		}
	}
	last := flags[len(rows)-1]
	if !hasFlag(last, domain.QualityFlagOutlier) {
		t.Errorf("スパイク行に外れ値フラグが無い: %v", last)
	}
	// 外れ値のみ(固着/物理/急変は付かない)。
	if hasFlag(last, domain.QualityFlagStuck) || hasFlag(last, domain.QualityFlagPhysical) {
		t.Errorf("スパイク行に余計なフラグ: %v", last)
	}
}

func TestRowQualityFlags_複数該当行は複数フラグ(t *testing.T) {
	// 温度70.0が6行連続 → 固着(stuck) かつ 物理範囲外(>60℃)。各行に2フラグ。
	rows := make([]repository.SensorReading, 6)
	for i := range rows {
		rows[i] = qRow(int64(i+1), float64(i*300), 1, 70.0, 50.0)
	}
	flags := rowQualityFlags(rows)
	for i, fs := range flags {
		if !hasFlag(fs, domain.QualityFlagStuck) || !hasFlag(fs, domain.QualityFlagPhysical) {
			t.Errorf("行 %d は固着+物理異常のはず: %v", i, fs)
		}
		if len(fs) != 2 {
			t.Errorf("行 %d のフラグ数=%d, want 2: %v", i, len(fs), fs)
		}
	}
}

func TestRowQualityFlags_欠測直後行にフラグ(t *testing.T) {
	// 中央値300秒に対し index1→2 が900秒ギャップ → 欠測直後は index2 の行。
	rows := []repository.SensorReading{
		qRow(1, 0, 1, 20, 60),
		qRow(2, 300, 1, 21, 61),
		qRow(3, 1200, 1, 20, 60), // 欠測直後
		qRow(4, 1500, 1, 22, 62),
	}
	flags := rowQualityFlags(rows)
	if !hasFlag(flags[2], domain.QualityFlagMissing) {
		t.Errorf("欠測直後の行(index2)に欠測フラグが無い: %v", flags[2])
	}
	if hasFlag(flags[0], domain.QualityFlagMissing) || hasFlag(flags[1], domain.QualityFlagMissing) {
		t.Errorf("欠測前の行に欠測フラグが付いた: %v %v", flags[0], flags[1])
	}
}

func TestRowQualityFlags_空入力は空(t *testing.T) {
	if got := rowQualityFlags(nil); len(got) != 0 {
		t.Errorf("len=%d, want 0", len(got))
	}
}

// ---- 4.3 fetchResults への品質メタ配線 (DB 非依存・Querier モック) ------------

// 全行 ASC で行フラグを算出し、ページング後 (DESC 窓) の表示行へ ID で正しく引き当てること、
// 期間メトリクス/総合バッジが BETWEEN 区間の全行から組まれて View に載ることを固定する。
func TestFetchResults_品質メタとフラグがViewに載る(t *testing.T) {
	// 全行(ASC): id3 の手前(id2→id3)が900秒ギャップ → 欠測直後は id3。欠測率 33%>30 → 総合「不良」。
	allRows := []repository.SensorReading{
		qRow(1, 0, 1, 20, 60),
		qRow(2, 300, 1, 21, 61),
		qRow(3, 1200, 1, 20, 60), // 欠測直後
		qRow(4, 1500, 1, 22, 62),
	}
	// 一覧(ページング DESC): 新しい順 id4,id3,id2,id1。
	listRows := []repository.SensorReading{
		qRow(4, 1500, 1, 22, 62),
		qRow(3, 1200, 1, 20, 60),
		qRow(2, 300, 1, 21, 61),
		qRow(1, 0, 1, 20, 60),
	}
	repo := ownerReadingsRepo()
	repo.countVal = int64(len(allRows))
	repo.summaryRow = fullSummaryRow()
	repo.listRows = listRows
	repo.rangeRows = allRows

	h := &ReadingsHandler{Repo: repo}
	device := repo.devices[1]
	list, err := h.fetchResults(context.Background(), device, "", "", distantPast, distantFuture, "1", nil)
	if err != nil {
		t.Fatalf("fetchResults() でエラー: %v", err)
	}

	// 期間メトリクス/総合バッジが載る。
	if !list.Quality.HasData {
		t.Fatal("Quality.HasData=false, want true")
	}
	if list.Quality.MissingRate == statEmptyMark || list.Quality.MissingRate == "" {
		t.Errorf("Quality.MissingRate が空: %q", list.Quality.MissingRate)
	}
	if list.Quality.Level != domain.QualityLevelBad {
		t.Errorf("Quality.Level=%q, want bad (欠測率33%%>30)", list.Quality.Level)
	}

	// 行フラグは ASC 文脈で算出し DESC 表示行へ ID マップ: id3(=表示 index1) が欠測直後。
	if len(list.Rows) != 4 {
		t.Fatalf("Rows 数=%d, want 4", len(list.Rows))
	}
	if !hasFlag(list.Rows[1].QualityFlags, domain.QualityFlagMissing) {
		t.Errorf("id3 行(表示index1)に欠測フラグが無い: %v", list.Rows[1].QualityFlags)
	}
	// 正常行(id4=index0, id1=index3)はフラグ空。
	if len(list.Rows[0].QualityFlags) != 0 || len(list.Rows[3].QualityFlags) != 0 {
		t.Errorf("正常行にフラグが付いた: index0=%v index3=%v", list.Rows[0].QualityFlags, list.Rows[3].QualityFlags)
	}
}

// 0件期間は品質メタが空状態 ("—"・バッジ非表示・HasData=false) になること。
func TestFetchResults_空期間は品質メタも空(t *testing.T) {
	repo := ownerReadingsRepo() // 既定 0 件
	h := &ReadingsHandler{Repo: repo}
	device := repo.devices[1]
	list, err := h.fetchResults(context.Background(), device, "", "", distantPast, distantFuture, "1", nil)
	if err != nil {
		t.Fatalf("fetchResults() でエラー: %v", err)
	}
	if list.Quality.HasData {
		t.Error("0件で Quality.HasData=true")
	}
	if list.Quality.MissingRate != statEmptyMark {
		t.Errorf("0件で MissingRate=%q, want %q", list.Quality.MissingRate, statEmptyMark)
	}
}
