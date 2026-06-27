package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// locPtr は所在地 (*string) の組み立てヘルパ。
func locPtr(s string) *string { return &s }

// --- 5.1 登録〜表示の全体フロー疎通 (cmd/server 合成ハンドラ) ---

// TestIntegration_地域登録から詳細とカード表示まで疎通 は、地域 select を持つ登録フォーム →
// CSRF 往復 POST で 303 → 詳細・ダッシュボードに認識名で所在地が出るまでを 1 本に通す
// (R1.1 単一地域 select / R3.1・R3.3 保存と既存フロー維持 / R6.1 詳細 / R6.2 カード)。
func TestIntegration_地域登録から詳細とカード表示まで疎通(t *testing.T) {
	devices := map[int64]repository.Device{
		10: {ID: 10, UserID: 1, Name: "ハウスA温湿度計", MacAddress: "AA:BB:CC:DD:EE:10", IsActive: true, Locality: locPtr("佐敷町")},
	}
	created := repository.Device{ID: 10, UserID: 1, Locality: locPtr("佐敷町")}
	app := deviceApp(devices, created, repository.Device{})
	cookies := loginCookies(t, app)

	// 1. 登録フォーム: 設置場所は単一の検索可能 select (Tom Select)・先頭に空 option・認識名 option。
	//    旧来の自由入力 text (name="location") は廃し locality へ切替済み。
	cw := getWithCookies(app, "/devices/create", cookies)
	if cw.Code != http.StatusOK {
		t.Fatalf("GET /devices/create = %d, want 200", cw.Code)
	}
	createBody := cw.Body.String()
	for _, want := range []string{
		`<select name="locality" class="js-tom-select">`, // 単一の検索可能 select (R1.1/R1.4)
		`<option value="">選択してください</option>`,           // 先頭の空 option (R1.3 任意)
		"佐敷（南城市）",                                       // 認識名 option (R2.1)
	} {
		if !strings.Contains(createBody, want) {
			t.Errorf("登録フォームに %q が含まれていない", want)
		}
	}
	if strings.Contains(createBody, `name="location"`) {
		t.Error("登録フォームに旧来の自由入力 location が残存している (locality へ切替のはず)")
	}

	token := extractCSRFToken(createBody)
	if token == "" {
		t.Fatal("登録フォームから CSRF トークンを取得できない")
	}
	cookies = mergeCookies(cookies, cw.Result().Cookies())

	// 2. 地域を選んで登録 → 303 /devices/10 (既存フロー維持・R3.1/R3.3)。
	form := url.Values{
		"name":               {"ハウスA温湿度計"},
		"mac_address":        {"AA:BB:CC:DD:EE:10"},
		"locality":           {"佐敷町"},
		"is_active":          {"1"},
		"gorilla.csrf.Token": {token},
	}
	pw := postFormWithCookies(app, "/devices", form, cookies)
	if pw.Code != http.StatusSeeOther || pw.Header().Get("Location") != "/devices/10" {
		t.Fatalf("地域付き POST /devices = %d Location=%q, want 303 /devices/10 (body=%s)", pw.Code, pw.Header().Get("Location"), pw.Body.String())
	}

	// 3. 詳細の情報パネルに認識名で所在地が出る (R6.1)。
	dw := getWithCookies(app, "/devices/10", cookies)
	if dw.Code != http.StatusOK {
		t.Fatalf("GET /devices/10 = %d, want 200 (body=%s)", dw.Code, dw.Body.String())
	}
	if !strings.Contains(dw.Body.String(), "佐敷（南城市）") {
		t.Errorf("詳細の情報パネルに認識名「佐敷（南城市）」が表示されていない:\n%s", dw.Body.String())
	}

	// 4. ダッシュボードのカードにも認識名で所在地が出る (R6.2)。
	bw := getWithCookies(app, "/dashboard", cookies)
	if bw.Code != http.StatusOK {
		t.Fatalf("GET /dashboard = %d, want 200", bw.Code)
	}
	board := bw.Body.String()
	if !strings.Contains(board, `id="device-card-10"`) {
		t.Error("ダッシュボードに device-card-10 が描画されていない")
	}
	if !strings.Contains(board, "佐敷（南城市）") {
		t.Errorf("ダッシュボードのカードに認識名「佐敷（南城市）」が表示されていない:\n%s", board)
	}
}

// --- 5.2 地域選択の E2E 相当 (サーバ契約レベル・Tom Select が消費する選択肢/復元を検証) ---

// TestIntegration_地域選択肢は認識名と同名区別で53件 は、地域 select の選択肢が沖縄53地域のみ
// (先頭の空 option を含め54 option)、認識名で提示され (旧町村は「短縮名（現市町村）」/未合併は
// 市町村名)、同名 (具志川) が現市町村併記で区別されることを検証する (R1.2/R2.1/R2.2/R2.3)。
// 旧町村名・現市町村名のいずれの入力でも候補に出せるよう、合併地域の label は両者を含む (R2.4)。
func TestIntegration_地域選択肢は認識名と同名区別で53件(t *testing.T) {
	app := deviceApp(nil, repository.Device{}, repository.Device{})
	cookies := loginCookies(t, app)

	cw := getWithCookies(app, "/devices/create", cookies)
	if cw.Code != http.StatusOK {
		t.Fatalf("GET /devices/create = %d, want 200", cw.Code)
	}
	body := cw.Body.String()

	for _, want := range []string{
		`<option value="">選択してください</option>`,        // 任意 (空) (R1.3)
		`<option value="佐敷町">佐敷（南城市）</option>`,        // 合併=旧町村（現市町村） (R2.1)・label に両名 (R2.4)
		`<option value="名護市">名護市</option>`,            // 未合併=市町村名 (R2.2)
		`<option value="具志川市">具志川（うるま市）</option>`,     // 同名の一方 (R2.3)
		`<option value="具志川村">具志川（久米島町）</option>`,     // 同名の他方 (R2.3)
	} {
		if !strings.Contains(body, want) {
			t.Errorf("地域選択肢に %q が含まれていない", want)
		}
	}

	// 沖縄53地域 + 先頭の空 option = 54 option ちょうど (locality select 内のみ・他都道府県/作物を含めない・R1.2)。
	// ページ全体には別途 作物 select も存在するため、locality の <select>…</select> 区間に限定して数える。
	localitySelect := body
	if i := strings.Index(localitySelect, `name="locality"`); i >= 0 {
		localitySelect = localitySelect[i:]
		if j := strings.Index(localitySelect, "</select>"); j >= 0 {
			localitySelect = localitySelect[:j]
		}
	}
	if got := strings.Count(localitySelect, "<option"); got != 54 {
		t.Errorf("locality の option 数 = %d, want 54 (沖縄53地域 + 空 option。他都道府県を含めない)", got)
	}
}

// TestIntegration_未選択での登録は成功303 は、設置場所を未選択 (空) のままでも他項目が妥当なら
// 登録が成功し詳細へ 303 リダイレクトすることを検証する (R3.2 任意項目)。
func TestIntegration_未選択での登録は成功303(t *testing.T) {
	app := deviceApp(nil, repository.Device{ID: 20, UserID: 1}, repository.Device{})
	cookies := loginCookies(t, app)

	cw := getWithCookies(app, "/devices/create", cookies)
	token := extractCSRFToken(cw.Body.String())
	cookies = mergeCookies(cookies, cw.Result().Cookies())

	form := url.Values{
		"name":               {"地域未選択デバイス"},
		"mac_address":        {"AA:BB:CC:DD:EE:20"},
		"locality":           {""}, // 未選択
		"is_active":          {"1"},
		"gorilla.csrf.Token": {token},
	}
	pw := postFormWithCookies(app, "/devices", form, cookies)
	if pw.Code != http.StatusSeeOther || pw.Header().Get("Location") != "/devices/20" {
		t.Fatalf("未選択 POST /devices = %d Location=%q, want 303 /devices/20 (body=%s)", pw.Code, pw.Header().Get("Location"), pw.Body.String())
	}
}

// TestIntegration_編集フォームで保存済み地域がselected復元 は、保存済み地域を持つデバイスの
// 編集フォームで、当該地域の option が selected 状態 (認識名表示) で復元されることを検証する (R4.1)。
func TestIntegration_編集フォームで保存済み地域がselected復元(t *testing.T) {
	devices := map[int64]repository.Device{
		1: {ID: 1, UserID: 1, Name: "ハウスA温湿度計", MacAddress: "AA:BB:CC:DD:EE:01", IsActive: true, Locality: locPtr("佐敷町")},
	}
	app := deviceApp(devices, repository.Device{}, repository.Device{})
	cookies := loginCookies(t, app)

	ew := getWithCookies(app, "/devices/1/edit", cookies)
	if ew.Code != http.StatusOK {
		t.Fatalf("GET /devices/1/edit = %d, want 200 (body=%s)", ew.Code, ew.Body.String())
	}
	body := ew.Body.String()
	// 保存済み地域 (佐敷町) の option が selected で復元され、認識名で表示される。
	if !strings.Contains(body, `<option value="佐敷町" selected>佐敷（南城市）</option>`) {
		t.Errorf("編集フォームで保存済み地域が selected 復元されていない:\n%s", body)
	}
}
