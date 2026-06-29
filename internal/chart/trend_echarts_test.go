package chart

import (
	"encoding/json"
	"strings"
	"testing"
)

const trendColorTest = "#7048e8" // --color-trend 想定（紫・テスト固定値）

// baselineTrendSpec は全オーバーレイを持つ TrendChartSpec（決定的テスト用）。
// 6点の月次系列・Sen 線・min/max 帯・ブートストラップ CI 帯・平年比・有意区間を固定する。
func baselineTrendSpec() TrendChartSpec {
	return TrendChartSpec{
		Labels:      []string{"2024-01", "2024-02", "2024-03", "2024-04", "2024-05", "2024-06"},
		Color:       trendColorTest,
		Unit:        "℃",
		RollupAvg:   []float64{18.0, 19.2, 20.1, 21.5, 22.3, 23.0},
		BandLower:   []float64{15, 16, 17, 18, 19, 20}, // min 帯下限
		BandUpper:   []float64{21, 22, 23, 24, 25, 26}, // max 帯上限
		SenLine:     []float64{18.1, 19.0, 19.9, 20.8, 21.7, 22.6},
		CILower:     []float64{17.6, 18.4, 19.2, 20.0, 20.8, 21.6},
		CIUpper:     []float64{18.6, 19.6, 20.6, 21.6, 22.6, 23.6},
		Climatology: []float64{18.5, 19.0, 19.5, 20.5, 21.0, 22.0},
		Significant: []Run{{StartIdx: 1, EndIdx: 4}}, // 有意区間（xAxis 範囲 markArea）
	}
}

// trendOptDoc は trend option JSON の構造アサート用スキーマ。
type trendOptDoc struct {
	XAxis  []map[string]any `json:"xAxis"`
	YAxis  []map[string]any `json:"yAxis"`
	Series []struct {
		Name      string `json:"name"`
		Type      string `json:"type"`
		Stack     string `json:"stack"`
		LineStyle *struct {
			Color string `json:"color"`
		} `json:"lineStyle"`
		AreaStyle *struct {
			Color   string   `json:"color"`
			Opacity *float64 `json:"opacity"`
		} `json:"areaStyle"`
		MarkLine json.RawMessage `json:"markLine"`
		MarkArea json.RawMessage `json:"markArea"`
	} `json:"series"`
	DataZoom json.RawMessage `json:"dataZoom"`
}

func parseTrendOption(t *testing.T, out string) trendOptDoc {
	t.Helper()
	var doc trendOptDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("trend option JSON が妥当でない: %v\noption=%s", err, out)
	}
	return doc
}

// countAreaSeries は areaStyle を持つ系列数を返す（CI 帯・min/max 帯の塗り検出用）。
func countAreaSeries(doc trendOptDoc) int {
	n := 0
	for _, s := range doc.Series {
		if s.AreaStyle != nil {
			n++
		}
	}
	return n
}

func findSeriesByName(doc trendOptDoc, name string) bool {
	for _, s := range doc.Series {
		if s.Name == name {
			return true
		}
	}
	return false
}

// ---- 主役線・基準色 -----------------------------------------------------------

func TestTrendChartOptionJSON_MainSeriesAndColor(t *testing.T) {
	out, err := TrendChartOptionJSON(baselineTrendSpec())
	if err != nil {
		t.Fatalf("TrendChartOptionJSON() でエラー: %v", err)
	}
	doc := parseTrendOption(t, out)
	if len(doc.Series) == 0 {
		t.Fatalf("series が空\noption=%s", out)
	}
	s0 := doc.Series[0]
	if s0.Type != "line" {
		t.Errorf("series[0].type = %q, want line", s0.Type)
	}
	if s0.LineStyle == nil || s0.LineStyle.Color != trendColorTest {
		t.Errorf("series[0] の基準色 %s が反映されていない\noption=%s", trendColorTest, out)
	}
	// X 軸はカテゴリ（月次/年次ラベル）、Y 軸は value。
	if typ, _ := doc.XAxis[0]["type"].(string); typ != "category" {
		t.Errorf("xAxis.type = %v, want category", doc.XAxis[0]["type"])
	}
	if typ, _ := doc.YAxis[0]["type"].(string); typ != "value" {
		t.Errorf("yAxis.type = %v, want value", doc.YAxis[0]["type"])
	}
}

// ---- Sen 線=markLine ＋ 有意区間=markArea --------------------------------------

func TestTrendChartOptionJSON_SenMarkLineAndSignificantMarkArea(t *testing.T) {
	out, err := TrendChartOptionJSON(baselineTrendSpec())
	if err != nil {
		t.Fatalf("TrendChartOptionJSON() でエラー: %v", err)
	}
	doc := parseTrendOption(t, out)
	s0 := doc.Series[0]

	// Sen トレンド線は series[0] の markLine（2端点の coord 線）。
	if len(s0.MarkLine) == 0 || string(s0.MarkLine) == "null" {
		t.Fatalf("series[0] に Sen markLine が無い\noption=%s", out)
	}
	if !strings.Contains(string(s0.MarkLine), `"coord"`) {
		t.Errorf("Sen markLine が coord 端点で表現されていない: %s", s0.MarkLine)
	}

	// 有意区間は series[0] の markArea（xAxis 範囲ゾーン）。
	if len(s0.MarkArea) == 0 || string(s0.MarkArea) == "null" {
		t.Fatalf("series[0] に有意区間 markArea が無い\noption=%s", out)
	}
	if !strings.Contains(string(s0.MarkArea), `"xAxis"`) {
		t.Errorf("有意区間 markArea が xAxis 範囲で表現されていない: %s", s0.MarkArea)
	}
}

// ---- ブートストラップ CI 帯・min/max 帯＝積み上げ area --------------------------

func TestTrendChartOptionJSON_StackedAreaBands(t *testing.T) {
	out, err := TrendChartOptionJSON(baselineTrendSpec())
	if err != nil {
		t.Fatalf("TrendChartOptionJSON() でエラー: %v", err)
	}
	doc := parseTrendOption(t, out)

	// CI 帯と min/max 帯の2つの塗り area がある（各々 透明ベース線＋area の積み上げ対）。
	if got := countAreaSeries(doc); got < 2 {
		t.Errorf("area 系列数 = %d, want >= 2（CI 帯＋min/max 帯）\noption=%s", got, out)
	}
	// 積み上げ stack グループが付与されている（帯下限＋帯幅を重ねるため）。
	hasStack := false
	for _, s := range doc.Series {
		if s.Stack != "" {
			hasStack = true
		}
	}
	if !hasStack {
		t.Errorf("stack グループを持つ系列が無い（帯の積み上げ未構成）\noption=%s", out)
	}
	// 平年比は独立系列として出る。
	if !findSeriesByName(doc, seriesNameClimatology) {
		t.Errorf("平年比系列 %q が無い\noption=%s", seriesNameClimatology, out)
	}
}

// ---- 長期閲覧用 dataZoom -------------------------------------------------------

func TestTrendChartOptionJSON_DataZoom(t *testing.T) {
	out, err := TrendChartOptionJSON(baselineTrendSpec())
	if err != nil {
		t.Fatalf("TrendChartOptionJSON() でエラー: %v", err)
	}
	doc := parseTrendOption(t, out)
	if len(doc.DataZoom) == 0 || string(doc.DataZoom) == "null" {
		t.Fatalf("dataZoom が無い（長期閲覧の拡大・スクロール手段）\noption=%s", out)
	}
	if !strings.Contains(string(doc.DataZoom), `"inside"`) || !strings.Contains(string(doc.DataZoom), `"slider"`) {
		t.Errorf("dataZoom に inside/slider が無い: %s", doc.DataZoom)
	}
}

// ---- 平年比なし・有意区間なし（縮退）-----------------------------------------

func TestTrendChartOptionJSON_MinimalNoOverlays(t *testing.T) {
	// 主役のみ（帯・CI・平年比・有意区間・Sen 線なし）。
	spec := TrendChartSpec{
		Labels:    []string{"2024", "2025", "2026"},
		Color:     trendColorTest,
		Unit:      "%",
		RollupAvg: []float64{60, 62, 64},
	}
	out, err := TrendChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("TrendChartOptionJSON() でエラー: %v", err)
	}
	doc := parseTrendOption(t, out)
	// 主役1系列のみ。
	if len(doc.Series) != 1 {
		t.Errorf("series 数 = %d, want 1（主役のみ）\noption=%s", len(doc.Series), out)
	}
	// 有意区間・Sen 線が無いので markArea/markLine は出ない。
	if s := string(doc.Series[0].MarkArea); s != "" && s != "null" {
		t.Errorf("有意区間なしなのに markArea が出ている: %s", s)
	}
	if s := string(doc.Series[0].MarkLine); s != "" && s != "null" {
		t.Errorf("Sen 線なしなのに markLine が出ている: %s", s)
	}
	// dataZoom は主役のみでも付ける（長期閲覧）。
	if len(doc.DataZoom) == 0 || string(doc.DataZoom) == "null" {
		t.Errorf("最小構成でも dataZoom は必要\noption=%s", out)
	}
}

// ---- ECharts 準拠の小文字キー -------------------------------------------------

func TestTrendChartOptionJSON_LowercaseKeys(t *testing.T) {
	out, err := TrendChartOptionJSON(baselineTrendSpec())
	if err != nil {
		t.Fatalf("TrendChartOptionJSON() でエラー: %v", err)
	}
	if !strings.Contains(out, `"xAxis"`) {
		t.Errorf("markArea に小文字 `xAxis` キーが無い\noption=%s", out)
	}
	if strings.Contains(out, `"XAxis"`) || strings.Contains(out, `"YAxis"`) {
		t.Errorf("ECharts 非準拠の大文字キーが混入している\noption=%s", out)
	}
}

// ---- HTML 安全（§10-E）-------------------------------------------------------

func TestTrendChartOptionJSON_HTMLSafe(t *testing.T) {
	spec := baselineTrendSpec()
	spec.Color = `</script><b>x&y`
	out, err := TrendChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("TrendChartOptionJSON() でエラー: %v", err)
	}
	for _, raw := range []string{"</script>", "<b>"} {
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

// ---- 日較差ΔT 推移の line option ----------------------------------------------

func TestDiurnalRangeChartOptionJSON_Basic(t *testing.T) {
	labels := []string{"2024-01", "2024-02", "2024-03", "2024-04"}
	deltaT := []float64{11.2, 12.6, 10.7, 9.8}
	out, err := DiurnalRangeChartOptionJSON(labels, deltaT, trendColorTest, "℃")
	if err != nil {
		t.Fatalf("DiurnalRangeChartOptionJSON() でエラー: %v", err)
	}
	doc := parseTrendOption(t, out)
	if len(doc.Series) != 1 {
		t.Fatalf("series 数 = %d, want 1（日較差線のみ）\noption=%s", len(doc.Series), out)
	}
	if doc.Series[0].LineStyle == nil || doc.Series[0].LineStyle.Color != trendColorTest {
		t.Errorf("日較差線の基準色が反映されていない\noption=%s", out)
	}
	// 長期の日次ΔT を拡大・スクロールできる dataZoom。
	if len(doc.DataZoom) == 0 || string(doc.DataZoom) == "null" {
		t.Errorf("日較差チャートに dataZoom が無い\noption=%s", out)
	}
	if typ, _ := doc.XAxis[0]["type"].(string); typ != "category" {
		t.Errorf("xAxis.type = %v, want category", doc.XAxis[0]["type"])
	}
}

func TestDiurnalRangeChartOptionJSON_HTMLSafe(t *testing.T) {
	out, err := DiurnalRangeChartOptionJSON([]string{"a", "b"}, []float64{1, 2}, `</script>&`, "℃")
	if err != nil {
		t.Fatalf("DiurnalRangeChartOptionJSON() でエラー: %v", err)
	}
	if strings.Contains(out, "</script>") {
		t.Errorf("HTML 安全でない: 生の </script> が漏れている\noption=%s", out)
	}
}
