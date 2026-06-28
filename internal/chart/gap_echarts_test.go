package chart

import (
	"encoding/json"
	"strings"
	"testing"
)

// seriesHead は option JSON を generic map へ unmarshal し series[0] を取り出すヘルパ。
func seriesHead(t *testing.T, out string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("option JSON が妥当でない: %v\noption=%s", err, out)
	}
	series, ok := m["series"].([]any)
	if !ok || len(series) == 0 {
		t.Fatalf("series[0] が無い\noption=%s", out)
	}
	head, ok := series[0].(map[string]any)
	if !ok {
		t.Fatalf("series[0] が map でない\noption=%s", out)
	}
	return head
}

// ---- 3.1 RawNullable による線分断（connectNulls:false 明示） -----------------

// RawNullable に欠測スロット(nil)を含めると series[0] data が null 点で分断され、
// connectNulls:false が明示されること（要件 5.1/5.4）。
func TestChartOptionJSON_RawNullableBreaksLine(t *testing.T) {
	v0, v2 := 20.0, 18.0
	spec := baselineSpec()
	spec.RawNullable = []*float64{&v0, nil, &v2} // 中央が欠測

	out, err := ChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("ChartOptionJSON() でエラー: %v", err)
	}

	// connectNulls:false を明示（欠測スロットで線を繋がない）。
	if !strings.Contains(out, `"connectNulls":false`) {
		t.Errorf("connectNulls:false が明示されていない\noption=%s", out)
	}

	head := seriesHead(t, out)
	data, ok := head["data"].([]any)
	if !ok || len(data) != 3 {
		t.Fatalf("series[0].data が3要素でない: %v", head["data"])
	}
	// 端点は値を持ち、中央の欠測スロットは value を持たない（ECharts null 点）。
	if d0, _ := data[0].(map[string]any); d0["value"] == nil {
		t.Errorf("data[0] に value が無い: %v", data[0])
	}
	if d2, _ := data[2].(map[string]any); d2["value"] == nil {
		t.Errorf("data[2] に value が無い: %v", data[2])
	}
	if mid, _ := data[1].(map[string]any); mid["value"] != nil {
		t.Errorf("欠測スロット data[1] に value があってはいけない（null 点）: %v", data[1])
	}

	// 欠測補間をしない＝穴を埋めた値を生成しない（要件 5.4）。
	// markPoint(max/min) は維持される（無回帰）。
	if mp, _ := head["markPoint"]; mp == nil {
		t.Errorf("series[0] の markPoint(max/min) が失われている\noption=%s", out)
	}
}

// RawNullable 未設定時は connectNulls キーを一切出さない（後方互換の不変条件・新フィールド非漏洩）。
func TestChartOptionJSON_NoConnectNullsWhenRawNullableAbsent(t *testing.T) {
	out, err := ChartOptionJSON(baselineSpec())
	if err != nil {
		t.Fatalf("ChartOptionJSON() でエラー: %v", err)
	}
	if strings.Contains(out, "connectNulls") {
		t.Errorf("RawNullable 未設定なのに connectNulls が出ている（後方互換違反）\noption=%s", out)
	}
	if strings.Contains(out, "markArea") {
		t.Errorf("GapBands 未設定なのに markArea が出ている（後方互換違反）\noption=%s", out)
	}
}

// ---- 3.2 連続欠測区間の xAxis 範囲 markArea 自前注入 ------------------------

// GapBands を設定すると series[0] へ小文字 xAxis キーの markArea が注入されること（要件 5.2）。
func TestChartOptionJSON_GapMarkAreaInjected(t *testing.T) {
	spec := baselineSpec()
	spec.GapBands = []GapBand{{StartIdx: 1, EndIdx: 2}}

	out, err := ChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("ChartOptionJSON() でエラー: %v", err)
	}

	// ECharts 準拠の小文字 xAxis キーで markArea が出る。大文字 XAxis は混入しない（D-1 回避）。
	if !strings.Contains(out, `"markArea"`) {
		t.Fatalf("markArea が注入されていない\noption=%s", out)
	}
	if !strings.Contains(out, `"xAxis"`) {
		t.Errorf("markArea に小文字 xAxis キーが無い\noption=%s", out)
	}
	if strings.Contains(out, `"XAxis"`) {
		t.Errorf("ECharts 非準拠の大文字 XAxis が混入している\noption=%s", out)
	}

	// 構造: series[0].markArea.silent=true・data は1帯・[start,end] の xAxis ペアで始点に塗り色。
	head := seriesHead(t, out)
	ma, ok := head["markArea"].(map[string]any)
	if !ok {
		t.Fatalf("series[0].markArea が無い\noption=%s", out)
	}
	if silent, _ := ma["silent"].(bool); !silent {
		t.Errorf("markArea.silent=true 想定\noption=%s", out)
	}
	data, ok := ma["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("markArea.data が1帯でない: %v", ma["data"])
	}
	pair, _ := data[0].([]any)
	if len(pair) != 2 {
		t.Fatalf("帯が [start,end] ペアでない: %v", data[0])
	}
	start, _ := pair[0].(map[string]any)
	end, _ := pair[1].(map[string]any)
	if sx, _ := start["xAxis"].(float64); sx != 1 {
		t.Errorf("start.xAxis=%v, want 1", start["xAxis"])
	}
	if ex, _ := end["xAxis"].(float64); ex != 2 {
		t.Errorf("end.xAxis=%v, want 2", end["xAxis"])
	}
	if _, has := start["itemStyle"]; !has {
		t.Errorf("帯の始点に itemStyle（塗り色）が無い: %v", start)
	}

	// markPoint(max/min) は維持される（markArea は併存・無回帰）。
	if head["markPoint"] == nil {
		t.Errorf("series[0] の markPoint が失われている\noption=%s", out)
	}
}

// 複数の連続欠測区間がそれぞれ独立した帯として注入されること。
func TestChartOptionJSON_MultipleGapBands(t *testing.T) {
	spec := ChartSpec{
		Labels: []string{"0", "1", "2", "3", "4"},
		Unit:   "℃",
		Color:  tempColorTest,
		Raw:    []float64{1, 2, 3, 4, 5},
		GapBands: []GapBand{
			{StartIdx: 0, EndIdx: 1},
			{StartIdx: 3, EndIdx: 4},
		},
	}
	out, err := ChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("ChartOptionJSON() でエラー: %v", err)
	}
	head := seriesHead(t, out)
	ma, _ := head["markArea"].(map[string]any)
	data, ok := ma["data"].([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("markArea.data が2帯でない: %v", ma["data"])
	}
}

// injectGapMarkArea は空 bands では原文をそのまま返す（要件 5.2・後方互換）。
func TestInjectGapMarkArea_EmptyBandsReturnsOriginal(t *testing.T) {
	sample := `{"series":[{"type":"line","data":[]}]}`
	got, err := injectGapMarkArea(sample, nil)
	if err != nil {
		t.Fatalf("injectGapMarkArea() でエラー: %v", err)
	}
	if got != sample {
		t.Errorf("nil bands で原文不変のはず: got %q, want %q", got, sample)
	}
	got2, err := injectGapMarkArea(sample, []GapBand{})
	if err != nil {
		t.Fatalf("injectGapMarkArea() でエラー: %v", err)
	}
	if got2 != sample {
		t.Errorf("空スライスで原文不変のはず: got %q, want %q", got2, sample)
	}
}

// injectGapMarkArea は不正な option JSON・series 欠落に対しエラーを返す（防御的契約）。
func TestInjectGapMarkArea_Errors(t *testing.T) {
	bands := []GapBand{{StartIdx: 0, EndIdx: 1}}
	tests := []struct {
		name  string
		input string
	}{
		{"不正なJSON", `{not json`},
		{"series が無い", `{"xAxis":{}}`},
		{"series が空", `{"series":[]}`},
		{"series[0] が map でない", `{"series":[42]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := injectGapMarkArea(tt.input, bands); err == nil {
				t.Errorf("エラーを期待したが nil: input=%s", tt.input)
			}
		})
	}
}

// 欠測ギャップ注入後も < > & は \uXXXX 化され HTML 安全であること（§10-E・要件 5）。
func TestChartOptionJSON_GapHTMLSafe(t *testing.T) {
	v0, v1 := 20.0, 25.5
	spec := ChartSpec{
		Labels:      []string{`</script><b>x&y`, "12:00"},
		Unit:        "℃",
		Color:       tempColorTest,
		Raw:         []float64{20.0, 25.5},
		RawNullable: []*float64{&v0, &v1},
		GapBands:    []GapBand{{StartIdx: 0, EndIdx: 1}},
	}
	out, err := ChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("ChartOptionJSON() でエラー: %v", err)
	}
	for _, raw := range []string{"<", ">", "</script>", "<b>"} {
		if strings.Contains(out, raw) {
			t.Errorf("HTML 安全でない: 生の %q が漏れている\noption=%s", raw, out)
		}
	}
	if !strings.Contains(out, "\\u003c") {
		t.Errorf("`<` がエスケープ形(\\u003c)で保持されていない\noption=%s", out)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Errorf("返却が妥当な JSON でない: %v\noption=%s", err, out)
	}
}
