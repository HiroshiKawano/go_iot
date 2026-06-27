package page

import (
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/view/component"
	"github.com/HiroshiKawano/go_iot/internal/view/layout"
)

func baseCreateView() DeviceFormView {
	return DeviceFormView{
		Layout: layout.AppLayoutData{
			Title:     "デバイス登録 - 農業IoTシステム",
			UserName:  "テストユーザー",
			CSRFToken: "tok-create",
			CSSURL:    "/static/css/style.css?v=dev",
		},
		Form: component.DeviceFormView{
			CSRFToken: "tok-create",
			Action:    "/devices",
			IsEdit:    false,
			CancelURL: "/dashboard",
			IsActive:  "1",
			Errors:    map[string]string{},
		},
	}
}

func baseEditView() DeviceFormView {
	return DeviceFormView{
		Layout: layout.AppLayoutData{
			Title:     "デバイス編集 - 農業IoTシステム",
			UserName:  "テストユーザー",
			CSRFToken: "tok-edit",
			CSSURL:    "/static/css/style.css?v=dev",
		},
		DeviceName: "ハウスA温湿度計",
		Form: component.DeviceFormView{
			CSRFToken:  "tok-edit",
			Action:     "/devices/1",
			IsEdit:     true,
			CancelURL:  "/devices/1",
			Name:       "ハウスA温湿度計",
			MacAddress: "AA:BB:CC:DD:EE:01",
			Locality:   "佐敷町",
			Localities: []component.SelectOption{
				{Value: "佐敷町", Label: "佐敷（南城市）", Selected: true},
			},
			IsActive: "1",
			Errors:   map[string]string{},
		},
	}
}

func TestDeviceCreatePage_見出しとPOSTフォーム(t *testing.T) {
	html := render(t, DeviceCreatePage(baseCreateView()))

	// App レイアウト継承 (ヘッダーにユーザー名・メインコンテンツ枠)
	assertContains(t, html, "テストユーザー")
	assertContains(t, html, `id="main-content"`)
	// 登録の見出し
	assertContains(t, html, "<h1>デバイス登録</h1>")
	// 共有フォーム本体 (component に委譲)
	assertContains(t, html, `id="device-form"`)
	assertContains(t, html, `action="/devices"`)
	assertContains(t, html, `href="/dashboard"`) // キャンセル
	assertContains(t, html, `name="gorilla.csrf.Token"`)
	// 登録ページは method override 隠しフィールドを持たない
	if strings.Contains(html, `name="_method"`) {
		t.Errorf("登録ページに _method 隠しフィールドが描画されている:\n%s", html)
	}
}

func TestDeviceEditPage_見出しにデバイス名とPUT用隠しフィールド(t *testing.T) {
	html := render(t, DeviceEditPage(baseEditView()))

	assertContains(t, html, "テストユーザー")
	assertContains(t, html, `id="main-content"`)
	// 編集の見出し (デバイス名込み)
	assertContains(t, html, "デバイス編集: ハウスA温湿度計")
	// 共有フォーム + PUT 用隠しフィールド + 既存値復元
	assertContains(t, html, `id="device-form"`)
	assertContains(t, html, `action="/devices/1"`)
	assertContains(t, html, `name="_method"`)
	assertContains(t, html, `value="put"`)
	assertContains(t, html, `value="ハウスA温湿度計"`) // 既存値復元
	assertContains(t, html, `value="AA:BB:CC:DD:EE:01"`)
	assertContains(t, html, `href="/devices/1"`) // キャンセル
}
