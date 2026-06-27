package component

import (
	"strings"
	"testing"
)

// baseDeviceInfoView は稼働中デバイスの情報パネル相当データ。各テストで差分のみ上書きする。
func baseDeviceInfoView() DeviceInfoView {
	return DeviceInfoView{
		Name:         "ハウスA温湿度計",
		MacAddress:   "AA:BB:CC:DD:EE:01",
		Location:     "佐敷（南城市）", // 所在地は構造化 locality の認識名 (handler が整形済)
		StatusActive: true,
		LastCommText: "2026-04-20 14:30:00",
		EditURL:      "/devices/1/edit",
	}
}

func TestDeviceInfoPanel_稼働中の全要素を描画(t *testing.T) {
	html := render(t, DeviceInfoPanel(baseDeviceInfoView()))

	// カードと見出し（モックの実クラスのみ）
	assertContains(t, html, "device-info")
	assertContains(t, html, "デバイス情報")

	// dl 項目ラベル
	for _, label := range []string{"名前", "MAC", "場所", "状態", "最終通信"} {
		assertContains(t, html, label)
	}
	// 各値
	assertContains(t, html, "ハウスA温湿度計")
	assertContains(t, html, "AA:BB:CC:DD:EE:01")
	assertContains(t, html, "佐敷（南城市）")              // 所在地を認識名で表示 (R6.1)
	assertContains(t, html, "2026-04-20 14:30:00") // 最終通信 YYYY-MM-DD HH:MM:SS

	// 稼働中の状態記号
	assertContains(t, html, "status-active")
	assertContains(t, html, "● 稼働中")
	if strings.Contains(html, "○ 停止中") {
		t.Errorf("稼働中なのに停止中表記が描画されている:\n%s", html)
	}

	// 編集リンク + 削除ボタン（削除は Alpine モーダルを開く）
	assertContains(t, html, `href="/devices/1/edit"`)
	assertContains(t, html, "編集")
	assertContains(t, html, `@click="deleteModalOpen = true"`)
	assertContains(t, html, "btn-danger")
	assertContains(t, html, "削除")
}

func TestDeviceInfoPanel_停止中は停止記号で区別(t *testing.T) {
	v := baseDeviceInfoView()
	v.StatusActive = false
	html := render(t, DeviceInfoPanel(v))

	assertContains(t, html, "status-inactive")
	assertContains(t, html, "○ 停止中")
	if strings.Contains(html, "● 稼働中") {
		t.Errorf("停止中なのに稼働中表記が描画されている:\n%s", html)
	}
}

func TestDeviceInfoPanel_未通信と未設定のフォールバック表示(t *testing.T) {
	v := baseDeviceInfoView()
	v.LastCommText = "未通信" // 一度も通信していない
	v.Location = "未設定"     // 場所未登録
	html := render(t, DeviceInfoPanel(v))

	assertContains(t, html, "未通信")
	assertContains(t, html, "未設定")
}
