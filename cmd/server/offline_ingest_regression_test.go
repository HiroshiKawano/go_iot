package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/config"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
)

// fakeIngestQuerier は受信経路 (DeviceAuth → SensorAPI.Create → service.AlertEvaluator) が
// 実際に叩く DB メソッドだけを差し替える Querier。repository.Querier を nil で埋め込むことで
// 残り 31 メソッドを「呼ばれたら panic」にしておき、受信経路が想定外の DB アクセス
// (= 外部サービス/別テーブルへの副作用) をしないことの保証も兼ねる (R6.4 オフライン非依存)。
//
// 実 SQLite を使わず in-memory で完結するため、ネットワーク/ファイル I/O が一切発生せず
// 「インターネット非依存」が構造的に成立する (R6.4)。
type fakeIngestQuerier struct {
	// repository.Querier を nil で埋め込み、差し替えていないメソッドが呼ばれたら nil 参照で
	// panic させる。受信経路が想定外の DB アクセス (= 別テーブルへの副作用) をしないことの保証も兼ねる。
	repository.Querier

	tokenErr error                  // GetDeviceTokenByHash が返すエラー (sql.ErrNoRows で 401 を再現)
	device   repository.Device      // 所有者認可 (authz.RequireDeviceOwner) が参照するデバイス
	rules    []repository.AlertRule // アラート同期評価が読むルール

	mu               sync.Mutex
	createReadingCnt int // CreateSensorReading 呼び出し回数 (= 保存された証跡)
	listRulesCnt     int // ListEnabledAlertRulesByDevice 呼び出し回数 (= アラート評価が呼ばれた証跡)
	createHistoryCnt int // CreateAlertHistory 呼び出し回数 (= アラート履歴化された証跡)
}

func (f *fakeIngestQuerier) GetDeviceTokenByHash(_ context.Context, _ string) (repository.DeviceToken, error) {
	if f.tokenErr != nil {
		return repository.DeviceToken{}, f.tokenErr
	}
	// 認証成功: トークン所有者をデバイス所有者と一致させ、後続の所有者認可を通す。
	return repository.DeviceToken{ID: 1, UserID: f.device.UserID}, nil
}

func (f *fakeIngestQuerier) UpdateDeviceTokenLastUsed(_ context.Context, _ int64) error { return nil }

func (f *fakeIngestQuerier) GetDevice(_ context.Context, _ int64) (repository.Device, error) {
	return f.device, nil
}

func (f *fakeIngestQuerier) CreateSensorReading(_ context.Context, arg repository.CreateSensorReadingParams) (repository.SensorReading, error) {
	f.mu.Lock()
	f.createReadingCnt++
	f.mu.Unlock()
	return repository.SensorReading{
		ID:          1,
		DeviceID:    arg.DeviceID,
		Temperature: arg.Temperature,
		Humidity:    arg.Humidity,
		RecordedAt:  arg.RecordedAt,
	}, nil
}

func (f *fakeIngestQuerier) UpdateDeviceLastCommunicated(_ context.Context, _ int64) error {
	return nil
}

func (f *fakeIngestQuerier) ListEnabledAlertRulesByDevice(_ context.Context, _ int64) ([]repository.AlertRule, error) {
	f.mu.Lock()
	f.listRulesCnt++
	f.mu.Unlock()
	return f.rules, nil
}

func (f *fakeIngestQuerier) CreateAlertHistory(_ context.Context, _ repository.CreateAlertHistoryParams) (repository.AlertHistory, error) {
	f.mu.Lock()
	f.createHistoryCnt++
	n := f.createHistoryCnt
	f.mu.Unlock()
	return repository.AlertHistory{ID: int64(n)}, nil
}

// newOfflineApp は本番と同一の合成ルート (newHTTPHandler) を、DB 境界だけ fakeIngestQuerier に
// 差し替えて組み立てる。scs は in-memory (memstore) の既定ストアで足り、health の ping も常に成功にする。
// gin アクセスログはテスト出力を汚すため破棄する。
//
// memstore の cleanup goroutine (1 分間隔) は停止しない: scs v2.9.0 の memstore は
// startCleanup() の `m.stopCleanup = make(chan bool)` と StopCleanup() の読みが未同期で、
// 生成直後に StopCleanup() を呼ぶとライブラリ内で data race になる (-race 検出)。1 分 ticker の
// 軽量 goroutine でプロセス終了時に回収されるため、既存テスト同様に許容リークとする。
func newOfflineApp(t *testing.T, q repository.Querier) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// gin.Logger() は生成時 (newHTTPHandler 内) の gin.DefaultWriter を取り込むため、構築前に破棄先へ差し替える。
	prevWriter := gin.DefaultWriter
	gin.DefaultWriter = io.Discard
	t.Cleanup(func() { gin.DefaultWriter = prevWriter })

	cfg := &config.Config{AppEnv: "development", SessionSecret: "0123456789abcdef0123456789abcdef"}
	return newHTTPHandler(cfg, scs.New(), q, func(context.Context) error { return nil })
}

func postSensorDataWithBearer(app http.Handler, bearer, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/sensor-data", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	return w
}

// TestOfflineIngest_有効Bearerで201保存されアラート同期評価が呼ばれる は、インターネット非依存の
// 受信経路 (DeviceAuth → SensorAPI.Create → AlertEvaluator) を本番配線そのまま (DB はモック) で通し、
// 201・保存・アラート同期評価の発火を観測する回帰テスト (R6.1, R6.2, R6.4)。
func TestOfflineIngest_有効Bearerで201保存されアラート同期評価が呼ばれる(t *testing.T) {
	// 温度 28 > 閾値 25 (operator ">") で必ず発火するルールを 1 件用意し、評価が同期実行される経路を観測する。
	q := &fakeIngestQuerier{
		device: repository.Device{ID: 10, UserID: 7, IsActive: true},
		rules: []repository.AlertRule{
			{ID: 1, DeviceID: 10, Metric: "temperature", Operator: ">", Threshold: 25, IsEnabled: true},
		},
	}
	app := newOfflineApp(t, q)

	body := `{"device_id":10,"temperature":28,"humidity":60,"recorded_at":"2026-06-09T12:00:00Z"}`
	w := postSensorDataWithBearer(app, "any-valid-looking-token", body)

	if got, want := w.Code, http.StatusCreated; got != want {
		t.Fatalf("有効 Bearer の受信ステータス: got %d, want %d (body=%s)", got, want, w.Body.String())
	}
	if q.createReadingCnt != 1 {
		t.Errorf("CreateSensorReading 呼び出し回数 = %d, want 1 (計測が保存されていない)", q.createReadingCnt)
	}
	// 「アラート評価呼び出しをアサート」(タスク 5.1 観測可能完了): 保存後に同期評価が呼ばれる。
	if q.listRulesCnt != 1 {
		t.Errorf("ListEnabledAlertRulesByDevice 呼び出し回数 = %d, want 1 (アラート同期評価が呼ばれていない)", q.listRulesCnt)
	}
	if q.createHistoryCnt != 1 {
		t.Errorf("CreateAlertHistory 呼び出し回数 = %d, want 1 (閾値超過が履歴化されていない)", q.createHistoryCnt)
	}

	var resp struct {
		AlertsFired int `json:"alerts_fired"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("レスポンス JSON のパースに失敗: %v body=%s", err, w.Body.String())
	}
	if resp.AlertsFired != 1 {
		t.Errorf("alerts_fired = %d, want 1 (同期評価の発火件数がレスポンスに反映されていない)", resp.AlertsFired)
	}
}

// TestOfflineIngest_発火しない閾値なら201だが履歴化されない は、受信は成功 (201・評価実行) しつつ
// 閾値を超えない計測ではアラート履歴を作らないことを固定する (R6.2 の境界・誤発火しない)。
func TestOfflineIngest_発火しない閾値なら201だが履歴化されない(t *testing.T) {
	q := &fakeIngestQuerier{
		device: repository.Device{ID: 10, UserID: 7, IsActive: true},
		rules: []repository.AlertRule{
			{ID: 1, DeviceID: 10, Metric: "temperature", Operator: ">", Threshold: 40, IsEnabled: true},
		},
	}
	app := newOfflineApp(t, q)

	body := `{"device_id":10,"temperature":28,"humidity":60,"recorded_at":"2026-06-09T12:00:00Z"}`
	w := postSensorDataWithBearer(app, "any-valid-looking-token", body)

	if got, want := w.Code, http.StatusCreated; got != want {
		t.Fatalf("ステータス: got %d, want %d (body=%s)", got, want, w.Body.String())
	}
	if q.listRulesCnt != 1 {
		t.Errorf("アラート同期評価が呼ばれていない (listRulesCnt=%d)", q.listRulesCnt)
	}
	if q.createHistoryCnt != 0 {
		t.Errorf("閾値未超過なのにアラート履歴が作られた (createHistoryCnt=%d, want 0)", q.createHistoryCnt)
	}
}

// TestOfflineIngest_不正トークンは401で保存も評価もしない は、未登録/不正 Bearer では DeviceAuth が
// 401 を返し、計測の保存もアラート評価も一切起きないこと (副作用ゼロ) を固定する回帰テスト (R6.3)。
func TestOfflineIngest_不正トークンは401で保存も評価もしない(t *testing.T) {
	q := &fakeIngestQuerier{
		tokenErr: sql.ErrNoRows, // 未登録トークン = ハッシュ照合で no rows
		device:   repository.Device{ID: 10, UserID: 7, IsActive: true},
	}
	app := newOfflineApp(t, q)

	body := `{"device_id":10,"temperature":28,"humidity":60,"recorded_at":"2026-06-09T12:00:00Z"}`
	w := postSensorDataWithBearer(app, "unregistered-token", body)

	if got, want := w.Code, http.StatusUnauthorized; got != want {
		t.Fatalf("不正トークンの受信ステータス: got %d, want %d (body=%s)", got, want, w.Body.String())
	}
	if q.createReadingCnt != 0 {
		t.Errorf("401 のはずが計測が保存された (createReadingCnt=%d, want 0)", q.createReadingCnt)
	}
	if q.listRulesCnt != 0 {
		t.Errorf("401 のはずがアラート評価が呼ばれた (listRulesCnt=%d, want 0)", q.listRulesCnt)
	}
}

// TestOfflineIngest_Authorizationヘッダ欠如も401 は、ヘッダ自体が無い (= ESP32 未設定相当) ケースでも
// 401 で弾かれ保存されないことを固定する (R6.3 の多重防御)。
func TestOfflineIngest_Authorizationヘッダ欠如も401(t *testing.T) {
	q := &fakeIngestQuerier{device: repository.Device{ID: 10, UserID: 7}}
	app := newOfflineApp(t, q)

	body := `{"device_id":10,"temperature":28,"humidity":60,"recorded_at":"2026-06-09T12:00:00Z"}`
	w := postSensorDataWithBearer(app, "", body) // Authorization ヘッダ無し

	if got, want := w.Code, http.StatusUnauthorized; got != want {
		t.Fatalf("ヘッダ欠如の受信ステータス: got %d, want %d (body=%s)", got, want, w.Body.String())
	}
	if q.createReadingCnt != 0 {
		t.Errorf("ヘッダ欠如のはずが計測が保存された (createReadingCnt=%d, want 0)", q.createReadingCnt)
	}
}
