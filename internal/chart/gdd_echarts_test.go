package chart

import (
	"encoding/json"
	"strings"
	"testing"
)

const gddColorTest = "#e8590c" // 暖色（--color-gdd 想定・テスト固定値）

// baselineGDDSpec は予測ありの GDDChartSpec（決定的テスト用）。
// 経過日数 value 軸・累積曲線・目標 1400・予測到達日 84 を固定し markLine/markPoint を一意に検証できる。
func baselineGDDSpec() GDDChartSpec {
	return GDDChartSpec{
		ElapsedDays: []float64{0, 10, 20, 30},
		Cumulative:  []float64{0, 150, 320, 500},
		Color:       gddColorTest,
		TargetGDD:   1400,
		ForecastDay: 84,
		HasForecast: true,
	}
}

// gddOptDoc は GDD option JSON の構造アサート用スキーマ。
type gddOptDoc struct {
	XAxis  []map[string]any `json:"xAxis"`
	YAxis  []map[string]any `json:"yAxis"`
	Series []struct {
		Name      string `json:"name"`
		Type      string `json:"type"`
		LineStyle *struct {
			Color string `json:"color"`
		} `json:"lineStyle"`
		Data []struct {
			Value []float64 `json:"value"`
		} `json:"data"`
		MarkLine *struct {
			Data []map[string]any `json:"data"`
		} `json:"markLine"`
		MarkPoint *struct {
			Data []map[string]any `json:"data"`
		} `json:"markPoint"`
	} `json:"series"`
	DataZoom json.RawMessage `json:"dataZoom"`
}

func parseGDDOption(t *testing.T, out string) gddOptDoc {
	t.Helper()
	var doc gddOptDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("GDD option JSON が妥当でない: %v\noption=%s", err, out)
	}
	return doc
}

// countMarkLineKey は markLine.data 内で指定キー（"yAxis"/"xAxis"）を持つ要素数を返す。
func countMarkLineKey(data []map[string]any, key string) int {
	n := 0
	for _, d := range data {
		if _, ok := d[key]; ok {
			n++
		}
	}
	return n
}

// ---- 累積曲線（主役）＋目標 markLine＋予測 markLine/markPoint -----------------

func TestGDDChartOptionJSON_BaselineWithForecast(t *testing.T) {
	out, err := GDDChartOptionJSON(baselineGDDSpec())
	if err != nil {
		t.Fatalf("GDDChartOptionJSON() でエラー: %v", err)
	}
	doc := parseGDDOption(t, out)

	// 累積曲線は series ちょうど1本（主役単独・クラッタ回避）。
	if len(doc.Series) != 1 {
		t.Fatalf("series 数 = %d, want 1（累積曲線のみ）\noption=%s", len(doc.Series), out)
	}
	s0 := doc.Series[0]
	if s0.Type != "line" {
		t.Errorf("series[0].type = %q, want line", s0.Type)
	}
	if s0.LineStyle == nil || s0.LineStyle.Color != gddColorTest {
		t.Errorf("series[0] の基準色 %s が反映されていない\noption=%s", gddColorTest, out)
	}
	// series[0] のデータは [経過日数, 累積] の座標ペア（value 軸ゆえ）。
	if len(s0.Data) != 4 {
		t.Fatalf("series[0].data 数 = %d, want 4\noption=%s", len(s0.Data), out)
	}
	if len(s0.Data[0].Value) != 2 || !approx(s0.Data[0].Value[0], 0) || !approx(s0.Data[0].Value[1], 0) {
		t.Errorf("data[0] = %v, want 座標ペア [0,0]", s0.Data[0].Value)
	}
	if !approx(s0.Data[3].Value[0], 30) || !approx(s0.Data[3].Value[1], 500) {
		t.Errorf("data[3] = %v, want 座標ペア [30,500]", s0.Data[3].Value)
	}

	// markLine: 目標 GDD 水平（yAxis）1本 ＋ 予測到達日 垂直（xAxis）1本 = 計2本。
	if s0.MarkLine == nil {
		t.Fatalf("series[0] に markLine が無い\noption=%s", out)
	}
	if got := countMarkLineKey(s0.MarkLine.Data, "yAxis"); got != 1 {
		t.Errorf("markLine の yAxis（目標水平線）本数 = %d, want 1\noption=%s", got, out)
	}
	if got := countMarkLineKey(s0.MarkLine.Data, "xAxis"); got != 1 {
		t.Errorf("markLine の xAxis（予測垂直線）本数 = %d, want 1\noption=%s", got, out)
	}
	// 目標水平線の y は TargetGDD=1400。
	for _, d := range s0.MarkLine.Data {
		if y, ok := d["yAxis"].(float64); ok && !approx(y, 1400) {
			t.Errorf("markLine yAxis = %v, want 1400(=TargetGDD)", y)
		}
		if x, ok := d["xAxis"].(float64); ok && !approx(x, 84) {
			t.Errorf("markLine xAxis = %v, want 84(=ForecastDay)", x)
		}
	}

	// markPoint: 予測到達点 coord [ForecastDay, TargetGDD] 1点。
	if s0.MarkPoint == nil || len(s0.MarkPoint.Data) != 1 {
		t.Fatalf("series[0] に予測 markPoint が1点無い\noption=%s", out)
	}
	coord, ok := s0.MarkPoint.Data[0]["coord"].([]any)
	if !ok || len(coord) != 2 {
		t.Fatalf("markPoint[0].coord が [x,y] でない: %v", s0.MarkPoint.Data[0]["coord"])
	}
	if x, _ := coord[0].(float64); !approx(x, 84) {
		t.Errorf("markPoint coord.x = %v, want 84(=ForecastDay)", coord[0])
	}
	if y, _ := coord[1].(float64); !approx(y, 1400) {
		t.Errorf("markPoint coord.y = %v, want 1400(=TargetGDD)", coord[1])
	}
}

// x 軸は経過日数の value 軸（category でない・予測到達日を xAxis 数値で表すため）。
func TestGDDChartOptionJSON_ValueXAxis(t *testing.T) {
	out, err := GDDChartOptionJSON(baselineGDDSpec())
	if err != nil {
		t.Fatalf("GDDChartOptionJSON() でエラー: %v", err)
	}
	doc := parseGDDOption(t, out)
	if len(doc.XAxis) == 0 {
		t.Fatalf("xAxis が無い\noption=%s", out)
	}
	if typ, _ := doc.XAxis[0]["type"].(string); typ != "value" {
		t.Errorf("xAxis.type = %v, want value\noption=%s", doc.XAxis[0]["type"], out)
	}
	if len(doc.YAxis) == 0 {
		t.Fatalf("yAxis が無い\noption=%s", out)
	}
	if typ, _ := doc.YAxis[0]["type"].(string); typ != "value" {
		t.Errorf("yAxis.type = %v, want value\noption=%s", doc.YAxis[0]["type"], out)
	}
	// dataZoom（inside/slider）で長期曲線を拡大・スクロールできる（要件 3.6）。
	if len(doc.DataZoom) == 0 || string(doc.DataZoom) == "null" {
		t.Errorf("dataZoom が無い（拡大・スクロール手段）\noption=%s", out)
	}
	if !strings.Contains(string(doc.DataZoom), `"inside"`) || !strings.Contains(string(doc.DataZoom), `"slider"`) {
		t.Errorf("dataZoom に inside/slider が無い: %s", doc.DataZoom)
	}
}

// 軸レンジは目標 markLine（y=TargetGDD）と予測 markLine/markPoint（x=ForecastDay）を
// 既定ビューで必ず可視にするため、yAxis.max >= TargetGDD・xAxis.max >= ForecastDay を明示する
// （未指定だと dataZoom がデータ域のみ表示しマークが見切れる・実機スモークで発見した回帰の固定）。
func TestGDDChartOptionJSON_AxisFramesTargetAndForecast(t *testing.T) {
	out, err := GDDChartOptionJSON(baselineGDDSpec()) // TargetGDD=1400, ForecastDay=84, Cumulative max=500
	if err != nil {
		t.Fatalf("GDDChartOptionJSON() でエラー: %v", err)
	}
	doc := parseGDDOption(t, out)
	if len(doc.YAxis) == 0 || len(doc.XAxis) == 0 {
		t.Fatalf("軸が無い\noption=%s", out)
	}
	yMax, okY := doc.YAxis[0]["max"].(float64)
	if !okY || yMax < 1400 {
		t.Errorf("yAxis.max = %v(ok=%v), want >= 1400(=TargetGDD で目標線可視)\noption=%s", doc.YAxis[0]["max"], okY, out)
	}
	xMax, okX := doc.XAxis[0]["max"].(float64)
	if !okX || xMax < 84 {
		t.Errorf("xAxis.max = %v(ok=%v), want >= 84(=ForecastDay で予測マーク可視)\noption=%s", doc.XAxis[0]["max"], okX, out)
	}
}

// 予測不能（HasForecast=false）でも目標線は可視にする（yAxis.max >= TargetGDD）。x は予測が無いのでデータ域でよい。
func TestGDDChartOptionJSON_AxisFramesTargetWhenNoForecast(t *testing.T) {
	spec := baselineGDDSpec()
	spec.HasForecast = false
	out, err := GDDChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("GDDChartOptionJSON() でエラー: %v", err)
	}
	doc := parseGDDOption(t, out)
	yMax, okY := doc.YAxis[0]["max"].(float64)
	if !okY || yMax < 1400 {
		t.Errorf("予測不能でも yAxis.max >= 1400 で目標線を可視にすべき: %v(ok=%v)", doc.YAxis[0]["max"], okY)
	}
}

// markLine のキーは ECharts 準拠の小文字 yAxis/xAxis で、go-echarts 大文字キー不具合が混入しないこと。
func TestGDDChartOptionJSON_LowercaseKeys(t *testing.T) {
	out, err := GDDChartOptionJSON(baselineGDDSpec())
	if err != nil {
		t.Fatalf("GDDChartOptionJSON() でエラー: %v", err)
	}
	if !strings.Contains(out, `"yAxis"`) {
		t.Errorf("markLine に小文字 `yAxis` キーが無い\noption=%s", out)
	}
	if !strings.Contains(out, `"xAxis"`) {
		t.Errorf("markLine に小文字 `xAxis` キーが無い\noption=%s", out)
	}
	if strings.Contains(out, `"YAxis"`) || strings.Contains(out, `"XAxis"`) {
		t.Errorf("ECharts 非準拠の大文字キーが混入している\noption=%s", out)
	}
}

// ---- 予測不能（HasForecast=false）: 予測マークを出さない -----------------------

func TestGDDChartOptionJSON_NoForecast(t *testing.T) {
	spec := baselineGDDSpec()
	spec.HasForecast = false
	out, err := GDDChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("GDDChartOptionJSON() でエラー: %v", err)
	}
	doc := parseGDDOption(t, out)
	s0 := doc.Series[0]

	// 目標 GDD 水平線（yAxis）は残るが、予測の垂直線（xAxis）は出ない。
	if s0.MarkLine == nil {
		t.Fatalf("HasForecast=false でも目標 markLine は必要\noption=%s", out)
	}
	if got := countMarkLineKey(s0.MarkLine.Data, "yAxis"); got != 1 {
		t.Errorf("目標水平線 yAxis 本数 = %d, want 1\noption=%s", got, out)
	}
	if got := countMarkLineKey(s0.MarkLine.Data, "xAxis"); got != 0 {
		t.Errorf("予測不能なのに予測垂直線 xAxis が %d 本出ている\noption=%s", got, out)
	}
	// 予測点 markPoint は出ない。
	if s0.MarkPoint != nil && len(s0.MarkPoint.Data) != 0 {
		t.Errorf("予測不能なのに markPoint が出ている: %+v\noption=%s", s0.MarkPoint.Data, out)
	}
}

// ElapsedDays と Cumulative の長さ不一致は短い方に合わせて防御的に座標化する（handler 縮退の保険）。
func TestGDDChartOptionJSON_LengthMismatchDefensive(t *testing.T) {
	spec := baselineGDDSpec()
	spec.ElapsedDays = []float64{0, 10, 20, 30} // 4点
	spec.Cumulative = []float64{0, 150, 320}    // 3点（短い）
	out, err := GDDChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("GDDChartOptionJSON() でエラー: %v", err)
	}
	doc := parseGDDOption(t, out)
	if len(doc.Series[0].Data) != 3 {
		t.Errorf("座標ペア数 = %d, want 3（短い方に合わせる）\noption=%s", len(doc.Series[0].Data), out)
	}
}

// 返却 JSON は HTML 安全（SetEscapeHTML=true）。Color に </script> を混ぜても生で漏れない（§10-E）。
func TestGDDChartOptionJSON_HTMLSafe(t *testing.T) {
	spec := baselineGDDSpec()
	spec.Color = `</script><b>x&y`
	out, err := GDDChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("GDDChartOptionJSON() でエラー: %v", err)
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
