package auth

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/config"
	infradb "github.com/HiroshiKawano/go_iot/internal/infra/db"
	"github.com/alexedwards/scs/sqlite3store"
)

// newSessionDB は WAL/busy_timeout を効かせた実 SQLite ファイル接続を開き、
// sqlite3store 要求スキーマ(db/migrations/00007 と同一)の sessions テーブルを用意する。
// :memory: は接続ごとに別 DB になり sqlite3store の永続検証が成立しないため必ずファイルを使う。
func newSessionDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "session_test.sqlite")
	sqlDB, err := infradb.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	// db/migrations/00007_create_sessions.sql と同一の DDL(token TEXT PK / data BLOB NOT NULL / expiry REAL NOT NULL)。
	if _, err := sqlDB.Exec(`CREATE TABLE sessions (token TEXT PRIMARY KEY, data BLOB NOT NULL, expiry REAL NOT NULL)`); err != nil {
		t.Fatalf("create sessions: %v", err)
	}
	if _, err := sqlDB.Exec(`CREATE INDEX sessions_expiry_idx ON sessions (expiry)`); err != nil {
		t.Fatalf("create index: %v", err)
	}
	return sqlDB
}

// countSessions は sessions テーブルの行数を返す(永続を実 DB の状態で観測するため)。
func countSessions(t *testing.T, sqlDB *sql.DB, token string) int {
	t.Helper()
	var n int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token = ?`, token).Scan(&n); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	return n
}

// TestNewSessionManager_ログイン状態が実SQLiteへ永続しログアウトで失効する は、
// sqlite3store を介して Login→Commit が実 DB へ INSERT($1+julianday)、別 context の Load が
// SELECT で読戻し(R6.1 リクエストまたぎ保持)、Logout の Destroy が DELETE で行を消す(R6.2)経路を
// 実 modernc ドライバで通すスモーク。$1 placeholder + julianday() が modernc で通ることを実機確認する(R-3)。
func TestNewSessionManager_ログイン状態が実SQLiteへ永続しログアウトで失効する(t *testing.T) {
	sqlDB := newSessionDB(t)
	sm := NewSessionManager(sqlDB, &config.Config{AppEnv: "development"})
	// NewSessionManager が起動する 5 分間隔 cleanup goroutine をテスト終了時に停止しリークを防ぐ。
	if store, ok := sm.Store.(*sqlite3store.SQLite3Store); ok {
		t.Cleanup(store.StopCleanup)
	}

	// ログイン: 新規セッション context へ user_id を格納
	ctx, err := sm.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("session load: %v", err)
	}
	if err := Login(ctx, sm, 42, false); err != nil {
		t.Fatalf("Login: %v", err)
	}

	// Commit で実 DB へ永続化(sqlite3store.Commit = REPLACE INTO sessions (token, data, expiry) VALUES ($1, $2, julianday($3)))
	token, _, err := sm.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit(実DBへの永続化): %v", err)
	}
	if token == "" {
		t.Fatal("Commit がトークンを返さない")
	}
	if got := countSessions(t, sqlDB, token); got != 1 {
		t.Fatalf("Commit 後の sessions 行数 = %d, want 1 (実DBへ永続していない)", got)
	}

	// 別 context(リクエストまたぎ)で実 DB から読戻し → user_id が保持されている(R6.1)
	ctx2, err := sm.Load(context.Background(), token)
	if err != nil {
		t.Fatalf("session reload: %v", err)
	}
	if got := UserIDFromSession(ctx2, sm); got != 42 {
		t.Errorf("リロード後の UserIDFromSession = %d, want 42 (永続セッションが読めていない)", got)
	}

	// ログアウト → Destroy が DELETE で行を消す(R6.2)
	if err := Logout(ctx2, sm); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if got := countSessions(t, sqlDB, token); got != 0 {
		t.Errorf("Logout 後の sessions 行数 = %d, want 0 (セッションが失効していない)", got)
	}

	// 失効後に同トークンで読んでも未ログイン(0)へ誘導される
	ctx3, err := sm.Load(context.Background(), token)
	if err != nil {
		t.Fatalf("session reload after logout: %v", err)
	}
	if got := UserIDFromSession(ctx3, sm); got != 0 {
		t.Errorf("ログアウト後の UserIDFromSession = %d, want 0", got)
	}
}

// TestSessionStore_cleanupと並行アクセスでSQLITE_BUSYが起きない は、高頻度 cleanup goroutine
// (DELETE WHERE expiry < julianday('now'))と並行する Commit/Find/Delete が WAL+busy_timeout により
// SQLITE_BUSY を起こさないこと(R6.3)を -race で検証する。$1+julianday の INSERT/SELECT/DELETE 実機確認も兼ねる(R-3)。
func TestSessionStore_cleanupと並行アクセスでSQLITE_BUSYが起きない(t *testing.T) {
	sqlDB := newSessionDB(t)
	// cleanup を 1ms 間隔まで高頻度化し、書込との DELETE 競合を意図的に誘発する。
	store := sqlite3store.NewWithCleanupInterval(sqlDB, time.Millisecond)
	defer store.StopCleanup()

	const writers = 8
	const iters = 25
	errs := make(chan error, writers*iters)
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				token := fmt.Sprintf("tok-%d-%d", id, i)
				if err := store.Commit(token, []byte("session-data"), time.Now().Add(time.Hour)); err != nil {
					errs <- fmt.Errorf("Commit: %w", err)
					return
				}
				if _, _, err := store.Find(token); err != nil {
					errs <- fmt.Errorf("Find: %w", err)
					return
				}
				if err := store.Delete(token); err != nil {
					errs <- fmt.Errorf("Delete: %w", err)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("cleanup と並行アクセスでエラー(SQLITE_BUSY 含む): %v", err)
	}
}
