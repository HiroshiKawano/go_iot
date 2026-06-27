package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// device_crop_integration_test.go は栽培作物フォームの全体フロー (描画・選択値復元・検証・保存・CSRF 往復) を
// 実 gorilla/csrf + scs session を通した合成ハンドラで固定する (タスク 6.2)。
//
// 注: device フォームは非 HTMX のフルページ送信のため、検証エラー時は 200 でフォームを再描画する
// (所在地 locality と同経路。design「既存 locality と同経路」)。422 は別物の alert_rule の HTMX Section
// 再描画の規約であり device フォームには適用しない。

// cropEditApp は編集対象 device1 に crop=goya を持たせた合成ハンドラを返す (選択値復元の検証用)。
func cropEditApp() http.Handler {
	devices := map[int64]repository.Device{
		1: {ID: 1, UserID: 1, Name: "ハウスA温湿度計", MacAddress: "AA:BB:CC:DD:EE:01", IsActive: true, Crop: locPtr("goya")},
	}
	return deviceApp(devices, repository.Device{ID: 1, UserID: 1}, repository.Device{ID: 1, UserID: 1})
}

// --- 6.2 登録フォームに作物 select (空 option + 9選択肢) が描画される ---

func TestIntegration_作物登録フォームに空optionと9選択肢(t *testing.T) {
	app := deviceApp(nil, repository.Device{}, repository.Device{})
	cookies := loginCookies(t, app)

	cw := getWithCookies(app, "/devices/create", cookies)
	if cw.Code != http.StatusOK {
		t.Fatalf("GET /devices/create = %d, want 200", cw.Code)
	}
	body := cw.Body.String()
	for _, want := range []string{
		`<select name="crop" class="js-tom-select">`,        // 所在地と同型の検索可能 select
		`<option value="">選択しない（既定しきい値）</option>`,        // 空 option (未選択=既定帯)
		`<option value="goya">ゴーヤ</option>`,                // 9作物
		`<option value="sugarcane">サトウキビ</option>`,
		`<option value="leafy_vegetable">葉野菜</option>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("登録フォームに %q が含まれていない", want)
		}
	}
	// 作物 select 区間に option ちょうど 10 個 (空 + 9作物)。
	cropSel := body
	if i := strings.Index(cropSel, `name="crop"`); i >= 0 {
		cropSel = cropSel[i:]
		if j := strings.Index(cropSel, "</select>"); j >= 0 {
			cropSel = cropSel[:j]
		}
	}
	if got := strings.Count(cropSel, "<option"); got != 10 {
		t.Errorf("crop の option 数 = %d, want 10 (空 + 9作物)", got)
	}
}

// --- 6.2 編集フォームで保存済み作物が選択復元される ---

func TestIntegration_作物編集で選択値復元(t *testing.T) {
	app := cropEditApp()
	cookies := loginCookies(t, app)

	ew := getWithCookies(app, "/devices/1/edit", cookies)
	if ew.Code != http.StatusOK {
		t.Fatalf("GET /devices/1/edit = %d, want 200 (body=%s)", ew.Code, ew.Body.String())
	}
	if !strings.Contains(ew.Body.String(), `<option value="goya" selected>ゴーヤ</option>`) {
		t.Errorf("編集フォームで作物 goya が選択復元されていない:\n%s", ew.Body.String())
	}
}

// --- 6.2 不正作物は再描画しフィールドエラー (CSRF 往復) ---

func TestIntegration_不正作物は再描画しフィールドエラー(t *testing.T) {
	app := deviceApp(nil, repository.Device{ID: 30, UserID: 1}, repository.Device{})
	cookies := loginCookies(t, app)

	cw := getWithCookies(app, "/devices/create", cookies)
	token := extractCSRFToken(cw.Body.String())
	cookies = mergeCookies(cookies, cw.Result().Cookies())

	form := url.Values{
		"name":               {"温室センサー"},
		"mac_address":        {"AA:BB:CC:DD:EE:30"},
		"locality":           {""},
		"crop":               {"tomato"}, // 9作物に無い値
		"is_active":          {"1"},
		"gorilla.csrf.Token": {token},
	}
	pw := postFormWithCookies(app, "/devices", form, cookies)
	// 非 HTMX フルページ再描画ゆえ 200 (locality と同経路)。
	if pw.Code != http.StatusOK {
		t.Fatalf("不正作物 POST = %d, want 200 (再描画・body=%s)", pw.Code, pw.Body.String())
	}
	if !strings.Contains(pw.Body.String(), "選択した作物が不正です") {
		t.Error("作物不正のフィールドエラーが表示されていない")
	}
}

// --- 6.2 他項目エラー時も選択作物を復元する (R3.3) ---

func TestIntegration_他項目エラー時に作物選択を復元(t *testing.T) {
	app := deviceApp(nil, repository.Device{ID: 31, UserID: 1}, repository.Device{})
	cookies := loginCookies(t, app)

	cw := getWithCookies(app, "/devices/create", cookies)
	token := extractCSRFToken(cw.Body.String())
	cookies = mergeCookies(cookies, cw.Result().Cookies())

	form := url.Values{
		"name":               {""}, // デバイス名必須エラー
		"mac_address":        {"AA:BB:CC:DD:EE:31"},
		"locality":           {""},
		"crop":               {"goya"}, // 正常な作物 → 再描画で復元されるべき
		"is_active":          {"1"},
		"gorilla.csrf.Token": {token},
	}
	pw := postFormWithCookies(app, "/devices", form, cookies)
	if pw.Code != http.StatusOK {
		t.Fatalf("他項目エラー POST = %d, want 200 (再描画)", pw.Code)
	}
	body := pw.Body.String()
	if !strings.Contains(body, "デバイス名を入力してください") {
		t.Error("他項目 (name) のエラーが表示されていない")
	}
	if !strings.Contains(body, `<option value="goya" selected>ゴーヤ</option>`) {
		t.Error("再描画で選択作物 goya が復元されていない (R3.3)")
	}
}

// --- 6.2 正常作物の登録は 303 (CSRF 往復成功) ---

func TestIntegration_正常作物の登録は303_CSRF往復(t *testing.T) {
	app := deviceApp(nil, repository.Device{ID: 32, UserID: 1}, repository.Device{})
	cookies := loginCookies(t, app)

	cw := getWithCookies(app, "/devices/create", cookies)
	token := extractCSRFToken(cw.Body.String())
	cookies = mergeCookies(cookies, cw.Result().Cookies())

	form := url.Values{
		"name":               {"ゴーヤハウス"},
		"mac_address":        {"AA:BB:CC:DD:EE:32"},
		"locality":           {"那覇市"},
		"crop":               {"goya"},
		"is_active":          {"1"},
		"gorilla.csrf.Token": {token},
	}
	pw := postFormWithCookies(app, "/devices", form, cookies)
	if pw.Code != http.StatusSeeOther || pw.Header().Get("Location") != "/devices/32" {
		t.Fatalf("正常作物 POST = %d Location=%q, want 303 /devices/32 (body=%s)", pw.Code, pw.Header().Get("Location"), pw.Body.String())
	}
}

// --- 6.2 CSRF トークン無しの作物 POST は拒否される (419・BOLA 403 とは分離) ---

func TestIntegration_作物POSTはCSRFトークン無しで拒否(t *testing.T) {
	app := deviceApp(nil, repository.Device{ID: 33, UserID: 1}, repository.Device{})
	cookies := loginCookies(t, app)

	form := url.Values{
		"name":        {"ゴーヤハウス"},
		"mac_address": {"AA:BB:CC:DD:EE:33"},
		"crop":        {"goya"},
		"is_active":   {"1"},
		// gorilla.csrf.Token を付さない
	}
	pw := postFormWithCookies(app, "/devices", form, cookies)
	// 本プロジェクトの CSRF 失敗は 419 (所有権エラー BOLA 403 と区別。419 は標準 http 定数が無い)。
	const statusCSRFFailed = 419
	if pw.Code != statusCSRFFailed {
		t.Errorf("CSRF トークン無し POST = %d, want %d (CSRF 失敗)", pw.Code, statusCSRFFailed)
	}
}
