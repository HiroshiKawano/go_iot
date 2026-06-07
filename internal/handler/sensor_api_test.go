package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

// newRouter は SensorAPI.Create を単独で呼ぶルータを組み立てる。
// 認証は通した前提でテストするため DeviceAuth は挟まない。
func newRouter(h *SensorAPI) *gin.Engine {
	r := gin.New()
	r.POST("/api/sensor-data", h.Create)
	return r
}

func TestSensorAPI_Create_InvalidJSONSyntax(t *testing.T) {
	h := &SensorAPI{}
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/sensor-data", strings.NewReader("{invalid-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got, want := w.Code, http.StatusBadRequest; got != want {
		t.Errorf("status code: got %d, want %d, body=%s", got, want, w.Body.String())
	}
}

func TestSensorAPI_Create_TypeMismatchReturns400(t *testing.T) {
	h := &SensorAPI{}
	r := newRouter(h)

	// device_id に文字列を入れて UnmarshalTypeError を誘発
	body := `{"device_id":"not-a-number","temperature":20,"humidity":50,"recorded_at":"2026-04-21T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sensor-data", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got, want := w.Code, http.StatusBadRequest; got != want {
		t.Errorf("status code: got %d, want %d", got, want)
	}
}

func TestSensorAPI_Create_ValidationFailureReturns422(t *testing.T) {
	h := &SensorAPI{}
	r := newRouter(h)

	cases := []struct {
		name string
		body string
	}{
		{
			name: "temperature below lower bound",
			body: `{"device_id":1,"temperature":-100,"humidity":50,"recorded_at":"2026-04-21T00:00:00Z"}`,
		},
		{
			name: "temperature above upper bound",
			body: `{"device_id":1,"temperature":200,"humidity":50,"recorded_at":"2026-04-21T00:00:00Z"}`,
		},
		{
			name: "humidity above 100",
			body: `{"device_id":1,"temperature":20,"humidity":150,"recorded_at":"2026-04-21T00:00:00Z"}`,
		},
		{
			name: "device_id missing",
			body: `{"temperature":20,"humidity":50,"recorded_at":"2026-04-21T00:00:00Z"}`,
		},
		{
			name: "recorded_at missing",
			body: `{"device_id":1,"temperature":20,"humidity":50}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/sensor-data", bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if got, want := w.Code, http.StatusUnprocessableEntity; got != want {
				t.Errorf("status code: got %d, want %d, body=%s", got, want, w.Body.String())
			}
		})
	}
}

// --- 認可経路 (所有者認可 / DB エラー) のテスト ---

// fakeSensorRepo は SensorRepo の最小モック (認可経路テスト用)。
type fakeSensorRepo struct {
	device       repository.Device
	getErr       error
	reading      repository.SensorReading
	createErr    error
	createCalled bool
}

func (f *fakeSensorRepo) GetDevice(_ context.Context, _ int64) (repository.Device, error) {
	return f.device, f.getErr
}

func (f *fakeSensorRepo) CreateSensorReading(_ context.Context, _ repository.CreateSensorReadingParams) (repository.SensorReading, error) {
	f.createCalled = true
	return f.reading, f.createErr
}

func (f *fakeSensorRepo) UpdateDeviceLastCommunicated(_ context.Context, _ int64) error {
	return nil
}

// spyEvaluator は AlertEvaluator (consumer interface) の最小スタブ。
// 評価結果・エラーを差し替え、呼び出しを記録する。判定ロジック自体は service 側でテスト済みのため、
// ここではハンドラが「評価を呼ぶ」「結果件数を返す」「失敗しても 201」を検証するためだけに使う。
type spyEvaluator struct {
	alerts []repository.AlertHistory
	err    error
	called bool
}

func (s *spyEvaluator) EvaluateAndNotify(_ context.Context, _ *repository.SensorReading) ([]repository.AlertHistory, error) {
	s.called = true
	return s.alerts, s.err
}

// newRouterWithUser は user_id を注入したうえで SensorAPI.Create を呼ぶルータ。
// DeviceAuth 成功後 (auth.SetUserID 済み) の状態を再現する。
func newRouterWithUser(h *SensorAPI, userID int64) *gin.Engine {
	r := gin.New()
	r.POST("/api/sensor-data", func(c *gin.Context) {
		auth.SetUserID(c, userID)
		c.Next()
	}, h.Create)
	return r
}

// validSensorBody は所有者認可以降の経路へ到達する有効なリクエストボディ。
const validSensorBody = `{"device_id":10,"temperature":20,"humidity":50,"recorded_at":"2026-04-21T00:00:00Z"}`

func postSensorData(t *testing.T, r *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/sensor-data", strings.NewReader(validSensorBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSensorAPI_Create_所有者一致なら201(t *testing.T) {
	repo := &fakeSensorRepo{device: repository.Device{ID: 10, UserID: 7}}
	api := &SensorAPI{Repo: repo, Evaluator: &spyEvaluator{}}
	w := postSensorData(t, newRouterWithUser(api, 7))

	if got, want := w.Code, http.StatusCreated; got != want {
		t.Errorf("status: got %d, want %d, body=%s", got, want, w.Body.String())
	}
	if !repo.createCalled {
		t.Error("所有者一致なのに CreateSensorReading が呼ばれていない")
	}
}

func TestSensorAPI_Create_他ユーザーのデバイスは403かつ保存しない(t *testing.T) {
	repo := &fakeSensorRepo{device: repository.Device{ID: 10, UserID: 999}}
	w := postSensorData(t, newRouterWithUser(&SensorAPI{Repo: repo}, 7))

	if got, want := w.Code, http.StatusForbidden; got != want {
		t.Errorf("status: got %d, want %d, body=%s", got, want, w.Body.String())
	}
	if repo.createCalled {
		t.Error("403 のはずが CreateSensorReading が呼ばれた (BOLA リスク)")
	}
}

func TestSensorAPI_Create_未認証userID0は401かつ保存しない(t *testing.T) {
	// device.UserID も 0 にし、ゼロ値一致で誤って通らない (BOLA 多重防御) ことを固定する。
	repo := &fakeSensorRepo{device: repository.Device{ID: 10, UserID: 0}}
	w := postSensorData(t, newRouterWithUser(&SensorAPI{Repo: repo}, 0))

	if got, want := w.Code, http.StatusUnauthorized; got != want {
		t.Errorf("status: got %d, want %d, body=%s", got, want, w.Body.String())
	}
	if repo.createCalled {
		t.Error("未認証(userID=0)で CreateSensorReading が呼ばれた (BOLA リスク)")
	}
}

func TestSensorAPI_Create_存在しないデバイスは422(t *testing.T) {
	repo := &fakeSensorRepo{getErr: pgx.ErrNoRows}
	w := postSensorData(t, newRouterWithUser(&SensorAPI{Repo: repo}, 7))

	if got, want := w.Code, http.StatusUnprocessableEntity; got != want {
		t.Errorf("status: got %d, want %d, body=%s", got, want, w.Body.String())
	}
}

func TestSensorAPI_Create_デバイス参照のDBエラーは500(t *testing.T) {
	repo := &fakeSensorRepo{getErr: errors.New("db down")}
	w := postSensorData(t, newRouterWithUser(&SensorAPI{Repo: repo}, 7))

	if got, want := w.Code, http.StatusInternalServerError; got != want {
		t.Errorf("status: got %d, want %d, body=%s", got, want, w.Body.String())
	}
}

func TestSensorAPI_Create_保存失敗は500(t *testing.T) {
	repo := &fakeSensorRepo{
		device:    repository.Device{ID: 10, UserID: 7},
		createErr: errors.New("insert failed"),
	}
	w := postSensorData(t, newRouterWithUser(&SensorAPI{Repo: repo}, 7))

	if got, want := w.Code, http.StatusInternalServerError; got != want {
		t.Errorf("status: got %d, want %d, body=%s", got, want, w.Body.String())
	}
}

// --- 2.1 アラート評価の同期接続とレスポンス拡張 ---

func TestSensorAPI_Create_保存後にアラート評価が呼ばれ発火件数を返す(t *testing.T) {
	repo := &fakeSensorRepo{device: repository.Device{ID: 10, UserID: 7}}
	spy := &spyEvaluator{alerts: []repository.AlertHistory{{ID: 1}, {ID: 2}}}
	api := &SensorAPI{Repo: repo, Evaluator: spy}
	w := postSensorData(t, newRouterWithUser(api, 7))

	if got, want := w.Code, http.StatusCreated; got != want {
		t.Fatalf("status: got %d, want %d, body=%s", got, want, w.Body.String())
	}
	if !spy.called {
		t.Error("保存後にアラート評価が呼ばれていない")
	}
	var resp struct {
		AlertsFired int `json:"alerts_fired"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("レスポンス JSON のパースに失敗: %v body=%s", err, w.Body.String())
	}
	if resp.AlertsFired != 2 {
		t.Errorf("alerts_fired: got %d, want 2", resp.AlertsFired)
	}
}

func TestSensorAPI_Create_発火0件なら件数0(t *testing.T) {
	repo := &fakeSensorRepo{device: repository.Device{ID: 10, UserID: 7}}
	api := &SensorAPI{Repo: repo, Evaluator: &spyEvaluator{}} // alerts=nil
	w := postSensorData(t, newRouterWithUser(api, 7))

	if got, want := w.Code, http.StatusCreated; got != want {
		t.Fatalf("status: got %d, want %d", got, want)
	}
	var resp struct {
		AlertsFired int `json:"alerts_fired"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("レスポンス JSON のパースに失敗: %v", err)
	}
	if resp.AlertsFired != 0 {
		t.Errorf("alerts_fired: got %d, want 0", resp.AlertsFired)
	}
}

func TestSensorAPI_Create_評価失敗でも201を維持(t *testing.T) {
	repo := &fakeSensorRepo{device: repository.Device{ID: 10, UserID: 7}}
	spy := &spyEvaluator{err: errors.New("eval failed")}
	api := &SensorAPI{Repo: repo, Evaluator: spy}
	w := postSensorData(t, newRouterWithUser(api, 7))

	// ベストエフォート: 評価が失敗しても受信成功 (201) を妨げない
	if got, want := w.Code, http.StatusCreated; got != want {
		t.Errorf("評価失敗でも 201 を返すべき: got %d, want %d, body=%s", got, want, w.Body.String())
	}
	if !spy.called {
		t.Error("評価が呼ばれていない")
	}
}
