// seasonal_trend_rollup.go は統計分析ページ（長期トレンド・季節サマリ）の handler 境界集約ロジックを担う。
// JST 日次集計（ListDailySensorAggregatesJST の戻り値）を月次/年次バケットへ二段集約し、平均/最高/最低/
// 日較差ΔT（=日次ΔTの平均）/σ/CV/サンプル数を算出する（要件 2.1〜2.4）。あわせて自社データの暦月平均
// （平年値）を算出し、利用可能年が3年以上のときのみ平年比を提示する（要件 7.1〜7.3・design D7）。
//
// O(N²) 検定（chart.trend）はこのロールアップ後系列にのみ適用する前提を満たすため、生粒度ではなく
// バケット平均系列を bucketSeries で取り出して検定へ渡す（要件 3.3）。純粋な統計は internal/chart へ委譲し、
// ここは時刻バケット（JST 暦年月の判定）と集約の handler 境界に集中する（device_show_gdd.go と同作法）。
package handler

import (
	"fmt"

	"github.com/HiroshiKawano/go_iot/internal/chart"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// 集計粒度（TrendQuery.Granularity の許容値・binding oneof=monthly yearly と一致）。
const (
	granularityMonthly = "monthly"
	granularityYearly  = "yearly"
)

// climatologyMinYears は平年比を提示する最小の利用可能年数（design D7・年<3 は非表示）。
const climatologyMinYears = 3

// 平年比の注記（要件 7.2/7.3）。
const (
	climatologyUncertaintyNote  = "平年比は自社データの暦月平均に基づきます。平年値は基準期間に依存し不確実性を伴うため、参考値としてご覧ください。"
	climatologyInsufficientNote = "平年比は利用可能年が3年未満のため非表示です（自社データの暦月平均は基準期間に依存し不確実なため断定しません）。"
)

// rollupStat は1バケット・1指標（温度 or 湿度）のロールアップ統計。
type rollupStat struct {
	Avg          float64 // 日次平均の平均
	Max          float64 // 日次最高の最大
	Min          float64 // 日次最低の最小
	DiurnalRange float64 // 日較差ΔT（=日次ΔT〔最高−最低〕の平均）
	StdDev       float64 // 日次平均系列の母標準偏差σ
	CV           float64 // 変動係数 σ/μ（HasCV=false なら未定義）
	HasCV        bool
}

// trendBucket は月次/年次1バケットの温度・湿度ロールアップ統計。
type trendBucket struct {
	Key      string // "2024-01"（月次）/ "2024"（年次）
	Year     int
	Month    int // 月次=1..12 / 年次=0
	Samples  int // 当バケットの日次件数の合計（温度・湿度で共通＝count(*)）
	Temp     rollupStat
	Humidity rollupStat
}

// climatologyResult は平年比（暦月平均）の算出結果。
type climatologyResult struct {
	Available bool      // 利用可能年≥3 で true
	Note      string    // 提示時/非表示時いずれも不確実性注記を持つ
	Values    []float64 // バケット整列の平年値（各バケットの暦月平均）。非表示時 nil
}

// bucketAccum はバケット集約途中の日次値アキュムレータ（非公開）。
type bucketAccum struct {
	key   string
	year  int
	month int
	avgT  []float64
	maxT  []float64
	minT  []float64
	avgH  []float64
	maxH  []float64
	minH  []float64
	samp  int
}

// rollupDailyJST は JST 日次集計を月次/年次バケットへ二段集約する（要件 2.1/2.2）。
// 行は ListDailySensorAggregatesJST で JST 暦日昇順に得られている前提。空バケット（計測の無い月/年）は
// 行が無いため自然に現れず、欠測を 0 補完しない（要件 2.3）。日較差ΔTは「月の max−min」でなく
// 「日次ΔT〔最高−最低〕の平均」（design D4）。granularity が "yearly" 以外は月次として扱う。
func rollupDailyJST(rows []repository.ListDailySensorAggregatesJSTRow, granularity string) []trendBucket {
	idx := make(map[string]int)
	accs := make([]*bucketAccum, 0)

	for _, r := range rows {
		// pgtype.Date は JST 暦日を UTC 0:00 で保持する（DATE(... AT TIME ZONE 'Asia/Tokyo')）。
		// 暦年月のみ使うため UTC のまま .Date() を取れば JST 暦年月になる（再 TZ 変換は不要）。
		y, mo, _ := r.ReadingDate.Time.Date()
		var key string
		bMonth := 0
		if granularity == granularityYearly {
			key = fmt.Sprintf("%04d", y)
		} else {
			bMonth = int(mo)
			key = fmt.Sprintf("%04d-%02d", y, int(mo))
		}

		i, ok := idx[key]
		if !ok {
			i = len(accs)
			idx[key] = i
			accs = append(accs, &bucketAccum{key: key, year: y, month: bMonth})
		}
		a := accs[i]
		a.avgT = append(a.avgT, pgconv.NumericToFloat(r.AvgTemperature))
		a.maxT = append(a.maxT, aggregateToFloat(r.MaxTemperature))
		a.minT = append(a.minT, aggregateToFloat(r.MinTemperature))
		a.avgH = append(a.avgH, pgconv.NumericToFloat(r.AvgHumidity))
		a.maxH = append(a.maxH, aggregateToFloat(r.MaxHumidity))
		a.minH = append(a.minH, aggregateToFloat(r.MinHumidity))
		a.samp += int(r.SampleCount)
	}

	buckets := make([]trendBucket, len(accs))
	for i, a := range accs {
		buckets[i] = trendBucket{
			Key:      a.key,
			Year:     a.year,
			Month:    a.month,
			Samples:  a.samp,
			Temp:     statFromDaily(a.avgT, a.maxT, a.minT),
			Humidity: statFromDaily(a.avgH, a.maxH, a.minH),
		}
	}
	return buckets
}

// statFromDaily は1指標の日次系列（平均/最高/最低）からバケット統計を算出する。
//   - Avg          = 日次平均の平均
//   - Max/Min      = 日次最高の最大 / 日次最低の最小
//   - DiurnalRange = 日次ΔT（最高−最低）の平均
//   - StdDev/CV    = 日次平均系列上の母標準偏差・変動係数（純粋層 chart に委譲）
func statFromDaily(avg, maxV, minV []float64) rollupStat {
	dt := make([]float64, 0, len(maxV))
	n := len(maxV)
	if len(minV) < n {
		n = len(minV)
	}
	for i := 0; i < n; i++ {
		dt = append(dt, maxV[i]-minV[i])
	}
	_, bucketMax := chart.MinMax(maxV) // 日次最高の最大
	bucketMin, _ := chart.MinMax(minV) // 日次最低の最小
	cv, hasCV := chart.CV(avg, 1e-9)
	return rollupStat{
		Avg:          chart.Mean(avg),
		Max:          bucketMax,
		Min:          bucketMin,
		DiurnalRange: chart.Mean(dt),
		StdDev:       chart.StdDev(avg),
		CV:           cv,
		HasCV:        hasCV,
	}
}

// bucketSeries はバケット平均系列（指標別）と月キーを取り出す（trend 検定へ渡す形）。
// O(N²) 検定は生粒度でなくこのロールアップ後系列に適用する（要件 3.3）。月キーは Seasonal MK に使う。
func bucketSeries(buckets []trendBucket, metric string) (values []float64, months []int) {
	values = make([]float64, len(buckets))
	months = make([]int, len(buckets))
	for i, b := range buckets {
		values[i] = metricStat(b, metric).Avg
		months[i] = b.Month
	}
	return values, months
}

// buildClimatology は自社データの暦月平均（平年値）を算出する（要件 7.1〜7.3・design D7）。
// 利用可能年（distinct Year）が3年以上のときのみ Available=true としバケット整列の平年値を返す。
// 未満は非表示（Values nil）＋「基準期間依存・不確実」注記とし、不安定な平年比を断定しない。
func buildClimatology(buckets []trendBucket, metric string) climatologyResult {
	years := make(map[int]bool)
	for _, b := range buckets {
		years[b.Year] = true
	}
	if len(years) < climatologyMinYears {
		return climatologyResult{Available: false, Note: climatologyInsufficientNote}
	}

	// 暦月（年次は Month=0 で一括）ごとに当指標の平均を集計し、各バケットへ整列する。
	sum := make(map[int]float64)
	cnt := make(map[int]int)
	for _, b := range buckets {
		v := metricStat(b, metric).Avg
		sum[b.Month] += v
		cnt[b.Month]++
	}
	values := make([]float64, len(buckets))
	for i, b := range buckets {
		values[i] = sum[b.Month] / float64(cnt[b.Month])
	}
	return climatologyResult{Available: true, Note: climatologyUncertaintyNote, Values: values}
}

// metricStat は指標名に対応するバケット統計を返す（湿度以外は温度扱い）。
func metricStat(b trendBucket, metric string) rollupStat {
	if metric == metricHumidity {
		return b.Humidity
	}
	return b.Temp
}
