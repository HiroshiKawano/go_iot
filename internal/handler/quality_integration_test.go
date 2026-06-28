package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// quality_integration_test.go は data-quality-meta の画面統合・無回帰を httptest+gin で固定する
// (タスク7)。readings (品質メトリクス/総合バッジ/行フラグの結線・非所有 404・from>to 空) と
// device-show (欠測ギャップ描画・欠測なし無回帰) を、ユニット (タスク4〜6) の上に統合経路で検証する。

// ---- 7.1 readings 画面の統合 (httptest) -------------------------------------

// 期間フィルタ結果のフラグメント HTML に品質メトリクスボックス・総合バッジ・行品質フラグ列が載る。
func TestReadingsIntegration_フラグメントに品質メタとバッジと行フラグ(t *testing.T) {
	repo := ownerReadingsRepo()
	repo.countVal = 5
	repo.summaryRow = fullSummaryRow()
	// 中央値300秒に対し id2→id3 が900秒ギャップ → id3 が「欠測直後」。欠測率28.6% → 総合「注意」。
	gapped := []repository.SensorReading{
		qRow(1, 0, 2, 20, 60),
		qRow(2, 300, 2, 21, 61),
		qRow(3, 1200, 2, 20, 60), // 欠測直後
		qRow(4, 1500, 2, 22, 62),
		qRow(5, 1800, 2, 21, 61),
	}
	repo.rangeRows = gapped // 期間メトリクス/行フラグの算出元 (全行 ASC)
	repo.listRows = gapped  // 表示行 (id3 を含める)
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)

	w := requestWithHeaders(r, http.MethodGet, "/devices/1/readings?from=2026-04-13&to=2026-04-20",
		map[string]string{"HX-Request": "true"})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()

	// フラグメントのみ (レイアウト非包含)。
	if strings.Contains(body, "<html") {
		t.Errorf("HTMX 応答にレイアウトが含まれている")
	}
	// 品質メトリクスボックス + 品質フラグ列 + 異常行フラグ。
	assertHistoryBodyHas(t, body, "データ品質", "欠測率", "間隔ばらつき", "<th>品質</th>", "欠測直後")
	// 総合品質バッジ (信号色クラス・欠測率28.6% → 注意)。
	if !strings.Contains(body, "badge-caution") {
		t.Errorf("総合品質バッジ (注意) が出ていない:\n%s", body)
	}
}

// 非所有デバイスの品質メタを含む画面は 404 とし、品質メタを一切漏らさない (BOLA・R8.2)。
func TestReadingsIntegration_非所有は品質メタを漏らさず404(t *testing.T) {
	repo := ownerReadingsRepo()
	repo.devices[1] = repository.Device{ID: 1, UserID: 999, Name: "他人のデバイス"} // 別所有者
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1/readings")
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404 (非所有は秘匿)", w.Code)
	}
	if strings.Contains(w.Body.String(), "データ品質") {
		t.Errorf("非所有応答に品質メタが漏れている:\n%s", w.Body.String())
	}
}

// from>to (意味的に空・形式は妥当) は通常0件経路を通り、品質メタも空 (総合バッジ非表示)。
// 形式エラー経路 (error-message) とは区別される。
func TestReadingsIntegration_fromto逆転で品質メタも空(t *testing.T) {
	repo := ownerReadingsRepo() // countVal=0・rangeRows=nil (0件)
	r := newReadingsRouterWithUser(&ReadingsHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1/readings?from=2026-04-20&to=2026-04-01")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()

	// 形式エラーではない (既存0件経路と区別・error-message を出さない)。
	if strings.Contains(body, "error-message") {
		t.Errorf("from>to で形式エラー (error-message) が出ている")
	}
	// データ品質ボックスは出るが、0件ゆえ総合バッジ (信号色) は出さない。
	assertHistoryBodyHas(t, body, "データ品質")
	for _, cls := range []string{"badge-good", "badge-caution", "badge-bad"} {
		if strings.Contains(body, cls) {
			t.Errorf("0件 (from>to) なのに総合品質バッジ %q が出ている", cls)
		}
	}
}

// ---- 7.2 device-show グラフの統合・無回帰 -----------------------------------

// 欠測を含むデータ: 線分断 (connectNulls:false) + 欠測区間 markArea + 凡例が温湿度フラグメントに載る。
func TestDeviceShowIntegration_欠測ありでギャップ描画(t *testing.T) {
	repo := showDeviceRepo()
	repo.recentReadings = gapRows() // 5点・30分ギャップ (device_show_gap_test.go)
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := hxGet(r, "/devices/1/chart?period=24h")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()

	// 欠測ギャップ凡例 (静的な器) + 線分断 + markArea (温度・湿度2グラフ)。
	if !strings.Contains(body, "欠測区間") {
		t.Errorf("欠測ギャップ凡例が無い")
	}
	if strings.Count(body, `"connectNulls":false`) < 2 {
		t.Errorf("温湿度2グラフ分の connectNulls:false が無い")
	}
	if strings.Count(body, `"markArea"`) < 2 {
		t.Errorf("温湿度2グラフ分の欠測 markArea が無い")
	}
}

// 欠測なしデータ: gap 出力を一切出さず、P2 オーバーレイ (移動平均/正常帯/乖離率) と markPoint(最高/最低) が
// 従来どおり載る (無回帰・R9.1)。
func TestDeviceShowIntegration_欠測なしは既存グラフ不変(t *testing.T) {
	repo := showDeviceRepo()
	repo.recentReadings = regressionRows() // 3点・等間隔1時間 (欠測なし)
	r := newShowRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := hxGet(r, "/devices/1/chart?period=24h")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()

	// 欠測なしは gap 由来の線分断/凡例を一切出さない (後方互換)。
	for _, gap := range []string{"connectNulls", "欠測区間"} {
		if strings.Contains(body, gap) {
			t.Errorf("欠測なしなのに gap 出力 %q が載っている", gap)
		}
	}
	// markArea は VPD パネルの適正帯ゾーン (常設) のみ＝温湿度グラフには欠測 markArea を出さない。
	if got := strings.Count(body, `"markArea"`); got != 1 {
		t.Errorf(`markArea 数=%d, want 1 (VPD のみ・温湿度グラフに欠測帯なし)`, got)
	}
	// P2 統計オーバーレイの凡例トグルと markPoint(最高/最低) が従来どおり (R9.1 無回帰)。
	for _, want := range []string{"移動平均", "正常帯", "乖離率(%)", `"max"`, `"min"`} {
		if !strings.Contains(body, want) {
			t.Errorf("P2 オーバーレイ/markPoint 無回帰崩れ: %q が無い", want)
		}
	}
}
