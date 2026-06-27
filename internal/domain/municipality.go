package domain

// Municipality は沖縄県の市町村（41）を表す Enum。所在地の集計軸（地域の親）として使う。
// Locality.Municipality() が返す親市町村であり、合併地域は現市町村、未合併地域は自身を指す。
// DB には文字列値として格納される想定で、devices.locality は Locality を保持し
// 市町村は導出するため、現状 DB 列には現れない（YAGNI・将来 SQL 集計で非破壊に追加可）。
// 確定データの出典は research.md（総務省 市町村合併資料集／沖縄県公式 ほか・confidence high）。
type Municipality string

// 11市。
const (
	MunicipalityNaha       Municipality = "那覇市"
	MunicipalityGinowan    Municipality = "宜野湾市"
	MunicipalityIshigaki   Municipality = "石垣市"
	MunicipalityUrasoe     Municipality = "浦添市"
	MunicipalityNago       Municipality = "名護市"
	MunicipalityItoman     Municipality = "糸満市"
	MunicipalityOkinawa    Municipality = "沖縄市"
	MunicipalityTomigusuku Municipality = "豊見城市"
	MunicipalityUruma      Municipality = "うるま市"
	MunicipalityMiyakojima Municipality = "宮古島市"
	MunicipalityNanjo      Municipality = "南城市"
)

// 11町。
const (
	MunicipalityMotobu    Municipality = "本部町"
	MunicipalityKin       Municipality = "金武町"
	MunicipalityKadena    Municipality = "嘉手納町"
	MunicipalityChatan    Municipality = "北谷町"
	MunicipalityNishihara Municipality = "西原町"
	MunicipalityYonabaru  Municipality = "与那原町"
	MunicipalityHaebaru   Municipality = "南風原町"
	MunicipalityKumejima  Municipality = "久米島町"
	MunicipalityYaese     Municipality = "八重瀬町"
	MunicipalityTaketomi  Municipality = "竹富町"
	MunicipalityYonaguni  Municipality = "与那国町"
)

// 19村。
const (
	MunicipalityKunigami       Municipality = "国頭村"
	MunicipalityOgimi          Municipality = "大宜味村"
	MunicipalityHigashi        Municipality = "東村"
	MunicipalityNakijin        Municipality = "今帰仁村"
	MunicipalityOnna           Municipality = "恩納村"
	MunicipalityGinoza         Municipality = "宜野座村"
	MunicipalityIe             Municipality = "伊江村"
	MunicipalityYomitan        Municipality = "読谷村"
	MunicipalityKitanakagusuku Municipality = "北中城村"
	MunicipalityNakagusuku     Municipality = "中城村"
	MunicipalityTokashiki      Municipality = "渡嘉敷村"
	MunicipalityZamami         Municipality = "座間味村"
	MunicipalityAguni          Municipality = "粟国村"
	MunicipalityTonaki         Municipality = "渡名喜村"
	MunicipalityMinamidaito    Municipality = "南大東村"
	MunicipalityKitadaito      Municipality = "北大東村"
	MunicipalityIheya          Municipality = "伊平屋村"
	MunicipalityIzena          Municipality = "伊是名村"
	MunicipalityTarama         Municipality = "多良間村"
)

// allMunicipalities は定義順（市→町→村）の全市町村。AllMunicipalities/Valid の単一ソース。
var allMunicipalities = []Municipality{
	// 11市
	MunicipalityNaha, MunicipalityGinowan, MunicipalityIshigaki, MunicipalityUrasoe,
	MunicipalityNago, MunicipalityItoman, MunicipalityOkinawa, MunicipalityTomigusuku,
	MunicipalityUruma, MunicipalityMiyakojima, MunicipalityNanjo,
	// 11町
	MunicipalityMotobu, MunicipalityKin, MunicipalityKadena, MunicipalityChatan,
	MunicipalityNishihara, MunicipalityYonabaru, MunicipalityHaebaru, MunicipalityKumejima,
	MunicipalityYaese, MunicipalityTaketomi, MunicipalityYonaguni,
	// 19村
	MunicipalityKunigami, MunicipalityOgimi, MunicipalityHigashi, MunicipalityNakijin,
	MunicipalityOnna, MunicipalityGinoza, MunicipalityIe, MunicipalityYomitan,
	MunicipalityKitanakagusuku, MunicipalityNakagusuku, MunicipalityTokashiki, MunicipalityZamami,
	MunicipalityAguni, MunicipalityTonaki, MunicipalityMinamidaito, MunicipalityKitadaito,
	MunicipalityIheya, MunicipalityIzena, MunicipalityTarama,
}

// validMunicipality は Valid() の O(1) 判定用。allMunicipalities から導出。
var validMunicipality = func() map[Municipality]bool {
	m := make(map[Municipality]bool, len(allMunicipalities))
	for _, x := range allMunicipalities {
		m[x] = true
	}
	return m
}()

// Label は画面表示用のラベルを返す。集計軸の市町村名そのもの。
func (m Municipality) Label() string {
	return string(m)
}

// Valid は Enum として定義された市町村かを判定する。
func (m Municipality) Valid() bool {
	return validMunicipality[m]
}

// AllMunicipalities は定義済み市町村の全列挙（定義順）。集計やテストに使用。
func AllMunicipalities() []Municipality {
	return append([]Municipality(nil), allMunicipalities...)
}
