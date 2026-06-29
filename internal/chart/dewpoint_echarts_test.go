package chart

import (
	"encoding/json"
	"strings"
	"testing"
)

const dewColorTest = "#3b5bdb"

// baselineDewpointSpec は露点 Td 線＋気温重ね＋結露帯2区間の決定的テスト用 spec。
func baselineDewpointSpec() DewpointChartSpec {
	return DewpointChartSpec{
		Labels:       []string{"00:00", "06:00", "12:00", "18:00"},
		DewColor:     dewColorTest,
		Dewpoint:     []float64{10, 12, 11, 9},
		Temperature:  []float64{12, 12.5, 20, 18},
		Condensation: []Run{{0, 1}, {3, 3}}, // spread 小=結露しやすい連続区間（2本）
	}
}

// dewpointOptDoc は露点 option JSON の構造アサート用スキーマ。
type dewpointOptDoc struct {
	YAxis  []map[string]any `json:"yAxis"`
	Series []struct {
		Name      string `json:"name"`
		Type      string `json:"type"`
		LineStyle *struct {
			Color string `json:"color"`
		} `json:"lineStyle"`
		ItemStyle *struct {
			Color string `json:"color"`
		} `json:"itemStyle"`
		MarkArea *struct {
			Silent *bool `json:"silent"`
			Data   [][]struct {
				XAxis     *float64 `json:"xAxis"`
				ItemStyle *struct {
					Color string `json:"color"`
				} `json:"itemStyle"`
			} `json:"data"`
		} `json:"markArea"`
	} `json:"series"`
}

func parseDewpointOption(t *testing.T, out string) dewpointOptDoc {
	t.Helper()
	var doc dewpointOptDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("露点 option JSON が妥当でない: %v\noption=%s", err, out)
	}
	return doc
}

// ---- series 2本（露点 Td 主役＋気温重ね）と結露帯 markArea ----------------------

func TestDewpointChartOptionJSON_TwoSeriesAndCondensationBands(t *testing.T) {
	out, err := DewpointChartOptionJSON(baselineDewpointSpec())
	if err != nil {
		t.Fatalf("DewpointChartOptionJSON() でエラー: %v", err)
	}
	doc := parseDewpointOption(t, out)

	// series は露点 Td（主役）＋気温重ねの2本。
	if len(doc.Series) != 2 {
		t.Fatalf("series 数 = %d, want 2（露点 Td＋気温）\noption=%s", len(doc.Series), out)
	}
	s0 := doc.Series[0]
	if s0.Type != "line" {
		t.Errorf("series[0].type = %q, want line", s0.Type)
	}
	// series[0]=露点 Td 線（寒色＝湿り側の基準色 DewColor）。
	if s0.LineStyle == nil || s0.LineStyle.Color != dewColorTest {
		t.Errorf("series[0] の露点基準色 %s（寒色）が反映されていない\noption=%s", dewColorTest, out)
	}

	// 結露帯 markArea は series[0] にあり、区間数ぶん（=2）。
	if s0.MarkArea == nil {
		t.Fatalf("series[0] に結露帯 markArea が無い\noption=%s", out)
	}
	if len(s0.MarkArea.Data) != 2 {
		t.Fatalf("結露帯ゾーン数 = %d, want 2（Condensation 区間数）\noption=%s", len(s0.MarkArea.Data), out)
	}
	// 各ゾーンは [start,end] の xAxis ペアで、Run のインデックスが反映されている。
	wantBounds := [][2]float64{{0, 1}, {3, 3}}
	for i, wb := range wantBounds {
		pair := s0.MarkArea.Data[i]
		if len(pair) != 2 || pair[0].XAxis == nil || pair[1].XAxis == nil {
			t.Fatalf("markArea[%d] が [start,end] の xAxis ペアでない: %+v", i, pair)
		}
		if *pair[0].XAxis != wb[0] || *pair[1].XAxis != wb[1] {
			t.Errorf("markArea[%d] = [%v,%v], want [%v,%v]", i, *pair[0].XAxis, *pair[1].XAxis, wb[0], wb[1])
		}
		// 始点に塗り色（itemStyle.color）。
		if pair[0].ItemStyle == nil || pair[0].ItemStyle.Color == "" {
			t.Errorf("markArea[%d] の始点に itemStyle.color（塗り色）が無い", i)
		}
	}
}

// markArea のキーは ECharts 準拠の小文字 `xAxis`（時間区間ハイライト）で、go-echarts 不具合の
// `XAxis`（大文字）が混入しないこと（P5 injectGapMarkArea と同型・D-1 回避を固定）。
func TestDewpointChartOptionJSON_LowercaseXAxisKey(t *testing.T) {
	out, err := DewpointChartOptionJSON(baselineDewpointSpec())
	if err != nil {
		t.Fatalf("DewpointChartOptionJSON() でエラー: %v", err)
	}
	if !strings.Contains(out, `"xAxis"`) {
		t.Errorf("結露帯 markArea に小文字 `xAxis` キーが無い\noption=%s", out)
	}
	if strings.Contains(out, `"XAxis"`) {
		t.Errorf("ECharts 非準拠の大文字 `XAxis` が混入している（D-1）\noption=%s", out)
	}
}

// 結露帯が空のときは markArea を付与しない（series[0] そのまま）。
func TestDewpointChartOptionJSON_NoMarkAreaWhenEmpty(t *testing.T) {
	spec := baselineDewpointSpec()
	spec.Condensation = nil
	out, err := DewpointChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("DewpointChartOptionJSON() でエラー: %v", err)
	}
	if strings.Contains(out, "markArea") {
		t.Errorf("結露帯が空なのに markArea が出ている\noption=%s", out)
	}
	doc := parseDewpointOption(t, out)
	if len(doc.Series) != 2 {
		t.Errorf("series 数 = %d, want 2", len(doc.Series))
	}
}

// 結露帯の塗り色は寒色トークン（湿り側）であること（物理規約・project_vpd_physics_convention）。
// 暖色（橙/赤系の rgb 高R成分）と取り違えていないことを符号化する。
func TestDewpointChartOptionJSON_CondensationBandIsColdColor(t *testing.T) {
	out, err := DewpointChartOptionJSON(baselineDewpointSpec())
	if err != nil {
		t.Fatalf("DewpointChartOptionJSON() でエラー: %v", err)
	}
	if !strings.Contains(out, condensationBandColor) {
		t.Errorf("結露帯の寒色トークン %q が option に無い\noption=%s", condensationBandColor, out)
	}
	doc := parseDewpointOption(t, out)
	color := doc.Series[0].MarkArea.Data[0][0].ItemStyle.Color
	if color != condensationBandColor {
		t.Errorf("結露帯色 = %q, want %q（寒色＝湿り側）", color, condensationBandColor)
	}
}

// 凡例マーカー/シンボル色（itemStyle）が線色（lineStyle）と一致する（凡例で露点=寒色・気温=暖色が読める）。
// 実機スモークで「気温の凡例マーカーが緑（既定パレット）・線が橙」の不一致を発見したため固定する。
func TestDewpointChartOptionJSON_SeriesItemStyleMatchesLine(t *testing.T) {
	out, err := DewpointChartOptionJSON(baselineDewpointSpec())
	if err != nil {
		t.Fatalf("DewpointChartOptionJSON() でエラー: %v", err)
	}
	doc := parseDewpointOption(t, out)
	if len(doc.Series) != 2 {
		t.Fatalf("series 数 = %d, want 2", len(doc.Series))
	}
	// series[0]=露点 Td 線（寒色 DewColor）。凡例マーカーも同色。
	if s0 := doc.Series[0]; s0.ItemStyle == nil || s0.ItemStyle.Color != dewColorTest {
		t.Errorf("series[0](露点) の itemStyle.color = %+v, want %s（凡例マーカーが線色と不一致）", s0.ItemStyle, dewColorTest)
	}
	// series[1]=気温 重ね線（暖色 dewTempOverlayColor）。凡例マーカーも同色。
	if s1 := doc.Series[1]; s1.ItemStyle == nil || s1.ItemStyle.Color != dewTempOverlayColor {
		t.Errorf("series[1](気温) の itemStyle.color = %+v, want %s（凡例マーカーが線色と不一致）", s1.ItemStyle, dewTempOverlayColor)
	}
}

// y 軸は auto 範囲（Scale:true）で気温と露点の同℃レンジに追従する（VPD の固定 min/max とは別方針）。
func TestDewpointChartOptionJSON_YAxisAutoScale(t *testing.T) {
	out, err := DewpointChartOptionJSON(baselineDewpointSpec())
	if err != nil {
		t.Fatalf("DewpointChartOptionJSON() でエラー: %v", err)
	}
	if !strings.Contains(out, `"scale":true`) {
		t.Errorf("y 軸が auto 範囲（scale:true）でない\noption=%s", out)
	}
}

// 外部入力（時刻ラベル）由来の < > & が混入しても返却 JSON は HTML 安全（§10-E）。
func TestDewpointChartOptionJSON_HTMLSafe(t *testing.T) {
	spec := baselineDewpointSpec()
	spec.Labels = []string{`</script><b>x&y`, "06:00", "12:00", "18:00"}
	out, err := DewpointChartOptionJSON(spec)
	if err != nil {
		t.Fatalf("DewpointChartOptionJSON() でエラー: %v", err)
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
