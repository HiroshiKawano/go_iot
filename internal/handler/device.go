// device.go はデバイス登録・編集の Web UI ハンドラ (4 ルートの HTTP 境界) を担う。
// 認可は internal/authz (所有者認可) へ、検証・正規化・型変換は device_form.go へ委譲し、
// ここではリクエスト解釈・sentinel error → HTTP ステータス写像・templ 描画に集中する。
// 永続化は repository.Querier を満たす最小 interface DeviceRepo 経由で受ける (service 層なし)。
package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/authz"
	"github.com/HiroshiKawano/go_iot/internal/domain"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/view"
	"github.com/HiroshiKawano/go_iot/internal/view/component"
	"github.com/HiroshiKawano/go_iot/internal/view/layout"
	"github.com/HiroshiKawano/go_iot/internal/view/page"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
	"github.com/jackc/pgx/v5"
)

const (
	deviceCreateTitle = "デバイス登録 - 農業IoTシステム"
	deviceEditTitle   = "デバイス編集 - 農業IoTシステム"
	// MAC の handler 内検証メッセージ (binding では表現できない形式/一意のため)。
	macFormatMessage    = "MACアドレスは XX:XX:XX:XX:XX:XX 形式で入力してください"
	macDuplicateMessage = "このMACアドレスは既に登録されています"
	// 地域 select の手続き検証メッセージ (選択肢に無い値が送られた場合)。
	localityInvalidMessage = "選択した地域が不正です"
	// 作物 select の手続き検証メッセージ (9作物に無い値が送られた場合)。
	cropInvalidMessage = "選択した作物が不正です"
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
	// JST 暦日バケットの日次集約 (heat-stress-thi で追加・熱帯夜 calendar/夜温/ΔT/年間日数トレンド用)。
	// 既存 ListDailySensorAggregates(UTC バケット) とは別物 (3d/7d/30d グラフ用は無改変で温存)。
	ListDailySensorAggregatesJST(ctx context.Context, arg repository.ListDailySensorAggregatesJSTParams) ([]repository.ListDailySensorAggregatesJSTRow, error)
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
	bindErr := c.ShouldBind(&form)

	// 検証は early-return せず errs へ累積し、全項目評価後に再描画する (R5.2 同時表示)。
	errs := map[string]string{}
	if bindErr != nil {
		errs = toDeviceFieldErrors(bindErr)
	}

	mac := normalizeMac(form.MacAddress)
	if dbErr := h.checkMacForCreate(ctx, mac, form.MacAddress, errs); dbErr {
		renderError(c, http.StatusInternalServerError)
		return
	}
	if !validLocalityInput(form.Locality) {
		errs["locality"] = localityInvalidMessage
	}
	if !validCropInput(form.Crop) {
		errs["crop"] = cropInvalidMessage
	}
	// 定植日: 空可・形式/未来日を procedural に検証 (locality/crop と同型・空→NULL)。
	plantingDate, pdErr := parsePlantingDate(form.PlantingDate, time.Now())
	if pdErr != "" {
		errs["planting_date"] = pdErr
	}

	if len(errs) > 0 {
		h.reRenderCreate(c, form, errs)
		return
	}

	device, err := h.Repo.CreateDevice(ctx, repository.CreateDeviceParams{
		UserID:       uid,
		Name:         form.Name,
		MacAddress:   mac,
		Location:     nil, // 新規デバイスは旧自由入力 location を持たない (所在地は locality)
		Locality:     nullableStr(form.Locality),
		Crop:         nullableStr(form.Crop),
		PlantingDate: plantingDate,
		IsActive:     parseIsActive(form.IsActive),
	})
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusSeeOther, "/devices/"+strconv.FormatInt(device.ID, 10))
}

// checkMacForCreate は MAC の形式・一意 (削除以外の全デバイス) を検査し、不備を errs に積む。
// DB 想定外エラー (ErrNoRows 以外) のときだけ true を返し、呼び出し側で 500 にする。
// MAC 必須は binding で担保済みのため、未入力時は形式・一意検査をスキップする。
func (h *DeviceHandler) checkMacForCreate(ctx context.Context, mac, raw string, errs map[string]string) (dbError bool) {
	if raw == "" {
		return false
	}
	if !isValidMacFormat(mac) {
		errs["mac_address"] = macFormatMessage
		return false
	}
	if _, err := h.Repo.GetDeviceByMacAddress(ctx, mac); err == nil {
		errs["mac_address"] = macDuplicateMessage
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	return false
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
		Name:         device.Name,
		MacAddress:   device.MacAddress,
		Locality:     deviceLocalityValue(device),
		Crop:         deviceCropValue(device),
		PlantingDate: devicePlantingDateValue(device),
		IsActive:     radioFromIsActive(device.IsActive),
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
	bindErr := c.ShouldBind(&form)

	// 検証は early-return せず errs へ累積し、全項目評価後に再描画する (R5.2 同時表示)。
	errs := map[string]string{}
	if bindErr != nil {
		errs = toDeviceFieldErrors(bindErr)
	}

	mac := normalizeMac(form.MacAddress)
	if dbErr := h.checkMacForUpdate(ctx, id, mac, form.MacAddress, errs); dbErr {
		renderError(c, http.StatusInternalServerError)
		return
	}
	if !validLocalityInput(form.Locality) {
		errs["locality"] = localityInvalidMessage
	}
	if !validCropInput(form.Crop) {
		errs["crop"] = cropInvalidMessage
	}
	// 定植日: 空可・形式/未来日を procedural に検証 (locality/crop と同型・空→NULL)。
	plantingDate, pdErr := parsePlantingDate(form.PlantingDate, time.Now())
	if pdErr != "" {
		errs["planting_date"] = pdErr
	}

	if len(errs) > 0 {
		h.reRenderEdit(c, id, device.Name, form, errs)
		return
	}

	updated, err := h.Repo.UpdateDevice(ctx, repository.UpdateDeviceParams{
		ID:           id,
		Name:         form.Name,
		MacAddress:   mac,
		Location:     device.Location, // 旧自由入力 location は編集対象外。既存値を保全 (非破壊)
		Locality:     nullableStr(form.Locality),
		Crop:         nullableStr(form.Crop),
		PlantingDate: plantingDate,
		IsActive:     parseIsActive(form.IsActive),
	})
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusSeeOther, "/devices/"+strconv.FormatInt(updated.ID, 10))
}

// checkMacForUpdate は MAC の形式・一意 (自己除外・自身の現在値は許可) を検査し、不備を errs に積む。
// DB 想定外エラーのときだけ true を返し、呼び出し側で 500 にする。
func (h *DeviceHandler) checkMacForUpdate(ctx context.Context, id int64, mac, raw string, errs map[string]string) (dbError bool) {
	if raw == "" {
		return false
	}
	if !isValidMacFormat(mac) {
		errs["mac_address"] = macFormatMessage
		return false
	}
	if existing, err := h.Repo.GetDeviceByMacAddress(ctx, mac); err == nil {
		if existing.ID != id {
			errs["mac_address"] = macDuplicateMessage
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	return false
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
			// 登録は対応メニュー項目を持たない画面=ゼロ値ナビ文脈 (active なし・文脈リンクなし・R1.3/2.6)。
			Nav: component.SidebarNav{},
		},
		Form: component.DeviceFormView{
			CSRFToken:    token,
			Action:       "/devices",
			IsEdit:       false,
			CancelURL:    "/dashboard",
			Name:         form.Name,
			MacAddress:   form.MacAddress,
			Locality:     form.Locality,
			Localities:   localityOptions(form.Locality),
			Crop:         form.Crop,
			Crops:        cropOptions(form.Crop),
			PlantingDate: form.PlantingDate,
			IsActive:     form.IsActive,
			Errors:       errs,
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
			// 編集は URL に device id (path 変数) を持つが、デバイス文脈には入れない
			// (確定済みのユーザー判断・要件 Out of scope)。DeviceID を設定せずゼロ値とし、
			// 文脈リンク・active を出さない (R1.3 boundary/2.6)。
			Nav: component.SidebarNav{},
		},
		DeviceName: deviceName,
		Form: component.DeviceFormView{
			CSRFToken:    token,
			Action:       path,
			IsEdit:       true,
			CancelURL:    path,
			Name:         form.Name,
			MacAddress:   form.MacAddress,
			Locality:     form.Locality,
			Localities:   localityOptions(form.Locality),
			Crop:         form.Crop,
			Crops:        cropOptions(form.Crop),
			PlantingDate: form.PlantingDate,
			IsActive:     form.IsActive,
			Errors:       errs,
		},
	}
}

// renderDeviceOwnerError は authz.RequireDeviceOwner の sentinel error を HTTP ステータスへ写す。
// 不在/論理削除→404、非所有→403、未認証 (前置 RequireAuth で通常発生しない) と想定外→500。
func renderDeviceOwnerError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
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

// validLocalityInput は地域 select の送信値が許容されるかを判定する。
// 未選択 (空) は任意項目ゆえ許可。非空は domain.Locality の定義値 (53) のみ許可する。
func validLocalityInput(s string) bool {
	return s == "" || domain.Locality(s).Valid()
}

// deviceLocalityValue はデバイスの地域キー (*string) を復元用の文字列へ変換する (未設定は "")。
func deviceLocalityValue(d repository.Device) string {
	if d.Locality != nil {
		return *d.Locality
	}
	return ""
}

// localityOptions は地域 select の選択肢 (沖縄53地域) を組み立てる。
// Label は認識名 (合併=「旧町村（現市町村）」/未合併=市町村名)、Selected は現在値との一致。
// view が domain を直接 range せず handler が SelectOption を組むことで選択値復元を一貫させる。
func localityOptions(selected string) []component.SelectOption {
	all := domain.AllLocalities()
	opts := make([]component.SelectOption, 0, len(all))
	for _, l := range all {
		opts = append(opts, component.SelectOption{
			Value:    string(l),
			Label:    l.Label(),
			Selected: string(l) == selected,
		})
	}
	return opts
}

// validCropInput は作物 select の送信値が許容されるかを判定する (locality 写経)。
// 未選択 (空) は任意項目ゆえ許可。非空は domain.Crop の定義値 (9作物) のみ許可する。
func validCropInput(s string) bool {
	return s == "" || domain.Crop(s).Valid()
}

// deviceCropValue はデバイスの作物キー (*string) を復元用の文字列へ変換する (未設定は "")。
func deviceCropValue(d repository.Device) string {
	if d.Crop != nil {
		return *d.Crop
	}
	return ""
}

// cropOptions は作物 select の選択肢 (9作物) を組み立てる (locality 写経)。
// Label は日本語作物名、Selected は現在値との一致。空 option「選択しない（既定しきい値）」は templ 側で先頭付与。
// GDD 具体モデルを持つ作物 (米・ゴーヤ・インゲン・ウリ・いも・葉野菜) には「(GDD対応)」接尾辞を付す (表示専用・値は不変)。
// ※ 農場運営者が選択時点で GDD 予測の可否を判別でき、かつ定植日フィールドの出し分け判定にも使われる
//    (component.GDDCropLabelSuffix が単一の真実源)。情報パネル/VPD/CSV の作物表示は素の Crop.Label()。
func cropOptions(selected string) []component.SelectOption {
	all := domain.AllCrops()
	opts := make([]component.SelectOption, 0, len(all))
	for _, c := range all {
		label := c.Label()
		if c.HasGDDModel() {
			label += component.GDDCropLabelSuffix
		}
		opts = append(opts, component.SelectOption{
			Value:    string(c),
			Label:    label,
			Selected: string(c) == selected,
		})
	}
	return opts
}
