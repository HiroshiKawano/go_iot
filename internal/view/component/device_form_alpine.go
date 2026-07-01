package component

import "strings"

// device_form_alpine.go は DeviceForm の「GDD 収穫予測用(定植/播種日)」フィールドを、
// 栽培作物で GDD 対応作物（ラベルに (GDD対応) 接尾辞あり）を選んだ時だけ表示する Alpine 連携の補助。
// 判定はラベルの接尾辞で行う（サーバ初期表示・クライアント @change のどちらも同じ規約）。

// GDDCropLabelSuffix は栽培作物 select で GDD 収穫予測対応作物のラベル末尾に付す接尾辞（表示専用）。
// handler(cropOptions) がラベル生成に、本パッケージが定植日フィールドの初期表示判定に使う単一の真実源。
// クライアント側 Alpine の @change でも同一リテラル "(GDD対応)" で判定する（DeviceForm.templ 参照）。
const GDDCropLabelSuffix = "(GDD対応)"

// selectedCropIsGDD は選択中の作物が GDD 収穫予測対応（ラベルに GDDCropLabelSuffix を含む）かを返す。
// 未選択（空 option）や未対応作物では false。定植日フィールドの初期表示（サーバ側）判定に使う。
func selectedCropIsGDD(v DeviceFormView) bool {
	for _, o := range v.Crops {
		if o.Selected {
			return strings.Contains(o.Label, GDDCropLabelSuffix)
		}
	}
	return false
}

// deviceFormAlpineData は DeviceForm の Alpine x-data 初期状態（JS オブジェクトリテラル文字列）を返す。
// showPlantingDate = 選択中作物が GDD 対応か。初期値をサーバで確定しておくことで、対応作物選択済みの
// 編集画面で定植日フィールドがちらつかず即表示される（未対応・未選択なら初期非表示）。
func deviceFormAlpineData(v DeviceFormView) string {
	if selectedCropIsGDD(v) {
		return "{ showPlantingDate: true }"
	}
	return "{ showPlantingDate: false }"
}
