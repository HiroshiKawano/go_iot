// device_show_sma_window.go は sma-window-select（日スケール SMA 窓のユーザー選択）の
// handler 境界ヘルパを担う。device-show 温湿度グラフへ「数日〜2週間」スケールの単純移動平均
// を凡例トグルの追加系列（既定オフ）として上載せするための、窓集合の決定と日数→点数換算を行う。
//
// 純粋計算（SMA・中央値）は internal/chart（最下流純粋層）へ委譲し、本ファイルは点数換算・
// 窓ラベルの組立という handler 境界の関心に集中する（structure.md の依存方向）。
package handler

import (
	"fmt"
	"math"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/chart"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
)

const (
	// defaultPointsPerDay はサンプリング間隔が算出不能なときの点数/日フォールバック。
	// 約5分間隔＝288点/日（分析アイデアメモ第4章「5分維持」決定）。間隔が読めても med<=0 のときも本値。
	defaultPointsPerDay = 288.0
	// secondsPerDay は 1 日の秒数（86400）。点数/日 = 86400/中央間隔秒。
	secondsPerDay = 86400.0
)

// dayWindow は 1 本の日スケール SMA 窓を表す（窓の長さ＝日数と、凡例ラベルの組）。
// Label は中立な「移動平均 N日」（売買シグナル/交差語を含まない・R4.2）。
type dayWindow struct {
	Days  int    // 窓の長さ（日）
	Label string // 凡例ラベル・系列名「移動平均 N日」
}

// makeDayWindow は日数から日スケール窓を組む（ラベル書式を一元化）。
func makeDayWindow(days int) dayWindow {
	return dayWindow{Days: days, Label: fmt.Sprintf("移動平均 %d日", days)}
}

// dayScaleWindowsFor は表示期間に応じた日スケール SMA 窓集合を返す（最大3本・R2.4）。
//   - "7d"  → {3日, 7日}
//   - "30d" → {3日, 7日, 14日}
//   - "24h"/"3d"/その他 → nil（短期ビューでは窓を出さず既存パス完全分岐・5.2 縮退）
//
// 窓は当該ビューの可視スパン以下のもののみとし、1日窓は 30d ビューの既存点数窓 SMA
// （288点≒1日）が実質カバーするため除外する（重複回避・クラッタ抑制）。
func dayScaleWindowsFor(period string) []dayWindow {
	switch period {
	case "7d":
		return []dayWindow{makeDayWindow(3), makeDayWindow(7)}
	case "30d":
		return []dayWindow{makeDayWindow(3), makeDayWindow(7), makeDayWindow(14)}
	default: // "24h"/"3d"/その他は窓なし
		return nil
	}
}

// estimatePointsPerDay は実測間隔の中央値から 1 日あたりの点数（86400/中央値）を推定する。
// 行不足や中央値<=0（算出不能）は defaultPointsPerDay（288・5分）へフォールバックする。
// 中央値採用で台風スパイク等の外れ値間隔に頑健・将来の間隔変更にも追従する（⑤）。
func estimatePointsPerDay(rows []repository.SensorReading) float64 {
	secs := intervalSeconds(rows) // len(rows)<2 は nil（間隔が定義できない）
	if len(secs) == 0 {
		return defaultPointsPerDay
	}
	med := chart.Median(secs)
	if med <= 0 {
		return defaultPointsPerDay
	}
	return secondsPerDay / med
}

// maxWindowDays は窓集合の最長日数を返す（空なら 0＝ルックバックなし＝既存パス完全保持）。
func maxWindowDays(windows []dayWindow) int {
	max := 0
	for _, w := range windows {
		if w.Days > max {
			max = w.Days
		}
	}
	return max
}

// visibleStartIndex は ASC（recorded_at 昇順）の rows で recorded_at >= since となる最初の index を返す。
// 該当なしは len(rows)（可視窓は空スライス）。ルックバック分（since 未満）を可視窓から除外する。
func visibleStartIndex(rows []repository.SensorReading, since time.Time) int {
	for i, r := range rows {
		if !pgconv.TimestamptzToTime(r.RecordedAt).Before(since) {
			return i
		}
	}
	return len(rows)
}

// daySMASeriesFor は全系列値 fullValues から各窓の日スケール SMA を算出し、可視窓へスライスした
// 追加系列を返す。点数換算 = max(1, round(Days×ppd))。計算系列は chartRound2 で2桁丸めしてから
// [visibleStart:] でスライスする（左端は手前 Days 日分の実データを用いた真の日スケール平均・5.1）。
// 事後条件: 各 Values 長 == len(fullValues)-visibleStart（=可視窓長）。窓空は nil。
func daySMASeriesFor(windows []dayWindow, fullValues []float64, ppd float64, visibleStart int) []chart.DaySMASeries {
	if len(windows) == 0 {
		return nil
	}
	out := make([]chart.DaySMASeries, 0, len(windows))
	for _, w := range windows {
		pts := int(math.Round(float64(w.Days) * ppd))
		if pts < 1 {
			pts = 1 // 可視スパン以下の窓でも最低1点（線を欠落させない）
		}
		full := chartRound2(chart.SMA(fullValues, pts)) // 計算系列は2桁丸め（表示クラッタ抑制）
		out = append(out, chart.DaySMASeries{Label: w.Label, Values: full[visibleStart:]})
	}
	return out
}
