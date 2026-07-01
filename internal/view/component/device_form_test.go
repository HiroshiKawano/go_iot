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
		Locality:   "佐敷町",
		Localities: []SelectOption{
			{Value: "那覇市", Label: "那覇市", Selected: false},
			{Value: "佐敷町", Label: "佐敷（南城市）", Selected: true},
			{Value: "国頭村", Label: "国頭村", Selected: false},
		},
		Crop: "goya",
		Crops: []SelectOption{
			// GDD 対応作物は handler(cropOptions) が「(GDD対応)」接尾辞を付す。未対応(サトウキビ)は素のまま。
			{Value: "goya", Label: "ゴーヤ(GDD対応)", Selected: true},
			{Value: "ingen", Label: "インゲン(GDD対応)", Selected: false},
			{Value: "sugarcane", Label: "サトウキビ", Selected: false},
		},
		PlantingDate: "2026-04-01",
		IsActive:     "1",
		Errors:       map[string]string{},
	}
}

// TestDeviceForm_定植日のdate入力と値復元 は GDD 累積の起点となる定植/播種日が
// 作物 select の隣に素の date input (Tom Select 非対象) として描画され、保存値が value で復元されることを固定する (R2.1/2.3)。
func TestDeviceForm_定植日のdate入力と値復元(t *testing.T) {
	html := render(t, DeviceForm(baseDeviceFormView()))

	// 定植日は <input type="date" name="planting_date">（任意・空可）。
	assertContains(t, html, `type="date"`)
	assertContains(t, html, `name="planting_date"`)
	// ラベルは GDD 収穫予測の起点であることを明示（栽培作物は VPD/病害と共有ゆえ改名しない）。
	assertContains(t, html, "GDD収穫予測用(定植/播種日)")
	// GDD 対応作物選択時のみ表示するため、定植日フィールドは x-show/x-cloak で制御される。
	assertContains(t, html, `x-show="showPlantingDate"`)
	assertContains(t, html, "x-cloak")
	// 栽培作物 select の変更で表示可否を切り替える (@change・(GDD対応) ラベル判定)。
	assertContains(t, html, `@change="showPlantingDate = ($event.target.selectedOptions[0]?.text || '').includes('(GDD対応)')"`)
	// baseDeviceFormView は goya(GDD対応) 選択ゆえ初期表示 true。
	assertContains(t, html, `x-data="{ showPlantingDate: true }"`)
	// 保存値が value で復元される。
	assertContains(t, html, `value="2026-04-01"`)
	// Tom Select 非対象（素の input）ゆえ js-tom-select は所在地+作物の2つのまま（増えない）。
	if got := strings.Count(html, "js-tom-select"); got != 2 {
		t.Errorf("js-tom-select の数 = %d, want 2 (定植日は素の input で増やさない)", got)
	}
}

// TestDeviceForm_定植日の初期表示は選択作物のGDD対応で決まる は、Alpine x-data の初期 showPlantingDate が
// 「選択中の作物ラベルに (GDD対応) 接尾辞があるか」で決まることを固定する（サーバ側でちらつきなく初期表示）。
func TestDeviceForm_定植日の初期表示は選択作物のGDD対応で決まる(t *testing.T) {
	tests := []struct {
		name  string
		crops []SelectOption
		want  string // 期待する x-data
	}{
		{
			name:  "GDD対応作物(接尾辞あり)選択→初期表示true",
			crops: []SelectOption{{Value: "goya", Label: "ゴーヤ(GDD対応)", Selected: true}},
			want:  `x-data="{ showPlantingDate: true }"`,
		},
		{
			name:  "未対応作物(接尾辞なし)選択→初期非表示false",
			crops: []SelectOption{{Value: "sugarcane", Label: "サトウキビ", Selected: true}},
			want:  `x-data="{ showPlantingDate: false }"`,
		},
		{
			name:  "未選択(Selectedなし)→初期非表示false",
			crops: []SelectOption{{Value: "goya", Label: "ゴーヤ(GDD対応)", Selected: false}},
			want:  `x-data="{ showPlantingDate: false }"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := baseDeviceFormView()
			v.Crops = tt.crops
			html := render(t, DeviceForm(v))
			assertContains(t, html, tt.want)
		})
	}
}

// TestDeviceForm_定植日の項目別エラーを描画 は未来日等の検証エラーが planting_date 用 .error-message に出ることを固定する (R2.6)。
func TestDeviceForm_定植日の項目別エラーを描画(t *testing.T) {
	v := baseDeviceFormView()
	v.Errors = map[string]string{"planting_date": "定植日は未来日にできません"}
	html := render(t, DeviceForm(v))
	assertContains(t, html, "定植日は未来日にできません")
}

// TestDeviceForm_作物selectと空optionと選択復元 は栽培作物の検索可能 select が
// 所在地と同型で描画され、空 option (既定しきい値)・選択肢・選択値復元を持つことを固定する (R3.1/3.3)。
func TestDeviceForm_作物selectと空optionと選択復元(t *testing.T) {
	html := render(t, DeviceForm(baseDeviceFormView()))

	// 作物 select は所在地と同型の検索可能 select。
	assertContains(t, html, `name="crop"`)
	// 先頭の空 option (未選択=既定しきい値)。
	assertContains(t, html, `選択しない（既定しきい値）`)
	// 選択肢 (9作物のうち代表) と日本語ラベル。GDD 対応作物は「(GDD対応)」接尾辞付き。
	assertContains(t, html, "ゴーヤ(GDD対応)")
	assertContains(t, html, "インゲン(GDD対応)")
	assertContains(t, html, "サトウキビ") // 未対応は接尾辞なし
	// 保存値が option で選択復元される (goya=ゴーヤ・接尾辞込みラベル)。
	assertContains(t, html, `<option value="goya" selected>ゴーヤ(GDD対応)</option>`)
	// 作物用 select も js-tom-select (検索可能) で、所在地と合わせて2つ。
	if got := strings.Count(html, "js-tom-select"); got != 2 {
		t.Errorf("js-tom-select の数 = %d, want 2 (所在地+作物)", got)
	}
}

// TestDeviceForm_作物の項目別エラーを描画 は作物の検証エラーが crop 用 .error-message に出ることを固定する (R3.4)。
func TestDeviceForm_作物の項目別エラーを描画(t *testing.T) {
	v := baseDeviceFormView()
	v.Errors = map[string]string{"crop": "選択した作物が不正です"}
	html := render(t, DeviceForm(v))
	assertContains(t, html, "選択した作物が不正です")
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
	// 設置場所は単一の検索可能 地域 select。保存値が認識名 option で選択復元される。
	assertContains(t, html, `name="locality"`)
	assertContains(t, html, `js-tom-select`)
	assertContains(t, html, `<option value="佐敷町" selected>佐敷（南城市）</option>`)
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
		"locality":    "選択した地域が不正です",
		"is_active":   "ステータスが不正です",
	}
	html := render(t, DeviceForm(v))

	assertContains(t, html, "error-message")
	assertContains(t, html, "デバイス名を入力してください")
	assertContains(t, html, "MACアドレスを入力してください")
	assertContains(t, html, "選択した地域が不正です")
	assertContains(t, html, "ステータスが不正です")
}
