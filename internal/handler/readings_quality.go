// readings_quality.go はセンサーデータ履歴画面 (GET /devices/:device/readings) へ上載せする
// データ品質メタ層 (data-quality-meta フェーズ) の組み立てを担う。
// ListSensorReadingsInRange の期間内全行 (BETWEEN・ASC) から、温湿度値列と間隔秒列
// (recorded_at 差分・time 境界処理は本 handler 層) を作り、internal/chart の品質純関数へ渡す。
//
// 責務は (1) 期間メトリクス (欠測率/間隔一貫性/通信遅延代表値) と総合品質レベルの合成、
// (2) レコード単位の品質フラグ (温度・湿度双方の判定を OR・異常行のみ非空) の算出。
// 計算の実体は internal/chart の純関数に委譲し、しきい値はその既定定数を渡す (再実装しない)。
// 表示カテゴリは internal/domain の QualityFlag/QualityLevel 列挙で持つ (DB 非永続)。
package handler

import (
	"fmt"
	"math"

	"github.com/HiroshiKawano/go_iot/internal/chart"
	"github.com/HiroshiKawano/go_iot/internal/domain"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/view/component"
)

// 総合品質バッジの合成しきい値 (research/沖縄実環境で調整可・design Decision ⑤)。
// 赤=欠測率>30% or 固着検出 or 物理異常>0件 / 黄=欠測率>5% or 外れ値率>5% or 間隔CV>0.5 / 緑=その他。
const (
	badgeMissingBad     = 30.0 // 欠測率 これ超で「不良」
	badgeMissingCaution = 5.0  // 欠測率 これ超で「注意」
	badgeOutlierCaution = 5.0  // 外れ値率 これ超で「注意」
	badgeCVCaution      = 0.5  // 間隔CV これ超で「注意」(サンプリング間隔のばらつき大)
)

// QualityMetrics は期間 (BETWEEN 区間) の品質メトリクスの中間表現 (整形前の生値)。
// 表示用整形 (QualityMetricsView) とバッジ合成 (badgeLevel) の双方が参照する。
// HasData=false (0件/単一点/算出不能) のときは各値は無意味で、表示側が "—" へ写す。
type QualityMetrics struct {
	MissingRate   float64 // 欠測率(%)
	IntervalCV    float64 // サンプリング間隔の変動係数 (σ/μ)
	DelayAvg      float64 // 通信遅延 平均(秒)
	DelayMax      float64 // 通信遅延 最大(秒)
	OutlierRate   float64 // 外れ値率(%) (温度・湿度いずれかが外れ値の行の割合)
	StuckDetected bool    // 固着(stuck)を1行でも検出したか
	PhysicalCount int     // 物理異常(範囲外/急変)に該当する行数
	HasData       bool    // 算出可否 (2点以上かつ中央値/μ が定義可能)
}

// buildQualityMetrics は期間内全行 (ASC) から期間品質メトリクスを算出する。
// 間隔秒列は recorded_at 差分 (handler 境界の time 処理) で生成し、chart 純関数へ渡す。
// 0件/単一点/算出不能は HasData=false を返す (要件 3.5/6.1/6.2)。
func buildQualityMetrics(rows []repository.SensorReading) QualityMetrics {
	n := len(rows)
	if n < 2 {
		return QualityMetrics{HasData: false}
	}
	intervals := intervalSeconds(rows)
	missingRate, _, _, okMiss := chart.MissingStats(intervals)
	cv, okCV := chart.IntervalConsistency(intervals, statEpsilon)
	if !okMiss || !okCV {
		// 2点以上でも中央値0/μ0(同一時刻が並ぶ等)で算出不能。安全に空状態へ。
		return QualityMetrics{HasData: false}
	}

	// 集計カウントは行フラグから導出し、総合バッジと行表示の判定を一貫させる。
	outCount, physCount := 0, 0
	stuckDetected := false
	for _, fs := range rowQualityFlags(rows) {
		for _, f := range fs {
			switch f {
			case domain.QualityFlagOutlier:
				outCount++
			case domain.QualityFlagStuck:
				stuckDetected = true
			case domain.QualityFlagPhysical:
				physCount++
			}
		}
	}

	delayAvg, delayMax := delayStats(rows)
	return QualityMetrics{
		MissingRate:   missingRate,
		IntervalCV:    cv,
		DelayAvg:      delayAvg,
		DelayMax:      delayMax,
		OutlierRate:   float64(outCount) / float64(n) * 100,
		StuckDetected: stuckDetected,
		PhysicalCount: physCount,
		HasData:       true,
	}
}

// badgeLevel は期間メトリクスから総合品質レベル (信号色) を合成する (design Decision ⑤)。
// 赤を最優先で判定し、次に黄、いずれでもなければ緑。HasData には依存しない
// (表示可否は呼び出し側が HasData で判断する)。
func badgeLevel(m QualityMetrics) domain.QualityLevel {
	switch {
	case m.MissingRate > badgeMissingBad || m.StuckDetected || m.PhysicalCount > 0:
		return domain.QualityLevelBad
	case m.MissingRate > badgeMissingCaution || m.OutlierRate > badgeOutlierCaution || m.IntervalCV > badgeCVCaution:
		return domain.QualityLevelCaution
	default:
		return domain.QualityLevelGood
	}
}

// rowQualityFlags は各行の品質フラグ集合 (欠測直後/固着/物理異常/外れ値) を返す。
// 温度・湿度双方の判定を OR し、正常行は空 (nil) とする (異常行のみ強調・要件 1.1/1.2/1.3)。
// 事前条件: rows は recorded_at 昇順 (ListSensorReadingsInRange が保証)。
// 事後条件: len(out)==len(rows)。フラグ順は 欠測直後→固着→物理異常→外れ値 (AllQualityFlags 順)。
func rowQualityFlags(rows []repository.SensorReading) [][]domain.QualityFlag {
	n := len(rows)
	flags := make([][]domain.QualityFlag, n)
	if n == 0 {
		return flags
	}
	temps, hums := readingValues(rows)

	// 外れ値(ローリングσ・温湿度別)。warm-up と σ≈0 は false。
	tOut := chart.RollingOutliers(temps, chart.OutlierWindow, chart.OutlierK, chart.OutlierEps)
	hOut := chart.RollingOutliers(hums, chart.OutlierWindow, chart.OutlierK, chart.OutlierEps)
	// 固着(同値連続・温湿度別)。
	tStuck := chart.StuckRuns(temps, chart.StuckMinRun)
	hStuck := chart.StuckRuns(hums, chart.StuckMinRun)
	// 物理異常 = 温度の物理範囲外 or 急変(温湿度)。湿度の物理範囲は設けない(沖縄の高湿度継続は正常)。
	tPhys := chart.PhysicalAnomalies(temps, chart.TempPhysicalMin, chart.TempPhysicalMax)
	tRapid := chart.RapidChanges(temps, chart.TempRapidDelta)
	hRapid := chart.RapidChanges(hums, chart.HumidityRapidDelta)
	// 欠測直後 = gap の直後行(EndIdx)。
	missingAdj := make([]bool, n)
	if intervals := intervalSeconds(rows); len(intervals) >= 2 {
		if _, _, gaps, ok := chart.MissingStats(intervals); ok {
			for _, g := range gaps {
				if g.EndIdx >= 0 && g.EndIdx < n {
					missingAdj[g.EndIdx] = true
				}
			}
		}
	}

	for i := 0; i < n; i++ {
		var fs []domain.QualityFlag
		if missingAdj[i] {
			fs = append(fs, domain.QualityFlagMissing)
		}
		if tStuck[i] || hStuck[i] {
			fs = append(fs, domain.QualityFlagStuck)
		}
		if tPhys[i] || tRapid[i] || hRapid[i] {
			fs = append(fs, domain.QualityFlagPhysical)
		}
		if tOut[i] || hOut[i] {
			fs = append(fs, domain.QualityFlagOutlier)
		}
		flags[i] = fs // 正常行は nil(空)
	}
	return flags
}

// --- 純粋ヘルパ (DB 非依存・pgconv 境界変換) ---

// readingValues は計測行から温度・湿度の float 列を取り出す (pgconv 境界変換)。
// 事後条件: len(temps)==len(hums)==len(rows)。
func readingValues(rows []repository.SensorReading) (temps, hums []float64) {
	temps = make([]float64, len(rows))
	hums = make([]float64, len(rows))
	for i, r := range rows {
		temps[i] = pgconv.NumericToFloat(r.Temperature)
		hums[i] = pgconv.NumericToFloat(r.Humidity)
	}
	return temps, hums
}

// intervalSeconds は隣接行の recorded_at 差分(秒)の列を返す (time 境界処理は handler)。
// len(rows)<2 は nil (間隔が定義できない)。事後条件: 非 nil 時 len==len(rows)-1。
func intervalSeconds(rows []repository.SensorReading) []float64 {
	if len(rows) < 2 {
		return nil
	}
	out := make([]float64, len(rows)-1)
	for i := 0; i < len(rows)-1; i++ {
		t0 := pgconv.TimestamptzToTime(rows[i].RecordedAt)
		t1 := pgconv.TimestamptzToTime(rows[i+1].RecordedAt)
		out[i] = t1.Sub(t0).Seconds()
	}
	return out
}

// delayStats は通信遅延 (created_at - recorded_at・秒) の平均と最大を返す。
// クロックずれによる負値は 0 にクランプする (formatDelay と同規約)。空行は (0,0)。
func delayStats(rows []repository.SensorReading) (avg, max float64) {
	if len(rows) == 0 {
		return 0, 0
	}
	var sum float64
	for _, r := range rows {
		d := pgconv.TimestamptzToTime(r.CreatedAt).Sub(pgconv.TimestamptzToTime(r.RecordedAt)).Seconds()
		if d < 0 {
			d = 0
		}
		sum += d
		if d > max {
			max = d
		}
	}
	return sum / float64(len(rows)), max
}

// --- ViewModel 組み立て (4.3 配線) ---

// buildQualityMetricsView は期間内全行から品質メトリクスボックス+総合バッジの View を組む。
// HasData=false (0件/単一点/算出不能) は全項目 "—"・バッジ非表示 (要件 3.5/6・statEmptyMark 作法)。
func buildQualityMetricsView(rows []repository.SensorReading) component.QualityMetricsView {
	m := buildQualityMetrics(rows)
	if !m.HasData {
		return component.QualityMetricsView{
			MissingRate: statEmptyMark,
			IntervalCV:  statEmptyMark,
			DelayAvg:    statEmptyMark,
			DelayMax:    statEmptyMark,
			Level:       domain.QualityLevelGood, // HasData=false ゆえ表示されない
			HasData:     false,
		}
	}
	return component.QualityMetricsView{
		MissingRate: fmt.Sprintf("%.1f%%", m.MissingRate),
		IntervalCV:  formatStat(m.IntervalCV, ""),
		DelayAvg:    fmt.Sprintf("%d秒", int(math.Round(m.DelayAvg))),
		DelayMax:    fmt.Sprintf("%d秒", int(math.Round(m.DelayMax))),
		Level:       badgeLevel(m),
		HasData:     true,
	}
}

// rowFlagsByID は期間内全行 (ASC) から行フラグを算出し、reading ID → フラグ集合の写像を返す。
// 行フラグはローリング窓/欠測ギャップの文脈が要るため全行で算出し、ページング後の表示行へは
// ID で引き当てる (一覧は DESC・20件窓だが全行 ASC 文脈で判定する)。正常行 (空) は map に載せない。
func rowFlagsByID(rows []repository.SensorReading) map[int64][]domain.QualityFlag {
	flags := rowQualityFlags(rows)
	m := make(map[int64][]domain.QualityFlag)
	for i, r := range rows {
		if len(flags[i]) > 0 {
			m[r.ID] = flags[i]
		}
	}
	return m
}
