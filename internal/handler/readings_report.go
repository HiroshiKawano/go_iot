// readings_report.go はセンサーデータ履歴画面の集計帳票 (日次/時間別) を
// 期間内全行から読み取り時に組み立てる純粋層を担う (sensor-data-export フェーズ)。
// 温度・湿度それぞれの 平均/最高/最低/日較差/σ/CV と、作物の VPD 適正帯への滞在率を
// JST 暦日／JST 時間帯 (hour-of-day) でバケット化する。
//
// device_show.go の dailyStatRows / device_show_vpd.go の vpdHourlyRows と同じバケット作法
// (純粋層に time を持ち込まず handler 境界で In(jst))・同じ整形ヘルパ (jstDay/formatStat/
// formatPercent/statEmptyMark/statEpsilon/vpdCropLabel) を流用する。集計の実体は internal/chart
// (Mean/MinMax/DiurnalRange/StdDev/CV/VPDSeries/TimeInRange) に委譲し、新規計算は持たない。
// 既存 device_show 版 (DailyStatRow/VPDHourlyRow) は無改修 (別 View 構造)。
package handler

import (
	"fmt"
	"strconv"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/chart"
	"github.com/HiroshiKawano/go_iot/internal/domain"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/view/component"
)

// buildReadingsReport は期間内全行＋作物から日次/時間別の集計帳票 View を組む (R4/R5/R6)。
// 適正帯 (lower, upper) は crop.VPDRange() で解決する (未設定/不正は既定 0.3〜1.5 kPa・R6.3/6.4)。
// 事前条件: rows は recorded_at 昇順 (ListSensorReadingsInRange が保証)。
// 空 rows は HasData=false の空帳票を返す (数値を捏造しない・R4.3)。CropLabel/RangeLabel は
// 空帳票でもヘッダ表示用に保持する。
func buildReadingsReport(rows []repository.SensorReading, crop domain.Crop) component.ReadingsReportView {
	lower, upper := crop.VPDRange()
	view := component.ReadingsReportView{
		CropLabel:  vpdCropLabel(crop),
		RangeLabel: formatVPDRange(lower, upper),
	}
	if len(rows) == 0 {
		return view // HasData=false・Daily/Hourly nil (空帳票)
	}
	view.HasData = true
	view.Daily = readingsDailyRows(rows, lower, upper)
	view.Hourly = readingsHourlyRows(rows, lower, upper)
	return view
}

// readingsDailyRows は生行を JST 暦日でバケット化し、各日の温湿度統計＋適正帯滞在率を
// 日付昇順で返す (R4.1)。最古日〜最新日の間で計測の無い日は欠測行 ("—") として補完し表の
// 体裁を保つ (R4.4)。dailyStatRows を温湿度＋滞在率版へ一般化したもの (pick ではなく両指標を集める)。
func readingsDailyRows(rows []repository.SensorReading, lower, upper float64) []component.ReadingsReportRow {
	if len(rows) == 0 {
		return nil
	}
	// JST 暦日キー (時刻切り捨て) で温度・湿度を同時にグルーピング。
	buckets := make(map[time.Time]*metricBucket)
	var minDay, maxDay time.Time
	for i, r := range rows {
		day := jstDay(r.RecordedAt)
		b := buckets[day]
		if b == nil {
			b = &metricBucket{}
			buckets[day] = b
		}
		b.add(r)
		if i == 0 || day.Before(minDay) {
			minDay = day
		}
		if i == 0 || day.After(maxDay) {
			maxDay = day
		}
	}
	// 最古日〜最新日を1日ずつ進め、欠測日は "—" で埋める (JST は DST 無しのため AddDate で安全)。
	var out []component.ReadingsReportRow
	for d := minDay; !d.After(maxDay); d = d.AddDate(0, 0, 1) {
		label := d.Format("2006-01-02")
		b, ok := buckets[d]
		if !ok {
			out = append(out, emptyReadingsReportRow(label))
			continue
		}
		out = append(out, b.row(label, lower, upper))
	}
	return out
}

// readingsHourlyRows は生行を JST 時刻 (hour-of-day 0-23) でバケット化し、各時間帯の温湿度統計＋
// 適正帯滞在率を時刻昇順で返す (データのある時間帯のみ・R5.1/5.3)。境界は handler で In(jst)
// (純粋層に time を持ち込まない・R5.2)。vpdHourlyRows を温湿度版へ一般化したもの。
func readingsHourlyRows(rows []repository.SensorReading, lower, upper float64) []component.ReadingsReportRow {
	if len(rows) == 0 {
		return nil
	}
	buckets := make(map[int]*metricBucket)
	for _, r := range rows {
		h := pgconv.TimestamptzToTime(r.RecordedAt).In(jst).Hour()
		b := buckets[h]
		if b == nil {
			b = &metricBucket{}
			buckets[h] = b
		}
		b.add(r)
	}
	var out []component.ReadingsReportRow
	for h := 0; h < 24; h++ {
		b, ok := buckets[h]
		if !ok {
			continue // データのある時間帯のみ (vpdHourlyRows 作法)
		}
		out = append(out, b.row(fmt.Sprintf("%02d時", h), lower, upper))
	}
	return out
}

// metricBucket は1バケット分の温度・湿度の float 列を蓄える (集計直前の中間表現)。
type metricBucket struct {
	temps []float64
	hums  []float64
}

// add は計測行から温度・湿度を取り出し float 列へ積む (pgconv 境界変換)。
func (b *metricBucket) add(r repository.SensorReading) {
	b.temps = append(b.temps, pgconv.NumericToFloat(r.Temperature))
	b.hums = append(b.hums, pgconv.NumericToFloat(r.Humidity))
}

// row はバケットの温湿度列から帳票行を整形する (統計は chart へ委譲・単位は列見出し側)。
// 適正帯滞在率はバケットの VPD 系列のうち [lower,upper] に在帯した割合 (瞬間値/平均ではない・R6.2)。
func (b *metricBucket) row(bucket string, lower, upper float64) component.ReadingsReportRow {
	tMin, tMax := chart.MinMax(b.temps)
	hMin, hMax := chart.MinMax(b.hums)
	vpd := chart.VPDSeries(b.temps, b.hums)
	return component.ReadingsReportRow{
		Bucket:      bucket,
		TempAvg:     formatStat(chart.Mean(b.temps), ""),
		TempMax:     formatStat(tMax, ""),
		TempMin:     formatStat(tMin, ""),
		TempDiurnal: formatStat(chart.DiurnalRange(b.temps), ""),
		TempSigma:   formatStat(chart.StdDev(b.temps), ""),
		TempCV:      formatCV(b.temps),
		HumAvg:      formatStat(chart.Mean(b.hums), ""),
		HumMax:      formatStat(hMax, ""),
		HumMin:      formatStat(hMin, ""),
		HumDiurnal:  formatStat(chart.DiurnalRange(b.hums), ""),
		HumSigma:    formatStat(chart.StdDev(b.hums), ""),
		HumCV:       formatCV(b.hums),
		InRange:     formatPercent(chart.TimeInRange(vpd, lower, upper)),
	}
}

// formatCV は変動係数を整形する。|μ|<ε で未定義のときは "—" (既存 dailyRow と同規約)。
// 母σ=0 の単点でも μ≠0 なら CV=0.00 を返す (device_show 日次表と整合)。
func formatCV(values []float64) string {
	cv, ok := chart.CV(values, statEpsilon)
	if !ok {
		return statEmptyMark
	}
	return formatStat(cv, "")
}

// emptyReadingsReportRow は欠測バケットの行 (バケット名以外すべて "—") を返す (R4.4/5.3)。
func emptyReadingsReportRow(bucket string) component.ReadingsReportRow {
	return component.ReadingsReportRow{
		Bucket:      bucket,
		TempAvg:     statEmptyMark,
		TempMax:     statEmptyMark,
		TempMin:     statEmptyMark,
		TempDiurnal: statEmptyMark,
		TempSigma:   statEmptyMark,
		TempCV:      statEmptyMark,
		HumAvg:      statEmptyMark,
		HumMax:      statEmptyMark,
		HumMin:      statEmptyMark,
		HumDiurnal:  statEmptyMark,
		HumSigma:    statEmptyMark,
		HumCV:       statEmptyMark,
		InRange:     statEmptyMark,
	}
}

// formatVPDRange は適正帯の下限〜上限を "0.40〜1.20 kPa" に整形する (帳票ヘッダ表示用)。
func formatVPDRange(lower, upper float64) string {
	return strconv.FormatFloat(lower, 'f', 2, 64) + "〜" + strconv.FormatFloat(upper, 'f', 2, 64) + " kPa"
}
