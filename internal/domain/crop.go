package domain

import "fmt"

// Crop は栽培作物を表す Enum。VPD 適正帯の切替に使う作物マスタ (Go 定数)。
// DB には文字列値として devices.crop に格納され、00009 マイグレーションの
// CHECK 制約 (devices_crop_valid) と二重ミラーで同期する。
// ※ この作物集合を変更したら devices.crop の CHECK・sqlc・フォーム選択肢も同期し
//
//	make db-snapshot を再実行すること (design.md Revalidation Triggers)。
//
// 識別子 (値) は英語キー、Label() は日本語。作物別の派生指標属性 — VPD 適正帯 (VPDRange)・
// 病害モデル (DiseaseModel)・GDD モデル (GDDModel) — を保持する (いずれも DB 列を増やさず Go 定数)。
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

// DiseaseModel は作物別の病害しきい値（結露・葉面湿潤・発病好適温度帯）。
// VPDRange と同じく DB には持たず Go 定数で保持する（§100・DB 列を増やさない＝スキーマ非変更）。
// 葉面温度センサ不在のため葉面温度は気温で近似し、結露は露点スプレッド T−Td の代理しきい値で判定する。
// 値は文献ベースの暫定値で、確定はユーザー（沖縄実地知見=権威）/文献で行い本型と crop_test.go を更新する。
type DiseaseModel struct {
	CondensationMaxSpread float64 // 結露帯と見なすスプレッド T−Td の上限 [℃]（葉面温度=気温近似の代理）
	WetnessRHThreshold    float64 // 葉面湿潤/高湿度と見なす RH 下限 [%]
	HighHumidityMinHours  float64 // 高湿度イベントの最小継続 [時間]（handler が点数 minRun へ換算）
	DiseaseTempLow        float64 // 発病好適温度帯 下限 [℃]
	DiseaseTempHigh       float64 // 発病好適温度帯 上限 [℃]
}

// DefaultDiseaseModel は作物未設定・露地・未定義作物のフォールバック病害モデル（要件 5.4/6.3）。
// 既定でも全作物で病害スコアが具体値を持つため、病害スコア欄は常に非空になる（要件 5.2）。
var DefaultDiseaseModel = DiseaseModel{
	CondensationMaxSpread: 2.0,
	WetnessRHThreshold:    90,
	HighHumidityMinHours:  1.0,
	DiseaseTempLow:        15,
	DiseaseTempHigh:       25,
}

// DiseaseModel は作物別の病害モデルしきい値を返す（要件 6.1/6.2）。
// 未知・空・病害属性未定義の作物は DefaultDiseaseModel にフォールバックする（要件 5.4/6.3）。
// 常に DiseaseTempLow <= DiseaseTempHigh を満たす（DiseaseScore の事前条件）。
// VPDRange と同じ作物グルーピング作法で、値の確定時にこの1メソッドを更新する。
func (c Crop) DiseaseModel() DiseaseModel {
	switch c {
	case CropGoya, CropIngen, CropUri, CropMango:
		// 施設果菜（灰色かび病・うどんこ病の発病好適温度帯・暫定）。
		return DiseaseModel{
			CondensationMaxSpread: 2.0,
			WetnessRHThreshold:    90,
			HighHumidityMinHours:  1.0,
			DiseaseTempLow:        15,
			DiseaseTempHigh:       25,
		}
	case CropLeafyVegetable:
		// 施設葉菜・温度帯やや狭め（暫定）。
		return DiseaseModel{
			CondensationMaxSpread: 2.0,
			WetnessRHThreshold:    90,
			HighHumidityMinHours:  1.0,
			DiseaseTempLow:        15,
			DiseaseTempHigh:       22,
		}
	}
	// 露地（サトウキビ/米/パイナップル/いも）・未設定・不正は既定にフォールバック。
	return DefaultDiseaseModel
}

// GrowthStage は生育ステージ1段（表示名＋到達しきい値 GDD）。
// Stages は GDD 昇順で並べ、最終段の GDD が収穫目標を表す（GrowthStageIndex の昇順前提）。
type GrowthStage struct {
	Name string  // 表示用ステージ名（"発芽"/"出穂"/"収穫" 等）
	GDD  float64 // この段に到達する累積 GDD しきい値 [℃·日]
}

// GDDModel は作物別の GDD（積算温度）モデル。VPDRange/DiseaseModel と同じく
// DB には持たず Go 定数で保持する（§100・DB 列を増やさない＝スキーマ非変更）。
// Stages が空でも Tbase で日次 GDD・累積は算出でき、収穫目標・残り積算・予測のみ縮退する（要件 5.4）。
// 値は文献ベースの暫定値で、確定はユーザー（沖縄実地知見=権威）/文献で行い本型と crop_test.go を更新する。
type GDDModel struct {
	Tbase  float64       // 基準温度 [℃]（日次 GDD = max((Tmax+Tmin)/2 − Tbase, 0)）
	Stages []GrowthStage // GDD 昇順。最終段=収穫目標。空でも Tbase で日次/累積は算出可
}

// DefaultGDDModel は作物未設定・GDD 属性未定義作物のフォールバック GDD モデル（要件 5.4）。
// 既定 Tbase 10℃ で日次 GDD・累積は算出できるが、Stages 空ゆえ収穫目標・残り積算・到達日予測は
// handler が "—" に縮退する（生育ステージ表も空）。
var DefaultGDDModel = GDDModel{Tbase: 10, Stages: nil}

// GDDModel は作物別の GDD モデルを返す（要件 5.1〜5.3）。
// 未知・空・GDD 属性未定義の作物は DefaultGDDModel にフォールバックする（要件 5.4）。
// Stages は常に GDD 昇順・最終段=収穫目標を満たす。VPDRange/DiseaseModel と同じ作物グルーピング作法で、
// 値の確定時にこの1メソッドを更新する。本フェーズは米のみ具体値（他は段階拡張）。
func (c Crop) GDDModel() GDDModel {
	switch c {
	case CropRice:
		// 水稲（二期作の出穂予測=GDD の教科書作物）。Tbase=10℃・有効積算温度の暫定ステージ（℃·日）。
		// 数値は文献ベースの暫定値で、ユーザー（沖縄実地知見=権威）/文献の確定時に更新する（GO 後スモークで目視）。
		return GDDModel{
			Tbase: 10,
			Stages: []GrowthStage{
				{Name: "発芽", GDD: 0},
				{Name: "分げつ", GDD: 300},
				{Name: "出穂", GDD: 800},
				{Name: "登熟", GDD: 1100},
				{Name: "収穫", GDD: 1400},
			},
		}
	}
	// サトウキビ/ゴーヤ等・未設定・不正は既定（Tbase のみ・Stages 空）にフォールバック。
	return DefaultGDDModel
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
