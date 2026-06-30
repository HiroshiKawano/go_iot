package chart

import (
	"encoding/json"
	"strings"
	"testing"
)

const (
	tempColorTest     = "#e8590c"
	humidityColorTest = "#1971c2"
)

// baselineSpec は生実測線のみの ChartSpec（決定的テストのため値を固定）。
// 最大 25.5 / 最小 18.0 で markPoint(max/min)・値の存在を一意に検証できる。
func baselineSpec() ChartSpec {
	return ChartSpec{
		Labels: []string{"00:00", "12:00", "23:00"},
		Unit:   "℃",
		Color:  tempColorTest,
		Raw:    []float64{20.0, 25.5, 18.0},
	}
}

// optDoc は option JSON の構造アサート用の最小スキーマ。
type optDoc struct {
	Legend struct {
		Show     *bool           `json:"show"`
		Data     []string        `json:"data"`
		Selected map[string]bool `json:"selected"`
	} `json:"legend"`
	YAxis  []map[string]any `json:"yAxis"`
	Series []optSeries      `json:"series"`
}

type optSeries struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Stack      string `json:"stack"`
	YAxisIndex int    `json:"yAxisIndex"`
	ShowSymbol *bool  `json:"showSymbol"`
	AreaStyle  *struct {
		Color   string   `json:"color"`
		Opacity *float64 `json:"opacity"`
	} `json:"areaStyle"`
	LineStyle *struct {
		Color   string   `json:"color"`
		Type    string   `json:"type"`
		Opacity *float64 `json:"opacity"`
	} `json:"lineStyle"`
	MarkPoint json.RawMessage `json:"markPoint"`
}

// parseOption は ChartOptionJSON の返り値を optDoc へ unmarshal するヘルパ。
func parseOption(t *testing.T, out string) optDoc {
	t.Helper()
	var doc optDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("option JSON が妥当でない: %v\noption=%s", err, out)
	}
	return doc
}

// seriesByName は名前で系列を引く（無ければ nil）。
func seriesByName(doc optDoc, name string) *optSeries {
	for i := range doc.Series {
		if doc.Series[i].Name == name {
			return &doc.Series[i]
		}
	}
	return nil
}

// ---- 3.1 生実測線ベースライン ----------------------------------------------

// 生実測線のみの ChartSpec から、xAxis category(ラベル列)と series[0]=line(実測値)+markPoint を
// 含む option を返すこと（R2.4, 7.3）。
func TestChartOptionJSON_BaselineRawSeries(t *testing.T) {
	out, err := ChartOptionJSON(baselineSpec())
	if err != nil {
		t.Fatalf("ChartOptionJSON() でエラー: %v", err)
	}
	doc := parseOption(t, out)

	// 生線のみ → series はちょうど1本で、それが line。
	if len(doc.Series) != 1 {
		t.Fatalf("series 数 = %d, want 1\noption=%s", len(doc.Series), out)
	}
	if doc.Series[0].Type != "line" {
		t.Errorf("series[0].type = %q, want line", doc.Series[0].Type)
	}
	if doc.Series[0].Name != "℃" {
		t.Errorf("series[0].name = %q, want ℃", doc.Series[0].Name)
	}
	// series[0] に markPoint(max/min)。
	mp := string(doc.Series[0].MarkPoint)
	if mp == "" || !strings.Contains(mp, `"max"`) || !strings.Contains(mp, `"min"`) {
		t.Errorf("series[0] の markPoint に max/min が無い: %s", mp)
	}
	// xAxis category・ラベル・実測値・相対 yAxis・cross tooltip・基準色。
	for _, want := range []string{
		`"type":"category"`, "00:00", "12:00", "23:00",
		`"value":25.5`, `"scale":true`, `"trigger":"axis"`, `"type":"cross"`,
		`"color":"` + tempColorTest + `"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("option に %s が含まれない\noption=%s", want, out)
		}
	}
	// 生線のみのときは凡例を出さない（クラッタ回避・旧 LineOptionJSON 同等）。
	if doc.Legend.Show != nil && *doc.Legend.Show {
		t.Errorf("生線のみでは legend を出さない想定だが show=true\noption=%s", out)
	}
}

// 外部入力(時刻ラベル)由来の < > & が混入しても返却 JSON は HTML 安全であること（§10-E, R7.5）。
func TestChartOptionJSON_HTMLSafe(t *testing.T) {
	spec := ChartSpec{
		Labels: []string{`</script><b>x&y`, "12:00"},
		Unit:   "℃",
		Color:  tempColorTest,
		Raw:    []float64{20.0, 25.5},
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

// 温度/湿度それぞれの基準色を踏襲すること（R2.4・無回帰）。
func TestChartOptionJSON_LineColor(t *testing.T) {
	tests := []struct {
		name  string
		unit  string
		color string
	}{
		{"温度は暖色", "℃", tempColorTest},
		{"湿度は寒色", "%", humidityColorTest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := baselineSpec()
			spec.Unit, spec.Color = tt.unit, tt.color
			out, err := ChartOptionJSON(spec)
			if err != nil {
				t.Fatalf("ChartOptionJSON() でエラー: %v", err)
			}
			if !strings.Contains(out, `"color":"`+tt.color+`"`) {
				t.Errorf("lineStyle に基準色 %s が含まれない\noption=%s", tt.color, out)
			}
		})
	}
}

// ---- 3.2 SMA 線系列と凡例既定オフ ------------------------------------------

func TestChartOptionJSON_SMASeries(t *testing.T) {
	spec := baselineSpec()
	spec.SMA = []float64{20.0, 22.75, 21.17}
	out, err := ChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("ChartOptionJSON() でエラー: %v", err)
	}
	doc := parseOption(t, out)

	// 生線 + SMA の2本（EMA/WMA を作らない＝移動平均は SMA 1本のみ）。
	if len(doc.Series) != 2 {
		t.Fatalf("series 数 = %d, want 2（生線+SMA）\noption=%s", len(doc.Series), out)
	}
	sma := seriesByName(doc, seriesNameSMA)
	if sma == nil {
		t.Fatalf("SMA 系列 %q が無い\noption=%s", seriesNameSMA, out)
	}
	// SMA は symbol 非表示の細線。
	if sma.ShowSymbol == nil || *sma.ShowSymbol {
		t.Errorf("SMA は showSymbol:false 想定\noption=%s", out)
	}
	// 凡例は表示され、移動平均は selected:false（既定オフ）。
	if doc.Legend.Show == nil || !*doc.Legend.Show {
		t.Errorf("legend.show:true 想定\noption=%s", out)
	}
	if v, ok := doc.Legend.Selected[seriesNameSMA]; !ok || v {
		t.Errorf("legend.selected[%q]=false 想定, got ok=%v v=%v\noption=%s", seriesNameSMA, ok, v, out)
	}
	if !containsStr(doc.Legend.Data, seriesNameSMA) {
		t.Errorf("legend.data に %q が含まれない: %v", seriesNameSMA, doc.Legend.Data)
	}
}

// ---- 3.3 正常帯（2系列積み上げ area・単一凡例トグル） ----------------------

func TestChartOptionJSON_NormalBand(t *testing.T) {
	spec := baselineSpec()
	spec.BandLower = []float64{18.0, 20.0, 19.0}
	spec.BandWidth = []float64{4.0, 5.0, 4.5}
	out, err := ChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("ChartOptionJSON() でエラー: %v", err)
	}
	doc := parseOption(t, out)

	lower := seriesByName(doc, seriesNameBandLower)
	band := seriesByName(doc, seriesNameBand)
	if lower == nil || band == nil {
		t.Fatalf("帯下限/帯幅の2系列が揃っていない\noption=%s", out)
	}
	// 同一 stack グループで積み上げ。
	if lower.Stack == "" || lower.Stack != band.Stack {
		t.Errorf("帯下限と帯幅の stack が共有されていない: lower=%q band=%q", lower.Stack, band.Stack)
	}
	// 帯幅は半透明の塗り（areaStyle.opacity>0）。
	if band.AreaStyle == nil || band.AreaStyle.Opacity == nil || *band.AreaStyle.Opacity <= 0 {
		t.Errorf("帯幅の areaStyle.opacity>0 が無い\noption=%s", out)
	}
	// 凡例は「正常帯」のみトグル対象で既定オフ。帯下限は凡例項目に出さない。
	if v, ok := doc.Legend.Selected[seriesNameBand]; !ok || v {
		t.Errorf("legend.selected[%q]=false 想定, got ok=%v v=%v", seriesNameBand, ok, v)
	}
	if !containsStr(doc.Legend.Data, seriesNameBand) {
		t.Errorf("legend.data に %q が含まれない: %v", seriesNameBand, doc.Legend.Data)
	}
	if containsStr(doc.Legend.Data, seriesNameBandLower) {
		t.Errorf("legend.data に帯下限 %q が出てはいけない: %v", seriesNameBandLower, doc.Legend.Data)
	}
	if _, ok := doc.Legend.Selected[seriesNameBandLower]; ok {
		t.Errorf("legend.selected に帯下限の項目があってはいけない: %v", doc.Legend.Selected)
	}
}

// ---- 3.4 乖離率系列（第2 y軸） ---------------------------------------------

func TestChartOptionJSON_DeviationSecondaryAxis(t *testing.T) {
	d0, d2 := 5.0, -3.0
	spec := baselineSpec()
	// 中央は未定義（nil）→ 欠落点。
	spec.Deviation = []*float64{&d0, nil, &d2}
	out, err := ChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("ChartOptionJSON() でエラー: %v", err)
	}
	doc := parseOption(t, out)

	// 第2 y軸が存在（yAxis が2軸）。
	if len(doc.YAxis) != 2 {
		t.Fatalf("yAxis 数 = %d, want 2（第2軸）\noption=%s", len(doc.YAxis), out)
	}
	dev := seriesByName(doc, seriesNameDeviation)
	if dev == nil {
		t.Fatalf("乖離率系列 %q が無い\noption=%s", seriesNameDeviation, out)
	}
	// 乖離率は第2 y軸（yAxisIndex=1）・点線・symbol 非表示。
	if dev.YAxisIndex != 1 {
		t.Errorf("乖離率 yAxisIndex = %d, want 1", dev.YAxisIndex)
	}
	if dev.LineStyle == nil || dev.LineStyle.Type != "dotted" {
		t.Errorf("乖離率は点線(lineStyle.type=dotted)想定\noption=%s", out)
	}
	if dev.ShowSymbol == nil || *dev.ShowSymbol {
		t.Errorf("乖離率は showSymbol:false 想定\noption=%s", out)
	}
	// 凡例既定オフ。
	if v, ok := doc.Legend.Selected[seriesNameDeviation]; !ok || v {
		t.Errorf("legend.selected[%q]=false 想定, got ok=%v v=%v", seriesNameDeviation, ok, v)
	}
}

// ---- 全部入り: 系列構成と既定オフをまとめて固定 -----------------------------

func TestChartOptionJSON_AllOverlays(t *testing.T) {
	d0, d2 := 5.0, -3.0
	spec := baselineSpec()
	spec.SMA = []float64{20.0, 22.75, 21.17}
	spec.BandLower = []float64{18.0, 20.0, 19.0}
	spec.BandWidth = []float64{4.0, 5.0, 4.5}
	spec.Deviation = []*float64{&d0, nil, &d2}
	out, err := ChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("ChartOptionJSON() でエラー: %v", err)
	}
	doc := parseOption(t, out)

	// 生実測 + SMA + 帯下限 + 帯幅 + 乖離率 = 5系列。生実測は必ず series[0]。
	if len(doc.Series) != 5 {
		t.Fatalf("series 数 = %d, want 5\noption=%s", len(doc.Series), out)
	}
	if doc.Series[0].Name != "℃" {
		t.Errorf("series[0] は生実測線(℃)であるべき, got %q（client の endLabel/sampling 温存）", doc.Series[0].Name)
	}
	// 3つのオーバーレイすべて既定オフ。
	for _, name := range []string{seriesNameSMA, seriesNameBand, seriesNameDeviation} {
		if v, ok := doc.Legend.Selected[name]; !ok || v {
			t.Errorf("legend.selected[%q]=false 想定, got ok=%v v=%v", name, ok, v)
		}
	}
	// HTML 安全（</script> 不混入）。
	if strings.Contains(out, "</script>") {
		t.Errorf("</script> が混入している\noption=%s", out)
	}
}

// ---- 3.5 日スケール SMA 追加系列（sma-window-select） ----------------------

// 日スケール SMA 系列を渡すと、各「移動平均 N日」が既存系列の後ろへ
// 基準色・dashed 細線・symbol 非表示・端ラベル(markPoint)なしの追加系列として組まれ、
// 凡例 data に各ラベル・legend.selected[ラベル]==false（既定オフ）が付くこと（R1.1〜1.3, 2.1〜2.3, 4.1）。
func TestChartOptionJSON_DaySMASeries(t *testing.T) {
	spec := baselineSpec()
	spec.DaySMAs = []DaySMASeries{
		{Label: "移動平均 3日", Values: []float64{19.5, 21.0, 20.2}},
		{Label: "移動平均 7日", Values: []float64{19.8, 20.4, 20.1}},
	}
	out, err := ChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("ChartOptionJSON() でエラー: %v", err)
	}
	doc := parseOption(t, out)

	// 生実測 + 日スケール2本 = 3系列。生実測は必ず series[0]（client の endLabel/sampling 温存）。
	if len(doc.Series) != 3 {
		t.Fatalf("series 数 = %d, want 3（生線+日スケール2本）\noption=%s", len(doc.Series), out)
	}
	if doc.Series[0].Name != "℃" {
		t.Errorf("series[0] は生実測線(℃)であるべき, got %q", doc.Series[0].Name)
	}

	for _, s := range spec.DaySMAs {
		ds := seriesByName(doc, s.Label)
		if ds == nil {
			t.Fatalf("日スケール系列 %q が無い\noption=%s", s.Label, out)
		}
		// 基準色・dashed 細線・symbol 非表示。
		if ds.LineStyle == nil || ds.LineStyle.Type != "dashed" {
			t.Errorf("%q は dashed(lineStyle.type=dashed)想定\noption=%s", s.Label, out)
		}
		if ds.LineStyle == nil || ds.LineStyle.Color != tempColorTest {
			t.Errorf("%q は基準色 %s 想定, got lineStyle=%+v", s.Label, tempColorTest, ds.LineStyle)
		}
		if ds.ShowSymbol == nil || *ds.ShowSymbol {
			t.Errorf("%q は showSymbol:false 想定\noption=%s", s.Label, out)
		}
		// 端ラベル（markPoint）は生実測線のみ。追加系列には付かない（R2.3）。
		if mp := string(ds.MarkPoint); mp != "" && mp != "null" {
			t.Errorf("%q に markPoint が付いてはいけない: %s", s.Label, mp)
		}
		// 凡例 data に各ラベル・既定オフ（selected:false）。
		if !containsStr(doc.Legend.Data, s.Label) {
			t.Errorf("legend.data に %q が含まれない: %v", s.Label, doc.Legend.Data)
		}
		if v, ok := doc.Legend.Selected[s.Label]; !ok || v {
			t.Errorf("legend.selected[%q]=false 想定, got ok=%v v=%v", s.Label, ok, v)
		}
	}
	// 凡例は表示される（生実測 + 日スケール）。
	if doc.Legend.Show == nil || !*doc.Legend.Show {
		t.Errorf("日スケール系列があれば legend.show:true 想定\noption=%s", out)
	}
}

// 追加系列は SMA のみ＝加重平滑（EMA/WMA）やローソク足（OHLC）系列を一切出さないこと、
// 凡例ラベルが中立（売買シグナル/交差語を含まない）であること（R4.1, 4.2, 4.4）。
func TestChartOptionJSON_DaySMA_NoWeightedNoOHLC(t *testing.T) {
	spec := baselineSpec()
	spec.DaySMAs = []DaySMASeries{
		{Label: "移動平均 3日", Values: []float64{19.5, 21.0, 20.2}},
		{Label: "移動平均 14日", Values: []float64{19.8, 20.4, 20.1}},
	}
	out, err := ChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("ChartOptionJSON() でエラー: %v", err)
	}
	doc := parseOption(t, out)

	// 全系列は line（candlestick=OHLC を出さない）。
	for _, s := range doc.Series {
		if s.Type != "line" {
			t.Errorf("series %q の type=%q, want line（OHLC/candlestick 不可）", s.Name, s.Type)
		}
	}
	// 加重平滑・ローソク足・売買シグナル語が option（系列名/凡例）に現れない。
	for _, banned := range []string{"EMA", "WMA", "candlestick", "OHLC", "ゴールデンクロス", "デッドクロス"} {
		if strings.Contains(out, banned) {
			t.Errorf("禁止語 %q が option に混入している\noption=%s", banned, out)
		}
	}
}

// DaySMAs が nil/空のときは追加系列に関する一切の出力差分がなく、
// 従来 JSON とバイト等価であること（既存 echarts テストが無改変で緑＝後方互換の不変条件）。
func TestChartOptionJSON_DaySMA_EmptyByteEqual(t *testing.T) {
	// オーバーレイ込みでも空 DaySMAs が描画へ影響しないことを確かめる。
	base := baselineSpec()
	base.SMA = []float64{20.0, 22.75, 21.17}
	base.BandLower = []float64{18.0, 20.0, 19.0}
	base.BandWidth = []float64{4.0, 5.0, 4.5}

	want, err := ChartOptionJSON(base) // DaySMAs は nil（フィールド未設定）
	if err != nil {
		t.Fatalf("ChartOptionJSON() でエラー: %v", err)
	}

	withEmpty := base
	withEmpty.DaySMAs = []DaySMASeries{} // 空スライス
	gotEmpty, err := ChartOptionJSON(withEmpty)
	if err != nil {
		t.Fatalf("ChartOptionJSON() でエラー: %v", err)
	}
	if gotEmpty != want {
		t.Errorf("空 DaySMAs で従来 JSON とバイト等価でない\n want=%s\n got =%s", want, gotEmpty)
	}

	withNil := base
	withNil.DaySMAs = nil
	gotNil, err := ChartOptionJSON(withNil)
	if err != nil {
		t.Fatalf("ChartOptionJSON() でエラー: %v", err)
	}
	if gotNil != want {
		t.Errorf("nil DaySMAs で従来 JSON とバイト等価でない\n want=%s\n got =%s", want, gotNil)
	}
}

// 全オーバーレイ＋日スケール SMA の系列構成と既定オフをまとめて固定する（R1〜R4 結合）。
func TestChartOptionJSON_AllOverlaysWithDaySMA(t *testing.T) {
	d0, d2 := 5.0, -3.0
	spec := baselineSpec()
	spec.SMA = []float64{20.0, 22.75, 21.17}
	spec.BandLower = []float64{18.0, 20.0, 19.0}
	spec.BandWidth = []float64{4.0, 5.0, 4.5}
	spec.Deviation = []*float64{&d0, nil, &d2}
	spec.DaySMAs = []DaySMASeries{
		{Label: "移動平均 3日", Values: []float64{19.5, 21.0, 20.2}},
		{Label: "移動平均 7日", Values: []float64{19.8, 20.4, 20.1}},
	}
	out, err := ChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("ChartOptionJSON() でエラー: %v", err)
	}
	doc := parseOption(t, out)

	// 生実測 + SMA + 帯下限 + 帯幅 + 乖離率 + 日スケール2本 = 7系列。生実測は series[0]。
	if len(doc.Series) != 7 {
		t.Fatalf("series 数 = %d, want 7\noption=%s", len(doc.Series), out)
	}
	if doc.Series[0].Name != "℃" {
		t.Errorf("series[0] は生実測線(℃)であるべき, got %q", doc.Series[0].Name)
	}
	// 既存オーバーレイ＋日スケール全てが既定オフ。
	for _, name := range []string{
		seriesNameSMA, seriesNameBand, seriesNameDeviation, "移動平均 3日", "移動平均 7日",
	} {
		if v, ok := doc.Legend.Selected[name]; !ok || v {
			t.Errorf("legend.selected[%q]=false 想定, got ok=%v v=%v", name, ok, v)
		}
	}
	// HTML 安全（</script> 不混入）。
	if strings.Contains(out, "</script>") {
		t.Errorf("</script> が混入している\noption=%s", out)
	}
}

// DaySMASeries.Values の長さが Labels と異なっても（同長前提の契約は handler が担保）、
// ChartOptionJSON は panic せず妥当な JSON を返し、与えた値をそのまま描画すること（防御・契約崩れの検出）。
func TestChartOptionJSON_DaySMA_LengthMismatchGraceful(t *testing.T) {
	// Labels は3点。Values は2点（短い）で渡す異常系。
	spec := baselineSpec()
	spec.DaySMAs = []DaySMASeries{
		{Label: "移動平均 3日", Values: []float64{19.5, 21.0}},
	}
	out, err := ChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("長さ不一致でエラーになってはいけない: %v", err)
	}
	doc := parseOption(t, out)

	// 生実測 + 日スケール1本 = 2系列（落ちずに系列は組まれる）。
	if len(doc.Series) != 2 {
		t.Fatalf("series 数 = %d, want 2\noption=%s", len(doc.Series), out)
	}
	ds := seriesByName(doc, "移動平均 3日")
	if ds == nil {
		t.Fatalf("日スケール系列が無い\noption=%s", out)
	}
	// 与えた2値が忠実に描画される（黙ってパディング/切詰めしない＝契約崩れを検出可能）。
	for _, want := range []string{`"value":19.5`, `"value":21`} {
		if !strings.Contains(out, want) {
			t.Errorf("option に %s が含まれない（与えた値が忠実に描画されていない）\noption=%s", want, out)
		}
	}
}

// daySMALineWidth は細線の範囲（1.0〜2.0）で index ごとに僅かに太くし、上限 2.0 でクランプすること。
func TestDaySMALineWidth(t *testing.T) {
	tests := []struct {
		index int
		want  float32
	}{
		{0, 1.0},
		{1, 1.5},
		{2, 2.0}, // 最大3本（R2.4）の末尾
		{3, 2.0}, // 上限クランプ（防御・実運用では到達しない）
		{10, 2.0},
	}
	for _, tt := range tests {
		if got := daySMALineWidth(tt.index); got != tt.want {
			t.Errorf("daySMALineWidth(%d) = %v, want %v（細線 1.0〜2.0 にクランプ）", tt.index, got, tt.want)
		}
	}
}

// containsStr は文字列スライスに s が含まれるか。
func containsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
