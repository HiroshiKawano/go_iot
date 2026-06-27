package chart

import (
	"encoding/json"
	"strings"
	"testing"
)

const vpdColorTest = "#2f9e44"

// baselineVPDSpec は VPD 実測線のみの VPDChartSpec（決定的テスト用）。
// 適正帯 0.4–1.2 kPa・y 上限 3.0 kPa を固定し markArea 3ゾーンを一意に検証できる。
func baselineVPDSpec() VPDChartSpec {
	return VPDChartSpec{
		Labels: []string{"00:00", "12:00", "23:00"},
		Color:  vpdColorTest,
		VPD:    []float64{0.5, 1.6, 0.9},
		Lower:  0.4,
		Upper:  1.2,
		YMax:   3.0,
	}
}

// vpdOptDoc は VPD option JSON の構造アサート用スキーマ。
type vpdOptDoc struct {
	Legend struct {
		Show     *bool           `json:"show"`
		Data     []string        `json:"data"`
		Selected map[string]bool `json:"selected"`
	} `json:"legend"`
	YAxis  []map[string]any `json:"yAxis"`
	Series []struct {
		Name      string `json:"name"`
		Type      string `json:"type"`
		LineStyle *struct {
			Color string `json:"color"`
		} `json:"lineStyle"`
		ShowSymbol *bool           `json:"showSymbol"`
		MarkPoint  json.RawMessage `json:"markPoint"`
		MarkArea   *struct {
			Silent *bool `json:"silent"`
			Data   [][]struct {
				YAxis     *float64        `json:"yAxis"`
				ItemStyle json.RawMessage `json:"itemStyle"`
			} `json:"data"`
		} `json:"markArea"`
	} `json:"series"`
}

func parseVPDOption(t *testing.T, out string) vpdOptDoc {
	t.Helper()
	var doc vpdOptDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("VPD option JSON が妥当でない: %v\noption=%s", err, out)
	}
	return doc
}

// approx は浮動小数の許容比較。
func approx(a, b float64) bool {
	d := a - b
	return d > -1e-9 && d < 1e-9
}

// ---- VPD 実測線（主役）と markArea 3ゾーン ---------------------------------

func TestVPDChartOptionJSON_BaselineLineAndZones(t *testing.T) {
	out, err := VPDChartOptionJSON(baselineVPDSpec())
	if err != nil {
		t.Fatalf("VPDChartOptionJSON() でエラー: %v", err)
	}
	doc := parseVPDOption(t, out)

	// VPD 実測線のみ → series はちょうど1本（SMA 未指定）。
	if len(doc.Series) != 1 {
		t.Fatalf("series 数 = %d, want 1（VPD 実測線のみ）\noption=%s", len(doc.Series), out)
	}
	s0 := doc.Series[0]
	if s0.Type != "line" {
		t.Errorf("series[0].type = %q, want line", s0.Type)
	}
	if s0.Name != "VPD" {
		t.Errorf("series[0].name = %q, want VPD", s0.Name)
	}
	if s0.LineStyle == nil || s0.LineStyle.Color != vpdColorTest {
		t.Errorf("series[0] の基準色 %s が反映されていない\noption=%s", vpdColorTest, out)
	}
	// series[0] に markPoint(max/min)。
	mp := string(s0.MarkPoint)
	if !strings.Contains(mp, `"max"`) || !strings.Contains(mp, `"min"`) {
		t.Errorf("series[0] の markPoint に max/min が無い: %s", mp)
	}

	// 適正帯 markArea は series[0] に存在し、3ゾーン。
	if s0.MarkArea == nil {
		t.Fatalf("series[0] に markArea が無い\noption=%s", out)
	}
	if len(s0.MarkArea.Data) != 3 {
		t.Fatalf("markArea のゾーン数 = %d, want 3\noption=%s", len(s0.MarkArea.Data), out)
	}
	// 各ゾーンは [start,end] のペアで、しきい値（0→lower→upper→YMax）が反映されている。
	wantBounds := [][2]float64{
		{0, 0.4},   // 湿りすぎ [0, lower]（低VPD=多湿）
		{0.4, 1.2}, // 適正 [lower, upper]
		{1.2, 3.0}, // 乾きすぎ [upper, YMax]（高VPD=乾燥）
	}
	for i, wb := range wantBounds {
		pair := s0.MarkArea.Data[i]
		if len(pair) != 2 || pair[0].YAxis == nil || pair[1].YAxis == nil {
			t.Fatalf("markArea[%d] が [start,end] の yAxis ペアでない: %+v", i, pair)
		}
		if !approx(*pair[0].YAxis, wb[0]) || !approx(*pair[1].YAxis, wb[1]) {
			t.Errorf("markArea[%d] = [%v,%v], want [%v,%v]", i, *pair[0].YAxis, *pair[1].YAxis, wb[0], wb[1])
		}
		// 始点に塗り色（itemStyle）。
		if len(pair[0].ItemStyle) == 0 {
			t.Errorf("markArea[%d] の始点に itemStyle（塗り色）が無い", i)
		}
	}
}

// markArea のキーは ECharts 準拠の小文字 `yAxis` で、go-echarts 不具合の `YAxis`（大文字）が
// 混入しないこと（D-1 回避を固定）。
func TestVPDChartOptionJSON_LowercaseYAxisKey(t *testing.T) {
	out, err := VPDChartOptionJSON(baselineVPDSpec())
	if err != nil {
		t.Fatalf("VPDChartOptionJSON() でエラー: %v", err)
	}
	if !strings.Contains(out, `"yAxis"`) {
		t.Errorf("markArea に小文字 `yAxis` キーが無い\noption=%s", out)
	}
	if strings.Contains(out, `"YAxis"`) {
		t.Errorf("ECharts 非準拠の大文字 `YAxis` が混入している（D-1）\noption=%s", out)
	}
}

// y 軸は min:0・max:YMax 固定で3ゾーンを常時可視にする。
func TestVPDChartOptionJSON_YAxisFixedRange(t *testing.T) {
	out, err := VPDChartOptionJSON(baselineVPDSpec())
	if err != nil {
		t.Fatalf("VPDChartOptionJSON() でエラー: %v", err)
	}
	doc := parseVPDOption(t, out)
	if len(doc.YAxis) == 0 {
		t.Fatalf("yAxis が無い\noption=%s", out)
	}
	min, okMin := doc.YAxis[0]["min"].(float64)
	max, okMax := doc.YAxis[0]["max"].(float64)
	if !okMin || min != 0 {
		t.Errorf("yAxis.min = %v(ok=%v), want 0", doc.YAxis[0]["min"], okMin)
	}
	if !okMax || max != 3.0 {
		t.Errorf("yAxis.max = %v(ok=%v), want 3.0(=YMax)", doc.YAxis[0]["max"], okMax)
	}
}

// ---- VPD移動平均（SMA）: 指定時のみ・凡例既定オフ -------------------------

func TestVPDChartOptionJSON_SMALegendDefaultOff(t *testing.T) {
	spec := baselineVPDSpec()
	spec.SMA = []float64{0.5, 1.05, 1.0}
	out, err := VPDChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("VPDChartOptionJSON() でエラー: %v", err)
	}
	doc := parseVPDOption(t, out)

	// VPD 実測線 + SMA の2本。
	if len(doc.Series) != 2 {
		t.Fatalf("series 数 = %d, want 2（VPD+移動平均）\noption=%s", len(doc.Series), out)
	}
	// 移動平均は2本目で symbol 非表示。
	var sma *struct {
		Name      string `json:"name"`
		Type      string `json:"type"`
		LineStyle *struct {
			Color string `json:"color"`
		} `json:"lineStyle"`
		ShowSymbol *bool           `json:"showSymbol"`
		MarkPoint  json.RawMessage `json:"markPoint"`
		MarkArea   *struct {
			Silent *bool `json:"silent"`
			Data   [][]struct {
				YAxis     *float64        `json:"yAxis"`
				ItemStyle json.RawMessage `json:"itemStyle"`
			} `json:"data"`
		} `json:"markArea"`
	}
	for i := range doc.Series {
		if doc.Series[i].Name == "VPD移動平均" {
			sma = &doc.Series[i]
		}
	}
	if sma == nil {
		t.Fatalf("VPD移動平均 系列が無い\noption=%s", out)
	}
	if sma.ShowSymbol == nil || *sma.ShowSymbol {
		t.Errorf("VPD移動平均 は showSymbol:false 想定\noption=%s", out)
	}
	// 凡例は表示・移動平均は selected:false（既定オフ）。
	if doc.Legend.Show == nil || !*doc.Legend.Show {
		t.Errorf("legend.show:true 想定\noption=%s", out)
	}
	if v, ok := doc.Legend.Selected["VPD移動平均"]; !ok || v {
		t.Errorf("legend.selected[VPD移動平均]=false 想定, got ok=%v v=%v\noption=%s", ok, v, out)
	}
}

// SMA 未指定時は移動平均系列を出さない（既定オフの素・要件7）。
func TestVPDChartOptionJSON_NoSMAWhenAbsent(t *testing.T) {
	out, err := VPDChartOptionJSON(baselineVPDSpec()) // SMA なし
	if err != nil {
		t.Fatalf("VPDChartOptionJSON() でエラー: %v", err)
	}
	if strings.Contains(out, "VPD移動平均") {
		t.Errorf("SMA 未指定なのに移動平均系列が出ている\noption=%s", out)
	}
	doc := parseVPDOption(t, out)
	if len(doc.Series) != 1 {
		t.Errorf("series 数 = %d, want 1（SMA 未指定）", len(doc.Series))
	}
}

// 外部入力（時刻ラベル）由来の < > & が混入しても返却 JSON は HTML 安全（§10-E）。
func TestVPDChartOptionJSON_HTMLSafe(t *testing.T) {
	spec := baselineVPDSpec()
	spec.Labels = []string{`</script><b>x&y`, "12:00", "23:00"}
	out, err := VPDChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("VPDChartOptionJSON() でエラー: %v", err)
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
