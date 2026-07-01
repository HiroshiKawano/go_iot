package component

import (
	"strings"
	"testing"
)

// baselineGDDPanelView は予測あり・データありの GDDPanelView（決定的テスト用）。
func baselineGDDPanelView() GDDPanelView {
	return GDDPanelView{
		OptionJSON: `{"series":[{"type":"line"}]}`,
		Color:      "#e03131",
		CropLabel:  "米",
		Card: GDDCardView{
			Cumulative:   "500 ℃·日",
			Remaining:    "900 ℃·日",
			ForecastDate: "2026-09-15",
			Stage:        "分げつ",
			ElapsedDays:  "30 日",
		},
		Stages: []GrowthStageRow{
			{Name: "発芽", GDD: "0 ℃·日", Current: false},
			{Name: "分げつ", GDD: "300 ℃·日", Current: true},
			{Name: "収穫", GDD: "1400 ℃·日", Current: false},
		},
		Guidance: "",
		Note:     "予測収穫日は線形外挿による目安です（季節変動は織り込みません）。",
	}
}

// TestGDDPanel_チャート器とdata_no_connectとカードとステージ表 は、データあり時に
// GDD 累積曲線の器（#gdd-chart・data-echarts・data-no-connect・data-unit="℃·日"・data-color）と
// option script・数値カード・生育ステージ表が描画されることを固定する（R3.4/3.5/4.1/4.2/7.4）。
func TestGDDPanel_チャート器とdata_no_connectとカードとステージ表(t *testing.T) {
	html := render(t, GDDPanel(baselineGDDPanelView()))

	// ECharts マウント先（connect 除外マーカー data-no-connect 付き・経過日数 value 軸ゆえ）。
	assertContains(t, html, `id="gdd-chart"`)
	assertContains(t, html, "data-echarts")
	assertContains(t, html, "data-no-connect")
	assertContains(t, html, `data-unit="℃·日"`)
	assertContains(t, html, `data-color="#e03131"`)
	// option JSON は <script type="application/json"> で安全供給。
	assertContains(t, html, `<script type="application/json" id="gdd-chart-option">`)
	assertContains(t, html, `{"series":[{"type":"line"}]}`)

	// 数値カード（累積/残り積算/予測収穫日/現在ステージ/経過日数）。.summary-grid-4 をモックから流用。
	assertContains(t, html, "summary-grid-4")
	for _, v := range []string{"500 ℃·日", "900 ℃·日", "2026-09-15", "分げつ", "30 日"} {
		assertContains(t, html, v)
	}

	// 生育ステージ⇔GDD 対応表（.data-table 流用）。各段名・しきい値が出る。
	assertContains(t, html, "data-table")
	for _, v := range []string{"発芽", "収穫", "1400 ℃·日"} {
		assertContains(t, html, v)
	}

	// 近似注記（予測が線形外挿の目安である旨・R3.4）。
	assertContains(t, html, "線形外挿による目安")

	// 見出しの右に、GDD 収穫予測が馴染まない作物群（多年生・葉野菜）の未対応注記を出す。
	assertContains(t, html, "多年生（マンゴー/パイナップル/サトウキビ）は、GDD収穫予測が馴染まないため未対応")
}

// TestGDDPanel_前提欠落は導線注記のみでチャート器を出さない は、Guidance 非空時に
// 設定導線の注記のみを出し、#gdd-chart（累積曲線・予測）を描かないことを固定する（R6.3）。
func TestGDDPanel_前提欠落は導線注記のみでチャート器を出さない(t *testing.T) {
	v := GDDPanelView{
		Guidance: "作物と定植日を設定すると GDD を表示します。",
	}
	html := render(t, GDDPanel(v))

	assertContains(t, html, "作物と定植日を設定すると GDD を表示します。")
	// 縮退（導線注記）時も見出しの未対応注記は出る（見出しに付随・状態非依存）。
	assertContains(t, html, "多年生（マンゴー/パイナップル/サトウキビ）は、GDD収穫予測が馴染まないため未対応")
	// 前提欠落（OptionJSON 空）ではチャート器・カードを描かない（破綻させず縮退）。
	if strings.Contains(html, `id="gdd-chart"`) {
		t.Errorf("導線注記時に #gdd-chart が描画されている（縮退すべき）:\n%s", html)
	}
}
