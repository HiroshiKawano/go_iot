package component

import "testing"

// heatstress_panel_test.go は高温ストレスパネル templ (HeatStressPanel) の描画を
// Render→strings.Contains で固定する (タスク 6・要件 3.1/4.3/5.1/6.1/7.1/9.3)。
// 5つのチャート器 (data-echarts data-no-connect data-color) と option script・カード・
// カラースケール枠・トレンドノートが出ること、空データ時は Guidance のみになることを検証する。

// TestHeatStressPanel_データありで器とカードとカラースケール は、HasData=true 時に
// THI ヒートマップ・熱帯夜カレンダー・夜温/ΔT・AH・年間日数トレンドの各コンテナ
// (data-no-connect 付き・暑熱=暖色 data-color) と option script・カード・カラースケール枠が出ることを固定する。
func TestHeatStressPanel_データありで器とカードとカラースケール(t *testing.T) {
	html := render(t, HeatStressPanel(baselineHeatStressPanelView()))

	// 5つの ECharts マウント先 (すべて connect 除外マーカー data-no-connect 付き・calendar/heatmap は時刻軸でない)。
	for _, id := range []string{
		`id="thi-heatmap"`,
		`id="tropical-night-calendar"`,
		`id="night-temp-delta"`,
		`id="ah-line"`,
		`id="tropical-night-trend"`,
	} {
		assertContains(t, html, id)
	}
	assertContains(t, html, "data-echarts")
	assertContains(t, html, "data-no-connect")
	assertContains(t, html, `data-color="#d6336c"`) // 暑熱=暖色 (--color-heat)

	// option JSON は <script type="application/json"> で安全供給 (各コンテナの兄弟)。
	for _, id := range []string{
		`id="thi-heatmap-option"`,
		`id="tropical-night-calendar-option"`,
		`id="night-temp-delta-option"`,
		`id="ah-line-option"`,
		`id="tropical-night-trend-option"`,
	} {
		assertContains(t, html, id)
	}

	// 数値カード (現在THI/現在AH/直近夜温/熱帯夜連続)。.summary-grid-4 をモックから流用。
	assertContains(t, html, "summary-grid-4")
	for _, v := range []string{"現在THI", "79.5", "現在AH", "18.2 g/m³", "直近夜温", "26.3℃", "熱帯夜連続", "最長 12 日", "現在 5 日"} {
		assertContains(t, html, v)
	}

	// 暑熱カラースケール凡例 (枠 .heat-scale + 境界ラベル 涼/暑)。
	assertContains(t, html, "heat-scale")
	assertContains(t, html, "涼（安全）")
	assertContains(t, html, "暑（危険）")

	// 検出力の留保注記 (Sen 傾き＋符号・非有意≠トレンド無し)。
	assertContains(t, html, "Sen 傾き")
	assertContains(t, html, "トレンドが無いことを意味しません")
}

// TestHeatStressPanel_空データはGuidanceのみ は、HasData=false 時に導線注記だけ描き、
// チャート器・カード・カラースケールを一切出さない (レイアウト非破壊・縮退) ことを固定する (要件 9.2/9.4)。
func TestHeatStressPanel_空データはGuidanceのみ(t *testing.T) {
	v := HeatStressPanelView{
		HasData:  false,
		Guidance: "計測データがまだありません。",
		Color:    "#d6336c",
	}
	html := render(t, HeatStressPanel(v))

	assertContains(t, html, "計測データがまだありません。")
	// チャート器・カード・カラースケールは出さない。
	for _, absent := range []string{
		`id="thi-heatmap"`,
		`id="tropical-night-calendar"`,
		`id="ah-line"`,
		"summary-grid-4",
		"heat-scale",
	} {
		assertNotContains(t, html, absent)
	}
}

// TestHeatStressPanel_トレンド無しはトレンド器を出さず注記のみ は、HasData=true だが HasTrend=false
// (蓄積1年以下) のとき、年間日数トレンドの器を出さず「複数年必要」の注記へ縮退することを固定する (要件 6.4)。
func TestHeatStressPanel_トレンド無しはトレンド器を出さず注記のみ(t *testing.T) {
	v := baselineHeatStressPanelView()
	v.HasTrend = false
	v.TrendJSON = "{}"
	v.Card.SenSlopeSign = "—"
	html := render(t, HeatStressPanel(v))

	// 主要チャート器は出るが、トレンドの器は出ない。
	assertContains(t, html, `id="thi-heatmap"`)
	assertNotContains(t, html, `id="tropical-night-trend"`)
	// 検出力の留保注記 (複数年必要) は出る。
	assertContains(t, html, "複数年の蓄積が必要")
}

// TestHeatStressPanel_ゼロ値は何も描かない は、未結線/無関係 fixture (HasData=false・Guidance 空) で
// 孤立見出しを出さない (GDDPanel と同方針) ことを固定する。
func TestHeatStressPanel_ゼロ値は何も描かない(t *testing.T) {
	html := render(t, HeatStressPanel(HeatStressPanelView{}))
	assertNotContains(t, html, "高温ストレス")
	assertNotContains(t, html, `id="thi-heatmap"`)
}
