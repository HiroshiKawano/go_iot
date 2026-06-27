package handler

import (
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/view/component"
)

// TestBuildDeviceInfoView_所在地は認識名で未設定は未設定表記 はデバイス行→情報パネル View の
// 所在地写像を検証する (R6.1/R6.3)。所在地表示は自由入力 location ではなく構造化 locality を
// 認識名 (合併=「旧町村（現市町村）」/未合併=市町村名) で出し、未設定・未知値は従来同等の「未設定」とする。
func TestBuildDeviceInfoView_所在地は認識名で未設定は未設定表記(t *testing.T) {
	tests := []struct {
		name         string
		device       repository.Device
		wantLocation string
	}{
		{
			name:         "合併地域は旧町村（現市町村）の認識名",
			device:       repository.Device{ID: 1, Name: "D", MacAddress: "AA:BB:CC:DD:EE:01", Locality: strPtr("佐敷町")},
			wantLocation: "佐敷（南城市）",
		},
		{
			name:         "未合併は市町村名そのもの",
			device:       repository.Device{ID: 1, Name: "D", MacAddress: "AA:BB:CC:DD:EE:01", Locality: strPtr("名護市")},
			wantLocation: "名護市",
		},
		{
			name:         "同名異所は現市町村併記で区別",
			device:       repository.Device{ID: 1, Name: "D", MacAddress: "AA:BB:CC:DD:EE:01", Locality: strPtr("具志川村")},
			wantLocation: "具志川（久米島町）",
		},
		{
			name:         "所在地未設定(nil)は未設定",
			device:       repository.Device{ID: 1, Name: "D", MacAddress: "AA:BB:CC:DD:EE:01", Locality: nil},
			wantLocation: "未設定",
		},
		{
			name:         "自由入力location残置でもlocality未設定なら未設定（表示はlocalityへ切替済）",
			device:       repository.Device{ID: 1, Name: "D", MacAddress: "AA:BB:CC:DD:EE:01", Location: strPtr("旧自由入力ハウスA"), Locality: nil},
			wantLocation: "未設定",
		},
		{
			name:         "未知のlocality値は未設定（防御的）",
			device:       repository.Device{ID: 1, Name: "D", MacAddress: "AA:BB:CC:DD:EE:01", Locality: strPtr("存在しない地域")},
			wantLocation: "未設定",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDeviceInfoView(tt.device)
			if got.Location != tt.wantLocation {
				t.Errorf("Location = %q, want %q", got.Location, tt.wantLocation)
			}
		})
	}
}

// TestBuildDeviceInfoView_所在地以外の項目は従来どおり写像 は所在地切替が他項目の写像を
// 壊していないことを確認する回帰ガード (Name/MAC/状態/編集URL/未通信)。
func TestBuildDeviceInfoView_所在地以外の項目は従来どおり写像(t *testing.T) {
	d := repository.Device{
		ID: 5, Name: "ハウスA温湿度計", MacAddress: "AA:BB:CC:DD:EE:05",
		IsActive: true, Locality: strPtr("佐敷町"),
		// LastCommunicatedAt は Valid=false (未通信)
	}
	want := component.DeviceInfoView{
		Name:         "ハウスA温湿度計",
		MacAddress:   "AA:BB:CC:DD:EE:05",
		Location:     "佐敷（南城市）",
		StatusActive: true,
		LastCommText: "未通信",
		EditURL:      "/devices/5/edit",
	}
	if got := buildDeviceInfoView(d); got != want {
		t.Errorf("buildDeviceInfoView() =\n  %+v\nwant\n  %+v", got, want)
	}
}
