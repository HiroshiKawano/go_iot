package chart

import (
	"encoding/json"
	"fmt"

	"github.com/go-echarts/go-echarts/v2/opts"
)

// 欠測区間ハイライト帯の塗り色（markArea・薄い灰系）。
// 主役の生実測線の可読性を損なわない控えめな表現とする（要件 5.3）。
// グラフ内部描画ゆえモック反映対象外（feedback_mock_graph_rendering_exception）。
const gapBandColor = "rgba(120,120,120,0.12)"

// nullableLineData は欠測スロット nil を含む []*float64 を opts.LineData 列へ変換する。
// nil（欠測）は Value 未設定（omitempty で値なし＝ECharts は null 点として扱い線を分断する）。
// deviationData の nil 欠落点処理を生実測線へ適用したもの（要件 5.1/5.4・補間しない）。
func nullableLineData(values []*float64) []opts.LineData {
	data := make([]opts.LineData, len(values))
	for i, p := range values {
		if p != nil {
			data[i] = opts.LineData{Value: *p}
		}
		// nil は opts.LineData{}（Value 未設定）のまま＝ECharts null 点＝補間しない。
	}
	return data
}

// injectGapMarkArea は option JSON の series[0] へ連続欠測区間の xAxis 範囲 markArea を注入する。
// injectVPDMarkArea を踏襲し JSON→map→series[0] へ ECharts 準拠の小文字 `xAxis` キー markArea を
// 自前注入→再 Marshal で HTML 安全化する。空 bands は無注入で原文をそのまま返す（要件 5.2）。
func injectGapMarkArea(optionJSON string, bands []GapBand) (string, error) {
	if len(bands) == 0 {
		return optionJSON, nil
	}
	var optionMap map[string]any
	if err := json.Unmarshal([]byte(optionJSON), &optionMap); err != nil {
		return "", fmt.Errorf("chart: 欠測 option の再構築に失敗: %w", err)
	}
	series, ok := optionMap["series"].([]any)
	if !ok || len(series) == 0 {
		return "", fmt.Errorf("chart: 欠測 option に series[0] が無い")
	}
	head, ok := series[0].(map[string]any)
	if !ok {
		return "", fmt.Errorf("chart: 欠測 option の series[0] が想定形でない")
	}

	zones := make([]any, 0, len(bands))
	for _, b := range bands {
		zones = append(zones, gapZone(b.StartIdx, b.EndIdx))
	}
	head["markArea"] = map[string]any{
		"silent": true,
		"data":   zones,
	}

	// 再シリアライズ（SetEscapeHTML=true 既定）で < > & を \uXXXX 化し HTML 安全化する。
	out, err := json.Marshal(optionMap)
	if err != nil {
		return "", fmt.Errorf("chart: 欠測 option の HTML 安全化に失敗: %w", err)
	}
	return string(out), nil
}

// gapZone は欠測区間 [startIdx, endIdx] の xAxis 範囲 markArea ゾーン（薄い灰帯1本）を構築する。
// 始点に itemStyle.color（塗り色）を持たせ、両端は ECharts 準拠の小文字 xAxis キーで指定する。
func gapZone(startIdx, endIdx int) []any {
	return []any{
		map[string]any{"xAxis": startIdx, "itemStyle": map[string]any{"color": gapBandColor}},
		map[string]any{"xAxis": endIdx},
	}
}
