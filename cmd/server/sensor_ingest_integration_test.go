package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/config"
	infradb "github.com/HiroshiKawano/go_iot/internal/infra/db"
	"github.com/HiroshiKawano/go_iot/internal/infra/token"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
	"github.com/pressly/goose/v3"
	"golang.org/x/crypto/bcrypt"
)

// migratedServerDB は本番同条件(NewPool: WAL/busy_timeout)の実 SQLite ファイルへ
// goose(sqlite3)で全マイグレーションを適用した *sql.DB を返す。
func migratedServerDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "ingest_test.sqlite")
	db, err := infradb.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, thisFile, _, _ := runtime.Caller(0)
	migDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "db", "migrations") // cmd/server → リポジトリルート
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("goose.SetDialect: %v", err)
	}
	goose.SetLogger(goose.NopLogger())
	if err := goose.Up(db, migDir); err != nil {
		t.Fatalf("goose.Up: %v", err)
	}
	return db
}

// TestIntegration_gen_token発行トークンでsensor_dataが201で保存される は、gen-token の観測可能完了
// (発行トークンで POST /api/sensor-data が 201 を返し計測が保存される)を実 SQLite + 実ドライバで end-to-end 検証する。
// gen-token と同じ token.Generate→CreateDeviceToken(ExpiresAt *time.Time)経路でトークンを発行し、
// device_auth(Bearer)→所有者認可→CreateSensorReading の全段が SQLite 上で通ることを確認する(R8.x/R7.4)。
func TestIntegration_gen_token発行トークンでsensor_dataが201で保存される(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := migratedServerDB(t)
	ctx := context.Background()
	q := repository.New(db)

	// user → device(所有) を用意
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	user, err := q.CreateUser(ctx, repository.CreateUserParams{
		Name: "テストユーザー", Email: "ingest@example.com", PasswordHash: string(hash),
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

	// gen-token と同じ経路でトークンを発行 (ExpiresAt は無期限 = nil)
	plaintext, tokenHash, err := token.Generate()
	if err != nil {
		t.Fatalf("token.Generate: %v", err)
	}
	if _, err := q.CreateDeviceToken(ctx, repository.CreateDeviceTokenParams{
		UserID:    user.ID,
		Name:      "ingest-token",
		TokenHash: tokenHash,
		Abilities: json.RawMessage(`["sensor:write"]`),
		ExpiresAt: nil,
	}); err != nil {
		t.Fatalf("CreateDeviceToken: %v", err)
	}

	cfg := &config.Config{AppEnv: "development", SessionSecret: "0123456789abcdef0123456789abcdef"}
	app := newHTTPHandler(cfg, scs.New(), q, db.PingContext)

	// 発行した平文トークンで POST /api/sensor-data
	body := `{"device_id":` + strconv.FormatInt(device.ID, 10) + `,"temperature":28.567,"humidity":65.3,"recorded_at":"2026-06-09T12:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sensor-data", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+plaintext)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("POST /api/sensor-data = %d, want 201 (body=%s)", w.Code, w.Body.String())
	}

	// 計測が実 DB に保存されている (1 件) こと、保存値が R4.1 で2桁量子化されていること
	var count int
	var savedTemp float64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*), IFNULL(MAX(temperature),0) FROM sensor_readings WHERE device_id = ?`, device.ID).Scan(&count, &savedTemp); err != nil {
		t.Fatalf("保存確認クエリ: %v", err)
	}
	if count != 1 {
		t.Errorf("sensor_readings 件数 = %d, want 1 (計測が保存されていない)", count)
	}
	if savedTemp != 28.57 {
		t.Errorf("保存温度 = %v, want 28.57 (Quantize2 による2桁量子化)", savedTemp)
	}
}

// TestIntegration_不正トークンのsensor_dataは401 は、未登録トークンでは device_auth が 401 を返し
// 計測が保存されないこと(移行前と等価)を実 SQLite で確認する。
func TestIntegration_不正トークンのsensor_dataは401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := migratedServerDB(t)
	q := repository.New(db)
	cfg := &config.Config{AppEnv: "development", SessionSecret: "0123456789abcdef0123456789abcdef"}
	app := newHTTPHandler(cfg, scs.New(), q, db.PingContext)

	body := `{"device_id":1,"temperature":25,"humidity":50,"recorded_at":"2026-06-09T12:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sensor-data", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalid-token-value")
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("不正トークン POST /api/sensor-data = %d, want 401 (body=%s)", w.Code, w.Body.String())
	}
	var count int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM sensor_readings`).Scan(&count); err != nil {
		t.Fatalf("保存確認: %v", err)
	}
	if count != 0 {
		t.Errorf("不正トークンで計測が保存された (count=%d, want 0)", count)
	}
}
