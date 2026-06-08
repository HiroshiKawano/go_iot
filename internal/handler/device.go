// device.go はデバイス登録・編集の Web UI ハンドラ (4 ルートの HTTP 境界) を担う。
// 認可は internal/authz (所有者認可) へ、検証・正規化・型変換は device_form.go へ委譲し、
// ここではリクエスト解釈・sentinel error → HTTP ステータス写像・templ 描画に集中する。
// 永続化は repository.Querier を満たす最小 interface DeviceRepo 経由で受ける (service 層なし)。
package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/authz"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/view"
	"github.com/HiroshiKawano/go_iot/internal/view/component"
	"github.com/HiroshiKawano/go_iot/internal/view/layout"
	"github.com/HiroshiKawano/go_iot/internal/view/page"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
)

const (
	deviceCreateTitle = "デバイス登録 - 農業IoTシステム"
	deviceEditTitle   = "デバイス編集 - 農業IoTシステム"
	// MAC の handler 内検証メッセージ (binding では表現できない形式/一意のため)。
	macFormatMessage    = "MACアドレスは XX:XX:XX:XX:XX:XX 形式で入力してください"
	macDuplicateMessage = "このMACアドレスは既に登録されています"
)

// DeviceRepo はデバイス登録・編集・詳細・削除ハンドラが必要とする最小の DB ポート。
// repository.Querier が満たす (main.go は repository.New(pool) をそのまま渡すため無改修)。
// GetDevice を含むため authz.RequireDeviceOwner の DeviceGetter も満たす。
// 詳細画面 (device_show.go) 向けに最新10件取得・24h生データ・日次集計・論理削除を宣言追加する
// (クエリは sqlc 生成済み。本 interface はその consumer 最小 interface への宣言追加のみ)。
type DeviceRepo interface {
	GetUser(ctx context.Context, id int64) (repository.User, error)
	GetDevice(ctx context.Context, id int64) (repository.Device, error)
	GetDeviceByMacAddress(ctx context.Context, macAddress string) (repository.Device, error)
	CreateDevice(ctx context.Context, arg repository.CreateDeviceParams) (repository.Device, error)
	UpdateDevice(ctx context.Context, arg repository.UpdateDeviceParams) (repository.Device, error)
	// --- デバイス詳細画面 (device-detail) で追加 ---
	ListLatestSensorReadings(ctx context.Context, deviceID int64) ([]repository.SensorReading, error)
	ListRecentSensorReadings(ctx context.Context, arg repository.ListRecentSensorReadingsParams) ([]repository.SensorReading, error)
	ListDailySensorAggregates(ctx context.Context, arg repository.ListDailySensorAggregatesParams) ([]repository.ListDailySensorAggregatesRow, error)
	SoftDeleteDevice(ctx context.Context, id int64) error
}

// DeviceHandler はデバイス登録・編集の 4 ルートを提供する。
type DeviceHandler struct {
	Repo DeviceRepo
}

// ShowCreateForm は空のデバイス登録フォームを表示する (GET /devices/create・RequireAuth 前提)。
// ステータス初期は稼働中 ("1")、送信先 /devices (POST)、キャンセルは /dashboard。
func (h *DeviceHandler) ShowCreateForm(c *gin.Context) {
	user, err := h.Repo.GetUser(c.Request.Context(), auth.UserID(c))
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	form := deviceForm{IsActive: "1"} // 初期選択=稼働中
	v := buildCreateView(csrf.Token(c.Request), user.Name, form, map[string]string{})
	renderPage(c, http.StatusOK, page.DeviceCreatePage(v))
}

// Create はデバイス登録を実行する (POST /devices・RequireAuth 前提)。
// bind → 項目検証 → MAC 正規化・形式 → MAC 一意 (削除以外の全デバイス) → 作成 → 303。
// 各検証失敗はリダイレクトせず 200 で入力値復元付き再描画、DB 想定外エラーは 500。
// 所有者はフォーム入力ではなく session 由来の uid から決定する (R7.3)。
func (h *DeviceHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)

	var form deviceForm
	if err := c.ShouldBind(&form); err != nil {
		h.reRenderCreate(c, form, toDeviceFieldErrors(err))
		return
	}

	mac := normalizeMac(form.MacAddress)
	if !isValidMacFormat(mac) {
		h.reRenderCreate(c, form, map[string]string{"mac_address": macFormatMessage})
		return
	}

	if _, err := h.Repo.GetDeviceByMacAddress(ctx, mac); err == nil {
		h.reRenderCreate(c, form, map[string]string{"mac_address": macDuplicateMessage})
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		renderError(c, http.StatusInternalServerError)
		return
	}

	device, err := h.Repo.CreateDevice(ctx, repository.CreateDeviceParams{
		UserID:     uid,
		Name:       form.Name,
		MacAddress: mac,
		Location:   locationPtr(form.Location),
		IsActive:   parseIsActive(form.IsActive),
	})
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusSeeOther, "/devices/"+strconv.FormatInt(device.ID, 10))
}

// ShowEditForm は本人所有デバイスの編集フォームを既存値入りで表示する
// (GET /devices/:device/edit・RequireAuth 前提)。
// 非数値 ID・不在/論理削除は 404、他ユーザー所有は 403。送信先は PUT (hidden _method)。
func (h *DeviceHandler) ShowEditForm(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)

	id, err := strconv.ParseInt(c.Param("device"), 10, 64)
	if err != nil {
		renderError(c, http.StatusNotFound)
		return
	}

	device, err := authz.RequireDeviceOwner(ctx, h.Repo, id, uid)
	if err != nil {
		renderDeviceOwnerError(c, err)
		return
	}

	user, err := h.Repo.GetUser(ctx, uid)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	form := deviceForm{
		Name:       device.Name,
		MacAddress: device.MacAddress,
		Location:   deviceLocation(device),
		IsActive:   radioFromIsActive(device.IsActive),
	}
	v := buildEditView(csrf.Token(c.Request), user.Name, id, device.Name, form, map[string]string{})
	renderPage(c, http.StatusOK, page.DeviceEditPage(v))
}

// Update はデバイス更新を実行する (PUT /devices/:device・RequireAuth + MethodOverride 前提)。
// 認可 → bind → 項目検証 → MAC 正規化・形式 → MAC 一意 (自己除外・自身の現在値は許可) → 更新 → 303。
// 不在/論理削除は 404、非所有は 403、各検証失敗は 200 再描画、DB 想定外エラーは 500。
func (h *DeviceHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)

	id, err := strconv.ParseInt(c.Param("device"), 10, 64)
	if err != nil {
		renderError(c, http.StatusNotFound)
		return
	}

	device, err := authz.RequireDeviceOwner(ctx, h.Repo, id, uid)
	if err != nil {
		renderDeviceOwnerError(c, err)
		return
	}

	var form deviceForm
	if err := c.ShouldBind(&form); err != nil {
		h.reRenderEdit(c, id, device.Name, form, toDeviceFieldErrors(err))
		return
	}

	mac := normalizeMac(form.MacAddress)
	if !isValidMacFormat(mac) {
		h.reRenderEdit(c, id, device.Name, form, map[string]string{"mac_address": macFormatMessage})
		return
	}

	// 一意検査は自分自身を除外 (existing.ID != id が重複)。自身の現在値は許可 (R6.6)。
	if existing, err := h.Repo.GetDeviceByMacAddress(ctx, mac); err == nil {
		if existing.ID != id {
			h.reRenderEdit(c, id, device.Name, form, map[string]string{"mac_address": macDuplicateMessage})
			return
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		renderError(c, http.StatusInternalServerError)
		return
	}

	updated, err := h.Repo.UpdateDevice(ctx, repository.UpdateDeviceParams{
		ID:         id,
		Name:       form.Name,
		MacAddress: mac,
		Location:   locationPtr(form.Location),
		IsActive:   parseIsActive(form.IsActive),
	})
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusSeeOther, "/devices/"+strconv.FormatInt(updated.ID, 10))
}

// reRenderCreate は登録フォームを 200 で再描画する (入力値復元 + 項目別エラー)。
// レイアウトのユーザー名取得に失敗したら 500。
func (h *DeviceHandler) reRenderCreate(c *gin.Context, form deviceForm, errs map[string]string) {
	user, err := h.Repo.GetUser(c.Request.Context(), auth.UserID(c))
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	v := buildCreateView(csrf.Token(c.Request), user.Name, form, errs)
	renderPage(c, http.StatusOK, page.DeviceCreatePage(v))
}

// reRenderEdit は編集フォームを 200 で再描画する (入力値復元 + 項目別エラー)。
// deviceName は見出し用の編集対象現在名。ユーザー名取得失敗は 500。
func (h *DeviceHandler) reRenderEdit(c *gin.Context, id int64, deviceName string, form deviceForm, errs map[string]string) {
	user, err := h.Repo.GetUser(c.Request.Context(), auth.UserID(c))
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	v := buildEditView(csrf.Token(c.Request), user.Name, id, deviceName, form, errs)
	renderPage(c, http.StatusOK, page.DeviceEditPage(v))
}

// buildCreateView は登録ページの View を組み立てる (初期表示・再描画 共通)。
func buildCreateView(token, userName string, form deviceForm, errs map[string]string) page.DeviceFormView {
	return page.DeviceFormView{
		Layout: layout.AppLayoutData{
			Title:     deviceCreateTitle,
			UserName:  userName,
			CSRFToken: token,
			CSSURL:    view.CSSURL(),
		},
		Form: component.DeviceFormView{
			CSRFToken:  token,
			Action:     "/devices",
			IsEdit:     false,
			CancelURL:  "/dashboard",
			Name:       form.Name,
			MacAddress: form.MacAddress,
			Location:   form.Location,
			IsActive:   form.IsActive,
			Errors:     errs,
		},
	}
}

// buildEditView は編集ページの View を組み立てる (初期表示・再描画 共通)。
// deviceName は見出し「デバイス編集: {deviceName}」用 (編集対象の現在名)。
func buildEditView(token, userName string, id int64, deviceName string, form deviceForm, errs map[string]string) page.DeviceFormView {
	path := "/devices/" + strconv.FormatInt(id, 10)
	return page.DeviceFormView{
		Layout: layout.AppLayoutData{
			Title:     deviceEditTitle,
			UserName:  userName,
			CSRFToken: token,
			CSSURL:    view.CSSURL(),
		},
		DeviceName: deviceName,
		Form: component.DeviceFormView{
			CSRFToken:  token,
			Action:     path,
			IsEdit:     true,
			CancelURL:  path,
			Name:       form.Name,
			MacAddress: form.MacAddress,
			Location:   form.Location,
			IsActive:   form.IsActive,
			Errors:     errs,
		},
	}
}

// renderDeviceOwnerError は authz.RequireDeviceOwner の sentinel error を HTTP ステータスへ写す。
// 不在/論理削除→404、非所有→403、未認証 (前置 RequireAuth で通常発生しない) と想定外→500。
func renderDeviceOwnerError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		renderError(c, http.StatusNotFound)
	case errors.Is(err, authz.ErrNotOwner):
		renderError(c, http.StatusForbidden)
	default:
		renderError(c, http.StatusInternalServerError)
	}
}

// radioFromIsActive は稼働状態 (bool) を radio 値 ("1"/"0") へ変換する (parseIsActive の逆)。
// 編集フォームでの選択状態復元に使う (稼働中=true→"1" / 停止中=false→"0")。
func radioFromIsActive(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
