package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/config"
	infradb "github.com/HiroshiKawano/go_iot/internal/infra/db"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/alexedwards/scs/sqlite3store"
)

// TestServerWiring_実SQLiteで本番配線がブートしhealthが200 は、本番 run() と同一の依存結線
// (NewPool→repository.New→NewSessionManager→newHTTPHandler の pool.PingContext 結線)を
// 実 modernc ドライバ上で組み、起動可能状態(/health 200・DB 疎通)に到達することを検証する(R1.1/9.1)。
// stub ping ではなく実 DB の PingContext を通すことで、型追従(*sql.DB が DBTX/セッション/health を満たす)を実機保証する。
func TestServerWiring_実SQLiteで本番配線がブートしhealthが200(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "wiring_smoke.sqlite")
	pool, err := infradb.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	cfg := &config.Config{AppEnv: "development", SessionSecret: "0123456789abcdef0123456789abcdef"}
	q := repository.New(pool)              // *sql.DB が repository.DBTX を満たす(型追従)
	sm := auth.NewSessionManager(pool, cfg) // *sql.DB を受ける(Task 5.1)
	// 本番ストア(sqlite3store)が結線されていること = セッション永続先が SQLite であることの保証。
	// 併せて cleanup goroutine をテスト終了時に停止しリークを防ぐ。
	store, ok := sm.Store.(*sqlite3store.SQLite3Store)
	if !ok {
		t.Fatalf("NewSessionManager の Store が sqlite3store ではない: %T", sm.Store)
	}
	t.Cleanup(store.StopCleanup)

	// 本番と同じ health 結線(pool.PingContext)で http.Handler を合成
	app := newHTTPHandler(cfg, sm, q, pool.PingContext)

	w := get(app, "/health")
	if w.Code != 200 {
		t.Fatalf("GET /health = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Errorf("health 応答に status:ok が無い: %s", w.Body.String())
	}
}

// TestServerWiring_DB切断時healthは503 は、実 DB を閉じた状態で PingContext が失敗し、
// health 結線が 503(db_unreachable)へ落ちることを実ドライバで検証する(health 結線の異常系)。
func TestServerWiring_DB切断時healthは503(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "wiring_smoke_down.sqlite")
	pool, err := infradb.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	cfg := &config.Config{AppEnv: "development", SessionSecret: "0123456789abcdef0123456789abcdef"}
	sm := auth.NewSessionManager(pool, cfg)
	if store, ok := sm.Store.(*sqlite3store.SQLite3Store); ok {
		t.Cleanup(store.StopCleanup) // cleanup goroutine を停止しリークを防ぐ(1本目と対称)
	}
	app := newHTTPHandler(cfg, sm, repository.New(pool), pool.PingContext)

	// 接続を閉じてから health を叩く → PingContext が失敗し 503 を返す
	if err := pool.Close(); err != nil {
		t.Fatalf("pool.Close: %v", err)
	}
	w := get(app, "/health")
	if w.Code != 503 {
		t.Fatalf("DB 切断後の GET /health = %d, want 503 (body=%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "db_unreachable") {
		t.Errorf("health 応答に db_unreachable が無い: %s", w.Body.String())
	}
}
