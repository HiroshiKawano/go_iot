package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// fakeAlertRuleRepo は AlertRuleRepo の手書きモック (DB 非依存)。
// map で引き、未登録は pgx.ErrNoRows。各ミューテーションは呼び出し記録と戻り値/エラー注入に対応する。
type fakeAlertRuleRepo struct {
	users    map[int64]repository.User
	devices  map[int64]repository.Device      // GetDevice 用
	byUser   map[int64][]repository.Device    // ListDevicesByUser 用
	rules    map[int64]repository.AlertRule   // GetAlertRule 用
	byDevice map[int64][]repository.AlertRule // ListAlertRulesByDevice 用

	getUserErr  error
	listDevErr  error
	getDevErr   error
	getRuleErr  error
	listRuleErr error

	createResult repository.AlertRule
	createErr    error
	createCalled bool
	lastCreate   repository.CreateAlertRuleParams

	updateResult repository.AlertRule
	updateErr    error
	updateCalled bool
	lastUpdate   repository.UpdateAlertRuleParams

	toggleResult repository.AlertRule
	toggleErr    error
	toggleCalled bool
	toggleID     int64

	softDeleteErr error
	softDeleted   bool
	softDeleteID  int64
}

func (f *fakeAlertRuleRepo) GetUser(_ context.Context, id int64) (repository.User, error) {
	if f.getUserErr != nil {
		return repository.User{}, f.getUserErr
	}
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return repository.User{}, pgx.ErrNoRows
}

func (f *fakeAlertRuleRepo) ListDevicesByUser(_ context.Context, _ int64) ([]repository.Device, error) {
	if f.listDevErr != nil {
		return nil, f.listDevErr
	}
	return f.byUser[0], nil // テストは uid 非依存で byUser[0] に所有デバイス列を入れる
}

func (f *fakeAlertRuleRepo) GetDevice(_ context.Context, id int64) (repository.Device, error) {
	if f.getDevErr != nil {
		return repository.Device{}, f.getDevErr
	}
	if d, ok := f.devices[id]; ok {
		return d, nil
	}
	return repository.Device{}, pgx.ErrNoRows
}

func (f *fakeAlertRuleRepo) GetAlertRule(_ context.Context, id int64) (repository.AlertRule, error) {
	if f.getRuleErr != nil {
		return repository.AlertRule{}, f.getRuleErr
	}
	if r, ok := f.rules[id]; ok {
		return r, nil
	}
	return repository.AlertRule{}, pgx.ErrNoRows
}

func (f *fakeAlertRuleRepo) ListAlertRulesByDevice(_ context.Context, deviceID int64) ([]repository.AlertRule, error) {
	if f.listRuleErr != nil {
		return nil, f.listRuleErr
	}
	return f.byDevice[deviceID], nil
}

func (f *fakeAlertRuleRepo) CreateAlertRule(_ context.Context, arg repository.CreateAlertRuleParams) (repository.AlertRule, error) {
	f.createCalled = true
	f.lastCreate = arg
	if f.createErr != nil {
		return repository.AlertRule{}, f.createErr
	}
	return f.createResult, nil
}

func (f *fakeAlertRuleRepo) UpdateAlertRule(_ context.Context, arg repository.UpdateAlertRuleParams) (repository.AlertRule, error) {
	f.updateCalled = true
	f.lastUpdate = arg
	if f.updateErr != nil {
		return repository.AlertRule{}, f.updateErr
	}
	return f.updateResult, nil
}

func (f *fakeAlertRuleRepo) ToggleAlertRule(_ context.Context, id int64) (repository.AlertRule, error) {
	f.toggleCalled = true
	f.toggleID = id
	if f.toggleErr != nil {
		return repository.AlertRule{}, f.toggleErr
	}
	return f.toggleResult, nil
}

func (f *fakeAlertRuleRepo) SoftDeleteAlertRule(_ context.Context, id int64) error {
	f.softDeleted = true
	f.softDeleteID = id
	return f.softDeleteErr
}

// コンパイル時に AlertRuleRepo を満たすことを保証する。
var _ AlertRuleRepo = (*fakeAlertRuleRepo)(nil)

const ruleTestUID = 7

// alertRuleOwnerRepo は uid=7 が所有する device 200 と、その配下ルール 10 を備えた fake を返す。
// device 300 は他ユーザー(999)所有、rule 20 はその配下 (非所有テスト用)。
func alertRuleOwnerRepo() *fakeAlertRuleRepo {
	return &fakeAlertRuleRepo{
		users: map[int64]repository.User{ruleTestUID: {ID: ruleTestUID, Name: "テスト農場主"}},
		devices: map[int64]repository.Device{
			200: {ID: 200, UserID: ruleTestUID, Name: "ハウスA温湿度計"},
			300: {ID: 300, UserID: 999, Name: "他人のデバイス"},
		},
		byUser: map[int64][]repository.Device{
			0: {{ID: 200, UserID: ruleTestUID, Name: "ハウスA温湿度計"}},
		},
		rules: map[int64]repository.AlertRule{
			10: {ID: 10, DeviceID: 200, Metric: "temperature", Operator: ">", Threshold: pgconv.Numeric2(35.00), IsEnabled: true},
			20: {ID: 20, DeviceID: 300, Metric: "humidity", Operator: "<", Threshold: pgconv.Numeric2(30.00), IsEnabled: true},
		},
		byDevice: map[int64][]repository.AlertRule{
			200: {{ID: 10, DeviceID: 200, Metric: "temperature", Operator: ">", Threshold: pgconv.Numeric2(35.00), IsEnabled: true}},
		},
	}
}

func newAlertRuleRouterWithUser(h *AlertRuleHandler, uid int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	withUser := func(c *gin.Context) { auth.SetUserID(c, uid); c.Next() }
	r.GET("/alerts/rules", withUser, h.Index)
	r.POST("/alerts/rules", withUser, h.Add)
	r.GET("/alerts/rules/:rule/edit", withUser, h.Edit)
	r.PUT("/alerts/rules/:rule", withUser, h.Update)
	r.PATCH("/alerts/rules/:rule/toggle", withUser, h.Toggle)
	r.DELETE("/alerts/rules/:rule", withUser, h.Delete)
	return r
}

func methodRequest(r http.Handler, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func validRuleVals(deviceID int64) url.Values {
	return url.Values{
		"device_id": {strconv.FormatInt(deviceID, 10)},
		"metric":    {"temperature"},
		"operator":  {">"},
		"threshold": {"35.00"},
	}
}

// --- 4.2 Index / デバイス切替 ---

func TestAlertRuleIndex_所有デバイス指定で一覧とフォーム(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := getPath(r, "/alerts/rules?device_id=200")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "<html") {
		t.Error("通常 GET はフルページを返すべき")
	}
	for _, want := range []string{`id="alert-rule-section"`, `id="alert-rule-form"`, `id="alert-rule-row-10"`, "温度", "35.00℃"} {
		if !strings.Contains(body, want) {
			t.Errorf("body に %q が無い", want)
		}
	}
}

// TestAlertRuleIndex_サイドバーはアラートルールのみactiveで文脈リンクなし は、アラートルール
// フルページのサイドバーが「🔔 アラートルール」のみ active で、デバイス文脈リンクを描画しない
// ことを固定する (R1.3/2.4)。device_id クエリを持つが文脈には入らない。
func TestAlertRuleIndex_サイドバーはアラートルールのみactiveで文脈リンクなし(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := getPath(r, "/alerts/rules?device_id=200")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `class="active">🔔 アラートルール`) {
		t.Errorf("アラートルールが active になっていない:\n%s", body)
	}
	if strings.Contains(body, "📟 デバイス詳細") || strings.Contains(body, "📈 センサーデータ履歴") {
		t.Errorf("アラートルールにデバイス文脈リンクが描画されている (R1.3 違反):\n%s", body)
	}
}

func TestAlertRuleIndex_device_id省略で先頭デバイス(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := getPath(r, "/alerts/rules")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	// 先頭デバイス(200)のルールが描画される
	if !strings.Contains(w.Body.String(), `id="alert-rule-row-10"`) {
		t.Error("device_id 省略で先頭デバイスのルールを表示すべき")
	}
}

func TestAlertRuleIndex_不在デバイスは404(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := getPath(r, "/alerts/rules?device_id=999")
	if w.Code != http.StatusNotFound {
		t.Errorf("不在デバイス: status=%d, want 404", w.Code)
	}
}

func TestAlertRuleIndex_非所有デバイスは403(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := getPath(r, "/alerts/rules?device_id=300")
	if w.Code != http.StatusForbidden {
		t.Errorf("非所有デバイス: status=%d, want 403", w.Code)
	}
}

func TestAlertRuleIndex_所有デバイス0件で案内(t *testing.T) {
	repo := alertRuleOwnerRepo()
	repo.byUser = map[int64][]repository.Device{0: {}} // 所有デバイス0件
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := getPath(r, "/alerts/rules")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "デバイスがありません") {
		t.Error("0件時は案内文を表示すべき")
	}
}

func TestAlertRuleIndex_HXでセクション部分返却(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := hxGet(r, "/alerts/rules?device_id=200")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "<html") {
		t.Error("HX-Request はフルページではなく部分(section)を返すべき")
	}
	if !strings.Contains(body, `id="alert-rule-section"`) {
		t.Error("HX-Request は alert-rule-section を返すべき")
	}
}

// --- 4.3 Add ---

func TestAlertRuleAdd_成功で新ルールとフォーム空(t *testing.T) {
	repo := alertRuleOwnerRepo()
	repo.createResult = repository.AlertRule{ID: 11, DeviceID: 200, Metric: "temperature", Operator: ">", Threshold: pgconv.Numeric2(35.00), IsEnabled: true}
	// 作成後の一覧 (既存10 + 新規11)
	repo.byDevice[200] = []repository.AlertRule{
		{ID: 10, DeviceID: 200, Metric: "temperature", Operator: ">", Threshold: pgconv.Numeric2(35.00), IsEnabled: true},
		{ID: 11, DeviceID: 200, Metric: "temperature", Operator: ">", Threshold: pgconv.Numeric2(35.00), IsEnabled: true},
	}
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := formRequest(r, http.MethodPost, "/alerts/rules", validRuleVals(200))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if !repo.createCalled {
		t.Fatal("CreateAlertRule が呼ばれていない")
	}
	if !repo.lastCreate.IsEnabled {
		t.Error("新規ルールは有効状態 (is_enabled=true) で作成すべき")
	}
	if repo.lastCreate.DeviceID != 200 {
		t.Errorf("CreateAlertRule.DeviceID=%d, want 200", repo.lastCreate.DeviceID)
	}
	// 指標・条件がフォーム値どおり保存される (ラベルや空文字の誤送信を検出・§4.12/§4.13)。
	if repo.lastCreate.Metric != "temperature" {
		t.Errorf("CreateAlertRule.Metric=%q, want temperature", repo.lastCreate.Metric)
	}
	if repo.lastCreate.Operator != ">" {
		t.Errorf("CreateAlertRule.Operator=%q, want >", repo.lastCreate.Operator)
	}
	body := w.Body.String()
	if !strings.Contains(body, `id="alert-rule-row-11"`) {
		t.Error("新ルールが一覧に現れるべき")
	}
	// フォームが空に戻る (追加モード・hx-post)
	if !strings.Contains(body, `hx-post="/alerts/rules"`) {
		t.Error("成功後は空の追加フォーム(hx-post)に戻すべき")
	}
}

func TestAlertRuleAdd_バリデーションエラーは422と入力復元(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	vals := validRuleVals(200)
	vals.Set("metric", "")       // 指標未選択
	vals.Set("threshold", "abc") // 非数値
	w := formRequest(r, http.MethodPost, "/alerts/rules", vals)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d, want 422 (body=%s)", w.Code, w.Body.String())
	}
	if repo.createCalled {
		t.Error("バリデーションエラー時は CreateAlertRule を呼んではならない")
	}
	body := w.Body.String()
	if !strings.Contains(body, "error-message") {
		t.Error("エラーメッセージ領域が描画されるべき")
	}
	if !strings.Contains(body, "指標を選択してください") {
		t.Error("指標エラーが表示されるべき")
	}
	if !strings.Contains(body, `value="abc"`) {
		t.Error("入力値(閾値)を復元すべき")
	}
	// 現一覧を保持
	if !strings.Contains(body, `id="alert-rule-row-10"`) {
		t.Error("422 でも現在の一覧を保持すべき")
	}
}

func TestAlertRuleAdd_非所有デバイスは403(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := formRequest(r, http.MethodPost, "/alerts/rules", validRuleVals(300)) // 他人のデバイス
	if w.Code != http.StatusForbidden {
		t.Errorf("status=%d, want 403", w.Code)
	}
	if repo.createCalled {
		t.Error("非所有デバイスへの追加で CreateAlertRule を呼んではならない")
	}
}

func TestAlertRuleAdd_DBエラーは500(t *testing.T) {
	repo := alertRuleOwnerRepo()
	repo.createErr = pgx.ErrTxClosed // 任意の DB エラー
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := formRequest(r, http.MethodPost, "/alerts/rules", validRuleVals(200))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500", w.Code)
	}
}

// --- 4.4 Edit ---

func TestAlertRuleEdit_既存値と更新送信(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := getPath(r, "/alerts/rules/10/edit")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "<html") {
		t.Error("編集フォームは部分(form)を返すべき")
	}
	if !strings.Contains(body, `hx-put="/alerts/rules/10"`) {
		t.Error("送信先を更新(PUT)に切り替えるべき")
	}
	if !strings.Contains(body, "更新") {
		t.Error("ボタン文言は「更新」にすべき")
	}
	if !strings.Contains(body, `value="35.00"`) {
		t.Error("既存の閾値を復元すべき")
	}
	if !strings.Contains(body, `value="temperature" selected`) {
		t.Error("既存の指標を選択状態にすべき")
	}
}

func TestAlertRuleEdit_不在は404(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := getPath(r, "/alerts/rules/404/edit")
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", w.Code)
	}
}

func TestAlertRuleEdit_非所有は403(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := getPath(r, "/alerts/rules/20/edit") // rule 20 は device 300(他人)
	if w.Code != http.StatusForbidden {
		t.Errorf("status=%d, want 403", w.Code)
	}
}

// --- 4.5 Update ---

func TestAlertRuleUpdate_成功で値反映とis_enabled保全(t *testing.T) {
	repo := alertRuleOwnerRepo()
	// rule 10 は is_enabled=true。更新では有効状態を保全する。
	repo.updateResult = repository.AlertRule{ID: 10, DeviceID: 200, Metric: "humidity", Operator: "<", Threshold: pgconv.Numeric2(40.00), IsEnabled: true}
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	vals := url.Values{"metric": {"humidity"}, "operator": {"<"}, "threshold": {"40.00"}}
	w := formRequest(r, http.MethodPut, "/alerts/rules/10", vals)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if !repo.updateCalled {
		t.Fatal("UpdateAlertRule が呼ばれていない")
	}
	if !repo.lastUpdate.IsEnabled {
		t.Error("更新で is_enabled の現在値(true)を保全すべき")
	}
	if repo.lastUpdate.Metric != "humidity" || repo.lastUpdate.Operator != "<" {
		t.Errorf("更新値が反映されていない: %+v", repo.lastUpdate)
	}
	// 成功後は空の追加フォーム(hx-post)に戻る
	if !strings.Contains(w.Body.String(), `hx-post="/alerts/rules"`) {
		t.Error("更新成功後は空の追加フォームに戻すべき")
	}
}

func TestAlertRuleUpdate_バリデーションエラーは422で編集フォーム保持(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	vals := url.Values{"metric": {"temperature"}, "operator": {">"}, "threshold": {""}} // 閾値空
	w := formRequest(r, http.MethodPut, "/alerts/rules/10", vals)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d, want 422 (body=%s)", w.Code, w.Body.String())
	}
	if repo.updateCalled {
		t.Error("バリデーションエラー時は UpdateAlertRule を呼んではならない")
	}
	// 編集モードのフォームを保持 (hx-put + 「更新」)
	if !strings.Contains(w.Body.String(), `hx-put="/alerts/rules/10"`) {
		t.Error("422 では編集フォーム(hx-put)を保持すべき")
	}
}

func TestAlertRuleUpdate_不在は404(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := formRequest(r, http.MethodPut, "/alerts/rules/404", validRuleVals(200))
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", w.Code)
	}
}

func TestAlertRuleUpdate_非所有は403(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := formRequest(r, http.MethodPut, "/alerts/rules/20", validRuleVals(300))
	if w.Code != http.StatusForbidden {
		t.Errorf("status=%d, want 403", w.Code)
	}
}

// --- 4.6 Toggle ---

func TestAlertRuleToggle_反転後の行を返す(t *testing.T) {
	repo := alertRuleOwnerRepo()
	// rule 10 (is_enabled=true) を反転 → false
	repo.toggleResult = repository.AlertRule{ID: 10, DeviceID: 200, Metric: "temperature", Operator: ">", Threshold: pgconv.Numeric2(35.00), IsEnabled: false}
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := methodRequest(r, http.MethodPatch, "/alerts/rules/10/toggle")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if !repo.toggleCalled || repo.toggleID != 10 {
		t.Errorf("ToggleAlertRule(10) が呼ばれていない: called=%v id=%d", repo.toggleCalled, repo.toggleID)
	}
	body := w.Body.String()
	if !strings.Contains(body, `id="alert-rule-row-10"`) {
		t.Error("当該行(outerHTML)を返すべき")
	}
	if strings.Contains(body, "checked") {
		t.Error("反転後(無効)は checked を含めないべき")
	}
	// 当該行のみ (他行を含まない)
	if strings.Contains(body, `id="alert-rule-row-11"`) {
		t.Error("当該行のみ返し他行に影響しないべき")
	}
}

func TestAlertRuleToggle_不在は404(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := methodRequest(r, http.MethodPatch, "/alerts/rules/404/toggle")
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", w.Code)
	}
	if repo.toggleCalled {
		t.Error("不在ルールで ToggleAlertRule を呼んではならない")
	}
}

func TestAlertRuleToggle_非所有は403(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := methodRequest(r, http.MethodPatch, "/alerts/rules/20/toggle")
	if w.Code != http.StatusForbidden {
		t.Errorf("status=%d, want 403", w.Code)
	}
}

// --- 4.7 Delete ---

func TestAlertRuleDelete_論理削除と残一覧(t *testing.T) {
	repo := alertRuleOwnerRepo()
	// device 200 にルール 10, 12 がある状態。10 を削除すると残り 12。
	repo.rules[12] = repository.AlertRule{ID: 12, DeviceID: 200, Metric: "humidity", Operator: "<", Threshold: pgconv.Numeric2(20.00), IsEnabled: true}
	repo.byDevice[200] = []repository.AlertRule{
		{ID: 12, DeviceID: 200, Metric: "humidity", Operator: "<", Threshold: pgconv.Numeric2(20.00), IsEnabled: true},
	} // 削除後の一覧 (10 は除外済み)
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := methodRequest(r, http.MethodDelete, "/alerts/rules/10")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if !repo.softDeleted || repo.softDeleteID != 10 {
		t.Errorf("SoftDeleteAlertRule(10) が呼ばれていない: deleted=%v id=%d", repo.softDeleted, repo.softDeleteID)
	}
	body := w.Body.String()
	if strings.Contains(body, "<html") {
		t.Error("削除は部分(list)を返すべき")
	}
	if !strings.Contains(body, `id="alert-rule-list"`) {
		t.Error("alert-rule-list を返すべき")
	}
	if strings.Contains(body, `id="alert-rule-row-10"`) {
		t.Error("削除した行は一覧から消えるべき")
	}
	if !strings.Contains(body, `id="alert-rule-row-12"`) {
		t.Error("残ったルールは一覧に表示すべき")
	}
}

func TestAlertRuleDelete_最後の1件で空状態(t *testing.T) {
	repo := alertRuleOwnerRepo()
	repo.byDevice[200] = []repository.AlertRule{} // 削除後0件
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := methodRequest(r, http.MethodDelete, "/alerts/rules/10")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "設定されていません") {
		t.Error("最後の1件削除で空状態メッセージを表示すべき")
	}
}

func TestAlertRuleDelete_不在は404(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := methodRequest(r, http.MethodDelete, "/alerts/rules/404")
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", w.Code)
	}
	if repo.softDeleted {
		t.Error("不在ルールで SoftDeleteAlertRule を呼んではならない")
	}
}

func TestAlertRuleDelete_非所有は403(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := methodRequest(r, http.MethodDelete, "/alerts/rules/20")
	if w.Code != http.StatusForbidden {
		t.Errorf("status=%d, want 403", w.Code)
	}
	if repo.softDeleted {
		t.Error("非所有ルールで SoftDeleteAlertRule を呼んではならない")
	}
}

// --- 6.1 エラー経路の網羅 (各 DB 500 / 非数値 ID 404 / fail-closed 401) ---

func TestAlertRuleIndex_デバイス一覧取得失敗は500(t *testing.T) {
	repo := alertRuleOwnerRepo()
	repo.listDevErr = pgx.ErrTxClosed
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := getPath(r, "/alerts/rules")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("ListDevicesByUser 失敗: status=%d, want 500", w.Code)
	}
}

func TestAlertRuleIndex_ユーザー取得失敗は500(t *testing.T) {
	repo := alertRuleOwnerRepo()
	repo.getUserErr = pgx.ErrTxClosed
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	// 非 HX フルページ経路で GetUser を呼ぶ。
	w := getPath(r, "/alerts/rules?device_id=200")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("GetUser 失敗: status=%d, want 500", w.Code)
	}
}

func TestAlertRuleIndex_0件案内のユーザー取得失敗は500(t *testing.T) {
	repo := alertRuleOwnerRepo()
	repo.byUser = map[int64][]repository.Device{0: {}} // 0件
	repo.getUserErr = pgx.ErrTxClosed
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := getPath(r, "/alerts/rules")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("0件案内の GetUser 失敗: status=%d, want 500", w.Code)
	}
}

func TestAlertRuleAdd_成功後の一覧取得失敗は500(t *testing.T) {
	repo := alertRuleOwnerRepo()
	repo.createResult = repository.AlertRule{ID: 11, DeviceID: 200}
	repo.listRuleErr = pgx.ErrTxClosed // renderSection の ListAlertRulesByDevice で失敗
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := formRequest(r, http.MethodPost, "/alerts/rules", validRuleVals(200))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("作成後の一覧取得失敗: status=%d, want 500", w.Code)
	}
}

func TestAlertRule_非数値ルールIDは404(t *testing.T) {
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	cases := []struct {
		method, path string
	}{
		{http.MethodGet, "/alerts/rules/abc/edit"},
		{http.MethodPut, "/alerts/rules/abc"},
		{http.MethodPatch, "/alerts/rules/abc/toggle"},
		{http.MethodDelete, "/alerts/rules/abc"},
	}
	for _, tc := range cases {
		var w = methodRequest(r, tc.method, tc.path)
		if tc.method == http.MethodPut {
			w = formRequest(r, tc.method, tc.path, validRuleVals(200))
		}
		if w.Code != http.StatusNotFound {
			t.Errorf("%s %s (非数値 ID): status=%d, want 404", tc.method, tc.path, w.Code)
		}
	}
}

func TestAlertRuleUpdate_DB更新失敗は500(t *testing.T) {
	repo := alertRuleOwnerRepo()
	repo.updateErr = pgx.ErrTxClosed
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	vals := url.Values{"metric": {"humidity"}, "operator": {"<"}, "threshold": {"40.00"}}
	w := formRequest(r, http.MethodPut, "/alerts/rules/10", vals)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("UpdateAlertRule 失敗: status=%d, want 500", w.Code)
	}
}

func TestAlertRuleToggle_DB失敗は500(t *testing.T) {
	repo := alertRuleOwnerRepo()
	repo.toggleErr = pgx.ErrTxClosed
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := methodRequest(r, http.MethodPatch, "/alerts/rules/10/toggle")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("ToggleAlertRule 失敗: status=%d, want 500", w.Code)
	}
}

func TestAlertRuleDelete_DB削除失敗は500(t *testing.T) {
	repo := alertRuleOwnerRepo()
	repo.softDeleteErr = pgx.ErrTxClosed
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := methodRequest(r, http.MethodDelete, "/alerts/rules/10")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("SoftDeleteAlertRule 失敗: status=%d, want 500", w.Code)
	}
}

func TestAlertRule_未認証uidはfail_closedで401(t *testing.T) {
	// uid<=0 は authz が GetAlertRule 前に fail-closed する (通常は RequireAuth が先に 302)。
	repo := alertRuleOwnerRepo()
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, 0) // uid=0

	w := getPath(r, "/alerts/rules/10/edit")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("uid=0 fail-closed: status=%d, want 401", w.Code)
	}
}

func TestAlertRuleAdd_422時の一覧取得失敗は500(t *testing.T) {
	repo := alertRuleOwnerRepo()
	repo.listRuleErr = pgx.ErrTxClosed // 422 再描画の ListAlertRulesByDevice で失敗
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	vals := validRuleVals(200)
	vals.Set("metric", "") // バリデーションエラー → renderSectionWithForm 経路
	w := formRequest(r, http.MethodPost, "/alerts/rules", vals)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("422 再描画の一覧取得失敗: status=%d, want 500", w.Code)
	}
}

func TestAlertRuleDelete_削除後の一覧取得失敗は500(t *testing.T) {
	repo := alertRuleOwnerRepo()
	repo.listRuleErr = pgx.ErrTxClosed // SoftDelete 成功後の ListAlertRulesByDevice で失敗
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := methodRequest(r, http.MethodDelete, "/alerts/rules/10")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("削除後の一覧取得失敗: status=%d, want 500", w.Code)
	}
	if !repo.softDeleted {
		t.Error("一覧取得失敗でも論理削除自体は実行済みのはず")
	}
}

func TestAlertRuleEdit_認可のDBエラーは500(t *testing.T) {
	repo := alertRuleOwnerRepo()
	repo.getRuleErr = pgx.ErrTxClosed // GetAlertRule が DB エラー (ErrNoRows/ErrNotOwner でない)
	r := newAlertRuleRouterWithUser(&AlertRuleHandler{Repo: repo}, ruleTestUID)

	w := getPath(r, "/alerts/rules/10/edit")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("認可の DB エラー: status=%d, want 500", w.Code)
	}
}
