package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
)

// device_planting_date_test.go は定植/播種日フォーム項目のバインド・検証・保存・復元を固定する (タスク 6.4)。
// Querier 手書きモック (fakeDeviceRepo) で DB 非依存・httptest+gin・form-urlencoded 送信。

// 正常な定植日が pgtype.Date(Valid) として保存される。
func TestCreate_定植日を保存する(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.createResult = repository.Device{ID: 9, UserID: 7}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("planting_date", "2026-04-01")
	w := formRequest(r, http.MethodPost, "/devices", vals)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303 (body=%s)", w.Code, w.Body.String())
	}
	if !repo.createCalled {
		t.Fatal("CreateDevice が呼ばれていない")
	}
	pd := repo.lastCreate.PlantingDate
	if !pd.Valid {
		t.Fatalf("PlantingDate.Valid=false, want true（保存されていない）")
	}
	if got := pd.Time.Format("2006-01-02"); got != "2026-04-01" {
		t.Errorf("PlantingDate=%q, want 2026-04-01", got)
	}
}

// 空の定植日は NULL（Valid=false）で保存される。
func TestCreate_定植日空はNULL保存(t *testing.T) {
	repo := deviceOwnerRepo()
	repo.createResult = repository.Device{ID: 9, UserID: 7}
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals() // planting_date を含めない
	w := formRequest(r, http.MethodPost, "/devices", vals)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303 (body=%s)", w.Code, w.Body.String())
	}
	if repo.lastCreate.PlantingDate.Valid {
		t.Errorf("PlantingDate.Valid=true, want false（空は NULL）")
	}
}

// 未来日は field error「定植日は未来日にできません」で同フォーム再描画（保存しない・入力値保持）。
func TestCreate_定植日未来日はフィールドエラーで再描画(t *testing.T) {
	repo := deviceOwnerRepo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("planting_date", "2099-01-01") // 未来日
	w := formRequest(r, http.MethodPost, "/devices", vals)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200（同フォーム再描画）", w.Code)
	}
	if repo.createCalled {
		t.Error("未来日なのに CreateDevice が呼ばれている（保存してしまっている）")
	}
	body := w.Body.String()
	if !strings.Contains(body, "定植日は未来日にできません") {
		t.Errorf("未来日の field error が描画されていない:\n%s", body)
	}
	// 入力値（未来日）が value で保持される。
	if !strings.Contains(body, `value="2099-01-01"`) {
		t.Errorf("入力値が再描画で保持されていない")
	}
}

// 不正形式は field error で同フォーム再描画。
func TestCreate_定植日不正形式はフィールドエラーで再描画(t *testing.T) {
	repo := deviceOwnerRepo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("planting_date", "2026/04/01") // 不正形式（スラッシュ）
	w := formRequest(r, http.MethodPost, "/devices", vals)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200（同フォーム再描画）", w.Code)
	}
	if repo.createCalled {
		t.Error("不正形式なのに CreateDevice が呼ばれている")
	}
	if !strings.Contains(w.Body.String(), "error-message") {
		t.Error("不正形式の field error が描画されていない")
	}
}

// 編集フォームは保存済み定植日を value でプリセット復元する。
func TestShowEditForm_定植日をプリセット復元(t *testing.T) {
	repo := ownedDevice1Repo()
	d := repo.devices[1]
	d.PlantingDate = pgtype.Date{Time: dateOnlyUTC(2026, 4, 1), Valid: true}
	repo.devices[1] = d
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	w := getPath(r, "/devices/1/edit")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `name="planting_date"`) {
		t.Error("定植日 input が編集フォームに無い")
	}
	if !strings.Contains(body, `value="2026-04-01"`) {
		t.Errorf("保存済み定植日が value でプリセットされていない:\n%s", body)
	}
}

// 更新でも定植日が保存される。
func TestUpdate_定植日を保存する(t *testing.T) {
	repo := ownedDevice1Repo()
	r := newDeviceRouterWithUser(&DeviceHandler{Repo: repo}, 7)

	vals := validDeviceVals()
	vals.Set("mac_address", "aa:bb:cc:dd:ee:99")
	vals.Set("planting_date", "2026-03-15")
	w := formRequest(r, http.MethodPut, "/devices/1", vals)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303 (body=%s)", w.Code, w.Body.String())
	}
	pd := repo.lastUpdate.PlantingDate
	if !pd.Valid || pd.Time.Format("2006-01-02") != "2026-03-15" {
		t.Errorf("更新の PlantingDate=%+v, want 2026-03-15", pd)
	}
}
