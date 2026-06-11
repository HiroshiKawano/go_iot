package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/config"
	"github.com/HiroshiKawano/go_iot/internal/infra/token"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// seedDeviceForLoad は負荷テスト用に user → device(所有) → token(無期限) → 必ず発火する alert_rule を
// 実 DB に用意し、POST /api/sensor-data に使う平文トークンとデバイス ID を返す。
// 発火するルールを 1 件入れることで、受信 1 回あたり「INSERT(計測) + UPDATE(last_communicated)
// + SELECT(ルール) + INSERT(履歴)」という最も書込の重い経路を並行負荷にかける。
func seedDeviceForLoad(t *testing.T, db *sql.DB) (plaintext string, deviceID int64) {
	t.Helper()
	ctx := context.Background()
	q := repository.New(db)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	user, err := q.CreateUser(ctx, repository.CreateUserParams{
		Name: "負荷テストユーザー", Email: "load@example.com", PasswordHash: string(hash),
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	loc := "ハウスA"
	device, err := q.CreateDevice(ctx, repository.CreateDeviceParams{
		UserID: user.ID, Name: "温湿度計", MacAddress: "AA:BB:CC:DD:EE:01", Location: &loc, IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	pt, tokenHash, err := token.Generate()
	if err != nil {
		t.Fatalf("token.Generate: %v", err)
	}
	if _, err := q.CreateDeviceToken(ctx, repository.CreateDeviceTokenParams{
		UserID: user.ID, Name: "load-token", TokenHash: tokenHash,
		Abilities: json.RawMessage(`["sensor:write"]`), ExpiresAt: nil,
	}); err != nil {
		t.Fatalf("CreateDeviceToken: %v", err)
	}
	// temperature > 0 は計測値 28 で常に発火する → 受信ごとに履歴 INSERT が走る (書込負荷の最大化)。
	if _, err := q.CreateAlertRule(ctx, repository.CreateAlertRuleParams{
		DeviceID: device.ID, Metric: "temperature", Operator: ">", Threshold: 0, IsEnabled: true,
	}); err != nil {
		t.Fatalf("CreateAlertRule: %v", err)
	}
	return pt, device.ID
}

// TestConcurrentLoad_受信書込と読取とセッション削除が同時でもSQLITE_BUSY由来の500を出さない は、
// 一時 SQLite DB に対し以下を同時に与える R8 並行安定性テスト:
//   - 書込: 複数 goroutine の連続 POST /api/sensor-data (計測 INSERT + アラート履歴 INSERT)
//   - 書込: 並行するセッション Commit
//   - 書込: 1ms 間隔の高頻度セッション cleanup (期限切れ DELETE)
//   - 読取: GET /health (DB ping) と sensor_readings の直接 SELECT
//
// 検証 (R8.1/R8.2): WAL + busy_timeout(5000) + SetMaxOpenConns(4) (infra/db で実装済) により
// SQLITE_BUSY 由来の 500/503 が 1 件も出ず、全 POST が 201 で完了し、計測が全件保存されること。
// -race 下でも競合検出されないこと (テストスイートは go test -race ./... で回す)。
func TestConcurrentLoad_受信書込と読取とセッション削除が同時でもSQLITE_BUSY由来の500を出さない(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 大量リクエストのアクセスログでテスト出力が汚れるのを防ぐ (gin.Logger は生成時の DefaultWriter を取り込む)。
	prevWriter := gin.DefaultWriter
	gin.DefaultWriter = io.Discard
	t.Cleanup(func() { gin.DefaultWriter = prevWriter })

	db := migratedServerDB(t) // 本番同条件 (NewPool: WAL/busy_timeout/MaxOpenConns=4) + 全 migration 適用
	plaintext, deviceID := seedDeviceForLoad(t, db)

	cfg := &config.Config{AppEnv: "development", SessionSecret: "0123456789abcdef0123456789abcdef"}
	// HTTP セッションは memstore。DB 競合は下の sqlite3store cleanup/commit で与えるため store 自体は使わない。
	// memstore cleanup goroutine は停止しない (scs v2.9.0 の StopCleanup は生成直後呼び出しで
	// ライブラリ内 data race になる既知問題。1 分 ticker の許容リークとする)。
	app := newHTTPHandler(cfg, scs.New(), repository.New(db), db.PingContext)

	// scs cleanup を 1ms 間隔まで高頻度化し、ESP32 INSERT × cleanup DELETE の書込競合を意図的に誘発する。
	store := sqlite3store.NewWithCleanupInterval(db, time.Millisecond)
	t.Cleanup(store.StopCleanup)

	const (
		writers       = 8  // 連続受信 (書込)
		httpReaders   = 4  // GET /health (HTTP 読取 + DB ping)
		dbReaders     = 4  // sensor_readings の直接 SELECT (UI 読取相当)
		sessionWriter = 4  // 並行セッション Commit (書込)
		iters         = 20 // 1 goroutine あたりの反復
	)
	// 各 goroutine は最初のエラーで 1 件送って return するため、送られ得る最大件数は goroutine 数。
	// バッファをこの上限で確保し、送信ブロック由来の wg.Wait() デッドロックを防ぐ。
	errs := make(chan error, writers+httpReaders+dbReaders+sessionWriter)
	var wg sync.WaitGroup

	// --- 書込: POST /api/sensor-data ---
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := `{"device_id":` + strconv.FormatInt(deviceID, 10) + `,"temperature":28,"humidity":60,"recorded_at":"2026-06-09T12:00:00Z"}`
			for i := 0; i < iters; i++ {
				req := httptest.NewRequest(http.MethodPost, "/api/sensor-data", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+plaintext)
				rec := httptest.NewRecorder()
				app.ServeHTTP(rec, req)
				if rec.Code != http.StatusCreated {
					errs <- fmt.Errorf("POST /api/sensor-data = %d, want 201 (DB ビジー由来の失敗の疑い) body=%s", rec.Code, rec.Body.String())
					return
				}
			}
		}()
	}

	// --- 読取: GET /health (DB ping) ---
	for r := 0; r < httpReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				req := httptest.NewRequest(http.MethodGet, "/health", nil)
				rec := httptest.NewRecorder()
				app.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					errs <- fmt.Errorf("GET /health = %d, want 200 (読取が書込競合でブロックされた疑い) body=%s", rec.Code, rec.Body.String())
					return
				}
			}
		}()
	}

	// --- 読取: sensor_readings の直接 SELECT (WAL により writer と並行できることの確認) ---
	for r := 0; r < dbReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				var n int
				if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM sensor_readings`).Scan(&n); err != nil {
					errs <- fmt.Errorf("SELECT COUNT(sensor_readings): %w", err)
					return
				}
			}
		}()
	}

	// --- 書込: 並行セッション Commit (cleanup DELETE との writer 競合を増やす) ---
	for s := 0; s < sessionWriter; s++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				tok := fmt.Sprintf("load-sess-%d-%d", id, i)
				if err := store.Commit(tok, []byte("session-data"), time.Now().Add(time.Hour)); err != nil {
					errs <- fmt.Errorf("session Commit: %w", err)
					return
				}
			}
		}(s)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("並行負荷でエラー (SQLITE_BUSY 由来の 500 含む): %v", err)
	}

	// 全書込が完了している = 受信 INSERT が 1 件も欠落していない (R8.2 待機リトライで完了)。
	var saved int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM sensor_readings WHERE device_id = ?`, deviceID).Scan(&saved); err != nil {
		t.Fatalf("保存件数の確認: %v", err)
	}
	if want := writers * iters; saved != want {
		t.Errorf("保存された計測件数 = %d, want %d (並行下で書込が欠落した)", saved, want)
	}
	// 記録: 本結果が green であれば SetMaxOpenConns(4) のままで R8.1/R8.2 を満たす。
	// この保証は SensorAPI.Create が計測 INSERT・last_communicated UPDATE・アラート履歴 INSERT を
	// いずれも「Tx を張らない個別の単一文 auto-commit」で発行することを前提とする (sensor_api.go)。
	// 将来 read-then-write の明示 Tx を導入すると writer 昇格デッドロックが busy_timeout では解消できず、
	// 本テストの保証範囲を超える (DSN に _txlock=immediate を付与する Decision 6・別フェーズ)。
}
