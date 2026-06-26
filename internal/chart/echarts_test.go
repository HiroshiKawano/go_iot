package chart

import (
	"encoding/json"
	"strings"
	"testing"
)

// optionSeries は ECharts option ビルダ用の固定点列を返す（決定的テストのため値を固定）。
// 最大 25.5 / 最小 18.0 で、markPoint(max/min)・値の存在を一意に検証できるよう構成する。
// svg_test.go の sampleSeries とは独立に定義し、本ファイル単体で自己完結させる
// (svg_test.go は task 6 で撤去予定のため依存しない)。
func optionSeries() []Series {
	return []Series{{
		Name: "",
		Points: []Point{
			{Label: "00:00", Y: 20.0},
			{Label: "12:00", Y: 25.5},
			{Label: "23:00", Y: 18.0},
		},
	}}
}

const (
	tempColorTest     = "#e8590c"
	humidityColorTest = "#1971c2"
)

// LineOptionJSON は単一系列の折れ線について、xAxis カテゴリ(ラベル列)と
// 1 本の line series(実測値)を含む ECharts option JSON を返すこと(R2.1, 2.5, 7.2)。
func TestLineOptionJSON_SeriesAndXAxis(t *testing.T) {
	out, err := LineOptionJSON(optionSeries(), "℃", tempColorTest)
	if err != nil {
		t.Fatalf("LineOptionJSON() でエラー: %v", err)
	}

	// 折れ線 series はちょうど 1 本(単一系列)。
	if n := strings.Count(out, `"type":"line"`); n != 1 {
		t.Errorf("line series 数 = %d, want 1\noption=%s", n, out)
	}
	// xAxis は category 軸。
	if !strings.Contains(out, `"type":"category"`) {
		t.Errorf("xAxis category が含まれない\noption=%s", out)
	}
	// xAxis ラベル列(Label)が含まれる。
	for _, label := range []string{"00:00", "12:00", "23:00"} {
		if !strings.Contains(out, label) {
			t.Errorf("xAxis ラベル %q が含まれない\noption=%s", label, out)
		}
	}
	// series データ(Y 実測値)が含まれる。
	if !strings.Contains(out, `"value":25.5`) {
		t.Errorf("series の実測値 25.5 が含まれない\noption=%s", out)
	}
}

// yAxis はデータ範囲に追従する相対表示(scale:true)であること。0 始まり固定だと値が一定帯に
// 偏って折れ線が平たく見えるため、旧SVG(データ範囲ベース)と同様に 0 を強制しない(無回帰)。
func TestLineOptionJSON_YAxisScaleRelative(t *testing.T) {
	out, err := LineOptionJSON(optionSeries(), "℃", tempColorTest)
	if err != nil {
		t.Fatalf("LineOptionJSON() でエラー: %v", err)
	}
	if !strings.Contains(out, `"scale":true`) {
		t.Errorf("yAxis に scale:true (相対表示) が含まれない\noption=%s", out)
	}
}

// markPoint に最高(max)と最低(min)が含まれること(R2.2)。
func TestLineOptionJSON_MarkPointMaxMin(t *testing.T) {
	out, err := LineOptionJSON(optionSeries(), "℃", tempColorTest)
	if err != nil {
		t.Fatalf("LineOptionJSON() でエラー: %v", err)
	}

	if !strings.Contains(out, `"markPoint"`) {
		t.Errorf("markPoint が含まれない\noption=%s", out)
	}
	for _, typ := range []string{`"type":"max"`, `"type":"min"`} {
		if !strings.Contains(out, typ) {
			t.Errorf("markPoint に %s が含まれない\noption=%s", typ, out)
		}
	}
}

// tooltip(trigger axis) と axisPointer(type cross) を含むこと(R3.1, 3.2)。
func TestLineOptionJSON_TooltipAxisPointerCross(t *testing.T) {
	out, err := LineOptionJSON(optionSeries(), "℃", tempColorTest)
	if err != nil {
		t.Fatalf("LineOptionJSON() でエラー: %v", err)
	}

	if !strings.Contains(out, `"trigger":"axis"`) {
		t.Errorf("tooltip trigger:axis が含まれない\noption=%s", out)
	}
	if !strings.Contains(out, `"type":"cross"`) {
		t.Errorf("axisPointer type:cross が含まれない\noption=%s", out)
	}
}

// lineStyle.color が温度/湿度それぞれの配色を踏襲すること(R2.4)。
func TestLineOptionJSON_LineColor(t *testing.T) {
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
			out, err := LineOptionJSON(optionSeries(), tt.unit, tt.color)
			if err != nil {
				t.Fatalf("LineOptionJSON() でエラー: %v", err)
			}
			want := `"color":"` + tt.color + `"`
			if !strings.Contains(out, want) {
				t.Errorf("lineStyle に %s が含まれない\noption=%s", want, out)
			}
		})
	}
}

// 外部入力(時刻ラベル)由来の < > & が混入しても、返却 JSON は HTML 安全であること(§10-E, R7.2)。
// encoding/json(SetEscapeHTML=true 既定)で \uXXXX 化され、生タグ/`</script>` が漏れない。
func TestLineOptionJSON_HTMLSafe(t *testing.T) {
	malicious := []Series{{
		Name: "",
		Points: []Point{
			{Label: `</script><b>x&y`, Y: 20.0},
			{Label: "12:00", Y: 25.5},
		},
	}}

	out, err := LineOptionJSON(malicious, "℃", tempColorTest)
	if err != nil {
		t.Fatalf("LineOptionJSON() でエラー: %v", err)
	}

	// 生の山括弧/閉じスクリプトタグが一切漏れていないこと。
	for _, raw := range []string{"<", ">", "</script>", "<b>"} {
		if strings.Contains(out, raw) {
			t.Errorf("HTML 安全でない: 生の %q が漏れている\noption=%s", raw, out)
		}
	}
	// ラベル内容は失われず、エスケープ形(< など)で保持されていること。
	if !strings.Contains(out, "\\u003c") {
		t.Errorf("`<` がエスケープ形(\\u003c)で保持されていない\noption=%s", out)
	}

	// 返却文字列は妥当な JSON であること(壊れていない)。
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Errorf("返却が妥当な JSON でない: %v\noption=%s", err, out)
	}
}
