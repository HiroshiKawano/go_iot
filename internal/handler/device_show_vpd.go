// device_show_vpd.go はデバイス詳細画面の VPD (飽差) 適正帯ダッシュボードのパネル組立を担う。
// 温湿度データから読み取り時に VPD 系列・適正帯滞在率・時間帯別逸脱・VPD移動平均を算出し、
// 作物別しきい値 (domain.Crop.VPDRange) を適用した VPD パネル View を組む (研究用・保存しない)。
// 純粋な算出は internal/chart へ委譲し、ここは時刻バケット (JST 時間帯) と表示整形の handler 境界に集中する。
package handler

import (
	"fmt"
	"math"
	"strconv"

	"github.com/HiroshiKawano/go_iot/internal/chart"
	"github.com/HiroshiKawano/go_iot/internal/domain"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/view/component"
)

const (
	// vpdLineColor は VPD 線・滞在率バーの基準色。mocks/html/style.css の --color-vpd と対応し、
	// 温度橙 (#e8590c)/湿度青 (#1971c2) と区別する。
	vpdLineColor = "#0ca678"
	// vpdYMaxHeadroom は markArea の乾きすぎゾーン (高VPD側) を常に可視にする y 上限の余裕 [kPa]。
	vpdYMaxHeadroom = 0.3
	// 時間帯別逸脱・最大逸脱の方向ラベル。無逸脱は statEmptyMark ("—") を用いる。
	vpdDirDry = "乾き"
	vpdDirWet = "湿り"
)

// buildVPDPanel は整形済みの温湿度列＋生行＋作物から VPD パネル View を組む (R4/R5/R6/R7)。
// labels/temps/hums は buildChartArea が整形済み (温湿度グラフと共通の時刻ラベル列)。
// rows は時間帯バケット (JST 時刻) 用の生行・crop は適正帯解決用・period は VPD移動平均の窓幅用。
// crop.VPDRange() で適正帯 (未設定は既定 0.3-1.5) を解決し、VPD 系列→滞在率→SMA→option を構築する。
// 時刻を要する hour-of-day バケットは handler 境界で行い純粋層に時刻を持ち込まない (dailyStatRows と同作法)。
// 計測 0 件 (空系列) のときは Card を "—"・Hourly 空・OptionJSON 空のパネルを返す (呼出側は HasData で非表示)。
func buildVPDPanel(labels []string, temps, hums []float64, rows []repository.SensorReading, crop domain.Crop, period string) (component.VPDPanelView, error) {
	lower, upper := crop.VPDRange()
	view := component.VPDPanelView{
		Color:      vpdLineColor,
		CropLabel:  vpdCropLabel(crop),
		LowerLabel: formatVPD(lower),
		UpperLabel: formatVPD(upper),
	}

	vpd := chart.VPDSeries(temps, hums)
	if len(vpd) == 0 {
		view.Card = emptyVPDCard()
		return view, nil
	}

	ratio := chart.TimeInRange(vpd, lower, upper) // 0..1 (滞在率の素)
	view.InRangeRatio = ratio

	// VPD移動平均 (既存 SMA・期間別窓幅を共用) と y 上限 (乾きすぎゾーン可視化) を算出し option を構築。
	sma := chart.SMA(vpd, smaWindowFor(period))
	_, vpdMax := chart.MinMax(vpd)
	optionJSON, err := chart.VPDChartOptionJSON(chart.VPDChartSpec{
		Labels: labels,
		Color:  vpdLineColor,
		VPD:    vpd,
		SMA:    sma,
		Lower:  lower,
		Upper:  upper,
		YMax:   vpdYMax(vpdMax, upper),
	})
	if err != nil {
		return component.VPDPanelView{}, err
	}
	view.OptionJSON = optionJSON

	view.Card = component.VPDCardView{
		CurrentVPD:   formatVPD(vpd[len(vpd)-1]),       // 現在=最新点
		AverageVPD:   formatVPD(chart.Mean(vpd)),       // 期間平均
		TimeInRange:  formatPercent(ratio),             // 適正帯滞在率
		MaxDeviation: vpdMaxDeviation(vpd, lower, upper), // 最大逸脱 (量+方向)
	}
	view.Hourly = vpdHourlyRows(rows, vpd, lower, upper)
	return view, nil
}

// vpdCropLabel は作物名を返す。未設定・不正作物は "既定" (既定適正帯で表示している旨を明示)。
func vpdCropLabel(crop domain.Crop) string {
	if crop.Valid() {
		return crop.Label()
	}
	return "既定"
}

// vpdYMax は y 軸上限を ceil(max(観測最大, 上限) + 余裕) で算出する (3ゾーンを常時可視に)。
func vpdYMax(observedMax, upper float64) float64 {
	base := observedMax
	if upper > base {
		base = upper
	}
	return math.Ceil(base + vpdYMaxHeadroom)
}

// vpdMaxDeviation は適正帯から最も外れた量と方向を整形する ("+0.40 kPa（乾き）"/"-0.30 kPa（湿り）")。
// VPD は乾燥度指標: 上限超 (高VPD) = 乾きすぎ "+"、下限割れ (低VPD=多湿) = 湿りすぎ "-"。
// 全点が適正帯内 (逸脱なし) のときは statEmptyMark ("—")。
func vpdMaxDeviation(vpd []float64, lower, upper float64) string {
	maxDev := 0.0
	dir := ""
	for _, v := range vpd {
		if v < lower {
			if d := lower - v; d > maxDev {
				maxDev, dir = d, vpdDirWet // 低VPD=多湿=湿りすぎ
			}
		} else if v > upper {
			if d := v - upper; d > maxDev {
				maxDev, dir = d, vpdDirDry // 高VPD=乾燥=乾きすぎ
			}
		}
	}
	if dir == "" {
		return statEmptyMark
	}
	sign := "+" // 上限超 (乾きすぎ) は "+"
	if dir == vpdDirWet {
		sign = "-" // 下限割れ (湿りすぎ) は "-"
	}
	return sign + strconv.FormatFloat(maxDev, 'f', 2, 64) + " kPa（" + dir + "）"
}

// vpdHourlyRows は生行を JST 時刻 (hour-of-day 0-23) でバケット化し、各時間帯の平均VPD・在帯率・
// 主な逸脱方向を時刻昇順で返す (データのある時間帯のみ・R6.1)。vpd は rows と同順 (buildVPDPanel が整形)。
func vpdHourlyRows(rows []repository.SensorReading, vpd []float64, lower, upper float64) []component.VPDHourlyRow {
	n := len(rows)
	if len(vpd) < n {
		n = len(vpd)
	}
	if n == 0 {
		return nil
	}
	buckets := make(map[int][]float64)
	for i := 0; i < n; i++ {
		h := pgconv.TimestamptzToTime(rows[i].RecordedAt).In(jst).Hour()
		buckets[h] = append(buckets[h], vpd[i])
	}
	var out []component.VPDHourlyRow
	for h := 0; h < 24; h++ {
		vals, ok := buckets[h]
		if !ok {
			continue
		}
		out = append(out, component.VPDHourlyRow{
			Hour:           fmt.Sprintf("%02d:00", h),
			AvgVPD:         strconv.FormatFloat(chart.Mean(vals), 'f', 2, 64), // 単位なし (列見出しに kPa)
			InRangePercent: formatPercent(chart.TimeInRange(vals, lower, upper)),
			Direction:      vpdDirection(vals, lower, upper),
		})
	}
	return out
}

// vpdDirection はバケット内の逸脱点の多数決で主な逸脱方向を返す。
// VPD は乾燥度指標: 上限超 (高VPD・above) = 乾きすぎ、下限割れ (低VPD=多湿・below) = 湿りすぎ。
// 逸脱点が無ければ statEmptyMark ("—")。同数のときは湿り優先 (below>=above)。
func vpdDirection(vals []float64, lower, upper float64) string {
	below, above := 0, 0
	for _, v := range vals {
		if v < lower {
			below++
		} else if v > upper {
			above++
		}
	}
	if below == 0 && above == 0 {
		return statEmptyMark
	}
	if above > below {
		return vpdDirDry // 高VPD多数=乾きすぎ
	}
	return vpdDirWet // 低VPD多数 (同数含む)=湿りすぎ
}

// formatVPD は VPD 値を小数2桁＋単位 "X.XX kPa" に整形する (数値カード・適正帯表示用)。
func formatVPD(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64) + " kPa"
}

// formatPercent は割合 (0..1) を整数パーセント "NN%" に整形する (滞在率・在帯率用)。
func formatPercent(ratio float64) string {
	return strconv.FormatFloat(ratio*100, 'f', 0, 64) + "%"
}

// emptyVPDCard はデータ未到着時の VPD 数値カード (全項目 "—") を返す。
func emptyVPDCard() component.VPDCardView {
	return component.VPDCardView{
		CurrentVPD: statEmptyMark, AverageVPD: statEmptyMark,
		TimeInRange: statEmptyMark, MaxDeviation: statEmptyMark,
	}
}
