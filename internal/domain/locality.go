package domain

import "fmt"

// Locality は沖縄の「地域」（53）を表す Enum。所在地選択の正規マスタ。
// 沖縄の農家は市町村合併前（旧町村）の呼び名で土地を認識するため、認識名で1つ選ぶ平坦モデルを採る。
//
// 内訳（research.md 確定データ・confidence high）:
//   - 未合併36: 地域値＝市町村名・親市町村＝自身。
//   - 旧町村17: 平成の大合併5件（うるま市/宮古島市/南城市/八重瀬町/久米島町）の旧町村。
//     値は旧町村の正式名（例「具志川市」「具志川村」）で一意化し、親は現市町村。
//
// 値（string）は一意。同名異所（旧具志川市＝うるま市／旧具志川村＝久米島町）は
// 正式名で値を区別し、Label() で現市町村を併記して表示上も区別する。
type Locality string

// 未合併36地域（地域値＝市町村名・親＝自身）。
const (
	// 8市
	LocalityNaha       Locality = "那覇市"
	LocalityGinowan    Locality = "宜野湾市"
	LocalityIshigaki   Locality = "石垣市"
	LocalityUrasoe     Locality = "浦添市"
	LocalityNago       Locality = "名護市"
	LocalityItoman     Locality = "糸満市"
	LocalityOkinawa    Locality = "沖縄市"
	LocalityTomigusuku Locality = "豊見城市"
	// 9町
	LocalityMotobu    Locality = "本部町"
	LocalityKin       Locality = "金武町"
	LocalityKadena    Locality = "嘉手納町"
	LocalityChatan    Locality = "北谷町"
	LocalityNishihara Locality = "西原町"
	LocalityYonabaru  Locality = "与那原町"
	LocalityHaebaru   Locality = "南風原町"
	LocalityTaketomi  Locality = "竹富町"
	LocalityYonaguni  Locality = "与那国町"
	// 19村
	LocalityKunigami       Locality = "国頭村"
	LocalityOgimi          Locality = "大宜味村"
	LocalityHigashi        Locality = "東村"
	LocalityNakijin        Locality = "今帰仁村"
	LocalityOnna           Locality = "恩納村"
	LocalityGinoza         Locality = "宜野座村"
	LocalityIe             Locality = "伊江村"
	LocalityYomitan        Locality = "読谷村"
	LocalityKitanakagusuku Locality = "北中城村"
	LocalityNakagusuku     Locality = "中城村"
	LocalityTokashiki      Locality = "渡嘉敷村"
	LocalityZamami         Locality = "座間味村"
	LocalityAguni          Locality = "粟国村"
	LocalityTonaki         Locality = "渡名喜村"
	LocalityMinamidaito    Locality = "南大東村"
	LocalityKitadaito      Locality = "北大東村"
	LocalityIheya          Locality = "伊平屋村"
	LocalityIzena          Locality = "伊是名村"
	LocalityTarama         Locality = "多良間村"
)

// 旧町村17（平成の大合併・親＝現市町村）。
const (
	// うるま市(2005) ← 石川市・具志川市・与那城町・勝連町
	LocalityIshikawaShi  Locality = "石川市"
	LocalityGushikawaShi Locality = "具志川市"
	LocalityYonashiroCho Locality = "与那城町"
	LocalityKatsurenCho  Locality = "勝連町"
	// 宮古島市(2005) ← 平良市・城辺町・下地町・上野村・伊良部町
	LocalityHiraraShi   Locality = "平良市"
	LocalityGusukubeCho Locality = "城辺町"
	LocalityShimojiCho  Locality = "下地町"
	LocalityUenoSon     Locality = "上野村"
	LocalityIrabuCho    Locality = "伊良部町"
	// 南城市(2006) ← 佐敷町・知念村・玉城村・大里村
	LocalitySashikiCho    Locality = "佐敷町"
	LocalityChinenSon     Locality = "知念村"
	LocalityTamagusukuSon Locality = "玉城村"
	LocalityOzatoSon      Locality = "大里村"
	// 八重瀬町(2006) ← 東風平町・具志頭村
	LocalityKochindaCho  Locality = "東風平町"
	LocalityGushichanSon Locality = "具志頭村"
	// 久米島町(2002) ← 仲里村・具志川村
	LocalityNakazatoSon  Locality = "仲里村"
	LocalityGushikawaSon Locality = "具志川村"
)

// localityTable は地域→親市町村の対応（表示順）。AllLocalities/Valid/Municipality の単一ソース。
// 表示順は AllMunicipalities()（市→町→村）に沿い、合併市町村は旧町村へ展開する。
var localityTable = []struct {
	loc    Locality
	parent Municipality
}{
	// --- 市 ---
	{LocalityNaha, MunicipalityNaha},
	{LocalityGinowan, MunicipalityGinowan},
	{LocalityIshigaki, MunicipalityIshigaki},
	{LocalityUrasoe, MunicipalityUrasoe},
	{LocalityNago, MunicipalityNago},
	{LocalityItoman, MunicipalityItoman},
	{LocalityOkinawa, MunicipalityOkinawa},
	{LocalityTomigusuku, MunicipalityTomigusuku},
	// うるま市（旧4町村）
	{LocalityIshikawaShi, MunicipalityUruma},
	{LocalityGushikawaShi, MunicipalityUruma},
	{LocalityYonashiroCho, MunicipalityUruma},
	{LocalityKatsurenCho, MunicipalityUruma},
	// 宮古島市（旧5町村）
	{LocalityHiraraShi, MunicipalityMiyakojima},
	{LocalityGusukubeCho, MunicipalityMiyakojima},
	{LocalityShimojiCho, MunicipalityMiyakojima},
	{LocalityUenoSon, MunicipalityMiyakojima},
	{LocalityIrabuCho, MunicipalityMiyakojima},
	// 南城市（旧4町村）
	{LocalitySashikiCho, MunicipalityNanjo},
	{LocalityChinenSon, MunicipalityNanjo},
	{LocalityTamagusukuSon, MunicipalityNanjo},
	{LocalityOzatoSon, MunicipalityNanjo},
	// --- 町 ---
	{LocalityMotobu, MunicipalityMotobu},
	{LocalityKin, MunicipalityKin},
	{LocalityKadena, MunicipalityKadena},
	{LocalityChatan, MunicipalityChatan},
	{LocalityNishihara, MunicipalityNishihara},
	{LocalityYonabaru, MunicipalityYonabaru},
	{LocalityHaebaru, MunicipalityHaebaru},
	// 久米島町（旧2村）
	{LocalityNakazatoSon, MunicipalityKumejima},
	{LocalityGushikawaSon, MunicipalityKumejima},
	// 八重瀬町（旧2町村）
	{LocalityKochindaCho, MunicipalityYaese},
	{LocalityGushichanSon, MunicipalityYaese},
	{LocalityTaketomi, MunicipalityTaketomi},
	{LocalityYonaguni, MunicipalityYonaguni},
	// --- 村 ---
	{LocalityKunigami, MunicipalityKunigami},
	{LocalityOgimi, MunicipalityOgimi},
	{LocalityHigashi, MunicipalityHigashi},
	{LocalityNakijin, MunicipalityNakijin},
	{LocalityOnna, MunicipalityOnna},
	{LocalityGinoza, MunicipalityGinoza},
	{LocalityIe, MunicipalityIe},
	{LocalityYomitan, MunicipalityYomitan},
	{LocalityKitanakagusuku, MunicipalityKitanakagusuku},
	{LocalityNakagusuku, MunicipalityNakagusuku},
	{LocalityTokashiki, MunicipalityTokashiki},
	{LocalityZamami, MunicipalityZamami},
	{LocalityAguni, MunicipalityAguni},
	{LocalityTonaki, MunicipalityTonaki},
	{LocalityMinamidaito, MunicipalityMinamidaito},
	{LocalityKitadaito, MunicipalityKitadaito},
	{LocalityIheya, MunicipalityIheya},
	{LocalityIzena, MunicipalityIzena},
	{LocalityTarama, MunicipalityTarama},
}

// localityParent は localityTable から導出した 地域→親市町村 マップ（Municipality/Valid 用）。
var localityParent = func() map[Locality]Municipality {
	m := make(map[Locality]Municipality, len(localityTable))
	for _, e := range localityTable {
		m[e.loc] = e.parent
	}
	return m
}()

// localityAlias は ParseLocality 用の 入力文字列→地域 マップ。
// キーは「正式名（値）」と「一意な短縮名」。同名で曖昧な短縮名（具志川）は登録しない。
var localityAlias = func() map[string]Locality {
	m := make(map[string]Locality, len(localityTable)*2)
	// 正式名（値）。未合併は市町村名＝現市町村名のエイリアスも兼ねる。
	for _, e := range localityTable {
		m[string(e.loc)] = e.loc
	}
	// 短縮名は重複（同名）を除いて一意なものだけ登録する。
	shortCount := make(map[string]int, len(localityTable))
	for _, e := range localityTable {
		shortCount[localityShortName(string(e.loc))]++
	}
	for _, e := range localityTable {
		s := localityShortName(string(e.loc))
		if shortCount[s] == 1 {
			if _, exists := m[s]; !exists {
				m[s] = e.loc
			}
		}
	}
	return m
}()

// localityShortName は末尾の「市/町/村」を1文字落とした短縮名を返す（例「佐敷町」→「佐敷」）。
func localityShortName(name string) string {
	r := []rune(name)
	if n := len(r); n > 0 {
		switch r[n-1] {
		case '市', '町', '村':
			return string(r[:n-1])
		}
	}
	return name
}

// Label は画面表示用の認識名を返す。
// 合併地域は「短縮名（現市町村）」（例「佐敷（南城市）」）、未合併は市町村名そのもの。
func (l Locality) Label() string {
	parent := l.Municipality()
	if string(l) == string(parent) {
		// 未合併（親＝自身）＝市町村名そのもの。
		return string(l)
	}
	// 合併地域＝旧町村の短縮名に現市町村を併記して同名を区別する。
	return localityShortName(string(l)) + "（" + string(parent) + "）"
}

// Municipality は親市町村（集計軸）を返す。合併＝現市町村、未合併＝自身。
// 有効な Locality に対しては常に有効な Municipality を返す。
func (l Locality) Municipality() Municipality {
	return localityParent[l]
}

// Valid は Enum として定義された地域かを判定する。
func (l Locality) Valid() bool {
	_, ok := localityParent[l]
	return ok
}

// ParseLocality は文字列から Locality への変換を試みる。
// 正式名（値）・現市町村名（未合併のみ）・一意な短縮名（旧名）のエイリアスを解決する。
// 同名で曖昧な短縮名（具志川）や、複数地域に分かれる合併後市町村名（うるま市等）は解決できずエラー。
// 既存 location の非破壊移行（backfill）で旧名/正式名/現市町村名を吸収するために使う。
func ParseLocality(s string) (Locality, error) {
	if l, ok := localityAlias[s]; ok {
		return l, nil
	}
	return "", fmt.Errorf("invalid locality: %q", s)
}

// AllLocalities は定義済み地域の全列挙（表示順）。フォーム選択肢の生成等に使用。
func AllLocalities() []Locality {
	out := make([]Locality, 0, len(localityTable))
	for _, e := range localityTable {
		out = append(out, e.loc)
	}
	return out
}
