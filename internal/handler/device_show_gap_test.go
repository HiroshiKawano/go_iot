package handler

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/chart"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// device_show_gap_test.go は device-show の欠測ギャップ可視化配線 (applyGapGrid 純変換と
// buildChartArea 統合) を DB 非依存で検証する (data-quality-meta タスク5.1)。

// ---- 5.1 applyGapGrid (拡張グリッド純変換) ----------------------------------

func TestApplyGapGrid_欠測スロットを挿入し系列とLabelsを揃える(t *testing.T) {
	// 3点・点1の後に2スロット欠測。
	spec := chart.ChartSpec{
		Labels:    []string{"a", "b", "c"},
		Unit:      "℃",
		Color:     "#000",
		Raw:       []float64{10, 20, 30},
		SMA:       []float64{10, 15, 25},
		BandLower: []float64{8, 13, 23},
		BandWidth: []float64{4, 4, 4},
		Deviation: []*float64{f64(1), f64(2), f64(3)},
	}
	slotsAfter := []int{0, 2, 0} // 点1の後に2スロット

	out := applyGapGrid(spec, slotsAfter)

	// 拡張後の長さ = 元3 + 2スロット = 5。全系列が Labels と同長。
	wantLen := 5
	if len(out.Labels) != wantLen {
		t.Fatalf("Labels 長=%d, want %d", len(out.Labels), wantLen)
	}
	for name, got := range map[string]int{
		"RawNullable": len(out.RawNullable),
		"SMA":         len(out.SMA),
		"BandLower":   len(out.BandLower),
		"BandWidth":   len(out.BandWidth),
		"Deviation":   len(out.Deviation),
	} {
		if got != wantLen {
			t.Errorf("%s 長=%d, want %d (Labels と揃っていない)", name, got, wantLen)
		}
	}

	// 欠測スロット (index 2,3) は RawNullable が nil・Deviation が nil (補間しない・分断)。
	for _, i := range []int{2, 3} {
		if out.RawNullable[i] != nil {
			t.Errorf("欠測スロット index%d の RawNullable が非 nil: %v", i, *out.RawNullable[i])
		}
		if out.Deviation[i] != nil {
			t.Errorf("欠測スロット index%d の Deviation が非 nil", i)
		}
	}
	// 実点 (index 0,1,4) は値を保持。
	for _, i := range []int{0, 1, 4} {
		if out.RawNullable[i] == nil {
			t.Errorf("実点 index%d の RawNullable が nil", i)
		}
	}

	// GapBands は点1(ext index1)→点2(ext index4) の1帯。
	if len(out.GapBands) != 1 {
		t.Fatalf("GapBands 数=%d, want 1: %+v", len(out.GapBands), out.GapBands)
	}
	if out.GapBands[0].StartIdx != 1 || out.GapBands[0].EndIdx != 4 {
		t.Errorf("GapBand=%+v, want {1,4}", out.GapBands[0])
	}

	// 元 spec は破壊されない (イミュータブル)。
	if len(spec.Labels) != 3 || len(spec.RawNullable) != 0 {
		t.Errorf("元 spec が破壊された: Labels=%d RawNullable=%d", len(spec.Labels), len(spec.RawNullable))
	}
}

// f64 はポインタ float ヘルパ。
func f64(v float64) *float64 { return &v }

// ---- 5.1 buildChartArea 統合 (欠測あり/なし) --------------------------------

// gapRows は中央値5分に対し1区間だけ30分ギャップを含む生データ (24h)。
func gapRows() []repository.SensorReading {
	base := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	return []repository.SensorReading{
		sensorRow(1, base, 20, 60),
		sensorRow(1, base.Add(5*time.Minute), 21, 61),
		sensorRow(1, base.Add(10*time.Minute), 20, 60),
		sensorRow(1, base.Add(40*time.Minute), 22, 62), // 30分ギャップ (中央値5分)
		sensorRow(1, base.Add(45*time.Minute), 21, 61),
	}
}

func TestBuildChartArea_欠測ありで線分断とmarkAreaが載る(t *testing.T) {
	repo := showDeviceRepo()
	repo.recentReadings = gapRows()
	h := &DeviceHandler{Repo: repo}
	now := time.Date(2026, 4, 20, 1, 0, 0, 0, time.UTC)

	area, err := h.buildChartArea(context.Background(), repo.devices[1], "24h", now)
	if err != nil {
		t.Fatalf("buildChartArea() でエラー: %v", err)
	}

	if !area.HasGap {
		t.Error("HasGap=false, want true (欠測あり)")
	}
	for _, opt := range []struct {
		name string
		json string
	}{
		{"温度", area.TemperatureOptionJSON},
		{"湿度", area.HumidityOptionJSON},
	} {
		// 線分断 (connectNulls:false) と欠測区間 markArea (小文字 xAxis) が載る。
		if !strings.Contains(opt.json, `"connectNulls":false`) {
			t.Errorf("%s option に connectNulls:false が無い", opt.name)
		}
		if !strings.Contains(opt.json, `"markArea"`) {
			t.Errorf("%s option に markArea が無い", opt.name)
		}
		if !strings.Contains(opt.json, `"xAxis"`) {
			t.Errorf("%s option に小文字 xAxis キーが無い", opt.name)
		}
		if strings.Contains(opt.json, `"XAxis"`) {
			t.Errorf("%s option に大文字 XAxis が混入", opt.name)
		}
	}
}

func TestBuildChartArea_欠測なしは従来出力で不変(t *testing.T) {
	// 等間隔5分 (欠測なし) のデータ。
	base := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	rows := []repository.SensorReading{
		sensorRow(1, base, 20, 60),
		sensorRow(1, base.Add(5*time.Minute), 21, 61),
		sensorRow(1, base.Add(10*time.Minute), 20, 60),
		sensorRow(1, base.Add(15*time.Minute), 22, 62),
	}
	repo := showDeviceRepo()
	repo.recentReadings = rows
	h := &DeviceHandler{Repo: repo}
	now := time.Date(2026, 4, 20, 1, 0, 0, 0, time.UTC)

	area, err := h.buildChartArea(context.Background(), repo.devices[1], "24h", now)
	if err != nil {
		t.Fatalf("buildChartArea() でエラー: %v", err)
	}

	if area.HasGap {
		t.Error("HasGap=true, want false (欠測なし)")
	}
	// 欠測なしは gap 由来の出力が一切載らない (後方互換)。
	if strings.Contains(area.TemperatureOptionJSON, "connectNulls") || strings.Contains(area.TemperatureOptionJSON, "markArea") {
		t.Errorf("欠測なしなのに gap 出力が載っている (後方互換違反): %s", area.TemperatureOptionJSON)
	}

	// 期待 = 従来パイプライン (overlaySpec→ChartOptionJSON・gap 配線なし) とバイト完全一致。
	window := smaWindowFor("24h")
	labels := make([]string, len(rows))
	temps := make([]float64, len(rows))
	hums := make([]float64, len(rows))
	for i, r := range rows {
		labels[i] = rawLabelFor("24h")(r.RecordedAt)
		temps[i] = pgconv.NumericToFloat(r.Temperature)
		hums[i] = pgconv.NumericToFloat(r.Humidity)
	}
	wantTemp, _ := chart.ChartOptionJSON(overlaySpec(labels, temps, tempChartUnit, tempLineColor, window))
	wantHum, _ := chart.ChartOptionJSON(overlaySpec(labels, hums, humidityChartUnit, humidityLineColor, window))
	if area.TemperatureOptionJSON != wantTemp {
		t.Error("欠測なしの温度 option が従来出力と一致しない (無回帰崩れ)")
	}
	if area.HumidityOptionJSON != wantHum {
		t.Error("欠測なしの湿度 option が従来出力と一致しない (無回帰崩れ)")
	}
}
