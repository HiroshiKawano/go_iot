package domain

import "fmt"

// Crop は栽培作物を表す Enum。VPD 適正帯の切替に使う作物マスタ (Go 定数)。
// DB には文字列値として devices.crop に格納され、00009 マイグレーションの
// CHECK 制約 (devices_crop_valid) と二重ミラーで同期する。
// ※ この作物集合を変更したら devices.crop の CHECK・sqlc・フォーム選択肢も同期し
//
//	make db-snapshot を再実行すること (design.md Revalidation Triggers)。
//
// 識別子 (値) は英語キー、Label() は日本語。VPD 適正帯のみを保持し、
// GDD 基準温度・病害モデル等の他属性は別フェーズが非破壊的に追加する前提。
type Crop string

const (
	CropGoya           Crop = "goya"
	CropIngen          Crop = "ingen"
	CropSugarcane      Crop = "sugarcane"
	CropMango          Crop = "mango"
	CropPineapple      Crop = "pineapple"
	CropUri            Crop = "uri"
	CropRice           Crop = "rice"
	CropImo            Crop = "imo"
	CropLeafyVegetable Crop = "leafy_vegetable"
)

// 既定 VPD 適正帯 (kPa)。作物未設定・未知・固有帯未定義のフォールバック値 (要件 2.3/2.4)。
const (
	DefaultVPDLower = 0.3
	DefaultVPDUpper = 1.5
)

// Label は画面表示用の日本語ラベルを返す。
func (c Crop) Label() string {
	switch c {
	case CropGoya:
		return "ゴーヤ"
	case CropIngen:
		return "インゲン"
	case CropSugarcane:
		return "サトウキビ"
	case CropMango:
		return "マンゴー"
	case CropPineapple:
		return "パイナップル"
	case CropUri:
		return "ウリ"
	case CropRice:
		return "米"
	case CropImo:
		return "いも"
	case CropLeafyVegetable:
		return "葉野菜"
	}
	return string(c)
}

// Valid は Enum として定義された作物かを判定する。
func (c Crop) Valid() bool {
	switch c {
	case CropGoya, CropIngen, CropSugarcane, CropMango,
		CropPineapple, CropUri, CropRice, CropImo, CropLeafyVegetable:
		return true
	}
	return false
}

// VPDRange は作物別の VPD 適正帯 (下限/上限 kPa) を返す。
// 未知・空・固有帯が未定義の作物は既定 (DefaultVPDLower, DefaultVPDUpper) にフォールバックする。
// 値は文献ベースの暫定値で、ユーザー (沖縄実地知見=権威)/文献の確定時にこの1メソッドを更新する。
// 常に lower <= upper を満たす (滞在率/markArea の前提条件)。
func (c Crop) VPDRange() (lower, upper float64) {
	switch c {
	case CropGoya, CropIngen, CropUri, CropMango:
		// 施設果菜・VPD 本命 (暫定)。
		return 0.4, 1.2
	case CropLeafyVegetable:
		// 施設葉菜・低め (暫定)。
		return 0.3, 1.0
	}
	// 露地 (サトウキビ/米/パイナップル/いも)・未設定・不正は既定帯。
	return DefaultVPDLower, DefaultVPDUpper
}

// ParseCrop は文字列から Crop への変換を試み、不正値ならエラーを返す。
func ParseCrop(s string) (Crop, error) {
	c := Crop(s)
	if !c.Valid() {
		return "", fmt.Errorf("invalid crop: %q", s)
	}
	return c, nil
}

// AllCrops は定義済み作物の全列挙 (表示順)。フォーム選択肢の生成等に使用。
// 並びは要件 2.1・devices.crop CHECK と一致させる。
func AllCrops() []Crop {
	return []Crop{
		CropGoya, CropIngen, CropSugarcane, CropMango,
		CropPineapple, CropUri, CropRice, CropImo, CropLeafyVegetable,
	}
}
