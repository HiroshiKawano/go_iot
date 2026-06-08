// alert_rule.go はアラートルール管理 (インライン CRUD) の Web UI ハンドラ
// (GET /alerts/rules ほか 6 ルートの HTTP 境界) を担う。認可は internal/authz
// (rule→device→owner の所有者認可) へ、検証・型変換は alert_rule_form.go へ委譲し、
// ここではリクエスト解釈・sentinel error → HTTP ステータス写像・部分/全体の templ 描画に集中する。
// 永続化は repository.Querier を満たす最小 interface AlertRuleRepo 経由で受ける (service 層なし)。
package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/authz"
	"github.com/HiroshiKawano/go_iot/internal/domain"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/view"
	"github.com/HiroshiKawano/go_iot/internal/view/component"
	"github.com/HiroshiKawano/go_iot/internal/view/layout"
	"github.com/HiroshiKawano/go_iot/internal/view/page"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
	"github.com/jackc/pgx/v5"
)

const alertRulesTitle = "アラートルール管理 - 農業IoTシステム"

// AlertRuleRepo は AlertRuleHandler が必要とする最小 DB ポート (DIP・consumer 最小 interface)。
// repository.Querier が満たす。GetAlertRule+GetDevice を含むため authz の
// AlertRuleDeviceGetter / DeviceGetter も満たす (所有者認可で流用)。
type AlertRuleRepo interface {
	GetUser(ctx context.Context, id int64) (repository.User, error)
	ListDevicesByUser(ctx context.Context, userID int64) ([]repository.Device, error)
	GetDevice(ctx context.Context, id int64) (repository.Device, error)
	GetAlertRule(ctx context.Context, id int64) (repository.AlertRule, error)
	ListAlertRulesByDevice(ctx context.Context, deviceID int64) ([]repository.AlertRule, error)
	CreateAlertRule(ctx context.Context, arg repository.CreateAlertRuleParams) (repository.AlertRule, error)
	UpdateAlertRule(ctx context.Context, arg repository.UpdateAlertRuleParams) (repository.AlertRule, error)
	ToggleAlertRule(ctx context.Context, id int64) (repository.AlertRule, error)
	SoftDeleteAlertRule(ctx context.Context, id int64) error
}

// AlertRuleHandler はアラートルール管理の 6 ルートを提供する。
type AlertRuleHandler struct {
	Repo AlertRuleRepo
}

// Index は初期表示・デバイス切替を担う (GET /alerts/rules・RequireAuth 前提)。
// device_id 指定時はその所有デバイス、省略時は所有デバイス先頭 (created_at DESC) を選択中とし、
// 当該デバイスのルール一覧を描画する。HX-Request はセクション部分、通常はフルページを 200 で返す。
// 所有デバイス 0 件は案内表示、不在=404、非所有=403、DB 想定外=500。
func (h *AlertRuleHandler) Index(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)

	devices, err := h.Repo.ListDevicesByUser(ctx, uid)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	// 所有デバイス 0 件: デバイス選択・セクションの代わりに案内文を表示する (常にフルページ)。
	if len(devices) == 0 {
		user, err := h.Repo.GetUser(ctx, uid)
		if err != nil {
			renderError(c, http.StatusInternalServerError)
			return
		}
		renderPage(c, http.StatusOK, page.AlertRules(page.AlertRulesPageView{
			Layout:    h.layoutData(c, user.Name),
			HasDevice: false,
		}))
		return
	}

	// 選択中デバイスを決定する (指定あり=所有検証 / 省略=先頭)。
	selected := devices[0]
	if q := c.Query("device_id"); q != "" {
		id, perr := strconv.ParseInt(q, 10, 64)
		if perr != nil {
			renderError(c, http.StatusNotFound) // 非数値 ID は不在扱い
			return
		}
		selected, err = authz.RequireDeviceOwner(ctx, h.Repo, id, uid)
		if err != nil {
			renderAlertRuleOwnerError(c, err)
			return
		}
	}

	// HX-Request はセクションのみ部分返却する (デバイス切替)。
	if c.GetHeader("HX-Request") != "" {
		h.renderSection(c, http.StatusOK, selected.ID)
		return
	}

	// 通常 GET はフルページ。
	rules, err := h.Repo.ListAlertRulesByDevice(ctx, selected.ID)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	user, err := h.Repo.GetUser(ctx, uid)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	renderPage(c, http.StatusOK, page.AlertRules(page.AlertRulesPageView{
		Layout:    h.layoutData(c, user.Name),
		Devices:   toDeviceOptions(devices, selected.ID),
		HasDevice: true,
		Section:   buildSectionView(selected.ID, emptyForm(selected.ID), rules),
	}))
}

// Add はルールを新規追加する (POST /alerts/rules・RequireAuth 前提)。
// 所有者検証を最優先 (BOLA。device_id はフォーム値だが所有者は uid で判定) し、検証通過後に
// 有効状態で作成する。成功時は更新後一覧+空フォームの Section を 200、バリデーションエラーは
// 入力値復元+項目別エラー付きフォーム+現一覧を 422 で返す。非所有=403、不在=404、内部失敗=500。
func (h *AlertRuleHandler) Add(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)

	var form alertRuleForm
	bindErr := c.ShouldBind(&form)

	device, err := authz.RequireDeviceOwner(ctx, h.Repo, form.DeviceID, uid)
	if err != nil {
		renderAlertRuleOwnerError(c, err)
		return
	}

	threshold, errs := validateAlertRuleForm(form, bindErr)
	if len(errs) > 0 {
		h.renderSectionWithForm(c, http.StatusUnprocessableEntity, device.ID, formViewFrom(form, device.ID, 0, errs))
		return
	}

	if _, err := h.Repo.CreateAlertRule(ctx, repository.CreateAlertRuleParams{
		DeviceID:  device.ID,
		Metric:    form.Metric,
		Operator:  form.Operator,
		Threshold: pgconv.Numeric2(threshold),
		IsEnabled: true, // 新規は有効状態で作成 (要件 3.1)
	}); err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	h.renderSection(c, http.StatusOK, device.ID)
}

// Edit は編集フォームを既存値入りで返す (GET /alerts/rules/:rule/edit・RequireAuth 前提)。
// ルール所有を検証し、送信先 PUT・ボタン「更新」の編集モードフォームを #alert-rule-form へ差し替える。
// 不在/論理削除=404、非所有=403。
func (h *AlertRuleHandler) Edit(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)

	ruleID, err := strconv.ParseInt(c.Param("rule"), 10, 64)
	if err != nil {
		renderError(c, http.StatusNotFound)
		return
	}

	rule, _, err := authz.RequireAlertRuleOwner(ctx, h.Repo, ruleID, uid)
	if err != nil {
		renderAlertRuleOwnerError(c, err)
		return
	}

	renderComponent(c, component.AlertRuleForm(component.AlertRuleFormView{
		DeviceID:      rule.DeviceID,
		EditingRuleID: rule.ID,
		Metric:        rule.Metric,
		Operator:      rule.Operator,
		Threshold:     fmt.Sprintf("%.2f", pgconv.NumericToFloat(rule.Threshold)),
	}))
}

// Update はルールを更新する (PUT /alerts/rules/:rule・RequireAuth + MethodOverride 前提)。
// ルール所有を検証 (現在ルール取得) し、検証通過後に metric/operator/threshold を更新する。
// is_enabled は現在値を保全する (有効状態は切替で別管理。意図せぬ反転を防ぐ)。
// 成功時は更新後一覧+空フォームの Section を 200、バリデーションエラーは編集フォーム保持で 422。
// 不在=404、非所有=403、内部失敗=500。
func (h *AlertRuleHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)

	ruleID, err := strconv.ParseInt(c.Param("rule"), 10, 64)
	if err != nil {
		renderError(c, http.StatusNotFound)
		return
	}

	rule, device, err := authz.RequireAlertRuleOwner(ctx, h.Repo, ruleID, uid)
	if err != nil {
		renderAlertRuleOwnerError(c, err)
		return
	}

	var form alertRuleForm
	bindErr := c.ShouldBind(&form)
	threshold, errs := validateAlertRuleForm(form, bindErr)
	if len(errs) > 0 {
		// 編集モード (EditingRuleID=rule.ID) を保持して 422。device は rule 所属を正とする。
		h.renderSectionWithForm(c, http.StatusUnprocessableEntity, device.ID, formViewFrom(form, device.ID, rule.ID, errs))
		return
	}

	if _, err := h.Repo.UpdateAlertRule(ctx, repository.UpdateAlertRuleParams{
		ID:        rule.ID,
		Metric:    form.Metric,
		Operator:  form.Operator,
		Threshold: pgconv.Numeric2(threshold),
		IsEnabled: rule.IsEnabled, // 現在値を保全 (要件 5.1)
	}); err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	h.renderSection(c, http.StatusOK, device.ID)
}

// Toggle は有効/無効を反転する (PATCH /alerts/rules/:rule/toggle・RequireAuth 前提)。
// ルール所有を検証し、サーバ側で状態を反転して当該行のみを反転後状態で #alert-rule-row-{id} へ差し替える。
// 不在=404、非所有=403、内部失敗=500。
func (h *AlertRuleHandler) Toggle(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)

	ruleID, err := strconv.ParseInt(c.Param("rule"), 10, 64)
	if err != nil {
		renderError(c, http.StatusNotFound)
		return
	}

	if _, _, err := authz.RequireAlertRuleOwner(ctx, h.Repo, ruleID, uid); err != nil {
		renderAlertRuleOwnerError(c, err)
		return
	}

	toggled, err := h.Repo.ToggleAlertRule(ctx, ruleID)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	renderComponent(c, component.AlertRuleRow(toRowView(toggled)))
}

// Delete はルールを論理削除する (DELETE /alerts/rules/:rule・RequireAuth 前提)。
// ルール所有を検証し、論理削除後に残りの一覧を #alert-rule-list へ差し替える。
// 不在=404、非所有=403、内部失敗=500。
func (h *AlertRuleHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)

	ruleID, err := strconv.ParseInt(c.Param("rule"), 10, 64)
	if err != nil {
		renderError(c, http.StatusNotFound)
		return
	}

	rule, _, err := authz.RequireAlertRuleOwner(ctx, h.Repo, ruleID, uid)
	if err != nil {
		renderAlertRuleOwnerError(c, err)
		return
	}

	if err := h.Repo.SoftDeleteAlertRule(ctx, ruleID); err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	rules, err := h.Repo.ListAlertRulesByDevice(ctx, rule.DeviceID)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	renderComponent(c, component.AlertRuleList(toRowViews(rules)))
}

// renderSection は当該デバイスの最新一覧と空の追加フォームを Section として描画する (成功時)。
func (h *AlertRuleHandler) renderSection(c *gin.Context, status int, deviceID int64) {
	rules, err := h.Repo.ListAlertRulesByDevice(c.Request.Context(), deviceID)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	renderPage(c, status, component.AlertRuleSection(buildSectionView(deviceID, emptyForm(deviceID), rules)))
}

// renderSectionWithForm はエラー付きフォーム+現一覧を Section として描画する (422 再描画)。
func (h *AlertRuleHandler) renderSectionWithForm(c *gin.Context, status int, deviceID int64, form component.AlertRuleFormView) {
	rules, err := h.Repo.ListAlertRulesByDevice(c.Request.Context(), deviceID)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	renderPage(c, status, component.AlertRuleSection(buildSectionView(deviceID, form, rules)))
}

// layoutData は App レイアウトの共通データを組み立てる。
func (h *AlertRuleHandler) layoutData(c *gin.Context, userName string) layout.AppLayoutData {
	return layout.AppLayoutData{
		Title:     alertRulesTitle,
		UserName:  userName,
		CSRFToken: csrf.Token(c.Request),
		CSSURL:    view.CSSURL(),
	}
}

// emptyForm は対象デバイスの空の追加フォーム ViewModel を返す (追加モード)。
func emptyForm(deviceID int64) component.AlertRuleFormView {
	return component.AlertRuleFormView{DeviceID: deviceID}
}

// formViewFrom は入力値・編集モード・項目別エラーを保持したフォーム ViewModel を返す (422 再描画用)。
func formViewFrom(form alertRuleForm, deviceID, editingRuleID int64, errs map[string]string) component.AlertRuleFormView {
	return component.AlertRuleFormView{
		DeviceID:      deviceID,
		EditingRuleID: editingRuleID,
		Metric:        form.Metric,
		Operator:      form.Operator,
		Threshold:     form.Threshold,
		Errors:        errs,
	}
}

// buildSectionView はフォームとルール一覧を束ねた Section ViewModel を組み立てる。
func buildSectionView(deviceID int64, form component.AlertRuleFormView, rules []repository.AlertRule) component.AlertRuleSectionView {
	return component.AlertRuleSectionView{
		DeviceID: deviceID,
		Form:     form,
		Rules:    toRowViews(rules),
	}
}

// toRowViews は repository.AlertRule 列を表示用 RowView 列へ写す (新スライス生成・元を破壊しない)。
func toRowViews(rules []repository.AlertRule) []component.AlertRuleRowView {
	out := make([]component.AlertRuleRowView, 0, len(rules))
	for _, r := range rules {
		out = append(out, toRowView(r))
	}
	return out
}

// toRowView は repository.AlertRule を表示用 RowView へ写す (metric/operator は domain 型へ)。
func toRowView(r repository.AlertRule) component.AlertRuleRowView {
	return component.AlertRuleRowView{
		ID:        r.ID,
		Metric:    domain.Metric(r.Metric),
		Operator:  domain.ComparisonOperator(r.Operator),
		Threshold: pgconv.NumericToFloat(r.Threshold),
		IsEnabled: r.IsEnabled,
	}
}

// toDeviceOptions は所有デバイス列を選択肢へ写し、selectedID を選択状態にする。
func toDeviceOptions(devices []repository.Device, selectedID int64) []component.DeviceOption {
	out := make([]component.DeviceOption, 0, len(devices))
	for _, d := range devices {
		out = append(out, component.DeviceOption{
			ID:       d.ID,
			Name:     d.Name,
			Selected: d.ID == selectedID,
		})
	}
	return out
}

// renderAlertRuleOwnerError は authz の sentinel error を HTTP ステータスへ写す。
// 未認証 (fail-closed)→401・不在/論理削除→404・非所有→403・想定外→500
// (本画面の方針: 非所有=403 / 不在=404。要件どおり。RequireAuth が前段で 302 するため 401 は通常未到達)。
func renderAlertRuleOwnerError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, authz.ErrUnauthenticated):
		renderError(c, http.StatusUnauthorized)
	case errors.Is(err, pgx.ErrNoRows):
		renderError(c, http.StatusNotFound)
	case errors.Is(err, authz.ErrNotOwner):
		renderError(c, http.StatusForbidden)
	default:
		renderError(c, http.StatusInternalServerError)
	}
}
