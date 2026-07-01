package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/view/component"
)

// --- 5.1 デバイスフォームの作物バインド・検証・復元・保存 (locality 写経) ---

func TestCreate_作物は非nilのcropで保存(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.createResult = repository.Device{ID: 21, UserID: 7}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("crop", "goya")
	w := formRequest(r, http.MethodPost, "/devices", vals)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303 (body=%s)", w.Code, w.Body.String())
	}
	if repo.lastCreate.Crop == nil || *repo.lastCreate.Crop != "goya" {
		t.Errorf("Crop=%v, want &\"goya\"", repo.lastCreate.Crop)
	}
}

func TestCreate_作物未選択はnil_cropで保存(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.createResult = repository.Device{ID: 22, UserID: 7}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("crop", "") // 未選択 (任意項目)
	w := formRequest(r, http.MethodPost, "/devices", vals)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303 (未選択も成功)", w.Code)
	}
	if repo.lastCreate.Crop != nil {
		t.Errorf("Crop=%v, want nil (未選択は未設定=既定帯)", repo.lastCreate.Crop)
	}
}

func TestCreate_不正な作物は200で作成せずエラー(t *testing.T) {
	repo := deviceOwnerRepo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("crop", "tomato") // 9作物に無い値
	w := formRequest(r, http.MethodPost, "/devices", vals)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (再描画)", w.Code)
	}
	if repo.createCalled {
		t.Error("不正な作物で CreateDevice を呼んではいけない")
	}
	if !strings.Contains(w.Body.String(), cropInvalidMessage) {
		t.Error("作物不正エラーが表示されていない")
	}
}

func TestUpdate_作物を更新して保存(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.devices = map[int64]repository.Device{
		1: {ID: 1, UserID: 7, Name: "ハウスA", MacAddress: "AA:BB:CC:DD:EE:01", IsActive: true},
	}
	repo.updateResult = repository.Device{ID: 1, UserID: 7}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("crop", "ingen")
	w := formRequest(r, http.MethodPut, "/devices/1", vals)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303 (body=%s)", w.Code, w.Body.String())
	}
	if repo.lastUpdate.Crop == nil || *repo.lastUpdate.Crop != "ingen" {
		t.Errorf("Crop=%v, want &\"ingen\" (フォームの作物)", repo.lastUpdate.Crop)
	}
}

// TestCropOptions_GDD対応作物に接尾辞 は作物 select の選択肢ラベルに、GDD 具体モデルを持つ作物
// (米・ゴーヤ・インゲン・ウリ・いも) だけ「(GDD対応)」が付き、他作物には付かないことを固定する。
// 値 (Value) は不変 (接尾辞は表示専用) で、選択復元 (Selected) が正しく効くことも確認する。
func TestCropOptions_GDD対応作物に接尾辞(t *testing.T) {
	opts := cropOptions("goya")
	byValue := make(map[string]component.SelectOption, len(opts))
	for _, o := range opts {
		byValue[o.Value] = o
	}

	// GDD 対応作物: ラベル末尾に「(GDD対応)」が付き、値は素のキーのまま。
	supported := map[string]string{
		"rice": "米", "goya": "ゴーヤ", "ingen": "インゲン", "uri": "ウリ", "imo": "いも", "leafy_vegetable": "葉野菜",
	}
	for value, base := range supported {
		o, ok := byValue[value]
		if !ok {
			t.Fatalf("作物 %q の選択肢が無い", value)
		}
		if want := base + "(GDD対応)"; o.Label != want {
			t.Errorf("%q の Label = %q, want %q", value, o.Label, want)
		}
	}

	// 未対応作物: 接尾辞は付かない (素の日本語ラベル)。
	unsupported := map[string]string{
		"sugarcane": "サトウキビ", "mango": "マンゴー", "pineapple": "パイナップル",
	}
	for value, base := range unsupported {
		o := byValue[value]
		if o.Label != base {
			t.Errorf("%q の Label = %q, want %q (未対応は接尾辞なし)", value, o.Label, base)
		}
		if strings.Contains(o.Label, "GDD対応") {
			t.Errorf("未対応作物 %q に (GDD対応) が付いている: %q", value, o.Label)
		}
	}

	// 選択復元 (Selected) は Value 一致で効く (接尾辞はラベルのみゆえ影響しない)。
	if !byValue["goya"].Selected {
		t.Errorf("selected=goya なのに goya が Selected でない")
	}
	if byValue["rice"].Selected {
		t.Errorf("selected=goya なのに rice が Selected になっている")
	}
}

// validCropInput は空 (任意) と作物マスタ定義値のみ許可する純関数 (locality 写経)。
func TestValidCropInput(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"", true},                // 未選択 (任意項目)
		{"goya", true},            // 定義値
		{"leafy_vegetable", true}, // 定義値
		{"tomato", false},         // 未定義
		{"GOYA", false},           // 大文字は別値
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := validCropInput(tt.in); got != tt.want {
				t.Errorf("validCropInput(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
