package component

import (
	"strings"
	"testing"
)

// baseDeviceFormView は登録フォーム相当の有効データ。各テストで差分のみ上書きする。
func baseDeviceFormView() DeviceFormView {
	return DeviceFormView{
		CSRFToken:  "tok-xyz",
		Action:     "/devices",
		IsEdit:     false,
		CancelURL:  "/dashboard",
		Name:       "温室センサー",
		MacAddress: "AA:BB:CC:DD:EE:FF",
		Location:   "第1ハウス",
		IsActive:   "1",
		Errors:     map[string]string{},
	}
}

func TestDeviceForm_共通要素とCSRFと入力値復元(t *testing.T) {
	html := render(t, DeviceForm(baseDeviceFormView()))

	// フォーム本体 (R27: id=device-form) と送信先・メソッド
	assertContains(t, html, `id="device-form"`)
	assertContains(t, html, `action="/devices"`)
	assertContains(t, html, `method="post"`)
	// CSRF 隠しフィールド (非 HTMX フォームのため必須)
	assertContains(t, html, `name="gorilla.csrf.Token"`)
	assertContains(t, html, "tok-xyz")
	// 入力値復元 (value)
	assertContains(t, html, `name="name"`)
	assertContains(t, html, `value="温室センサー"`)
	assertContains(t, html, `name="mac_address"`)
	assertContains(t, html, `value="AA:BB:CC:DD:EE:FF"`)
	assertContains(t, html, `name="location"`)
	assertContains(t, html, `value="第1ハウス"`)
	// MAC 補助表示
	assertContains(t, html, "形式: XX:XX:XX:XX:XX:XX")
	// キャンセル導線
	assertContains(t, html, `href="/dashboard"`)
	// モックの実クラスのみ使用 (独自クラス新設禁止)
	for _, cls := range []string{"card-narrow", "form-group", "radio-group", "required-mark", "form-help", "form-actions", "btn"} {
		assertContains(t, html, cls)
	}
}

func TestDeviceForm_登録時はmethodオーバーライド隠しフィールドなし(t *testing.T) {
	html := render(t, DeviceForm(baseDeviceFormView()))
	if strings.Contains(html, `name="_method"`) {
		t.Errorf("登録フォームに _method 隠しフィールドが描画されている:\n%s", html)
	}
	// 登録ボタン
	assertContains(t, html, "登録")
}

func TestDeviceForm_編集時はPUT用隠しフィールドと更新ボタン(t *testing.T) {
	v := baseDeviceFormView()
	v.IsEdit = true
	v.Action = "/devices/1"
	v.CancelURL = "/devices/1"
	html := render(t, DeviceForm(v))

	assertContains(t, html, `name="_method"`)
	assertContains(t, html, `value="put"`)
	assertContains(t, html, `action="/devices/1"`)
	assertContains(t, html, `href="/devices/1"`)
	assertContains(t, html, "更新")
}

func TestDeviceForm_稼働中はvalue1がchecked(t *testing.T) {
	v := baseDeviceFormView()
	v.IsActive = "1"
	html := render(t, DeviceForm(v))

	assertContains(t, html, `value="1" checked`)
	if strings.Contains(html, `value="0" checked`) {
		t.Errorf("稼働中(=1)なのに停止中(value=0)が checked になっている:\n%s", html)
	}
}

func TestDeviceForm_停止中はvalue0がchecked(t *testing.T) {
	v := baseDeviceFormView()
	v.IsActive = "0"
	html := render(t, DeviceForm(v))

	assertContains(t, html, `value="0" checked`)
	if strings.Contains(html, `value="1" checked`) {
		t.Errorf("停止中(=0)なのに稼働中(value=1)が checked になっている:\n%s", html)
	}
}

func TestDeviceForm_項目別エラーをそれぞれ描画(t *testing.T) {
	v := baseDeviceFormView()
	v.Name = ""
	v.MacAddress = ""
	v.Errors = map[string]string{
		"name":        "デバイス名を入力してください",
		"mac_address": "MACアドレスを入力してください",
		"location":    "設置場所は255文字以内で入力してください",
		"is_active":   "ステータスが不正です",
	}
	html := render(t, DeviceForm(v))

	assertContains(t, html, "error-message")
	assertContains(t, html, "デバイス名を入力してください")
	assertContains(t, html, "MACアドレスを入力してください")
	assertContains(t, html, "設置場所は255文字以内で入力してください")
	assertContains(t, html, "ステータスが不正です")
}
